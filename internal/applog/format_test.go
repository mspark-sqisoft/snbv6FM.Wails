package applog

// 원본 tests/tst_LogFormat.cpp·tst_LogProxyModel.cpp 의 표를 옮겼다.

import (
	"log/slog"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var kNow = time.Date(2026, 9, 16, 10, 45, 52, 606_000_000, time.Local)

func TestBaseName(t *testing.T) {
	for in, want := range map[string]string{
		`E:\git\snBv6FM\src\ViewController.cpp`: "ViewController.cpp",
		"/home/u/snBv6FM/src/main.cpp":          "main.cpp",
		"qrc:/qt/qml/Main/content/HomePage.qml": "HomePage.qml",
		`E:/git\x/y.cpp`:                        "y.cpp",
		"main.cpp":                              "main.cpp",
		"":                                      "",
	} {
		if got := BaseName(in); got != want {
			t.Errorf("BaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLevel5(t *testing.T) {
	want := map[slog.Level]string{slog.LevelDebug: "DEBUG", slog.LevelInfo: "INFO ", slog.LevelWarn: "WARN ", slog.LevelError: "ERROR", LevelFatal: "FATAL"}
	for l, w := range want {
		if got := Level5(l); got != w || len(got) != 5 {
			t.Errorf("Level5(%v) = %q", l, got)
		}
	}
}

func TestFormatLine(t *testing.T) {
	cases := []struct {
		name                       string
		lv                         slog.Level
		file                       string
		line                       int
		cat, proc, acct, msg, want string
	}{
		{"info-account", slog.LevelInfo, `E:\git\snBv6FM\imports\FileMoverGrpc\FileTransfer.cpp`, 465, "snb.transfer.upload", "9120", "test1@업무망", "BATCH 전송 시작 >>> 3건",
			"2026-09-16 10:45:52.606 INFO  9120 test1@업무망 snb.transfer.upload FileTransfer.cpp:465 BATCH 전송 시작 >>> 3건\n"},
		{"debug-no-account", slog.LevelDebug, "/src/main.cpp", 247, "snb.app", "18244", "", "Full Arguments",
			"2026-09-16 10:45:52.606 DEBUG 18244 - snb.app main.cpp:247 Full Arguments\n"},
		{"instance-process", slog.LevelInfo, "x.cpp", 449, "snb.auth", "e2e2:7732", "test2_1@업무망", "로그인 완료",
			"2026-09-16 10:45:52.606 INFO  e2e2:7732 test2_1@업무망 snb.auth x.cpp:449 로그인 완료\n"},
		{"qt-internal", slog.LevelWarn, "", 0, "default", "9120", "test1@업무망", "Retrying to obtain clipboard.",
			"2026-09-16 10:45:52.606 WARN  9120 test1@업무망 default -:0 Retrying to obtain clipboard.\n"},
		{"quotes-stripped-newline-escaped", slog.LevelError, "a.cpp", 1, "snb.grpc", "1", "", "status \"UNAVAILABLE\"\r\nretry",
			"2026-09-16 10:45:52.606 ERROR 1 - snb.grpc a.cpp:1 status UNAVAILABLE\\nretry\n"},
		{"whole-quote-pair-stripped", slog.LevelDebug, "a.cpp", 4, "snb.grpc", "1", "", "\"gRPC 상태 [채널 준비]\"",
			"2026-09-16 10:45:52.606 DEBUG 1 - snb.grpc a.cpp:4 gRPC 상태 [채널 준비]\n"},
		{"inner-quotes-stripped-too", slog.LevelDebug, "a.cpp", 5, "snb.ui", "1", "", "\"C:/a\" 크기 \"262\"",
			"2026-09-16 10:45:52.606 DEBUG 1 - snb.ui a.cpp:5 C:/a 크기 262\n"},
		{"space-in-account", slog.LevelInfo, "a.cpp", 2, "snb.auth", "1", "test 1@업무 망", "x",
			"2026-09-16 10:45:52.606 INFO  1 test_1@업무_망 snb.auth a.cpp:2 x\n"},
		{"empty-process-and-message", slog.LevelDebug, "a.cpp", 3, "snb.ui", "", "", "",
			"2026-09-16 10:45:52.606 DEBUG - - snb.ui a.cpp:3 \n"},
		{"empty-category-is-default", slog.LevelInfo, "a.cpp", 3, "", "1", "", "x",
			"2026-09-16 10:45:52.606 INFO  1 - default a.cpp:3 x\n"},
	}
	for _, c := range cases {
		got := FormatLine(kNow, c.lv, c.proc, c.acct, c.cat, Where(c.file, c.line), c.msg)
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMessageLongIsTruncated(t *testing.T) {
	out := CleanMessage(strings.Repeat("a", MaxMessage+1))
	if !strings.HasPrefix(out, strings.Repeat("a", MaxMessage)) || !strings.HasSuffix(out, "…(잘림, 원래 4001자)") {
		t.Fatalf("잘림: %q", out[len(out)-40:])
	}
	if got := CleanMessage(strings.Repeat("가", MaxMessage)); len([]rune(got)) != MaxMessage {
		t.Fatalf("4000자는 그대로여야 한다: %d", len([]rune(got)))
	}
	if got := CleanMessage(strings.Repeat("가", MaxMessage+5)); !strings.HasSuffix(got, "원래 4005자)") {
		t.Fatalf("한글은 글자 수로 센다: %q", got[len(got)-30:])
	}
}

// 읽는 쪽 규칙: 공백 한 개 이상으로 일곱 번 나누고 나머지가 메시지다(원본 roundtrip).
func TestRoundtrip(t *testing.T) {
	head := regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+) (.*)$`)
	for _, c := range []struct {
		lv        slog.Level
		acct, msg string
	}{
		{slog.LevelInfo, "test1@업무망", "로그인 완료 >>> msg[LOGIN SUCCESS]"},
		{slog.LevelDebug, "", "a - b  c"},
		{slog.LevelWarn, "", "update file missing"},
	} {
		out := strings.TrimSuffix(FormatLine(kNow, c.lv, "updater:5120", c.acct, "snb.updater", Where(`C:\x\Upd.cpp`, 12), c.msg), "\n")
		m := head.FindStringSubmatch(out)
		acct := c.acct
		if acct == "" {
			acct = "-"
		}
		want := []string{"2026-09-16", "10:45:52.606", strings.TrimSpace(Level5(c.lv)), "updater:5120", acct, "snb.updater", "Upd.cpp:12", c.msg}
		if m == nil || !reflect.DeepEqual(m[1:], want) {
			t.Errorf("정규식 되읽기 %q → %q", out, m)
		}
		l, ok := ParseLine(out)
		got := []string{l.Date, l.Time, l.Level, l.Process, l.Account, l.Category, l.Where, l.Message}
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("ParseLine %q → %q", out, got)
		}
	}
}

// 원본 docs/로그_형식.md 의 예시 5줄(tst_LogProxyModel 픽스처와 같다).
var docLines = []string{
	`2026-09-16 10:45:52.606 INFO  9120 - snb.app main.cpp:155 앱 시작 >>> 버전 6.0.0.1, 빌드 2026-09-16 10:40:12, 수준 DEBUG, 저장 C:\Users\홍길동\AppData\Local\SQIsoft\snB App\log\, 보관 15일·상한 200MB`,
	`2026-09-16 10:45:53.010 DEBUG 9120 - snb.auth ViewController.cpp:2278 LOGIN TRY >>> ID[test1] PW[*** len=8]`,
	`2026-09-16 10:45:53.412 INFO  9120 test1@업무망 snb.transfer.upload FileTransfer.cpp:465 BATCH 전송 시작 >>> 3건, 보고서 초안.pdf 외`,
	`2026-09-16 10:45:54.000 WARN  e2e2:7732 test2_1@업무망 snb.grpc FileMoverGrpc.cpp:1410 서버 응답 지연 >>> 1500 ms\n둘째 줄은 \n 두 글자로 이어진다`,
	`2026-09-16 10:45:55.250 ERROR 9120 test1@업무망 default filemoverpolicy.cpp:161 (같은 줄 3 회 더, 10:45:55.900 까지)`,
}

func TestParseDocExamples(t *testing.T) {
	re := regexp.MustCompile(`^(\d{4}-\d\d-\d\d) (\d\d:\d\d:\d\d\.\d{3}) (\S+)\s+(\S+) (\S+) (\S+) (\S+) ?(.*)$`) // 원본 되읽기 정규식
	for _, s := range docLines {
		m := re.FindStringSubmatch(s)
		l, ok := ParseLine(s)
		if m == nil || !ok {
			t.Fatalf("못 읽음: %s", s)
		}
		got := []string{l.Date, l.Time, l.Level, l.Process, l.Account, l.Category, l.Where, l.Message}
		if !reflect.DeepEqual(got, m[1:]) {
			t.Errorf("칸이 다르다\n got %q\nwant %q", got, m[1:])
		}
	}
	l, _ := ParseLine(docLines[0])
	if !strings.HasPrefix(l.Message, "앱 시작 >>> 버전 6.0.0.1, ") || !strings.HasSuffix(l.Message, "보관 15일·상한 200MB") {
		t.Errorf("메시지 안 공백·경로는 그대로: %q", l.Message)
	}
	for _, bad := range []string{"", "옛 형식^^줄", "2026-09-16 10:45:52.606", "2026-09-16 10:45:52.606 INFO  9120 - snb.app", "2026-09-16 10:45:52 INFO  1 - a b c"} {
		if _, ok := ParseLine(bad); ok {
			t.Errorf("읽으면 안 되는 줄: %q", bad)
		}
	}
	if l, ok := ParseLine("2026-09-16 10:45:52.606 DEBUG - - snb.ui a.cpp:3 "); !ok || l.Where != "a.cpp:3" || l.Message != "" {
		t.Errorf("빈 메시지 줄: %+v %v", l, ok)
	}
}

func TestFilePathIsV2(t *testing.T) {
	if got, want := FilePath("C:/d/log", kNow), filepath.Join("C:/d/log", "20260916.v2.log"); got != want {
		t.Fatalf("%q != %q", got, want)
	}
}

func TestFilesToDelete(t *testing.T) {
	four := []string{"20260830.log", "20260901.log", "20260901.v2.log", "20260916.log"}
	mk := func(names []string, sizes ...int64) []File {
		var fs []File
		for i, n := range names {
			fs = append(fs, File{n, sizes[i]})
		}
		slices.Reverse(fs) // 입력 순서에 좌우되지 않아야 한다
		return fs
	}
	cases := []struct {
		name  string
		files []File
		keep  int
		max   int64
		want  []string
	}{
		{"keep15", mk(four, 1, 1, 1, 1), 15, 0, []string{"20260830.log"}},
		{"keep14", mk(four, 1, 1, 1, 1), 14, 0, []string{"20260830.log", "20260901.log", "20260901.v2.log"}},
		{"keep0-none", mk(four, 1, 1, 1, 1), 0, 0, []string{}},
		{"size-oldest-first", mk(four, 100, 100, 100, 100), 0, 250, []string{"20260830.log", "20260901.log"}},
		{"today-never", mk([]string{"20260915.log", "20260916.log", "20260916.v2.log"}, 10, 500, 500), 0, 100, []string{"20260915.log"}},
		{"unparsable-kept", mk([]string{"updater_20260801.log", "tempLog.log", "20260801.log"}, 1, 1, 1), 3, 0, []string{"20260801.log"}},
		{"days-then-size", mk(four, 100, 100, 100, 100), 15, 250, []string{"20260830.log", "20260901.log"}},
	}
	for _, c := range cases {
		if got := FilesToDelete(c.files, kNow, c.keep, c.max); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPw(t *testing.T) {
	if got := Pw("abcd1234"); got != "*** len=8" {
		t.Fatal(got)
	}
}
