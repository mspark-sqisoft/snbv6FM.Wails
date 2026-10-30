package applog

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestWriter(t *testing.T) (*writer, *slog.Logger) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "log")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	w := &writer{dir: dir, process: "4242", level: "DEBUG", min: slog.LevelDebug, envKeep: 15, envMax: 200, version: "0.1.0", stderr: &bytes.Buffer{}}
	return w, slog.New(&Handler{w: w})
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// 원본 E2E tools/e2e/cases/88-log-format.ps1 의 줄 규칙 LF-01~06 을 이 핸들러가 쓴 파일에 건다.
func TestLogFormatRules(t *testing.T) {
	w, log := newTestWriter(t)
	w.account = "test7_1@업무망"
	log.Debug("LOGIN TRY >>> ID[test7_1] PW["+Pw("s3cr3t-Pass!")+"]", "category", CatAuth)
	log.Info("로그인", "category", CatAuth, "password", "s3cr3t-Pass!", "token", "abc.def", "user", "test7_1")
	log.Info("설정 변경 >>> useLogFile: false → true", "category", CatSettings)
	log.Warn("여러 줄\r\n\"따옴표\"")
	slog.New(w.handler().WithAttrs([]slog.Attr{slog.String("cat", CatGrpc)})).Error("grpc 오류", "err", "x")

	lines := readLines(t, FilePath(w.dir, time.Now()))
	stamp := regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3} `)
	src := regexp.MustCompile(`[A-Za-z]:[\\/]\S*[\\/](src|imports|content|controls|settings|internal)[\\/]\S*\.(cpp|h|qml|go)\b`)
	var first, settings bool
	for _, l := range lines {
		if !stamp.MatchString(l) || len(regexp.MustCompile(`\s+`).Split(l, 8)) < 8 { // LF-01
			t.Errorf("LF-01 못 읽는 줄: %q", l)
		}
		p, ok := ParseLine(l)
		if !ok {
			t.Errorf("ParseLine 실패: %q", l)
		}
		if strings.ContainsAny(p.Where, `\/`) { // LF-02
			t.Errorf("LF-02 파일 칸에 경로: %q", l)
		}
		if strings.Contains(l, "s3cr3t-Pass!") || strings.Contains(l, "abc.def") { // LF-03 + 토큰
			t.Errorf("LF-03 비밀값: %q", l)
		}
		if src.MatchString(l) { // LF-04
			t.Errorf("LF-04 소스 경로: %q", l)
		}
		first = first || strings.Contains(l, "앱 시작 >>> ")
		settings = settings || strings.Contains(l, "설정 변경 >>> ")
	}
	if !first || !settings {
		t.Errorf("LF-05 첫 줄 %v, LF-06 설정 변경 %v\n%s", first, settings, strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], " INFO  4242 test7_1@업무망 snb.app applog.go:") || !strings.Contains(lines[0], "앱 시작 >>> 버전 0.1.0, ") {
		t.Errorf("첫 줄: %q", lines[0])
	}
	if p, _ := ParseLine(lines[2]); !strings.HasSuffix(p.Message, "password=*** token=*** user=test7_1") || p.Where[:15] != "applog_test.go:" {
		t.Errorf("속성: %+v", p)
	}
	if p, _ := ParseLine(lines[4]); p.Category != CatDefault || p.Message != `여러 줄\n따옴표` {
		t.Errorf("기본 분류·메시지 정리: %+v", p)
	}
	if p, _ := ParseLine(lines[5]); p.Category != CatGrpc || p.Level != "ERROR" || p.Message != "grpc 오류 err=x" {
		t.Errorf("WithAttrs 분류: %+v", p)
	}
}

func (w *writer) handler() *Handler { return &Handler{w: w} }

func TestNoFolderNoWrite(t *testing.T) {
	w, log := newTestWriter(t)
	os.Remove(w.dir)
	log.Info("버려진다")
	if _, err := os.Stat(w.dir); err == nil {
		t.Fatal("폴더를 만들면 안 된다")
	}
	os.Mkdir(w.dir, 0o755) // 폴더가 생기면 재시작 없이 다음 줄부터
	log.Info("남는다")
	lines := readLines(t, FilePath(w.dir, time.Now()))
	if len(lines) != 2 || !strings.HasSuffix(lines[1], " 남는다") {
		t.Fatalf("%q", lines)
	}
}

func TestRepeatCollapse(t *testing.T) {
	w, _ := newTestWriter(t)
	t0 := time.Date(2026, 9, 16, 10, 0, 0, 0, time.Local)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	w.write(at(0), slog.LevelError, CatDefault, "p.cpp:161", "같은 말")
	w.write(at(100), slog.LevelError, CatDefault, "p.cpp:161", "같은 말")
	w.write(at(200), slog.LevelError, CatDefault, "p.cpp:161", "같은 말")
	w.write(at(900), slog.LevelError, CatDefault, "p.cpp:161", "같은 말")
	w.write(at(950), slog.LevelInfo, CatDefault, "p.cpp:9", "다른 말")
	w.write(at(960), slog.LevelInfo, CatDefault, "p.cpp:9", "다른 말")  // 한 번 → 원래 줄 한 번 더
	w.write(at(3000), slog.LevelInfo, CatDefault, "p.cpp:9", "다른 말") // 1초 지나 다시 쓴다(앞 접힘 먼저)
	lines := readLines(t, FilePath(w.dir, t0))[1:]
	want := []string{
		"2026-09-16 10:00:00.000 ERROR 4242 - default p.cpp:161 같은 말",
		"2026-09-16 10:00:00.900 ERROR 4242 - default p.cpp:161 (같은 줄 3 회 더, 10:00:00.900 까지)",
		"2026-09-16 10:00:00.950 INFO  4242 - default p.cpp:9 다른 말",
		"2026-09-16 10:00:00.960 INFO  4242 - default p.cpp:9 다른 말",
		"2026-09-16 10:00:03.000 INFO  4242 - default p.cpp:9 다른 말",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestRepeatFlushedByTimer(t *testing.T) {
	w, log := newTestWriter(t)
	w.flushAfter = 50 * time.Millisecond
	for range 5 {
		log.Warn("폭주")
	}
	time.Sleep(300 * time.Millisecond)
	lines := readLines(t, FilePath(w.dir, time.Now()))
	if len(lines) != 3 || !strings.Contains(lines[2], "(같은 줄 4 회 더, ") {
		t.Fatalf("%q", lines)
	}
}

func TestDateRolloverAndRetain(t *testing.T) {
	w, _ := newTestWriter(t)
	d1 := time.Date(2026, 9, 15, 23, 59, 59, 0, time.Local)
	os.WriteFile(filepath.Join(w.dir, "20260801.v2.log"), []byte("x\n"), 0o644)
	w.write(d1, slog.LevelInfo, CatApp, "a.go:1", "어제")
	w.write(d1, slog.LevelInfo, CatApp, "a.go:1", "어제") // 접힌 채 날짜가 바뀐다
	w.mu.Lock()
	w.retain, w.keepDays = true, 15
	w.mu.Unlock()
	w.write(d1.Add(2*time.Second), slog.LevelInfo, CatApp, "a.go:2", "오늘")
	old := readLines(t, FilePath(w.dir, d1))
	if len(old) != 3 || !strings.HasSuffix(old[2], " 어제") {
		t.Fatalf("옛 파일에 접힌 반복을 쓰고 닫는다: %q", old)
	}
	cur := readLines(t, FilePath(w.dir, d1.Add(2*time.Second)))
	if len(cur) != 3 || !strings.Contains(cur[0], "로그 파일을 새로 열었다 >>> ") || !strings.HasSuffix(cur[1], "오래된 로그 삭제 >>> 20260801.v2.log") {
		t.Fatalf("새 파일: %q", cur)
	}
	if _, err := os.Stat(filepath.Join(w.dir, "20260801.v2.log")); err == nil {
		t.Fatal("오래된 파일이 남았다")
	}
}

// 여러 goroutine 이 동시에 써도 줄이 찢어지지 않는다(여러 프로세스는 같은 핸들 권한 FILE_APPEND_DATA 로 보장한다).
func TestConcurrentWritesAndSecondHandle(t *testing.T) {
	w, log := newTestWriter(t)
	w2 := &writer{dir: w.dir, process: "5555", level: "DEBUG", stderr: &bytes.Buffer{}, started: true}
	log2 := slog.New(&Handler{w: w2}) // 다른 프로세스 흉내: 같은 파일을 따로 연다
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				l := log
				if g%2 == 1 {
					l = log2
				}
				l.Info(strings.Repeat("가", 50), "g", g, "i", i)
			}
		}()
	}
	wg.Wait()
	w2.mu.Lock()
	w2.closeLocked()
	w2.mu.Unlock()
	lines := readLines(t, FilePath(w.dir, time.Now()))
	if len(lines) != 1+1+1600 {
		t.Errorf("줄 수 %d", len(lines))
	}
	for _, l := range lines {
		if _, ok := ParseLine(l); !ok {
			t.Fatalf("찢어진 줄: %q", l)
		}
	}
}

func TestSetLogFileClosesAndRemoves(t *testing.T) {
	w, log := newTestWriter(t)
	old := std
	std = w
	defer func() { std = old }()
	log.Info("하나")
	if err := SetLogFile(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.dir); err == nil {
		t.Fatal("폴더가 남았다")
	}
	log.Info("버려진다")
	if err := SetLogFile(true); err != nil {
		t.Fatal(err)
	}
	log.Info("다시")
	lines := readLines(t, FilePath(w.dir, time.Now()))
	if len(lines) != 2 || !strings.Contains(lines[0], "로그 파일을 새로 열었다 >>> ") {
		t.Fatalf("%q", lines)
	}
}

func TestLevelFilter(t *testing.T) {
	for in, want := range map[string]slog.Level{"info": slog.LevelInfo, " WARN ": slog.LevelWarn, "5": slog.LevelDebug, "": slog.LevelDebug} {
		if got, _ := parseLevel(in); got != want {
			t.Errorf("%q → %v", in, got)
		}
	}
	w, log := newTestWriter(t)
	w.min = slog.LevelInfo
	log.Debug("안 남는다")
	log.Info("남는다")
	if lines := readLines(t, FilePath(w.dir, time.Now())); len(lines) != 2 {
		t.Fatalf("%q", lines)
	}
}
