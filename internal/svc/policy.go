package svc

// 정책 영역: 정책 받기(프로파일·확장자 허용 목록[3005]·헤더 패턴[3015]), 업로드 파일 검사와 [파일 검사 결과]·[압축 비밀번호]
// 창의 답, 결재자 받기 검사, 정책 조회 화면(docs/port/02). 판정은 internal/policy 가 하고 여기서는 세션·창·세대를 잇는다.
//
// 화면이 부르는 메서드는 PolicyService(Wails 에 묶임)에, 전송 담당이 Go 에서 부르는 입구는 PolicyChecker(묶지 않음 ―
// *nbs.Session·context 를 받으므로)에 둔다. PolicyChecker 는 c.PolicyChecker() 로 얻는다.

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"snbapp/internal/applog"
	"snbapp/internal/nbs"
	"snbapp/internal/policy"
	"snbapp/pb/commonservice"
)

// 화면에 내는 검사 창 값. 이름은 events.go 의 EvFileCheck*·EvZipPasswordAsk 다.
const (
	CheckModeUpload   = "UPLOAD"
	CheckModeDownload = "DOWNLOAD"
)

// FileCheckOpen 은 EvFileCheckOpen 의 값이다 ― 창을 비우고 "검사가 진행중입니다." 로 연다. 답(FileCheckReply)에 Gen 을 싣는다.
type FileCheckOpen struct {
	Gen   int64  `json:"gen"`
	Mode  string `json:"mode"`  // UPLOAD · DOWNLOAD
	Total int    `json:"total"` // 검사할 파일 수(폴더 안 파일 포함)
}

// FileCheckRows 는 EvFileCheckRow 의 값이다. 120ms 마다 모아 보낸다. 같은 Key 의 행은 바꾼다.
type FileCheckRows struct {
	Gen     int64        `json:"gen"`
	Rows    []policy.Row `json:"rows"`
	Done    int          `json:"done"`
	Total   int          `json:"total"`
	Current string       `json:"current"` // 지금 검사한 파일 이름
}

// FileCheckResult 는 EvFileCheckResult 의 값이다(원본 FILE_CHECK_RESULT 의 ret·value).
type FileCheckResult struct {
	Gen   int64  `json:"gen"`
	Fails int    `json:"fails"`
	Value string `json:"value"` // "AUTO" · "SKIP:n" · "DOWNLOAD" · ""
	Skip  int    `json:"skip"`  // 폴더에서 빼고 올릴 파일 수(Value 의 n)
}

// FileCheckClose 는 EvFileCheckClose 의 값이다 ― 이 검사의 창을 답 없이 닫는다(밀려남·로그아웃).
type FileCheckClose struct {
	Gen int64 `json:"gen"`
}

// ZipPasswordAsk 는 EvZipPasswordAsk 의 값이다. Name 이 비면 떠 있는 창을 닫는다.
type ZipPasswordAsk struct {
	Gen   int64  `json:"gen"`
	Name  string `json:"name"`
	Retry bool   `json:"retry"` // 앞 비밀번호가 틀렸다(「비밀번호가 틀렸습니다. 다시 입력하세요.」)
}

// PolicyRow 는 정책 화면의 한 줄이다.
type PolicyRow struct {
	Type        string `json:"type"` // "Profile" · "LinkRedirect" · "Extension" · "ExtProfile"
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// PolicyStatus 는 지금 정책 한 벌의 요약이다(정책 화면 머리, EvPolicyChanged 의 값).
type PolicyStatus struct {
	Ready      bool   `json:"ready"` // 프로파일·확장자 목록·헤더 패턴을 다 받았다
	Method     string `json:"method"`
	FileSizeKB int64  `json:"fileSizeKB"` // 0 = 제한 없음
	FileCount  int    `json:"fileCount"`
	ClipSizeKB int64  `json:"clipSizeKB"`
	Profile    int    `json:"profile"`
	ExtRows    int    `json:"extRows"`
	PattResp   int    `json:"pattResp"`
	PattStored int    `json:"pattStored"`
	InsideZip  bool   `json:"insideZip"`
	ZipDepth   int    `json:"zipDepth"`
}

// policyWait 는 검사를 시작할 때 아직 정책을 다 받지 못했으면 기다리는 상한이다(로그인 직후의 업로드).
const policyWait = 5 * time.Second

// autoProceedFallback 은 "모두 정상" 결과 뒤 화면의 2초 자동 진행이 오지 않을 때 Go 가 스스로 진행하는 시간이다.
const autoProceedFallback = 3 * time.Second

// PolicyService 는 파일 검사 결과 창·압축 비밀번호 창·정책 화면이 부른다.
type PolicyService struct {
	base
	mu    sync.Mutex
	set   *policy.Set   // 지금 정책 한 벌. 로그인 전·로그아웃 뒤 nil
	ready chan struct{} // 세 가지를 다 받으면 닫힌다(로그인마다 새로)
	run   *checkRun     // 답을 기다리는 업로드 검사(하나뿐)
	seq   atomic.Int64  // 검사 번호
}

type pwAnswer struct {
	pw string
	ok bool
}

// checkRun 은 업로드 검사 하나다.
type checkRun struct {
	gen    int64
	cancel context.CancelFunc
	reply  chan bool     // 창의 답(참 = 취소)
	pw     chan pwAnswer // 비밀번호 창의 답
	done   chan struct{} // 검사가 끝나 답까지 받았다
	result bool          // 결과를 창에 냈다(그 전의 [확인] 은 버린다)
	asking bool
}

var policyServices sync.Map // *Core → *PolicyService

func newPolicyService(c *Core) *PolicyService {
	if v, ok := policyServices.Load(c); ok {
		return v.(*PolicyService)
	}
	p := &PolicyService{base: base{c}}
	if v, loaded := policyServices.LoadOrStore(c, p); loaded {
		return v.(*PolicyService)
	}
	c.OnLogin(p.onLogin)
	c.OnLogout(p.onLogout)
	return p
}

func (p *PolicyService) log() *slog.Logger { return p.c.Log().With("category", applog.CatCheck) }

func (p *PolicyService) onLogin(s *nbs.Session) {
	set := policy.NewSet(s.Server.Loc, int64(s.Info.FmFileSize), int(s.Info.FmFileCount), int64(s.Info.FmClipSize)).WithProfile(s.Profile)
	p.mu.Lock()
	p.set = set
	p.ready = make(chan struct{})
	p.mu.Unlock()
	gen := p.c.Gen()
	go func() { _ = p.load(s, gen, false) }()
}

func (p *PolicyService) onLogout() {
	p.mu.Lock()
	r := p.run
	p.run, p.set = nil, nil
	p.mu.Unlock()
	if r != nil {
		r.cancel()
		p.c.Emit(EvZipPasswordAsk, ZipPasswordAsk{Gen: r.gen})
		p.c.Emit(EvFileCheckClose, FileCheckClose{Gen: r.gen})
	}
}

// current 는 지금 정책 한 벌이다. 검사 하나는 시작 때 잡은 한 벌로 끝까지 판정한다.
func (p *PolicyService) current() *policy.Set {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.set
}

// update 는 같은 로그인 세대일 때만 정책 한 벌을 바꾼다. 세 가지를 다 받았으면 ready 를 닫는다.
func (p *PolicyService) update(gen uint64, fn func(*policy.Set) *policy.Set) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.set == nil || !p.c.IsCurrent(gen) {
		return false
	}
	p.set = fn(p.set)
	if p.set.Ready() {
		select {
		case <-p.ready:
		default:
			close(p.ready)
		}
	}
	return true
}

// load 는 확장자 허용 목록·헤더 패턴(withProfile 이면 프로파일도)을 받아 망의 정책을 통째로 바꾼다.
// 응답이 실패한 부분은 앞 값을 그대로 둔다(받지 못한 채면 그 검사가 막힌다).
func (p *PolicyService) load(s *nbs.Session, gen uint64, withProfile bool) error {
	log := p.c.Log().With("category", applog.CatGrpc)
	var errs []string
	if withProfile {
		ctx, cancel := s.Ctx(nbs.RPCTimeout)
		res, err := s.Common().GetProfile(ctx, &commonservice.ProfileRequest{Userid: s.UserID})
		cancel()
		if err != nil {
			log.Error("fm_profile2 목록 조회[3007] 실패!!!", "err", err)
			errs = append(errs, "3007")
		} else {
			m := policy.ParseProfile(res.Profile)
			if p.update(gen, func(st *policy.Set) *policy.Set { return st.WithProfile(m) }) {
				p.c.RetainLogs(m) // 로그 보관 일수는 정책 remove_logfile 을 따른다(LOG-026)
				p.c.setProfile(gen, nbs.ParseProfile(res.Profile))
			}
		}
	}
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	ext, err := policy.FetchExtInfo(ctx, s.FileExtension())
	cancel()
	if err != nil {
		log.Error("확장자 목록 조회[3005] 실패!!!", "err", err)
		errs = append(errs, "3005")
	} else if p.update(gen, func(st *policy.Set) *policy.Set { return st.WithExtInfo(ext) }) {
		log.Info(fmt.Sprintf("확장자 목록 조회[3005] >>> 응답 건수 %d, 이 망에 적용 %d", len(ext), len(p.current().ExtRows())))
	}
	ctx, cancel = s.Ctx(nbs.RPCTimeout)
	patt, err := policy.FetchPatterns(ctx, s.FileExtension())
	cancel()
	if err != nil {
		log.Error("헤더패턴 조회[3015] 실패!!!", "err", err)
		errs = append(errs, "3015")
	} else if p.update(gen, func(st *policy.Set) *policy.Set {
		return st.WithPatterns(patt, func(w string) { log.Warn(w) })
	}) {
		st := p.current()
		log.Info(fmt.Sprintf("헤더패턴 조회[3015] >>> 응답 건수 %d, 저장된 행 %d", st.PattResp, st.PattStored))
	}
	if st := p.current(); st != nil && p.c.IsCurrent(gen) {
		log.Info(fmt.Sprintf("권한 조회 >>> fileMaxSize: %d KB, fileMaxCount: %d, clipMaxSize: %d KB, 판정 방식: %s, 압축 내부 검사: %v",
			st.FileSizeKB, st.FileCount, st.ClipSizeKB, st.MethodName(), st.InsideCheck()), "category", applog.CatAuth)
		p.c.Emit(EvPolicyChanged, statusOf(st))
	}
	if len(errs) > 0 {
		return fmt.Errorf("정책을 받지 못했습니다(%s)", strings.Join(errs, "·"))
	}
	return nil
}

func statusOf(st *policy.Set) PolicyStatus {
	depth, _, _ := st.DepthFromPolicy()
	return PolicyStatus{Ready: st.Ready(), Method: st.MethodName(), FileSizeKB: st.FileSizeKB, FileCount: st.FileCount,
		ClipSizeKB: st.ClipSizeKB, Profile: len(st.Profile()), ExtRows: len(st.ExtRows()), PattResp: st.PattResp,
		PattStored: st.PattStored, InsideZip: st.InsideCheck(), ZipDepth: depth}
}

// FileCheckReply 는 파일 검사 결과 창의 답이다. gen 은 EvFileCheckOpen 이 준 검사 세대(다르면 버린다), reject 는 [취소].
// 결과가 나오기 전의 [확인] 은 버리고, [취소]·창 닫힘은 언제든 받는다.
func (p *PolicyService) FileCheckReply(gen int64, reject bool) error {
	p.mu.Lock()
	r := p.run
	cur := int64(0)
	if r != nil {
		cur = r.gen
	}
	what := map[bool]string{true: "취소", false: "진행"}[reject]
	switch {
	case r == nil || r.gen != gen:
		p.mu.Unlock()
		p.log().Info(fmt.Sprintf("파일검사 회신 >>> 지난 검사의 답이라 버린다 >>> 답의 검사[%d] 지금 검사[%d] %s", gen, cur, what))
		return nil
	case !r.result && !reject:
		p.mu.Unlock()
		p.log().Info(fmt.Sprintf("파일검사 회신 >>> 결과가 나오기 전의 답이라 버린다 >>> 답의 검사[%d] 지금 검사[%d] %s", gen, cur, what))
		return nil
	}
	p.mu.Unlock()
	if reject && !r.result {
		r.cancel() // 검사 도중 [취소] ― 남은 파일을 건너뛴다
	}
	select {
	case r.reply <- reject:
	default:
	}
	return nil
}

// ZipPasswordReply 는 압축 비밀번호 입력 창의 답이다. 빈 값은 취소. 비밀번호는 로그에 남기지 않는다.
func (p *PolicyService) ZipPasswordReply(gen int64, password string) error {
	p.mu.Lock()
	r := p.run
	ok := r != nil && r.gen == gen && r.asking
	p.mu.Unlock()
	if !ok {
		p.log().Info(fmt.Sprintf("암호 압축 >>> 기다리는 질문이 없는 답이라 버린다 >>> 답의 검사[%d]", gen))
		return nil
	}
	select {
	case r.pw <- pwAnswer{pw: password, ok: password != ""}:
	default:
	}
	return nil
}

// PolicyList 는 정책 화면 목록이다. kind: "Profile" · "LinkRedirect" · "Extension" · "ExtProfile", word: 조회할 단어(명칭·값, 대소문자 무시).
func (p *PolicyService) PolicyList(kind, word string) ([]PolicyRow, error) {
	st := p.current()
	if st == nil {
		return nil, ErrNotLoggedIn
	}
	var rows []PolicyRow
	switch kind {
	case "Profile":
		for _, kv := range st.Profile() {
			rows = append(rows, PolicyRow{Type: kind, Name: kv.Key, Value: kv.Value})
		}
	case "Extension":
		for _, pv := range st.Patterns() {
			desc := "차단"
			if pv.Allow {
				desc = "허용"
			}
			rows = append(rows, PolicyRow{Type: kind, Name: pv.Ext, Value: spacedPattern(pv.Patt), Description: desc})
		}
	case "ExtProfile":
		for _, e := range st.ExtRows() {
			v := "차단"
			if e.AllowUpload != 0 {
				v = "허용"
			}
			rows = append(rows, PolicyRow{Type: kind, Name: strings.ToUpper(e.Ext), Value: v,
				Description: fmt.Sprintf("course=%d aflag=%d mode=%d aprv_type=%d aprv_user_flag=%d", e.Course, e.AFlag, e.Mode, e.AprvType, e.AprvUserFlag)})
		}
	case "LinkRedirect":
		rows = p.c.LinkPolicyRows()
	default:
		return nil, fmt.Errorf("알 수 없는 정책 유형: %s", kind)
	}
	if w := strings.ToLower(strings.TrimSpace(word)); w != "" {
		out := rows[:0]
		for _, r := range rows {
			if strings.Contains(strings.ToLower(r.Name), w) || strings.Contains(strings.ToLower(r.Value), w) {
				out = append(out, r)
			}
		}
		rows = out
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].Name < rows[b].Name })
	if rows == nil {
		rows = []PolicyRow{}
	}
	return rows, nil
}

// spacedPattern 은 헤더 패턴을 두 글자(1바이트)씩 띄운다(원본 정책 화면).
func spacedPattern(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(p[i:min(i+2, len(p))])
	}
	return b.String()
}

// Status 는 지금 정책 한 벌의 요약이다.
func (p *PolicyService) Status() (PolicyStatus, error) {
	st := p.current()
	if st == nil {
		return PolicyStatus{}, ErrNotLoggedIn
	}
	return statusOf(st), nil
}

// Reload 는 지금 망의 정책(프로파일·확장자 허용 목록·헤더 패턴)을 다시 받는다(트레이 [정책 받아오기]).
// 받은 것은 통째로 바꾸고, 실패한 것은 앞 값을 둔다. URL 필터는 링크 연동 담당이 따로 받는다.
func (p *PolicyService) Reload() error {
	s, err := p.c.Session()
	if err != nil {
		return err
	}
	return p.load(s, p.c.Gen(), true)
}

// ── 전송 담당이 부르는 Go 입구 ──

// PolicyChecker 는 전송·클립보드 담당이 Go 에서 부르는 검사 입구다(Wails 에 묶지 않는다).
type PolicyChecker struct{ p *PolicyService }

// PolicyChecker 는 정책 영역의 Go 입구다.
func (c *Core) PolicyChecker() *PolicyChecker { return &PolicyChecker{p: newPolicyService(c)} }

// CheckRequest 는 업로드 검사 요청이다.
type CheckRequest struct {
	Paths []string // 파일·폴더 경로(화면에서 고른 그대로)
	Auto  bool     // 자동 업로드
}

// CheckResult 는 업로드 검사 결과다. Proceed 가 참일 때만 Items 를 올린다.
type CheckResult struct {
	Gen     int64               // 검사 번호(창의 gen)
	Proceed bool                // 사용자가 [확인]·[차단파일 제외 업로드] 했고(또는 자동 진행) 막는 오류가 없다
	Reason  string              // Proceed 가 거짓인 까닭: empty · count · policy · canceled · rejected · blocked
	Items   []policy.PassedItem // 올릴 것(폴더는 Excludes 를 빼고, 맞힌 zip 비밀번호와 검사 때의 크기·수정 시각을 담는다)
	Fails   int
	Dropped []string // __CLIP__ 이 든 이름이라 뺀 경로
}

// CheckUpload 는 업로드할 파일·폴더를 검사하고 [파일 검사 결과] 창의 답까지 기다린다(원본 uploadFiles 의 개수 판정 → checkUploadFiles →
// fileCheckReply). 암호 압축이면 [압축 비밀번호] 창을 띄워 답을 기다린다. 새 검사가 오면 앞 검사는 창을 닫고 Reason "canceled" 로 돌아온다.
// 답을 받은 뒤 불일치 기록[3316]을 서버에 보낸다(진행·취소 모두).
func (k *PolicyChecker) CheckUpload(ctx context.Context, s *nbs.Session, req CheckRequest) (*CheckResult, error) {
	p := k.p
	if s == nil {
		return nil, ErrNotLoggedIn
	}
	log := p.log()
	res := &CheckResult{}
	var paths []string
	for _, f := range req.Paths {
		if policy.HasClipMarker(f) {
			log.Info("사용할 수 없는 파일명이 포함되어 있어 업로드에서 제외되었습니다.( __CLIP__ )", "path", f)
			res.Dropped = append(res.Dropped, f)
			continue
		}
		paths = append(paths, f)
	}
	if len(paths) == 0 {
		res.Reason = "empty"
		return res, nil
	}

	st, err := p.waitReady(ctx)
	if err != nil {
		return nil, err
	}
	if st == nil {
		log.Warn("업로드 검사 거절 >>> 정책(프로파일·확장자 목록·헤더 패턴)을 아직 다 받지 못했다")
		if !req.Auto {
			p.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: "정책을 받지 못해 파일을 검사할 수 없습니다. 잠시 뒤 다시 시도하세요."})
		}
		res.Reason = "policy"
		return res, nil
	}
	if st.FileCount > 0 && len(paths) > st.FileCount {
		text := fmt.Sprintf("파일 최대 전송 개수를 초과해서 전송할 수 없습니다.\n( 파일 최대 전송개수 : %d )", st.FileCount)
		if req.Auto {
			log.Info("[Auto]" + text)
		} else {
			p.c.dialog("정보", text, "")
		}
		res.Reason = "count"
		return res, nil
	}

	// 새 검사 ― 앞 검사(자동 업로드 등)는 밀어낸다
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r := &checkRun{gen: p.seq.Add(1), cancel: cancel, reply: make(chan bool, 1), pw: make(chan pwAnswer, 1), done: make(chan struct{})}
	defer close(r.done)
	res.Gen = r.gen
	p.mu.Lock()
	old := p.run
	p.run = r
	p.mu.Unlock()
	if old != nil {
		log.Info(fmt.Sprintf("새 검사가 앞 검사를 밀어낸다 >>> 앞 검사[%d] 새 검사[%d]", old.gen, r.gen))
		old.cancel()
		p.c.Emit(EvZipPasswordAsk, ZipPasswordAsk{Gen: old.gen})
		p.c.Emit(EvFileCheckClose, FileCheckClose{Gen: old.gen})
	}
	finish := func() {
		p.mu.Lock()
		if p.run == r {
			p.run = nil
		}
		p.mu.Unlock()
	}
	defer finish()

	log.Debug(fmt.Sprintf("----------------- 파일 검사 시작 ---------------- autoUpload: %v, 검사[%d], 항목 %d", req.Auto, r.gen, len(paths)))
	p.c.Emit(EvFileCheckOpen, FileCheckOpen{Gen: r.gen, Mode: CheckModeUpload, Total: len(paths)})
	b := newRowBatcher(p.c, r.gen)
	out := policy.RunUploadCheck(rctx, st, policy.CheckOptions{Paths: paths, Auto: req.Auto}, policy.Hooks{
		Rows:        b.add,
		Progress:    b.progress,
		AskPassword: p.askPassword(r),
		Log:         func(s string) { log.Info(s) },
	})
	b.stop()

	if out.Canceled {
		p.mu.Lock()
		stale := p.run != r
		p.mu.Unlock()
		if !stale { // 밀려난 것이 아니면(로그아웃은 onLogout 이 닫는다) 이 창을 닫는다
			p.c.Emit(EvFileCheckClose, FileCheckClose{Gen: r.gen})
		}
		res.Reason = "canceled"
		return res, nil
	}
	p.mu.Lock()
	r.result = true
	p.mu.Unlock()
	res.Fails = out.Fails
	p.c.Emit(EvFileCheckResult, FileCheckResult{Gen: r.gen, Fails: out.Fails, Value: out.Value, Skip: out.Skip})

	// 답을 기다린다. "모두 정상" 이면 화면이 2초 뒤 스스로 진행하고, 화면이 없으면 Go 가 3초 뒤 진행한다.
	var fallback <-chan time.Time
	if out.Fails == 0 {
		fallback = time.After(autoProceedFallback)
	}
	var reject bool
	select {
	case reject = <-r.reply:
	case <-fallback:
		log.Info(fmt.Sprintf("파일검사 회신 >>> 화면의 답이 없어 진행한다 >>> 검사[%d]", r.gen))
		p.c.Emit(EvFileCheckClose, FileCheckClose{Gen: r.gen})
	case <-rctx.Done():
		res.Reason = "canceled"
		return res, nil
	}
	go p.sendMismatch(s, out.Mismatches)
	if reject || out.Blocks > 0 {
		log.Info("사용자에 의해 파일 업로드가 취소되었습니다.")
		res.Reason = "rejected"
		if !reject {
			res.Reason = "blocked"
		}
		return res, nil
	}
	res.Proceed = true
	res.Items = out.Items
	return res, nil
}

// waitReady 는 정책 세 가지를 다 받을 때까지 policyWait 만큼 기다린다. 못 받았으면 nil(오류 아님).
func (p *PolicyService) waitReady(ctx context.Context) (*policy.Set, error) {
	p.mu.Lock()
	ready := p.ready
	p.mu.Unlock()
	if ready == nil {
		return nil, ErrNotLoggedIn
	}
	t := time.NewTimer(policyWait)
	defer t.Stop()
	select {
	case <-ready:
		return p.current(), nil
	case <-t.C:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// askPassword 는 비밀번호 창을 띄우고 답을 기다린다(검사 작업자가 부르고, policy 가 한 번에 하나만 부른다).
func (p *PolicyService) askPassword(r *checkRun) func(context.Context, string, bool) (string, bool) {
	return func(ctx context.Context, name string, retry bool) (string, bool) {
		p.mu.Lock()
		if p.run != r {
			p.mu.Unlock()
			return "", false
		}
		r.asking = true
		select { // 앞 질문에 늦게 온 답을 버린다
		case <-r.pw:
		default:
		}
		p.mu.Unlock()
		defer func() {
			p.mu.Lock()
			r.asking = false
			p.mu.Unlock()
		}()
		p.c.Emit(EvZipPasswordAsk, ZipPasswordAsk{Gen: r.gen, Name: name, Retry: retry})
		select {
		case a := <-r.pw:
			return a.pw, a.ok
		case <-ctx.Done():
			p.c.Emit(EvZipPasswordAsk, ZipPasswordAsk{Gen: r.gen})
			return "", false
		}
	}
}

// sendMismatch 는 불일치 기록[3316]을 5건씩 보낸다. 실패하면 로그만 남긴다(업로드는 막지 않는다).
func (p *PolicyService) sendMismatch(s *nbs.Session, list []policy.Mismatch) {
	log := p.log()
	for _, req := range policy.MismatchRequests(s.UserID, list) {
		for _, m := range req.List {
			log.Debug("파일 패턴 불일치 목록 등록[3316]: " + m.Fname)
		}
		ctx, cancel := s.Ctx(nbs.RPCTimeout)
		_, err := s.FileExtension().SetMismatch(ctx, req)
		cancel()
		if err != nil {
			log.Warn("파일 패턴 불일치 목록 등록[3316] 실패!!!", "err", err, "count", req.MCount)
		}
	}
}

// ShowDownloadCheck 는 결재자가 받은 것(압축·폴더)의 내부 목록을 [파일 검사 결과] 창에 [확인] 만 두고 보인다(원본 verifyDownloadZip).
// 내부 파일이 없으면(압축 아님·잠김·손상) 창을 띄우지 않는다. 답을 기다리는 업로드 검사가 있으면 그것이 끝난 뒤에 띄운다.
func (k *PolicyChecker) ShowDownloadCheck(ctx context.Context, path string) error {
	p := k.p
	st := p.current()
	if st == nil {
		return ErrNotLoggedIn
	}
	depth, _, _ := st.DepthFromPolicy()
	log := p.log()
	rows, fails, show := policy.DownloadCheck(ctx, path, depth, "", policy.Limits{}, func(s string) { log.Info(s) })
	if !show {
		return ctx.Err()
	}
	for {
		p.mu.Lock()
		r := p.run
		p.mu.Unlock()
		if r == nil {
			break
		}
		select {
		case <-r.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	gen := p.seq.Add(1)
	p.c.Emit(EvFileCheckOpen, FileCheckOpen{Gen: gen, Mode: CheckModeDownload, Total: len(rows)})
	p.c.Emit(EvFileCheckRow, FileCheckRows{Gen: gen, Rows: rows, Done: len(rows), Total: len(rows)})
	p.c.Emit(EvFileCheckResult, FileCheckResult{Gen: gen, Fails: fails, Value: CheckModeDownload})
	return nil
}

// CheckClipSize 는 클립보드 업로드의 크기 판정이다. 넘으면 「정보」 창(전송 가능한 파일 크기 초과)을 띄우고 거짓.
func (k *PolicyChecker) CheckClipSize(size int64) bool {
	st := k.p.current()
	if st == nil {
		return false
	}
	ok, text := policy.ClipSizeOK(st, size)
	if !ok {
		k.p.c.dialog("정보", text, "")
	}
	return ok
}

// Current 는 지금 정책 한 벌이다(로그인 전 nil). 읽기만 한다.
func (k *PolicyChecker) Current() *policy.Set { return k.p.current() }

// CloseCheck 는 답을 기다리는 업로드 검사를 취소로 끝내고 창을 닫는다(전송 담당의 목록 비우기·로그아웃 정리).
func (k *PolicyChecker) CloseCheck() {
	p := k.p
	p.mu.Lock()
	r := p.run
	p.run = nil
	p.mu.Unlock()
	if r != nil {
		r.cancel()
		p.c.Emit(EvZipPasswordAsk, ZipPasswordAsk{Gen: r.gen})
		p.c.Emit(EvFileCheckClose, FileCheckClose{Gen: r.gen})
	}
}

// rowBatcher 는 검사 행·진행을 120ms 마다 모아 EvFileCheckRow 로 보낸다(파일 수천 개에서 사건이 넘치지 않게).
type rowBatcher struct {
	c              *Core
	gen            int64
	mu             sync.Mutex
	rows           []policy.Row
	done, total    int
	cur            string
	dirty          bool
	quit, finished chan struct{}
}

func newRowBatcher(c *Core, gen int64) *rowBatcher {
	b := &rowBatcher{c: c, gen: gen, quit: make(chan struct{}), finished: make(chan struct{})}
	go func() {
		defer close(b.finished)
		t := time.NewTicker(120 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				b.flush()
			case <-b.quit:
				b.flush()
				return
			}
		}
	}()
	return b
}

func (b *rowBatcher) add(rows []policy.Row) {
	b.mu.Lock()
	b.rows = append(b.rows, rows...)
	b.dirty = true
	b.mu.Unlock()
}

func (b *rowBatcher) progress(done, total int, cur string) {
	b.mu.Lock()
	b.done, b.total, b.cur, b.dirty = done, total, cur, true
	b.mu.Unlock()
	b.c.Touch() // 오래 걸리는 검사 중 자동 로그아웃되지 않게
}

func (b *rowBatcher) flush() {
	b.mu.Lock()
	if !b.dirty {
		b.mu.Unlock()
		return
	}
	ev := FileCheckRows{Gen: b.gen, Rows: b.rows, Done: b.done, Total: b.total, Current: b.cur}
	b.rows, b.dirty = nil, false
	b.mu.Unlock()
	if ev.Rows == nil {
		ev.Rows = []policy.Row{}
	}
	b.c.Emit(EvFileCheckRow, ev)
}

func (b *rowBatcher) stop() {
	close(b.quit)
	<-b.finished
}
