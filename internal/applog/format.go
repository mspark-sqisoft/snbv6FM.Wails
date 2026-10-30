package applog

// 한 줄 형식·되읽기·보관 정리 ― 순수 함수(원본 imports/FmLog/FmLog.h, docs/로그_형식.md).
// 형식은 쓰는 쪽(이 앱·원본 앱)과 읽는 쪽([로그] 화면·E2E 하네스·snb-gui)이 함께 따르는 계약이다.

import (
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// LevelFatal 은 FATAL 수준이다(slog 에는 없다).
const LevelFatal = slog.Level(12)

// MaxMessage 는 메시지 칸의 상한(UTF-16 코드 단위, 원본 QString::size)이다.
const MaxMessage = 4000

// 분류 11개(원본 FmLog 의 lcApp…lcUi) + 분류 없음·화면.
const (
	CatApp       = "snb.app"
	CatAuth      = "snb.auth"
	CatUpload    = "snb.transfer.upload"
	CatDownload  = "snb.transfer.download"
	CatClipboard = "snb.transfer.clipboard"
	CatAuto      = "snb.transfer.auto"
	CatApproval  = "snb.approval"
	CatCheck     = "snb.check"
	CatSettings  = "snb.settings"
	CatGrpc      = "snb.grpc"
	CatUI        = "snb.ui"
	CatDefault   = "default"
	CatQML       = "qml"
)

const timeLayout = "2006-01-02 15:04:05.000"

// Level5 는 수준 5자 고정폭이다: DEBUG·"INFO "·"WARN "·ERROR·FATAL.
func Level5(l slog.Level) string {
	switch {
	case l >= LevelFatal:
		return "FATAL"
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	}
	return "DEBUG"
}

// Token 은 머리 칸 하나다: 앞뒤 공백을 떼고 비면 "-", 안의 공백 문자는 '_'.
func Token(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '_'
		}
		return r
	}, s)
}

// CleanMessage 는 메시지 칸이다: 큰따옴표 전부 삭제, \r 삭제, 줄바꿈은 `\n` 두 글자, 4000자 넘으면 자르고 「…(잘림, 원래 N자)」.
func CleanMessage(m string) string {
	m = strings.NewReplacer(`"`, "", "\r", "", "\n", `\n`).Replace(m)
	n := 0 // UTF-16 길이
	cut := -1
	for i, r := range m {
		w := utf16.RuneLen(r)
		if w < 0 {
			w = 1
		}
		if n+w > MaxMessage && cut < 0 {
			cut = i
		}
		n += w
	}
	if cut < 0 {
		return m
	}
	return m[:cut] + "…(잘림, 원래 " + strconv.Itoa(n) + "자)"
}

// BaseName 은 경로의 마지막 '/' 또는 '\' 뒤다(파일 칸에 개발 PC 경로가 남지 않게).
func BaseName(f string) string {
	if i := strings.LastIndexAny(f, `/\`); i >= 0 {
		return f[i+1:]
	}
	return f
}

// Where 는 파일:줄 칸이다. 파일이 없으면 "-:0".
func Where(file string, line int) string {
	if file == "" {
		return "-:0"
	}
	return Token(BaseName(file) + ":" + strconv.Itoa(line))
}

// FormatLine 은 한 줄(끝 \n 포함)이다: 날짜 시각 수준 프로세스 계정 분류 파일:줄 메시지.
func FormatLine(t time.Time, l slog.Level, process, account, category, where, msg string) string {
	if strings.TrimSpace(category) == "" {
		category = CatDefault
	}
	if where == "" {
		where = "-:0"
	}
	var b strings.Builder
	b.Grow(64 + len(msg))
	b.WriteString(t.Format(timeLayout))
	b.WriteByte(' ')
	b.WriteString(Level5(l))
	b.WriteByte(' ')
	b.WriteString(Token(process))
	b.WriteByte(' ')
	b.WriteString(Token(account))
	b.WriteByte(' ')
	b.WriteString(Token(category))
	b.WriteByte(' ')
	b.WriteString(Token(where))
	b.WriteByte(' ')
	b.WriteString(CleanMessage(msg))
	b.WriteByte('\n')
	return b.String()
}

// Line 은 되읽은 한 줄이다.
type Line struct {
	Date, Time, Level, Process, Account, Category, Where, Message string
}

// ParseLine 은 한 줄(끝 \n 없이)을 칸으로 나눈다. 새 형식이 아니면 ok=false.
// 원본 정규식 ^(\d{4}-\d\d-\d\d) (\d\d:\d\d:\d\d\.\d{3}) (\S+)\s+(\S+) (\S+) (\S+) (\S+) ?(.*)$ 와 같은 결과를
// 정규식 없이 낸다(200MB 파일을 한 번에 훑기 때문).
func ParseLine(s string) (Line, bool) {
	s = strings.TrimSuffix(s, "\r")
	if len(s) < 24 || !isStamp(s[:23]) || s[23] != ' ' {
		return Line{}, false
	}
	l := Line{Date: s[:10], Time: s[11:23]}
	rest := s[24:]
	var ok bool
	if l.Level, rest, ok = cutField(rest); !ok {
		return Line{}, false
	}
	rest = strings.TrimLeft(rest, " ") // 수준 뒤는 공백 하나 이상("INFO  9120")
	for _, p := range []*string{&l.Process, &l.Account, &l.Category} {
		if *p, rest, ok = cutField(rest); !ok {
			return Line{}, false
		}
	}
	l.Where, l.Message, _ = strings.Cut(rest, " ") // 메시지가 비면 "a.cpp:3 " 또는 "a.cpp:3"
	if l.Where == "" {
		return Line{}, false
	}
	return l, true
}

// cutField 는 빈칸 하나 앞까지의 비지 않은 칸과 그 뒤다.
func cutField(s string) (string, string, bool) {
	i := strings.IndexByte(s, ' ')
	if i <= 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func isStamp(s string) bool {
	const pat = "dddd-dd-dd dd:dd:dd.ddd"
	for i := 0; i < len(pat); i++ {
		if pat[i] == 'd' {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
		} else if s[i] != pat[i] {
			return false
		}
	}
	return true
}

// FilePath 는 날짜 t 의 로그 파일 경로다(로컬 날짜 yyyyMMdd.v2.log). 경로를 만드는 곳은 여기 하나다.
func FilePath(dir string, t time.Time) string {
	return filepath.Join(dir, t.Format("20060102")+".v2.log")
}

// File 은 보관 정리에 넘기는 로그 파일 하나다.
type File struct {
	Name string
	Size int64
}

// FilesToDelete 는 지울 파일 이름이다(오래된 날짜부터). keepDays>0 이면 날짜+keepDays < 오늘인 파일,
// 그 뒤 합이 maxBytes(>0) 를 넘으면 오래된 것부터. 오늘 파일·이름 앞 8자가 날짜가 아닌 파일은 지우지 않는다(합에는 센다).
func FilesToDelete(files []File, today time.Time, keepDays int, maxBytes int64) []string {
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	today = day(today)
	type dated struct {
		File
		d time.Time
	}
	var total int64
	var ds []dated
	for _, f := range files {
		total += f.Size
		if len(f.Name) < 8 {
			continue
		}
		d, err := time.Parse("20060102", f.Name[:8])
		if err != nil || !d.Before(today) {
			continue
		}
		ds = append(ds, dated{f, d})
	}
	sort.Slice(ds, func(i, j int) bool {
		if !ds[i].d.Equal(ds[j].d) {
			return ds[i].d.Before(ds[j].d)
		}
		return ds[i].Name < ds[j].Name
	})
	out := []string{}
	for _, f := range ds {
		tooOld := keepDays > 0 && f.d.AddDate(0, 0, keepDays).Before(today)
		tooBig := maxBytes > 0 && total > maxBytes
		if tooOld || tooBig {
			out = append(out, f.Name)
			total -= f.Size
		}
	}
	return out
}

// Pw 는 비밀값을 길이만 남긴다: "*** len=N"(LOGIN TRY 의 PW[...] 등).
func Pw(s string) string { return "*** len=" + strconv.Itoa(utf8.RuneCountInString(s)) }
