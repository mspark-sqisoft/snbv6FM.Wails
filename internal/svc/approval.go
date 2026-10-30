package svc

// 결재 영역: 결재자 선택 창, 결재요청(사후결재 4127), 결재처리(3390·3092), 결재 받기, 대리결재자(docs/port/06). 결재 알림은 log.go 가 받는다.
// 판정·RPC 는 internal/approval 에 있고, 여기는 화면이 부르는 메서드와 창 왕복(approval:select ↔ ApproverReply)만 둔다.
//
// 다른 영역이 부르는 Go 입구(서비스 메서드가 아니라 패키지 함수 ― Wails 가 화면 바인딩으로 내보내지 않게):
//   - PickApprover(ctx, s, req)  업로드 직전(전송), 결재방식·결재자·resv 등을 정한다

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/approval"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
)

// EvApproverSelectClose 는 결재자 선택 창을 닫으라는 사건이다 {gen}(부른 쪽이 취소했을 때). 여는 사건은 EvApproverSelectAsk.
const EvApproverSelectClose = "approval:select-close"

// ApprovalService 는 결재자 선택 창·[결재요청]·[결재처리]·[대리결재자] 화면이 부른다.
type ApprovalService struct {
	base

	mu     sync.Mutex
	ask    *pendingAsk // 답을 기다리는 결재자 창(한 번에 하나)
	askSeq int64

	downloading atomic.Bool
}

type pendingAsk struct {
	gen    int64
	myID   string
	selfOK bool
	order  []approval.User // 트리 순서
	ch     chan askAnswer  // 크기 1
}

type askAnswer struct {
	ids, names []string
	cause      string
	cancel     bool
}

// approvalSvc 는 패키지 함수 PickApprover 가 쓰는 하나뿐인 서비스다.
var approvalSvc *ApprovalService

func newApprovalService(c *Core) *ApprovalService {
	a := &ApprovalService{base: base{c}}
	approvalSvc = a
	c.OnLogout(a.onLogout)
	return a
}

func (a *ApprovalService) log() *slog.Logger { return a.c.Log().With("category", applog.CatApproval) }

func (a *ApprovalService) onLogout() {
	a.mu.Lock()
	p := a.ask
	a.ask = nil
	a.mu.Unlock()
	if p != nil {
		p.ch <- askAnswer{cancel: true}
	}
}

// ── 결재방식 ──────────────────────────────────────────

// ApprovalMode 는 화면이 결재 흐름을 고르는 데 쓰는 값이다.
type ApprovalMode struct {
	PreMode       bool   `json:"preMode"`      // 사전결재 모드(fm_upfile_mode 0). 거짓이면 [결재요청] 단추를 보인다
	FileType      int32  `json:"fileType"`     // 이 망의 파일 업로드 결재방식
	ClipType      int32  `json:"clipType"`     // 이 망의 클립보드 업로드 결재방식
	FileTypeText  string `json:"fileTypeText"` // 문구
	ClipTypeText  string `json:"clipTypeText"`
	SelfApprove   bool   `json:"selfApprove"`   // self_approve == on
	MaxDeputyDays int    `json:"maxDeputyDays"` // max_deputy_date(없으면 5)
}

// Mode 는 로그인한 망의 결재방식·결재 정책이다.
func (a *ApprovalService) Mode() (*ApprovalMode, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	f, cl := s.AprvType(false), s.AprvType(true)
	return &ApprovalMode{
		PreMode: approval.PreMode(s.Profile), FileType: f, ClipType: cl,
		FileTypeText: approval.TypeText(f), ClipTypeText: approval.TypeText(cl),
		SelfApprove: approval.SelfApproveAllowed(s.Profile), MaxDeputyDays: approval.DeputyLimit(s.Profile),
	}, nil
}

// ── 결재자 선택 창 ────────────────────────────────────

// SelectAsk 는 EvApproverSelectAsk("approval:select") 의 값이다. 화면은 창을 띄우고 ApproverReply(gen, …) 로 답한다.
type SelectAsk struct {
	Gen         int64               `json:"gen"`
	Tree        []*approval.Dept    `json:"tree"`
	MyID        string              `json:"myId"`
	SelfApprove bool                `json:"selfApprove"`
	ReqDept     string              `json:"reqDept"`
	ReqName     string              `json:"reqName"`
	AprvType    int32               `json:"aprvType"`
	TypeOptions []string            `json:"typeOptions"` // 결재방법 콤보(값 = 자리)
	FileDesc    string              `json:"fileDesc"`    // "a.txt" · "a.txt 외 2건" · "클립보드"
	Direction   string              `json:"direction"`   // 전송방향(fmenv.ini NETWORK)
	TotalSize   int64               `json:"totalSize"`
	Clip        bool                `json:"clip"`
	Files       []approval.PickFile `json:"files"`
}

// PickApprover 는 업로드(또는 사후결재 [결재요청]) 직전에 결재 칸을 정한다. 전송 담당이 업로드 대기열에 넣기 전에 부른다.
//   - 업로드이고 사후결재 모드(fm_upfile_mode 1): 창 없이 결재자 없음, resv·aprv_kind 만
//   - 결재방식 0(자동)·6(자동자가, 결재자 = 나): 창 없이
//   - 그 밖: 결재자 목록[3370] 을 새로 받아 창을 띄우고 답을 기다린다(ctx 가 끝나면 창을 닫고 ctx 오류)
//
// 오류: approval.ErrCanceled(사용자 취소·로그아웃), approval.ErrNoApprover(고를 결재자 없음),
// approval.ErrBusy(다른 결재자 창이 떠 있음), 결재자 목록 조회 실패. 어느 오류든 부른 쪽은 업로드 목록을 놓는다.
func PickApprover(ctx context.Context, s *nbs.Session, req approval.PickRequest) (*approval.PickResult, error) {
	return approvalSvc.pick(ctx, s, req)
}

func (a *ApprovalService) pick(ctx context.Context, s *nbs.Session, req approval.PickRequest) (*approval.PickResult, error) {
	typ := s.AprvType(req.Clip)
	pre := approval.PreMode(s.Profile)
	res := &approval.PickResult{AprvType: typ, AprvDept: s.Info.GetUserDeptName()}
	done := func() (*approval.PickResult, error) {
		res.Resv, res.AprvKind = approval.UploadFields(req.Clip, pre, typ, len(res.ApproverIDs))
		return res, nil
	}
	if !req.After && !pre {
		return done()
	}
	if !approval.NeedsApprover(typ) {
		if typ == nbs.AprvSelf {
			res.ApproverIDs, res.ApproverNames = []string{s.UserID}, []string{s.Info.GetUserName()}
		}
		return done()
	}

	list, err := approval.Approvers(s)
	if err != nil {
		return nil, err
	}
	tree := approval.BuildTree(list)
	if tree == nil {
		a.log().Info("결재자선택 >>> 결재자 트리가 비어 창을 띄우지 않습니다.")
		return nil, approval.ErrNoApprover
	}

	a.mu.Lock()
	if a.ask != nil {
		a.mu.Unlock()
		return nil, approval.ErrBusy
	}
	a.askSeq++
	p := &pendingAsk{gen: a.askSeq, myID: s.UserID, selfOK: approval.SelfApproveAllowed(s.Profile), order: approval.TreeUsers(tree), ch: make(chan askAnswer, 1)}
	a.ask = p
	a.mu.Unlock()

	names := make([]string, len(req.Files))
	var total int64
	for i, f := range req.Files {
		names[i] = f.Name
		total += f.Size
	}
	a.c.Emit(EvApproverSelectAsk, SelectAsk{
		Gen: p.gen, Tree: tree, MyID: s.UserID, SelfApprove: p.selfOK,
		ReqDept: s.Info.GetUserDeptName(), ReqName: s.Info.GetUserName(),
		AprvType: typ, TypeOptions: approval.TypeOptions(), FileDesc: approval.FileDesc(names, req.Clip),
		Direction: a.direction(s.Server.Loc), TotalSize: total, Clip: req.Clip, Files: req.Files,
	})
	a.log().Info("결재자선택 창", "gen", p.gen, "aprvType", typ, "files", len(req.Files), "approvers", len(p.order))

	select {
	case ans := <-p.ch:
		if ans.cancel {
			return nil, approval.ErrCanceled
		}
		res.ApproverIDs, res.ApproverNames, res.Cause, res.Dialog = ans.ids, ans.names, ans.cause, true
		return done()
	case <-ctx.Done():
		a.mu.Lock()
		if a.ask == p {
			a.ask = nil
		}
		a.mu.Unlock()
		a.c.Emit(EvApproverSelectClose, map[string]int64{"gen": p.gen})
		return nil, ctx.Err()
	}
}

// direction 은 망 위치의 전송방향 글자다(fmenv.ini NETWORK, 원본 getDirectionText). 없으면 "Unknown".
func (a *ApprovalService) direction(loc int) string {
	if env, _ := a.c.Env(); env != nil {
		for _, sv := range env.Servers {
			if sv.Loc == loc {
				return sv.Network
			}
		}
	}
	return "Unknown"
}

// ApproverReply 는 결재자 선택 창(approval:select)의 답이다. cancel 이면 취소([취소]·X·Esc·창 닫힘 어느 길이든).
// 고른 결재자가 없거나, 사유가 비거나, self_approve 가 아닌데 나를 골랐으면 오류를 돌려주고 창은 그대로 둔다(다시 고르게).
// 이미 끝난 창(gen 이 다름)의 답은 버린다.
func (a *ApprovalService) ApproverReply(gen int64, approverIDs []string, cause string, cancel bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.ask
	if p == nil || p.gen != gen {
		return nil
	}
	if cancel {
		a.ask = nil
		p.ch <- askAnswer{cancel: true}
		a.log().Info("사용자에 의해 결재요청이 취소되었습니다.")
		return nil
	}
	picked := map[string]bool{}
	for _, id := range approverIDs {
		picked[id] = true
	}
	var ids, names []string
	for _, u := range p.order { // 트리 순서, 같은 ID 는 한 번
		if picked[u.ID] {
			ids, names = append(ids, u.ID), append(names, u.Name)
			delete(picked, u.ID)
		}
	}
	cause = strings.TrimSpace(cause)
	switch {
	case len(ids) == 0:
		return errors.New("결재자를 선택해 주세요.")
	case cause == "":
		return errors.New("결재요청 사유를 입력해 주세요.")
	case !p.selfOK && slices.Contains(ids, p.myID):
		return errors.New("자신을 결재자로 선택할 수 없습니다.")
	}
	a.ask = nil
	p.ch <- askAnswer{ids: ids, names: names, cause: cause}
	return nil
}

// ── 결재요청(사후결재 모드) ───────────────────────────

// RequestRow 는 [결재요청] 화면의 한 줄(이 망에서 올린 서버 파일)이다.
type RequestRow struct {
	ServerName string `json:"serverName"`
	Name       string `json:"name"`
	Size       uint64 `json:"size"`
	Clip       bool   `json:"clip"`
	AprvState  int32  `json:"aprvState"`
	StateText  string `json:"stateText"`
	StatusKind string `json:"statusKind"` // 칩 색(파일전송 화면의 statusKind 와 같은 판정)
	CanRequest bool   `json:"canRequest"`
	Block      string `json:"block"` // 상신할 수 없는 까닭(가능하면 "")
	Cause      string `json:"cause"` // 요청 사유(이미 상신한 것)
}

// RequestList 는 이 망에서 올린 서버 파일 목록이다(서버파일 목록[3311] 에서 course == 이 망).
func (a *ApprovalService) RequestList() ([]RequestRow, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	list, err := files.List(s)
	if err != nil {
		return nil, err
	}
	rows := []RequestRow{}
	for _, f := range list {
		if int(f.Course) != s.Server.Loc {
			continue
		}
		block := approval.RequestBlock(f.VirusFlag, f.PrivState, f.State)
		clip := approval.IsClipName(f.NameStr)
		name := filepath.Base(f.NameStr)
		if clip {
			name = "클립보드"
		}
		rows = append(rows, RequestRow{
			ServerName: f.ServerStr, Name: name, Size: f.FileSize, Clip: clip, AprvState: f.State,
			StateText: approval.StateText(f.State), StatusKind: files.GetState(f.State, f.PrivState, f.VirusFlag).Kind,
			CanRequest: block == "", Block: block, Cause: f.ReqCause,
		})
	}
	return rows, nil
}

// ApprovalRequest 는 사후결재 [결재요청](4127) 할 서버 파일들이다. 결재자·사유는 이 호출이 띄우는 결재자 창에서 받는다.
type ApprovalRequest struct {
	ServerNames []string `json:"serverNames"`
}

// RequestResult 는 [결재요청] 결과다.
type RequestResult struct {
	Canceled bool     `json:"canceled"` // 결재자 창에서 취소
	Sent     int      `json:"sent"`     // 상신한 파일 수
	Skipped  int      `json:"skipped"`  // 상신할 수 없어 뺀 파일 수
	Fails    []string `json:"fails"`    // 실패한 묶음의 서버 사유
}

// RequestApproval 은 고른 서버 파일을 결재요청[4127] 한다(APR-007~011·041~043).
// 상신 직전에 서버파일 목록을 새로 받아 판정하고(APR-009 경합 해소), 결재자 창을 띄워 답을 받은 뒤 비밀번호별 10개 묶음으로 보낸다.
func (a *ApprovalService) RequestApproval(req ApprovalRequest) (*RequestResult, error) {
	if len(req.ServerNames) == 0 {
		return nil, errors.New("결재요청할 파일을 선택해 주세요.")
	}
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	list, err := files.List(s)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range req.ServerNames {
		want[n] = true
	}
	var names, pswds []string
	var pf []approval.PickFile
	nClip, nPicked := 0, 0
	for _, f := range list {
		if !want[f.ServerStr] {
			continue
		}
		nPicked++
		clip := approval.IsClipName(f.NameStr)
		if clip {
			nClip++
		}
		if !approval.CanRequest(f.VirusFlag, f.PrivState, f.State) {
			continue
		}
		names, pswds = append(names, f.ServerStr), append(pswds, f.ZipPswd)
		pf = append(pf, approval.PickFile{Name: filepath.Base(f.NameStr), Size: int64(f.FileSize)})
	}
	if nClip > 0 && nClip < nPicked {
		return nil, errors.New("파일과 클립보드는 동시에 결재 요청 할 수 없습니다.")
	}
	if len(names) == 0 {
		return nil, errors.New("결재요청 가능한 목록이 없습니다.")
	}
	out := &RequestResult{Skipped: len(req.ServerNames) - len(names)}
	if out.Skipped > 0 {
		a.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: "결재요청이 가능한 파일만 상신합니다."})
	}
	clip := nClip > 0
	if clip {
		for i := range pf {
			pf[i].Name = "클립보드"
		}
	}
	t := a.c.Transfers()
	ctx, cancel, token, err := t.claim(false) // 결재자 창도 업로드 입구를 쓴다(원본 claimUploadList) ― 겹치지 않고, 로그아웃에 닫힌다
	if err != nil {
		return nil, err
	}
	defer t.release(token, cancel)
	gen := a.c.Gen()
	pick, err := a.pick(ctx, s, approval.PickRequest{Files: pf, Clip: clip, After: true})
	if errors.Is(err, approval.ErrCanceled) || errors.Is(err, context.Canceled) {
		out.Canceled = true
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if !a.c.IsCurrent(gen) {
		return nil, ErrNotLoggedIn
	}
	out.Sent, out.Fails = approval.RequestAfter(s, approval.AfterRequest{
		Names: names, Pswds: pswds, ApproverIDs: pick.ApproverIDs, ApproverNames: pick.ApproverNames,
		AprvType: pick.AprvType, Cause: pick.Cause,
	})
	a.log().Info("결재요청[4127]", "sent", out.Sent, "failedGroups", len(out.Fails))
	if out.Sent > 0 {
		a.c.Emit(EvServerFilesChanged, nil)
	}
	return out, nil
}

// ── 결재처리 ──────────────────────────────────────────

// ApprovalItem 은 [결재처리] 목록의 한 줄이다(서버 AprvFileInfo).
type ApprovalItem struct {
	Seqno       int32  `json:"seqno"`
	FileName    string `json:"fileName"` // PC 파일 이름(경로 없이)
	ServerName  string `json:"serverName"`
	FileSize    uint64 `json:"fileSize"`
	Course      int32  `json:"course"`     // 0 = 다운로드(인터넷망→업무망), 1 = 업로드
	CourseText  string `json:"courseText"` // 전송방향(fmenv.ini NETWORK)
	Requester   string `json:"requester"`  // 요청자 ID
	UserName    string `json:"userName"`   // 요청자 이름
	Dept        string `json:"dept"`
	RequestTime string `json:"requestTime"`
	Cause       string `json:"cause"`
	AprvState   int32  `json:"aprvState"`
	StateText   string `json:"stateText"`
	AprvRank    int32  `json:"aprvRank"`
	DirFlag     bool   `json:"dirFlag"` // 폴더(압축으로 올라온 것)
	Clip        bool   `json:"clip"`
	PrivCnt     int32  `json:"privCnt"` // 개인정보 검출 건수
}

// Pending 은 [결재처리] 목록이다(결재대상목록[3390]).
func (a *ApprovalService) Pending() ([]ApprovalItem, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	list, err := approval.Pending(s)
	if err != nil {
		return nil, err
	}
	out := make([]ApprovalItem, 0, len(list))
	for _, f := range list {
		clip := approval.IsClipName(f.FileNamePc) || approval.IsClipName(f.FileNameSvr)
		name := filepath.Base(strings.ReplaceAll(f.FileNamePc, "\\", "/"))
		if clip {
			name = "클립보드"
		}
		out = append(out, ApprovalItem{
			Seqno: f.Seqno, FileName: name, ServerName: f.FileNameSvr, FileSize: f.FileSize,
			Course: f.FileCourse, CourseText: a.direction(int(f.FileCourse)), Requester: f.FileUser, UserName: f.UserName,
			Dept: f.UserDeptName, RequestTime: f.FileDate, Cause: f.FileCause, AprvState: f.AprvState,
			StateText: approval.StateText(f.AprvState), AprvRank: f.AprvRank, DirFlag: f.DirFlag == "1", Clip: clip, PrivCnt: f.PrivCnt,
		})
	}
	return out, nil
}

// ApproveResult 는 승인·반려 한 건의 결과다.
type ApproveResult struct {
	Seqno  int32  `json:"seqno"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason"`
}

// Approve 는 고른 건을 승인(approve 참) 또는 반려한다(결재[3092], 동시 4건). 건별 결과를 모아 돌려준다.
// 성공 건은 요청자에게 결재 완료 알림(9004 kind 5)을 보낸다. 화면은 끝나면 목록을 다시 받는다.
func (a *ApprovalService) Approve(items []ApprovalItem, approve bool, cause string) ([]ApproveResult, error) {
	if len(items) == 0 {
		return nil, errors.New("선택된 결재 항목이 없습니다.")
	}
	cause = strings.TrimSpace(cause)
	if cause == "" {
		return nil, errors.New("결재 사유를 입력해 주세요.")
	}
	if utf8.RuneCountInString(cause) > 100 {
		cause = string([]rune(cause)[:100])
	}
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	ts := make([]approval.Target, len(items))
	for i, it := range items {
		ts[i] = approval.Target{Seqno: it.Seqno, ServerName: it.ServerName, Requester: it.Requester, AprvRank: it.AprvRank}
	}
	outs := approval.Approve(s, ts, approve, cause)
	res := make([]ApproveResult, len(outs))
	fails := 0
	for i, o := range outs {
		res[i] = ApproveResult(o)
		if !o.OK {
			fails++
			a.log().Warn("결재[3093] 실패", "seqno", o.Seqno, "reason", o.Reason)
		}
	}
	a.log().Info("결재[3092]", "approve", approve, "count", len(outs), "failed", fails)
	return res, nil
}

// ApprovalDownload 는 결재 받기 결과다.
type ApprovalDownload struct {
	Busy     bool     `json:"busy"`     // 앞 결재 받기가 아직 돈다 ― 화면은 「받는 중」 창
	Canceled bool     `json:"canceled"` // 폴더 고르기 취소
	Saved    []string `json:"saved"`    // 받은 PC 경로
	Fails    []string `json:"fails"`    // "이름: 사유"
}

// DownloadForApproval 은 결재 대상을 받는다(내용 확인용). 받은 뒤 서버 파일을 지우지 않는다(APR-061).
// 클립보드는 폴더를 묻지 않고 임시 폴더로 받아 결재자 클립보드에 넣고(붙여넣기 흉내 없음, Saved 에 없다), 파일은 폴더를 고른 뒤 받는다.
// 폴더(서버에 zip)는 일반 받기처럼 풀어서 폴더로 둔다(원본 FileTransfer.cpp `Recv Directory Extract!`). 같은 이름이 있으면 파일 중복 확인 창으로 묻는다.
// 겹쳐 부르면 Busy ― 받은 것의 내부 목록을 훑는 동안까지다(창이 뜬 뒤에는 풀린다).
// 받기는 전송 대기열(서랍에 보인다)로 돌리고 모두 끝날 때까지 기다린다. 받은 압축·폴더는 내부 목록 창을 띄운다(APR-063).
func (a *ApprovalService) DownloadForApproval(items []ApprovalItem) (*ApprovalDownload, error) {
	if len(items) == 0 {
		return nil, errors.New("다운로드할 파일을 선택해 주세요.")
	}
	if !a.downloading.CompareAndSwap(false, true) {
		return &ApprovalDownload{Busy: true}, nil
	}
	inspecting := false // 내부 목록 고루틴이 표시를 넘겨받았다
	defer func() {
		if !inspecting {
			a.downloading.Store(false)
		}
	}()
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	out := &ApprovalDownload{}
	dir := ""
	for _, it := range items {
		if !it.Clip && dir == "" {
			dir, err = application.Get().Dialog.OpenFile().SetTitle("받을 폴더").SetDirectory(a.c.dialogFolder()).
				CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).PromptForSingleSelection()
			if err != nil || dir == "" {
				out.Canceled = true
				return out, nil
			}
		}
	}
	root := filepath.Join(tempDir(), "clip")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	clipDir, err := os.MkdirTemp(root, "aprv")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(clipDir) // 받은 클립보드 내용을 디스크에 남기지 않는다
	var reqs, fileReqs []DownloadRequest
	var cands []files.Candidate
	for _, it := range items {
		if it.Clip { // 확장자는 받기가 서버 이름으로 붙인다
			reqs = append(reqs, DownloadRequest{ServerName: it.ServerName, Dest: filepath.Join(clipDir, clipMarker+strconv.Itoa(int(it.Seqno))),
				Size: int64(it.FileSize), Clip: true, Display: it.FileName})
			continue
		}
		name := it.FileName
		if it.DirFlag && len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".zip") {
			name = name[:len(name)-4] // 폴더는 서버에 zip 으로 있다 ― 푸는 자리의 이름에는 .zip 이 없다
		}
		dest, err := files.SavePath(dir, name)
		if err != nil {
			out.Fails = append(out.Fails, it.FileName+": "+err.Error())
			continue
		}
		cands = append(cands, files.Candidate{Dir: dir, Name: name})
		fileReqs = append(fileReqs, DownloadRequest{ServerName: it.ServerName, Dest: dest, Size: int64(it.FileSize), Folder: it.DirFlag, Display: name})
	}
	t := a.c.Transfers()
	over, skip, ok := t.askConflicts(cands, func(i int) bool { return fileReqs[i].Folder })
	if !ok {
		return nil, ErrNotLoggedIn
	}
	for i, r := range fileReqs {
		if !skip[i] {
			r.Overwrite = over[i]
			reqs = append(reqs, r)
		}
	}
	saved, fails := t.downloadApproval(s, reqs)
	out.Fails = append(out.Fails, fails...)
	for _, p := range saved {
		if filepath.Dir(p) == clipDir { // CLIP-040: 결재자 클립보드에 넣는다. 안내·실패 창은 deliver 가 띄운다. 여러 건이면 마지막 것이 남는다
			clips.deliver(&transfer.DownloadResult{Path: p})
		} else {
			out.Saved = append(out.Saved, p)
		}
	}
	shown := out.Saved
	gen := a.c.Gen()
	inspecting = true
	go func() {
		defer a.downloading.Store(false)
		for _, p := range shown {
			if !a.c.IsCurrent(gen) {
				return
			}
			// ponytail: 여러 건이면 창이 차례로 바뀐다(앞 창을 닫기를 기다리지 않는다). 목록 창 하나로 묶으려면 정책 영역에 입구가 필요하다.
			if err := a.c.PolicyChecker().ShowDownloadCheck(a.c.Transfers().loginCtx(), p); err != nil {
				a.log().Debug("결재 받기 내부 목록을 보이지 못했다", "path", p, "err", err)
			}
		}
	}()
	return out, nil
}

// ── 대리결재자 ────────────────────────────────────────

// ProxyApprover 는 대리결재자 한 명과 기간이다. 날짜는 "YYYY-MM-DD".
type ProxyApprover struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Dept  string `json:"dept"`
	From  string `json:"from"`
	To    string `json:"to"`
	Cause string `json:"cause"`
}

// ProxyState 는 [대리결재자] 화면 값이다: 등록된 대리결재자(없으면 nil)·후보(나를 뺀 결재자 목록)·기간 상한.
type ProxyState struct {
	Current    *ProxyApprover  `json:"current"`
	Candidates []approval.User `json:"candidates"`
	MaxDays    int             `json:"maxDays"`
}

// Proxy 는 등록된 대리결재자[3174] 와 후보 목록[3370] 이다.
func (a *ApprovalService) Proxy() (*ProxyState, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	cur, err := a.currentProxy(s)
	if err != nil {
		return nil, err
	}
	list, err := approval.Approvers(s)
	if err != nil {
		return nil, err
	}
	return &ProxyState{Current: cur, Candidates: approval.Candidates(list, s.UserID), MaxDays: approval.DeputyLimit(s.Profile)}, nil
}

func (a *ApprovalService) currentProxy(s *nbs.Session) (*ProxyApprover, error) {
	r, err := approval.Proxy(s)
	if err != nil {
		return nil, err
	}
	if r.GetAltUserid() == "" {
		return nil, nil
	}
	return &ProxyApprover{ID: r.AltUserid, Name: r.AltUsername, From: approval.ScreenDate(r.ExpireFrom), To: approval.ScreenDate(r.ExpireTo)}, nil
}

// RegisterProxy 는 대리결재자를 등록한다[3272]. 막는 순서(APR-082): 이미 있음 → 후보 미선택 → 사유 없음 → 기간(끝<시작·상한 초과).
func (a *ApprovalService) RegisterProxy(p ProxyApprover) error {
	s, err := a.c.Session()
	if err != nil {
		return err
	}
	cur, err := a.currentProxy(s)
	if err != nil {
		return err
	}
	p.Cause = strings.TrimSpace(p.Cause)
	switch {
	case cur != nil:
		return errors.New("등록된 대리결재자가 있습니다.\n삭제하고 등록해 주세요.")
	case p.ID == "":
		return errors.New("아래 목록에서 대리결재자를 선택해 주세요.")
	case p.Cause == "":
		return errors.New("대리결재 사유를 입력해 주세요.")
	}
	if err := approval.CheckDeputyPeriod(p.From, p.To, approval.DeputyLimit(s.Profile)); err != nil {
		return err
	}
	return approval.RegisterProxy(s, p.ID, p.Name, p.From, p.To, p.Cause)
}

// DeleteProxy 는 등록된 대리결재자를 지운다[3276]. 없으면 「삭제할 대리결재자가 없습니다.」
func (a *ApprovalService) DeleteProxy() error {
	s, err := a.c.Session()
	if err != nil {
		return err
	}
	cur, err := a.currentProxy(s)
	if err != nil {
		return err
	}
	if cur == nil {
		return errors.New("삭제할 대리결재자가 없습니다.")
	}
	return approval.DeleteProxy(s, cur.ID, cur.Name)
}
