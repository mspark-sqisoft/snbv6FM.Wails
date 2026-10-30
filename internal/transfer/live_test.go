package transfer

// 실 서버 전송 시험. 환경변수가 없으면 SKIP. 비밀번호는 환경변수로만 받는다(출력·파일 금지).
//   SNB_APP_DIR             fmenv.ini·root_ca.pem 폴더(예: E:\git\snBv6FM\build\release)
//   SNB_LIVE_REQ_ID / _PW   올리는 사람(test<N>_1). 받기는 같은 계정으로 인터넷망에 로그인한다
//   SNB_LIVE_APR_ID / _PW   결재자(test<N>_2, 올리는 사람의 결재선)
//
//   go test -run Live -v -count=1 ./internal/transfer
//
// 올린 시험 파일(WAILS_TR_*)은 각 시험 끝에 업무망·인터넷망 목록에서 지운다.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"snbapp/internal/approval"
	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/pb/serverfile"
)

func liveLogin(t *testing.T, idKey, pwKey string, loc int) *nbs.Session {
	t.Helper()
	id := os.Getenv(idKey)
	if id == "" {
		t.Skipf("%s 가 없다", idKey)
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, srv := range env.Servers {
		if srv.Loc == loc {
			s, err := nbs.Login(dir, srv, id, os.Getenv(pwKey), "0.1.0-test", slog.Default())
			if err != nil {
				t.Fatalf("%s 로그인(LOC %d): %v", id, loc, err)
			}
			t.Cleanup(func() { s.Close() })
			return s
		}
	}
	t.Fatalf("LOC %d 서버가 없다", loc)
	return nil
}

// livePick 은 올릴 때 실을 결재 칸이다. 사전결재 모드면 결재자(SNB_LIVE_APR_ID)를 싣고, 사후결재 모드면 비운다(4127 은 따로).
func livePick(t *testing.T, s *nbs.Session) *approval.PickResult {
	t.Helper()
	pre, typ := approval.PreMode(s.Profile), s.AprvType(false)
	pk := &approval.PickResult{AprvType: typ, AprvDept: s.Info.UserDeptName, Cause: "WAILS 전송 시험"}
	switch {
	case typ == nbs.AprvSelf:
		pk.ApproverIDs = []string{s.UserID}
	case pre && approval.NeedsApprover(typ):
		apr := os.Getenv("SNB_LIVE_APR_ID")
		if apr == "" {
			t.Skip("사전결재 모드라 결재자가 필요한데 SNB_LIVE_APR_ID 가 없다")
		}
		pk.ApproverIDs = []string{apr}
	}
	pk.Resv, pk.AprvKind = approval.UploadFields(false, pre, typ, len(pk.ApproverIDs))
	return pk
}

// liveJob 은 svc 와 같은 모양의 업로드 일감이다: Prepare 에서 검사값, Run 에서 Send.
func liveJob(s *nbs.Session, path, name string, dir bool, pk *approval.PickResult, done func(Result)) Job {
	var sum string
	st, _ := os.Stat(path)
	stamp, _ := StampOf(path)
	return Job{
		Dir: Up, Name: name, Size: st.Size(), Key: "up:" + name,
		Prepare: func(ctx context.Context, p *Progress) (err error) {
			if s.Profile["fm_checksum"] == "0" {
				return nil
			}
			sum, err = HashFile(ctx, path, p)
			return err
		},
		Run: func(ctx context.Context, p *Progress) error {
			info := &serverfile.FileTransInfo{
				Fpath: name, ApproverId: pk.ApproverIDs, AprvKind: pk.AprvKind, AprvType: pk.AprvType,
				AprvDept: pk.AprvDept, Fcause: pk.Cause, Checksum: sum, Resv: pk.Resv,
			}
			if dir {
				info.DirFlag = 1
			}
			sctx, cancel := SessionCtx(ctx, s)
			defer cancel()
			return Send(sctx, s.ServerFile(), UploadSpec{Path: path, Info: info, Stamp: stamp, Stall: 30 * time.Second}, p)
		},
		Done: done,
	}
}

func writeRandom(t *testing.T, path string, size int, seed uint64) string {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	buf := make([]byte, 64*1024)
	for left := size; left > 0; {
		n := min(left, len(buf))
		for i := 0; i < n; i += 8 {
			v := r.Uint64()
			for j := 0; j < 8 && i+j < n; j++ {
				buf[i+j] = byte(v >> (8 * j))
			}
		}
		f.Write(buf[:n])
		h.Write(buf[:n])
		left -= n
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sumOf(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func findLive(t *testing.T, s *nbs.Session, name string) *serverfile.FileInfo {
	t.Helper()
	list, err := files.List(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range list {
		if f.NameStr == name {
			return f
		}
	}
	return nil
}

func waitLive(t *testing.T, s *nbs.Session, name string, limit time.Duration, cond func(*serverfile.FileInfo) bool) *serverfile.FileInfo {
	t.Helper()
	var f *serverfile.FileInfo
	for end := time.Now().Add(limit); time.Now().Before(end); time.Sleep(time.Second) {
		if f = findLive(t, s, name); f != nil && cond(f) {
			return f
		}
	}
	if f == nil {
		t.Fatalf("%s 가 서버 목록(LOC %d)에 없다", name, s.Server.Loc)
	}
	t.Fatalf("%s 상태가 바뀌지 않았다: state %d virus %d priv %d", name, f.State, f.VirusFlag, f.PrivState)
	return nil
}

// cleanupLive 는 시험 끝에 이름으로 찾아 서버 목록에서 지운다(없으면 넘어간다).
func cleanupLive(t *testing.T, sessions []*nbs.Session, names *[]string) {
	t.Cleanup(func() {
		for _, s := range sessions {
			for _, n := range *names {
				if f := findLive(t, s, n); f != nil {
					err := files.Remove(s, f.ServerStr, false)
					t.Logf("지움(LOC %d) %s: %v", s.Server.Loc, n, err)
				}
			}
		}
	})
}

// heapPeak 은 fn 이 도는 동안 힙을 5ms 마다 재어 시작 때보다 가장 많이 는 양과 누적 할당량을 준다.
func heapPeak(fn func()) (growth int64, alloc uint64) {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak uint64 // 표본 고루틴만 쓰고, sampled 가 닫힌 뒤에 읽는다
	stop, sampled := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(sampled)
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			peak = max(peak, m.HeapAlloc)
			select {
			case <-stop:
				return
			case <-tick.C:
			}
		}
	}()
	fn()
	close(stop)
	<-sampled
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return int64(peak) - int64(base.HeapAlloc), after.TotalAlloc - base.TotalAlloc
}

// 중간에 서버가 끊겨 시험이 지우지 못한 파일(WAILS_TR_*)을 양쪽 망에서 지운다. SNB_LIVE_SWEEP=1 일 때만 돈다.
func TestLiveSweep(t *testing.T) {
	if os.Getenv("SNB_LIVE_SWEEP") != "1" {
		t.Skip("SNB_LIVE_SWEEP 이 없다")
	}
	for _, loc := range []int{config.LocBusiness, config.LocInternet} {
		s := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW", loc)
		list, err := files.List(s)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range list {
			if strings.HasPrefix(f.NameStr, "WAILS_TR_") {
				n++
				t.Logf("지움(LOC %d) %s: %v", loc, f.NameStr, files.Remove(s, f.ServerStr, false))
			}
		}
		t.Logf("LOC %d 목록 %d건 중 시험 파일 %d건", loc, len(list), n)
	}
}

func stampName(tag, ext string) string {
	return "WAILS_TR_" + time.Now().Format("20060102_150405") + "_" + tag + ext
}

// 동시 업로드 6건(20MB 하나 포함): 한도 5까지 함께 돌고, 모두 서버 목록에 같은 크기로 오르고, 힙 증가가 가장 큰 파일보다 작다.
func TestLiveUploadConcurrent(t *testing.T) {
	s := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW", config.LocBusiness)
	pk := livePick(t, s)
	dir := t.TempDir()
	var uploaded []string
	cleanupLive(t, []*nbs.Session{s}, &uploaded)

	sizes := []int{20 << 20, 64 << 10, 256 << 10, 1 << 20, 3 << 20, 5 << 20}
	type entry struct{ path, name string }
	var list []entry
	for i, n := range sizes {
		name := stampName("con"+string(rune('a'+i)), ".bin")
		p := filepath.Join(dir, name)
		writeRandom(t, p, n, uint64(i+1))
		list = append(list, entry{p, name})
	}

	var running, maxActive atomic.Int32
	m := NewManager(5, 100<<20, Hooks{})
	var mu sync.Mutex
	results := map[string]Result{}
	start := time.Now()
	growth, alloc := heapPeak(func() {
		var wg sync.WaitGroup
		for _, e := range list {
			wg.Add(1)
			name := e.name
			j := liveJob(s, e.path, name, false, pk, func(r Result) {
				mu.Lock()
				results[name] = r
				mu.Unlock()
				wg.Done()
			})
			run := j.Run
			j.Run = func(ctx context.Context, p *Progress) error {
				now := running.Add(1)
				defer running.Add(-1)
				for old := maxActive.Load(); now > old && !maxActive.CompareAndSwap(old, now); old = maxActive.Load() {
				}
				return run(ctx, p)
			}
			if _, err := m.Add(j); err != nil {
				t.Error(err)
				wg.Done()
				continue
			}
			uploaded = append(uploaded, name)
		}
		wg.Wait()
	})
	elapsed := time.Since(start)
	total := 0
	for _, n := range sizes {
		total += n
	}
	t.Logf("6건 %.1fMB %v (%.1f MB/s), 동시 최대 %d, 힙 증가 최고 %.1fMB, 누적 할당 %.1fMB",
		float64(total)/(1<<20), elapsed.Round(time.Millisecond), float64(total)/(1<<20)/elapsed.Seconds(), maxActive.Load(),
		float64(growth)/(1<<20), float64(alloc)/(1<<20))
	for _, e := range list {
		if r := results[e.name]; r.State != Done {
			t.Errorf("%s: 상태 %d 오류 %v", e.name, r.State, r.Err)
		}
	}
	if maxActive.Load() < 2 {
		t.Errorf("동시에 돈 것이 %d 건뿐이다", maxActive.Load())
	}
	for i, e := range list {
		f := waitLive(t, s, e.name, 30*time.Second, func(*serverfile.FileInfo) bool { return true })
		if f.FileSize != uint64(sizes[i]) {
			t.Errorf("%s: 서버 크기 %d, 보낸 크기 %d", e.name, f.FileSize, sizes[i])
		}
	}
}

// 진행 중 취소: 취소됨으로 끝나고 서버가 잘린 파일을 커밋하지 않는다(CloseSend 하지 않는다, SR002).
func TestLiveUploadCancel(t *testing.T) {
	s := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW", config.LocBusiness)
	pk := livePick(t, s)
	dir := t.TempDir()
	name := stampName("cancel", ".bin")
	var uploaded = []string{name}
	cleanupLive(t, []*nbs.Session{s}, &uploaded)
	p := filepath.Join(dir, name)
	writeRandom(t, p, 40<<20, 99)

	m := NewManager(5, 100<<20, Hooks{})
	got := make(chan Result, 1)
	n, err := m.Add(liveJob(s, p, name, false, pk, func(r Result) { got <- r }))
	if err != nil {
		t.Fatal(err)
	}
	go func() { // 2MB 쯤 보냈을 때 끊는다(시계 200ms 를 기다리면 빠른 망에서 거의 다 보낸 뒤가 된다)
		for len(got) == 0 {
			if items := m.Items(); len(items) == 1 && items[0].Phase == "send" && items[0].Done > 2<<20 {
				m.Cancel(n)
				return
			}
		}
	}()
	var r Result
	select {
	case r = <-got:
	case <-time.After(2 * time.Minute):
		t.Fatal("2분 안에 끝나지 않았다")
	}
	t.Logf("취소 결과: 상태 %d 오류 %v 보낸 %d", r.State, r.Err, r.Bytes)
	if r.State != Cancelled || !IsCanceled(r.Err) {
		t.Fatalf("취소됨이어야: 상태 %d 오류 %v", r.State, r.Err)
	}
	time.Sleep(3 * time.Second)
	if f := findLive(t, s, name); f != nil {
		t.Errorf("취소한 파일이 서버 목록에 있다(크기 %d)", f.FileSize)
	}
}

// 파일 하나·폴더 하나를 올려 결재받고, 인터넷망에서 같은 계정으로 받는다:
// 검사값 일치, 같은 이름은 번호를 붙여 따로 받고(덮어쓰지 않음), 덮어쓰기면 같은 자리, 폴더는 풀려서 내용이 같다.
func TestLiveApproveDownload(t *testing.T) {
	req := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW", config.LocBusiness)
	apr := liveLogin(t, "SNB_LIVE_APR_ID", "SNB_LIVE_APR_PW", config.LocBusiness)
	typ := req.AprvType(false)
	if !approval.NeedsApprover(typ) {
		t.Skipf("결재방식 %d 는 결재자가 필요 없어 결재 흐름을 볼 수 없다", typ)
	}
	pre := approval.PreMode(req.Profile)
	pk := livePick(t, req)
	src := t.TempDir()

	fileName := stampName("dl", ".bin")
	filePath := filepath.Join(src, fileName)
	dlMB := 30 // SNB_LIVE_DL_MB 로 키워 힙이 파일 크기를 따라가지 않음을 본다
	if v, err := strconv.Atoi(os.Getenv("SNB_LIVE_DL_MB")); err == nil && v > 0 {
		dlMB = v
	}
	fileSum := writeRandom(t, filePath, dlMB<<20, 7)

	folder := filepath.Join(src, stampName("dir", ""))
	want := map[string][]byte{
		"한글 이름.txt":       []byte("snbApp 폴더 전송 시험\n"),
		"sub/깊은/data.bin": bytes.Repeat([]byte{1, 2, 3, 4}, 300000),
		"sub/empty.txt":   {},
	}
	for rel, b := range want {
		p := filepath.Join(folder, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipName := filepath.Base(folder) + ".zip"
	zipPath := filepath.Join(t.TempDir(), zipName)
	if n, err := ZipFolder(context.Background(), folder, zipPath, nil, nil); err != nil || n != len(want) {
		t.Fatalf("압축: %d개 %v", n, err)
	}

	names := []string{fileName, zipName}
	uploaded := append([]string(nil), names...)
	inet := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW", config.LocInternet)
	cleanupLive(t, []*nbs.Session{req, inet}, &uploaded)

	m := NewManager(5, 100<<20, Hooks{})
	var wg sync.WaitGroup
	for i, e := range []struct {
		path string
		dir  bool
	}{{filePath, false}, {zipPath, true}} {
		wg.Add(1)
		name := names[i]
		if _, err := m.Add(liveJob(req, e.path, name, e.dir, pk, func(r Result) {
			if r.State != Done {
				t.Errorf("%s 올리기: %v", name, r.Err)
			}
			wg.Done()
		})); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if t.Failed() {
		return
	}

	// 결재요청(사후결재 모드) → 결재자 승인
	srv := map[string]string{}
	if !pre {
		var svr, pws []string
		for _, n := range names {
			f := waitLive(t, req, n, 60*time.Second, func(f *serverfile.FileInfo) bool { return approval.CanRequest(f.VirusFlag, f.PrivState, f.State) })
			svr, pws = append(svr, f.ServerStr), append(pws, f.ZipPswd)
		}
		sent, fails := approval.RequestAfter(req, approval.AfterRequest{Names: svr, Pswds: pws, ApproverIDs: []string{apr.UserID},
			ApproverNames: []string{apr.Info.UserName}, AprvType: typ, Cause: "WAILS 전송 시험"})
		if sent != len(names) || len(fails) > 0 {
			t.Fatalf("결재요청 sent %d fails %v", sent, fails)
		}
	}
	for _, n := range names {
		srv[n] = waitLive(t, req, n, 60*time.Second, func(f *serverfile.FileInfo) bool { return f.State == 0 }).ServerStr
	}
	var targets []approval.Target
	for i := 0; i < 20 && len(targets) < len(names); i++ {
		targets = nil
		pend, err := approval.Pending(apr)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pend {
			for _, n := range names {
				if p.FileNameSvr == srv[n] {
					targets = append(targets, approval.Target{Seqno: p.Seqno, ServerName: p.FileNameSvr, Requester: p.FileUser, AprvRank: p.AprvRank})
				}
			}
		}
		if len(targets) < len(names) {
			time.Sleep(time.Second)
		}
	}
	if len(targets) != len(names) {
		t.Fatalf("결재대상 %d건(%d건이어야)", len(targets), len(names))
	}
	for _, o := range approval.Approve(apr, targets, true, "WAILS 승인") {
		if !o.OK {
			t.Fatalf("승인 실패: %+v", o)
		}
	}

	// 인터넷망에서 받기
	out := t.TempDir()
	down := func(f *serverfile.FileInfo, overwrite bool) (*DownloadResult, error) {
		var res *DownloadResult
		var rerr error
		done := make(chan struct{})
		_, err := m.Add(Job{Dir: Down, Name: f.NameStr, Size: int64(f.FileSize), Key: "down:" + f.ServerStr + time.Now().String(),
			Run: func(ctx context.Context, p *Progress) error {
				sctx, cancel := SessionCtx(ctx, inet)
				defer cancel()
				res, rerr = Receive(sctx, inet.ServerFile(), DownloadSpec{ServerName: f.ServerStr, Dest: filepath.Join(out, files.DisplayName(f)),
					Size: int64(f.FileSize), Folder: f.DirFlag == 1, Overwrite: overwrite, TempDir: t.TempDir(), Biz: inet.IsBusiness()}, p)
				return rerr
			},
			Done: func(Result) { close(done) },
		})
		if err != nil {
			return nil, err
		}
		<-done
		return res, rerr
	}
	ok := func(f *serverfile.FileInfo) bool { return files.Classify(f, inet.Server.Loc, inet.Profile) == 0 }
	ff := waitLive(t, inet, fileName, 120*time.Second, ok)
	var r1 *DownloadResult
	var err error
	begin := time.Now()
	growth, alloc := heapPeak(func() { r1, err = down(ff, false) })
	if err != nil {
		t.Fatalf("받기: %v (원인 %v, 서버 크기 %d, state %d virus %d)", err, errors.Unwrap(err), ff.FileSize, ff.State, ff.VirusFlag)
	}
	took := time.Since(begin)
	t.Logf("%dMB 받기 %v (%.1f MB/s), 힙 증가 최고 %.1fMB, 누적 할당 %.1fMB", dlMB, took.Round(time.Millisecond), float64(dlMB)/took.Seconds(), float64(growth)/(1<<20), float64(alloc)/(1<<20))
	r2, err := down(ff, false)
	if err != nil {
		t.Fatalf("다시 받기: %v", err)
	}
	r3, err := down(ff, true)
	if err != nil {
		t.Fatalf("덮어쓰며 받기: %v", err)
	}
	t.Logf("받은 자리: %s / 같은 이름 → %s / 덮어쓰기 → %s, 서버 검사값 %s", filepath.Base(r1.Path), filepath.Base(r2.Path), filepath.Base(r3.Path), r1.Checksum)
	if r1.Path == r2.Path || r3.Path != r1.Path {
		t.Errorf("이름 충돌 처리: %s %s %s", r1.Path, r2.Path, r3.Path)
	}
	for _, p := range []string{r1.Path, r2.Path} {
		if got := sumOf(t, p); got != fileSum {
			t.Errorf("%s 검사값 %s, 원본 %s", p, got, fileSum)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(out, "*"+PartSuffix)); len(matches) > 0 {
		t.Errorf("임시 파일이 남았다: %v", matches)
	}

	fz := waitLive(t, inet, zipName, 120*time.Second, ok)
	rz, err := down(fz, false)
	if err != nil {
		t.Fatalf("폴더 받기: %v", err)
	}
	for rel, b := range want {
		got, err := os.ReadFile(filepath.Join(rz.Path, filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(got, b) {
			t.Errorf("폴더 %s: %v (크기 %d/%d)", rel, err, len(got), len(b))
		}
	}
	t.Logf("폴더 받기: %s", rz.Path)

	// 받는 중 취소하면 받던 파일이 남지 않는다
	ctx, cancel := context.WithCancel(context.Background())
	p := &Progress{}
	go func() {
		for ctx.Err() == nil {
			if _, d, _ := p.Snapshot(); d > 0 {
				cancel()
			}
			runtime.Gosched()
		}
	}()
	cancelDir := filepath.Join(out, "cancel")
	os.MkdirAll(cancelDir, 0o755)
	sctx, scancel := SessionCtx(ctx, inet)
	_, err = Receive(sctx, inet.ServerFile(), DownloadSpec{ServerName: ff.ServerStr, Dest: filepath.Join(cancelDir, fileName),
		Size: int64(ff.FileSize), TempDir: t.TempDir()}, p)
	scancel()
	cancel()
	_, done, _ := p.Snapshot()
	t.Logf("받는 중 취소: %d 바이트에서 %v", done, err)
	if !IsCanceled(err) {
		t.Errorf("받기 취소: %v", err)
	}
	if left, _ := os.ReadDir(cancelDir); len(left) > 0 {
		t.Errorf("취소 뒤 남은 파일: %d개", len(left))
	}
}
