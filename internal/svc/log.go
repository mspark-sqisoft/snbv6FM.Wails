package svc

// 알림·로그 영역(docs/port/09): 서버 알림 스트림과 기본 처리, 알림 처리기 등록(c.Notify()), 앱 로그 계정·보관,
// [로그] 화면 조회, 화면 로그 받기, [로그 저장], GUI 멈춤 기록.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/nbs"
	"snbapp/internal/notify"
	pbn "snbapp/pb/notify"
)

// LogService 는 [로그] 화면과 화면 쪽 로그가 부른다.
type LogService struct {
	base
	freezeOnce sync.Once
}

// notifyHub 는 Core 하나의 알림 상태다. Core 를 고치지 않으려고 이 파일에서 Core 열쇠로 따로 둔다.
type notifyHub struct {
	d   *notify.Dispatcher
	deb *notify.Debouncer

	mu      sync.Mutex
	cancel  context.CancelFunc // 지금 스트림
	balloon func(title, body string) error
}

var hubs sync.Map // *Core → *notifyHub

func hubOf(c *Core) *notifyHub {
	v, _ := hubs.LoadOrStore(c, &notifyHub{d: notify.NewDispatcher(), deb: notify.NewDebouncer(time.Second)})
	return v.(*notifyHub)
}

// Notify 는 서버 알림 처리기 등록소다. 영역은 서비스 생성자에서 등록한다:
//
//	c.Notify().Register(notify.SvcEvent, notify.KindURLSend, func(m *pbn.NotifyMessage) { … m.Dat … }) // 링크
//	c.Notify().Register(notify.SvcIntegrity, notify.AnyKind, func(*pbn.NotifyMessage) { … })          // 무결성
//
// 처리기는 알림 스트림 goroutine 에서 불린다(오래 걸리면 go 로 띄운다). 로그아웃 뒤 온 알림·남의 userid 알림·자가 승인은
// 처리기에 닿기 전에 버린다. 기본 처리(목록 갱신·결재 알림·중복 로그인·공지)는 이 영역이 먼저 등록해 둔다.
func (c *Core) Notify() *notify.Dispatcher { return hubOf(c).d }

// SendNotify 는 서버로 알림 하나를 보낸다(NotifyOnce). 예: 결재 뒤 notify.ApprovalDone(요청자, 나, 승인?).
func (c *Core) SendNotify(m *pbn.NotifyMessage) error {
	if m == nil {
		return nil
	}
	s, err := c.Session()
	if err != nil {
		return err
	}
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	c.log.Debug(fmt.Sprintf("서버로 Notify 메시지 전송 >>> 전문번호[%d] kind[%d] userid[%s] dat[%s]", m.NotifySvcno, m.Kind, m.Userid, m.Dat), "category", applog.CatGrpc)
	if _, err = s.Notify().NotifyOnce(ctx, m); err != nil {
		c.log.Warn("Notify 메시지 전송 실패", "category", applog.CatGrpc, "err", err)
	}
	return err
}

// SetBalloon 은 트레이 풍선(운영체제 알림)을 띄우는 함수를 둔다(main 이 Wails notifications 서비스로 채운다).
// 없거나 실패하면 결재 요청은 알림 띠, 결재 완료는 확인 창으로 대신한다(원본: 트레이가 없을 때와 같다).
func (c *Core) SetBalloon(fn func(title, body string) error) {
	h := hubOf(c)
	h.mu.Lock()
	h.balloon = fn
	h.mu.Unlock()
}

// Balloon 은 트레이 풍선을 띄운다. 못 띄우면 false.
func (c *Core) Balloon(title, body string) bool {
	h := hubOf(c)
	h.mu.Lock()
	fn := h.balloon
	h.mu.Unlock()
	if fn == nil {
		return false
	}
	if err := fn(title, body); err != nil {
		c.log.Debug("트레이 알림을 띄우지 못했다", "category", applog.CatApp, "err", err)
		return false
	}
	return true
}

// RetainLogs 는 로그 보관 정리다(LOG-026): 일수는 정책 remove_logfile 이 있으면 그것, 없으면 fmlocal.ini [LOG] KEEP_DAYS(15).
// 로그인 때 이 영역이 부르고, 정책을 다시 받을 때(policy.go load)도 부른다.
func (c *Core) RetainLogs(profile map[string]string) {
	keep, maxMB := applog.Defaults()
	src := "fmlocal.ini·기본값"
	if v, ok := profile["remove_logfile"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			keep, src = n, "서버 정책"
		}
	}
	removed := applog.Retain(keep, maxMB)
	tail := ""
	if len(removed) > 0 {
		tail = ": " + strings.Join(removed, ", ")
	}
	c.log.Info(fmt.Sprintf("로그 보관 >>> %d일(%s), 상한 %dMB, 지운 파일 %d개%s", keep, src, maxMB, len(removed), tail), "category", applog.CatApp)
}

func newLogService(c *Core) *LogService {
	applog.SetVersion(c.Version())
	l := &LogService{base: base{c}}
	h := hubOf(c)
	l.registerDefaults(h)
	c.OnLogin(func(s *nbs.Session) {
		applog.SetAccount(s.UserID + "@" + s.Server.Name)
		c.RetainLogs(s.Profile)
		l.startStream(h, s)
	})
	c.OnLogout(func() { // 기다리지 않는다 ― 스트림 goroutine 안에서 Logout 이 불릴 수 있다
		applog.SetAccount("")
		h.deb.Stop()
		h.mu.Lock()
		if h.cancel != nil {
			h.cancel()
			h.cancel = nil
		}
		h.mu.Unlock()
	})
	return l
}

// ServiceShutdown 은 세션을 정리하고(base) 종료 줄을 남긴 뒤 접힌 반복을 쓴다. 파일은 main 이 applog.Close 로 닫는다.
func (l *LogService) ServiceShutdown() error {
	l.base.ServiceShutdown()
	l.c.log.Info("앱 종료 >>> 서비스 종료", "category", applog.CatApp)
	applog.Flush()
	return nil
}

// startStream 은 로그인한 세션의 알림 스트림을 연다. 스트림이 끝났을 때 그 세션이 아직 살아 있으면 끊김이다.
func (l *LogService) startStream(h *notifyHub, s *nbs.Session) {
	c := l.c
	gen := c.Gen()
	ctx, cancel := s.Ctx(0)
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.cancel = cancel
	h.mu.Unlock()
	go func() {
		defer cancel()
		notify.Run(ctx, s.Notify(), c.log, func(m *pbn.NotifyMessage) { l.onNotify(h, gen, s.UserID, m) })
		// 로그아웃 먼저: 화면은 session:ended 에서 열린 창을 모두 닫으므로, 끊김 안내는 그 뒤에 보내야 남는다.
		if !c.LogoutIf(gen, ReasonDisconnected) {
			return // 우리가 로그아웃해서 끊었다
		}
		c.log.Debug(fmt.Sprintf("gRPC notify 스트림 비정상 종료 감지 (서버: %d )", s.Server.Index), "category", applog.CatAuth)
		c.log.Debug(fmt.Sprintf("DISCONNECTED SOCKET >>> 로그인 상태에서 연결 끊김. 서버: %d", s.Server.Index), "category", applog.CatAuth)
		c.Emit(EvDisconnected, nil)
	}()
}

// onNotify 는 알림 하나를 거른 뒤 등록된 처리기에 나눠 준다.
func (l *LogService) onNotify(h *notifyHub, gen uint64, myID string, m *pbn.NotifyMessage) {
	c := l.c
	if !c.IsCurrent(gen) {
		txt := []rune(m.Msg + " " + m.Dat)
		if len(txt) > 80 {
			txt = txt[:80]
		}
		c.log.Debug(fmt.Sprintf("로그아웃 뒤 도착한 알림을 버립니다 >>> 종류: %d 내용: %s", m.NotifySvcno, string(txt)), "category", applog.CatAuth)
		return
	}
	if why := notify.Ignore(m, myID); why != "" {
		c.log.Debug(why, "category", applog.CatGrpc)
		return
	}
	if m.NotifySvcno == notify.SvcEvent && (m.Kind < notify.KindFileList || m.Kind > notify.KindReconFM) {
		c.log.Debug(fmt.Sprintf("알 수 없는 Notify kind: %d", m.Kind), "category", applog.CatGrpc)
		return
	}
	if h.d.Dispatch(m) == 0 {
		c.log.Debug(fmt.Sprintf("처리할 곳이 없는 Notify >>> 전문번호[%d] kind[%d]", m.NotifySvcno, m.Kind), "category", applog.CatGrpc)
	}
}

func (c *Core) dialog(title, text, onClose string) {
	c.Emit(EvDialog, Dialog{Title: title, Text: text, OnClose: onClose})
}

// registerDefaults 는 원본 receivedNotify·processNotify 의 기본 처리다.
func (l *LogService) registerDefaults(h *notifyHub) {
	c := l.c
	refresh := func(key, ev string) {
		gen := c.Gen()
		h.deb.Do(key, func() { // 서버 처리 완료를 1초 기다린다(원본 singleShot 1000). 짧은 사이 여러 번 와도 한 번
			if c.IsCurrent(gen) {
				c.Emit(ev, nil)
			}
		})
	}
	files := func() { refresh("files", EvServerFilesChanged) }
	approval := func() { refresh("approval", EvApprovalChanged) }
	grpcDebug := func(msg string) { c.log.Debug(msg, "category", applog.CatGrpc) }
	d := h.d

	d.Register(notify.SvcFileList, notify.AnyKind, func(*pbn.NotifyMessage) { files() })

	d.Register(notify.SvcApprovalReq, notify.AnyKind, func(m *pbn.NotifyMessage) {
		approval()
		if m.Dat == notify.ProxyProcessDat { // 대리결재 처리 ― 원결재자에게 알림창을 띄우지 않는다
			return
		}
		const title, body = "결재 요청", "파일 이동에 대한 승인 요청 건이 있습니다."
		if !c.Balloon(title, body) {
			c.Emit(EvNotice, Notice{Severity: SevInfo, Text: body})
		}
	})

	d.Register(notify.SvcEvent, notify.KindFileList, func(*pbn.NotifyMessage) {
		grpcDebug("파일목록 갱신 요청 Notify 수신")
		files()
		grpcDebug("결재파일목록 갱신 요청 Notify 수신") // 원본의 fallthrough ― 결재 목록도 갱신
		approval()
	})
	d.Register(notify.SvcEvent, notify.KindFileAprv, func(*pbn.NotifyMessage) {
		grpcDebug("결재파일목록 갱신 요청 Notify 수신")
		approval()
	})
	d.Register(notify.SvcEvent, notify.KindMailList, func(*pbn.NotifyMessage) { grpcDebug("메일목록 갱신 요청 Notify 수신") })
	d.Register(notify.SvcEvent, notify.KindMailAprv, func(*pbn.NotifyMessage) { grpcDebug("메일결재목록 갱신 요청 Notify 수신") })
	d.Register(notify.SvcEvent, notify.KindURLSend, func(m *pbn.NotifyMessage) {
		grpcDebug("링크 리다이렉트 Notify 수신 >>> URL: " + m.Dat) // 여는 일은 링크 영역이 같은 자리에 등록한다
	})
	d.Register(notify.SvcEvent, notify.KindEndAprv, func(m *pbn.NotifyMessage) {
		grpcDebug("결재요청자에게 결재완료(승인/반려) Notify >>> 수신 메시지: " + m.Dat)
		const title = "결재 완료"
		body := "결재 완료\n승인 요청하신 파일의 결재가 완료되었습니다.\n요청 결과 : " + notify.ApprovalResult(m.Dat)
		files()
		if !c.Balloon(title, body) {
			c.dialog(title, body, "")
		}
	})
	dup := func(m *pbn.NotifyMessage) {
		grpcDebug("동일사용자 다중로그인 감지 후 종료 >>> IP: " + m.Ipaddr)
		if m.Side != notify.SideSame {
			return
		}
		// 로그아웃 먼저(우리 스트림은 조용히 끝난다), 그다음 안내 ― 닫으면 앱을 끝낸다(원본 m_exitWhenNoticeClosed)
		c.LogoutNotice(ReasonDuplicate, TitleDuplicate, MsgDuplicateKick, DialogExit)
	}
	d.Register(notify.SvcEvent, notify.KindDupFM, dup)
	d.Register(notify.SvcEvent, notify.KindReconFM, dup)

	d.Register(notify.SvcNotice, notify.AnyKind, func(m *pbn.NotifyMessage) {
		title := m.Msg
		if strings.TrimSpace(title) == "" {
			title = "정보"
		}
		c.dialog(title, m.Dat, "")
	})
	// 9005(무결성)는 wiring.go 가 잇는다.
}

// LogRow 는 [로그] 표의 한 줄이다.
type LogRow struct {
	No       int    `json:"no"`   // 맞은 줄의 순번(1부터, 파일 순서)
	Time     string `json:"time"` // hh:mm:ss.zzz
	Level    string `json:"level"`
	Account  string `json:"account"` // ID@망이름
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file"` // 파일:줄
}

// LogResult 는 조회 결과다.
type LogResult struct {
	Rows      []LogRow `json:"rows"`      // 파일 순서(오래된 것 먼저). 화면이 시간 내림차순으로 보인다
	Total     int      `json:"total"`     // 파일의 전체 줄 수
	Matched   int      `json:"matched"`   // 조건에 맞은 줄 수
	Truncated bool     `json:"truncated"` // 맞은 줄이 너무 많아 마지막 MaxLogRows 줄만 보낸다
	Missing   bool     `json:"missing"`   // 그날 파일이 없다
	Path      string   `json:"path"`      // 읽은 파일
	Ms        int64    `json:"ms"`
}

// MaxLogRows 는 한 번에 화면에 보내는 최대 줄 수다(맞은 줄이 더 많으면 가장 최근 것).
const MaxLogRows = 50_000

// Search 는 date(yyyy-MM-dd 또는 yyyyMMdd) 하루치 로그를 거른다. level: "" 전부·"INFO" 정확히 그 수준(원본)·"INFO+" 그 수준 이상,
// category: 접두사, word: 메시지 칸만·대소문자 무시.
func (l *LogService) Search(date, level, category, word string) (*LogResult, error) {
	r, err := applog.Search(date, applog.Query{Level: level, Category: category, Word: word, Max: MaxLogRows})
	if err != nil {
		return nil, err
	}
	out := &LogResult{Rows: make([]LogRow, len(r.Lines)), Total: r.Total, Matched: r.Matched, Missing: r.Missing,
		Truncated: r.Matched > len(r.Lines), Path: r.Path, Ms: r.Elapsed.Milliseconds()}
	for i, x := range r.Lines {
		out.Rows[i] = LogRow{No: r.Nos[i], Time: x.Time, Level: x.Level, Account: x.Account, Category: x.Category, Message: x.Message, File: x.Where}
	}
	if r.Missing {
		l.c.log.Debug(r.Path+" 파일이 없습니다.", "category", applog.CatUI)
	} else {
		l.c.log.Debug(fmt.Sprintf("로그 조회 >>> %d 줄 중 %d 줄, %d ms", r.Total, r.Matched, out.Ms), "category", applog.CatUI)
	}
	return out, nil
}

// Log 는 화면 쪽 로그다(분류 qml, 파일:줄 -:0). level: DEBUG·INFO·WARN·ERROR.
func (l *LogService) Log(level, message string) {
	lv := slog.LevelInfo
	switch strings.ToUpper(level) {
	case "DEBUG":
		lv = slog.LevelDebug
	case "WARN":
		lv = slog.LevelWarn
	case "ERROR":
		lv = slog.LevelError
	}
	l.c.Log().Log(context.Background(), lv, message, "category", applog.CatQML, "where", "-:0")
}

// FreezeWatchEnabled 는 화면 멈춤 감시를 켤까다(SNB_FREEZE_WATCH=1). 켜져 있으면 처음 물을 때 한 번 그렇다고 남긴다.
func (l *LogService) FreezeWatchEnabled() bool {
	on := os.Getenv("SNB_FREEZE_WATCH") == "1"
	if on {
		l.freezeOnce.Do(func() {
			l.c.log.Info("GUI 멈춤 감시를 켰습니다(500ms 넘게 멈추면 로그에 남깁니다).", "category", applog.CatApp)
		})
	}
	return on
}

// Freeze 는 화면(WebView)의 200ms 시계가 ms 만큼 늦게 불렸다는 보고다(500 초과만 화면이 보낸다).
func (l *LogService) Freeze(ms int) {
	l.c.log.Warn(fmt.Sprintf("GUI 멈춤 >>> %d ms", ms), "category", applog.CatApp, "where", "-:0")
}

// SetLogFile 은 [로그 저장] 이다: 켜면 로그 폴더를 만들고, 끄면 파일을 닫고 폴더를 지운 뒤 useLogFile 을 저장한다.
func (l *LogService) SetLogFile(use bool) error {
	err := applog.SetLogFile(use)
	if err != nil {
		if use {
			l.c.log.Warn("로그 폴더를 만들지 못했다: "+applog.Dir(), "category", applog.CatApp, "err", err)
		} else {
			l.c.log.Warn("로그 폴더를 지우지 못했다(다른 프로세스가 쓰는 중일 수 있다) ― 로그가 계속 남는다: "+applog.Dir(), "category", applog.CatApp)
		}
	} else {
		l.c.log.Info(fmt.Sprintf("로그 저장 >>> %v", use), "category", applog.CatSettings)
	}
	if st := l.c.Settings(); st != nil {
		if e := st.Update(func(s *config.Settings) { s.UseLogFile = use }); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// LogFileEnabled 는 로그 폴더가 있는가다 ― [로그 저장] 의 실제 상태(useLogFile 은 표시값일 뿐).
func (l *LogService) LogFileEnabled() bool {
	fi, err := os.Stat(applog.Dir())
	return err == nil && fi.IsDir()
}

// InjectNotify 는 시험 진입점이다(개발 빌드이면서 SNB_TEST_HOOKS=1 일 때만): 서버 알림 한 건을 받은 것처럼 처리한다.
// 아무 알림이나 흉내 낼 수 있어 배포본에서는 환경변수로도 열리지 않는다. 실서버가 9007 공지를 보내지 않아 이것 없이는 공지 처리를 시험할 길이 없다(docs/port/09 7.4).
func (l *LogService) InjectNotify(svcno, kind, side int32, userid, msg, dat string) error {
	if !config.IsDev() || os.Getenv("SNB_TEST_HOOKS") != "1" {
		l.c.log.Warn("[시험진입점] notify >>> 개발 빌드의 SNB_TEST_HOOKS 가 없어 무시합니다.", "category", applog.CatApp)
		return ErrNotImplemented
	}
	s, err := l.c.Session()
	if err != nil {
		return err
	}
	m := &pbn.NotifyMessage{NotifySvcno: svcno, Kind: kind, Side: side, Userid: userid, Msg: msg, Dat: dat}
	l.c.log.Warn("[시험진입점] notify >>> "+notify.ReceivedLine(m), "category", applog.CatApp)
	l.onNotify(hubOf(l.c), l.c.Gen(), s.UserID, m)
	return nil
}
