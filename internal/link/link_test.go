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

	"snbapp/pb/commonservice"
)

// rows 는 같은 적용 범위의 필터 행을 만든다.
func rows(flag int32, urls ...string) []*commonservice.UrlFilterInfo {
	var out []*commonservice.UrlFilterInfo
	for _, u := range urls {
		out = append(out, &commonservice.UrlFilterInfo{Flag: flag, Url: u})
	}
	return out
}

func TestHostName(t *testing.T) {
	cases := []struct{ url, method, want string }{
		{"https://www.daum.net/", "white", "www.daum.net"},
		{"http://abc.com:8080/x", "white", "abc.com"},
		{"https://a.www.b.com/x", "white", "a.www.b.com"},
		{"https://abc.com/x", "", "abc.com"},
		{"https://evil.example?.abc.com", "white", "evil.example"}, // 글자 자르기는 evil.example?.abc.com 으로 읽어 끝 일치를 통과시킨다
		{"https://evil.example#.abc.com", "white", "evil.example"},
		{"https://abc.com@evil.example/", "white", "evil.example"},
		{"https://abc.com:x@evil.example/", "white", "evil.example"},
		{"https://abc.com\\.evil.example/", "white", ""}, // 파서를 못 지나면 호스트가 없다
		{"http://[fe80::1]:8080/", "white", "fe80::1"},
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
		{"daum.net", "white", []string{"daum.net"}, true},
		{"mail.daum.net", "white", []string{"daum.net"}, true},
		{"daum.net", "", []string{"daum.net"}, true},
		{"daum.net", "white", []string{"naver.com"}, false},
		{"daum.net", "white", []string{"daum"}, false}, // 도메인 경계에서만 걸린다(보안 D-006)
		{"daum.net.evil.example", "white", []string{"daum.net"}, false},
		{"daum.net.evil.example", "", []string{"daum.net"}, false},
		{"evildaum.net", "white", []string{"daum.net"}, false},
		{"daum.evil.example", "white", []string{"daum"}, false},
		{"www.daum.net", "white", []string{"www.daum.net"}, true},
		{"daum.net", "white", []string{"www.daum.net"}, false},
		{"10.1.2.3", "white", []string{"10.1.2.3"}, true},
		{"10.1.2.33", "white", []string{"10.1.2.3"}, false},
		{"daum.net", "white", []string{"https://daum.net"}, false}, // 호스트가 아닌 항목은 원본 white 처럼 걸리지 않는다
		{"daum.net", "black", []string{"daum"}, false},             // black 은 부분 문자열 그대로다
		{"daum.net.evil.example", "black", []string{"daum.net"}, false},
		{"daum.net", "BLACK", []string{"naver"}, true},
		{"daum.net", "white", []string{""}, false}, // 빈 항목은 걸리지 않는다
		{"daum.net", "black", []string{""}, true},
		{"daum.net", "white", []string{" DAUM.net "}, true}, // 항목 공백·대소문자 통일
		{"", "black", nil, false},
	}
	for _, c := range cases {
		if got := IsTarget(c.host, c.method, rows(flagProxy, c.filter...)); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func TestIsTargetFlag(t *testing.T) {
	const block = 2
	allow := rows(flagProxy, "daum.net")
	cases := []struct {
		name, host, method string
		filter             []*commonservice.UrlFilterInfo
		want               bool
	}{
		{"white 미적용", "daum.net", "white", rows(flagOff, "daum.net"), false},
		{"white 프록시 허용", "daum.net", "white", allow, true},
		{"white 차단", "daum.net", "white", rows(block, "daum.net"), false},
		{"white 모르는 값", "daum.net", "white", rows(3, "daum.net"), false},
		{"black 미적용", "daum.net", "black", rows(flagOff, "daum.net"), true},
		{"black 프록시 허용", "daum.net", "black", allow, false},
		{"black 차단", "daum.net", "black", rows(block, "daum.net"), false},
		{"black 모르는 값", "daum.net", "black", rows(3, "daum.net"), false},
		{"허용 뒤 하위 차단", "bad.daum.net", "white", append(rows(flagProxy, "daum.net"), rows(block, "bad.daum.net")...), false},
		{"하위 차단 뒤 허용", "bad.daum.net", "white", append(rows(block, "bad.daum.net"), rows(flagProxy, "daum.net")...), false},
		{"다른 하위는 허용", "mail.daum.net", "white", append(rows(block, "bad.daum.net"), rows(flagProxy, "daum.net")...), true},
	}
	for _, c := range cases {
		if got := IsTarget(c.host, c.method, c.filter); got != c.want {
			t.Errorf("%s: %v, 기대 %v", c.name, got, c.want)
		}
	}
}

func TestDecide(t *testing.T) {
	rule := Rule{Business: true, Mode: "2", Method: "white", Filter: rows(flagProxy, "daum.net")}
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
		{"하위 도메인", "~~docTitle=t redirectUrl=https://mail.daum.net/", rule, "https://mail.daum.net/"},
		{"유사 도메인", "~~docTitle=t redirectUrl=https://daum.net.evil.example/", rule, ""},
		{"앞붙은 도메인", "~~docTitle=t redirectUrl=https://evildaum.net/", rule, ""},
		{"사용자 정보", "~~docTitle=t redirectUrl=https://daum.net@evil.example/", rule, ""},
		{"쿼리 속 도메인", "~~docTitle=t redirectUrl=https://evil.example?.daum.net", rule, ""},
		{"조각 속 도메인", "~~docTitle=t redirectUrl=https://evil.example#.daum.net", rule, ""},
		{"경로 속 도메인", "~~docTitle=t redirectUrl=https://evil.example/.daum.net", rule, ""},
		{"제목 속 값", "~~docTitle=redirectUrl=https://x.test redirectUrl=https://daum.net/", rule, "https://daum.net/"},
		{"주소 속 값", "~~docTitle=t redirectUrl=https://daum.net/?redirectUrl=https://a", rule, "https://daum.net/?redirectUrl=https://a"},
		{"redirectUrl 없음", "~~docTitle=daum", rule, ""},
		{"인터넷망", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Business = false }), ""},
		{"모드 없음", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Mode = "" }), ""},
		{"모드 0", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Mode = "0" }), ""},
		{"대상 아님", "~~docTitle=t redirectUrl=https://naver.com/", rule, ""},
		{"웹 주소 아님", "~~docTitle=t redirectUrl=file://daum.net/x", rule, ""},
		{"호스트의 따옴표", `~~docTitle=t redirectUrl=https://evil.example"x.daum.net/`, rule, ""},
		{"소문자로 줄어드는 글자", "~~docTitle=t redirectUrl=https://dİum.daum.net/", rule, ""},
		{"대문자 호스트는 그대로 보낸다", "~~docTitle=t redirectUrl=https://daum.neT/Path", rule, "https://daum.neT/Path"},
		{"black 대상", "~~docTitle=t redirectUrl=https://naver.com/", mod(func(r *Rule) { r.Method = "black" }), "https://naver.com/"},
		{"black 걸림", "~~docTitle=t redirectUrl=https://daum.net/", mod(func(r *Rule) { r.Method = "black" }), ""},
		{"black 숫자 호스트", "~~docTitle=t redirectUrl=http://012.1.2.3/", mod(func(r *Rule) { r.Method = "black" }), ""},
	}
	for _, c := range cases {
		if got := Decide(c.body, c.rule, nil); got != c.want {
			t.Errorf("%s: %q, 기대 %q", c.name, got, c.want)
		}
	}
}

func TestIsWebURL(t *testing.T) {
	for s, want := range map[string]bool{
		"https://daum.net/a":               true,
		"http://my_host-1.intra:8080/":     true,
		"http://10.0.0.1/":                 true,
		"http://[2001:db8::1]:8080/":       true,
		"https://xn--github-qyd.com/":      true,
		"http://1e3/":                      true,
		`https://daum.net/a"b`:             true,
		"http://010.1.2.3/":                false,
		"http://192.168.1/":                false,
		"http://0x7f.1/":                   false,
		"http://0x7f/":                     false,
		"http://1.0X7F/":                   false,
		"http://a.0x/":                     false,
		"http://0xg/":                      true,
		"http://2130706433/":               false,
		"http://10.0.0.1./":                false,
		`https://evil.example"x.daum.net/`: false,
		"https://gİthub.com/":              false,
		"https://한글.kr/":                   false,
		"https://ex%C4%B0mple.com/":        false,
		"http://[fe80::1%25en0]/":          false,
		"http:///a":                        false,
		"file:///C:/a":                     false,
		"javascript:alert(1)":              false,
	} {
		if got := IsWebURL(s); got != want {
			t.Errorf("%s: %v", s, got)
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

func TestGate(t *testing.T) {
	var g Gate
	asks := 0
	answer := func(ok bool) func() bool {
		return func() bool { asks++; return ok }
	}
	if g.Open("a.test", answer(false)) || asks != 1 {
		t.Fatal("거절하면 열지 않는다")
	}
	if !g.Open("a.test", answer(true)) || asks != 2 {
		t.Fatal("거절한 호스트는 다시 묻는다")
	}
	if !g.Open("a.test", answer(false)) || asks != 2 {
		t.Fatal("허락한 호스트는 묻지 않는다")
	}
	if g.Open("b.test", func() bool {
		if g.Open("c.test", answer(true)) {
			t.Error("창이 떠 있는 동안 온 새 호스트를 열었다")
		}
		if !g.Open("a.test", answer(false)) {
			t.Error("창이 떠 있어도 허락한 호스트는 연다")
		}
		return false
	}) {
		t.Fatal("거절하면 열지 않는다")
	}
	if asks != 2 {
		t.Fatalf("창이 떠 있는 동안 물었다: %d", asks)
	}
	g.Reset()
	if !g.Open("a.test", answer(true)) || asks != 3 {
		t.Fatal("로그아웃하면 다시 묻는다")
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
