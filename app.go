package main

// 화면(frontend)이 부르는 메서드. Wails 가 이 서비스의 공개 메서드를 frontend/bindings 로 묶어 준다(wails3 generate bindings).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const version = "0.1.0"

type AppService struct {
	dir string // fmenv.ini·root_ca.pem 이 있는 폴더

	mu sync.Mutex
	s  *Session
}

// appDir 는 SNB_APP_DIR 가 있으면 그것, 없으면 실행 파일 폴더다(원본 앱도 실행 파일 옆의 fmenv.ini·root_ca.pem 을 읽는다).
func appDir() string {
	if d := os.Getenv("SNB_APP_DIR"); d != "" {
		return d
	}
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

func (a *AppService) ServiceStartup(context.Context, application.ServiceOptions) error {
	a.dir = appDir()
	return nil
}

func (a *AppService) ServiceShutdown() error {
	a.Logout()
	return nil
}

func (a *AppService) session() (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s == nil {
		return nil, errors.New("로그인하지 않았습니다")
	}
	return a.s, nil
}

// Servers 는 fmenv.ini 의 서버 목록이다(로그인 화면의 망 선택).
func (a *AppService) Servers() ([]Server, error) {
	return readServers(filepath.Join(a.dir, "fmenv.ini"))
}

type LoginInfo struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Dept     string `json:"dept"`
	Server   Server `json:"server"`
}

func (a *AppService) Login(srv Server, id, pw string) (*LoginInfo, error) {
	a.Logout()
	s, err := Login(a.dir, srv, id, pw, version)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.s = s
	a.mu.Unlock()
	return &LoginInfo{UserID: s.UserID, UserName: s.Info.UserName, Dept: s.Info.UserDeptName, Server: srv}, nil
}

// Logout 은 연결을 끊는다. 서버에 Logout RPC 가 없고 TCP 가 끊길 때 세션을 회수한다.
func (a *AppService) Logout() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s != nil {
		a.s.Close()
		a.s = nil
	}
}

type FileRow struct {
	ServerName string `json:"serverName"`
	Name       string `json:"name"`
	Size       uint64 `json:"size"`
	State      int32  `json:"state"`  // 0 결재요청, 1 승인, 2 반려, 4 결재요청 가능(업로드 뒤 상신 전)
	Course     int32  `json:"course"` // 올린 망: 1 업무망, 0 인터넷망
	AprvTime   string `json:"aprvTime"`
	ExpireTime string `json:"expireTime"`
	Blocked    string `json:"blocked"` // 받을 수 없는 까닭. 받을 수 있으면 빈 값
}

func (a *AppService) Files() ([]FileRow, error) {
	s, err := a.session()
	if err != nil {
		return nil, err
	}
	list, err := s.Files()
	if err != nil {
		return nil, err
	}
	rows := make([]FileRow, 0, len(list))
	for _, f := range list {
		rows = append(rows, FileRow{
			ServerName: f.ServerStr, Name: f.NameStr, Size: f.FileSize, State: f.State, Course: f.Course,
			AprvTime: f.AprvTime, ExpireTime: f.ExpireTime, Blocked: downloadBlock(f, s.Server.Loc, s.Profile),
		})
	}
	return rows, nil
}

func (a *AppService) progress(name string) func(done, total int64) {
	return func(done, total int64) {
		application.Get().Event.Emit("progress", map[string]any{"name": name, "done": done, "total": total})
	}
}

// Upload 는 파일 선택 창에서 고른 파일을 차례로 올리고 올린 파일 이름을 돌려준다. 고르지 않으면 빈 목록이다.
// ponytail: 한 번에 하나씩 올린다. 원본의 동시 전송(최대 5개, TransferQueue)은 전송 상태 화면과 함께 다음 단계.
func (a *AppService) Upload() ([]string, error) {
	s, err := a.session()
	if err != nil {
		return nil, err
	}
	paths, err := application.Get().Dialog.OpenFile().SetTitle("올릴 파일").PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	var done []string
	for _, p := range paths {
		name := filepath.Base(p)
		if err := s.Upload(p, a.progress(name)); err != nil {
			return done, errors.New(name + ": " + err.Error())
		}
		done = append(done, name)
	}
	return done, nil
}

// Download 는 폴더 선택 창에서 고른 폴더에 서버 파일들을 받고 저장한 경로를 돌려준다. 폴더를 고르지 않으면 빈 목록이다.
func (a *AppService) Download(serverNames []string) ([]string, error) {
	s, err := a.session()
	if err != nil {
		return nil, err
	}
	dir, err := application.Get().Dialog.OpenFile().SetTitle("받을 폴더").
		CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if err != nil || dir == "" {
		return nil, err
	}
	var saved []string
	for _, n := range serverNames {
		p, err := s.Download(n, dir, a.progress(n))
		if err != nil {
			return saved, err
		}
		saved = append(saved, p)
	}
	return saved, nil
}
