package svc

// 전송 영역: 업로드·다운로드 요청, 대기열 조회·취소, 파일 중복 확인 창, 자동 업로드·다운로드(docs/port/03·04).
// 대기열·스트림·압축 규칙은 internal/transfer 에 있고, 여기는 세션·정책 검사·결재자 창·화면 사건을 잇는다.
//
// 화면이 부르는 메서드는 TransferService(Wails 에 묶임)에, 다른 영역(클립보드·결재)이 Go 에서 부르는 입구는
// Transfers(묶지 않음 ― *nbs.Session·콜백을 받으므로)에 둔다. Transfers 는 c.Transfers() 로 얻는다.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"snbapp/internal/applog"
	"snbapp/internal/approval"
	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
	"snbapp/pb/serverfile"
)

// 전송 영역이 더하는 사건(events.go 의 EvTransferState·EvTransferProgress·EvDownloadConflict 와 함께).
//   - EvTransferState: []transfer.Item 전체(200ms 묶음)
//   - EvTransferProgress: []transfer.Tick(진행값이 바뀐 행만, 200ms 묶음)
//   - EvDownloadConflict: ConflictAsk ― 답은 TransferService.ConflictReply
const (
	EvTransferOpen  = "transfer:open"           // 사람이 시작한 전송이 대기열에 들어갔다 ― 서랍을 연다
	EvUploadQueued  = "transfer:upload-queued"  // 탐색기 패널의 올리기가 검사를 지나 대기열에 들어갔다 {건수} ― 안내를 띄운다
	EvConflictClose = "transfer:conflict-close" // 떠 있는 파일 중복 확인 창을 닫는다 {gen}(로그아웃)
)

// 파일 중복 확인 창의 답(ConflictReply choice).
const (
	ConflictOverwrite = 0
	ConflictRename    = 1
	ConflictCancel    = 2
)

// 사람에게 보이는 문구(원본 tr 원문).
const (
	msgNoUploadPerm   = "현재 망에서는 업로드할 수 없습니다."
	msgNoDownloadDir  = "다운로드할 폴더를 선택해 주세요."
	msgSelectDownload = "다운로드할 파일을 선택해 주세요."
	titleTransferFail = "파일 송수신 실패"
	clipDisplay       = "클립보드"
	clipMarker        = "__CLIP__"
)

// ConflictAsk 는 EvDownloadConflict 의 값이다. 제목 「파일 중복 확인」, 본문 「중복된 파일이 존재합니다. 덮어쓰시겠습니까?」 + Name.
type ConflictAsk struct {
	Gen    int64  `json:"gen"`
	Name   string `json:"name"`   // 표의 [파일명]
	Path   string `json:"path"`   // 겹친 PC 경로
	Folder bool   `json:"folder"` // 폴더면 [덮어쓰기] = 병합
	Index  int    `json:"index"`  // 이 요청에서 몇 번째 질문(1부터)
	Total  int    `json:"total"`  // 이 요청의 질문 수
}

// ErrUploadBusy 는 사람이 답을 기다리는 업로드(검사·결재자 창)가 있어 새 업로드를 받지 않을 때다.
var ErrUploadBusy = approval.ErrBusy

// TransferService 는 업로드·다운로드 단추, 전송 상태 서랍, 파일 중복 확인 창이 부른다.
type TransferService struct {
	base
	t *Transfers
}

func newTransferService(c *Core) *TransferService {
	return &TransferService{base: base{c}, t: c.Transfers()}
}

// ServiceStartup 은 직렬 기준(fmenv SERIAL_TRANSFER_SIZE)을 읽고, 창의 파일 놓기와 설정 변경을 듣는다.
func (ts *TransferService) ServiceStartup(ctx context.Context, o application.ServiceOptions) error {
	if err := ts.base.ServiceStartup(ctx, o); err != nil {
		return err
	}
	t := ts.t
	t.m.SetSerialMin(t.envInt("SERIAL_TRANSFER_SIZE", 100) << 20)
	if st := t.c.Settings(); st != nil {
		st.OnChange(t.settingsChanged)
	}
	if app := application.Get(); app != nil {
		for _, w := range app.Window.GetAll() {
			t.watchDrop(w)
		}
		app.Window.OnCreate(t.watchDrop)
	}
	return nil
}

// Upload 는 파일 선택 창에서 고른 파일을 올린다(검사·결재자 창·대기열). 고르지 않으면 아무것도 안 한다.
func (ts *TransferService) Upload() error {
	if _, err := ts.c.Session(); err != nil {
		return err
	}
	paths, err := application.Get().Dialog.OpenFile().SetTitle("올릴 파일").SetDirectory(ts.c.dialogFolder()).PromptForMultipleSelection()
	if err != nil || len(paths) == 0 {
		return err
	}
	return ts.t.UploadPaths(paths, false)
}

// UploadFolder 는 폴더 선택 창에서 고른 폴더를 압축해 올린다.
func (ts *TransferService) UploadFolder() error {
	if _, err := ts.c.Session(); err != nil {
		return err
	}
	dir, err := application.Get().Dialog.OpenFile().SetTitle("올릴 폴더").SetDirectory(ts.c.dialogFolder()).
		CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if err != nil || dir == "" {
		return err
	}
	return ts.t.UploadPaths([]string{dir}, false)
}

// UploadPaths 는 탐색기 패널에서 고르거나 끈 경로를 검사 뒤 대기열에 넣는다. 들어가면 EvUploadQueued 로 알린다.
// 권한·입구(답을 기다리는 업로드) 판정만 곧바로 하고, 검사·결재자 창·전송은 뒤에서 돈다.
func (ts *TransferService) UploadPaths(paths []string) error {
	return ts.t.uploadPaths(paths, false, true)
}

// Download 는 폴더 선택 창에서 고른 폴더로 서버 파일들을 받는다. 폴더를 고르지 않으면 아무것도 안 한다.
func (ts *TransferService) Download(serverNames []string) error {
	if len(serverNames) == 0 {
		return errors.New(msgSelectDownload)
	}
	if _, err := ts.c.Session(); err != nil {
		return err
	}
	dir, err := application.Get().Dialog.OpenFile().SetTitle("받을 폴더").SetDirectory(ts.c.dialogFolder()).
		CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).PromptForSingleSelection()
	if err != nil || dir == "" {
		return err
	}
	return ts.t.DownloadTo(serverNames, dir)
}

// DownloadTo 는 서버 파일들을 dir 로 받는 일을 대기열에 넣는다(탐색기 패널의 [다운로드]). 받을 수 없는 파일은 사유를 알린다.
func (ts *TransferService) DownloadTo(serverNames []string, dir string) error {
	return ts.t.DownloadTo(serverNames, dir)
}

// Items 는 대기열 전체다(서랍을 처음 열 때. 그 뒤는 transfer:state·transfer:progress 로 고친다).
func (ts *TransferService) Items() ([]transfer.Item, error) { return ts.t.m.Items(), nil }

// Cancel 은 전송 하나를 취소한다(대기 중이면 빼고, 진행 중이면 끊는다 ― 올리던 것은 서버가, 받던 것은 앱이 지운다).
func (ts *TransferService) Cancel(id int64) error {
	if ts.t.m.Cancel(id) {
		ts.t.log(applog.CatUpload).Info("전송 취소", "id", id)
	}
	return nil
}

// CancelAll 은 대기·진행 중 전송을 모두 취소한다.
func (ts *TransferService) CancelAll() error {
	n := ts.t.m.CancelAll(nil, nil)
	ts.t.log(applog.CatUpload).Info(fmt.Sprintf("전송 모두 취소 >>> %d 건", n))
	return nil
}

// Clear 는 끝난 행을 목록에서 지운다([목록 지우기] ― 진행 중인 전송은 계속된다).
func (ts *TransferService) Clear() error {
	ts.t.m.Clear()
	return nil
}

// ConflictReply 는 파일 중복 확인 창의 답이다. choice: 0 덮어쓰기, 1 이름 변경, 2 취소. applyAll 은 [모두 적용].
// gen 이 지금 질문과 다르면 버린다.
func (ts *TransferService) ConflictReply(gen int64, choice int, applyAll bool) error {
	t := ts.t
	t.mu.Lock()
	a := t.ask
	ok := a != nil && a.gen == gen
	t.mu.Unlock()
	if !ok {
		t.log(applog.CatDownload).Info(fmt.Sprintf("파일 중복 확인 회신 >>> 지난 질문의 답이라 버린다 >>> 답[%d]", gen))
		return nil
	}
	if choice < ConflictOverwrite || choice > ConflictCancel {
		return fmt.Errorf("알 수 없는 답: %d", choice)
	}
	select {
	case a.ch <- conflictAnswer{choice: choice, all: applyAll}:
	default:
	}
	return nil
}

// ── Go 입구 ──────────────────────────────────────────

// Transfers 는 전송 영역의 Go 입구다(Wails 에 묶지 않는다). c.Transfers() 로 얻는다.
type Transfers struct {
	c    *Core
	m    *transfer.Manager
	reqs chan downloadReq // 사람의 받기 요청(파일 중복 확인 창을 차례로 묻는다)

	mu        sync.Mutex
	owner     transfer.Owner
	ownerSeq  int64
	autoStop  context.CancelFunc // 검사·결재자 창 단계의 자동 업로드(사람이 밀어낸다)
	login     context.Context    // 로그인마다 새로, 로그아웃에 끊긴다
	logout    context.CancelFunc
	ask       *conflictAsk
	askSeq    int64
	askMu     sync.Mutex // 중복 확인 창은 하나다 ― 사람 받기와 결재 받기가 겹치면 묶음째 차례를 지킨다(mu 와 별개)
	autoIdx   int        // 자동 전송을 돌리는 망(Server.Index), -1 이면 없음
	autoUp    *time.Ticker
	autoUpEnd chan struct{}
	autoDn    *time.Timer
	autoPaths map[string]bool // 자동 업로드로 대기·진행 중인 PC 경로
	gotAuto   map[string]bool // 이 로그인에서 자동 다운로드로 넣은 server_str
	autoRetry bool            // 자동 다운로드가 실패해 다음 목록 응답에 다시 돈다
	refresh   *time.Timer
	fails     []string
	failT     *time.Timer
}

type conflictAsk struct {
	gen int64
	ch  chan conflictAnswer
}

type conflictAnswer struct {
	choice int
	all    bool
}

type downloadReq struct {
	s    *nbs.Session
	gen  uint64
	list []*serverfile.FileInfo
	dir  string
}

var transferSets sync.Map // *Core → *Transfers

// Transfers 는 전송 영역의 Go 입구다(클립보드·결재 담당이 부른다).
func (c *Core) Transfers() *Transfers {
	if v, ok := transferSets.Load(c); ok {
		return v.(*Transfers)
	}
	t := &Transfers{c: c, reqs: make(chan downloadReq, 64), autoIdx: -1}
	t.m = transfer.NewManager(5, 100<<20, transfer.Hooks{
		OnState:    func(it []transfer.Item) { c.Emit(EvTransferState, it) },
		OnTick:     func(tk []transfer.Tick) { c.Emit(EvTransferProgress, tk) },
		OnActivity: c.Touch, // 전송 진행도 사용 중으로 본다(AUTH-028)
	})
	if v, loaded := transferSets.LoadOrStore(c, t); loaded {
		return v.(*Transfers)
	}
	c.OnLogin(t.onLogin)
	c.OnLogout(t.onLogout)
	if serverFiles != nil {
		serverFiles.onChange(func(s *nbs.Session, _ []*serverfile.FileInfo) { t.scheduleAutoDownload(10 * time.Second) })
		serverFiles.onSame(t.retryAutoDownload)
	}
	go t.downloadLoop()
	return t
}

func (t *Transfers) log(cat string) *slog.Logger { return t.c.Log().With("category", cat) }

// autoLog 는 자동 전송이면 snb.transfer.auto, 아니면 cat 분류다.
func (t *Transfers) autoLog(auto bool, cat string) *slog.Logger {
	if auto {
		cat = applog.CatAuto
	}
	return t.log(cat)
}

func (t *Transfers) envInt(key string, def int64) int64 {
	if env, _ := t.c.Env(); env != nil {
		if v, ok := env.Value("CONFIG", key); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n >= 0 {
				return n
			}
		}
	}
	return def
}

// tempDir 는 전송 임시 자리다(폴더 zip·받은 폴더 풀기). 로그인마다 비운다.
func tempDir() string { return filepath.Join(config.DataDir(), "wails-temp") }

func (t *Transfers) loginCtx() context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.login == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return t.login
}

func (t *Transfers) onLogin(s *nbs.Session) {
	os.RemoveAll(tempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.login, t.logout = ctx, cancel
	t.gotAuto = map[string]bool{}
	t.autoPaths = map[string]bool{}
	t.owner = transfer.OwnerNone
	t.mu.Unlock()
	t.restartAuto(s)
}

func (t *Transfers) onLogout() {
	t.c.PolicyChecker().CloseCheck()
	t.mu.Lock()
	if t.logout != nil {
		t.logout()
	}
	t.login, t.logout = nil, nil
	if t.autoStop != nil {
		t.autoStop()
		t.autoStop = nil
	}
	t.owner = transfer.OwnerNone
	a := t.ask
	t.ask = nil
	t.stopAutoLocked()
	t.mu.Unlock()
	if a != nil {
		t.c.Emit(EvConflictClose, map[string]int64{"gen": a.gen})
	}
	if n := t.m.CancelAll(&transfer.Fail{Code: transfer.CodeLoggedOut}, nil); n > 0 {
		t.log(applog.CatUpload).Info(fmt.Sprintf("로그아웃으로 전송 정리 >>> %d 건", n))
	}
}

// ── 업로드 ──

// uploadAllowed 는 지금 망에서 업로드 권한이 있는가다(업무망 fm_file_up, 인터넷망 fm_file_down).
func uploadAllowed(s *nbs.Session) bool {
	if s.Info == nil {
		return false
	}
	if s.Server.Loc == config.LocBusiness {
		return s.Info.FmFileUp != 0
	}
	return s.Info.FmFileDown != 0
}

// UploadPaths 는 파일·폴더를 검사(파일 검사 결과 창)·결재자 창을 거쳐 대기열에 넣는다. auto 는 자동 업로드.
// 권한이 없거나 사람이 답을 기다리는 업로드가 있으면 곧바로 오류(자동은 nil 로 건너뛴다). 나머지는 뒤에서 돈다.
func (t *Transfers) UploadPaths(paths []string, auto bool) error {
	return t.uploadPaths(paths, auto, false)
}

func (t *Transfers) uploadPaths(paths []string, auto, announce bool) error {
	s, err := t.c.Session()
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	if !uploadAllowed(s) {
		if auto {
			t.log(applog.CatAuto).Info("[Auto]" + msgNoUploadPerm)
			return nil
		}
		return errors.New(msgNoUploadPerm)
	}
	ctx, cancel, token, err := t.claim(auto)
	if err != nil || ctx == nil {
		return err
	}
	gen := t.c.Gen()
	go func() {
		defer t.release(token, cancel)
		t.uploadPipeline(ctx, s, gen, token, paths, auto, announce)
	}()
	return nil
}

// claim 은 업로드 입구를 잡는다(원본 claimUploadList, UploadGate.h). ctx 가 nil 이면 받지 않았다(자동 건너뜀).
// 돌려준 ctx 는 검사·결재자 창 단계의 것이다 ― 대기열에 들어간 전송은 이것과 무관하다.
func (t *Transfers) claim(auto bool) (context.Context, context.CancelFunc, int64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.login == nil {
		return nil, nil, 0, ErrNotLoggedIn
	}
	switch transfer.AdmitUpload(t.owner, auto) {
	case transfer.SkipAuto:
		t.log(applog.CatAuto).Info("[Auto]자동 업로드 >>> 답을 기다리는 업로드가 있어 이번 주기를 건너뛴다")
		return nil, nil, 0, nil
	case transfer.RejectHuman:
		t.log(applog.CatUpload).Info("업로드 거절 >>> 답을 기다리는 업로드가 있다")
		return nil, nil, 0, ErrUploadBusy
	case transfer.PreemptAuto:
		t.log(applog.CatUpload).Info("사람의 업로드가 자동 업로드 검사를 밀어낸다")
		if t.autoStop != nil {
			t.autoStop()
		}
	}
	ctx, cancel := context.WithCancel(t.login)
	t.ownerSeq++
	t.owner = transfer.OwnerHuman
	t.autoStop = nil
	if auto {
		t.owner = transfer.OwnerAuto
		t.autoStop = cancel
	}
	return ctx, cancel, t.ownerSeq, nil
}

// release 는 입구를 놓는다(원본 releaseUploadList). 밀려난 자동 업로드가 늦게 놓아도 사람의 것을 풀지 않는다.
func (t *Transfers) release(token int64, cancel context.CancelFunc) {
	cancel()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ownerSeq == token {
		t.owner = transfer.OwnerNone
		t.autoStop = nil
	}
}

// handToHuman 은 검사를 지나 사람의 답을 기다리는 단계로 넘긴다(원본 ViewController.cpp:2404) ― 자동 업로드라도 이제 사람의 것이라 밀려나지 않는다.
func (t *Transfers) handToHuman(token int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ownerSeq == token {
		t.owner, t.autoStop = transfer.OwnerHuman, nil
	}
}

// uploadStartText 는 업로드 시작 로그 문구다(원본 형식 "%1 %2" + " 업로드 시작").
func uploadStartText(paths []string) string {
	more := ""
	if len(paths) > 1 {
		more = fmt.Sprintf("외 %d 건", len(paths)-1)
	}
	return fmt.Sprintf("%s %s 업로드 시작", paths[0], more)
}

func (t *Transfers) uploadPipeline(ctx context.Context, s *nbs.Session, gen uint64, token int64, paths []string, auto, announce bool) {
	log := t.autoLog(auto, applog.CatUpload)
	tag := ""
	if auto {
		tag = "[Auto]"
	}
	log.Info(uploadStartText(paths))
	// 폴더는 검사가 읽기 전에 내용을 적어 둔다 ― 압축 직전·직후에 대조한다(원본 097)
	base := map[string]map[string]transfer.Stamp{}
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			if snap, err := transfer.Snapshot(p); err == nil {
				base[filepath.Clean(p)] = snap
			}
		}
	}
	res, err := t.c.PolicyChecker().CheckUpload(ctx, s, CheckRequest{Paths: paths, Auto: auto})
	if err != nil {
		if ctx.Err() == nil {
			log.Warn(tag+"업로드 검사 실패", "err", err)
		}
		return
	}
	if len(res.Dropped) > 0 && !auto {
		t.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: "사용할 수 없는 파일명이 포함되어 있어 업로드에서 제외되었습니다.( __CLIP__ )"})
	}
	if !res.Proceed || len(res.Items) == 0 {
		return
	}
	pf := make([]approval.PickFile, len(res.Items))
	for i, it := range res.Items {
		pf[i] = approval.PickFile{Name: filepath.Base(it.Path), Size: it.Size}
	}
	t.handToHuman(token)
	pick, err := PickApprover(ctx, s, approval.PickRequest{Files: pf})
	if err != nil {
		t.pickFailed(err, auto)
		return
	}
	if !t.c.IsCurrent(gen) {
		return
	}
	queued := 0
	for _, it := range res.Items {
		req := UploadRequest{Path: it.Path, Pick: pick, ZipPassword: it.ZipPassword, Auto: auto}
		if it.IsDir {
			req.baseline = base[filepath.Clean(it.Path)]
			req.excludes = it.Excludes
		} else if !it.ModTime.IsZero() {
			req.stamp = transfer.Stamp{Size: it.Size, MtimeMs: it.ModTime.UnixMilli()}
		}
		req.size = it.Size
		if _, err := t.enqueueUpload(s, req); err != nil {
			log.Info(tag+"업로드 대기열에 넣지 못했다", "path", it.Path, "err", err)
			if !auto {
				t.reportFail(filepath.Base(it.Path), err)
			}
			continue
		}
		queued++
	}
	if queued > 0 {
		up, dn := t.counts()
		log.Info(fmt.Sprintf("BATCH 전송 시작 >>> 대기 업로드[%d] 다운로드[%d]", up, dn))
		if !auto {
			t.c.Emit(EvTransferOpen, nil)
		}
		if announce {
			t.c.Emit(EvUploadQueued, queued)
		}
	}
}

func (t *Transfers) counts() (up, dn int) {
	for _, it := range t.m.Items() {
		if it.State == "pending" || it.State == "active" {
			if it.Dir == "upload" {
				up++
			} else {
				dn++
			}
		}
	}
	return
}

func (t *Transfers) pickFailed(err error, auto bool) {
	log := t.autoLog(auto, applog.CatUpload)
	switch {
	case errors.Is(err, approval.ErrCanceled), errors.Is(err, context.Canceled):
		log.Info(approval.ErrCanceled.Error())
	case auto:
		log.Info("[Auto]결재자 정하기 실패", "err", err)
	case errors.Is(err, approval.ErrNoApprover):
		t.c.Emit(EvDialog, Dialog{ID: t.c.dialogSeq.Add(1), Title: "정보", Text: err.Error()})
	default:
		log.Warn("결재자 정하기 실패", "err", err)
		t.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: err.Error()})
	}
}

// UploadRequest 는 업로드 한 건이다(EnqueueUpload).
//   - Path: PC 경로(파일 또는 폴더). ServerName: 서버에 실을 이름(fpath, 비면 파일 이름·폴더는 <이름>.zip). 예: "__CLIP__.png"
//   - Display: 서랍에 보일 이름(비면 파일 이름, 클립보드는 "클립보드")
//   - Pick: 결재 칸. nil 이면 EnqueueUpload 가 PickApprover 로 정한다(창이 뜰 수 있다)
//   - SkipPolicy: 참이면 파일 검사(정책)를 건너뛴다(부른 쪽이 이미 검사했다 ― 클립보드). 거짓이면 검사 창을 거친다
//   - ZipPassword: 암호 압축 비밀번호(128자 넘으면 싣지 않는다). PkgNo: 묶음 번호(파일 업로드는 0)
//   - OnDone: 끝나면 한 번(nil = 성공, 취소면 transfer.ErrCanceled). 대기열 고루틴에서 부른다
type UploadRequest struct {
	Path        string
	ServerName  string
	Display     string
	Pick        *approval.PickResult
	SkipPolicy  bool
	ZipPassword string
	PkgNo       int32
	Auto        bool
	OnDone      func(error)

	size     int64
	stamp    transfer.Stamp
	baseline map[string]transfer.Stamp
	excludes []string
}

// EnqueueUpload 는 한 건을 대기열에 넣고 번호를 준다. SkipPolicy 가 거짓이거나 Pick 이 nil 이면 창의 답까지 기다리므로
// 화면 스레드가 아닌 고루틴에서 부른다. 같은 파일이 이미 대기·진행 중이면 오류.
func (t *Transfers) EnqueueUpload(s *nbs.Session, r UploadRequest) (int64, error) {
	if s == nil {
		return 0, ErrNotLoggedIn
	}
	st, err := os.Stat(r.Path)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", transfer.Text(transfer.CodeOpen, "", ""), err)
	}
	ctx := t.loginCtx()
	r.size = st.Size()
	if !r.SkipPolicy {
		res, err := t.c.PolicyChecker().CheckUpload(ctx, s, CheckRequest{Paths: []string{r.Path}, Auto: r.Auto})
		if err != nil {
			return 0, err
		}
		if !res.Proceed || len(res.Items) == 0 {
			return 0, approval.ErrCanceled
		}
		it := res.Items[0]
		r.size, r.excludes = it.Size, it.Excludes
		if r.ZipPassword == "" {
			r.ZipPassword = it.ZipPassword
		}
		if !it.IsDir && !it.ModTime.IsZero() {
			r.stamp = transfer.Stamp{Size: it.Size, MtimeMs: it.ModTime.UnixMilli()}
		}
	}
	if r.Pick == nil {
		clip := strings.Contains(r.ServerName, clipMarker)
		pick, err := PickApprover(ctx, s, approval.PickRequest{Files: []approval.PickFile{{Name: r.displayName(), Size: r.size}}, Clip: clip})
		if err != nil {
			return 0, err
		}
		r.Pick = pick
	}
	return t.enqueueUpload(s, r)
}

func (r *UploadRequest) displayName() string {
	switch {
	case r.Display != "":
		return r.Display
	case strings.Contains(r.ServerName, clipMarker):
		return clipDisplay
	}
	return filepath.Base(r.Path)
}

// uploadJob 은 한 건의 준비(압축·검사값)와 보내기다.
func (t *Transfers) enqueueUpload(s *nbs.Session, r UploadRequest) (int64, error) {
	st, err := os.Stat(r.Path)
	if err != nil {
		return 0, &transfer.Fail{Code: transfer.CodeOpen, Cause: err}
	}
	isDir := st.IsDir()
	if r.size == 0 && !isDir {
		r.size = st.Size()
	}
	name := r.displayName()
	serverName := r.ServerName
	if serverName == "" {
		serverName = filepath.Base(r.Path)
		if isDir {
			serverName += ".zip"
		}
	}
	checksum := s.Profile["fm_checksum"] != "0" // 키가 없으면 켠 것(원본 PolicySet::useChecksum)
	log := t.autoLog(r.Auto, applog.CatUpload)
	tag := ""
	if r.Auto {
		tag = "[Auto]"
	}
	var (
		sendPath = r.Path
		sum      string
		work     struct {
			sync.Mutex
			dir    string
			closed bool
		}
	)
	// 대기 중 취소면 Done 이 준비(압축)와 겹쳐 불린다 ― 임시 폴더는 잠금 안에서 넘겨받는다
	cleanup := func() {
		work.Lock()
		work.closed = true
		d := work.dir
		work.Unlock()
		if d != "" {
			os.RemoveAll(d)
		}
	}
	prepare := func(ctx context.Context, p *transfer.Progress) error {
		if isDir {
			if err := os.MkdirAll(tempDir(), 0o755); err != nil {
				return &transfer.Fail{Code: transfer.CodeZip, Cause: err}
			}
			wd, err := os.MkdirTemp(tempDir(), "up")
			if err != nil {
				return &transfer.Fail{Code: transfer.CodeZip, Cause: err}
			}
			work.Lock()
			closed := work.closed
			if !closed {
				work.dir = wd
			}
			work.Unlock()
			if closed {
				os.RemoveAll(wd)
				return transfer.ErrCanceled
			}
			if err := t.folderSame(r, "압축 직전"); err != nil {
				return err
			}
			before, _ := transfer.Snapshot(r.Path)
			zp := filepath.Join(wd, serverName)
			n, err := transfer.ZipFolder(ctx, r.Path, zp, r.excludes, p)
			if err != nil {
				if ctx.Err() != nil {
					return err
				}
				log.Warn("폴더를 압축하는데 오류가 발생했습니다.", "path", r.Path, "err", err)
				return &transfer.Fail{Code: transfer.CodeZip, Cause: err}
			}
			if after, _ := transfer.Snapshot(r.Path); before != nil {
				if d := transfer.DiffSnapshots(before, after); !d.Empty() {
					log.Info(fmt.Sprintf("업로드 취소 >>> 검사 뒤 내용이 달라졌다 >>> 폴더: %s %s", r.Path, d))
					return &transfer.Fail{Code: transfer.CodeFolderChanged, Arg: filepath.Base(r.Path)}
				}
			}
			log.Debug(fmt.Sprintf("폴더 압축 >>> %s 파일 %d 개, 제외 %d 개", r.Path, n, len(r.excludes)))
			sendPath = zp
		}
		if !checksum {
			log.Debug("검사값 생략(fm_checksum=0)", "path", r.Path)
			return nil
		}
		h, err := transfer.HashFile(ctx, sendPath, p)
		if err != nil {
			if ctx.Err() != nil {
				log.Info("업로드 취소됨 - 체크섬 계산 중에 정리됐다", "path", r.Path)
			}
			return err
		}
		sum = h
		return nil
	}
	run := func(ctx context.Context, p *transfer.Progress) error {
		pk := r.Pick
		if pk == nil {
			pk = &approval.PickResult{}
		}
		info := &serverfile.FileTransInfo{
			Fpath: serverName, ApproverId: pk.ApproverIDs, PkgNo: r.PkgNo, AprvKind: pk.AprvKind, AprvType: pk.AprvType,
			AprvDept: pk.AprvDept, Fcause: pk.Cause, Checksum: sum, Resv: pk.Resv,
		}
		if isDir {
			info.DirFlag = 1
		}
		if pw := r.ZipPassword; pw != "" {
			if utf8.RuneCountInString(pw) <= 128 {
				info.ZipPswd = pw
			} else {
				log.Warn("압축 비밀번호를 서버로 보내지 않았다 ― 서버 칸보다 길다", "path", r.Path)
			}
		}
		sctx, cancel := transfer.SessionCtx(ctx, s)
		defer cancel()
		spec := transfer.UploadSpec{Path: sendPath, Info: info, Stall: time.Duration(t.envInt("UPLOAD_STALL_SEC", 30)) * time.Second,
			Limit: files.SizeText(uint64(s.Info.GetFmFileSize()) * 1024)}
		if !isDir {
			spec.Stamp = r.stamp
		}
		err := transfer.Send(sctx, s.ServerFile(), spec, p)
		if transfer.CodeOf(err) == transfer.CodeChanged {
			log.Info(fmt.Sprintf("업로드 취소 >>> 검사 뒤 내용이 달라졌다 >>> 파일: %s", r.Path))
		}
		return err
	}
	done := func(res transfer.Result) {
		cleanup()
		t.unmarkAuto(r)
		switch {
		case res.Err == nil:
			log.Info(fmt.Sprintf("파일업로드 성공 >>> transIndex[%d] labTime[%dms] %s - %s", res.ID, res.Elapsed.Milliseconds(), name, speedText(res)))
			if r.Auto {
				log.Info(fmt.Sprintf("[Auto]자동업로드 성공 - %s", r.Path))
				if dir := t.autoUploadFolder(); dir != "" && strings.EqualFold(filepath.Clean(filepath.Dir(r.Path)), filepath.Clean(dir)) {
					if dst, err := transfer.MoveToDone(r.Path, dir); err != nil {
						log.Warn("[Auto]완료 폴더로 옮기지 못했다", "path", r.Path, "err", err)
					} else {
						log.Debug("[Auto]완료 폴더로 옮김", "to", dst)
					}
				}
			}
			t.refreshSoon()
		case transfer.IsCanceled(res.Err):
			log.Info(tag+"업로드 취소", "name", name)
		default:
			log.Warn(fmt.Sprintf("%s업로드 실패 - %s >>> %s", tag, res.Err, name), "code", transfer.CodeOf(res.Err), "cause", errors.Unwrap(res.Err))
			if r.Auto {
				log.Info(fmt.Sprintf("[Auto]자동업로드 실패 - %s", r.Path))
			}
			if !quietFail(res.Err, r.Auto) {
				t.reportFail(name, res.Err)
			}
			t.refreshSoon()
		}
		if r.OnDone != nil {
			r.OnDone(res.Err)
		}
	}
	key := ""
	if !strings.Contains(serverName, clipMarker) {
		key = fmt.Sprintf("up|%s|%d|%d", strings.ToLower(filepath.Clean(r.Path)), st.Size(), st.ModTime().UnixMilli())
	}
	t.markAuto(r)
	id, err := t.m.Add(transfer.Job{Dir: transfer.Up, Name: name, Size: r.size, Key: key, Auto: r.Auto,
		Prepare: prepare, Run: run, Done: done})
	if err != nil {
		t.unmarkAuto(r)
		return 0, err
	}
	return id, nil
}

// folderSame 은 폴더가 검사 때 그대로인가다(검사에서 빠지는 파일·숨김은 보지 않는다).
func (t *Transfers) folderSame(r UploadRequest, when string) error {
	if r.baseline == nil {
		return nil
	}
	now, err := transfer.Snapshot(r.Path)
	if err != nil {
		return &transfer.Fail{Code: transfer.CodeOpen, Cause: err}
	}
	var ignore []string
	for _, e := range r.excludes {
		if rel, err := filepath.Rel(r.Path, e); err == nil {
			ignore = append(ignore, filepath.ToSlash(rel))
		}
	}
	if d := transfer.DiffSnapshots(r.baseline, now, ignore...); !d.Empty() {
		t.log(applog.CatUpload).Info(fmt.Sprintf("업로드 취소 >>> 검사 뒤 내용이 달라졌다(%s) >>> 폴더: %s %s", when, r.Path, d))
		return &transfer.Fail{Code: transfer.CodeFolderChanged, Arg: filepath.Base(r.Path)}
	}
	return nil
}

func (t *Transfers) markAuto(r UploadRequest) {
	if r.Auto {
		t.mu.Lock()
		if t.autoPaths != nil {
			t.autoPaths[filepath.Clean(r.Path)] = true
		}
		t.mu.Unlock()
	}
}

func (t *Transfers) unmarkAuto(r UploadRequest) {
	if r.Auto {
		t.mu.Lock()
		delete(t.autoPaths, filepath.Clean(r.Path))
		t.mu.Unlock()
	}
}

func speedText(r transfer.Result) string {
	sec := r.Elapsed.Seconds()
	if sec <= 0 {
		sec = 0.001
	}
	return files.SizeText(uint64(float64(r.Bytes)/sec)) + "/s"
}

// quietFail 은 창 없이 로그만 남기는 실패인가다(원본 ViewControllerUiSlot.cpp:332-339, ViewController.cpp:2560-2575): 로그아웃 뒤의 결과,
// 자동 업로드의 검사 뒤 바뀐 파일·폴더와 폴더 압축 실패(폴더에 남아 다음 주기에 다시 올라간다). 나머지 자동 전송 실패는 창을 띄운다.
func quietFail(err error, autoUpload bool) bool {
	code := transfer.CodeOf(err)
	return code == transfer.CodeLoggedOut ||
		autoUpload && (code == transfer.CodeChanged || code == transfer.CodeFolderChanged || code == transfer.CodeZip)
}

// reportFail 은 전송 실패를 「파일 송수신 실패」 창 하나로 모아 알린다(0.5초 안의 실패는 한 창).
func (t *Transfers) reportFail(name string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fails = append(t.fails, name+" : "+err.Error())
	if t.failT != nil {
		return
	}
	t.failT = time.AfterFunc(500*time.Millisecond, func() {
		t.mu.Lock()
		list := t.fails
		t.fails, t.failT = nil, nil
		t.mu.Unlock()
		if len(list) > 20 {
			list = append(list[:20], fmt.Sprintf("… 외 %d건", len(list)-20))
		}
		t.c.Emit(EvDialog, Dialog{ID: t.c.dialogSeq.Add(1), Title: titleTransferFail, Text: strings.Join(list, "\n")})
	})
}

// refreshSoon 은 전송이 끝나면 서버파일 목록을 다시 받게 한다(0.5초 안의 끝남은 한 번으로 합친다).
func (t *Transfers) refreshSoon() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.refresh != nil {
		t.refresh.Reset(500 * time.Millisecond)
		return
	}
	t.refresh = time.AfterFunc(500*time.Millisecond, func() {
		t.mu.Lock()
		t.refresh = nil
		t.mu.Unlock()
		if serverFiles != nil {
			serverFiles.refresh()
		}
	})
}

// ── 받기 ──

// DownloadTo 는 서버 파일들을 dir 로 받는다. 받을 수 없는 것(클립보드·동일망·검사 중·상태 아님)은 빼고 사유를 알린다.
// 같은 이름은 파일 중복 확인 창으로 차례로 묻고, 답을 기다리는 동안 온 요청은 그 뒤에 묻는다.
func (t *Transfers) DownloadTo(serverNames []string, dir string) error {
	s, err := t.c.Session()
	if err != nil {
		return err
	}
	if len(serverNames) == 0 {
		return errors.New(msgSelectDownload)
	}
	if strings.TrimSpace(dir) == "" {
		return errors.New(msgNoDownloadDir)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("받을 폴더가 없습니다: %s", dir)
	}
	var ok []*serverfile.FileInfo
	var blocked []string
	for _, n := range serverNames {
		f := serverFiles.find(n)
		if f == nil {
			blocked = append(blocked, n+" 서버 파일 목록에 없습니다.")
			continue
		}
		if b := files.Classify(f, s.Server.Loc, s.Profile); b != files.BlockNone {
			blocked = append(blocked, files.DisplayName(f)+" "+b.Text())
			continue
		}
		ok = append(ok, f)
	}
	switch {
	case len(blocked) == 1 && len(ok) == 0:
		t.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: blocked[0]})
	case len(blocked) > 0:
		t.c.Emit(EvDialog, Dialog{ID: t.c.dialogSeq.Add(1), Title: "정보", Text: strings.Join(blocked, "\n")})
	}
	if len(ok) == 0 {
		return nil
	}
	select {
	case t.reqs <- downloadReq{s: s, gen: t.c.Gen(), list: ok, dir: dir}:
	default:
		return ErrUploadBusy
	}
	return nil
}

func (t *Transfers) downloadLoop() {
	for req := range t.reqs {
		if t.c.IsCurrent(req.gen) {
			t.processDownload(req)
		}
	}
}

func (t *Transfers) processDownload(req downloadReq) {
	log := t.log(applog.CatDownload)
	cands := make([]files.Candidate, len(req.list))
	for i, f := range req.list {
		cands[i] = files.Candidate{Dir: req.dir, Name: files.DisplayName(f)}
	}
	over, skip, ok := t.askConflicts(cands, func(i int) bool { return req.list[i].DirFlag == 1 })
	if !ok || !t.c.IsCurrent(req.gen) {
		return
	}
	remove := files.RemoveAfterDownload(req.s.Profile)
	queued := 0
	for i, f := range req.list {
		if skip[i] {
			continue
		}
		dest, err := files.SavePath(req.dir, cands[i].Name)
		if err != nil {
			t.reportFail(cands[i].Name, err)
			continue
		}
		if _, err := t.enqueueDownload(req.s, DownloadRequest{ServerName: f.ServerStr, Dest: dest, Size: int64(f.FileSize),
			Folder: f.DirFlag == 1, Overwrite: over[i], Remove: remove, Display: cands[i].Name}, false); err != nil {
			log.Info("받기 대기열에 넣지 못했다", "name", cands[i].Name, "err", err)
			t.reportFail(cands[i].Name, err)
			continue
		}
		queued++
	}
	if queued > 0 {
		up, dn := t.counts()
		log.Info(fmt.Sprintf("BATCH 전송 시작 >>> 대기 업로드[%d] 다운로드[%d]", up, dn))
		t.c.Emit(EvTransferOpen, nil)
	}
}

// askConflicts 는 받을 자리에 이미 있는 이름을 파일 중복 확인 창으로 차례로 묻는다.
// over 는 덮어쓸 색인, skip 은 받지 않을 색인이다. 로그아웃이면 ok 거짓.
func (t *Transfers) askConflicts(cands []files.Candidate, folder func(int) bool) (over, skip map[int]bool, ok bool) {
	t.askMu.Lock()
	defer t.askMu.Unlock()
	over, skip = map[int]bool{}, map[int]bool{}
	asking := files.Duplicates(cands, nil)
	all := -1
	for qi, i := range asking {
		choice := all
		if choice < 0 {
			p, _ := files.SavePath(cands[i].Dir, cands[i].Name)
			ans, ok := t.askConflict(ConflictAsk{Name: cands[i].Name, Path: p, Folder: folder(i), Index: qi + 1, Total: len(asking)})
			if !ok {
				return nil, nil, false
			}
			choice = ans.choice
			if ans.all {
				all = choice
			}
		}
		switch choice {
		case ConflictOverwrite:
			over[i] = true
		case ConflictCancel:
			skip[i] = true
			t.log(applog.CatDownload).Info("받기를 취소했습니다 - " + cands[i].Name)
		}
	}
	return over, skip, true
}

// askConflict 는 파일 중복 확인 창을 띄우고 답을 기다린다. 로그아웃이면 거짓.
func (t *Transfers) askConflict(q ConflictAsk) (conflictAnswer, bool) {
	ctx := t.loginCtx()
	t.mu.Lock()
	t.askSeq++
	a := &conflictAsk{gen: t.askSeq, ch: make(chan conflictAnswer, 1)}
	t.ask = a
	t.mu.Unlock()
	q.Gen = a.gen
	t.c.Emit(EvDownloadConflict, q)
	defer func() {
		t.mu.Lock()
		if t.ask == a {
			t.ask = nil
		}
		t.mu.Unlock()
	}()
	select {
	case ans := <-a.ch:
		return ans, true
	case <-ctx.Done():
		return conflictAnswer{}, false
	}
}

// DownloadRequest 는 받기 한 건이다(EnqueueDownload).
//   - ServerName: 서버 이름(spath). Fpath: 클립보드 받기면 "__CLIP__"
//   - Dest: 저장 경로(폴더면 풀 폴더, 클립보드면 확장자 없는 경로 ― 첫 응답의 code 1 글·2 그림·3 묶음(.fmh), 없으면 서버 이름으로 붙는다)
//   - Overwrite: 거짓이면 받기 직전에 겹침을 다시 보고 번호(_00000)를 붙인다
//   - Remove: 받은 뒤 서버에서 지운다(RemoveFile flag 1, 클립보드는 첫 응답의 서버 경로 + fpath "__CLIP__")
//   - OnDone: 끝나면 한 번. 대기열 고루틴에서 부른다
type DownloadRequest struct {
	ServerName string
	Fpath      string
	Dest       string
	Size       int64
	Folder     bool
	Clip       bool
	Overwrite  bool
	Remove     bool
	Display    string
	Auto       bool
	OnDone     func(*transfer.DownloadResult, error)
}

// EnqueueDownload 는 받기 한 건을 대기열에 넣고 번호를 준다. 같은 서버 파일이 이미 대기·진행 중이면 오류.
func (t *Transfers) EnqueueDownload(s *nbs.Session, r DownloadRequest) (int64, error) {
	if s == nil {
		return 0, ErrNotLoggedIn
	}
	return t.enqueueDownload(s, r, true)
}

func (t *Transfers) enqueueDownload(s *nbs.Session, r DownloadRequest, external bool) (int64, error) {
	log := t.autoLog(r.Auto, applog.CatDownload)
	name := r.Display
	if name == "" {
		name = filepath.Base(r.Dest)
		if r.Clip {
			name = clipDisplay
		}
	}
	tag := ""
	if r.Auto {
		tag = "[Auto]"
	}
	run := func(ctx context.Context, p *transfer.Progress) (*transfer.DownloadResult, error) {
		sctx, cancel := transfer.SessionCtx(ctx, s)
		defer cancel()
		return transfer.Receive(sctx, s.ServerFile(), transfer.DownloadSpec{
			ServerName: r.ServerName, Fpath: r.Fpath, Dest: r.Dest, Size: r.Size, Folder: r.Folder, Clip: r.Clip,
			Overwrite: r.Overwrite, TempDir: tempDir(), Biz: s.IsBusiness(), Limit: files.SizeText(uint64(s.Info.GetFmFileSize()) * 1024),
		}, p)
	}
	var result *transfer.DownloadResult
	done := func(res transfer.Result) {
		switch {
		case res.Err == nil:
			log.Info(fmt.Sprintf("%s%s 파일 송수신 완료 - %s", tag, name, speedText(res)), "path", result.Path)
			if r.Overwrite && !r.Clip && result.Path != r.Dest {
				log.Warn("덮어쓰지 못해 다른 이름으로 저장했다", "dest", r.Dest, "path", result.Path)
				t.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: "덮어쓸 수 없어 다른 이름으로 저장했습니다: " + filepath.Base(result.Path)})
			}
			if r.Remove {
				if err := t.removeAfter(s, r, result); err != nil {
					log.Warn("받은 뒤 서버 목록에서 지우지 못했다", "server", r.ServerName, "err", err)
				}
			}
			if !r.Clip {
				t.refreshSoon()
			}
		case transfer.IsCanceled(res.Err):
			log.Info("다운로드 취소 - 받던 파일 삭제", "name", name)
		default:
			log.Warn(fmt.Sprintf("%s다운로드 실패 - %s >>> %s", tag, res.Err, name), "code", transfer.CodeOf(res.Err), "cause", errors.Unwrap(res.Err))
			if !external && !quietFail(res.Err, false) {
				t.reportFail(name, res.Err)
			}
		}
		if r.OnDone != nil {
			r.OnDone(result, res.Err)
		}
	}
	key := "down|" + r.ServerName
	if r.Clip && r.ServerName == "" {
		key = ""
	}
	return t.m.Add(transfer.Job{Dir: transfer.Down, Name: name, Size: r.Size, Key: key, Auto: r.Auto,
		Run: func(ctx context.Context, p *transfer.Progress) error {
			res, err := run(ctx, p)
			result = res
			return err
		}, Done: done})
}

// removeAfter 는 받은 뒤 서버 원본을 지운다(원본 FileTransfer: 클립보드는 첫 응답의 서버 경로·fpath __CLIP__).
func (t *Transfers) removeAfter(s *nbs.Session, r DownloadRequest, res *transfer.DownloadResult) error {
	if !r.Clip {
		return files.Remove(s, r.ServerName, true)
	}
	spath := r.ServerName
	if res != nil && res.ServerPath != "" {
		spath = res.ServerPath
	}
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	_, err := s.ServerFile().RemoveFile(ctx, &serverfile.FileRemoveRequest{Spath: spath, Flag: 1, Fpath: clipMarker})
	return err
}

// downloadApproval 은 결재 대상 받기를 대기열로 돌리고 모두 끝날 때까지 기다린다(서버에서 지우지 않는다, APR-061).
func (t *Transfers) downloadApproval(s *nbs.Session, reqs []DownloadRequest) (saved, fails []string) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, r := range reqs {
		name := r.Display
		r.Remove = false
		r.OnDone = func(res *transfer.DownloadResult, err error) {
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails = append(fails, name+": "+err.Error())
			} else {
				saved = append(saved, res.Path)
			}
			wg.Done()
		}
		wg.Add(1)
		if _, err := t.enqueueDownload(s, r, true); err != nil {
			mu.Lock()
			fails = append(fails, name+": "+err.Error())
			mu.Unlock()
			wg.Done()
		}
	}
	if len(reqs) > 0 {
		t.c.Emit(EvTransferOpen, nil)
	}
	wg.Wait()
	return saved, fails
}

// ── 창에 놓기 ──

// dropTargetID 는 서버파일 표의 놓기 자리(data-file-drop-target 이 붙은 요소의 id)다.
const dropTargetID = "serverFileDrop"

func (t *Transfers) watchDrop(w application.Window) {
	w.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		ctx := e.Context()
		paths := ctx.DroppedFiles()
		target := ""
		if d := ctx.DropTargetDetails(); d != nil {
			target = d.ElementID
		}
		if target != dropTargetID || len(paths) == 0 {
			return
		}
		t.log(applog.CatUpload).Info("끌어 놓기 업로드", "count", len(paths))
		if err := t.UploadPaths(paths, false); err != nil {
			t.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: err.Error()})
		}
	})
}

// ── 자동 전송 ──

func (t *Transfers) autoSettings(idx int) config.AutoTransfer {
	if st := t.c.Settings(); st != nil {
		return st.Get().Auto[idx]
	}
	return config.AutoTransfer{}
}

func (t *Transfers) autoUploadFolder() string {
	t.mu.Lock()
	idx := t.autoIdx
	t.mu.Unlock()
	if idx < 0 {
		return ""
	}
	return t.autoSettings(idx).UploadFolder
}

func (t *Transfers) stopAutoLocked() {
	if t.autoUpEnd != nil {
		close(t.autoUpEnd)
		t.autoUpEnd = nil
	}
	if t.autoUp != nil {
		t.autoUp.Stop()
		t.autoUp = nil
	}
	if t.autoDn != nil {
		t.autoDn.Stop()
		t.autoDn = nil
	}
	t.autoIdx = -1
}

// restartAuto 는 이 세션 망의 자동 업로드 주기를 (다시) 켜고 자동 다운로드를 예약한다.
func (t *Transfers) restartAuto(s *nbs.Session) {
	idx := s.Server.Index
	cfg := t.autoSettings(idx)
	t.mu.Lock()
	t.stopAutoLocked()
	t.autoIdx = idx
	if cfg.UploadUse {
		period := cfg.UploadPeriod
		if period <= 0 {
			period = config.DefaultUploadPeriod
		}
		tk := time.NewTicker(time.Duration(period) * time.Minute)
		end := make(chan struct{})
		t.autoUp, t.autoUpEnd = tk, end
		gen := t.c.Gen()
		go func() {
			for {
				select {
				case <-end:
					return
				case <-tk.C:
					if t.c.IsCurrent(gen) {
						t.AutoUploadOnce()
					}
				}
			}
		}()
		t.log(applog.CatAuto).Info(fmt.Sprintf("[Auto]자동 업로드 시작 >>> autoUploadFolder_%d 주기 %d분: %s", idx, period, cfg.UploadFolder))
	}
	t.mu.Unlock()
	if cfg.DownloadUse {
		t.scheduleAutoDownload(30 * time.Second)
	}
}

// settingsChanged 는 지금 망의 자동 전송 설정이 바뀌었을 때만 그 망의 시계를 다시 건다.
func (t *Transfers) settingsChanged(old, cur config.Settings) {
	s, err := t.c.Session()
	if err != nil {
		return
	}
	idx := s.Server.Index
	if old.Auto[idx] == cur.Auto[idx] {
		return
	}
	t.log(applog.CatAuto).Info(fmt.Sprintf("[Auto]자동 전송 설정 변경 >>> 망[%d] 시계를 다시 건다", idx))
	t.restartAuto(s)
}

// AutoUploadOnce 는 자동 업로드 한 주기다(원본 checkAutoUploadFiles). 시험·설정 화면의 [지금 실행]이 부를 수 있다.
func (t *Transfers) AutoUploadOnce() {
	s, err := t.c.Session()
	if err != nil {
		return
	}
	cfg := t.autoSettings(s.Server.Index)
	if !cfg.UploadUse {
		return
	}
	dir := cfg.UploadFolder
	log := t.log(applog.CatAuto)
	if p := transfer.FolderProblem(dir); p != "" {
		log.Info(fmt.Sprintf("[Auto]자동 업로드 >>> 올리지 않는다(autoUploadFolder_%d): %s", s.Server.Index, p))
		return
	}
	done := filepath.Join(dir, transfer.DoneDir)
	if !files.Exists(done) { // 만든 주기에는 올리지 않는다(원본 checkAutoUploadFiles)
		if err := os.MkdirAll(done, 0o755); err != nil {
			t.c.Emit(EvDialog, Dialog{ID: t.c.dialogSeq.Add(1), Title: "정보", Text: "자동업로드 완료된 파일이 이동할 폴더를 생성하지 못했습니다. -" + done})
		}
		return
	}
	list, err := transfer.AutoCandidates(dir)
	if err != nil {
		log.Warn("[Auto]자동 업로드 폴더를 읽지 못했다", "err", err)
		return
	}
	t.mu.Lock()
	var paths []string
	for _, p := range list {
		if !t.autoPaths[filepath.Clean(p)] {
			paths = append(paths, p)
		}
	}
	t.mu.Unlock()
	if len(paths) == 0 {
		return
	}
	log.Info(fmt.Sprintf("[Auto]자동 업로드 수행 : %s (%d 건)", dir, len(paths)))
	if err := t.UploadPaths(paths, true); err != nil {
		log.Info("[Auto]자동 업로드 시작 못함", "err", err)
	}
}

// scheduleAutoDownload 는 d 뒤에 자동 다운로드를 한 번 돈다(다시 부르면 시계를 다시 건다 ― 겹쳐 돌지 않는다).
func (t *Transfers) scheduleAutoDownload(d time.Duration) {
	s, err := t.c.Session()
	if err != nil || !t.autoSettings(s.Server.Index).DownloadUse {
		return
	}
	gen := t.c.Gen()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.autoDn != nil {
		t.autoDn.Stop()
	}
	t.autoDn = time.AfterFunc(d, func() {
		if t.c.IsCurrent(gen) {
			t.AutoDownloadOnce()
		}
	})
}

// retryAutoDownload 는 지난 자동 다운로드가 실패했으면 목록이 그대로여도 한 번 다시 돈다(원본은 목록 응답마다 예약한다).
// 표시를 지우고 걸므로 조회가 잦아도 시계가 밀리지 않는다.
func (t *Transfers) retryAutoDownload() {
	t.mu.Lock()
	retry := t.autoRetry
	t.autoRetry = false
	t.mu.Unlock()
	if retry {
		t.scheduleAutoDownload(10 * time.Second)
	}
}

func (t *Transfers) markAutoRetry() {
	t.mu.Lock()
	t.autoRetry = true
	t.mu.Unlock()
}

// AutoDownloadOnce 는 자동 다운로드 한 번이다(원본 autoDownloadFiles, docs/port/04 7.5).
func (t *Transfers) AutoDownloadOnce() {
	s, err := t.c.Session()
	if err != nil {
		return
	}
	cfg := t.autoSettings(s.Server.Index)
	if !cfg.DownloadUse {
		return
	}
	log := t.log(applog.CatAuto)
	pol := files.AutoPolicyOf(s.Profile)
	if !pol.AutoTransAll && !pol.PrintRelayUsed {
		log.Info(fmt.Sprintf("자동 다운로드 불가 >>> autotrans_step: %s print_relay: %s", s.Profile["autotrans_step"], s.Profile["print_relay"]))
		return
	}
	pol.UserFolder = cfg.DownloadFolder
	if pol.UserFolder != "" {
		if err := files.UsableFolder(pol.UserFolder); err != nil {
			log.Info(fmt.Sprintf("자동다운로드 >>> 사용자 폴더로는 받지 않는다(autoDownloadFolder_%d): %v", s.Server.Index, err))
		} else {
			pol.UserFolderUsable = true
		}
	}
	pol.PrintFolder = filepath.Join(config.DataDir(), "print_Download")
	remove := files.RemoveAfterDownload(s.Profile)
	made := map[string]bool{}
	n := 0
	for _, f := range serverFiles.snapshot() {
		if files.IsClip(f) || files.Classify(f, s.Server.Loc, s.Profile) != files.BlockNone {
			continue
		}
		t.mu.Lock()
		seen := t.gotAuto == nil || t.gotAuto[f.ServerStr]
		t.mu.Unlock()
		if seen {
			continue
		}
		folder, skip := files.AutoTarget(f.AutoDown, pol)
		if skip != files.AutoNone {
			log.Debug("자동다운로드 건너뜀", "name", f.NameStr, "why", string(skip))
			continue
		}
		if f.AutoDown == 1 || f.AutoDown == 3 {
			if !made[folder] {
				if err := os.MkdirAll(folder, 0o755); err != nil {
					t.c.Emit(EvDialog, Dialog{ID: t.c.dialogSeq.Add(1), Title: "정보", Text: "자동다운로드폴더 생성에 실패했습니다. -" + folder})
					t.markAutoRetry()
					return
				}
				made[folder] = true
			}
		}
		if err := files.CheckSpace(folder, f.FileSize); err != nil {
			log.Warn("자동다운로드 >>> " + err.Error())
			t.markAutoRetry()
			break
		}
		dest, err := files.SavePath(folder, files.DisplayName(f))
		if err != nil {
			continue
		}
		// 실패·취소하면 표시를 지워 다음 번에 다시 받는다. 표시를 먼저 해야 금방 끝난 실패의 지우기가 표시에 덮이지 않는다
		server := f.ServerStr
		unmark := func() {
			t.mu.Lock()
			delete(t.gotAuto, server)
			t.mu.Unlock()
			t.markAutoRetry()
		}
		t.mu.Lock()
		if t.gotAuto != nil {
			t.gotAuto[server] = true
		}
		t.mu.Unlock()
		if _, err := t.enqueueDownload(s, DownloadRequest{ServerName: server, Dest: dest, Size: int64(f.FileSize),
			Folder: f.DirFlag == 1, Remove: remove, Display: files.DisplayName(f), Auto: true,
			OnDone: func(_ *transfer.DownloadResult, err error) {
				if err != nil {
					unmark()
				}
			}}, false); err != nil {
			unmark()
			continue
		}
		n++
	}
	if n > 0 {
		log.Info(fmt.Sprintf("[Auto]자동 다운로드 >>> %d 건을 대기열에 넣었다", n))
	}
}
