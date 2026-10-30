package svc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
	"snbapp/pb/common"
	"snbapp/pb/serverfile"
)

func newTestTransfers(ctx context.Context) *Transfers {
	return &Transfers{c: NewCore("test", slog.Default()), login: ctx}
}

// 검사를 지난 자동 업로드는 사람의 답을 기다리므로 사람의 업로드가 밀어내지 못한다(원본 ViewController.cpp:2404).
func TestHandToHuman(t *testing.T) {
	ts := newTestTransfers(context.Background())
	ctx, cancel, token, err := ts.claim(true)
	if err != nil || ctx == nil {
		t.Fatal(err)
	}
	ts.handToHuman(token)
	if _, _, _, err := ts.claim(false); !errors.Is(err, ErrUploadBusy) {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("답을 기다리는 업로드가 밀려났다")
	}
	ts.release(token, cancel)
	if _, cancel, _, err := ts.claim(false); err != nil {
		t.Fatal("놓은 뒤에는 받아야 한다", err)
	} else {
		cancel()
	}
}

// 완료 응답을 못 받은 자동 업로드는 검사 때 내용 그대로면 다시 올리지 않고, 내용이 바뀌면 올린다.
func TestAutoPending(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	save := func(body string) transfer.Stamp {
		t.Helper()
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		st, err := transfer.StampOf(p)
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	ts := newTestTransfers(context.Background())
	ts.autoPaths, ts.autoSent = map[string]bool{}, map[string]UploadRequest{}
	req := UploadRequest{Path: p, Auto: true, stamp: save("old")}
	pending := func() int { return len(ts.autoPending([]string{p})) }
	ts.markAuto(req)
	if pending() != 0 {
		t.Fatal("진행 중인 파일을 또 올린다")
	}
	ts.settleAuto(req, false)
	if pending() != 1 {
		t.Fatal("실패한 파일은 다시 올린다")
	}
	ts.markAuto(req)
	ts.settleAuto(req, true)
	if pending() != 0 {
		t.Fatal("결과를 모르는 같은 내용을 또 올린다")
	}
	save("new content")
	if pending() != 1 || len(ts.autoSent) != 0 {
		t.Fatal("바뀐 내용을 막았거나 표시가 남았다", ts.autoSent)
	}
	// 완료 응답을 기다리는 동안 바뀐 내용은 보낸 내용이 아니다
	ts.markAuto(req)
	ts.settleAuto(req, true)
	if pending() != 1 {
		t.Fatal("기다리는 동안 바뀐 내용을 막았다")
	}
	ts.settleAuto(UploadRequest{Path: p}, true)
	if len(ts.autoSent) != 0 {
		t.Fatal("사람의 업로드에 표시를 남겼다")
	}
}

// 폴더는 안쪽 파일이 바뀌어도 폴더의 수정 시각이 그대로라 파일별 기준값으로 본다.
func TestAutoPendingFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "폴더")
	inner := filepath.Join(dir, "하위", "a.txt")
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	base, err := transfer.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := newTestTransfers(context.Background())
	ts.autoPaths, ts.autoSent = map[string]bool{}, map[string]UploadRequest{}
	ts.settleAuto(UploadRequest{Path: dir, Auto: true, baseline: base}, true)
	if got := ts.autoPending([]string{dir}); len(got) != 0 {
		t.Fatal("같은 내용을 또 올린다", got)
	}
	if err := os.WriteFile(inner, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ts.autoPending([]string{dir}); len(got) != 1 {
		t.Fatal("안쪽 파일이 바뀐 폴더를 막았다")
	}
}

type commitLost struct {
	serverfile.UnimplementedServerFileServiceServer
}

func (commitLost) FileUpload(st grpc.BidiStreamingServer[serverfile.FileUploadRequest, common.AnswerResponse]) error {
	for {
		if _, err := st.Recv(); err == io.EOF {
			return status.Error(codes.Unavailable, "끊김")
		} else if err != nil {
			return err
		}
	}
}

// 서버의 답 없이 끊긴 자동 업로드는 전송 결과가 표시까지 이어져 다음 주기 후보에서 빠진다.
func TestEnqueueCommitUnknown(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	serverfile.RegisterServerFileServiceServer(srv, commitLost{})
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///fake",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp, err := transfer.StampOf(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := newTestTransfers(context.Background())
	ts.m = transfer.NewManager(5, 100<<20, transfer.Hooks{})
	ts.autoPaths, ts.autoSent = map[string]bool{}, map[string]UploadRequest{}
	done := make(chan error, 1)
	if _, err := ts.enqueueUpload(nbs.NewTestSession(conn), UploadRequest{Path: p, Auto: true, stamp: stamp,
		OnDone: func(e error) { done <- e }}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("끝나지 않았다")
	}
	if got := ts.autoPending([]string{p}); len(got) != 0 {
		t.Fatal("완료 응답을 못 받은 같은 내용을 다음 주기에 또 올린다", got)
	}
}

// 로그인이 표시 맵을 만들어야 완료 응답 미수신을 적을 수 있고, 다시 로그인하면 풀린다.
func TestOnLoginResetsAutoSent(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	st, err := config.Open(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	ts := newTestTransfers(context.Background())
	ts.c.settings = st
	s := &nbs.Session{}
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp, err := transfer.StampOf(p)
	if err != nil {
		t.Fatal(err)
	}
	ts.onLogin(s)
	ts.settleAuto(UploadRequest{Path: p, Auto: true, stamp: stamp}, true)
	if got := ts.autoPending([]string{p}); len(got) != 0 {
		t.Fatal("첫 로그인 뒤 표시가 남지 않았다", got)
	}
	ts.onLogin(s)
	if got := ts.autoPending([]string{p}); len(got) != 1 {
		t.Fatal("다시 로그인해도 풀리지 않았다", got)
	}
}

func TestRequestDownloadBusy(t *testing.T) {
	ts := newTestTransfers(context.Background())
	ts.reqs = make(chan downloadReq, 1)
	if err := ts.requestDownload(downloadReq{}); err != nil {
		t.Fatal(err)
	}
	if err := ts.requestDownload(downloadReq{}); !errors.Is(err, ErrDownloadBusy) {
		t.Fatal(err)
	}
}

func TestQuietFail(t *testing.T) {
	loggedOut := &transfer.Fail{Code: transfer.CodeLoggedOut}
	changed := &transfer.Fail{Code: transfer.CodeChanged}
	folder := &transfer.Fail{Code: transfer.CodeFolderChanged}
	zip := &transfer.Fail{Code: transfer.CodeZip}
	if !quietFail(loggedOut, false) || !quietFail(changed, true) || !quietFail(folder, true) || !quietFail(zip, true) {
		t.Fatal("로그아웃·자동 업로드의 검사 뒤 바뀜·압축 실패는 창을 띄우지 않는다")
	}
	if quietFail(changed, false) || quietFail(folder, false) || quietFail(zip, false) || quietFail(&transfer.Fail{Code: transfer.CodeOpen}, true) {
		t.Fatal("나머지 실패는 창을 띄운다")
	}
}

// 실패 표시가 있으면 한 번만 다시 걸고, 조회가 이어져도 시계를 밀지 않는다.
func TestRetryAutoDownload(t *testing.T) {
	st, err := config.Open(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	cur := st.Get()
	cur.Auto = map[int]config.AutoTransfer{0: {DownloadUse: true}}
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	ts := newTestTransfers(context.Background())
	ts.c.settings = st
	ts.c.sess = &nbs.Session{}
	ts.retryAutoDownload()
	if ts.autoDn != nil {
		t.Fatal("실패가 없으면 걸지 않는다")
	}
	ts.autoRetry = true
	ts.retryAutoDownload()
	first := ts.autoDn
	if first == nil {
		t.Fatal("실패가 있으면 건다")
	}
	defer first.Stop()
	ts.retryAutoDownload()
	if ts.autoDn != first {
		t.Fatal("시계가 다시 걸렸다")
	}
}

// 파일·폴더 선택 창은 탐색기의 마지막 폴더를 따르지 않는다.
func TestDialogFolder(t *testing.T) {
	st, err := config.Open(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	cur := st.Get()
	cur.LastFolder = t.TempDir()
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	c := NewCore("test", slog.Default())
	c.settings = st
	if c.dialogFolder() == cur.LastFolder || c.startFolder().Path != cur.LastFolder {
		t.Fatal(c.dialogFolder(), c.startFolder().Path)
	}
	cur.InitFolder = t.TempDir()
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	if c.dialogFolder() != cur.InitFolder {
		t.Fatal(c.dialogFolder())
	}
}

func TestUploadStartText(t *testing.T) {
	if got := uploadStartText([]string{"a"}); got != "a  업로드 시작" {
		t.Fatal(got)
	}
	if got := uploadStartText([]string{"a", "b", "c"}); got != "a 외 2 건 업로드 시작" {
		t.Fatal(got)
	}
}

// 겹치는 이름만 묻고, 답대로 덮어쓰기·건너뛰기를 돌려준다. 결재 받기도 이 함수를 쓴다.
func TestAskConflicts(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ts := newTestTransfers(context.Background())
	type out struct {
		over, skip map[int]bool
		ok         bool
	}
	ch := make(chan out, 1)
	go func() {
		over, skip, ok := ts.askConflicts([]files.Candidate{{Dir: dir, Name: "a.txt"}, {Dir: dir, Name: "new.txt"}, {Dir: dir, Name: "b.txt"}},
			func(int) bool { return false })
		ch <- out{over, skip, ok}
	}()
	answer := func(gen int64, choice int) {
		t.Helper()
		for range 500 {
			ts.mu.Lock()
			a := ts.ask
			ts.mu.Unlock()
			if a != nil && a.gen == gen {
				a.ch <- conflictAnswer{choice: choice}
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("질문이 오지 않았다")
	}
	answer(1, ConflictOverwrite)
	answer(2, ConflictCancel)
	r := <-ch
	if !r.ok || len(r.over) != 1 || !r.over[0] || len(r.skip) != 1 || !r.skip[2] {
		t.Fatalf("%+v", r)
	}
}

// 로그아웃이면 묻기를 멈춘다.
func TestAskConflictsLogout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ts := newTestTransfers(ctx)
	if _, _, ok := ts.askConflicts([]files.Candidate{{Dir: dir, Name: "a.txt"}}, func(int) bool { return false }); ok {
		t.Fatal("로그아웃인데 끝까지 물었다")
	}
}
