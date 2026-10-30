package svc

// 메인 영역: 설정 읽기·쓰기, 폴더 선택·쓰기 검사, 앱 정보(docs/port/12 5.8).

import (
	"context"
	"errors"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/localfs"
)

// EvTrayPolicyReload 는 트레이 [정책 받아오기] 를 눌렀다는 사건이다(값 없음). 화면(shell)이 PolicyService.Reload 를 부른다.
const EvTrayPolicyReload = "tray:policy-reload"

// SettingsService 는 [설정] 화면과 머리줄(테마·언어·배치)이 부른다.
type SettingsService struct{ base }

func newSettingsService(c *Core) *SettingsService { return &SettingsService{base{c}} }

// ServiceStartup 은 설정이 바뀔 때마다 `설정 변경 >>> 키: 이전 → 새` 를 남기게 한다(원본 SET-089, 비밀 단어는 가린다).
func (s *SettingsService) ServiceStartup(ctx context.Context, o application.ServiceOptions) error {
	if err := s.base.ServiceStartup(ctx, o); err != nil {
		return err
	}
	if st := s.c.Settings(); st != nil {
		st.OnChange(func(old, cur config.Settings) {
			for _, ch := range config.Changes(old, cur) {
				msg := "설정 변경 >>> " + ch.Key + ": " + ch.Old + " → " + ch.New
				if ch.Key == "columnWidths" || ch.Key == "lastFolder" {
					s.c.Log().Debug(msg, "category", applog.CatSettings) // 끌기·탐색마다 바뀌는 값
				} else {
					s.c.Log().Info(msg, "category", applog.CatSettings)
				}
			}
		})
	}
	return nil
}

// Get 은 지금 설정이다.
func (s *SettingsService) Get() config.Settings { return s.c.Settings().Get() }

// Set 은 설정 전체를 저장한다. 바뀐 것이 있으면 EvSettingsChanged 가 나간다. 실패하면 화면이 알림 띠로 알린다.
func (s *SettingsService) Set(v config.Settings) error { return s.c.Settings().Set(v) }

// AppInfo 는 [프로그램 정보] 카드 값이다.
type AppInfo struct {
	Name      string `json:"name"`      // snB App
	Title     string `json:"title"`     // snB App v6.0 (창 제목)
	Version   string `json:"version"`   // 빌드 버전(로그인 요청 app_version 과 같다)
	Copyright string `json:"copyright"` // Copyrightⓒ SQIsoft All rights reserved.
	OS        string `json:"os"`        // windows·darwin·linux ― 화면이 단축키 카드를 Linux 에서 숨긴다(원본 SET-072)
	DataDir   string `json:"dataDir"`   // 사용자 데이터 폴더(로그·설정)
	AppDir    string `json:"appDir"`    // fmenv.ini 폴더
}

// WindowTitle 은 창 제목이다(원본 Main.qml).
const WindowTitle = "snB App v6.0"

// AppInfo 는 앱 이름·버전·저작권 줄과 폴더다.
func (s *SettingsService) AppInfo() AppInfo {
	return AppInfo{
		Name: "snB App", Title: WindowTitle, Version: s.c.Version(), Copyright: "Copyrightⓒ SQIsoft All rights reserved.",
		OS: runtime.GOOS, DataDir: config.DataDir(), AppDir: s.c.AppDir(),
	}
}

// PickFolder 는 폴더 선택 창(제목 「디렉토리를 선택해 주세요.」)을 start 에서 열고 고른 경로를 돌려준다. 취소면 빈 값.
// start 가 비었거나 쓸 수 없으면 시작 폴더 규칙(설정 시작 폴더 → 바탕 화면 → 홈 → 루트)으로 연다.
func (s *SettingsService) PickFolder(start string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("창이 없습니다")
	}
	init := ""
	if st := s.c.Settings(); st != nil {
		init = st.Get().InitFolder
	}
	if start == "" {
		start = init
	}
	dir, _ := localfs.ResolveStart(start, init)
	d := app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).
		SetTitle("디렉토리를 선택해 주세요.").SetDirectory(dir)
	if w := app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	p, err := d.PromptForSingleSelection()
	if err != nil && p == "" {
		return "", nil // 취소도 오류로 오는 플랫폼이 있다
	}
	return p, nil
}

// 폴더 검사 문구.
var (
	errFolderMissing  = errors.New("폴더를 찾을 수 없습니다.")
	errFolderReadOnly = errors.New("폴더에 쓸 수 없습니다. 쓰기 권한을 확인해 주세요.")
)

// CheckFolder 는 자동 업로드·다운로드 폴더로 쓸 수 있는지(있고, 폴더이고, 파일을 만들 수 있는지) 본다. 빈 값은 통과(끄는 것).
func (s *SettingsService) CheckFolder(path string) error {
	if path == "" {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return errFolderMissing
	}
	f, err := os.CreateTemp(path, ".snb-write-check-*")
	if err != nil {
		return errFolderReadOnly
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return nil
}
