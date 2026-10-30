package clip

// 원본 tests/tst_ClipFmh.cpp 의 표를 옮겼다(디스크 없이 판정만 본다).

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	kRoot    = "C:/Users/u/temp/clip/"
	kStamp   = int64(1700000000)
	kBundled = "11727000000.png"
)

func placeholder(n int, ext string) string {
	return fmt.Sprintf("%s%d%d%s", Placeholder, n, kStamp, ext)
}
func imageHead(magic string) []byte       { return append([]byte(magic), make([]byte, 16)...) }
func cleanReceived(html string) CleanHTML { return CleanReceivedHTML(html, kRoot, []string{kBundled}) }

func nameCell(s string) []byte {
	b := make([]byte, maxName)
	copy(b, s)
	return b
}

// normalHeader 는 보내는 쪽이 만드는 모양이다: 0번 dst.htm, 1번 그림, 2번은 못 읽은 그림(크기 0·빈 이름).
func normalHeader() []byte {
	h := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(h, 3)
	binary.LittleEndian.PutUint32(h[4:], 5)
	binary.LittleEndian.PutUint32(h[8:], 4)
	setName(h, 0, "dst.htm")
	setName(h, 1, "11727000000.png")
	return h
}
func setName(h []byte, i int, name string) { copy(h[nameBase+i*maxName:], nameCell(name)) }
func setSize(h []byte, i int, n int32)     { binary.LittleEndian.PutUint32(h[4+i*4:], uint32(n)) }

func TestEntryNameKeepsFileNameOnly(t *testing.T) {
	cases := map[string]string{
		`..\x`: "x", "../x": "x", `C:\x`: "x", "C:x": "x", "a.png ": "a.png", "/abs/x": "x",
		`..\..\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Startup\x.hta`: "x.hta",
		`\\host\share\x.png`: "x.png", "dst.htm": "dst.htm", "11727000000.png": "11727000000.png",
	}
	for raw, want := range cases {
		if got := EntryName(nameCell(raw)); got != want {
			t.Errorf("%q: %q, 기대 %q", raw, got, want)
		}
	}
}

func TestEntryNameRejects(t *testing.T) {
	for _, raw := range []string{"", ".", "..", "../", `..\..`, "C:", "x.png:evil", "COM1", "nul.png", `..\CON`, "a\xffb.png"} {
		if got := EntryName(nameCell(raw)); got != "" {
			t.Errorf("%q 가 %q 로 받아들여졌다", raw, got)
		}
	}
	if got := EntryName(bytes.Repeat([]byte("A"), maxName)); got != "" {
		t.Errorf("NUL 없는 칸이 받아들여졌다: %q", got)
	}
}

func TestHeaderProblem(t *testing.T) {
	file := int64(HeaderSize + 9)
	mod := func(fn func(h []byte)) []byte {
		h := normalHeader()
		fn(h)
		return h
	}
	bad := map[string]struct {
		h    []byte
		size int64
	}{
		"한 바이트 짧음":   {normalHeader()[:HeaderSize-1], file},
		"12 바이트":     {normalHeader()[:12], 12},
		"0 바이트":      {nil, 0},
		"항목 수 음수":    {mod(func(h []byte) { setSizeRaw(h, 0, -1) }), file},
		"항목 수 초과":    {mod(func(h []byte) { setSizeRaw(h, 0, ImgMaxCnt+2) }), file},
		"항목 수 0":     {mod(func(h []byte) { setSizeRaw(h, 0, 0) }), file},
		"크기 합 초과":    {normalHeader(), HeaderSize + 8},
		"큰 크기":       {mod(func(h []byte) { setSize(h, 1, 1<<31-1) }), file},
		"더하면 int 넘음": {mod(func(h []byte) { setSize(h, 0, 1<<31-1); setSize(h, 1, 1<<31-1) }), file},
		"이름 없음":      {mod(func(h []byte) { setName(h, 1, "..") }), file},
		"이름 NUL 없음":  {mod(func(h []byte) { copy(h[nameBase+maxName:], bytes.Repeat([]byte("A"), maxName)) }), file},
		"0번 크기 0":    {mod(func(h []byte) { setSize(h, 0, 0) }), file},
		"0번 이름 다름":   {mod(func(h []byte) { setName(h, 0, "a.htm") }), file},
		"작업 파일 묶음":   {mod(func(h []byte) { setName(h, 1, clipFm) }), file},
		"작업 파일 대소문자": {mod(func(h []byte) { setName(h, 1, "__clip__.FMH") }), file},
		"작업 파일 src":  {mod(func(h []byte) { setName(h, 1, htmlSrc) }), file},
		"작업 파일 dst":  {mod(func(h []byte) { setName(h, 1, htmlDst) }), file},
		"끝 점":        {mod(func(h []byte) { setName(h, 1, "__CLIP__.fmh.") }), file},
		"끝 점 공백":     {mod(func(h []byte) { setName(h, 1, "dst.htm. .") }), file},
	}
	for name, c := range bad {
		if _, p := headerProblem(c.h, c.size); p == "" {
			t.Errorf("%s: 거부되지 않았다", name)
		}
	}
	h, p := headerProblem(normalHeader(), file)
	if p != "" || h.cnt != 3 || h.sizes[1] != 4 || h.names[1] != kBundled {
		t.Fatalf("정상 묶음: %q %+v", p, h.cnt)
	}
	if _, p := headerProblem(normalHeader(), HeaderSize+100); p != "" {
		t.Fatalf("뒤에 바이트가 남아도 받는다: %q", p)
	}
}

func setSizeRaw(h []byte, at int, n int32) { binary.LittleEndian.PutUint32(h[at:], uint32(n)) }

func TestPlainText(t *testing.T) {
	cases := map[string]string{
		`<p>가나 <b>굵게</b> <a href="https://x">링크</a></p>`:                                                                    "가나 굵게 링크",
		`<html><head><style>p{color:red}</style></head><body><!--StartFragment--><p>본문</p><!--EndFragment--></body></html>`: "본문",
		"<p>a&nbsp;b</p>\uFFFC<br>c": "a b\nc",
	}
	for in, want := range cases {
		if got := PlainText(in); got != want {
			t.Errorf("%q: %q, 기대 %q", in, got, want)
		}
	}
	got := PlainText(`<p>앞</p><img src="file:///C:/x.png"><table><tr><td>칸1</td><td>칸2</td></tr></table>`)
	if !strings.Contains(got, "앞") || !strings.Contains(got, "칸1") || !strings.Contains(got, "칸2") {
		t.Errorf("표: %q", got)
	}
}

func TestSendFindsSrcInAnyForm(t *testing.T) {
	for _, html := range []string{
		`<img src="file:///C:/w/a.png">`, `<IMG SRC="file:///C:/w/a.png">`, `<img Src = "file:///C:/w/a.png">`,
		`<img src='file:///C:/w/a.png'>`, `<img src=file:///C:/w/a.png>`, "<img\nalt=\"x\"\nsrc=\"file:///C:/w/a.png\"\n>",
		`<img alt="x"src="file:///C:/w/a.png">`, `<img/src="file:///C:/w/a.png">`,
		`<!--[if gte vml 1]><v:imagedata src="file:///C:/w/a.png" o:title=""/><![endif]-->`,
		`<img src="file&#x3A;&#47;&#47;/C:/w/a.png">`, `<img src="file:///C:\w\a.png">`,
	} {
		out := PrepareSendHTML(html, "", kRoot, kStamp, ImgMaxCnt)
		if len(out.Refs) != 1 || out.Refs[0].Remote || out.Refs[0].Local != "C:/w/a.png" || out.Refs[0].Name != fmt.Sprintf("1%d.png", kStamp) {
			t.Errorf("%s: %+v", html, out.Refs)
			continue
		}
		if !strings.Contains(out.HTML, placeholder(1, ".png")) || strings.Contains(out.HTML, "w/a.png") || strings.Contains(out.HTML, `w\a.png`) {
			t.Errorf("%s → %s", html, out.HTML)
		}
	}
}

func TestLocalImageOK(t *testing.T) {
	text := []byte("SNB-E2E-DUMMY secret text 0123456789")
	png := imageHead("\x89PNG\r\n\x1a\n")
	cases := []struct {
		path string
		head []byte
		ok   bool
	}{
		{"C:/w/dummy.txt", text, false}, {"C:/w/fake.png", text, false}, {"C:/w/id_rsa", png, false},
		{"C:/w/a.png.exe", png, false}, {"C:/w/a.png", []byte("\x89PNG"), false}, {"C:/w/a.png", png, true},
		{"C:/w/A.PNG", png, true}, {"C:/w/a.jpg", imageHead("\xff\xd8\xff\xe0"), true}, {"C:/w/a.gif", imageHead("GIF89a"), true},
		{"C:/w/a.bmp", imageHead("BM"), true}, {"C:/w/a.webp", imageHead("RIFF\x10\x00\x00\x00WEBP"), true},
		{"C:/w/a.tif", imageHead("II*\x00"), true}, {"C:/w/a.tiff", imageHead("MM\x00*"), true}, {"C:/w/a.jpg", png, true},
	}
	for _, c := range cases {
		if got := LocalImageOK(c.path, c.head); got != c.ok {
			t.Errorf("%s: %v", c.path, got)
		}
	}
}

func TestImageFormatAllowList(t *testing.T) {
	cases := map[string]string{
		"\x89PNG\r\n\x1a\n": "png", "\xff\xd8\xff\xe0": "jpeg", "GIF87a": "gif", "BM": "bmp", "RIFF\x10\x00\x00\x00WEBP": "webp", "II*\x00": "tiff", "MM\x00*": "tiff",
		"<svg xmlns=": "", "icns": "", "\x00\x00\x01\x00": "", "RIFF\x10\x00\x00\x00WAVE": "",
	}
	for magic, want := range cases {
		if got := ImageFormat(imageHead(magic)); got != want {
			t.Errorf("%q: %q, 기대 %q", magic, got, want)
		}
	}
	if ImageFormat(nil) != "" || ImageFormat([]byte("\x89PNG\r\n\x1a\n123")) != "" {
		t.Error("0·11바이트는 빈 값이어야 한다")
	}
}

func TestClassifyRemote(t *testing.T) {
	cases := []struct{ value, domain, url string }{
		{"http://127.0.0.1:8080/a.png", "", ""}, {"http://127.9.9.9/a.png", "", ""}, {"http://[::1]/a.png", "", ""},
		{"http://localhost/a.png", "", ""}, {"http://LOCALHOST./a.png", "", ""}, {"http://x.localhost/a.png", "", ""},
		{"http://0.0.0.0/a.png", "", ""}, {"http://[::]/a.png", "", ""}, {"http://169.254.169.254/latest", "", ""},
		{"http://[fe80::1]/a.png", "", ""}, {"http://2130706433/a.png", "", ""}, {"http://127.1/a.png", "", ""},
		{"http://0x7f.1/a.png", "", ""}, {"http://0177.0.0.1/a.png", "", ""},
		{"http://[::ffff:127.0.0.1]/a.png", "", ""}, {"http://[::127.0.0.1]/a.png", "", ""}, {"http://intra@127.0.0.1/a.png", "", ""},
		{"qrc:/img/a.png", "", ""}, {"file://host/share/a.png", "", ""}, {"file://localhost/C:/w/a.png", "", ""},
		{`\\host\share\a.png`, "", ""}, {`&#92;&#92;host\share\a.png`, "", ""}, {"&#92;&#92;host/share/a.png", "http://intra.example/", ""},
		{"ftp://example.com/a.png", "", ""}, {"cid:image001.png@01D", "", ""}, {"about:blank", "", ""}, {"img/a.png", "", ""},
		{"img/a.png", "http://127.0.0.1/", ""}, {"C:/Users/u/other/a.png", "", ""}, {"", "", ""},
		{"http://10.1.2.3/a.png", "", "http://10.1.2.3/a.png"}, {"http://192.168.0.5/a.png", "", "http://192.168.0.5/a.png"},
		{"http://[fc00::1]/a.png", "", "http://[fc00::1]/a.png"}, {"https://example.com/a.png", "", "https://example.com/a.png"},
		{"https://example.com/a.png?x=1&amp;y=2", "", "https://example.com/a.png?x=1&y=2"},
		{"img/a.png", "http://intra.example/x/", "http://intra.example/x/img/a.png"},
		{"//cdn.example/a.png", "https://intra.example/", "https://cdn.example/a.png"},
	}
	for _, c := range cases {
		r := ClassifyRef(c.value, c.domain, kRoot)
		if c.url == "" {
			if r.Kind != RefDrop {
				t.Errorf("%s: 막혀야 한다 %+v", c.value, r)
			}
		} else if r.Kind != RefRemote || r.URL.String() != c.url {
			t.Errorf("%s: %+v", c.value, r)
		}
	}
}

// 퍼센트 인코딩한 \ 로 적은 UNC·장치 경로와 드라이브 상대 경로는 여는 쪽(os.Open)까지 가지 않는다.
func TestClassifyFileDriveOnly(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 전용")
	}
	for _, v := range []string{
		"file:///%5Chost%5Cs%5Cx.png", "file:///%2F%5Chost%5Cs%5Cx.png", "file:///%5C.%5Cpipe%5Cx", "file:///%5C.%5CUNC%5Chost%5Cs%5Cx.png",
		"file:///%5C%3F%5CUNC%5Chost%5Cs%5Cx.png", "file:///C:x.png", "file:////host/s/x.png", "file:///x.png", "file:C:/x.png",
		"file:///C:/../%5C%5Chost%5Cs%5Cx.png", "file:///C:/a/../../%5C%5Chost%5Cs%5Cx.png", "file:///C:/../x.png",
	} {
		if r := ClassifyRef(v, "", kRoot); r.Kind != RefDrop {
			t.Errorf("%s: 막혀야 한다 %+v", v, r)
		}
	}
	if r := ClassifyRef("file:///C:%5Cw%5Ca.png", "", kRoot); r.Kind != RefLocal {
		t.Errorf("드라이브 경로는 열어야 한다 %+v", r)
	}
}

// file: 주소의 판정은 PC 종류별로 다르다: Windows 는 드라이브 경로만, 그 밖은 절대 경로를 남긴다.
func TestFileURLPathByOS(t *testing.T) {
	for _, c := range []struct {
		v       string
		windows bool
		want    string
		ok      bool
	}{
		{"file:///C:/a/../b.png", true, "C:/b.png", true},
		{"file:///tmp/a.png", true, "", false},
		{"file:////evil/s/../../C:/x.png", true, "", false},
		{"file:///%5C%5Cevil%5Cs/../../C:/x.png", true, "", false},
		{"file:///tmp/a/../b.png", false, "/tmp/b.png", true},
		{"file:a.png", false, "", false},
		{"file://host/a.png", false, "", false},
	} {
		got, ok := fileURLPath(c.v, c.windows)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%s(windows=%v): %q %v", c.v, c.windows, got, ok)
		}
	}
}

func TestResolvedAllowed(t *testing.T) {
	addrs := func(s ...string) []netip.Addr {
		var out []netip.Addr
		for _, a := range s {
			out = append(out, netip.MustParseAddr(a))
		}
		return out
	}
	for want, list := range map[bool][][]netip.Addr{
		true:  {addrs("10.0.0.1"), addrs("203.0.113.5", "fc00::5")},
		false: {addrs("10.0.0.1", "127.0.0.1"), addrs("::1"), nil},
	} {
		for _, l := range list {
			if ResolvedAllowed(l) != want {
				t.Errorf("%v: 기대 %v", l, want)
			}
		}
	}
}

func TestClassifyDataAndFolder(t *testing.T) {
	if ClassifyRef("data:image/png;base64,AAAA", "", kRoot).Kind != RefDataImage ||
		ClassifyRef("DATA:image/JPEG;base64,AAAA", "", kRoot).Kind != RefDataImage ||
		ClassifyRef("data:text/html,<b>x</b>", "", kRoot).Kind != RefDrop ||
		ClassifyRef("data:image/svg+xml,<svg/>", "", kRoot).Kind != RefDrop {
		t.Error("data:")
	}
	if r := ClassifyRef(kRoot+"preload_0.png", "", kRoot); r.Kind != RefLocal || r.Local != kRoot+"preload_0.png" {
		t.Errorf("클립 폴더 안: %+v", r)
	}
	if ClassifyRef(kRoot+"../../fmx.dat", "", kRoot).Kind != RefDrop || ClassifyRef(kRoot+`..\..\fmx.dat`, "", kRoot).Kind != RefDrop {
		t.Error("클립 폴더 밖으로 나간다")
	}
}

func TestSendDropsUnbundled(t *testing.T) {
	var b strings.Builder
	b.WriteString("<html><body><!--StartFragment-->")
	for i := 1; i <= ImgMaxCnt+1; i++ {
		fmt.Fprintf(&b, `<img src="file:///C:/w/%d.png">`, i)
	}
	b.WriteString(`<img src="http://127.0.0.1:8080/b.png"><img src="qrc:/c.png"><img srcset="https://example.com/c.png 2x">` +
		`<div style="background:url(&quot;https://example.com/d.png&quot;)">t</div>` +
		`<style>p{background:url('https://example.com/e.png')}</style><script src="https://example.com/a.js"></script>` +
		`<img src="data:image/png;base64,AAAA"><img src="data:text/html,x"><!--EndFragment--></body></html>`)
	out := PrepareSendHTML(b.String(), "", kRoot, kStamp, ImgMaxCnt)
	if len(out.Refs) != ImgMaxCnt {
		t.Fatalf("싣는 수 %d", len(out.Refs))
	}
	for _, gone := range []string{"C:/w/", "127.0.0.1", "qrc:", "example.com", "data:text/html"} {
		if strings.Contains(out.HTML, gone) {
			t.Errorf("%s 가 남았다", gone)
		}
	}
	if !strings.Contains(out.HTML, "data:image/png;base64,AAAA") || !strings.Contains(out.HTML, placeholder(ImgMaxCnt, ".png")) {
		t.Error("남아야 할 것이 없다")
	}
	d := out.Dropped
	if d[DropOverLimit] != 1 || d[DropBlocked] != 1 || d[DropScheme] != 2 || d[DropNotBundled] != 4 || d[DropNotImage] != 0 {
		t.Errorf("%+v", d)
	}
}

func TestSendSameRefOnce(t *testing.T) {
	out := PrepareSendHTML(`<p>file:///C:/w/a.png</p><img src="file:///C:/w/a.png"><img src='file:///C:/w/a.png'>`+
		`<img src="https://example.com/p/b.webp?w=1">`, "", kRoot, kStamp, ImgMaxCnt)
	if len(out.Refs) != 2 || strings.Count(out.HTML, placeholder(1, ".png")) != 2 || !out.Refs[1].Remote ||
		out.Refs[1].URL.String() != "https://example.com/p/b.webp?w=1" || !strings.Contains(out.HTML, placeholder(2, ".webp")) ||
		!strings.HasPrefix(out.HTML, "<p>file:///C:/w/a.png</p>") {
		t.Errorf("%+v %s", out.Refs, out.HTML)
	}
	out = PrepareSendHTML(`<!-- <x a=" --><img src="http://127.0.0.1/e.png"><!-- " -->`, "", kRoot, kStamp, ImgMaxCnt)
	if strings.Contains(out.HTML, "127.0.0.1") {
		t.Errorf("주석의 따옴표가 태그를 삼켰다: %s", out.HTML)
	}
}

func TestReceiveKeepsOnlyBundle(t *testing.T) {
	out := cleanReceived(`<img src="C:/Users/u/temp/clip/11727000000.png">` +
		`<img src="file:///C:/Users/u/temp/clip/11727000000.png">` +
		`<IMG SRC="http://127.0.0.1:8080/a.png">` +
		`<img src="\\127.0.0.1\dummy\a.png">` +
		`<img src="&#92;&#92;127.0.0.1\dummy\b.png">` +
		`<img src=http://x.example/d.png>` +
		`<img srcset="http://x.example/a.png 2x">` +
		`<div style="background:url('http://x.example/c.png')">t</div>` +
		`<img src="file://127.0.0.1/C:/Users/u/temp/clip/11727000000.png">` +
		`<img src="C:/Users/u/temp/clip/../11727000000.png">`)
	if strings.Count(out.HTML, kBundled) != 2 || strings.Contains(out.HTML, "127.0.0.1") || strings.Contains(out.HTML, "x.example") ||
		!strings.Contains(out.HTML, "url()") || out.Refs != 8 || out.Scripts != 0 {
		t.Errorf("%+v", out)
	}
	out = cleanReceived(`<img src="C:/Users/u/temp/clip/dst.htm"><img src="C:/Users/u/temp/clip/__CLIP__.fmh">` +
		`<img src="C:/Users/u/temp/clip/old.png"><img src="C:/Users/u/temp/clip/11727000000.PNG">`)
	if strings.Contains(out.HTML, "dst.htm") || strings.Contains(out.HTML, "__CLIP__") || strings.Contains(out.HTML, "old.png") || out.Refs != 3 {
		t.Errorf("%+v", out)
	}
}

func TestReceiveRemovesScripts(t *testing.T) {
	out := cleanReceived(`<html><head><base href="http://x.example/">` +
		`<meta http-equiv="Refresh" content="0;url=http://x.example/r">` +
		`<link rel="stylesheet" href="http://x.example/a.css">` +
		`<style>@import "http://x.example/i.css"; p{color:red}</style>` +
		`<script>alert(1)</script></head>` +
		`<body background="http://x.example/bg.png"><!--StartFragment-->` +
		`<SCRIPT type="text/javascript">alert(2)</SCRIPT>` +
		`<p onclick="steal()">본문</p>` +
		`<img src="C:/Users/u/temp/clip/11727000000.png" onerror="steal()" ONLOAD="steal()">` +
		`<a href="http://example.com/page">링크</a>` +
		`<a href="javascript:alert(3)">j</a>` +
		`<a href=" java&#x09;script:alert(4)">k</a>` +
		`<svg><image xlink:href="http://x.example/s.png"/></svg>` +
		`<video poster="http://x.example/p.png"></video>` +
		`<object data="data:text/html,x"></object>` +
		`<img src="data:image/png;base64,AAAA">` +
		`<!--EndFragment--></body></html>`)
	low := strings.ToLower(out.HTML)
	for _, gone := range []string{"<script", "alert", "onclick", "onerror", "onload", "steal", "x.example", "javascript", "<base", "refresh", "data:text/html"} {
		if strings.Contains(low, gone) {
			t.Errorf("%s 가 남았다: %s", gone, out.HTML)
		}
	}
	for _, kept := range []string{`<a href="http://example.com/page">`, "C:/Users/u/temp/clip/11727000000.png", "data:image/png;base64,AAAA", "p{color:red}"} {
		if !strings.Contains(out.HTML, kept) {
			t.Errorf("%s 가 사라졌다", kept)
		}
	}
	if out.Scripts != 5 || out.Refs != 10 {
		t.Errorf("스크립트 %d 바깥 참조 %d", out.Scripts, out.Refs)
	}
	if text := PlainText(out.HTML); !strings.Contains(text, "본문") || !strings.Contains(text, "링크") {
		t.Errorf("평문: %q", text)
	}
}

func TestReceiveOthers(t *testing.T) {
	out := cleanReceived(`<!-- <x a=" --><img src="http://127.0.0.1/e.png"><!-- " --><!--[if gte vml 1]>` +
		`<v:imagedata src="\\127.0.0.1\s\v.png"/><![endif]-->`)
	if strings.Contains(out.HTML, "127.0.0.1") || out.Refs != 2 {
		t.Errorf("%+v", out)
	}
	out = cleanReceived(`<v:imagedata src="C:/Users/u/temp/clip/11727000000.png" o:href="\\127.0.0.1\s\v.png"/>` +
		`<iframe srcdoc="&lt;img src=&quot;http://127.0.0.1/i.png&quot;&gt;"></iframe>`)
	if strings.Contains(out.HTML, "127.0.0.1") || !strings.Contains(out.HTML, kBundled) || out.Refs != 2 {
		t.Errorf("%+v", out)
	}
	html := `<html><body><!--StartFragment--><p class=MsoNormal style='margin:0'>가 <b>나</b>` +
		`<img width=10 src="C:/Users/u/temp/clip/11727000000.png"></p><!--EndFragment--></body></html>`
	if out = cleanReceived(html); out.HTML != html || out.Refs+out.Scripts != 0 {
		t.Errorf("지울 것이 없으면 그대로: %+v", out)
	}
}

func TestAccelerator(t *testing.T) {
	cases := []struct{ mod, ch, want string }{
		{"ALT", "C", "Alt+C"}, {"ALT+SHIFT", "c", "Alt+Shift+C"}, {"CTRL+SHIFT", "V", "Ctrl+Shift+V"}, {"SHIFT", "1", "Shift+1"},
		{"ALT", "", ""}, {"ALT", "가", ""}, {"ALT", "AB", ""}, {"", "C", ""},
	}
	for _, c := range cases {
		if got := Accelerator(c.mod, c.ch); got != c.want {
			t.Errorf("%+v: %q", c, got)
		}
	}
}

func TestAllowClip(t *testing.T) {
	// 4칸 표: 망 × 방향 → 어느 칸을 보는가
	cases := []struct {
		business, up bool
		clipUp, down int32
		want         bool
	}{
		{true, true, 0, 1, true}, {true, true, 1, 0, false},
		{true, false, 1, 0, true}, {true, false, 0, 1, false},
		{false, true, 1, 0, true}, {false, true, 0, 1, false},
		{false, false, 0, 1, true}, {false, false, 1, 0, false},
	}
	for _, c := range cases {
		if got := AllowClip(c.business, c.up, c.clipUp, c.down); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

// ── 묶음 왕복·CF_HTML·그림 ──

func TestBundleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	pngBytes, err := EncodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "a.png")
	if err := os.WriteFile(p, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	html := `<html><body><!--StartFragment--><p>글자</p><img src="file:///` + filepath.ToSlash(p) + `"><img src="file:///C:/none/b.png"><img src="http://127.0.0.1/x.png"><!--EndFragment--></body></html>`
	data, log, err := PackHTMLText(context.Background(), html, "", kStamp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "막는 주소 1") || !strings.Contains(log, "받지 못함 1") {
		t.Errorf("로그: %s", log)
	}
	got, err := ParseBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.HTML, `src="data:image/png;base64,`) || strings.Contains(got.HTML, Placeholder) || strings.Contains(got.HTML, "127.0.0.1") {
		t.Errorf("HTML: %s", got.HTML)
	}
	if got.Text != "글자" {
		t.Errorf("평문: %q", got.Text)
	}
}

func TestParseBundleDropsNonImageEntry(t *testing.T) {
	data, err := BuildBundle([]byte(`<p>x</p><img src="`+Placeholder+`a.png">`), []Entry{{Name: "a.png", Data: []byte("<html><script>x</script></html>")}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.HTML, "a.png") || strings.Contains(got.HTML, "base64") {
		t.Errorf("그림 아닌 항목이 남았다: %s", got.HTML)
	}
	// 다른 OS 가 만든 절대 경로도 묶음 항목으로 본다
	pngBytes, _ := EncodePNG(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	data, _ = BuildBundle([]byte(`<img src="/home/u/clip/a.png">`), []Entry{{Name: "a.png", Data: pngBytes}})
	if got, err = ParseBundle(data); err != nil || !strings.Contains(got.HTML, "data:image/png") {
		t.Errorf("%v %+v", err, got)
	}
	// data:image 의 바이트도 허용 목록으로 본다
	data, _ = BuildBundle([]byte(`<img src="data:image/png;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmciPg==">`), nil)
	if got, err = ParseBundle(data); err != nil || strings.Contains(got.HTML, "base64") {
		t.Errorf("%v %+v", err, got)
	}
}

func TestCFHTML(t *testing.T) {
	html := "<html><body>가나</body></html>"
	b := BuildCFHTML(html)
	got, src := ParseCFHTML(b)
	if !strings.Contains(got, "<!--StartFragment-->가나<!--EndFragment-->") || src != "" {
		t.Fatalf("%q %q", got, src)
	}
	// 머리의 오프셋은 바이트 단위다(한글이 있어도 맞아야 한다)
	head := string(b)
	frag := head[strings.Index(head, fragStart)+len(fragStart) : strings.Index(head, fragEnd)]
	if frag != "가나" {
		t.Fatalf("조각: %q", frag)
	}
	raw := "Version:0.9\r\nStartHTML:-1\r\nEndHTML:-1\r\nStartFragment:0000000200\r\nEndFragment:0000000210\r\nSourceURL:https://a.example/x/\r\n"
	raw += strings.Repeat(" ", 200-len(raw)) + "<b>가</b>"
	if got, src = ParseCFHTML([]byte(raw + "\x00\x00")); got != "<b>가</b>" || src != "https://a.example/x/" {
		t.Fatalf("조각 갈래: %q %q", got, src)
	}
	if got, _ = ParseCFHTML([]byte("Version:0.9\r\n<p>x</p>")); got != "<p>x</p>" {
		t.Fatalf("머리 뒤 전부: %q", got)
	}
	if got := BuildCFHTML("<p>조각</p>"); !bytes.Contains(got, []byte("<html><body><!--StartFragment--><p>조각</p><!--EndFragment--></body></html>")) {
		t.Fatalf("감싸기: %s", got)
	}
}

func TestImageConversions(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{10, 20, 30, 255})
	img.SetNRGBA(1, 1, color.NRGBA{40, 50, 60, 255})
	dib := ImageToDIB(img)
	bmpBytes, err := DIBToBMP(dib)
	if err != nil {
		t.Fatal(err)
	}
	back, f, err := DecodeImage(bmpBytes)
	if err != nil || f != "bmp" {
		t.Fatalf("%v %q", err, f)
	}
	if r, g, b, _ := back.At(0, 0).RGBA(); r>>8 != 10 || g>>8 != 20 || b>>8 != 30 {
		t.Fatalf("왼쪽 위 %d %d %d", r>>8, g>>8, b>>8)
	}
	// 40바이트 머리 + BI_BITFIELDS 마스크(브라우저가 주는 모양)
	le := binary.LittleEndian
	bf := append(append([]byte{}, dib[:40]...), make([]byte, 12)...)
	le.PutUint32(bf[16:], 3)
	le.PutUint32(bf[40:], 0xff0000)
	le.PutUint32(bf[44:], 0xff00)
	le.PutUint32(bf[48:], 0xff)
	bf = append(bf, dib[40:]...)
	if bmpBytes, err = DIBToBMP(bf); err != nil {
		t.Fatal(err)
	}
	if _, _, err := DecodeImage(bmpBytes); err != nil {
		t.Fatalf("BI_BITFIELDS: %v", err)
	}
	// 8비트 색표: 픽셀 시작 = 14 + 40 + 색 × 4
	pal := make([]byte, 40+4*4+4)
	le.PutUint32(pal, 40)
	le.PutUint32(pal[4:], 1)
	le.PutUint32(pal[8:], 1)
	le.PutUint16(pal[12:], 1)
	le.PutUint16(pal[14:], 8)
	le.PutUint32(pal[32:], 4)
	if bmpBytes, err = DIBToBMP(pal); err != nil || le.Uint32(bmpBytes[10:]) != 14+40+16 {
		t.Fatalf("%v", err)
	}
	if _, err := DIBToBMP(make([]byte, 10)); err == nil {
		t.Fatal("짧은 DIB")
	}
	// 잘린 PNG 는 앞머리가 맞아도 열지 못한다
	pngBytes, _ := EncodePNG(img)
	if _, f, err := DecodeImage(pngBytes[:20]); err == nil || f != "png" {
		t.Fatalf("잘린 PNG: %v %q", err, f)
	}
}

// 머리에 큰 크기만 적은 작은 파일은 풀기 전에 거절한다.
func TestDecodeImageRejectsHugeHeader(t *testing.T) {
	ihdr := []byte("IHDR\x00\x00\x4e\x20\x00\x00\x4e\x20\x08\x06\x00\x00\x00") // 20000x20000 RGBA
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d")
	b.Write(ihdr)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(ihdr))
	if _, f, err := DecodeImage(b.Bytes()); err == nil || f != "png" || !strings.Contains(err.Error(), "너무 크다") {
		t.Fatalf("%v %q", err, f)
	}
}
