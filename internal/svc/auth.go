package svc

// 인증 영역: 서버 목록, 로그인, 로그아웃, 사용자 정보, ID 저장, 자동 로그아웃, 비밀번호 변경(docs/port/01).
//
// 로그인 흐름(3.1): 길이 검사 → PreLogin·Login·GetProfile(nbs.Login) → 무결성 검사(검사유형 0) → 중복 로그인 판정
// → 비밀번호 판단(기본 비밀번호·잔여일) → 세션 들임(Core.login: OnLogin 훅·자동 로그아웃 시계) → 화면 전환.
// 비밀번호 기한이 지났으면(잔여일 < 0) 세션을 들이지 않고 쥐고만 있다가, 변경에 성공해 CompleteLogin 을 부를 때 들인다
// ― 변경 전에는 메인 화면도 OnLogin 훅(알림 스트림·자동 전송 등)도 시작하지 않는다(8장, AUTH-046).

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/integrity"
	"snbapp/internal/nbs"
)

// 확인 창 제목·문구(원본 ViewControllerNotifySlot.cpp, docs/port/01 5.7). 다른 영역도 LogoutNotice 에 쓴다.
const (
	TitleError        = "오류"
	TitleDuplicate    = "중복 로그인"
	MsgDuplicateLogin = "해당 계정은 이미 다른 PC에서 로그인 중입니다.\n프로그램을 종료합니다." // 로그인 때 중복
	MsgDuplicateKick  = "다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다."           // 사용 중 알림 9004 kind 6·7
	TitleIntegrity    = "무결성 검증 실패"
	MsgIntegrity      = "프로그램 파일이 손상되었거나 서버에 등록된 것과 다릅니다.\n안전을 위해 프로그램을 종료합니다."
	msgRequired       = "계정정보를 정확히 입력해 주세요."
)

// 새 비밀번호 규칙 문구(원본 PasswordChangeDlg.qml). 규칙은 코드 고정이다(AUTH-047·050).
const (
	MsgPwRule        = "영문, 숫자, 특수문자 포함 10~16글자를 입력해 주세요."
	MsgPwCurEmpty    = "현재 비밀번호를 입력해 주세요."
	MsgPwNewEmpty    = "새 비밀번호를 입력해 주세요."
	MsgPwSame        = "새 비밀번호가 현재 비밀번호와 같습니다."
	MsgPwConfirmNone = "확인 비밀번호를 입력해 주세요."
	MsgPwMismatch    = "새 비밀번호와 확인 비밀번호는 같아야 합니다."
)

// AuthService 는 로그인 화면과 툴바 사용자 칸이 부른다.
type AuthService struct {
	base
	loginMu sync.Mutex // 로그인 두 번이 겹치지 않게

	mu      sync.Mutex
	pending *nbs.Session // 비밀번호 기한 지남 ― 변경 성공 전까지 들이지 않은 세션
	pendID  string       // 저장할 입력 ID(pending 이 들어갈 때 저장)
}

func newAuthService(c *Core) *AuthService { return &AuthService{base: base{c}} }

func (a *AuthService) log() *slog.Logger { return a.c.Log().With("category", applog.CatAuth) }

// ServiceShutdown 은 쥐고 있던 세션도 닫는다(서버 세션 회수).
func (a *AuthService) ServiceShutdown() error {
	a.dropPending()
	return a.base.ServiceShutdown()
}

// LoginInfo 는 로그인한 사용자와 접속한 망이다.
type LoginInfo struct {
	UserID          string        `json:"userId"`
	UserName        string        `json:"userName"`
	Dept            string        `json:"dept"`
	IsApprover      bool          `json:"isApprover"`      // 결재자(aprv_yn) ― [결재처리] 메뉴
	IsProxyApprover bool          `json:"isProxyApprover"` // 대리결재자(replace_yn) ― [대리결재자] 메뉴
	LogoutAutoMin   int           `json:"logoutAutoMin"`   // 자동 로그아웃(분), 0 = 끄지 않음
	Server          config.Server `json:"server"`
}

func loginInfo(s *nbs.Session) *LoginInfo {
	u := s.User()
	return &LoginInfo{
		UserID: u.UserID, UserName: u.UserName, Dept: u.Dept,
		IsApprover: u.IsApprover, IsProxyApprover: u.IsProxyApprover, LogoutAutoMin: u.LogoutAutoMin,
		Server: s.Server,
	}
}

// 로그인 뒤 물을 비밀번호 변경(LoginResult.Prompt).
const (
	PromptNone    = ""
	PromptDefault = "default" // 기본 비밀번호와 같다 → 변경 창(일반)
	PromptExpire  = "expire"  // 만료 예정 → 「패스워드 사용기간이 %1일 이후 만료예정입니다.…」 물음(RemainDays 가 %1)
	PromptForce   = "force"   // 기한 지남 → 강제 변경 창. 성공하면 CompleteLogin, [취소] 는 CancelLogin 뒤 종료
)

// LoginResult 는 로그인 결과다. Error 가 비면 성공이다.
type LoginResult struct {
	User       *LoginInfo `json:"user"`       // 성공이면 사용자(PromptForce 면 nil ― 아직 들이지 않았다)
	Error      string     `json:"error"`      // 화면 문구(로그인 화면 경고와 오류 창에 같이)
	Title      string     `json:"title"`      // 오류 창 제목(오류·중복 로그인·무결성 검증 실패)
	Exit       bool       `json:"exit"`       // 오류 창을 닫으면 앱 종료(미등록 ID·중복 로그인·무결성 실패)
	Prompt     string     `json:"prompt"`     // PromptNone·PromptDefault·PromptExpire·PromptForce
	RemainDays int        `json:"remainDays"` // PromptExpire 의 %1(서버 값 − 1, 원본과 같다)
}

// SavedLogin 은 로그인 화면을 처음 채울 값이다.
type SavedLogin struct {
	SaveID      bool   `json:"saveId"`
	UserID      string `json:"userId"`      // SaveID 가 거짓이면 빈 값
	ServerIndex int    `json:"serverIndex"` // Servers() 의 자리. 저장된 망 이름이 목록에 있으면 그 자리, 목록 밖이면 0
}

// Servers 는 fmenv.ini 의 서버 목록이다(로그인 화면의 망 콤보, fmlocal.ini 의 망이 맨 앞).
func (a *AuthService) Servers() ([]config.Server, error) {
	env, err := a.c.Env()
	if err != nil {
		return nil, err
	}
	return append([]config.Server{}, env.Servers...), nil
}

// SavedLogin 은 저장된 ID·망 순번이다.
func (a *AuthService) SavedLogin() SavedLogin {
	st := a.c.Settings().Get()
	out := SavedLogin{SaveID: st.SaveUserID, ServerIndex: st.ServerIndex}
	if st.SaveUserID {
		out.UserID = st.UserID
	}
	servers, _ := a.Servers()
	for _, s := range servers {
		if st.ServerName != "" && s.Name == st.ServerName {
			out.ServerIndex = s.Index
		}
	}
	if out.ServerIndex < 0 || out.ServerIndex >= len(servers) {
		out.ServerIndex = 0
	}
	return out
}

// SetServerIndex 는 콤보에서 고른 망을 곧바로 저장한다(로그인을 누르지 않아도, 09 5.1).
func (a *AuthService) SetServerIndex(index int) error {
	servers, err := a.Servers()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(servers) {
		return errors.New(nbs.MsgNoServer)
	}
	return a.c.Settings().Update(func(s *config.Settings) {
		s.ServerIndex, s.ServerName = index, servers[index].Name
	})
}

// SetSaveID 는 [ID 저장] 체크다. 끄면 저장된 ID 도 지운다.
func (a *AuthService) SetSaveID(save bool) error {
	return a.c.Settings().Update(func(s *config.Settings) {
		s.SaveUserID = save
		if !save {
			s.UserID = ""
		}
	})
}

// Login 은 serverIndex 자리의 망에 로그인한다. 실패 문구는 원본 그대로다(docs/port/01 3.2·3.3).
// 서버 주소는 화면이 보낸 값이 아니라 fmenv.ini 에서 읽은 목록에서 고른다. 오류는 LoginResult.Error 로 돌려준다.
func (a *AuthService) Login(serverIndex int, id, pw string) (*LoginResult, error) {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	id = strings.TrimSpace(id)
	if id == "" || pw == "" {
		return fail(msgRequired, false), nil
	}
	if err := nbs.CheckLength(id, pw); err != nil {
		return fail(err.Error(), false), nil
	}
	servers, err := a.Servers()
	if err != nil {
		return fail(nbs.MsgNoServer, false), nil
	}
	if serverIndex < 0 || serverIndex >= len(servers) {
		return fail(nbs.MsgNoServer, false), nil
	}
	srv := servers[serverIndex]
	a.dropPending()
	a.c.Logout(ReasonLogout)    // 앞 세션(다른 망)을 먼저 닫는다 ― 한 번에 망 하나
	a.c.restartIntegrityTimer() // 로그인 시도마다 주기 검사 시계를 다시 켠다(SEC-002)
	a.log().Info("doLogin: 접속 서버 인덱스 = " + strconv.Itoa(serverIndex) + " " + srv.Name)

	s, err := nbs.Login(a.c.AppDir(), srv, id, pw, a.c.Version(), a.c.Log().With("category", applog.CatGrpc))
	if err != nil {
		var le *nbs.LoginError
		if errors.As(err, &le) {
			return fail(le.Msg, le.Exit), nil
		}
		return fail(err.Error(), false), nil
	}
	// 무결성 검사는 중복 로그인 판정보다 먼저(SEC-001). 끝까지 기다려 실패면 메인 화면을 열지 않는다.
	if f := a.c.checkIntegrity(s, integrity.KindStart); len(f) > 0 {
		s.Close()
		return &LoginResult{Error: MsgIntegrity, Title: TitleIntegrity, Exit: true}, nil
	}
	if s.MultiLogin { // 다른 PC 세션은 그대로 둔다(AUTH-024)
		a.log().Warn("중복 로그인 >>> 다중 로그인을 허용하지 않는 계정이 이미 다른 PC 에서 로그인 중이다")
		s.Close()
		return &LoginResult{Error: MsgDuplicateLogin, Title: TitleDuplicate, Exit: true}, nil
	}

	res := &LoginResult{}
	switch {
	case s.DefaultPassword: // 잔여일을 보지 않는다(5.5)
		res.Prompt = PromptDefault
	case s.RemainDays < 0:
		res.Prompt = PromptForce
	case s.RemainDays > 0:
		res.Prompt, res.RemainDays = PromptExpire, s.RemainDays-1
	}
	if res.Prompt == PromptForce {
		a.mu.Lock()
		a.pending, a.pendID = s, id
		a.mu.Unlock()
		a.log().Info("비밀번호 기한 지남 >>> 변경할 때까지 메인 화면을 열지 않는다")
		return res, nil
	}
	a.admit(s, id)
	res.User = loginInfo(s)
	return res, nil
}

func fail(msg string, exit bool) *LoginResult {
	return &LoginResult{Error: msg, Title: TitleError, Exit: exit}
}

// admit 은 세션을 들이고 ID·망을 저장한다.
func (a *AuthService) admit(s *nbs.Session, id string) {
	a.c.login(s)
	if err := a.c.Settings().Update(func(st *config.Settings) {
		st.ServerIndex, st.ServerName = s.Server.Index, s.Server.Name
		st.UserID = ""
		if st.SaveUserID {
			st.UserID = id
		}
	}); err != nil {
		a.log().Warn("로그인 설정을 저장하지 못했다", "err", err)
	}
}

func (a *AuthService) takePending() (*nbs.Session, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, id := a.pending, a.pendID
	a.pending, a.pendID = nil, ""
	return s, id
}

func (a *AuthService) dropPending() {
	if s, _ := a.takePending(); s != nil {
		s.Close()
	}
}

// CompleteLogin 은 강제 비밀번호 변경에 성공한 뒤 쥐고 있던 세션을 들인다(메인 화면으로).
func (a *AuthService) CompleteLogin() (*LoginInfo, error) {
	s, id := a.takePending()
	if s == nil {
		return nil, ErrNotLoggedIn
	}
	a.admit(s, id)
	return loginInfo(s), nil
}

// CancelLogin 은 강제 변경 창의 [취소]다. 쥐고 있던 세션(TCP)을 닫는다 ― 화면은 이어서 앱을 끝낸다.
func (a *AuthService) CancelLogin() { a.dropPending() }

// Logout 은 로그아웃한다(연결을 끊는다). 로그아웃 처리 방식(종료·트레이)은 화면이 고른 뒤 따로 부른다.
func (a *AuthService) Logout() {
	a.dropPending()
	a.c.Logout(ReasonLogout)
}

// CurrentUser 는 지금 로그인한 사용자다. 없으면 nil(화면이 시작할 때 묻는 정상 경우라 오류로 내지 않는다).
func (a *AuthService) CurrentUser() (*LoginInfo, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, nil
	}
	return loginInfo(s), nil
}

// Touch 는 화면 입력(누름·키·휠·터치)이 있을 때 1초에 한 번쯤 부른다 ― 자동 로그아웃 시계를 되돌린다.
func (a *AuthService) Touch() { a.c.Touch() }

// Authority 는 사용자 정보 팝업의 전송 권한이다(docs/port/01 5.6). In = 들어오는 방향(인터넷망→업무망), Out = 나가는 방향.
type Authority struct {
	InName       string `json:"inName"`  // 들어오는 방향 글자(LOC 0 서버의 NETWORK, 예: 인터넷망->업무망)
	OutName      string `json:"outName"` // 나가는 방향 글자(LOC 1 서버의 NETWORK)
	FileIn       bool   `json:"fileIn"`
	FileOut      bool   `json:"fileOut"`
	FileInAprv   int32  `json:"fileInAprv"` // 결재 방식 0~7
	FileOutAprv  int32  `json:"fileOutAprv"`
	ClipIn       bool   `json:"clipIn"`
	ClipOut      bool   `json:"clipOut"`
	ClipInAprv   int32  `json:"clipInAprv"`
	ClipOutAprv  int32  `json:"clipOutAprv"`
	MaxFileSize  int64  `json:"maxFileSize"`  // 바이트, 0 = 무제한(서버 값은 KB)
	MaxFileCount int    `json:"maxFileCount"` // 0 = 무제한
	MaxClipSize  int64  `json:"maxClipSize"`  // 바이트, 0 = 무제한
}

// Authority 는 사용자 정보 팝업 값이다. 로그인 응답 필드 주석을 기준으로 한 번에 계산한다:
// fm_file_up 내부→외부, fm_file_down 외부→내부, fm_clip_down 클립보드 내부→외부, fm_clip_up 클립보드 외부→내부,
// fm_influp/inclup_aprv 내부망(나가는), fm_otflup/otclup_aprv 외부망(들어오는). 원본은 클립보드 업·다운을 두 번 바꿔
// 같은 결과가 된다(원본 확인 필요 10 ― loginrespolicy.cpp setAuthority·getAuthority, UserProfileInfo.qml).
func (a *AuthService) Authority() (*Authority, error) {
	s, err := a.c.Session()
	if err != nil {
		return nil, err
	}
	env, _ := a.c.Env()
	return authorityOf(s.Info, env), nil
}

func authorityOf(i interface {
	GetFmFileUp() int32
	GetFmFileDown() int32
	GetFmClipUp() int32
	GetFmClipDown() int32
	GetFmInflupAprv() int32
	GetFmOtflupAprv() int32
	GetFmInclupAprv() int32
	GetFmOtclupAprv() int32
	GetFmFileSize() int32
	GetFmFileCount() int32
	GetFmClipSize() int32
}, env *config.Env) *Authority {
	out := &Authority{
		FileOut: i.GetFmFileUp() != 0, FileIn: i.GetFmFileDown() != 0,
		ClipOut: i.GetFmClipDown() != 0, ClipIn: i.GetFmClipUp() != 0,
		FileOutAprv: i.GetFmInflupAprv(), FileInAprv: i.GetFmOtflupAprv(),
		ClipOutAprv: i.GetFmInclupAprv(), ClipInAprv: i.GetFmOtclupAprv(),
		MaxFileSize: int64(i.GetFmFileSize()) * 1024, MaxFileCount: int(i.GetFmFileCount()),
		MaxClipSize: int64(i.GetFmClipSize()) * 1024,
	}
	if env != nil {
		for _, s := range env.Servers {
			if s.Loc == config.LocInternet && out.InName == "" {
				out.InName = s.Network
			}
			if s.Loc == config.LocBusiness && out.OutName == "" {
				out.OutName = s.Network
			}
		}
	}
	return out
}

// ValidatePasswordChange 는 비밀번호 변경 창의 [확인] 검사다(원본 PasswordChangeDlg.qml:178-206 순서·문구). 통과면 "".
// 화면도 같은 검사를 하지만, 서버로 보내기 전 Go 에서도 본다.
func ValidatePasswordChange(cur, next, confirm string) string {
	switch {
	case cur == "":
		return MsgPwCurEmpty
	case next == "":
		return MsgPwNewEmpty
	case next == cur:
		return MsgPwSame
	case confirm == "":
		return MsgPwConfirmNone
	case next != confirm:
		return MsgPwMismatch
	case !pwRuleOK(confirm):
		return MsgPwRule
	}
	return ""
}

// pwRuleOK 는 ^(?=.*[A-Za-z])(?=.*\d)(?=.*[@$!%*#?&])[A-Za-z\d@$!%*#?&]{10,16}$ 이다(RE2 는 앞보기가 없어 손으로 본다).
func pwRuleOK(p string) bool {
	if len(p) < 10 || len(p) > 16 {
		return false
	}
	var letter, digit, special bool
	for _, r := range p {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			letter = true
		case r >= '0' && r <= '9':
			digit = true
		case strings.ContainsRune("@$!%*#?&", r):
			special = true
		default:
			return false
		}
	}
	return letter && digit && special
}

// ChangePassword 는 비밀번호 변경[3002]이다. 성공이면 nil(화면이 「비밀번호가 정상적으로 변경되었습니다.」),
// 실패면 화면에 그대로 낼 문구(검사 문구·서버 사유·gRPC 오류). 기한 지남으로 쥐고 있는 세션이 있으면 그 세션으로 보낸다.
func (a *AuthService) ChangePassword(current, next, confirm string) error {
	if msg := ValidatePasswordChange(current, next, confirm); msg != "" {
		return errors.New(msg)
	}
	a.mu.Lock()
	s := a.pending
	a.mu.Unlock()
	if s == nil {
		var err error
		if s, err = a.c.Session(); err != nil {
			return err
		}
	}
	if err := s.ChangePassword(current, next); err != nil {
		a.log().Warn("비밀번호 변경 실패", "user", s.UserID, "reason", err.Error())
		return err
	}
	a.log().Info("비밀번호 변경 성공", "user", s.UserID)
	return nil
}
