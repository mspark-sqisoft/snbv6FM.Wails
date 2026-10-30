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
	"net/url"
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
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
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
	Business bool     // 업무망 세션
	Mode     string   // fm_hyperlink_mode
	Method   string   // url_check_method
	Filter   []string // URL 필터
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

// HostName 은 소문자 주소에서 판정할 호스트를 원본과 같은 글자 자르기로 뽑는다.
// method 가 정확히 "black" 이면 스킴이 남고(https://abc.com), 그 밖이면 스킴·www. 를 모두 지운다(abc.com).
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
	s := strings.NewReplacer("http://", "", "https://", "", "www.", "").Replace(webURL)
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}

// IsTarget 은 호스트가 전송 대상인가다. white 는 목록에 걸린 것만, black 은 걸리지 않은 것만 보낸다(서버가 proxy.pac 을 만드는 규칙과 같다).
// 호스트는 이미 소문자라 필터 항목도 소문자로 맞춘다. 빈 항목은 모든 호스트에 걸리므로 건너뛴다.
func IsTarget(host, method string, filter []string) bool {
	if host == "" {
		return false
	}
	hit := false
	for _, f := range filter {
		if f = strings.ToLower(f); f != "" && strings.Contains(host, f) {
			hit = true
			break
		}
	}
	if strings.EqualFold(method, "black") {
		return !hit
	}
	return hit
}

// IsWebURL 은 http/https 이고 호스트가 있는 주소인가다. 보내는 쪽·받는 쪽이 같은 판정을 쓴다(file://·UNC·등록된 프로토콜을 열지 않는다).
func IsWebURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && u.Hostname() != ""
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
			warn("링크연동 주소가 http/https 가 아니라 보내지 않았다") // 방문 주소는 로그에 남기지 않는다
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

// FetchFilter 는 [3094] URL 필터를 받아 url 칸만 돌려준다.
func FetchFilter(s *nbs.Session) ([]string, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Common().GetUrlFilter(ctx, &commonservice.UrlFilterRequest{Code: 0})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res.List))
	for _, f := range res.List {
		out = append(out, f.Url)
	}
	return out, nil
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
