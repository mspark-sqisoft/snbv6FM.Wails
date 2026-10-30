package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeDay(t *testing.T, dir string, lines ...string) {
	t.Helper()
	path := FilePath(dir, time.Date(2026, 7, 31, 0, 0, 0, 0, time.Local))
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ln(tm, lv, cat, msg string) string {
	return fmt.Sprintf("2026-07-31 %s %-5s 9120 test1@업무망 %s a.cpp:1 %s", tm, lv, cat, msg)
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	writeDay(t, dir,
		ln("10:00:00.000", "INFO", "snb.transfer.upload", "업로드 시작"),
		"옛 형식^^줄",
		ln("10:00:01.000", "DEBUG", "snb.grpc", "Notify 수신"),
		ln("10:00:02.000", "WARN", "snb.transfer", "느림 UPLOAD"),
		ln("10:00:03.000", "ERROR", "snb.auth", "실패"),
		ln("10:00:04.000", "INFO", "snb.auth", "upload 라는 단어"),
	)
	cases := []struct {
		date string
		q    Query
		want []string // 메시지
	}{
		{"2026-07-31", Query{}, []string{"업로드 시작", "Notify 수신", "느림 UPLOAD", "실패", "upload 라는 단어"}},
		{"20260731", Query{Level: "INFO"}, []string{"업로드 시작", "upload 라는 단어"}},
		{"2026-07-31", Query{Level: "WARN+"}, []string{"느림 UPLOAD", "실패"}},
		{"2026-07-31", Query{Category: "snb.transfer"}, []string{"업로드 시작", "느림 UPLOAD"}},
		{"2026-07-31", Query{Category: "snb.transfer.upload"}, []string{"업로드 시작"}},
		{"2026-07-31", Query{Word: "upload"}, []string{"느림 UPLOAD", "upload 라는 단어"}}, // 메시지만, 대소문자 무시
		{"2026-07-31", Query{Word: "test1"}, nil},                                    // 계정 칸은 안 본다
		{"2026-07-31", Query{Max: 2}, []string{"실패", "upload 라는 단어"}},
	}
	for _, c := range cases {
		r, err := searchIn(dir, c.date, c.q)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range r.Lines {
			got = append(got, l.Message)
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") || r.Total != 6 {
			t.Errorf("%+v: %q (total %d)", c.q, got, r.Total)
		}
	}
	r, _ := searchIn(dir, "2026-07-31", Query{Max: 2})
	if r.Matched != 5 || r.Nos[0] != 4 || r.Nos[1] != 5 {
		t.Errorf("잘린 결과의 순번: %+v %v", r.Matched, r.Nos)
	}
	r, err := searchIn(dir, "2026-07-30", Query{})
	if err != nil || !r.Missing || len(r.Lines) != 0 || filepath.Base(r.Path) != "20260730.v2.log" {
		t.Errorf("없는 날: %+v %v", r, err)
	}
	if _, err := searchIn(dir, "어제", Query{}); err == nil {
		t.Error("날짜 형식 오류여야 한다")
	}
}

// 16만 줄을 1초 안에(원본 FR-021). 실제로는 훨씬 빠르다.
func TestSearchBigFileFast(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := range 160_000 {
		fmt.Fprintf(&b, "2026-07-31 10:00:00.000 DEBUG 9120 test1@업무망 snb.grpc FileMoverGrpc.cpp:1410 서버 응답 %d 번째 줄입니다 >>> 조금 긴 메시지\n", i)
	}
	os.WriteFile(FilePath(dir, time.Date(2026, 7, 31, 0, 0, 0, 0, time.Local)), []byte(b.String()), 0o644)
	start := time.Now()
	r, err := searchIn(dir, "2026-07-31", Query{Level: "DEBUG", Word: "99999", Max: 50_000})
	if err != nil || r.Total != 160_000 || r.Matched != 1 {
		t.Fatalf("%v %d %d", err, r.Total, r.Matched)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("느리다: %v", d)
	}
}
