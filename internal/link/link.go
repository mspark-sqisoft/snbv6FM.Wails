// Package link 은 브라우저 확장과 잇는 Link Redirect 창구와 판정, URL 필터·URL 보내기, 두 번째 실행 인자 읽기다(docs/port/07).
//
// 창구는 로그인과 무관하게 앱이 끝날 때까지 열어 둔다(원본 LinkRedirectServer). 판정은 Decide 하나다.
package link

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"snbapp/internal/applog"
	"snbapp/internal/nbs"
	"snbapp/pb/commonservice"
)

// Port 는 브라우저 확장이 보내는 포트다.
const Port = 29000

const bodyLimit = 64 << 10

var (
	webHostRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	numTailRe = regexp.MustCompile(`(?i)(?:^|\.)(?:[0-9]+|0x[0-9a-f]*)\.?$`)
)

// fm_url_filter.allow_flag 값. 2 는 SBM 의 차단이다.
const (
	flagOff   = 0 // 미적용
	flagProxy = 1 // 프록시 허용
)

// extensionSchemes 는 브라우저 확장이 보내는 Origin 의 스킴이다. 확장 ID 는 고정되어 있지 않아(manifest.json 에 key 가 없다) 스킴까지만 본다.
var extensionSchemes = []string{"chrome-extension", "moz-extension", "safari-web-extension"}

// trusted 는 요청이 브라우저 확장이나 이 PC 의 프로그램에서 왔는가다. 창구로 온 주소는 서버로 나가므로 웹 페이지가 부르면 안 된다 ―
// 페이지의 POST 는 Origin 에 페이지 주소가 붙고, DNS rebinding 은 Host 에 공격자의 이름이 남는다.
func trusted(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil || !(strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1") {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	scheme, _, ok := strings.Cut(origin, "://")
	return ok && slices.Contains(extensionSchemes, strings.ToLower(scheme))
}

// Server 는 루프백 두 소켓의 HTTP 창구다.
type Server struct {
	srv *http.Server
	lns []net.Listener
}

// Start 는 127.0.0.1·::1 에서 듣기 시작한다. handle 이 거짓이면 205(확장이 안내 페이지로 바꾼다), 참이면 200 이다.
// 둘 다 못 열면 오류 ― 앱은 그대로 돌고 링크 연동만 쓰지 못한다.
func Start(log *slog.Logger, port int, handle func(body string) bool) (*Server, error) {
	log = log.With("category", applog.CatApp)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{$}", func(w http.ResponseWriter, r *http.Request) {
		if !trusted(r) {
			log.Warn("LinkRedirect httpServer 출처를 믿을 수 없는 요청 거부:", "origin", r.Header.Get("Origin"), "host", r.Host)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyLimit))
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		// 방문 주소가 로그에 남지 않게 메서드·길이만 적는다
		log.Debug("LinkRedirect httpServer 요청 수신:", "method", r.Method, "len", len(body))
		if handle(string(body)) {
			w.Write([]byte("LinkRedirect"))
			return
		}
		w.WriteHeader(http.StatusResetContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Warn("LinkRedirect httpServer 처리되지 않은 요청(404):", "method", r.Method, "path", r.URL.Path)
		http.NotFound(w, r)
	})
	s := &Server{srv: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}}
	lc := net.ListenConfig{Control: exclusive}
	var errs []error
	for _, ip := range []string{"127.0.0.1", "::1"} {
		ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.lns = append(s.lns, ln)
	}
	if len(s.lns) == 0 {
		log.Warn(fmt.Sprintf("LinkRedirect httpServer listen failed port[%d] - 다른 프로그램이 port를 사용중인지 확인하세요.", port), "err", errors.Join(errs...))
		return nil, errors.Join(errs...)
	}
	for _, ln := range s.lns {
		go s.srv.Serve(ln)
	}
	if len(s.lns) == 1 {
		log.Warn(fmt.Sprintf("LinkRedirect httpServer Listening port[%d] - 한쪽만 열렸습니다.", port), "addr", s.lns[0].Addr().String(), "err", errors.Join(errs...))
	} else {
		log.Info(fmt.Sprintf("LinkRedirect httpServer Listening port[%d] IPv4[127.0.0.1] IPv6[::1]", port))
	}
	return s, nil
}

// Close 는 창구를 닫는다.
func (s *Server) Close() {
	if s != nil {
		s.srv.Close()
	}
}

// Rule 은 판정에 쓰는 정책 값이다.
type Rule struct {
	Business bool                           // 업무망 세션
	Mode     string                         // fm_hyperlink_mode
	Method   string                         // url_check_method
	Filter   []*commonservice.UrlFilterInfo // URL 필터
}

// redirectURL 은 본문에서 확장이 끝에 붙인 실제 주소다. 페이지 제목에 든 값을 피하려고 마지막 것을 쓴다.
// " redirectUrl=" 를 먼저 찾아 주소 안에 든 redirectUrl= 로 잘리지 않게 한다.
func redirectURL(body string) (string, bool) {
	const key = "redirectUrl="
	i := strings.LastIndex(body, " "+key)
	if i >= 0 {
		i++
	} else if i = strings.LastIndex(body, key); i < 0 {
		return "", false
	}
	return strings.TrimSpace(body[i+len(key):]), true
}

// HostName 은 소문자 주소에서 판정할 호스트를 뽑는다. 파서를 못 지나면 "".
// method 가 정확히 "black" 이면 원본과 같은 글자 자르기라 스킴이 남고(https://abc.com), 그 밖이면 URL 파서의 호스트다(www.abc.com) ―
// 글자 자르기는 https://evil.example?.abc.com·https://abc.com@evil.example 의 호스트를 잘못 읽는다.
func HostName(webURL, method string) string {
	if method == "black" {
		s := webURL
		at := 0
		if i := strings.Index(s, "http://"); i >= 0 {
			at = i + len("http://")
		} else if i := strings.Index(s, "https://"); i >= 0 {
			at = i + len("https://")
		}
		if i := strings.Index(s[at:], ":"); i >= 0 {
			s = s[:at+i]
		}
		s = strings.TrimPrefix(s, "www.")
		if at <= len(s) {
			if i := strings.Index(s[at:], "/"); i >= 0 {
				s = s[:at+i]
			}
		}
		return s
	}
	u, err := url.Parse(webURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// IsTarget 은 호스트가 전송 대상인가다. white 는 목록의 도메인과 그 하위 도메인만 보낸다 ― 목록은 망을 건너는 허용 목록이라 daum.net 이
// daum.net.evil.example 에 걸리면 안 된다(보안 D-006, 서버 proxy.pac 의 부분 문자열 규칙과 일부러 다르다). black 은 항목이 호스트 어디에든
// 있으면 빼는 proxy.pac 규칙 그대로다 ― 넓게 걸려도 보내지 않는 쪽으로 틀린다. 호스트는 이미 소문자라 항목도 소문자로 맞추고, 빈 항목은 건너뛴다.
// flag(SBM 의 allow_flag): 미적용 행은 판정에 쓰지 않고, 프록시 허용 행만 white 목록이 되며, 그 밖의 행은 어느 모드든 black 처럼 뺀다.
func IsTarget(host, method string, filter []*commonservice.UrlFilterInfo) bool {
	if host == "" {
		return false
	}
	black := strings.EqualFold(method, "black")
	target := black
	for _, f := range filter {
		item := strings.ToLower(strings.TrimSpace(f.Url))
		if item == "" || f.Flag == flagOff {
			continue
		}
		if (black || f.Flag != flagProxy) && strings.Contains(host, item) {
			return false
		}
		if f.Flag == flagProxy && (host == item || strings.HasSuffix(host, "."+item)) {
			target = true
		}
	}
	return target
}

// IsWebURL 은 http/https 이고 호스트가 있는 주소인가다. 보내는 쪽·받는 쪽이 같은 판정을 쓴다(file://·UNC·등록된 프로토콜을 열지 않는다).
// 호스트는 ASCII 이름이나 IP 만 받는다 ― url.Parse 는 호스트의 " 를 받는데 브라우저로 넘길 때 \" 가 되어 다른 호스트로 열리고, ToLower 는 İ 를 i 로
// 줄이는데 브라우저는 다른 도메인으로 읽는다. 끝 라벨이 숫자인 이름도 거절한다 ― 브라우저는 010.1.2.3·0x7f.1·192.168.1 을 IPv4 로 풀어 표기와 다른 주소를 연다.
// 브라우저가 한글 도메인을 xn-- 로 바꿔 주므로 확장이 보내는 주소는 걸리지 않는다.
func IsWebURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && webHost(u.Hostname())
}

func webHost(h string) bool {
	if a, err := netip.ParseAddr(h); err == nil {
		return a.Zone() == ""
	}
	return webHostRe.MatchString(h) && !numTailRe.MatchString(h)
}

// Decide 는 확장 요청 본문을 판정해 반대 망으로 보낼 주소를 돌려준다. 보낼 것이 없으면 "".
// 원본은 주소 전체를 소문자로 보내 대소문자를 가리는 경로가 깨졌다 ― 판정만 소문자로 하고 보내는 주소는 그대로 둔다.
func Decide(body string, r Rule, warn func(string)) string {
	if !r.Business || r.Mode == "" || r.Mode == "0" {
		return ""
	}
	raw, ok := redirectURL(body)
	if !ok {
		return ""
	}
	lower := strings.ToLower(raw)
	host := HostName(lower, r.Method)
	if !IsTarget(host, r.Method, r.Filter) {
		return ""
	}
	if !IsWebURL(raw) {
		if warn != nil {
			warn("링크연동 주소가 http/https 가 아니거나 호스트를 받지 않아 보내지 않았다") // 방문 주소는 로그에 남기지 않는다
		}
		return ""
	}
	if !strings.Contains(lower, host) {
		return ""
	}
	return raw
}

// Dedup 은 같은 주소를 잠깐 동안 한 번만 통과시킨다(확장이 하위 프레임 완료마다 보낸다).
type Dedup struct {
	mu   sync.Mutex
	last string
	at   time.Time
}

// First 는 url 이 window 안에 처음 온 것이면 참이다.
func (d *Dedup) First(url string, now time.Time, window time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if url == d.last && now.Sub(d.at) < window {
		return false
	}
	d.last, d.at = url, now
	return true
}

// Gate 는 상대망에서 온 주소를 열기 전에 사용자에게 묻는 자리다. 허락한 호스트는 Reset 까지 다시 묻지 않는다.
// ponytail: 창은 하나만 띄우고 그동안 온 새 호스트는 버린다(밀려드는 주소에 창이 쌓이지 않게) ― 대기열이 필요하면 여기서 늘린다.
type Gate struct {
	mu     sync.Mutex
	hosts  map[string]bool
	asking bool
}

// Open 은 host 를 열어도 되는가다. 허락한 호스트면 묻지 않고, 아니면 ask 가 답할 때까지 기다린다.
func (g *Gate) Open(host string, ask func() bool) bool {
	g.mu.Lock()
	if g.hosts[host] {
		g.mu.Unlock()
		return true
	}
	if g.asking {
		g.mu.Unlock()
		return false
	}
	g.asking = true
	g.mu.Unlock()

	ok := ask()

	g.mu.Lock()
	defer g.mu.Unlock()
	g.asking = false
	if ok {
		if g.hosts == nil {
			g.hosts = map[string]bool{}
		}
		g.hosts[host] = true
	}
	return ok
}

// Reset 은 허락을 모두 잊는다(로그아웃).
func (g *Gate) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hosts = nil
}

// FetchFilter 는 [3094] URL 필터를 받는다. 서버는 적용 범위와 상관없이 모든 행을 준다.
func FetchFilter(s *nbs.Session) ([]*commonservice.UrlFilterInfo, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Common().GetUrlFilter(ctx, &commonservice.UrlFilterRequest{Code: 0})
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

// ErrMalware 는 [3204] 가 악성코드로 거절했을 때다.
var ErrMalware = errors.New("링크 연동에서 악성코드가 발견되었습니다.")

// SendURL 은 [3204] 로 주소를 반대 망에 보낸다.
func SendURL(s *nbs.Session, webURL string) error {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Common().SendURL(ctx, &commonservice.URLInfo{Url: webURL})
	if err != nil {
		return err
	}
	if res.Res == 1 {
		return nil
	}
	if res.Code == 4 {
		return ErrMalware
	}
	return fmt.Errorf("링크 연동 실패: code[%d] %s", res.Code, res.Reason)
}

// ParseArgs 는 실행 인자(0번 실행 파일 제외)에서 --mode 값과 경로들을 읽는다. 원본과 달리 경로를 모두 받는다.
func ParseArgs(args []string) (mode string, paths []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--mode" && i+1 < len(args):
			i++
			mode = args[i]
		case strings.HasPrefix(a, "--mode="):
			mode = a[len("--mode="):]
		case strings.HasPrefix(a, "-"):
		default:
			paths = append(paths, a)
		}
	}
	return strings.ToLower(mode), paths
}
