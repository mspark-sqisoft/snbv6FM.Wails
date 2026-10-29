package main

// 실 서버 시험. SNB_LIVE_ID 가 있을 때만 돈다(없으면 SKIP).
//   SNB_APP_DIR      fmenv.ini·root_ca.pem 이 있는 폴더(예: E:\git\snBv6FM\build\release)
//   SNB_LIVE_ID/PW   시험 계정
//   SNB_LIVE_LOC     붙을 망의 LOC(1 업무망, 0 인터넷망). 기본 1
//   SNB_LIVE_DOWNLOAD  받을 서버 파일 이름(server_str). 있으면 업로드 대신 받기를 시험한다(승인된 반대 망 파일이어야 한다)
//
//   go test -run Live -v .

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLive(t *testing.T) {
	id := os.Getenv("SNB_LIVE_ID")
	if id == "" {
		t.Skip("SNB_LIVE_ID 가 없다")
	}
	loc := os.Getenv("SNB_LIVE_LOC")
	if loc == "" {
		loc = "1"
	}
	dir := appDir()
	servers, err := readServers(filepath.Join(dir, "fmenv.ini"))
	if err != nil {
		t.Fatal(err)
	}
	var srv *Server
	for i := range servers {
		if servers[i].Loc == loc {
			srv = &servers[i]
		}
	}
	if srv == nil {
		t.Fatalf("LOC=%s 서버가 없다", loc)
	}
	s, err := Login(dir, *srv, id, os.Getenv("SNB_LIVE_PW"), version)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t.Logf("로그인 %s(%s) upfile_mode=%s aprv=%d", s.UserID, s.Info.UserName, s.Profile["fm_upfile_mode"], s.aprvType())

	noop := func(int64, int64) {}
	if name := os.Getenv("SNB_LIVE_DOWNLOAD"); name != "" {
		p, err := s.Download(name, t.TempDir(), noop)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := os.Stat(p)
		t.Logf("받음 %s %d 바이트", p, st.Size())
		return
	}

	src := filepath.Join(t.TempDir(), "WAILS_"+time.Now().Format("20060102_150405")+".txt")
	if err := os.WriteFile(src, make([]byte, 1500*1024), 0o644); err != nil { // 청크 셋에 걸치게
		t.Fatal(err)
	}
	var last int64
	if err := s.Upload(src, func(d, _ int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	files, err := s.Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.NameStr == filepath.Base(src) {
			t.Logf("올림 %s → 서버 %s 크기 %d state %d (진행률 마지막 %d)", f.NameStr, f.ServerStr, f.FileSize, f.State, last)
			if f.FileSize != 1500*1024 {
				t.Errorf("서버 크기 %d", f.FileSize)
			}
			return
		}
	}
	t.Fatalf("올린 파일이 목록(%d건)에 없다", len(files))
}
