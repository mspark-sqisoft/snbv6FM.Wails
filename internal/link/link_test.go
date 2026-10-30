package link

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHostName(t *testing.T) {
	cases := []struct{ url, method, want string }{
		{"https://www.daum.net/", "white", "daum.net"},
		{"http://abc.com:8080/x", "white", "abc.com"},
		{"https://a.www.b.com/x", "white", "a.b.com"},
		{"https://abc.com/x", "", "abc.com"},
		{"https://abc.com/x", "black", "https://abc.com"},
		{"http://abc.com:8080/x", "black", "http://abc.com"},
		{"www.abc.com/x", "black", "abc.com"},
		{"https://www.abc.com/x", "black", "https://www.abc.com"},
		{"https://abc.com/x", "BLACK", "abc.com"}, // 원본: 호스트 뽑기는 대소문자를 가린다
	}
	for _, c := range cases {
		if got := HostName(c.url, c.method); got != c.want {
			t.Errorf("%s %s: %q, 기대 %q", c.url, c.method, got, c.want)
		}
	}
}

func TestIsTarget(t *testing.T) {
	cases := []struct {
		host, method string
		filter       []string
		want         bool
	}{
		{"daum.net", "white", []string{"daum"}, true},
		{"daum.net", "white", []string{"naver"}, false},
		{"daum.net", "black", []string{"daum"}, false},
		{"daum.net", "BLACK", []string{"naver"}, true},
		{"daum.net", "white", []string{""}, false}, // 빈 항목은 걸리지 않는다
		{"daum.net", "black", []string{""}, true},
		{"daum.net", "white", []string{"DAUM"}, true}, // 항목 대소문자 통일
		{"", "black", nil, false},
	}
	for _, c := range cases {
		if got := IsTarget(c.host, c.method, c.filter); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func TestDecide(t *testing.T) {
	rule := Rule{Business: true, Mode: "2", Method: "white", Filter: []string{"daum"}}
	mod := func(fn func(*Rule)) Rule {
		r := rule
		fn(&r)
		return r
	}
	cases := []struct {
		name, body string
		rule       Rule
		want       string
	}{
		{"정상", "~~docTitle=Daum redirectUrl=https://www.daum.net/Path", rule, "https://www.daum.net/Path"},
		{"제목 속 값", "~~docTitle=redirectUrl=https://x.test redirectUrl=https://daum.net/", rule, "https://daum.net/"},
		{"주소 속 값", "~~docTitle=t redirectUrl=https://daum.net/?redirectUrl=https://a", rule, "https://daum.net/?redirectUrl=https://a"},
		{"redirectUrl 없음", "~~docTitle=daum", rule, ""},
		{"인터넷망", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Business = false }), ""},
		{"모드 없음", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Mode = "" }), ""},
		{"모드 0", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Mode = "0" }), ""},
		{"대상 아님", "~~docTitle=t redirectUrl=https://naver.com/", rule, ""},
		{"웹 주소 아님", "~~docTitle=t redirectUrl=file://daum/x", rule, ""},
		{"black 대상", "~~docTitle=t redirectUrl=https://naver.com/", mod(func(r *Rule) { r.Method = "black" }), "https://naver.com/"},
		{"black 걸림", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Method = "black" }), ""},
	}
	for _, c := range cases {
		if got := Decide(c.body, c.rule, nil); got != c.want {
			t.Errorf("%s: %q, 기대 %q", c.name, got, c.want)
		}
	}
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args  []string
		mode  string
		paths []string
	}{
		{[]string{"--mode=upload", `C:\a b\c.txt`}, "upload", []string{`C:\a b\c.txt`}},
		{[]string{"--mode", "download", `C:\d`}, "download", []string{`C:\d`}},
		{[]string{"--mode", "UPLOAD", "a", "b"}, "upload", []string{"a", "b"}},
		{[]string{`C:\noupdate\a`}, "", []string{`C:\noupdate\a`}}, // 경로의 noupdate 로 모드가 바뀌지 않는다
		{nil, "", nil},
	}
	for _, c := range cases {
		mode, paths := ParseArgs(c.args)
		if mode != c.mode || !reflect.DeepEqual(paths, c.paths) {
			t.Errorf("%v: %q %v", c.args, mode, paths)
		}
	}
}

func TestDedup(t *testing.T) {
	var d Dedup
	now := time.Now()
	if !d.First("a", now, time.Second) || d.First("a", now.Add(time.Millisecond), time.Second) ||
		!d.First("b", now, time.Second) || !d.First("a", now.Add(3*time.Second), time.Second) {
		t.Fatal("중복 억제")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestServer(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	port := freePort(t)
	got := make(chan string, 1)
	s, err := Start(log, port, func(body string) bool { got <- body; return !strings.Contains(body, "send") })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	post := func(path, body string) int {
		res, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d%s", port, path), "application/x-www-form-urlencoded", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if c := post("/", "~~docTitle=x redirectUrl=y"); c != 200 || <-got == "" {
		t.Fatalf("200 이어야 한다: %d", c)
	}
	if c := post("/", "send"); c != 205 {
		t.Fatalf("205 여야 한다: %d", c)
	}
	<-got
	if c := post("/other", "x"); c != 404 {
		t.Fatalf("404 여야 한다: %d", c)
	}
	if c := post("/", strings.Repeat("x", bodyLimit+1)); c != http.StatusRequestEntityTooLarge {
		t.Fatalf("본문 상한: %d", c)
	}

	// 확장·로컬 프로그램만 받는다. 웹 페이지(Origin), DNS rebinding(Host), 메서드 없는 GET(img 태그)은 판정까지 가지 않는다
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	cases := []struct {
		method, host, origin string
		want                 int
	}{
		{"POST", addr, "chrome-extension://abcdef", 205},
		{"POST", addr, "moz-extension://1234", 205},
		{"POST", fmt.Sprintf("localhost:%d", port), "", 205},
		{"POST", fmt.Sprintf("[::1]:%d", port), "", 205},
		{"POST", addr, "https://evil.example", 403},
		{"POST", addr, "http://127.0.0.1:8080", 403},
		{"POST", addr, "null", 403},
		{"POST", fmt.Sprintf("evil.example:%d", port), "", 403},
		{"POST", "evil.example", "chrome-extension://abcdef", 403},
		{"GET", addr, "", 404},
		{"OPTIONS", addr, "https://evil.example", 404},
	}
	for _, c := range cases {
		req, err := http.NewRequest(c.method, "http://"+addr+"/", strings.NewReader("send"))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = c.host
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Errorf("%+v: %d", c, res.StatusCode)
		}
		if c.want == 205 {
			<-got
		} else if len(got) != 0 {
			t.Fatalf("%+v: 판정까지 갔다", c)
		}
	}

	// 포트가 점유되면 실패를 돌려준다(원본 시험과 같다)
	if s2, err := Start(log, port, func(string) bool { return true }); err == nil {
		s2.Close()
		t.Fatal("같은 포트가 또 열렸다")
	}
}
