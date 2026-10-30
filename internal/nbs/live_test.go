package nbs

// 실 서버 로그인 시험. 환경변수가 없으면 SKIP 한다. 비밀번호는 환경변수로만 넘기고 출력하지 않는다.
//   SNB_APP_DIR=E:\git\snBv6FM\build\release  SNB_LIVE_AUTH_ID=…  SNB_LIVE_AUTH_PW=…  [SNB_LIVE_SERVER=0]
//   go test -run Live -v ./internal/nbs

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"snbapp/internal/config"
	"snbapp/internal/integrity"
)

func liveEnv(t *testing.T) (appDir string, srv config.Server, id, pw string) {
	id, pw = os.Getenv("SNB_LIVE_AUTH_ID"), os.Getenv("SNB_LIVE_AUTH_PW")
	if id == "" || pw == "" {
		t.Skip("SNB_LIVE_AUTH_ID·SNB_LIVE_AUTH_PW 가 없다")
	}
	appDir = config.AppDir()
	env, err := config.LoadEnv(appDir)
	if err != nil {
		t.Fatal(err)
	}
	idx, _ := strconv.Atoi(os.Getenv("SNB_LIVE_SERVER"))
	return appDir, env.Servers[idx], id, pw
}

func TestLiveLogin(t *testing.T) {
	appDir, srv, id, pw := liveEnv(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if testing.Verbose() {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	external := false                     // 시험 전부터 같은 계정의 다른 세션이 서버에 있었다(다른 PC·다른 시험)
	for round := 1; round <= 2; round++ { // 두 번째: 앞 세션을 Close 로 닫은 뒤 중복 로그인 없이 다시 들어가는가(AUTH-033)
		s, err := Login(appDir, srv, id, pw, "0.1.0", log)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		t.Logf("round %d: server=%s(%s) user=%s name=%q enc=%d ip=%s mac=%v multi=%v remain=%d defaultPw=%v aprv=%v replace=%v logoutAuto=%d profileKeys=%d",
			round, srv.Name, s.Addr, s.UserID, s.Info.UserName, s.EncMode, s.IP, s.MAC != "", s.MultiLogin, s.RemainDays,
			s.DefaultPassword, s.Info.AprvYn, s.Info.ReplaceYn, s.Info.FmLogoutAuto, len(s.Profile))
		switch {
		case s.MultiLogin && round == 1:
			external = true
			t.Logf("round 1: 시험 전부터 같은 계정의 다른 세션이 있다 ― Close 뒤 재로그인 판정은 할 수 없다")
		case s.MultiLogin && !external:
			t.Errorf("round %d: 앞 세션을 닫았는데 중복 로그인으로 판정됐다", round)
		}
		if round == 1 {
			ctx, cancel := s.Ctx(RPCTimeout)
			f, skipped := integrity.Check(ctx, s.Common(), integrity.Files(appDir), integrity.KindStart, s.Server.Loc, log)
			cancel()
			t.Logf("무결성(서버 미등록이라 실패가 정상): skipped=%v failures=%v", skipped, f)
		}
		s.Close()
	}
}

// 세션을 연 채 같은 계정으로 또 로그인하면 로그인 응답의 중복 표시(fm_login_multi·multi_login_flag)를 본다.
// 서버 정책(다중 로그인 허용 여부)에 따라 결과가 달라 값만 남긴다.
func TestLiveDuplicate(t *testing.T) {
	appDir, srv, id, pw := liveEnv(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s1, err := Login(appDir, srv, id, pw, "0.1.0", log)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	s2, err := Login(appDir, srv, id, pw, "0.1.0", log)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	t.Logf("fm_login_multi=%d multi_login_flag=%d → MultiLogin=%v", s2.Info.FmLoginMulti, s2.Info.MultiLoginFlag, s2.MultiLogin)
}

// 없는 ID 는 계정 실패 한 문구 + 창을 닫으면 종료(AUTH-021). 실제 계정의 실패 횟수를 늘리지 않는다.
func TestLiveUnknownID(t *testing.T) {
	appDir, srv, _, _ := liveEnv(t)
	var w io.Writer = io.Discard
	if testing.Verbose() {
		w = os.Stderr // 서버가 보낸 상태 코드·문구(로그인 실패 상세)를 본다
	}
	_, err := Login(appDir, srv, "zz_no_such_user_wails", "x", "0.1.0", slog.New(slog.NewTextHandler(w, nil)))
	le, ok := err.(*LoginError)
	if !ok || le.Msg != MsgAccount || !le.Exit {
		t.Fatalf("got %#v", err)
	}
}
