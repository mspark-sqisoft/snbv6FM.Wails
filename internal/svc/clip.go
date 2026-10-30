package svc

// 클립보드 영역: 보내기·받기, 받을 건수, 전역 단축키(docs/port/07).
// 클립보드 내용 다루기는 internal/clip, Link Redirect·두 번째 실행 인자·도메인 정책은 link.go 다.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/approval"
	"snbapp/internal/clip"
	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
	"snbapp/pb/serverfile"
)

// EvClipReceived 는 받은 내용을 이 PC 클립보드에 넣었다는 사건이다(화면 안내).
const EvClipReceived = "clip:received"

// ClipService 는 [클립보드 업로드]·[클립보드 다운로드] 단추와 단축키 설정이 부른다.
type ClipService struct {
	base
	count      atomic.Int32
	receiving  atomic.Bool // 받는 중 ― 겹쳐 받으면 임시 파일과 클립보드가 엉킨다
	hotkeyBusy atomic.Bool // 단축키를 누른 채 반복되어도 한 번만

	hk     sync.Mutex
	accels []string      // 지금 등록된 가속기
	active config.Hotkey // 그때 쓴 설정
}

// clips 는 앱에 하나인 ClipService 다(결재 받기가 받은 클립보드를 넣는다).
var clips *ClipService

func newClipService(c *Core) *ClipService {
	cs := &ClipService{base: base{c}}
	clips = cs
	serverFiles.onChange(cs.onFiles)
	c.OnLogin(cs.onLogin)
	c.OnLogout(cs.onLogout)
	return cs
}

func (c *ClipService) log() *slog.Logger { return c.c.Log().With("category", applog.CatClipboard) }

func (c *ClipService) info(text string) { c.dialog("정보", text) }

func (c *ClipService) dialog(title, text string) {
	c.c.Emit(EvDialog, Dialog{ID: c.c.dialogSeq.Add(1), Title: title, Text: text})
}

func clipAllowed(s *nbs.Session, up bool) bool {
	return clip.AllowClip(s.IsBusiness(), up, s.Info.GetFmClipUp(), s.Info.GetFmClipDown())
}

// ── 받을 건수 ──

// clipCount 는 내려받을 수 있는 __CLIP__ 행 수다.
func clipCount(list []*serverfile.FileInfo, loc int, profile map[string]string) int {
	n := 0
	for _, f := range list {
		if files.IsClip(f) && files.FileStatus(f, loc, profile).Code == 1 {
			n++
		}
	}
	return n
}

func (c *ClipService) setCount(n int) {
	if c.count.Swap(int32(n)) != int32(n) {
		c.c.Emit(EvClipCount, map[string]int{"count": n})
	}
}

func (c *ClipService) onFiles(s *nbs.Session, list []*serverfile.FileInfo) {
	c.setCount(clipCount(list, s.Server.Loc, s.Profile))
}

// Count 는 받을 클립보드 건수다(바뀌면 EvClipCount 로도 알린다).
func (c *ClipService) Count() (int, error) { return int(c.count.Load()), nil }

// ── 보내기 ──

// Upload 는 지금 클립보드 내용을 올린다. 안내는 사건으로 나가고, 대기열에 넣으면 돌아온다.
func (c *ClipService) Upload() error {
	c.upload()
	return nil
}

func (c *ClipService) upload() {
	content, err := clip.Read()
	if err != nil {
		c.info(err.Error())
		return
	}
	if content.Kind == clip.KindNone || content.Kind == clip.KindText && content.Text == "" {
		c.info("저장된 클립보드가 없습니다.")
		return
	}
	s, err := c.c.Session()
	if err != nil {
		return
	}
	if !clipAllowed(s, true) {
		c.info("클립보드 업로드가 허용되어 있지 않습니다.")
		return
	}
	t := c.c.Transfers()
	ctx, cancel, token, err := t.claim(false)
	if err != nil {
		c.info(err.Error())
		return
	}
	defer t.release(token, cancel)
	queued, err := c.send(ctx, s, content, nil)
	if err != nil {
		c.log().Warn("클립보드 보내기 실패", "err", err)
		c.info(err.Error())
		return
	}
	if !queued {
		return // 크기 초과·결재자 취소면 클립보드를 그대로 둔다
	}
	if err := clip.Clear(); err != nil {
		c.log().Warn("클립보드를 비우지 못했다", "err", err)
	}
	c.c.Emit(EvTransferOpen, nil)
}

// clipFile 은 내용을 보낼 파일 바이트와 확장자로 만든다. 묶음이 뺀 참조는 앱 로그에 남긴다.
func (c *ClipService) clipFile(ctx context.Context, content *clip.Content) ([]byte, string, error) {
	switch content.Kind {
	case clip.KindImage:
		return content.ImageFile(c.c.Settings().Get().ClipImageFormat)
	case clip.KindHTML:
		b, drop, err := clip.PackHTML(ctx, content.CFHTML, time.Now().Unix())
		if drop != "" {
			c.log().Info(drop)
		}
		return b, ".fmh", err
	}
	return []byte(content.Text), ".txt", nil
}

// send 는 내용을 임시 파일로 써서 결재 갈래를 거쳐 대기열에 넣는다. 넣었으면 queued 가 참이고, 임시 파일은 전송이 끝나면 지운다.
// 파일 검사는 없다(클립보드는 크기만 본다, 원본과 같다). done 은 전송이 끝나면 불린다(nil 가능).
func (c *ClipService) send(ctx context.Context, s *nbs.Session, content *clip.Content, done func(error)) (queued bool, err error) {
	gen := c.c.Gen()
	data, ext, err := c.clipFile(ctx, content)
	if err != nil {
		return false, err
	}
	if !c.c.PolicyChecker().CheckClipSize(int64(len(data))) {
		return false, nil
	}
	pick, err := PickApprover(ctx, s, approval.PickRequest{Files: []approval.PickFile{{Name: clipDisplay, Size: int64(len(data))}}, Clip: true})
	if err != nil {
		c.c.Transfers().pickFailed(err, false)
		return false, nil
	}
	if !c.c.IsCurrent(gen) {
		return false, nil
	}
	root := filepath.Join(tempDir(), "clip")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return false, err
	}
	dir, err := os.MkdirTemp(root, "up")
	if err != nil {
		return false, err
	}
	name := clipMarker + ext
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		os.RemoveAll(dir)
		return false, err
	}
	_, err = c.c.Transfers().EnqueueUpload(s, UploadRequest{Path: path, ServerName: name, Display: clipDisplay, Pick: pick, SkipPolicy: true,
		OnDone: func(err error) {
			os.RemoveAll(dir)
			if done != nil {
				done(err)
			}
		}})
	if err != nil {
		os.RemoveAll(dir)
		return false, err
	}
	c.log().Info(fmt.Sprintf("클립보드 보내기 >>> 대기열에 넣었다 형식[%s] 크기[%d]", ext, len(data)))
	return true, nil
}

// ── 받기 ──

// Download 는 받을 클립보드를 받아 이 PC 클립보드에 넣는다. 안내는 사건으로 나가고, 대기열에 넣으면 돌아온다.
func (c *ClipService) Download() error {
	c.download(false)
	return nil
}

// download 의 paste 는 단축키 받기다 ― 받아 넣은 뒤 Ctrl+V 를 흉내 낸다.
func (c *ClipService) download(paste bool) {
	s, err := c.c.Session()
	if err != nil {
		return
	}
	if c.count.Load() < 1 {
		c.info("다운로드 가능한 클립보드가 없습니다.")
		return
	}
	if !clipAllowed(s, false) {
		c.info("클립보드 다운로드가 제한되어 있습니다.")
		return
	}
	if !c.receiving.CompareAndSwap(false, true) {
		c.c.Emit(EvNotice, Notice{Severity: SevInfo, Text: "앞 클립보드를 받는 중입니다."})
		return
	}
	err = c.receive(s, func(res *transfer.DownloadResult, err error) {
		defer c.receiving.Store(false)
		if err != nil {
			return // 전송 쪽이 로그를 남겼고, 안내는 receive 가 한다
		}
		if c.deliver(res) && paste {
			go clip.SendCtrl('V') // 키를 뗄 때까지 기다린다 ― 대기열을 막지 않는다
		}
	})
	if err != nil {
		c.receiving.Store(false)
		c.info(err.Error())
		return
	}
	c.c.Emit(EvTransferOpen, nil)
}

// receive 는 클립보드 받기 한 건을 대기열에 넣는다. done 이 끝나면 임시 폴더를 지우고 목록을 새로 받는다. 받은 뒤 서버 원본은 전송 쪽이 지운다.
func (c *ClipService) receive(s *nbs.Session, done func(*transfer.DownloadResult, error)) error {
	root := filepath.Join(tempDir(), "clip")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(root, "down")
	if err != nil {
		return err
	}
	t := c.c.Transfers()
	_, err = t.EnqueueDownload(s, DownloadRequest{Fpath: clipMarker, Dest: filepath.Join(dir, clipMarker), Clip: true, Remove: true, Display: clipDisplay,
		OnDone: func(res *transfer.DownloadResult, err error) {
			defer os.RemoveAll(dir)
			if err != nil && !transfer.IsCanceled(err) && transfer.CodeOf(err) != transfer.CodeLoggedOut {
				t.reportFail(clipDisplay, err)
			}
			done(res, err)
			serverFiles.refresh()
		}})
	if err != nil {
		os.RemoveAll(dir)
	}
	return err
}

// deliver 는 받은 파일을 이 PC 클립보드에 넣는다. 묶음을 거부하면 아무것도 올리지 않고 로그만 남긴다(원본과 같다).
// 그림을 열 수 없으면 안내하고, 있던 클립보드는 그대로 둔다(원본은 먼저 비웠다). 단추·단축키가 같은 길을 지난다.
func (c *ClipService) deliver(res *transfer.DownloadResult) bool {
	log := c.log()
	data, err := os.ReadFile(res.Path)
	if err != nil {
		log.Warn("받은 클립보드 파일을 읽지 못했다", "err", err)
		c.info(err.Error())
		return false
	}
	in, err := clip.Interpret(data, filepath.Ext(res.Path))
	switch {
	case errors.Is(err, clip.ErrBundle):
		log.Warn("클립보드 수신 >>> 묶음 거부: " + err.Error())
		return false
	case err != nil:
		log.Warn("클립보드 수신 >>> 그림 거부: " + err.Error())
		c.dialog("클립보드 받기", "받은 클립보드 그림을 열 수 없습니다. PNG·JPEG·GIF·BMP·WEBP·TIFF 형식만 받을 수 있습니다.")
		return false
	}
	if err := in.Put(); err != nil {
		log.Warn("클립보드에 넣지 못했다", "err", err)
		c.info(err.Error())
		return false
	}
	if in.Scripts > 0 {
		log.Info(fmt.Sprintf("클립보드 수신 >>> 스크립트 %d 개를 지웠다", in.Scripts))
	}
	c.c.Emit(EvClipReceived, nil)
	return true
}

// ── 단축키 ──

// SetHotkey 는 단축키 설정을 저장하고 로그인 중이면 다시 등록한다(값이 바뀔 때만).
func (c *ClipService) SetHotkey(h config.Hotkey) error {
	if c.c.Settings().Get().Hotkey != h {
		if err := c.c.Settings().Update(func(s *config.Settings) { s.Hotkey = h }); err != nil {
			return err
		}
	}
	if _, err := c.c.Session(); err != nil {
		return nil // 로그인 때 등록한다
	}
	c.hk.Lock()
	same := c.active == h && (len(c.accels) > 0 || !h.Use)
	c.hk.Unlock()
	if same {
		return nil
	}
	return c.applyHotkey(h)
}

// applyHotkey 는 앞 등록을 지우고 h 를 등록한다. 한쪽만 실패하면 된 쪽은 남기고 오류를 돌려준다.
func (c *ClipService) applyHotkey(h config.Hotkey) error {
	app := application.Get()
	if app == nil {
		return nil // 시험
	}
	c.hk.Lock()
	defer c.hk.Unlock()
	for _, a := range c.accels {
		if err := app.GlobalShortcut.Unregister(a); err != nil { // 앱 종료 때는 Wails 가 먼저 모두 풀어 이미 없다
			c.log().Debug("단축키 해제", "accel", a, "err", err)
		}
	}
	c.accels, c.active = nil, h
	if !h.Use {
		return nil
	}
	cp, ps := clip.Accelerator(h.CopyKey, h.CopyChar), clip.Accelerator(h.PasteKey, h.PasteChar)
	if cp == "" || ps == "" {
		return errors.New("단축키 글자는 영문·숫자 하나여야 합니다")
	}
	if cp == ps {
		return errors.New("복사 단축키와 붙여넣기 단축키가 같습니다")
	}
	var errs []error
	for accel, fn := range map[string]func(){cp: func() { c.hotkey(true) }, ps: func() { c.hotkey(false) }} {
		if err := app.GlobalShortcut.Register(accel, fn); err != nil {
			c.log().Warn("단축키 등록 실패", "accel", accel, "err", err)
			errs = append(errs, fmt.Errorf("%s: 다른 프로그램이 이 단축키를 쓰고 있습니다", accel))
			continue
		}
		c.accels = append(c.accels, accel)
	}
	return errors.Join(errs...)
}

// hotkey 는 복사 단축키(선택한 내용을 복사해 보내기)·붙여넣기 단축키(받아서 붙여넣기)다.
func (c *ClipService) hotkey(copySel bool) {
	if !c.hotkeyBusy.CompareAndSwap(false, true) {
		return
	}
	defer c.hotkeyBusy.Store(false)
	if copySel {
		clip.CopySelection()
		c.upload()
		return
	}
	c.download(true)
}

func (c *ClipService) onLogin(*nbs.Session) {
	if err := c.applyHotkey(c.c.Settings().Get().Hotkey); err != nil {
		c.c.Emit(EvNotice, Notice{Severity: SevWarning, Text: err.Error()})
	}
}

func (c *ClipService) onLogout() {
	c.applyHotkey(config.Hotkey{})
	c.setCount(0)
}
