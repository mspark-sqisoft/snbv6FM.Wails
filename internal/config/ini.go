package config

// fmenv.ini·fmlocal.ini·fmusr.ini 읽기. 원본은 Qt QSettings(IniFormat)로 읽는다(FilemoverSettings.cpp:58-123, main.cpp:665-674).
// 외부 ini 패키지 없이 손으로 짠 해석기 하나로 읽는다(docs/port/09 7.1, 12 7.1).

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 망 위치(fmenv.ini LOC).
const (
	LocInternet = 0 // 인터넷망(EXTERNAL)
	LocBusiness = 1 // 업무망(INTERNAL)
)

const defaultPort = 29001 // 원본 FilemoverSettings.cpp:75. 배포 파일은 PORT=29002 를 적는다

// Server 는 fmenv.ini 의 [SERVERnn] 한 칸이다.
type Server struct {
	Index   int      `json:"index"`   // 목록 자리(fmlocal.ini 로 정렬한 뒤). 설정 serverIndex 가 가리키는 값
	Name    string   `json:"name"`    // 화면에 보이는 망 이름(예: 업무망). 기본 NONAME
	Network string   `json:"network"` // 전송 방향 글자(예: 업무망->인터넷망). 기본 NONAME
	IP      string   `json:"ip"`      // fmenv.ini 에 적힌 그대로(`_` 로 여럿)
	IPs     []string `json:"ips"`     // IP 를 `_` 로 가른 것(빈 조각 제외)
	Port    int      `json:"port"`    // 숫자가 아니면 0 ― 접속에서 실패한다(원본 toInt() 와 같다)
	Loc     int      `json:"loc"`     // LocBusiness 또는 LocInternet
}

// Env 는 실행 파일 폴더의 fmenv.ini·fmlocal.ini 를 읽은 결과다. 앱 시작 때 한 번 읽는다(고치면 재시작).
type Env struct {
	Servers []Server // fmlocal.ini LOC 의 망이 맨 앞
	Home    int      // fmlocal.ini [NETWORK] LOC. 없거나 이상하면 -1

	LogLevel    string // fmlocal.ini [LOG] LEVEL(대문자). 기본 DEBUG
	LogKeepDays int    // fmlocal.ini [LOG] KEEP_DAYS. 기본 15(정책 remove_logfile 이 이긴다)
	LogMaxMB    int    // fmlocal.ini [LOG] MAX_MB. 기본 200, 0 = 상한 없음

	fmenv iniFile
}

// Value 는 fmenv.ini 의 아무 값이나 읽는다([CONFIG] SERIAL_TRANSFER_SIZE·DOMAIN_POLICY 등은 쓰는 영역이 직접 읽는다).
func (e *Env) Value(section, key string) (string, bool) {
	v, ok := e.fmenv[section][key]
	return v, ok
}

// LoadEnv 는 appDir 의 fmenv.ini(없으면 오류)와 fmlocal.ini(없으면 기본값)를 읽는다.
func LoadEnv(appDir string) (*Env, error) {
	envPath := filepath.Join(appDir, "fmenv.ini")
	fmenv, err := readINI(envPath)
	if err != nil {
		return nil, fmt.Errorf("fmenv.ini 를 읽지 못했습니다: %w", err)
	}
	local, err := readINI(filepath.Join(appDir, "fmlocal.ini"))
	if err != nil && !os.IsNotExist(err) {
		slog.Warn("fmlocal.ini 를 읽지 못했다 ― 기본값으로 시작한다", "err", err)
	}
	if tls, ok := fmenv["GRPC"]["TLS"]; ok && tls == "0" {
		slog.Warn("gRPC TLS 를 끄려는 설정을 무시한다(fmenv.ini [GRPC] TLS=0) >>> 암호화 접속을 유지한다")
	}
	e := &Env{
		Home:        homeLoc(local),
		LogLevel:    strings.ToUpper(strings.TrimSpace(orDefault(local["LOG"]["LEVEL"], "DEBUG"))),
		LogKeepDays: intOr(local["LOG"]["KEEP_DAYS"], 15),
		LogMaxMB:    intOr(local["LOG"]["MAX_MB"], 200),
		fmenv:       fmenv,
	}
	e.Servers = orderByHome(parseServers(fmenv), e.Home)
	return e, nil
}

// parseServers 는 [SERVER01] 부터 IP 가 빈 첫 구획에서 멈춘다(그 뒤 번호는 읽지 않는다).
// IP 를 `_` 로 갈라 남는 것이 없으면 그 구획만 건너뛰고 다음 번호를 계속 읽는다.
func parseServers(f iniFile) []Server {
	var out []Server
	for n := 1; ; n++ {
		sec := fmt.Sprintf("SERVER%02d", n)
		raw := strings.TrimSpace(f[sec]["IP"])
		if raw == "" {
			break
		}
		var ips []string
		for _, p := range strings.Split(raw, "_") {
			if p = strings.TrimSpace(p); p != "" {
				ips = append(ips, p)
			}
		}
		if len(ips) == 0 {
			slog.Warn("서버 IP 가 비어 이 구획을 건너뛴다", "section", sec, "ip", raw)
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(orDefault(f[sec]["PORT"], strconv.Itoa(defaultPort))))
		if err != nil {
			port = 0
		}
		loc := LocBusiness
		switch v := strings.TrimSpace(f[sec]["LOC"]); v {
		case "0":
			loc = LocInternet
		case "1":
		default:
			if _, ok := f[sec]["LOC"]; ok {
				slog.Warn("fmenv.ini LOC 가 0·1 이 아니다 ― 업무망(1)으로 본다", "section", sec, "loc", v)
			}
		}
		out = append(out, Server{
			Name:    strings.TrimSpace(orDefault(f[sec]["NAME"], "NONAME")),
			Network: strings.TrimSpace(orDefault(f[sec]["NETWORK"], "NONAME")),
			IP:      raw, IPs: ips, Port: port, Loc: loc,
		})
	}
	return out
}

// homeLoc 는 fmlocal.ini [NETWORK] LOC(이 PC 의 망)다. 없거나 이상하면 -1(경고는 이상한 값일 때만).
func homeLoc(local iniFile) int {
	v, ok := local["NETWORK"]["LOC"]
	switch v = strings.TrimSpace(v); {
	case v == "0":
		return LocInternet
	case v == "1":
		return LocBusiness
	case ok:
		slog.Warn("fmlocal.ini [NETWORK] LOC 가 0·1 이 아니다 ― fmenv.ini 순서를 쓴다", "loc", v)
	}
	return -1
}

// orderByHome 은 home 망의 서버를 앞에 두고(순서 유지) Index 를 매긴다.
func orderByHome(servers []Server, home int) []Server {
	out := make([]Server, 0, len(servers))
	for pass := 0; pass < 2; pass++ {
		for _, s := range servers {
			if (s.Loc == home) == (pass == 0) {
				out = append(out, s)
			}
		}
	}
	for i := range out {
		out[i].Index = i
	}
	return out
}

// UserINI 는 [USER] 값이다: fmenv.ini [USER] 를 dataDir 의 fmusr.ini [USER] 가 덮는다.
// fmusr.ini 의 다른 구획(접속 키 등)은 무시하고 경고한다(원본 SEC-016). 옛 설정을 옮길 때 쓴다.
func UserINI(appDir, dataDir string) map[string]string {
	out := map[string]string{}
	if f, err := readINI(filepath.Join(appDir, "fmenv.ini")); err == nil {
		for k, v := range f["USER"] {
			out[k] = v
		}
	}
	usrPath := filepath.Join(dataDir, "fmusr.ini")
	f, err := readINI(usrPath)
	if err != nil {
		return out
	}
	for sec, kv := range f {
		for k, v := range kv {
			if sec != "USER" {
				slog.Warn(fmt.Sprintf("사용자 설정으로 덮어쓸 수 없는 항목이다(무시한다) >>> %s/%s - %s", sec, k, usrPath))
				continue
			}
			out[k] = v
		}
	}
	return out
}

// iniFile 은 구획 → 키 → 값이다. 구획·키는 대소문자를 가린다(원본 주석 "SECTION, KEY 는 모두 대문자").
type iniFile map[string]map[string]string

func readINI(path string) (iniFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseINI(string(b)), nil
}

// parseINI 는 BOM 을 견디고, 줄 앞 `;`·`#` 는 주석, 줄의 첫 `=` 로 키·값을 가른다. 값 뒤의 `;` 는 주석으로 보지 않는다.
func parseINI(s string) iniFile {
	f := iniFile{}
	sec := ""
	for _, line := range strings.Split(strings.TrimPrefix(s, "\xEF\xBB\xBF"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || line[0] == ';' || line[0] == '#':
		case line[0] == '[' && strings.HasSuffix(line, "]"):
			sec = strings.TrimSpace(line[1 : len(line)-1])
		default:
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			if f[sec] == nil {
				f[sec] = map[string]string{}
			}
			f[sec][strings.TrimSpace(k)] = qtUnescape(strings.TrimSpace(v))
		}
	}
	return f
}

// qtUnescape 는 QSettings 가 쓴 값을 푼다: 감싼 큰따옴표를 벗기고 \\ \" \n \t \r 을 바꾼다.
// 원본이 쓴 fmusr.ini 는 경로의 \ 를 \\ 로 적는다(예: C:/Users/x/Desktop\\AUTO_UPLOAD).
func qtUnescape(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	if !strings.Contains(v, `\`) {
		return v
	}
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' || i+1 == len(v) {
			b.WriteByte(v[i])
			continue
		}
		i++
		switch v[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		default: // \\ → \, \" → "
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intOr(v string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}
