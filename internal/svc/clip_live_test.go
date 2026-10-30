package svc

// 클립보드·링크·도메인 실 서버 시험. 환경변수가 없으면 SKIP. 비밀번호는 환경변수로만.
//   SNB_APP_DIR=E:\git\snBv6FM\build\release  SNB_LIVE_CLIP_ID=test8_1  SNB_LIVE_CLIP_PW=…
//   go test -run LiveClip -v -count=1 ./internal/svc
//
// 이 PC 클립보드는 건드리지 않는다: 보내기는 Content 를 직접 만들고, 받기는 clip.Interpret 결과를 비교한다.

import (
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"snbapp/internal/approval"
	"snbapp/internal/clip"
	"snbapp/internal/config"
	"snbapp/internal/domain"
	"snbapp/internal/files"
	"snbapp/internal/link"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
	"snbapp/pb/serverfile"
)

func TestLiveClip(t *testing.T) {
	id, pw := os.Getenv("SNB_LIVE_CLIP_ID"), os.Getenv("SNB_LIVE_CLIP_PW")
	if id == "" || pw == "" {
		t.Skip("SNB_LIVE_CLIP_ID·SNB_LIVE_CLIP_PW 가 없다")
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := func(loc int) config.Server {
		for _, s := range env.Servers {
			if s.Loc == loc {
				return s
			}
		}
		t.Fatalf("LOC %d 서버가 없다", loc)
		return config.Server{}
	}
	buf := &syncBuf{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	c := NewCore("0.1.0-test", log)
	c.appDir, c.env = dir, env
	if c.settings, err = config.Open(filepath.Join(t.TempDir(), "settings.json")); err != nil {
		t.Fatal(err)
	}
	c.startOnce.Do(func() {}) // 개발 PC 의 진짜 설정을 건드리지 않는다
	newFilesService(c)
	newTransferService(c)
	newPolicyService(c)
	newApprovalService(c)
	cs := newClipService(c)
	newLogService(c)
	l := newLinkService(c)

	login := func(loc int) *nbs.Session {
		t.Helper()
		s, err := nbs.Login(dir, server(loc), id, pw, "0.1.0-test", log)
		if err != nil {
			t.Fatalf("로그인(LOC %d): %v", loc, err)
		}
		c.login(s)
		waitFor(t, "정책 받기", 15*time.Second, func() bool { return c.PolicyChecker().Current() != nil })
		time.Sleep(time.Second) // 프로파일·확장자까지
		return s
	}
	// 끝나면 양쪽 망의 내 __CLIP__ 행을 지운다
	sweep := func(s *nbs.Session) {
		list, err := files.List(s)
		if err != nil {
			t.Errorf("목록: %v", err)
			return
		}
		for _, f := range list {
			if files.IsClip(f) {
				if err := files.Remove(s, f.ServerStr, false); err != nil {
					t.Errorf("시험 파일을 지우지 못했다: %v", err)
				}
			}
		}
	}
	defer func() {
		c.Logout(ReasonLogout)
		for _, loc := range []int{config.LocBusiness, config.LocInternet} {
			if s, err := nbs.Login(dir, server(loc), id, pw, "0.1.0-test", log); err == nil {
				sweep(s)
				s.Close()
			}
		}
		if testing.Verbose() {
			t.Log("\n" + buf.String())
		}
	}()

	// 앞서 남은 건이 있으면 받는 건이 섞인다
	s := login(config.LocBusiness)
	sweep(s)
	t.Logf("업무망: 사전결재 %v, 클립보드 결재 방식 %d, 보내기 권한 %v, 받기 권한 %v", approval.PreMode(s.Profile), s.AprvType(true), clipAllowed(s, true), clipAllowed(s, false))
	if approval.PreMode(s.Profile) && approval.NeedsApprover(s.AprvType(true)) {
		t.Skip("사전결재 모드라 결재자 창이 필요하다")
	}

	// 읽기만 하는 정책 호출
	l.loadFilter()
	t.Logf("URL 필터 %d 건", len(l.urlFilter()))
	if d, err := domain.Fetch(s); err != nil {
		t.Errorf("[3230]: %v", err)
	} else {
		t.Logf("도메인 정책 %d 건, 쓸 수 있는 행 %d", len(d), len(domain.Rows(d, nil)))
	}

	stamp := time.Now().Format("150405")
	text := "클립보드 왕복 시험 " + stamp + "\r\n둘째 줄 ABC"
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for y := range 3 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{uint8(40 * x), uint8(50 * y), 200, 255})
		}
	}
	local := filepath.Join(t.TempDir(), "그림.png")
	p, err := clip.EncodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, p, 0o644); err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf(`<html><body><p>서식 %s <b>굵게</b></p><img src="file:///%s"><script>alert(1)</script></body></html>`, stamp, filepath.ToSlash(local))

	samePixels := func(got image.Image) error {
		if got.Bounds().Size() != img.Bounds().Size() {
			return fmt.Errorf("크기 %v", got.Bounds())
		}
		for y := range 3 {
			for x := range 4 {
				r1, g1, b1, _ := got.At(got.Bounds().Min.X+x, got.Bounds().Min.Y+y).RGBA()
				r2, g2, b2, _ := img.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 {
					return fmt.Errorf("(%d,%d) 색이 다르다", x, y)
				}
			}
		}
		return nil
	}

	cases := []struct {
		name    string
		format  string
		content *clip.Content
		check   func(*clip.Incoming) error
	}{
		{"글", "", &clip.Content{Kind: clip.KindText, Text: text}, func(in *clip.Incoming) error {
			if in.Kind != clip.KindText || in.Text != text {
				return fmt.Errorf("%+v", in)
			}
			return nil
		}},
		{"그림 bmp", "bmp", &clip.Content{Kind: clip.KindImage, DIB: clip.ImageToDIB(img)}, func(in *clip.Incoming) error {
			if in.Kind != clip.KindImage || in.Format != "bmp" {
				return fmt.Errorf("%+v", in)
			}
			return samePixels(in.Image)
		}},
		{"그림 png", "png", &clip.Content{Kind: clip.KindImage, PNG: p, DIB: clip.ImageToDIB(img)}, func(in *clip.Incoming) error {
			if in.Kind != clip.KindImage || in.Format != "png" {
				return fmt.Errorf("%+v", in)
			}
			return samePixels(in.Image)
		}},
		{"HTML", "", &clip.Content{Kind: clip.KindHTML, CFHTML: clip.BuildCFHTML(src)}, func(in *clip.Incoming) error {
			if in.Kind != clip.KindHTML || !strings.Contains(in.HTML, "<b>굵게</b>") || strings.Contains(in.HTML, "<script") ||
				!strings.Contains(in.HTML, "data:image/png;base64,") || strings.Contains(in.HTML, "file:") {
				return fmt.Errorf("%+v", in)
			}
			if !strings.Contains(in.Text, "서식 "+stamp) {
				return fmt.Errorf("평문: %q", in.Text)
			}
			return nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := c.mustSession(t)
			c.Settings().Update(func(x *config.Settings) { x.ClipImageFormat = tc.format })
			tr := c.Transfers()
			ctx, cancel, token, err := tr.claim(false)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			queued, err := cs.send(ctx, s, tc.content, func(err error) { done <- err })
			tr.release(token, cancel)
			if err != nil || !queued {
				t.Fatalf("보내기: %v %v", queued, err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("업로드: %v", err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("업로드가 끝나지 않았다")
			}
			c.Logout(ReasonLogout)

			in := login(config.LocInternet)
			// 서버는 요청마다 연결 버퍼 128MiB 를 잡는다. 50ms 간격으로 조회하면 17초에 서버 메모리가 2GB 가 된다(10-30 측정)
			pollFiles := func(ok func() bool) func() bool {
				return func() bool { time.Sleep(2 * time.Second); serverFiles.refresh(); return ok() }
			}
			waitFor(t, "받을 건수", 60*time.Second, pollFiles(func() bool { return cs.count.Load() > 0 }))
			got := make(chan *clip.Incoming, 1)
			fail := make(chan error, 1)
			err = cs.receive(in, func(res *transfer.DownloadResult, err error) {
				if err != nil {
					fail <- err
					return
				}
				data, err := os.ReadFile(res.Path)
				if err != nil {
					fail <- err
					return
				}
				inc, err := clip.Interpret(data, filepath.Ext(res.Path))
				if err != nil {
					fail <- fmt.Errorf("%w (%s)", err, filepath.Base(res.Path))
					return
				}
				t.Logf("받음: %s %d 바이트 서버경로 %q 검사값 %q", filepath.Base(res.Path), len(data), res.ServerPath, res.Checksum)
				got <- inc
			})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case inc := <-got:
				if err := tc.check(inc); err != nil {
					t.Error(err)
				}
			case err := <-fail:
				t.Errorf("받기: %v", err)
			case <-time.After(30 * time.Second):
				t.Error("받기가 끝나지 않았다")
			}
			waitFor(t, "받은 뒤 건수 0", 20*time.Second, pollFiles(func() bool { return cs.count.Load() == 0 }))
			if list, err := files.List(in); err == nil {
				t.Logf("받은 뒤 인터넷망 __CLIP__ 행: %d", countClipRows(list))
			}
			c.Logout(ReasonLogout)
			login(config.LocBusiness)
		})
	}

	// 링크: 업무망에서 [3204] 로 보내면 인터넷망 세션이 9004 kind 4 를 받는다
	t.Run("URL 보내기", func(t *testing.T) {
		biz := c.mustSession(t)
		c.Logout(ReasonLogout)
		sender, err := nbs.Login(dir, biz.Server, id, pw, "0.1.0-test", log)
		if err != nil {
			t.Fatal(err)
		}
		defer sender.Close()
		login(config.LocInternet)
		time.Sleep(time.Second) // 스트림이 붙을 때까지
		if err := link.SendURL(sender, "https://example.com/Wails/Test?stamp="+stamp); err != nil {
			t.Fatalf("[3204]: %v", err)
		}
		waitFor(t, "9004 kind 4", 15*time.Second, func() bool { return strings.Contains(buf.String(), "업무망에서 전달된 URL 수신:") })
		if strings.Contains(buf.String(), "Wails/Test") && !strings.Contains(buf.String(), "dat[") {
			t.Error("주소 전체가 로그에 남았다")
		}
	})
}

func countClipRows(list []*serverfile.FileInfo) int {
	n := 0
	for _, f := range list {
		if files.IsClip(f) {
			n++
		}
	}
	return n
}

func (c *Core) mustSession(t *testing.T) *nbs.Session {
	t.Helper()
	s, err := c.Session()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
