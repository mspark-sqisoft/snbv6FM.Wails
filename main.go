package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/link"
	"snbapp/internal/nbs"
	"snbapp/internal/svc"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

// version 은 앱 버전이다(로그인 요청 app_version·[프로그램 정보]). 원본 CMakeLists project(VERSION 6.0.0.1) 와 같은 값.
// 빌드 때 -ldflags "-X main.version=..." 로 바꾼다.
var version = "6.0.0.1"

// 창 배경색(tokens.css 의 --color-bg). 첫 그림 전 흰 번쩍임을 막으려고 창을 만들 때 테마에 맞춰 둔다.
var (
	bgLight = application.RGBA{Red: 0xff, Green: 0xff, Blue: 0xff, Alpha: 0xff}
	bgDark  = application.RGBA{Red: 0x19, Green: 0x1d, Blue: 0x20, Alpha: 0xff}
)

func background(theme string) application.RGBA {
	if theme == "dark" || (theme != "light" && config.SystemDark()) {
		return bgDark
	}
	return bgLight
}

// warnOnly 는 Wails 자체 로그를 경고 이상만 앱 로그로 넘긴다. Wails 는 DEBUG 에 바인딩 호출 인자를 그대로 찍어
// 로그인 비밀번호가 파일에 남는다(LOG-032).
type warnOnly struct{ slog.Handler }

func (warnOnly) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelWarn }
func (h warnOnly) WithAttrs(a []slog.Attr) slog.Handler       { return warnOnly{h.Handler.WithAttrs(a)} }
func (h warnOnly) WithGroup(n string) slog.Handler            { return warnOnly{h.Handler.WithGroup(n)} }

func main() {
	if link.HandleInstallArgs(os.Args[1:]) { // 설치기가 부르는 --register-shell·--unregister-shell(창·로그 없이 끝낸다)
		return
	}
	logger := applog.New()
	// 잡히지 않은 panic 은 로그에 FATAL 로 남기고 끝낸다(LOG-020·022). 서비스·사건 처리기의 panic 은 Wails 가 PanicHandler 로 넘긴다.
	fatal := func(why any, stack []byte) {
		logger.Log(context.Background(), applog.LevelFatal, fmt.Sprintf("panic: %v\n%s", why, stack), "category", applog.CatApp)
		applog.Close("종료 코드 2")
		os.Exit(2)
	}
	defer func() {
		if r := recover(); r != nil {
			fatal(r, debug.Stack())
		}
	}()

	core := svc.NewCore(version, logger)
	note := notifications.New()
	core.SetBalloon(func(title, body string) error { // 결재 알림 풍선(MAIN-078). 못 띄우면 알림 띠·확인 창으로 대신한다
		return note.SendNotification(notifications.NotificationOptions{ID: strconv.FormatInt(time.Now().UnixNano(), 10), Title: title, Body: body})
	})

	// 시작 테마만 미리 읽는다(설정 저장소는 서비스 시작 때 연다).
	theme := config.Defaults().Theme
	if st, err := config.Open(filepath.Join(config.DataDir(), config.SettingsFile)); err == nil {
		theme = st.Get().Theme
	}

	var win *application.WebviewWindow
	show := func() {
		if win != nil {
			win.Show()
			win.Restore()
			win.Focus()
		}
	}

	// WebView2 자료도 데이터 폴더에 둔다(인스턴스마다 나뉜다). 기본값은 %APPDATA%\snbApp.exe 다.
	winOpts := application.WindowsOptions{WebviewUserDataPath: filepath.Join(config.DataDir(), "webview")}
	if p := os.Getenv("SNB_CDP_PORT"); p != "" && config.Instance() != "" { // 개발용: 화면 자동 조작(원격 디버깅). 인스턴스는 개발 빌드에만 있다
		winOpts.AdditionalBrowserArgs = []string{"--remote-debugging-port=" + p}
	}

	app := application.New(application.Options{
		Name:         "snB App",
		Description:  "snB 망연계 파일전송 클라이언트",
		Services:     append(core.Services(), application.NewService(note)),
		Assets:       application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Logger:       slog.New(warnOnly{logger.Handler()}),
		PanicHandler: func(d *application.PanicDetails) { fatal(d.Error, []byte(d.StackTrace)) },
		Mac:          application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Windows:      winOpts,
		// 한 PC 에 하나만 실행. Qt 판(MOSFM 6.0)과는 다른 ID 라 서로 막지 않는다(docs/port/01 7.8).
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.sqisoft.snbapp" + config.Instance(),
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				show() // 트레이로 숨은 창도 다시 보인다(원본은 보이는 창만 올렸다 ― docs/port/12 8.3)
				core.SecondInstance(d)
			},
		},
	})

	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            svc.WindowTitle,
		Width:            1100,
		Height:           720,
		MinWidth:         640, // 원본 최소 360 은 모바일 배치(P2)용 ― 데스크톱 배치가 깨지지 않는 폭까지만 줄인다
		MinHeight:        480,
		BackgroundColour: background(theme),
		URL:              "/",
		EnableFileDrop:   true, // 서버파일 표(data-file-drop-target)에 탐색기 파일 놓기 ― 처리는 svc/transfer.go watchDrop
	})

	// 창 닫기(X): 로그인 중이고 설정 「창을 닫으면 트레이로 내리기」 가 켜져 있으면 숨긴다. 그 밖은 원본처럼 종료(MAIN-076).
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		st := core.Settings()
		if _, err := core.Session(); err != nil || st == nil || !st.Get().CloseToTray {
			return
		}
		e.Cancel()
		win.Hide()
	})

	// 트레이(원본 setTrayIcon): 두 번 누름·[열기] = 창 보이기, [정책 받아오기]·[로그아웃] 은 로그인 중에만, [종료].
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("snB App")
	menu := application.NewMenu()
	menu.Add("열기").OnClick(func(*application.Context) { show() })
	reload := menu.Add("정책 받아오기").OnClick(func(*application.Context) {
		core.Emit(svc.EvTrayPolicyReload, nil) // 화면이 PolicyService.Reload 를 부르고 결과를 알림 띠로 보인다
	})
	logout := menu.Add("로그아웃").OnClick(func(*application.Context) {
		core.Logout(svc.ReasonLogout)
		show()
	})
	menu.AddSeparator()
	menu.Add("종료").OnClick(func(*application.Context) { app.Quit() })
	reload.SetEnabled(false)
	logout.SetEnabled(false)
	tray.SetMenu(menu)
	tray.OnDoubleClick(show)

	core.OnLogin(func(s *nbs.Session) {
		reload.SetEnabled(true)
		logout.SetEnabled(true)
		menu.Update()
		tray.SetTooltip("snB App - " + s.Server.Name)
	})
	core.OnLogout(func() {
		reload.SetEnabled(false)
		logout.SetEnabled(false)
		menu.Update()
		tray.SetTooltip("snB App")
	})

	// 설정에서 테마를 바꾸면 창 배경색도 맞춘다(창 크기를 바꿀 때 드러나는 가장자리).
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if st := core.Settings(); st != nil {
			st.OnChange(func(old, cur config.Settings) {
				if old.Theme != cur.Theme {
					win.SetBackgroundColour(background(cur.Theme))
				}
			})
		}
	})

	code := 0
	if err := app.Run(); err != nil {
		logger.Error("앱 실행 실패", "category", applog.CatApp, "err", err)
		code = 1
	}
	applog.Close("종료 코드 " + strconv.Itoa(code))
	os.Exit(code)
}
