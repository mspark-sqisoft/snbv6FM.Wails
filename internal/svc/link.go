package svc

// 링크·도메인·두 번째 실행 영역(docs/port/07 7.6~7.9): Link Redirect 창구, URL 필터, 알림 9004 kind 4, 실행 인자, hosts 도메인 정책.
// 화면에서 부르는 메서드는 없고 Wails 서비스 수명만 쓴다.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/domain"
	"snbapp/internal/link"
	"snbapp/internal/nbs"
	"snbapp/internal/notify"
	pbn "snbapp/pb/notify"
)

const (
	argBatch     = 400 * time.Millisecond // 탐색기가 여러 파일을 프로세스 하나씩 띄운다 ― 이만큼 묶는다
	uploadRetry  = 75                     // 앞 업로드가 답을 기다리면 argBatch 간격으로(약 30초)
	urlDedup     = 2 * time.Second
	msgLinkFatal = "링크연동을 사용할 수 없습니다 - 나머지 기능은 정상 동작합니다."
)

func init() {
	addService(func(c *Core) application.Service { return application.NewService(newLinkService(c)) })
}

type argJob struct {
	mode  string
	paths []string
}

// LinkService 는 앱 수명 동안 도는 일들이다.
type LinkService struct {
	base
	server *link.Server
	dedup  link.Dedup

	mu     sync.Mutex
	filter []string // URL 필터 [3094]

	argMu sync.Mutex
	batch argJob      // 묶는 중
	timer *time.Timer // 묶음 시계
	held  []argJob    // 로그인을 기다리는 일

	hostsWarned atomic.Bool
}

// links 는 앱에 하나인 LinkService 다(정책 화면이 URL 필터를 읽는다).
var links *LinkService

func newLinkService(c *Core) *LinkService {
	l := &LinkService{base: base{c}}
	links = l
	c.Notify().Register(notify.SvcEvent, notify.KindURLSend, l.onURL)
	c.OnSecondInstance(func(d application.SecondInstanceData) {
		if len(d.Args) > 1 {
			l.onArgs(d.Args[1:])
		}
	})
	c.OnLogin(l.onLogin)
	c.OnLogout(func() {
		l.mu.Lock()
		l.filter = nil
		l.mu.Unlock()
	})
	return l
}

func (l *LinkService) log() *slog.Logger { return l.c.Log().With("category", applog.CatApp) }

// ServiceStartup 은 창구를 열고 첫 실행의 인자를 처리한다.
func (l *LinkService) ServiceStartup(ctx context.Context, o application.ServiceOptions) error {
	l.base.ServiceStartup(ctx, o)
	srv, err := link.Start(l.log(), link.Port, l.handle)
	if err != nil {
		l.log().Warn(msgLinkFatal, "err", err)
	}
	l.server = srv
	if app := application.Get(); app != nil { // 정책을 받을 때마다(로그인·[정책 받아오기]) 필터도 받는다
		app.Event.On(EvPolicyChanged, func(*application.CustomEvent) { go l.loadFilter() })
	}
	if len(os.Args) > 1 {
		l.onArgs(os.Args[1:])
	}
	return nil
}

// ServiceShutdown 은 창구를 닫고 hosts 의 우리 구역을 지운다. 끌 때는 안내하지 않는다.
func (l *LinkService) ServiceShutdown() error {
	l.server.Close()
	if l.domainOn() {
		if path, err := domain.HostsPath(); err == nil {
			partial, err := domain.Remove(path)
			if partial {
				l.log().Warn("도메인 삭제 >>> 끝 표식이 없는 구역을 지웠다(넣다가 끊긴 것)")
			}
			if err != nil {
				l.log().Warn("도메인 삭제 - Hosts 파일 열기에 실패했습니다.", "err", err)
			}
		}
	}
	return l.base.ServiceShutdown()
}

func (l *LinkService) onLogin(s *nbs.Session) {
	l.argMu.Lock()
	held := l.held
	l.held = nil
	l.argMu.Unlock()
	for _, j := range held {
		go l.run(j)
	}
	if s.IsBusiness() && l.domainOn() {
		go l.applyDomain(s)
	}
}

// ── Link Redirect ──

// handle 은 확장 요청의 본문을 판정한다. 보낼 주소면 뒤에서 보내고 거짓(205)을 돌려준다.
func (l *LinkService) handle(body string) bool {
	s, err := l.c.Session()
	if err != nil {
		return true
	}
	st := l.c.PolicyChecker().Current()
	if st == nil {
		return true
	}
	mode, _ := st.Value("fm_hyperlink_mode")
	method, _ := st.Value("url_check_method")
	web := link.Decide(body, link.Rule{Business: s.IsBusiness(), Mode: mode, Method: method, Filter: l.urlFilter()},
		func(w string) { l.log().Warn(w) })
	if web == "" {
		return true
	}
	if l.dedup.First(web, time.Now(), urlDedup) { // 하위 프레임마다 온다 ― 보내기만 한 번, 응답은 같다
		go l.sendURL(s, web)
	}
	return false
}

func (l *LinkService) sendURL(s *nbs.Session, web string) {
	host := ""
	if u, err := url.Parse(web); err == nil {
		host = u.Hostname()
	}
	l.log().Info(fmt.Sprintf("인터넷망으로 URL전송[%s]", host))
	err := link.SendURL(s, web)
	switch {
	case errors.Is(err, link.ErrMalware):
		l.c.Emit(EvDialog, Dialog{ID: l.c.dialogSeq.Add(1), Title: "요청 처리 오류", Text: err.Error()})
	case err != nil:
		l.log().Warn("LinkRedirect URL 전송[3204] 실패!!!", "err", err)
	}
}

// onURL 은 업무망에서 온 URL(알림 9004 kind 4)을 기본 브라우저로 연다.
func (l *LinkService) onURL(m *pbn.NotifyMessage) {
	web := strings.TrimSpace(m.Dat)
	if !link.IsWebURL(web) {
		l.log().Warn("업무망에서 전달된 URL 을 열지 않았다(http/https 주소가 아니다)")
		return
	}
	u, _ := url.Parse(web)
	l.log().Info("업무망에서 전달된 URL 수신:", "host", u.Hostname())
	app := application.Get()
	if app == nil {
		return
	}
	go func() {
		if err := app.Browser.OpenURL(web); err != nil {
			l.log().Warn("브라우저를 열지 못했다", "err", err)
		}
	}()
}

func (l *LinkService) urlFilter() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.filter
}

func (l *LinkService) loadFilter() {
	s, err := l.c.Session()
	if err != nil {
		return
	}
	gen := l.c.Gen()
	list, err := link.FetchFilter(s)
	if err != nil {
		l.log().Warn("LinkRedirect UrlFilter 목록 조회[3094] 실패!!!", "err", err)
		return
	}
	if !l.c.IsCurrent(gen) {
		return
	}
	l.mu.Lock()
	l.filter = list
	l.mu.Unlock()
	l.log().Info(fmt.Sprintf("LinkRedirect UrlFilter 목록 조회[3095] 응답 건수: %d", len(list)))
}

// LinkPolicyRows 는 정책 화면 LinkRedirect 분기의 행이다(URL 필터 한 건이 한 행).
func (c *Core) LinkPolicyRows() []PolicyRow {
	if links == nil {
		return nil
	}
	list := links.urlFilter()
	rows := make([]PolicyRow, 0, len(list))
	for _, u := range list {
		rows = append(rows, PolicyRow{Type: "LinkRedirect", Name: u})
	}
	return rows
}

// ── 두 번째 실행 인자 ──

// onArgs 는 --mode upload|download 경로들을 묶는다. 모드가 바뀌면 앞 묶음을 바로 내보낸다.
func (l *LinkService) onArgs(args []string) {
	mode, paths := link.ParseArgs(args)
	if (mode != "upload" && mode != "download") || len(paths) == 0 {
		return
	}
	l.argMu.Lock()
	defer l.argMu.Unlock()
	if l.batch.mode != mode && len(l.batch.paths) > 0 {
		go l.run(l.batch)
		l.batch.paths = nil
	}
	l.batch.mode = mode
	l.batch.paths = append(l.batch.paths, paths...)
	if l.timer == nil {
		l.timer = time.AfterFunc(argBatch, l.flushArgs)
	} else {
		l.timer.Reset(argBatch)
	}
}

func (l *LinkService) flushArgs() {
	l.argMu.Lock()
	j := l.batch
	l.batch = argJob{}
	l.argMu.Unlock()
	if len(j.paths) > 0 {
		l.run(j)
	}
}

// run 은 인자 하나를 처리한다. 로그인 전이면 보관했다가 첫 로그인 뒤 다시 부른다.
func (l *LinkService) run(j argJob) {
	if _, err := l.c.Session(); err != nil {
		l.argMu.Lock()
		l.held = append(l.held, j)
		l.argMu.Unlock()
		text := fmt.Sprintf("로그인하면 파일 %d 개를 올립니다", len(j.paths))
		if j.mode == "download" {
			text = "로그인하면 고른 파일을 이 폴더로 받습니다"
		}
		l.c.Emit(EvNotice, Notice{Severity: SevInfo, Text: text})
		return
	}
	if j.mode == "download" { // 받을 행은 화면이 쥐고 있다 ― 받을 폴더만 넘긴다
		l.c.Emit(EvSecondInstanceArgs, map[string]string{"mode": "download", "path": j.paths[len(j.paths)-1]})
		return
	}
	t := l.c.Transfers()
	var err error
	for i := 0; i < uploadRetry; i++ {
		if err = t.UploadPaths(j.paths, false); !errors.Is(err, ErrUploadBusy) {
			break
		}
		time.Sleep(argBatch)
	}
	if errors.Is(err, ErrUploadBusy) {
		l.log().Warn("IPC 업로드 대기 한도 초과")
	}
	if err != nil {
		l.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: err.Error()})
	}
}

// ── 도메인 정책 ──

// domainOn 은 fmenv.ini [CONFIG] DOMAIN_POLICY 다.
func (l *LinkService) domainOn() bool {
	env, err := l.c.Env()
	if err != nil || env == nil {
		return false
	}
	v, _ := env.Value("CONFIG", "DOMAIN_POLICY")
	v = strings.ToLower(strings.TrimSpace(v))
	return v != "" && v != "0" && v != "false"
}

// hostsFailed 는 hosts 를 못 고쳤을 때다 ― 관리자 권한 없이 실행하면 매번 그러므로 안내는 앱을 켜 둔 동안 한 번만 한다.
func (l *LinkService) hostsFailed(text string, err error) {
	l.log().Warn(text, "err", err)
	if l.hostsWarned.CompareAndSwap(false, true) {
		l.c.Emit(EvDialog, Dialog{ID: l.c.dialogSeq.Add(1), Title: "정보", Text: text})
	}
}

func (l *LinkService) applyDomain(s *nbs.Session) {
	log := l.log()
	list, err := domain.Fetch(s)
	if err != nil {
		log.Warn("도메인 정책 조회[3230] 실패!!!", "err", err)
		return
	}
	rows := domain.Rows(list, func(ip, name string) {
		log.Warn("도메인 등록 >>> IP·이름 형식이 아니라 hosts 에 쓰지 않았다:", "ip", ip, "name", name)
	})
	if len(rows) == 0 {
		return
	}
	path, err := domain.HostsPath()
	if err != nil {
		l.hostsFailed(err.Error(), err)
		return
	}
	partial, err := domain.Apply(path, rows)
	if partial {
		log.Warn("도메인 등록 >>> 끝 표식이 없는 옛 구역을 지웠다(넣다가 끊긴 것)")
	}
	if err != nil {
		l.hostsFailed("도메인 등록 - Hosts 파일 열기에 실패했습니다.", err)
		return
	}
	log.Info(fmt.Sprintf("도메인 등록 >>> hosts 에 %d 건을 썼다", len(rows)))
}
