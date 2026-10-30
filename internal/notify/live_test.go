package notify

// 실 서버 알림 시험. 환경변수가 없으면 SKIP 한다. 비밀번호는 환경변수로만 넘기고 출력하지 않는다.
//   SNB_APP_DIR=E:\git\snBv6FM\build\release
//   SNB_LIVE_NOTIFY_ID/PW    올리는 계정(test7_1)
//   SNB_LIVE_NOTIFY_ID2/PW2  함께 알림을 받는 계정(test7_2, 결재자)
//   [SNB_LIVE_LOC=1]         붙을 망의 LOC(1 업무망)
//   go test -run Live -v ./internal/notify
//
// 둘 다 알림 스트림을 연 뒤 test7_1 이 파일을 올리면 알림(9001 또는 9004·9002)이 오는지 본다. 올린 파일은 지운다.

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer/transfertest"
	pb "snbapp/pb/notify"
)

// sendOnce 는 test7_1 이 NotifyOnce 로 test7_2 에게 결재 목록 갱신(9004 kind 1, 화면에 창을 띄우지 않는 종류)을 보내 받는 쪽 스트림에 닿는지 본다.
// 서버가 NotifyOnce 를 userid 로 전달하는지는 서버 동작이라 오지 않아도 실패로 보지 않고 남기기만 한다.
func sendOnce(t *testing.T, from *nbs.Session, to string, ch chan got) {
	ctx, cancel := from.Ctx(nbs.RPCTimeout)
	defer cancel()
	m := &pb.NotifyMessage{NotifySvcno: SvcEvent, Kind: KindFileAprv, Side: SideSame, Userid: to}
	if _, err := from.Notify().NotifyOnce(ctx, m); err != nil {
		t.Errorf("NotifyOnce: %v", err)
		return
	}
	timeout := time.After(5 * time.Second)
	for {
		select {
		case g := <-ch:
			t.Logf("NotifyOnce 뒤 받음 %s → %s", g.who, ReceivedLine(g.m))
			if g.who == to && g.m.NotifySvcno == SvcEvent && g.m.Kind == KindFileAprv {
				return
			}
		case <-timeout:
			t.Logf("NotifyOnce(%s 에게 9004 kind 1)가 5초 안에 받는 쪽 스트림에 오지 않았다", to)
			return
		}
	}
}

type got struct {
	who string
	m   *pb.NotifyMessage
}

func TestLiveNotify(t *testing.T) {
	id1, pw1 := os.Getenv("SNB_LIVE_NOTIFY_ID"), os.Getenv("SNB_LIVE_NOTIFY_PW")
	id2, pw2 := os.Getenv("SNB_LIVE_NOTIFY_ID2"), os.Getenv("SNB_LIVE_NOTIFY_PW2")
	if id1 == "" || pw1 == "" || id2 == "" || pw2 == "" {
		t.Skip("SNB_LIVE_NOTIFY_ID/PW·ID2/PW2 가 없다")
	}
	loc := config.LocBusiness
	if v := os.Getenv("SNB_LIVE_LOC"); v != "" {
		loc, _ = strconv.Atoi(v)
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	var srv *config.Server
	for i := range env.Servers {
		if env.Servers[i].Loc == loc {
			srv = &env.Servers[i]
		}
	}
	if srv == nil {
		t.Fatalf("LOC=%d 서버가 없다", loc)
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	var logw io.Writer = io.Discard
	if testing.Verbose() {
		logw = os.Stderr
	}
	streamLog := slog.New(slog.NewTextHandler(logw, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ch := make(chan got, 64)
	var wg sync.WaitGroup
	open := func(id, pw string) (*nbs.Session, context.CancelFunc) {
		s, err := nbs.Login(dir, *srv, id, pw, "0.1.0-test", quiet)
		if err != nil {
			t.Fatalf("%s 로그인: %v", id, err)
		}
		ctx, cancel := s.Ctx(0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Run(ctx, s.Notify(), streamLog, func(m *pb.NotifyMessage) { ch <- got{id, m} })
			t.Logf("%s 스트림 끝: %v", id, err)
		}()
		return s, cancel
	}
	s1, cancel1 := open(id1, pw1)
	s2, cancel2 := open(id2, pw2)
	defer func() {
		cancel1()
		cancel2()
		s1.Close()
		s2.Close()
		wg.Wait()
	}()
	t.Logf("로그인 %s·%s(%s) 결재 방식=%d", s1.UserID, s2.UserID, srv.Name, s1.AprvType(false))

	// 스트림이 서버에 열렸는지: 1초 안에 끝나지 않으면 열린 것으로 본다(끝났으면 Run 이 이미 돌아와 로그가 남는다).
	time.Sleep(time.Second)

	name := "E2E_WAILS_NOTIFY_" + time.Now().Format("150405") + ".txt"
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, []byte("notify live test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := transfertest.Upload(s1, src); err != nil {
		t.Fatalf("업로드: %v", err)
	}
	defer func() { // 시험 파일을 지운다
		list, err := files.List(s1)
		if err != nil {
			t.Errorf("목록: %v", err)
			return
		}
		for _, f := range list {
			if f.NameStr == name {
				if err := files.Remove(s1, f.ServerStr, false); err != nil {
					t.Errorf("시험 파일을 지우지 못했다(%s): %v", f.ServerStr, err)
				} else {
					t.Logf("시험 파일 지움 %s", f.ServerStr)
				}
			}
		}
	}()

	var seen []string
	deadline := time.After(15 * time.Second)
	for {
		select {
		case g := <-ch:
			seen = append(seen, g.who+":"+strconv.Itoa(int(g.m.NotifySvcno))+"/"+strconv.Itoa(int(g.m.Kind)))
			t.Logf("받음 %s → %s", g.who, ReceivedLine(g.m))
			if Ignore(g.m, g.who) == "" && (g.m.NotifySvcno == SvcFileList || g.m.NotifySvcno == SvcApprovalReq || g.m.NotifySvcno == SvcEvent) {
				time.Sleep(2 * time.Second) // 뒤따르는 알림도 모아 로그에 남긴다
				for len(ch) > 0 {
					g := <-ch
					t.Logf("받음 %s → %s", g.who, ReceivedLine(g.m))
				}
				sendOnce(t, s1, s2.UserID, ch)
				return
			}
		case <-deadline:
			t.Fatalf("업로드 뒤 15초 안에 알림이 오지 않았다(받은 것: %s)", strings.Join(seen, ", "))
		}
	}
}
