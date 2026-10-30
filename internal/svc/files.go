package svc

// 파일·내역 영역: 서버파일 목록, 상태, 삭제, 검출 조회(docs/port/04).
//
// 목록은 Go 가 한 벌 들고(로그인 세대별), 조회가 겹치면 합친다(조회 중에 또 요청되면 끝난 뒤 한 번 더).
// 목록이 실제로 바뀐 때만 화면에 EvServerFileList 를 내므로 표가 깜빡이지 않고 선택이 남는다.
// 검사 중 행이 있으면 1·2·4·8초 간격으로 스스로 다시 조회한다(최대 40회) ― 서버는 검사가 끝나도 알리지 않는다.
// 다른 곳(알림 9001 등)이 목록을 새로 받게 하려면 Go 에서 c.Emit(EvServerFilesChanged, nil) 이나 serverFiles.refresh().
// 같은 패키지의 서비스가 쓰는 것(refresh·snapshot·find·onChange)은 화면에 묶이지 않게 소문자로 둔다.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/pb/serverfile"
)

// EvServerFileList 는 서버파일 목록이 바뀌었을 때 낸다. 값은 []FileRow 전체(서버 순서).
const EvServerFileList = "files:list"

// serverFiles 는 앱에 하나인 FilesService 다. 같은 패키지의 다른 서비스(전송·결재·알림)가 목록·갱신에 쓴다.
var serverFiles *FilesService

// FilesService 는 [파일전송] 화면의 서버파일 표가 부른다.
type FilesService struct {
	base

	mu       sync.Mutex
	gen      uint64 // 아래 목록을 받은 로그인 세대
	list     []*serverfile.FileInfo
	rows     []FileRow
	running  bool
	again    bool
	waiters  []chan struct{}
	lastErr  error
	recheck  *time.Timer
	rechecks int
	hooks    []func(*nbs.Session, []*serverfile.FileInfo)
	same     func()
}

func newFilesService(c *Core) *FilesService {
	f := &FilesService{base: base{c}}
	serverFiles = f
	c.OnLogout(f.reset)
	c.OnLogin(func(*nbs.Session) { f.reset() })
	return f
}

// ServiceStartup 은 EvServerFilesChanged(「목록을 다시 받아라」)를 Go 쪽에서도 듣는다.
func (f *FilesService) ServiceStartup(ctx context.Context, o application.ServiceOptions) error {
	if err := f.base.ServiceStartup(ctx, o); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.On(EvServerFilesChanged, func(*application.CustomEvent) { f.refresh() })
	}
	return nil
}

// FileRow 는 서버파일 표의 한 줄이다(표 12칸 + 판정 값). 칸은 모두 비교할 수 있는 값이라 == 로 변화를 가린다.
type FileRow struct {
	No         int    `json:"no"`         // 서버 순서 번호(1부터)
	ServerName string `json:"serverName"` // server_str ― 받기·삭제·검출 조회의 열쇠
	Name       string `json:"name"`       // [파일명]: 마지막 경로 조각, 폴더는 .zip 뗌
	Size       uint64 `json:"size"`
	SizeText   string `json:"sizeText"`   // 원본 크기 글자(0 Byte · 1.0 KB …)
	UploadTime string `json:"uploadTime"` // [업로드일시](서버 expire_time ― 이름과 달리 올린 시각)
	Course     int32  `json:"course"`     // 올린 망 LOC: 1 업무망, 0 인터넷망
	CourseName string `json:"courseName"` // [전송방향]: fmenv.ini 의 그 망 NAME, 없으면 Unknown
	ReqCause   string `json:"reqCause"`
	Approver   string `json:"approver"`
	AprvCause  string `json:"aprvCause"` // "-" 는 빈 값
	AprvTime   string `json:"aprvTime"`
	IsDir      bool   `json:"isDir"` // [폴더전송]
	IsClip     bool   `json:"isClip"`
	StatusText string `json:"statusText"` // [상태] 글자(원본 getState + 동일망)
	StatusCode int    `json:"statusCode"` // 1 다운로드 가능, 0 결재 대기, 2 반려, 4 결재요청 가능, -1 받을 수 없음
	StatusKind string `json:"statusKind"` // 칩 색: danger·pending·success·warning·accent·muted
	Block      int    `json:"block"`      // [다운로드] 에서 빼는 사유(files.Block): 0 없음, 1 클립보드, 2 동일망, 3 검사 중, 4 상태 아님
	State      int32  `json:"state"`      // 원시 결재 상태
	VirusFlag  int32  `json:"virusFlag"`
	PrivState  int32  `json:"privState"`
	PrivCnt    int32  `json:"privCnt"`
	AutoDown   int32  `json:"autoDown"`
}

func (f *FilesService) makeRows(s *nbs.Session, list []*serverfile.FileInfo) ([]FileRow, int) {
	servers := filesEnvServers(f.c)
	loc := s.Server.Loc
	rows := make([]FileRow, 0, len(list))
	same := 0
	for i, x := range list {
		st := files.FileStatus(x, loc, s.Profile)
		if st.Kind == files.KindWarning {
			same++
		}
		rows = append(rows, FileRow{
			No: i + 1, ServerName: x.ServerStr, Name: files.DisplayName(x), Size: x.FileSize, SizeText: files.SizeText(x.FileSize),
			UploadTime: x.ExpireTime, Course: x.Course, CourseName: files.CourseName(servers, x.Course),
			ReqCause: x.ReqCause, Approver: x.AprvNameStr, AprvCause: files.AprvCause(x), AprvTime: x.AprvTime,
			IsDir: x.DirFlag == 1, IsClip: files.IsClip(x),
			StatusText: st.Text, StatusCode: st.Code, StatusKind: st.Kind, Block: int(files.Classify(x, loc, s.Profile)),
			State: x.State, VirusFlag: x.VirusFlag, PrivState: x.PrivState, PrivCnt: x.PrivCnt, AutoDown: x.AutoDown,
		})
	}
	return rows, same
}

// filesEnvServers 는 fmenv.ini 서버 목록이다(못 읽었으면 nil). [전송방향] 글자에 쓴다.
func filesEnvServers(c *Core) []config.Server {
	if env, _ := c.Env(); env != nil {
		return env.Servers
	}
	return nil
}

// List 는 서버에서 목록을 새로 받아 돌려준다(화면에 들어올 때·[새로고침]). 조회가 돌고 있으면 그것이 끝난 뒤 한 번 더 받은 결과다.
func (f *FilesService) List() ([]FileRow, error) {
	if _, err := f.c.Session(); err != nil {
		return nil, err
	}
	ch := make(chan struct{})
	f.mu.Lock()
	f.waiters = append(f.waiters, ch)
	start := f.kickLocked()
	f.mu.Unlock()
	if start {
		go f.loop()
	}
	<-ch
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastErr != nil {
		return nil, f.lastErr
	}
	return f.rows, nil
}

// refresh 는 목록을 새로 받게 하고 곧바로 돌아온다(알림 9001·삭제 뒤·검사 중 재조회·받은 뒤). 바뀌었으면 EvServerFileList 가 나간다.
func (f *FilesService) refresh() {
	if _, err := f.c.Session(); err != nil {
		return
	}
	f.mu.Lock()
	start := f.kickLocked()
	f.mu.Unlock()
	if start {
		go f.loop()
	}
}

// kickLocked 는 조회를 새로 시작해야 하면 참이다. 돌고 있으면 「끝난 뒤 한 번 더」만 표시한다(원본 serverFileRefreshPending_).
func (f *FilesService) kickLocked() bool {
	if f.running {
		f.again = true
		return false
	}
	f.running = true
	return true
}

func (f *FilesService) loop() {
	changed := false
	for {
		ch, err := f.fetchOnce()
		changed = changed || ch
		f.mu.Lock()
		if f.again {
			f.again = false
			f.mu.Unlock()
			continue
		}
		f.running = false
		f.lastErr = err
		ws := f.waiters
		f.waiters = nil
		f.mu.Unlock()
		for _, w := range ws {
			close(w)
		}
		if err != nil {
			if !errors.Is(err, ErrNotLoggedIn) {
				f.c.Log().Warn("서버파일목록 조회 실패", "category", applog.CatGrpc, "err", err)
			}
			return
		}
		f.scheduleRecheck(changed)
		return
	}
}

// fetchOnce 는 목록을 한 번 받아 캐시에 넣고, 바뀌었으면 화면과 등록된 곳에 알린다.
func (f *FilesService) fetchOnce() (bool, error) {
	s, err := f.c.Session()
	if err != nil {
		return false, err
	}
	gen := f.c.Gen()
	list, err := files.List(s)
	if err != nil {
		return false, err
	}
	if !f.c.IsCurrent(gen) { // 받는 사이 로그아웃·다시 로그인 ― 낡은 결과는 버린다
		return false, ErrNotLoggedIn
	}
	rows, same := f.makeRows(s, list)
	f.mu.Lock()
	changed := f.gen != gen || !slices.Equal(rows, f.rows)
	f.gen, f.list, f.rows = gen, list, rows
	hooks, unchanged := slices.Clone(f.hooks), f.same
	f.mu.Unlock()
	if !changed {
		if unchanged != nil {
			unchanged()
		}
		return false, nil
	}
	f.c.Log().Debug(fmt.Sprintf("서버파일 상태 판정 >>> %d 건, 동일망 다운로드 불가 %d 건", len(list), same), "category", applog.CatUI)
	f.c.Emit(EvServerFileList, rows)
	for _, h := range hooks {
		h(s, list)
	}
	return true, nil
}

// scheduleRecheck 는 검사 중 행이 있으면 다음 재조회를 건다(원본 schedulePendingScanPoll). 목록이 바뀌었으면 횟수를 다시 센다.
func (f *FilesService) scheduleRecheck(changed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recheck != nil {
		f.recheck.Stop()
		f.recheck = nil
	}
	if !files.Scanning(f.list) {
		f.rechecks = 0
		return
	}
	if changed {
		f.rechecks = 0
	}
	if f.rechecks >= files.MaxRechecks {
		f.c.Log().Debug("[검사중 항목 재조회] 상한에 도달해 더 조회하지 않는다.", "category", applog.CatGrpc)
		return
	}
	d := time.Duration(files.RecheckDelay(f.rechecks)) * time.Second
	f.rechecks++
	gen := f.gen
	f.recheck = time.AfterFunc(d, func() {
		if f.c.IsCurrent(gen) {
			f.refresh()
		}
	})
}

// reset 은 로그인·로그아웃 때 캐시와 재조회 시계를 비운다.
func (f *FilesService) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recheck != nil {
		f.recheck.Stop()
		f.recheck = nil
	}
	f.list, f.rows, f.rechecks, f.gen = nil, nil, 0, 0
}

// snapshot 은 마지막으로 받은 목록(서버 원본 행)이다. 지금 로그인의 것이 아니면 nil. 전송(받기 크기·자동 다운로드)·결재(zip_pswd)가 쓴다.
// 돌려준 행은 고치지 않는다.
func (f *FilesService) snapshot() []*serverfile.FileInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.c.IsCurrent(f.gen) {
		return nil
	}
	return f.list
}

// find 는 마지막 목록에서 server_str 이 같은 행이다(없으면 nil).
func (f *FilesService) find(serverName string) *serverfile.FileInfo {
	for _, x := range f.snapshot() {
		if x.ServerStr == serverName {
			return x
		}
	}
	return nil
}

// onChange 는 목록이 바뀔 때마다(첫 조회 포함) 부를 일을 더한다(자동 다운로드 예약 등). 조회 고루틴에서 부르므로 오래 막지 않는다.
// 서비스 생성자(newXxxService)에서 serverFiles.onChange(...) 로 건다 ― newFilesService 가 먼저 불린다(Core.Services 순서).
func (f *FilesService) onChange(fn func(*nbs.Session, []*serverfile.FileInfo)) {
	f.mu.Lock()
	f.hooks = append(f.hooks, fn)
	f.mu.Unlock()
}

// onSame 은 목록이 그대로일 때 부를 일을 건다(실패한 자동 다운로드 다시 돌기). 부르는 고루틴은 onChange 와 같다.
func (f *FilesService) onSame(fn func()) {
	f.mu.Lock()
	f.same = fn
	f.mu.Unlock()
}

// deleteParallel 은 동시에 보내는 삭제 수다(원본 MAX_CONCURRENT_DELETES 20 보다 줄였다).
const deleteParallel = 8

// Delete 는 서버 파일들을 지우고(RemoveFile flag 0) 지우지 못한 건수를 돌려준다. 끝나면 목록을 새로 받는다.
// 서버는 요청자 소유 행만 바꾼다. 확인 창은 화면이 띄운다.
func (f *FilesService) Delete(serverNames []string) (int, error) {
	s, err := f.c.Session()
	if err != nil {
		return 0, err
	}
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
		sem    = make(chan struct{}, deleteParallel)
	)
	for _, n := range serverNames {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := files.Remove(s, n, false); err != nil {
				f.c.Log().Warn("서버 파일 삭제 실패", "category", applog.CatDownload, "server", n, "err", err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			f.c.Log().Info("서버 파일 삭제", "category", applog.CatDownload, "server", n)
		}()
	}
	wg.Wait()
	f.refresh()
	return failed, nil
}

// DetectionDetail 은 검출 조회 창 본문이다. kind: 0 개인정보(목록의 상세를 다듬는다), 1 APT, 2 바이러스(서버에 묻는다).
// 없으면 「검출된 내용이 없습니다.」, 서버에 묻지 못했으면 오류(「검출 상세를 받지 못했습니다: …」).
func (f *FilesService) DetectionDetail(serverName string, kind int) (string, error) {
	s, err := f.c.Session()
	if err != nil {
		return "", err
	}
	switch kind {
	case files.DetectPrivacy:
		x := f.find(serverName)
		if x == nil {
			return "", fmt.Errorf("서버 파일 목록에 없습니다: %s", serverName)
		}
		return files.PrivDetailText(x.PrivDetail, x.ServerStr), nil
	case files.DetectAPT, files.DetectVirus:
		return files.DetectionDetail(s, serverName, int32(kind))
	}
	return "", fmt.Errorf("모르는 검출 종류입니다: %d", kind)
}
