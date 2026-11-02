package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"snbapp/internal/nbs"
	"snbapp/pb/common"
	"snbapp/pb/serverfile"
)

// 가짜 서버로 업로드·받기 규칙을 본다: 거절 즉시 멈춤, 실패 때 CloseSend 안 함(SR002), 크기·검사값, 번호, 폴더 풀기.

type fakeServer struct {
	serverfile.UnimplementedServerFileServiceServer
	mu sync.Mutex
	// 업로드
	rejectAfter int   // >0 이면 이만큼 조각을 받은 뒤 res=-22
	holdAfter   int   // >0 이면 이만큼 받은 뒤 더 읽지 않는다(정지·취소 시험)
	holdCommit  bool  // 끝까지 받은 뒤 완료 응답을 보내지 않는다
	commitErr   error // 끝까지 받은 뒤 이 오류로 끝낸다
	commitRej   bool  // 끝까지 받은 뒤 res=-22
	gotInfo     *serverfile.FileTransInfo
	gotBytes    []byte
	closedSend  bool // 클라이언트가 CloseSend 로 끝냈다(EOF)
	canceled    bool // 클라이언트가 끊었다
	// 받기
	data     []byte
	checksum string
	clipCode int32
	clipPath string
	clipLen  int32 // 클립보드 첫 응답 length(0 이면 데이터 크기). 실 서버는 암호화된 크기를 준다
	sumFirst bool  // 검사값 응답을 형식 응답보다 먼저 보낸다(실 서버의 fm_checksum)
	dnErr    error
	dnChunk  int // 조각 크기(0 이면 1000)
	gotDown  *serverfile.FileDownloadRequest
	onDown   func() // 받기 응답을 끝내기 전에 부른다
}

func (f *fakeServer) FileUpload(st grpc.BidiStreamingServer[serverfile.FileUploadRequest, common.AnswerResponse]) error {
	chunks := 0
	for {
		m, err := st.Recv()
		f.mu.Lock()
		if err == io.EOF {
			f.closedSend = true
			hold := f.holdCommit
			f.mu.Unlock()
			if hold {
				<-st.Context().Done()
				return st.Context().Err()
			}
			if f.commitErr != nil {
				return f.commitErr
			}
			if f.commitRej {
				return st.Send(&common.AnswerResponse{Res: 0, Code: -22, Reason: "size"})
			}
			return st.Send(&common.AnswerResponse{Res: 1})
		}
		if err != nil {
			f.canceled = true
			f.mu.Unlock()
			return err
		}
		if in := m.GetInfo(); in != nil {
			f.gotInfo = in
			f.mu.Unlock()
			continue
		}
		f.gotBytes = append(f.gotBytes, m.GetChunk()...)
		chunks++
		reject, hold := f.rejectAfter > 0 && chunks >= f.rejectAfter, f.holdAfter > 0 && chunks >= f.holdAfter
		f.mu.Unlock()
		if reject {
			st.Send(&common.AnswerResponse{Res: 0, Code: -22, Reason: "size"})
			<-st.Context().Done()
			f.mu.Lock()
			f.canceled = true
			f.mu.Unlock()
			return st.Context().Err()
		}
		if hold {
			<-st.Context().Done()
			f.mu.Lock()
			f.canceled = true
			f.mu.Unlock()
			return st.Context().Err()
		}
	}
}

func (f *fakeServer) FileDownload(req *serverfile.FileDownloadRequest, st grpc.ServerStreamingServer[serverfile.FileDownloadResponse]) error {
	f.mu.Lock()
	f.gotDown = req
	f.mu.Unlock()
	if f.dnErr != nil {
		return f.dnErr
	}
	if f.onDown != nil {
		f.onDown()
	}
	if f.sumFirst {
		st.Send(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Answer{Answer: &common.AnswerResponse{Res: 1, Code: 3, Reason: f.checksum}}})
	}
	if f.clipCode != 0 {
		length := f.clipLen
		if length == 0 {
			length = int32(len(f.data))
		}
		st.Send(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Answer{Answer: &common.AnswerResponse{
			Res: 1, Code: f.clipCode, Reason: f.clipPath, Length: length}}})
	}
	n := f.dnChunk
	if n == 0 {
		n = 1000
	}
	for i := 0; i < len(f.data); i += n {
		end := min(i+n, len(f.data))
		st.Send(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Chunk{Chunk: f.data[i:end]}})
	}
	return st.Send(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Answer{Answer: &common.AnswerResponse{Res: 1, Code: 3, Reason: f.checksum}}})
}

func startFake(t *testing.T, f *fakeServer) serverfile.ServerFileServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	serverfile.RegisterServerFileServiceServer(s, f)
	go s.Serve(lis)
	conn, err := grpc.NewClient("passthrough:///buf", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(nbs.MaxRecvMsg)), // 앱 채널과 같은 상한
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); s.Stop() })
	return serverfile.NewServerFileServiceClient(conn)
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for range 500 {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("시간 초과")
}

func TestSendOK(t *testing.T) {
	f := &fakeServer{}
	c := startFake(t, f)
	body := bytes.Repeat([]byte("0123456789"), 150_000) // 조각 셋
	p := write(t, t.TempDir(), "a.bin", string(body))
	st, _ := StampOf(p)
	var pr Progress
	err := Send(context.Background(), c, UploadSpec{Path: p, Stamp: st, Info: &serverfile.FileTransInfo{Fpath: "a.bin", Checksum: sum(body)}}, &pr)
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !bytes.Equal(f.gotBytes, body) || f.gotInfo.Fsize != uint64(len(body)) || f.gotInfo.Checksum != sum(body) || !f.closedSend {
		t.Fatalf("server got %d bytes fsize %d closed %v", len(f.gotBytes), f.gotInfo.Fsize, f.closedSend)
	}
	if ph, d, _ := pr.Snapshot(); ph != PhaseCommit || d != int64(len(body)) {
		t.Fatal(ph, d)
	}
}

func TestSendRejectedStopsAndNeverCloseSend(t *testing.T) {
	f := &fakeServer{rejectAfter: 1}
	c := startFake(t, f)
	p := write(t, t.TempDir(), "big.bin", strings.Repeat("x", 30<<20))
	start := time.Now()
	err := Send(context.Background(), c, UploadSpec{Path: p, Limit: "10 MB", Info: &serverfile.FileTransInfo{}}, &Progress{})
	if CodeOf(err) != CodeUpBizSize || !strings.Contains(err.Error(), "(10 MB)") {
		t.Fatal(err)
	}
	waitFor(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.canceled })
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closedSend || len(f.gotBytes) >= 30<<20 {
		t.Fatalf("거절 뒤에도 보냈다: closed %v got %d (%s)", f.closedSend, len(f.gotBytes), time.Since(start))
	}
}

func TestSendCancelNoCloseSend(t *testing.T) {
	f := &fakeServer{holdAfter: 1}
	c := startFake(t, f)
	p := write(t, t.TempDir(), "big.bin", strings.Repeat("x", 30<<20))
	ctx, cancel := context.WithCancel(context.Background())
	var pr Progress
	done := make(chan error)
	go func() { done <- Send(ctx, c, UploadSpec{Path: p, Info: &serverfile.FileTransInfo{}}, &pr) }()
	waitFor(t, func() bool { _, d, _ := pr.Snapshot(); return d > 0 })
	cancel()
	if err := <-done; !errors.Is(err, ErrCanceled) {
		t.Fatal(err)
	}
	waitFor(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.canceled })
	if f.closedSend {
		t.Fatal("취소인데 CloseSend 했다")
	}
}

func TestSendStall(t *testing.T) {
	f := &fakeServer{holdAfter: 1}
	c := startFake(t, f)
	p := write(t, t.TempDir(), "big.bin", strings.Repeat("x", 30<<20))
	err := Send(context.Background(), c, UploadSpec{Path: p, Stall: time.Second, Info: &serverfile.FileTransInfo{}}, &Progress{})
	if CodeOf(err) != CodeStall {
		t.Fatal(err)
	}
}

func TestSendCommitTimeout(t *testing.T) {
	c := startFake(t, &fakeServer{holdCommit: true})
	p := write(t, t.TempDir(), "a.txt", "abc")
	err := Send(context.Background(), c, UploadSpec{Path: p, Commit: 100 * time.Millisecond, Info: &serverfile.FileTransInfo{}}, &Progress{})
	if CodeOf(err) != CodeCommitUnknown || !errors.Is(err, ErrCommitUnknown) || !strings.Contains(err.Error(), "올라갔을 수") {
		t.Fatal(err)
	}
}

// 끝 신호를 보낸 뒤의 실패는 서버의 답이 없으면 결과를 모른다 ― 끊겨도, 취소해도 그렇다. 서버가 실패로 답하면 실패가 확실하다.
func TestSendCommitUnknown(t *testing.T) {
	p := write(t, t.TempDir(), "a.txt", "abc")
	send := func(ctx context.Context, f *fakeServer) error {
		return Send(ctx, startFake(t, f), UploadSpec{Path: p, Info: &serverfile.FileTransInfo{}}, &Progress{})
	}
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal} {
		err := send(context.Background(), &fakeServer{commitErr: status.Error(code, "끊김")})
		if CodeOf(err) != CodeCommitUnknown || !errors.Is(err, ErrCommitUnknown) {
			t.Fatal(code, err)
		}
	}
	err := send(context.Background(), &fakeServer{commitErr: errors.New("failed to save file info to DB")})
	if CodeOf(err) != CodeFailed || errors.Is(err, ErrCommitUnknown) {
		t.Fatal("저장 실패", err)
	}
	err = send(context.Background(), &fakeServer{commitRej: true})
	if CodeOf(err) != CodeUpBizSize || errors.Is(err, ErrCommitUnknown) {
		t.Fatal("거절", err)
	}
	// 시한이 아니라 취소로 끝낸다 ― 시한은 서버에도 전해져 서버가 먼저 끊으면 이쪽 ctx 가 아직 살아 있어 취소로 판정되지 않는다
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	err = send(ctx, &fakeServer{holdCommit: true})
	if !errors.Is(err, ErrCanceled) || !errors.Is(err, ErrCommitUnknown) {
		t.Fatal("취소", err)
	}
}

func TestSendChangedAfterCheck(t *testing.T) {
	c := startFake(t, &fakeServer{})
	p := write(t, t.TempDir(), "a.txt", "abc")
	st, _ := StampOf(p)
	write(t, filepath.Dir(p), "a.txt", "abcd")
	if err := Send(context.Background(), c, UploadSpec{Path: p, Stamp: st, Info: &serverfile.FileTransInfo{}}, &Progress{}); CodeOf(err) != CodeChanged {
		t.Fatal(err)
	}
	if err := Send(context.Background(), c, UploadSpec{Path: p + ".none", Info: &serverfile.FileTransInfo{}}, &Progress{}); CodeOf(err) != CodeOpen {
		t.Fatal(err)
	}
}

func TestReceiveOKAndConflictNumbering(t *testing.T) {
	body := bytes.Repeat([]byte("ab"), 5000)
	f := &fakeServer{data: body, checksum: strings.ToUpper(sum(body))}
	c := startFake(t, f)
	dir := t.TempDir()
	dest := write(t, dir, "a.txt", "old")
	var pr Progress
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S1", Dest: dest, Size: int64(len(body))}, &pr)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(res.Path) != "a_00000.txt" {
		t.Fatal(res.Path)
	}
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatal("원래 파일이 바뀌었다")
	}
	// 덮어쓰기
	res, err = Receive(context.Background(), c, DownloadSpec{ServerName: "S1", Dest: dest, Size: int64(len(body)), Overwrite: true}, &pr)
	if err != nil || res.Path != dest {
		t.Fatal(res, err)
	}
	if b, _ := os.ReadFile(dest); !bytes.Equal(b, body) {
		t.Fatal("덮어쓰지 않았다")
	}
	if _, err := os.Stat(partPath(dest)); err == nil {
		t.Fatal("쓰는 중 파일이 남았다")
	}
}

// 실 서버는 8MiB 조각으로 보낸다 ― gRPC 기본 상한 4MiB 로는 첫 조각에서 끊긴다.
func TestReceiveLargeChunks(t *testing.T) {
	body := bytes.Repeat([]byte{7}, 9<<20)
	c := startFake(t, &fakeServer{data: body, checksum: sum(body), dnChunk: 8 << 20})
	dest := filepath.Join(t.TempDir(), "big.bin")
	var pr Progress
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S1", Dest: dest, Size: int64(len(body))}, &pr)
	if err != nil {
		t.Fatal(err, errors.Unwrap(err))
	}
	if st, _ := os.Stat(res.Path); st == nil || st.Size() != int64(len(body)) {
		t.Fatal("크기가 다르다")
	}
}

func noPart(t *testing.T, dir string) {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	if len(ents) != 0 {
		t.Fatalf("남은 파일: %v", ents[0].Name())
	}
}

func TestReceiveChecksumAndSizeMismatch(t *testing.T) {
	body := []byte("hello")
	c := startFake(t, &fakeServer{data: body, checksum: strings.Repeat("0", 64)})
	dir := t.TempDir()
	_, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: filepath.Join(dir, "a"), Size: 5}, &Progress{})
	if CodeOf(err) != CodeChecksum || !strings.Contains(err.Error(), "Download_file("+sum(body)+")") {
		t.Fatal(err)
	}
	noPart(t, dir)
	c = startFake(t, &fakeServer{data: body, checksum: sum(body)})
	_, err = Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: filepath.Join(dir, "a"), Size: 9}, &Progress{})
	if CodeOf(err) != CodeSizeMismatch || err.Error() != "받은 크기(5 바이트)가 서버 파일 크기(9 바이트)와 달라 저장하지 않았습니다." {
		t.Fatal(err)
	}
	noPart(t, dir)
}

func TestReceiveServerRefusal(t *testing.T) {
	for msg, want := range map[string]int{
		"not allowed to download file to Inner PC":    CodeDnBizNoPerm,
		"not allowed to download file to Internet PC": CodeDnNetNoPerm,
		"invalid Downloadfile name format":            CodeNoData,
		"file size exceeds limit":                     CodeDnBizSize,
	} {
		c := startFake(t, &fakeServer{dnErr: status.Error(codes.PermissionDenied, msg)})
		dir := t.TempDir()
		_, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: filepath.Join(dir, "a"), Size: 1, Biz: true, Limit: "1 GB"}, &Progress{})
		if CodeOf(err) != want {
			t.Errorf("%s: %v", msg, err)
		}
		noPart(t, dir)
	}
}

func TestReceiveFolderMerge(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"same.txt": "new", "하위/n.txt": "n"} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	c := startFake(t, &fakeServer{data: buf.Bytes(), checksum: sum(buf.Bytes())})
	root, tmp := t.TempDir(), t.TempDir()
	dest := filepath.Join(root, "폴더")
	write(t, dest, "same.txt", "old")
	write(t, dest, "keep.txt", "k")
	spec := DownloadSpec{ServerName: "S", Dest: dest, Size: int64(buf.Len()), Folder: true, TempDir: tmp}
	// 이름 변경: 새 번호 폴더
	res, err := Receive(context.Background(), c, spec, &Progress{})
	if err != nil || filepath.Base(res.Path) != "폴더_00000" {
		t.Fatal(res, err)
	}
	// 덮어쓰기: 병합
	spec.Overwrite = true
	if res, err = Receive(context.Background(), c, spec, &Progress{}); err != nil || res.Path != dest {
		t.Fatal(res, err)
	}
	for rel, want := range map[string]string{"same.txt": "new", "keep.txt": "k", "하위/n.txt": "n"} {
		if b, _ := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel))); string(b) != want {
			t.Fatalf("%s=%q", rel, b)
		}
	}
	noPart(t, tmp)
}

func TestReceiveClip(t *testing.T) {
	body := []byte("PNGDATA")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body), clipCode: ClipImage, clipPath: "/srv/x/__CLIP__.png"})
	dir := t.TempDir()
	res, err := Receive(context.Background(), c, DownloadSpec{Fpath: "__CLIP__", Dest: filepath.Join(dir, "__CLIP__"), Clip: true, Overwrite: true}, &Progress{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(res.Path) != "__CLIP__.png" || res.Format != ClipImage || res.ServerPath != "/srv/x/__CLIP__.png" {
		t.Fatalf("%+v", res)
	}
}

// 실 서버의 클립보드 첫 응답 length 는 암호화된 크기다(원본 실측 1000 바이트 ↔ 1103). 받은 바이트와 대조하면 안 된다.
func TestReceiveClipLengthIsEncryptedSize(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 1000)
	c := startFake(t, &fakeServer{data: body, checksum: sum(body), clipCode: ClipText, clipPath: "/srv/x/__CLIP__.txt", clipLen: 1103})
	dir := t.TempDir()
	res, err := Receive(context.Background(), c, DownloadSpec{Fpath: "__CLIP__", Dest: filepath.Join(dir, "__CLIP__"), Clip: true, Overwrite: true}, &Progress{})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(res.Path); !bytes.Equal(b, body) {
		t.Fatal("받은 내용이 다르다")
	}
}

// 결재자의 클립보드 받기는 서버 파일명(spath)만 보내고 서버는 형식 응답을 주지 않는다 ― 서버 이름으로 확장자를 고른다(CLIP-040).
func TestReceiveClipBySpath(t *testing.T) {
	body := []byte("글")
	f := &fakeServer{data: body, checksum: sum(body)}
	c := startFake(t, f)
	dest := filepath.Join(t.TempDir(), "__CLIP__7")
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S1.TXT", Dest: dest, Clip: true}, &Progress{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != dest+".txt" || f.gotDown.Spath != "S1.TXT" || f.gotDown.Fpath != "" {
		t.Fatalf("%s %+v", res.Path, f.gotDown)
	}
}

// 서버가 검사값을 형식 응답보다 먼저 보내도 검사값을 형식으로 읽지 않는다.
func TestReceiveClipChecksumFirst(t *testing.T) {
	body := []byte("PNGDATA")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body), sumFirst: true, clipCode: ClipImage, clipPath: "/srv/x/__CLIP__.png"})
	dest := filepath.Join(t.TempDir(), "__CLIP__")
	res, err := Receive(context.Background(), c, DownloadSpec{Fpath: "__CLIP__", Dest: dest, Clip: true, Overwrite: true}, &Progress{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != dest+".png" || res.Format != ClipImage || res.ServerPath != "/srv/x/__CLIP__.png" || res.Checksum != sum(body) {
		t.Fatalf("%+v", res)
	}
}

// 앞머리가 없는 .snbpart 는 사용자 파일이다 ― 받기가 건드리지 않는다.
func TestReceiveKeepsExistingPart(t *testing.T) {
	body := []byte("new")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body)})
	dir := t.TempDir()
	dest := filepath.Join(dir, "a.txt")
	mine := write(t, dir, "a.txt"+PartSuffix, "mine")
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3}, &Progress{})
	if err != nil || res.Path != dest {
		t.Fatal(res, err)
	}
	if b, _ := os.ReadFile(mine); string(b) != "mine" {
		t.Fatal("있던 파일이 바뀌었다", string(b))
	}
	if names, _ := os.ReadDir(dir); len(names) != 2 {
		t.Fatal("임시 파일이 남았다", names)
	}
}

// 강제 종료가 남긴 쓰는 중 파일은 다시 받을 때 치운다 ― 재시도마다 쌓이지 않는다.
func TestReceiveReplacesLeftoverPart(t *testing.T) {
	body := []byte("new")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body)})
	dir := t.TempDir()
	dest := filepath.Join(dir, "a.txt")
	write(t, dir, filepath.Base(partPath(dest)), "잘린 내용")
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3}, &Progress{})
	if err != nil || res.Path != dest {
		t.Fatal(res, err)
	}
	if names, _ := os.ReadDir(dir); len(names) != 1 {
		t.Fatal("쓰는 중 파일이 남았다", names)
	}
}

// 덮어쓰기라도 바꾸기가 실패하면 원래 파일은 그대로다.
func TestCommitKeepsOriginalOnFailure(t *testing.T) {
	dir := t.TempDir()
	dest := write(t, dir, "a.txt", "old")
	if _, err := commit(filepath.Join(dir, "없는 파일"), dest, true); err == nil {
		t.Fatal("실패해야 한다")
	}
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatal("원래 파일이 사라졌다")
	}
}

func TestCommitOverwritesReadOnly(t *testing.T) {
	dir := t.TempDir()
	dest := write(t, dir, "a.txt", "old")
	if err := os.Chmod(dest, 0o444); err != nil {
		t.Fatal(err)
	}
	saved, err := commit(write(t, dir, "a.txt"+PartSuffix, "new"), dest, true)
	if err != nil || saved != dest {
		t.Fatal(saved, err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "new" {
		t.Fatal(string(b))
	}
}

// 덮어쓸 파일을 다른 프로그램이 열어 두면 바꾸지 못한다 ― 원래 파일과 읽기 전용 표시는 그대로이고 번호 이름으로 남는다.
func TestCommitDestHeld(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 전용")
	}
	dir := t.TempDir()
	dest := write(t, dir, "a.txt", "old")
	if err := os.Chmod(dest, 0o444); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close(); os.Chmod(dest, 0o644) })
	saved, err := commit(write(t, dir, "a.txt"+PartSuffix, "new"), dest, true)
	if err != nil || filepath.Base(saved) != "a_00000.txt" {
		t.Fatal(saved, err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatal("원래 파일이 바뀌었다", string(b))
	}
	if st, _ := os.Stat(dest); st.Mode().Perm()&0o200 != 0 {
		t.Fatal("읽기 전용 표시가 풀렸다")
	}
}

// 받는 동안 같은 이름의 파일이 생겨도 덮어쓰기가 아니면 그 파일을 바꾸지 않는다.
func TestCommitAvoidsFileThatAppeared(t *testing.T) {
	dir := t.TempDir()
	dest := write(t, dir, "a.txt", "user")
	saved, err := commit(write(t, dir, "a.txt"+PartSuffix, "new"), dest, false)
	if err != nil || filepath.Base(saved) != "a_00000.txt" {
		t.Fatal(saved, err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "user" {
		t.Fatal("생긴 파일이 바뀌었다", string(b))
	}
}

// 덮어쓰기 요청이 앞 받기에 밀려 번호 자리로 가면 덮어쓰기가 풀린다 ― 사용자가 동의한 자리는 번호 자리가 아니다.
func TestReceiveNumberedIsNotOverwrite(t *testing.T) {
	body := []byte("new")
	dir := t.TempDir()
	dest := filepath.Join(dir, "a.txt")
	alt := filepath.Join(dir, "a_00000.txt")
	reserved.Store(pathKey(dest), true) // 앞 받기가 dest 를 잡고 있다
	defer reserved.Delete(pathKey(dest))
	c := startFake(t, &fakeServer{data: body, checksum: sum(body), onDown: func() {
		for range 5000 { // 번호 자리를 잡은 뒤에 같은 이름의 파일을 만든다
			if _, err := os.Stat(partPath(alt)); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		os.WriteFile(alt, []byte("user"), 0o644)
	}})
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3, Overwrite: true}, &Progress{})
	if err != nil || res.Path == alt {
		t.Fatal(res, err)
	}
	if b, _ := os.ReadFile(alt); string(b) != "user" {
		t.Fatal("번호 자리의 파일이 바뀌었다", string(b))
	}
}

// 쓰는 중 파일 이름이 상한을 넘고 자리에 폴더가 있어도 받기를 시작하고 번호 이름으로 남긴다.
func TestReceiveOverwriteDirLongName(t *testing.T) {
	body := []byte("new")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body)})
	name := strings.Repeat("a", 245)
	dest := filepath.Join(t.TempDir(), name)
	write(t, dest, "x", "y")
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3, Overwrite: true}, &Progress{})
	if err != nil || filepath.Base(res.Path) != name+"_00000" {
		t.Fatal(res, err)
	}
}

// 덮어쓸 자리가 폴더면 번호 이름으로 남기고, 그 경로를 돌려준다.
func TestReceiveOverwriteBlocked(t *testing.T) {
	body := []byte("new")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body)})
	dir := t.TempDir()
	dest := filepath.Join(dir, "a.txt")
	write(t, dest, "x", "y") // dest 를 폴더로 만든다
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3, Overwrite: true}, &Progress{})
	if err != nil || filepath.Base(res.Path) != "a_00000.txt" {
		t.Fatal(res, err)
	}
	if b, _ := os.ReadFile(res.Path); !bytes.Equal(b, body) {
		t.Fatal("돌려준 경로에 받은 내용이 없다")
	}
}

func TestMoveFileKeepsDstOnFailure(t *testing.T) {
	dir := t.TempDir()
	dst := write(t, dir, "d.txt", "old")
	if err := moveFile(filepath.Join(dir, "없는 파일"), dst); err == nil {
		t.Fatal("실패해야 한다")
	}
	if b, _ := os.ReadFile(dst); string(b) != "old" {
		t.Fatal("원래 파일이 사라졌다")
	}
}

// 다른 프로그램이 쓰는 중 파일을 잡아 바꾸기가 실패해도 기존 파일은 남는다. Windows 는 잡힌 파일의 이름을 바꾸지 못한다.
func TestMoveFileKeepsDstWhenPartHeld(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 전용")
	}
	dir := t.TempDir()
	dst := write(t, dir, "d.txt", "old")
	src := write(t, dir, "s.txt", "new")
	for _, p := range []string{src, dst} { // src 를 잡으면 첫 바꾸기가 실패해 복사로 가고, dst 를 잡으면 마지막 바꾸기가 실패한다
		f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
	}
	if err := moveFile(src, dst); err == nil {
		t.Fatal("실패해야 한다")
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "old" {
		t.Fatal("기존 파일이 사라졌다", err)
	}
}

// 병합은 파일마다 자리를 잡지 않으므로 같은 파일을 받는 쪽의 쓰는 중 파일을 건드리지 않는다.
func TestMoveFileKeepsOthersPart(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 전용")
	}
	dir := t.TempDir()
	dst := write(t, dir, "d.txt", "old")
	src := write(t, dir, "s.txt", "new")
	other := write(t, dir, filepath.Base(partPath(dst)), "받는 중")
	f, err := os.Open(dst) // dst 를 잡아 첫 바꾸기를 실패시켜 복사로 보낸다
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	moveFile(src, dst)
	if b, err := os.ReadFile(other); err != nil || string(b) != "받는 중" {
		t.Fatal("받는 쪽의 쓰는 중 파일이 사라졌다", err)
	}
}

// 꼬리를 붙이면 파일 이름 상한(255자)을 넘는 이름도 받는다.
func TestReceiveLongName(t *testing.T) {
	body := []byte("new")
	c := startFake(t, &fakeServer{data: body, checksum: sum(body)})
	dest := filepath.Join(t.TempDir(), strings.Repeat("a", 250))
	res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: 3}, &Progress{})
	if err != nil || res.Path != dest {
		t.Fatal(res, err)
	}
	if b, _ := os.ReadFile(dest); !bytes.Equal(b, body) {
		t.Fatal(string(b))
	}
}

// 최종 이름을 만들 수 없으면 쓰는 중 파일을 만들기 전에 실패한다. 이미 있는 긴 이름은 그 자리에 쓸 수 있으니 임시 이름을 준다.
func TestCreatePartChecksFinalName(t *testing.T) {
	dir := t.TempDir()
	if f, err := createPart(filepath.Join(dir, strings.Repeat("a", 300))); err == nil {
		f.Close()
		t.Fatal("저장할 수 없는 이름은 실패해야 한다")
	}
	if names, _ := os.ReadDir(dir); len(names) != 0 {
		t.Fatal("남은 파일이 있다", names)
	}
	dst := write(t, dir, strings.Repeat("b", 252), "old")
	f, err := createPart(dst)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if b, _ := os.ReadFile(dst); string(b) != "old" {
		t.Fatal("기존 파일이 바뀌었다", string(b))
	}
}

// 바꾸지 못하면 읽기 전용 표시를 되돌린다.
func TestMoveFileKeepsReadOnlyWhenHeld(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 전용")
	}
	dir := t.TempDir()
	dst := write(t, dir, "d.txt", "old")
	src := write(t, dir, "s.txt", "new")
	if err := os.Chmod(dst, 0o444); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close(); os.Chmod(dst, 0o644) })
	if err := moveFile(src, dst); err == nil {
		t.Fatal("실패해야 한다")
	}
	if st, _ := os.Stat(dst); st.Mode().Perm()&0o200 != 0 {
		t.Fatal("읽기 전용 표시가 풀렸다")
	}
}

func TestMoveFileReplacesReadOnly(t *testing.T) {
	dir := t.TempDir()
	dst := write(t, dir, "d.txt", "old")
	src := write(t, dir, "s.txt", "new")
	if err := os.Chmod(dst, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "new" {
		t.Fatal(string(b))
	}
	if _, err := os.Stat(src); err == nil {
		t.Fatal("원본이 남았다")
	}
}

func TestQueueSetSerialMin(t *testing.T) {
	q := NewQueue(5, 100)
	q.Enqueue(1, Up, 50, "")
	q.Enqueue(2, Up, 200, "")
	if !q.Large(Up) {
		t.Fatal("기준 100 이면 200 이 크다")
	}
	if q.SetSerialMin(300); q.Large(Up) {
		t.Fatal("기준 300 이면 크지 않다")
	}
	q.SetSerialMin(100)
	q.Next(Up)
	q.Complete(1, Done)
	if !q.Large(Up) {
		t.Fatal("200 이 아직 대기 중이다")
	}
	if q.Cancel(2); q.Large(Up) {
		t.Fatal("취소하면 빠져야 한다")
	}
}

// 취소가 닿았어도 전송이 성공으로 끝났으면 Done 이다(파일은 이미 옮겨졌다).
func TestManagerCancelAfterSuccessIsDone(t *testing.T) {
	m := NewManager(5, 0, Hooks{})
	started := make(chan struct{})
	result := make(chan Result, 1)
	id, err := m.Add(Job{Dir: Up, Name: "f",
		Run:  func(ctx context.Context, _ *Progress) error { close(started); <-ctx.Done(); return nil },
		Done: func(r Result) { result <- r }})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	m.Cancel(id)
	if r := <-result; r.State != Done || r.Err != nil {
		t.Fatalf("%+v", r)
	}
}

// 끝난 행이 상한을 넘으면 실패 행보다 나머지가 먼저 사라진다 ― 창 없이 지나간 실패는 이 행이 유일한 화면 표시다.
func TestManagerKeepsFailedRows(t *testing.T) {
	m := NewManager(5, 0, Hooks{})
	var finished atomic.Int32
	add := func(run func(context.Context, *Progress) error) int64 {
		t.Helper()
		id, err := m.Add(Job{Dir: Up, Name: "f", Run: run, Done: func(Result) { finished.Add(1) }})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ok := func(context.Context, *Progress) error { return nil }
	failed := add(func(context.Context, *Progress) error { return errors.New("실패") })
	waitFor(t, func() bool { return finished.Load() == 1 })
	oldest := add(ok)
	waitFor(t, func() bool { return finished.Load() == 2 })
	for range maxFinished - 1 {
		add(ok)
	}
	waitFor(t, func() bool { return finished.Load() == maxFinished+1 })
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, kept := m.runs[failed]; !kept {
		t.Fatal("실패 행이 사라졌다")
	}
	if _, kept := m.runs[oldest]; kept {
		t.Fatal("성공 행이 남았다")
	}
}

// 끝난 행이 모두 실패여도 방금 끝난 행은 화면에 나오기 전에 사라지지 않는다.
func TestManagerKeepsJustEnded(t *testing.T) {
	m := NewManager(5, 0, Hooks{})
	var finished atomic.Int32
	done := func(Result) { finished.Add(1) }
	for range maxFinished {
		if _, err := m.Add(Job{Dir: Up, Name: "f", Run: func(context.Context, *Progress) error { return errors.New("실패") }, Done: done}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, func() bool { return finished.Load() == maxFinished })
	last, err := m.Add(Job{Dir: Up, Name: "f", Run: func(context.Context, *Progress) error { return nil }, Done: done})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return finished.Load() == maxFinished+1 })
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, kept := m.runs[last]; !kept {
		t.Fatal("방금 끝난 행이 사라졌다")
	}
}

// 끝난 행은 세션을 쥔 함수를 놓고, 끝난 순서로 maxFinished 를 넘는 만큼 사라진다 ― 오래 걸린 전송의 행은 끝나자마자 사라지지 않는다.
func TestManagerReleasesFinished(t *testing.T) {
	m := NewManager(5, 0, Hooks{})
	var finished atomic.Int32
	job := Job{Dir: Up, Name: "f",
		Prepare: func(context.Context, *Progress) error { return nil },
		Run:     func(context.Context, *Progress) error { return nil },
		Done:    func(Result) { finished.Add(1) }}
	first, err := m.Add(job)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return finished.Load() == 1 })
	m.mu.Lock()
	r := m.runs[first]
	released := r.job.Run == nil && r.job.Done == nil && r.job.Prepare == nil
	m.mu.Unlock()
	if !released {
		t.Fatal("끝난 행이 함수를 쥐고 있다")
	}
	gate := make(chan struct{})
	slow, err := m.Add(Job{Dir: Up, Name: "slow",
		Run:  func(context.Context, *Progress) error { <-gate; return errors.New("실패") },
		Done: func(Result) { finished.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	for range maxFinished {
		if _, err := m.Add(job); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, func() bool { return finished.Load() == maxFinished+1 })
	close(gate)
	waitFor(t, func() bool { return finished.Load() == maxFinished+2 })
	m.mu.Lock()
	_, kept := m.runs[first]
	st := m.q.State(first)
	_, slowKept := m.runs[slow]
	m.mu.Unlock()
	if kept || st != Unknown {
		t.Fatal("가장 먼저 끝난 행이 남았다", kept, st)
	}
	if !slowKept {
		t.Fatal("방금 끝난 행이 사라졌다")
	}
	if n := len(m.Items()); n != maxFinished {
		t.Fatal(n)
	}
}

func TestManagerConcurrencyCancelAndDup(t *testing.T) {
	m := NewManager(5, 100, Hooks{})
	var running, peak atomic.Int32
	release := make(chan struct{})
	results := make(chan Result, 20)
	add := func(size int64, key string) int64 {
		id, err := m.Add(Job{Dir: Up, Name: "f", Size: size, Key: key,
			Run: func(ctx context.Context, p *Progress) error {
				n := running.Add(1)
				for {
					if old := peak.Load(); n <= old || peak.CompareAndSwap(old, n) {
						break
					}
				}
				defer running.Add(-1)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			Done: func(r Result) { results <- r }})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ids := make([]int64, 7)
	for i := range ids {
		ids[i] = add(10, "k"+string(rune('a'+i)))
	}
	if _, err := m.Add(Job{Dir: Up, Key: "ka", Run: func(context.Context, *Progress) error { return nil }}); CodeOf(err) != CodeDuplicateQueue {
		t.Fatal("dup key", err)
	}
	waitFor(t, func() bool { return running.Load() == 5 })
	// 대기 중 취소 → 곧바로 Done(Cancelled)
	if !m.Cancel(ids[6]) {
		t.Fatal("cancel pending")
	}
	if r := <-results; r.State != Cancelled || !IsCanceled(r.Err) {
		t.Fatalf("%+v", r)
	}
	// 진행 중 취소 → ctx 가 끊기고 Cancelled
	if !m.Cancel(ids[0]) {
		t.Fatal("cancel active")
	}
	if r := <-results; r.State != Cancelled {
		t.Fatalf("%+v", r)
	}
	waitFor(t, func() bool { return running.Load() == 5 }) // 대기하던 6번째가 출발
	close(release)
	for range 5 {
		if r := <-results; r.State != Done {
			t.Fatalf("%+v", r)
		}
	}
	if peak.Load() != 5 {
		t.Fatal("peak", peak.Load())
	}
	m.Clear()
	if len(m.Items()) != 0 {
		t.Fatal("clear")
	}
}

func TestManagerSerialAndPrepare(t *testing.T) {
	var states atomic.Value
	m := NewManager(5, 100, Hooks{OnState: func(it []Item) { states.Store(it) }})
	gate := make(chan struct{})
	var prepared atomic.Int32
	mk := func(size int64) Job {
		return Job{Dir: Down, Name: "d", Size: size,
			Prepare: func(ctx context.Context, p *Progress) error { prepared.Add(1); return nil },
			Run:     func(ctx context.Context, p *Progress) error { <-gate; return nil }}
	}
	m.Add(mk(101)) // 큰 것 → 직렬
	m.Add(mk(1))
	waitFor(t, func() bool {
		v, _ := states.Load().([]Item)
		return len(v) == 2 && v[0].State == "active" && v[1].Wait == "serial"
	})
	waitFor(t, func() bool { return prepared.Load() == 2 }) // 차례 전에 미리 준비
	close(gate)
	waitFor(t, func() bool { v, _ := states.Load().([]Item); return len(v) == 2 && v[1].State == "done" })
	// 로그아웃: 대기·진행 모두 사유와 함께 취소
	block := make(chan struct{})
	defer close(block)
	for range 3 {
		m.Add(Job{Dir: Up, Name: "u", Size: 1000, Run: func(ctx context.Context, p *Progress) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-block:
				return nil
			}
		}})
	}
	if n := m.CancelAll(fail(CodeLoggedOut, nil), nil); n != 3 {
		t.Fatal(n)
	}
	waitFor(t, func() bool {
		for _, it := range m.Items() {
			if it.Dir == "upload" && (it.State != "cancelled" || it.Code != CodeLoggedOut) {
				return false
			}
		}
		return true
	})
}
