package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 서버 한 칸(IP 하나)을 짧게 적는다(Index 는 정렬 뒤 자리).
func srv(i int, name, ip string, port, loc int) Server {
	return Server{Index: i, Name: name, Network: "NONAME", IP: ip, IPs: []string{ip}, Port: port, Loc: loc}
}

func TestLoadEnvServers(t *testing.T) {
	cases := []struct {
		name  string
		fmenv string
		local string // 빈 값이면 fmlocal.ini 없음
		want  []Server
	}{
		{"BOM·CRLF·주석·다른 구획 무시",
			"\xEF\xBB\xBF# 주석\r\n[SERVER01]\r\nIP=10.0.0.1\r\nNAME=업무망\r\nLOC=1\r\nPORT=29002\r\n\r\n[CONFIG]\r\nIP=9.9.9.9\r\n[SERVER02]\r\nIP = 10.0.0.2\r\nPORT=1234\r\nLOC=0\r\n", "",
			[]Server{srv(0, "업무망", "10.0.0.1", 29002, 1), srv(1, "NONAME", "10.0.0.2", 1234, 0)}},
		{"SERVER01 IP 가 비면 0개", "[SERVER01]\nIP=\n[SERVER02]\nIP=10.0.0.2\n", "", nil},
		{"빈 구획에서 멈춘다(03 은 안 읽힌다)", "[SERVER01]\nIP=10.0.0.1\n[SERVER03]\nIP=10.0.0.3\n", "",
			[]Server{srv(0, "NONAME", "10.0.0.1", 29001, 1)}},
		{"IP=_ 는 그 구획만 건너뛴다", "[SERVER01]\nIP=_\n[SERVER02]\nIP=10.0.0.2\n", "",
			[]Server{srv(0, "NONAME", "10.0.0.2", 29001, 1)}},
		{"`_` 로 여러 IP·앞뒤 공백", "[SERVER01]\nIP= 10.0.0.1 _ 10.0.0.9 \n", "",
			[]Server{{Index: 0, Name: "NONAME", Network: "NONAME", IP: "10.0.0.1 _ 10.0.0.9", IPs: []string{"10.0.0.1", "10.0.0.9"}, Port: 29001, Loc: 1}}},
		{"LOC 이상·빈 값은 업무망", "[SERVER01]\nIP=1.1.1.1\nLOC=abc\n[SERVER02]\nIP=2.2.2.2\nLOC=\n[SERVER03]\nIP=3.3.3.3\nLOC=2\n", "",
			[]Server{srv(0, "NONAME", "1.1.1.1", 29001, 1), srv(1, "NONAME", "2.2.2.2", 29001, 1), srv(2, "NONAME", "3.3.3.3", 29001, 1)}},
		{"PORT 가 숫자가 아니면 0", "[SERVER01]\nIP=1.1.1.1\nPORT=abc\n", "",
			[]Server{srv(0, "NONAME", "1.1.1.1", 0, 1)}},
		{"NETWORK 키", "[SERVER01]\nIP=1.1.1.1\nNETWORK=업무망->인터넷망\n", "",
			[]Server{{Index: 0, Name: "NONAME", Network: "업무망->인터넷망", IP: "1.1.1.1", IPs: []string{"1.1.1.1"}, Port: 29001, Loc: 1}}},
		{"fmlocal LOC=0 이면 인터넷망을 맨 앞", "[SERVER01]\nIP=1.1.1.1\nLOC=1\n[SERVER02]\nIP=2.2.2.2\nLOC=0\n", "[NETWORK]\nLOC=0\n",
			[]Server{srv(0, "NONAME", "2.2.2.2", 29001, 0), srv(1, "NONAME", "1.1.1.1", 29001, 1)}},
		{"fmlocal LOC 이상하면 fmenv 순서", "[SERVER01]\nIP=1.1.1.1\nLOC=0\n[SERVER02]\nIP=2.2.2.2\nLOC=1\n", "[NETWORK]\nLOC=x\n",
			[]Server{srv(0, "NONAME", "1.1.1.1", 29001, 0), srv(1, "NONAME", "2.2.2.2", 29001, 1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "fmenv.ini", c.fmenv)
			if c.local != "" {
				writeFile(t, dir, "fmlocal.ini", c.local)
			}
			env, err := LoadEnv(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(env.Servers) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(env.Servers, c.want) {
				t.Fatalf("got  %+v\nwant %+v", env.Servers, c.want)
			}
		})
	}
}

func TestLoadEnvLocalAndValue(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadEnv(dir); err == nil {
		t.Fatal("fmenv.ini 가 없으면 오류여야 한다")
	}
	writeFile(t, dir, "fmenv.ini", "[CONFIG]\nSERIAL_TRANSFER_SIZE=1024\n")
	env, err := LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	if env.Home != -1 || env.LogLevel != "DEBUG" || env.LogKeepDays != 15 || env.LogMaxMB != 200 {
		t.Fatalf("fmlocal 기본값: %+v", env)
	}
	if v, ok := env.Value("CONFIG", "SERIAL_TRANSFER_SIZE"); !ok || v != "1024" {
		t.Fatalf("Value = %q %v", v, ok)
	}
	writeFile(t, dir, "fmlocal.ini", "[NETWORK]\nLOC=1\n[LOG]\nLEVEL= info \nKEEP_DAYS=0\nMAX_MB=0\n")
	env, _ = LoadEnv(dir)
	if env.Home != 1 || env.LogLevel != "INFO" || env.LogKeepDays != 0 || env.LogMaxMB != 0 {
		t.Fatalf("fmlocal: %+v", env)
	}
}

func TestUserINI(t *testing.T) {
	app, data := t.TempDir(), t.TempDir()
	writeFile(t, app, "fmenv.ini", "[USER]\nserverIndex=1\nautoUploadPeriod_0=10\n[SERVER01]\nIP=1.1.1.1\n")
	// 원본 Qt 가 쓴 꼴: 경로의 \ 는 \\, 쉼표가 든 값은 큰따옴표
	writeFile(t, data, "fmusr.ini", "[USER]\nautoUploadPeriod_0=30\nautoUploadFolder_0=C:/Users/x/Desktop\\\\AUTO_UPLOAD\nautoDownloadFolder_0=\"C:\\\\a,b\"\n[SERVER01]\nIP=6.6.6.6\n")
	got := UserINI(app, data)
	want := map[string]string{
		"serverIndex":          "1",
		"autoUploadPeriod_0":   "30",
		"autoUploadFolder_0":   `C:/Users/x/Desktop\AUTO_UPLOAD`,
		"autoDownloadFolder_0": `C:\a,b`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", SettingsFile)
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get(); !reflect.DeepEqual(got, Defaults()) {
		t.Fatalf("기본값이 아니다: %+v", got)
	}
	var calls int
	var seen Settings
	st.OnChange(func(old, cur Settings) { calls++; seen = cur })

	if err := st.Update(func(s *Settings) {
		s.LogoutType = 2
		s.Auto[1] = AutoTransfer{UploadUse: true, UploadPeriod: 5, UploadFolder: `C:\up`}
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(s *Settings) { s.LogoutType = 2 }); err != nil { // 바뀐 것 없음 → 콜백 없음
		t.Fatal(err)
	}
	if calls != 1 || seen.LogoutType != 2 {
		t.Fatalf("콜백 %d번, %+v", calls, seen)
	}
	g := st.Get()
	g.Auto[1] = AutoTransfer{} // 사본을 고쳐도 저장소는 그대로
	if st.Get().Auto[1].UploadFolder != `C:\up` {
		t.Fatal("Get 이 사본이 아니다")
	}

	st2, err := Open(path) // 다시 읽기
	if err != nil {
		t.Fatal(err)
	}
	if got := st2.Get(); got.LogoutType != 2 || got.Auto[1].UploadPeriod != 5 || !got.LogoutConfirm || got.Theme != "system" {
		t.Fatalf("다시 읽은 값: %+v", got)
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(m) != 0 {
		t.Fatalf("임시 파일이 남았다: %v", m)
	}

	if err := os.WriteFile(path, []byte("{깨짐"), 0o644); err != nil {
		t.Fatal(err)
	}
	st3, err := Open(path)
	if err == nil || st3 == nil || st3.Get().Layout != 2 {
		t.Fatalf("깨진 파일: err=%v", err)
	}
	if _, err := os.Stat(path + ".bad"); err != nil {
		t.Fatal("깨진 파일을 .bad 로 옮기지 않았다")
	}
}

func setBuild(t *testing.T, dev, seal bool) {
	oldDev, oldSeal := devBuild, sealed
	devBuild, sealed = dev, seal
	t.Cleanup(func() { devBuild, sealed = oldDev, oldSeal })
}

// 배포 빌드는 SNB_DEV 를 무시하고, -tags dev 는 SNB_DEV 없이도 개발 빌드다.
func TestIsDevBuilds(t *testing.T) {
	t.Setenv("SNB_INSTANCE", "a")
	t.Setenv("SNB_DEV", "1")
	setBuild(t, false, true)
	if IsDev() || Instance() != "" {
		t.Errorf("배포 빌드: IsDev=%v Instance=%q", IsDev(), Instance())
	}
	t.Setenv("SNB_DEV", "")
	setBuild(t, true, false)
	if !IsDev() || Instance() != "a" {
		t.Errorf("-tags dev: IsDev=%v Instance=%q", IsDev(), Instance())
	}
}

func TestInstance(t *testing.T) {
	setBuild(t, false, false)
	for _, c := range []struct{ dev, name, want string }{
		{"1", "int", "int"},
		{"1", "a_b-1", "a_b-1"},
		{"", "int", ""},   // 개발 표시가 없으면 무시
		{"1", "", ""},     // 이름 없음
		{"1", `..\x`, ""}, // 경로 조각 금지
		{"1", "with space", ""},
	} {
		t.Setenv("SNB_DEV", c.dev)
		t.Setenv("SNB_INSTANCE", c.name)
		if got := Instance(); got != c.want {
			t.Errorf("dev=%q name=%q: %q, want %q", c.dev, c.name, got, c.want)
		}
	}
}
