package policy

// 실 서버 정책 조회 시험(읽기만 ― 불일치 기록[3316]도 보내지 않는다). 환경변수가 없으면 SKIP. 비밀번호는 환경변수로만 받고 출력하지 않는다.
//   SNB_APP_DIR=E:\git\snBv6FM\build\release  SNB_LIVE_POLICY_ID=…  SNB_LIVE_POLICY_PW=…  [SNB_LIVE_SERVER=0]
//   go test -run Live -v ./internal/policy

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"snbapp/internal/config"
	"snbapp/internal/nbs"
)

func TestLivePolicyFetch(t *testing.T) {
	id, pw := os.Getenv("SNB_LIVE_POLICY_ID"), os.Getenv("SNB_LIVE_POLICY_PW")
	if id == "" || pw == "" {
		t.Skip("SNB_LIVE_POLICY_ID·SNB_LIVE_POLICY_PW 가 없다")
	}
	appDir := config.AppDir()
	env, err := config.LoadEnv(appDir)
	if err != nil {
		t.Fatal(err)
	}
	idx, _ := strconv.Atoi(os.Getenv("SNB_LIVE_SERVER"))
	srv := env.Servers[idx]
	s, err := nbs.Login(appDir, srv, id, pw, "0.1.0", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	set := NewSet(s.Server.Loc, int64(s.Info.FmFileSize), int(s.Info.FmFileCount), int64(s.Info.FmClipSize)).WithProfile(s.Profile)
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	ext, err := FetchExtInfo(ctx, s.FileExtension())
	cancel()
	if err != nil {
		t.Fatalf("3005: %v", err)
	}
	ctx, cancel = s.Ctx(nbs.RPCTimeout)
	patt, err := FetchPatterns(ctx, s.FileExtension())
	cancel()
	if err != nil {
		t.Fatalf("3015: %v", err)
	}
	var warns int
	set = set.WithExtInfo(ext).WithPatterns(patt, func(w string) { warns++; t.Log(w) })
	if !set.Ready() {
		t.Fatal("not ready")
	}
	depth, clamped, _ := set.DepthFromPolicy()
	t.Logf("server=%s loc=%d user=%s method=%s fileMaxKB=%d fileMaxCount=%d clipMaxKB=%d profileKeys=%d",
		srv.Name, set.Loc, s.UserID, set.MethodName(), set.FileSizeKB, set.FileCount, set.ClipSizeKB, len(set.Profile()))
	t.Logf("3005 응답 %d 행, 이 망 적용 %d 행 / 3015 응답 %d 행, 저장 %d 행, 버린 행 %d",
		len(ext), len(set.ExtRows()), set.PattResp, set.PattStored, warns)
	dz, _ := set.Value("fm_disable_zip_pass")
	uz, _ := set.Value("fm_use_Zip_pwd")
	t.Logf("insideZip=%v depth=%d(clamped=%v) fm_disable_zip_pass=%q fm_use_Zip_pwd=%q no_ext_no_permit=%d",
		set.InsideCheck(), depth, clamped, dz, uz, set.Int("no_ext_no_permit"))
	for _, c := range []struct {
		name string
		head []byte
	}{{"a.txt", []byte("hello world")}, {"a.pdf", []byte("%PDF-1.7")}, {"a.exe", []byte("MZ\x90\x00")}, {"a.zip", []byte("PK\x03\x04")}, {"renamed.txt", []byte("MZ\x90\x00")}} {
		code := CheckEntry(set, c.name, int64(len(c.head)), c.head)
		t.Logf("판정 %-12s → %d %s", c.name, code, PolicyText(code))
	}
}
