// Package applog 는 앱 로그다. 원본(snBv6FM main.cpp logToFile·imports/FmLog)과 같은 폴더·파일 이름·한 줄 형식으로 쓴다:
// <데이터 폴더>/log/yyyyMMdd.v2.log, 「날짜 시각 수준 프로세스 계정 분류 파일:줄 메시지」(docs/port/09 7.3, 원본 docs/로그_형식.md).
// 폴더가 없으면 쓰지 않는다([설정] [로그 저장] 이 폴더를 만들고 지운다). slog Handler 라 Core.Log()·slog 기본 로거·log 패키지가 모두 여기로 온다.
package applog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"snbapp/internal/config"
)

// writer 는 파일 하나와 반복 억제 상태다. 쓰기·열기·닫기·폴더 삭제를 잠금 하나로 직렬화한다(LOG-019).
// 잠금 안에서 slog 를 다시 부르지 않는다(같은 잠금으로 멈춘다) ― 잠금 안의 줄은 writeLocked 로 직접 쓴다.
type writer struct {
	mu sync.Mutex

	dir     string // 로그 폴더
	process string
	min     slog.Level
	level   string // 첫 줄의 수준 칸
	envKeep int    // fmlocal.ini KEEP_DAYS(첫 줄)
	envMax  int    // fmlocal.ini MAX_MB(첫 줄)
	build   string // 실행 파일 수정 시각

	version string
	account string

	f          *os.File
	day        string // 열린 파일의 yyyyMMdd
	started    bool   // 이 프로세스에서 한 번이라도 열었다(첫 줄 머리말이 갈린다)
	failReport bool   // 열기 실패를 stderr 에 한 번 알렸다
	closed     bool   // Close 뒤(stderr 로만)

	retain   bool // 로그인 뒤 보관 값이 정해졌다(날짜가 바뀌어 열 때도 정리)
	keepDays int
	maxBytes int64

	// 반복 억제(원본 main.cpp g_last*·g_repeat*)
	lastKey     string
	lastRest    string // 줄에서 앞 23자(시각)를 뗀 나머지
	lastMsgLen  int    // 그 줄 메시지 칸의 바이트 길이
	lastWritten time.Time
	repeat      int
	repeatUntil time.Time
	flushAfter  time.Duration // 접힌 반복을 내보낼 시계(0 이면 안 씀 ― 시험)
	timer       *time.Timer

	stderr io.Writer
}

var std = &writer{process: strconv.Itoa(os.Getpid()), level: "DEBUG", min: slog.LevelDebug, envKeep: 15, envMax: 200, flushAfter: time.Second, stderr: os.Stderr}

// New 는 앱 로거를 만들고 slog 기본 로거로도 둔다(config 등 로거를 받지 않는 패키지의 경고, log 패키지 출력도 같은 곳에 남게).
// 수준·보관 기본값은 fmlocal.ini [LOG](LEVEL·KEEP_DAYS·MAX_MB)에서 시작 때 한 번 읽는다.
func New() *slog.Logger {
	env, _ := config.LoadEnv(config.AppDir())
	lv, keep, max := "DEBUG", 15, 200
	if env != nil {
		lv, keep, max = env.LogLevel, env.LogKeepDays, env.LogMaxMB
	}
	std.mu.Lock()
	std.dir = filepath.Join(config.DataDir(), "log")
	std.min, std.level = parseLevel(lv)
	std.envKeep, std.envMax = keep, max
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			std.build = fi.ModTime().Format("2006-01-02 15:04:05")
		}
	}
	std.mu.Unlock()
	l := slog.New(&Handler{w: std})
	slog.SetDefault(l.With("category", CatApp))
	return l
}

// parseLevel 은 fmlocal.ini LEVEL 이다: INFO 는 DEBUG 끔, WARN 은 DEBUG·INFO 끔, 그 밖은 DEBUG(전부).
func parseLevel(s string) (slog.Level, string) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "INFO":
		return slog.LevelInfo, "INFO"
	case "WARN":
		return slog.LevelWarn, "WARN"
	}
	return slog.LevelDebug, "DEBUG"
}

// Dir 는 로그 폴더다.
func Dir() string {
	std.mu.Lock()
	defer std.mu.Unlock()
	return std.dir
}

// SetVersion 은 첫 줄의 버전 칸이다(Core 가 만들어질 때 한 번).
func SetVersion(v string) {
	std.mu.Lock()
	std.version = v
	std.mu.Unlock()
}

// SetAccount 는 계정 칸(ID@망이름)이다. 로그인 완료에서 채우고 로그아웃·끊김에서 "" 로 비운다(LOG-011).
func SetAccount(a string) {
	std.mu.Lock()
	std.account = a
	std.mu.Unlock()
}

// Defaults 는 fmlocal.ini 의 보관 일수·용량 상한(MB)이다(정책 remove_logfile 이 없을 때 쓴다).
func Defaults() (keepDays, maxMB int) {
	std.mu.Lock()
	defer std.mu.Unlock()
	return std.envKeep, std.envMax
}

// Retain 은 보관 정리다(LOG-026·027): 지금 정리하고, 날짜가 바뀌어 새 파일을 열 때도 같은 값으로 정리한다.
// 지운 파일 이름을 돌려준다(결과 줄 「로그 보관 >>> …」 은 부른 쪽이 남긴다 ― 원본 deleteOldLogs).
func Retain(keepDays, maxMB int) []string {
	std.mu.Lock()
	defer std.mu.Unlock()
	std.retain, std.keepDays, std.maxBytes = true, keepDays, int64(maxMB)*1024*1024
	return std.cleanupLocked(time.Now())
}

// SetLogFile 은 [로그 저장] 이다. 켜면 폴더를 만들고(다음 줄부터 쓴다), 끄면 접힌 반복을 쓰고 파일을 닫고 폴더를 통째로 지운다 ―
// 닫기와 지우기를 같은 잠금 안에서 한다(그 사이 다른 goroutine 의 줄이 파일을 다시 열지 않게, LOG-004).
func SetLogFile(use bool) error {
	std.mu.Lock()
	defer std.mu.Unlock()
	if use {
		return os.MkdirAll(std.dir, 0o755)
	}
	std.closeLocked()
	return os.RemoveAll(std.dir)
}

// Flush 는 접힌 반복을 지금 쓴다.
func Flush() {
	std.mu.Lock()
	std.flushRepeatLocked()
	std.mu.Unlock()
}

// Close 는 「앱 종료 >>> reason」 을 남기고 접힌 반복을 쓴 뒤 파일을 닫는다. 그 뒤의 줄은 stderr 로 간다(LOG-021·022).
func Close(reason string) {
	slog.Default().Info("앱 종료 >>> "+reason, "category", CatApp)
	std.mu.Lock()
	std.closeLocked()
	std.closed = true
	std.mu.Unlock()
}

func (w *writer) closeLocked() {
	w.flushRepeatLocked()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if w.f != nil {
		w.f.Close()
		w.f = nil
	}
	w.lastKey = ""
}

// write 는 한 줄을 쓴다. 같은 수준·분류·메시지가 1초 안에 다시 오면 세기만 한다(LOG-015).
func (w *writer) write(t time.Time, l slog.Level, cat, where, msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		fmt.Fprint(w.stderr, FormatLine(t, l, w.process, w.account, cat, where, msg))
		return
	}
	if !w.openLocked(t) {
		return // 폴더가 없거나 못 열면 그 줄만 버린다(LOG-029)
	}
	key := Level5(l) + "|" + cat + "|" + msg
	if key == w.lastKey && t.Sub(w.lastWritten) < time.Second {
		w.repeat++
		w.repeatUntil = t
		if w.timer == nil && w.flushAfter > 0 { // [Wails] 마지막 접힘을 다음 줄이 밀어낼 때까지 기다리지 않는다
			w.timer = time.AfterFunc(w.flushAfter, func() {
				w.mu.Lock()
				w.timer = nil
				w.flushRepeatLocked()
				w.mu.Unlock()
			})
		}
		return
	}
	w.flushRepeatLocked()
	line := FormatLine(t, l, w.process, w.account, cat, where, msg)
	w.writeLocked(line)
	w.lastKey, w.lastRest, w.lastWritten = key, line[23:], t
	w.lastMsgLen = len(CleanMessage(msg))
}

func (w *writer) writeLocked(line string) {
	if w.f != nil {
		w.f.WriteString(line) // 줄 전체를 write 한 번으로(버퍼 없음 ― 여러 프로세스 줄이 섞이지 않게)
	}
}

func (w *writer) flushRepeatLocked() {
	if w.repeat == 0 || w.f == nil {
		w.repeat = 0
		return
	}
	ts := w.repeatUntil.Format(timeLayout)
	if w.repeat == 1 {
		w.writeLocked(ts + w.lastRest)
	} else {
		head := w.lastRest[:len(w.lastRest)-w.lastMsgLen-1]
		w.writeLocked(ts + head + "(같은 줄 " + strconv.Itoa(w.repeat) + " 회 더, " + w.repeatUntil.Format("15:04:05.000") + " 까지)\n")
	}
	w.repeat = 0
}

// openLocked 는 "열려 있지 않거나 날짜가 다르면 연다" 다. 폴더가 생기면 재시작 없이 다음 줄부터 쓴다(LOG-003).
func (w *writer) openLocked(t time.Time) bool {
	day := t.Format("20060102")
	if w.f != nil && w.day == day {
		return true
	}
	if w.dir == "" {
		return false
	}
	if fi, err := os.Stat(w.dir); err != nil || !fi.IsDir() {
		return false
	}
	w.closeLocked() // 날짜가 바뀌었으면 접힌 반복을 옛 파일에 쓰고 닫는다(LOG-017)
	path := FilePath(w.dir, t)
	f, err := openAppend(path)
	if err != nil {
		if !w.failReport {
			fmt.Fprintf(w.stderr, "log: cannot open %s\n", path)
			w.failReport = true
		}
		return false
	}
	w.f, w.day = f, day
	head := "앱 시작 >>> "
	if w.started {
		head = "로그 파일을 새로 열었다 >>> "
	}
	_, file, line, _ := runtime.Caller(0)
	where := Where(file, line)
	// 시각은 파일을 열게 한 줄의 것을 쓴다(첫 줄이 바로 뒤 줄보다 늦게 찍히지 않게)
	w.writeLocked(FormatLine(t, slog.LevelInfo, w.process, w.account, CatApp, where, head+fmt.Sprintf(
		"버전 %s, 빌드 %s, 수준 %s, 저장 %s, 보관 %d일·상한 %dMB(서버 정책이 오면 그것을 따른다)",
		w.version, w.build, w.level, w.dir+string(filepath.Separator), w.envKeep, w.envMax)))
	if w.started && w.retain {
		if removed := w.cleanupLocked(t); len(removed) > 0 {
			w.writeLocked(FormatLine(t, slog.LevelInfo, w.process, w.account, CatApp, where, "오래된 로그 삭제 >>> "+strings.Join(removed, ", ")))
		}
	}
	w.started = true
	return true
}

// cleanupLocked 는 로그 폴더의 *.log 를 보관 규칙으로 지우고 지운 이름만 돌려준다(지우기 실패는 조용히 넘긴다).
func (w *writer) cleanupLocked(today time.Time) []string {
	ents, err := os.ReadDir(w.dir)
	if err != nil {
		return nil
	}
	var files []File
	for _, e := range ents {
		if !e.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			files = append(files, File{e.Name(), fi.Size()})
		}
	}
	var removed []string
	for _, n := range FilesToDelete(files, today, w.keepDays, w.maxBytes) {
		if os.Remove(filepath.Join(w.dir, n)) == nil {
			removed = append(removed, n)
		}
	}
	return removed
}

// Handler 는 slog 핸들러다. 분류는 속성 "category"(또는 "cat"), 파일:줄을 바꾸려면 속성 "where"(예: 화면 로그 "-:0").
// 그 밖의 속성은 메시지 뒤에 「 키=값」 으로 붙이고, 키가 비밀값처럼 보이면(pw·pass·token·secret·content·html) 값을 *** 로 바꾼다.
type Handler struct {
	w     *writer
	cat   string
	attrs string
	group string
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.w.min }

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	cat, where := h.cat, ""
	var b strings.Builder
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		h.attr(&b, &cat, &where, h.group, a)
		return true
	})
	if where == "" {
		where = "-:0"
		if r.PC != 0 {
			fr, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
			where = Where(fr.File, fr.Line)
		}
	}
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	h.w.write(t, r.Level, cat, where, b.String())
	return nil
}

var secretKey = regexp.MustCompile(`(?i)pw|pass|token|secret|content|html`)

func (h *Handler) attr(b *strings.Builder, cat, where *string, group string, a slog.Attr) {
	v := a.Value.Resolve()
	switch {
	case group == "" && (a.Key == "category" || a.Key == "cat"):
		*cat = v.String()
		return
	case group == "" && a.Key == "where":
		*where = v.String()
		return
	case v.Kind() == slog.KindGroup:
		g := a.Key
		if group != "" && g != "" {
			g = group + "." + g
		} else if g == "" {
			g = group
		}
		for _, x := range v.Group() {
			h.attr(b, cat, where, g, x)
		}
		return
	case a.Key == "" && v.Any() == nil:
		return
	}
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	val := v.String()
	if secretKey.MatchString(a.Key) {
		val = "***"
	}
	b.WriteString(" " + key + "=" + val)
}

func (h *Handler) WithAttrs(as []slog.Attr) slog.Handler {
	n := *h
	var b strings.Builder
	b.WriteString(h.attrs)
	where := ""
	for _, a := range as {
		n.attr(&b, &n.cat, &where, n.group, a)
	}
	n.attrs = b.String()
	return &n
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	n := *h
	if n.group != "" {
		name = n.group + "." + name
	}
	n.group = name
	return &n
}
