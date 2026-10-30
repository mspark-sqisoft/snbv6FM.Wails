package svc

// 알림 스트림의 세션 흐름 실 서버 시험(알림·로그 영역). 환경변수가 없으면 SKIP 한다. 비밀번호는 환경변수로만.
//   SNB_APP_DIR=E:\git\snBv6FM\build\release  SNB_LIVE_NOTIFY_ID=test7_1  SNB_LIVE_NOTIFY_PW=…
//   go test -run LiveNotifyFlow -v ./internal/svc
//
// 보는 것: 로그인 → 스트림 열림 → 업로드에 9001 수신 / 연결이 끊기면 끊김 처리(로그아웃) /
// 중복 로그인 알림(9004 kind 6)에 로그아웃 / 로그아웃 뒤 온 알림은 버림 / 남의 userid 알림은 버림.

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/notify"
	"snbapp/internal/transfer/transfertest"
	pbn "snbapp/pb/notify"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitFor(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("%s: %v 안에 되지 않았다", what, d)
}

func TestLiveNotifyFlow(t *testing.T) {
	id, pw := os.Getenv("SNB_LIVE_NOTIFY_ID"), os.Getenv("SNB_LIVE_NOTIFY_PW")
	if id == "" || pw == "" {
		t.Skip("SNB_LIVE_NOTIFY_ID·SNB_LIVE_NOTIFY_PW 가 없다")
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	var srv config.Server
	for _, s := range env.Servers {
		if s.Loc == config.LocBusiness {
			srv = s
		}
	}
	buf := &syncBuf{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := NewCore("0.1.0-test", log)
	l := newLogService(c)
	h := hubOf(c)
	login := func() *nbs.Session {
		s, err := nbs.Login(dir, srv, id, pw, "0.1.0-test", log)
		if err != nil {
			t.Fatal(err)
		}
		c.login(s)
		return s
	}
	defer func() {
		c.Logout(ReasonShutdown)
		if testing.Verbose() {
			t.Log("\n" + buf.String())
		}
	}()

	// 1) 로그인 → 스트림 → 업로드하면 9001
	s := login()
	time.Sleep(time.Second)
	if strings.Contains(buf.String(), "Notify Stream finished") {
		t.Fatal("스트림이 바로 끝났다")
	}
	name := "E2E_WAILS_NOTIFYFLOW_" + time.Now().Format("150405") + ".txt"
	src := filepath.Join(t.TempDir(), name)
	os.WriteFile(src, []byte("notify flow\n"), 0o644)
	if err := transfertest.Upload(s, src); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "9001 수신", 10*time.Second, func() bool { return strings.Contains(buf.String(), "Notify 수신: 전문번호[9001]") })
	if list, err := files.List(s); err == nil {
		for _, f := range list {
			if f.NameStr == name {
				if err := files.Remove(s, f.ServerStr, false); err != nil {
					t.Errorf("시험 파일을 지우지 못했다: %v", err)
				}
			}
		}
	}

	// 2) 남의 userid 알림은 버린다
	l.onNotify(h, c.Gen(), s.UserID, &pbn.NotifyMessage{NotifySvcno: notify.SvcFileList, Userid: "someone_else"})
	if !strings.Contains(buf.String(), "Notify Ignored: Different UserID ( someone_else  vs  "+s.UserID+" )") {
		t.Error("남의 userid 알림을 버리지 않았다")
	}

	// 3) 연결이 끊기면(우리가 로그아웃하지 않았는데 스트림이 끝남) 끊김 처리로 로그아웃
	oldGen := c.Gen()
	s.Close()
	waitFor(t, "끊김 처리", 10*time.Second, func() bool { _, err := c.Session(); return err == ErrNotLoggedIn })
	if !strings.Contains(buf.String(), "gRPC notify 스트림 비정상 종료 감지") || !strings.Contains(buf.String(), "reason=disconnected") {
		t.Error("끊김 로그가 없다")
	}

	// 4) 로그아웃 뒤 늦게 온 알림은 버린다
	l.onNotify(h, oldGen, s.UserID, &pbn.NotifyMessage{NotifySvcno: notify.SvcNotice, Msg: "늦은 공지"})
	if !strings.Contains(buf.String(), "로그아웃 뒤 도착한 알림을 버립니다 >>> 종류: 9007 내용: 늦은 공지") {
		t.Error("늦은 알림을 버리지 않았다")
	}

	// 5) 다시 로그인 → 중복 로그인 알림(시험 진입점)에 로그아웃, 우리 스트림 끝은 끊김으로 보지 않는다
	before := strings.Count(buf.String(), "비정상 종료 감지")
	s2 := login()
	time.Sleep(500 * time.Millisecond)
	t.Setenv("SNB_TEST_HOOKS", "1")
	if err := l.InjectNotify(notify.SvcEvent, notify.KindDupFM, notify.SideSame, s2.UserID, "", ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "중복 로그인 로그아웃", 5*time.Second, func() bool { _, err := c.Session(); return err == ErrNotLoggedIn })
	time.Sleep(500 * time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "동일사용자 다중로그인 감지 후 종료 >>> IP: ") || !strings.Contains(out, "reason=duplicate") {
		t.Error("중복 로그인 처리 로그가 없다")
	}
	if strings.Count(out, "비정상 종료 감지") != before {
		t.Error("우리가 끊은 스트림을 끊김으로 봤다")
	}
}
