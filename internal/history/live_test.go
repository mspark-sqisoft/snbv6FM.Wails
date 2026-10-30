package history

// 실 서버 시험(서버파일 목록·상태 판정·검출 조회·내역). SNB_LIVE_ID 가 있을 때만 돈다(없으면 SKIP).
//   SNB_APP_DIR      fmenv.ini·root_ca.pem 이 있는 폴더(예: E:\git\snBv6FM\build\release)
//   SNB_LIVE_ID/PW   시험 계정(비밀번호는 환경변수로만)
//   SNB_LIVE_LOC     붙을 망의 LOC(1 업무망, 0 인터넷망). 기본 1
//   SNB_LIVE_DELETE  1 이면 작은 시험 파일(WAILS_files_*.txt)을 올렸다가 [삭제](RemoveFile flag 0)로 지운다
//
//   go test -run Live -v ./internal/history

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer/transfertest"
)

func TestLive(t *testing.T) {
	id := os.Getenv("SNB_LIVE_ID")
	if id == "" {
		t.Skip("SNB_LIVE_ID 가 없다")
	}
	loc := 1
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
	s, err := nbs.Login(dir, *srv, id, os.Getenv("SNB_LIVE_PW"), "0.1.0-test", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t.Logf("로그인 %s 망 %s(LOC %d) down_same_network=%q max_downlod_count=%q", s.UserID, srv.Name, loc,
		s.Profile["down_same_network"], s.Profile["max_downlod_count"])

	start := time.Now()
	list, err := files.List(s)
	if err != nil {
		t.Fatal(err)
	}
	byStatus, byBlock := map[string]int{}, map[files.Block]int{}
	for _, f := range list {
		byStatus[files.FileStatus(f, loc, s.Profile).Text]++
		byBlock[files.Classify(f, loc, s.Profile)]++
	}
	t.Logf("서버파일 %d 건 %dms, 상태별 %v, 받기 사유별 %v", len(list), time.Since(start).Milliseconds(), byStatus, byBlock)

	for _, f := range list { // 바이러스·APT 검출 조회(읽기만)
		for _, k := range []int32{files.DetectVirus, files.DetectAPT} {
			d, err := files.DetectionDetail(s, f.ServerStr, k)
			if err != nil {
				t.Errorf("검출 조회 %d: %v", k, err)
			}
			t.Logf("검출 조회 type %d (%s): %q", k, files.DisplayName(f), d)
		}
		t.Logf("개인정보 상세: %q", files.PrivDetailText(f.PrivDetail, f.ServerStr))
		break
	}

	today := time.Now().Format(time.DateOnly)
	weekAgo := time.Now().AddDate(0, 0, -30).Format(time.DateOnly)
	for _, typ := range []int32{TypeTransfer, TypeApproval} {
		start := time.Now()
		h, err := Fetch(s, typ, weekAgo, today)
		if err != nil {
			t.Fatalf("내역 type %d: %v", typ, err)
		}
		states := map[string]int{}
		for _, x := range h {
			states[AprvStateText(x.AprvState)+"/"+AprvTypeText(x.AprvType)]++
		}
		t.Logf("내역 type %d %s~%s: %d 건 %dms %v", typ, weekAgo, today, len(h), time.Since(start).Milliseconds(), states)
		if len(h) > 0 {
			p := filepath.Join(t.TempDir(), FileName("전송내역", time.Now()))
			rows := make([][]string, 0, len(h))
			for _, x := range h {
				rows = append(rows, []string{x.AprvDept, BaseName(x.FileNamePc), files.SizeText(x.FileSize), x.FileUserName, x.FileDate,
					x.FileCause, files.CourseName(env.Servers, x.FileCourse), AprvStateText(x.AprvState), AprvTypeText(x.AprvType),
					x.AprvUser, x.AprvDate, x.AprvCause, x.DownDate})
			}
			if err := WriteXLSX(p, Sheet{Title: Title("전송내역", weekAgo, today, ""), Headers: make([]string, 13), Rows: rows}); err != nil {
				t.Errorf("엑셀: %v", err)
			}
		}
	}

	if os.Getenv("SNB_LIVE_DELETE") != "1" {
		return
	}
	name := "WAILS_files_" + time.Now().Format("150405") + ".txt"
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("wails files delete test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transfertest.Upload(s, p); err != nil {
		t.Skipf("시험 파일을 올리지 못해 삭제 시험을 건너뛴다: %v", err)
	}
	find := func() string {
		l, err := files.List(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range l {
			if files.DisplayName(f) == name {
				t.Logf("올린 행 상태: %s", files.FileStatus(f, loc, s.Profile).Text)
				return f.ServerStr
			}
		}
		return ""
	}
	srvName := find()
	if srvName == "" {
		t.Fatal("올린 파일이 목록에 없다")
	}
	if err := files.Remove(s, srvName, false); err != nil {
		t.Fatal(err)
	}
	if find() != "" {
		t.Error("지운 파일이 목록에 남았다")
	}
	t.Logf("올리고 지움: %s", name)
}
