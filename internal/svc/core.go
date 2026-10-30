// Package svc 는 Wails 에 묶는 서비스(화면이 부르는 메서드)다. 서비스마다 파일 하나, 모두 *Core 하나를 공유한다.
package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/integrity"
	"snbapp/internal/nbs"
)

// ErrNotLoggedIn 은 로그인하지 않은 상태에서 세션이 필요한 메서드를 부를 때다.
var ErrNotLoggedIn = errors.New("로그인하지 않았습니다")

// ErrNotImplemented 는 아직 채우지 않은 메서드다(영역 담당이 채운다).
var ErrNotImplemented = errors.New("아직 구현하지 않았습니다")

func notImplemented(method string) error { return fmt.Errorf("%s: %w", method, ErrNotImplemented) }

// Core 는 모든 서비스가 함께 쓰는 상태다: 세션, 설정, 로거, 로그인·로그아웃 때 할 일, 자동 로그아웃 시계.
// 세대 번호(Gen)는 로그인·로그아웃마다 오른다 ― 늦게 온 응답·알림·타이머가 IsCurrent(gen) 으로 자기가 낡았는지 안다.
type Core struct {
	version string
	log     *slog.Logger

	startOnce, stopOnce sync.Once
	appDir              string
	env                 *config.Env
	envErr              error
	settings            *config.Store

	mu        sync.Mutex
	sess      *nbs.Session
	gen       uint64
	idle      *time.Timer
	idleLimit time.Duration
	onLogin   []func(*nbs.Session)
	onLogout  []func()
	onSecond  []func(application.SecondInstanceData)
	integStop chan struct{} // 무결성 주기 검사 고루틴을 멈춘다
}

// Dialog.OnClose 값 ― 확인 창을 닫은 뒤 화면이 할 일.
const (
	DialogExit          = "exit"          // 앱 종료(중복 로그인·무결성 실패·미등록 ID)
	DialogLogoutProcess = "logoutProcess" // 로그아웃 처리 방식대로(연결 끊김)
)

// NewCore 는 Core 를 만든다. 파일 읽기는 ServiceStartup 에서 한다.
func NewCore(version string, log *slog.Logger) *Core {
	return &Core{version: version, log: log}
}

// extraServices 는 서비스 파일이 core.go 를 고치지 않고 스스로 더한 서비스다:
//
//	func init() { addService(func(c *Core) application.Service { return application.NewService(newLinkService(c)) }) }
var extraServices []func(*Core) application.Service

func addService(mk func(*Core) application.Service) { extraServices = append(extraServices, mk) }

// Services 는 Wails 에 등록할 서비스다. 서비스 생성자(newXxxService)에서 OnLogin·OnLogout 등을 등록하고,
// 영역 사이 연결은 모두 만든 뒤 wire(wiring.go)가 잇는다.
func (c *Core) Services() []application.Service {
	list := []application.Service{
		application.NewService(newAuthService(c)),
		application.NewService(newFilesService(c)),
		application.NewService(newHistoryService(c)),
		application.NewService(newTransferService(c)),
		application.NewService(newPolicyService(c)),
		application.NewService(newApprovalService(c)),
		application.NewService(newClipService(c)),
		application.NewService(newLogService(c)),
		application.NewService(newLocalService(c)),
		application.NewService(newSettingsService(c)),
	}
	for _, mk := range extraServices {
		list = append(list, mk(c))
	}
	c.wire()
	return list
}

// base 는 모든 서비스에 묻어 들어가 ServiceStartup·ServiceShutdown 을 Core 한 번으로 모은다.
// 영역 서비스가 ServiceStartup 을 직접 두려면 맨 앞에서 s.base.ServiceStartup(ctx, o) 를 부른다.
type base struct{ c *Core }

func (b base) ServiceStartup(context.Context, application.ServiceOptions) error {
	b.c.startOnce.Do(b.c.startup)
	return nil
}

func (b base) ServiceShutdown() error {
	b.c.stopOnce.Do(func() {
		b.c.mu.Lock()
		if b.c.integStop != nil {
			close(b.c.integStop)
			b.c.integStop = nil
		}
		b.c.mu.Unlock()
		b.c.Logout(ReasonShutdown) // 모든 서버 로그아웃(AUTH-034)
	})
	return nil
}

func (c *Core) startup() {
	c.appDir = config.AppDir()
	c.env, c.envErr = config.LoadEnv(c.appDir)
	if c.envErr != nil {
		c.log.Error("서버 설정을 읽지 못했다", "category", applog.CatApp, "err", c.envErr)
	}
	dataDir := config.DataDir()
	st, err := config.Open(filepath.Join(dataDir, config.SettingsFile))
	if err != nil {
		c.log.Warn(err.Error(), "category", applog.CatApp)
	}
	c.settings = st
	if err := config.MigrateLegacy(st, c.appDir, dataDir); err != nil {
		c.log.Warn("옛 설정을 옮기지 못했다", "category", applog.CatApp, "err", err)
	}
	st.OnChange(func(_, cur config.Settings) { c.Emit(EvSettingsChanged, cur) })
	c.log.Info("프로그램이 실행되었습니다.", "category", applog.CatApp, "version", c.version, "appDir", c.appDir, "dataDir", dataDir)
}

// Version 은 앱 버전(로그인 요청 app_version)이다.
func (c *Core) Version() string { return c.version }

// Log 는 앱 로거다.
func (c *Core) Log() *slog.Logger { return c.log }

// Settings 는 사용자 설정 저장소다(ServiceStartup 뒤부터 쓴다).
func (c *Core) Settings() *config.Store { return c.settings }

// AppDir 는 fmenv.ini·root_ca.pem 이 있는 폴더다.
func (c *Core) AppDir() string { return c.appDir }

// Env 는 시작 때 읽은 fmenv.ini·fmlocal.ini 다. fmenv.ini 를 못 읽었으면 그 오류다.
func (c *Core) Env() (*config.Env, error) { return c.env, c.envErr }

// Emit 은 화면에 사건을 보낸다. 앱이 없으면(시험) 아무것도 안 한다.
func (c *Core) Emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// Session 은 로그인한 세션이다. 없으면 ErrNotLoggedIn.
func (c *Core) Session() (*nbs.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess == nil {
		return nil, ErrNotLoggedIn
	}
	return c.sess, nil
}

// setProfile 은 다시 받은 권한 프로파일을 세션에 넣는다. Session 은 잠금 없이 읽으므로 사본으로 바꾼다 ―
// 이미 잡은 세션(진행 중인 전송)은 옛 값, 다음 Session() 부터 새 값이다.
func (c *Core) setProfile(gen uint64, m map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess != nil && c.gen == gen {
		ns := *c.sess
		ns.Profile = m
		c.sess = &ns
	}
}

// Gen 은 지금 세대 번호다. 오래 걸리는 일을 시작할 때 받아 두고, 끝나서 결과를 낼 때 IsCurrent 로 확인한다.
func (c *Core) Gen() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen
}

// IsCurrent 는 gen 세대의 로그인이 아직 살아 있는가다.
func (c *Core) IsCurrent(gen uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sess != nil && c.gen == gen
}

// Later 는 d 뒤에 fn 을 부르되, 그 사이 로그아웃(또는 다시 로그인)했으면 부르지 않는다(알림 9001·9002 의 1초 뒤 목록 조회 등).
func (c *Core) Later(d time.Duration, fn func(*nbs.Session)) {
	gen := c.Gen()
	time.AfterFunc(d, func() {
		c.mu.Lock()
		s := c.sess
		ok := s != nil && c.gen == gen
		c.mu.Unlock()
		if ok {
			fn(s)
		}
	})
}

// OnLogin 은 로그인 성공 뒤 부를 일을 더한다(정책 받기, 알림 스트림, 자동 전송, 단축키 등). 등록 순서대로 부른다.
func (c *Core) OnLogin(fn func(*nbs.Session)) {
	c.mu.Lock()
	c.onLogin = append(c.onLogin, fn)
	c.mu.Unlock()
}

// OnLogout 은 로그아웃 때 연결을 닫기 전에 부를 일을 더한다(알림 스트림 끊기, 전송 취소, 타이머 정지 등).
func (c *Core) OnLogout(fn func()) {
	c.mu.Lock()
	c.onLogout = append(c.onLogout, fn)
	c.mu.Unlock()
}

// OnSecondInstance 는 두 번째 실행의 인자를 받을 일을 더한다(--mode upload|download <경로> ― 클립보드·링크 담당).
func (c *Core) OnSecondInstance(fn func(application.SecondInstanceData)) {
	c.mu.Lock()
	c.onSecond = append(c.onSecond, fn)
	c.mu.Unlock()
}

// SecondInstance 는 main 이 단일 인스턴스 콜백에서 부른다.
func (c *Core) SecondInstance(d application.SecondInstanceData) {
	c.mu.Lock()
	hooks := append([]func(application.SecondInstanceData){}, c.onSecond...)
	c.mu.Unlock()
	c.log.Info("두 번째 실행", "category", applog.CatApp, "args", d.Args)
	for _, h := range hooks {
		h(d)
	}
}

// login 은 새 세션을 들인다(AuthService 만 부른다). 세대를 올리고, 자동 로그아웃 시계를 켜고, OnLogin 을 부른다.
func (c *Core) login(s *nbs.Session) {
	c.mu.Lock()
	old := c.sess
	c.stopIdleLocked()
	c.sess = s
	c.gen++
	if min := int(s.Info.FmLogoutAuto); min > 0 {
		gen := c.gen
		c.idleLimit = time.Duration(min) * time.Minute
		c.idle = time.AfterFunc(c.idleLimit, func() { c.idleFire(gen, min) })
	}
	hooks := append([]func(*nbs.Session){}, c.onLogin...)
	c.mu.Unlock()
	if old != nil { // 로그인 두 번이 겹쳤을 때 앞 세션
		old.Close()
	}
	for _, h := range hooks {
		h(s)
	}
}

// Logout 은 세션을 정리하고 EvSessionEnded 를 낸다. 로그인 상태가 아니면 아무것도 안 한다.
// 순서는 원본과 같다(docs/port/01 7.10): 로그인 상태 내림·세대 올림 → 시계 정지 → OnLogout(스트림·전송) → TCP 닫기 → 화면.
func (c *Core) Logout(reason string) {
	c.mu.Lock()
	if c.sess == nil {
		c.mu.Unlock()
		return
	}
	c.logoutLocked(reason)
}

// LogoutIf 는 gen 세대의 로그인이 살아 있을 때만 로그아웃한다. 늦게 온 끊김·시계가 새 세션을 끊지 않게 판정과 내림을 잠금 하나로 묶는다.
func (c *Core) LogoutIf(gen uint64, reason string) bool {
	c.mu.Lock()
	if c.sess == nil || c.gen != gen {
		c.mu.Unlock()
		return false
	}
	c.logoutLocked(reason)
	return true
}

// logoutLocked 는 c.mu 를 쥔 채 들어와 풀고 나간다(세션은 nil 이 아니다).
func (c *Core) logoutLocked(reason string) {
	s := c.sess
	c.sess = nil
	c.gen++
	c.stopIdleLocked()
	hooks := append([]func(){}, c.onLogout...)
	c.mu.Unlock()
	for _, h := range hooks {
		h()
	}
	s.Close() // 서버에 로그아웃 RPC 가 없다 ― 연결을 닫아야 서버가 세션을 회수한다
	c.log.Info("로그아웃", "category", applog.CatAuth, "user", s.UserID, "reason", reason)
	c.Emit(EvSessionEnded, SessionEnded{Reason: reason})
}

// LogoutNotice 는 로그아웃한 뒤 확인 창을 띄운다(창 닫힘 뒤 할 일은 onClose: "" · DialogExit · DialogLogoutProcess).
// 로그인 상태가 아니었으면 아무것도 안 하고 거짓 ― 앞 세션의 늦은 알림이 창을 띄우지 않는다.
// 예: 알림 9004 kind 6·7 → c.LogoutNotice(ReasonDuplicate, TitleDuplicate, MsgDuplicateKick, DialogExit).
// 종료는 언제나 창이 닫힌 뒤 화면이 한다(docs/port/01 5.7).
func (c *Core) LogoutNotice(reason, title, text, onClose string) bool {
	if _, err := c.Session(); err != nil {
		return false
	}
	c.Logout(reason) // EvSessionEnded 가 먼저 가서 화면이 열린 창을 닫은 뒤 이 창이 뜬다
	c.dialog(title, text, onClose)
	return true
}

// CheckIntegrity 는 무결성 검사를 고루틴으로 한 번 한다(kind: integrity.KindPeriodic 주기, integrity.KindAdmin 알림 9005).
// 로그인 상태가 아니면 건너뛴다. 통과하면 아무것도 띄우지 않는다. 실패면 설치본(integrity.Enforced)만
// 로그아웃 → 「무결성 검증 실패」 창 → 닫으면 종료, 개발 빌드는 로그만 남긴다.
func (c *Core) CheckIntegrity(kind int) {
	c.mu.Lock()
	s, gen := c.sess, c.gen
	c.mu.Unlock()
	if s == nil {
		c.log.Info("무결성검사 건너뜀 ― 로그인 상태가 아니다. 검사유형: "+strconv.Itoa(kind), "category", applog.CatGrpc)
		return
	}
	go func() {
		if f := c.checkIntegrity(s, kind); len(f) > 0 && c.IsCurrent(gen) {
			c.LogoutNotice(ReasonIntegrity, TitleIntegrity, MsgIntegrity, DialogExit)
		}
	}()
}

// checkIntegrity 는 검사 한 번이다. 앱을 끝내야 할 실패(설치본)만 돌려준다.
func (c *Core) checkIntegrity(s *nbs.Session, kind int) []string {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	log := c.log.With("category", applog.CatGrpc)
	failures, _ := integrity.Check(ctx, s.Common(), integrity.Files(c.appDir), kind, s.Server.Loc, log)
	if len(failures) > 0 && !integrity.Enforced() {
		log.Warn("무결성 검증 실패( " + strings.Join(failures, "; ") + " ) ― 개발 빌드라 종료하지 않는다")
		return nil
	}
	return failures
}

// restartIntegrityTimer 는 3시간 주기 검사 시계를 다시 켠다(로그인 시도마다, 앱이 끝날 때만 멈춘다 ― SEC-002).
func (c *Core) restartIntegrityTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.integStop != nil {
		close(c.integStop)
	}
	stop := make(chan struct{})
	c.integStop = stop
	period := integrity.Period()
	go func() {
		t := time.NewTicker(period)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				c.CheckIntegrity(integrity.KindPeriodic)
			}
		}
	}()
}

// Touch 는 자동 로그아웃 시계를 되돌린다(화면 입력·전송 진행 보고).
func (c *Core) Touch() {
	c.mu.Lock()
	if c.idle != nil {
		c.idle.Reset(c.idleLimit)
	}
	c.mu.Unlock()
}

func (c *Core) stopIdleLocked() {
	if c.idle != nil {
		c.idle.Stop()
		c.idle = nil
	}
}

// idleFire 는 자동 로그아웃이다. 로그아웃 방식(종료·트레이)과 무관하게 로그인 화면으로 간다(AUTH-029).
func (c *Core) idleFire(gen uint64, min int) {
	if c.LogoutIf(gen, ReasonIdle) {
		c.Emit(EvAutoLogout, AutoLogout{Minutes: min})
	}
}
