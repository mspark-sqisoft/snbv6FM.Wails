// Package clip 은 클립보드 영역의 순수 처리와 OS 클립보드 입출력이다(docs/port/07).
//
// refs.go 는 서식(HTML) 클립보드 안의 참조 판정이다(원본 src/clipboard/ClipRefs.h).
// 보내는 쪽은 그림인 로컬 파일과 http/https 그림만 묶음(.fmh)에 싣고, 받는 쪽은 묶음 안 그림을 가리키는 참조만 남긴다.
// HTML 은 다시 쓰지 않고 태그 안 속성의 바이트 위치만 고친다 ― 문서를 다시 만들면 한워드 서식이 바뀐다.
package clip

import (
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// RefKind 는 참조 하나의 갈래다.
type RefKind int

const (
	RefDrop      RefKind = iota // 열지도 요청하지도 않는다(Why)
	RefLocal                    // 이 PC 파일(Local)
	RefRemote                   // 요청할 http/https 주소(URL)
	RefDataImage                // data:image/<그림 형식> ― 바깥에 닿지 않아 그대로 둔다
)

// Drop 은 보내는 쪽이 뺀 참조의 사유다. 로그가 사유별로 센다(주소·경로는 남기지 않는다).
type Drop int

const (
	DropNotImage   Drop = iota // 그림 확장자가 아니거나 앞머리가 그림이 아니다
	DropBlocked                // 막는 주소(루프백·미지정·링크로컬, 그리 풀리는 이름)
	DropScheme                 // http/https·호스트 없는 file: 가 아닌 것
	DropOverLimit              // 묶는 그림 수 상한을 넘었다
	DropNotBundled             // 묶지 않는 자리(srcset·CSS url()·<script src>)
	DropNotFetched             // 받지 못했다
	dropCount
)

// DropCounts 는 사유별 개수다.
type DropCounts [dropCount]int

// Ref 는 참조 판정 결과다.
type Ref struct {
	Kind  RefKind
	Local string
	URL   *url.URL
	Why   Drop
}

// SendRef 는 보내는 HTML 에서 자리표시 이름으로 바꾼 그림 하나다.
type SendRef struct {
	Remote bool
	Local  string   // Remote 가 아니면 읽을 파일
	URL    *url.URL // Remote 면 받을 주소
	Name   string   // 묶음 항목 이름(<번호><시각><.확장자>)
}

// SendHTML 은 PrepareSendHTML 의 결과다.
type SendHTML struct {
	HTML    string
	Refs    []SendRef
	Dropped DropCounts
}

// CleanHTML 은 CleanReceivedHTML 의 결과다.
type CleanHTML struct {
	HTML    string
	Refs    int // 지운 바깥 참조
	Scripts int // 지운 <script> 요소와 이벤트(on…) 속성
}

var (
	entityRe = regexp.MustCompile(`&(?:#[xX]([0-9A-Fa-f]+);?|#([0-9]+);?|([A-Za-z]+);)`)
	// 표준 라이브러리 html.UnescapeString 은 이름 표가 훨씬 넓어 판정이 달라진다 ― 주소에 쓰일 만한 이름만 푼다
	entityNames = map[string]string{
		"amp": "&", "AMP": "&", "lt": "<", "LT": "<", "gt": ">", "GT": ">", "quot": `"`, "QUOT": `"`, "apos": "'",
		"colon": ":", "sol": "/", "bsol": `\`, "period": ".", "lpar": "(", "rpar": ")", "Tab": "\t", "NewLine": "\n", "nbsp": "\u00A0",
	}
	dataImageRe = regexp.MustCompile(`(?i)^data:image/(png|jpeg|jpg|gif|bmp|webp|tiff)[;,]`)
	schemeRe    = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*):`)
	importRe    = regexp.MustCompile(`(?i)@import\s*(?:url\(\s*(?:"[^"]*"|'[^']*'|[^)]*)\s*\)|"[^"]*"|'[^']*')[^;]*;?`)
	cssURLRe    = regexp.MustCompile(`(?i)url\(\s*("[^"]*"|'[^']*'|[^)]*?)\s*\)`)
	refWSRe     = regexp.MustCompile(`[\t\n\r]`)
	controlsRe  = regexp.MustCompile(`[\x00-\x20]`)
	driveURLRe  = regexp.MustCompile(`^/[A-Za-z]:[/\\]`)
)

// DecodeEntities 는 HTML 문자 참조를 푼다. 붙여 넣는 프로그램은 &#92;&#92;host 를 \\host 로 읽으므로 판정은 푼 값으로 한다.
func DecodeEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range entityRe.FindAllStringSubmatchIndex(s, -1) {
		var rep string
		switch {
		case m[2] >= 0:
			rep = codePoint(s[m[2]:m[3]], 16)
		case m[4] >= 0:
			rep = codePoint(s[m[4]:m[5]], 10)
		default:
			v, ok := entityNames[s[m[6]:m[7]]]
			if !ok {
				continue // 모르는 이름은 그대로 둔다
			}
			rep = v
		}
		b.WriteString(s[last:m[0]])
		b.WriteString(rep)
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

func codePoint(digits string, base int) string {
	cp, err := strconv.ParseUint(digits, base, 32)
	if err != nil || cp == 0 || cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF) {
		return "\uFFFD"
	}
	return string(rune(cp))
}

// IsImageSuffix 는 받는 쪽이 그림으로 보는 확장자인가다(점 없이). 이름 끝의 확장자만 본다 ― a.png.exe 는 그림이 아니다.
func IsImageSuffix(suffix string) bool {
	switch strings.ToLower(suffix) {
	case "jpg", "jpeg", "jpe", "jfif", "png", "gif", "bmp", "dib", "webp", "tif", "tiff":
		return true
	}
	return false
}

func suffixOf(p string) string { return strings.TrimPrefix(path.Ext(p), ".") }

// LocalImageOK 는 로컬 파일을 묶음에 실어도 되는가다 ― 그림 확장자이고 앞머리가 그림이다.
func LocalImageOK(p string, head []byte) bool {
	return IsImageSuffix(suffixOf(strings.ReplaceAll(p, `\`, "/"))) && LooksLikeImage(head)
}

// AddressBlocked 는 이 PC 자신이나 사내 웹 그림 자리가 아닌 주소인가다(루프백·미지정·링크로컬). 사설 대역·ULA·공인은 허용한다.
func AddressBlocked(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	if a.Is6() {
		// v4 호환(::a.b.c.d) ― ::(미지정)·::1(루프백)은 그대로 v6 으로 본다
		b := a.As16()
		tail := uint32(b[12])<<24 | uint32(b[13])<<16 | uint32(b[14])<<8 | uint32(b[15])
		if tail > 1 && [12]byte(b[:12]) == [12]byte{} {
			a = netip.AddrFrom4([4]byte(b[12:]))
		}
	}
	if a.Is4() {
		b := a.As4()
		return b[0] == 127 || b[0] == 0 || (b[0] == 169 && b[1] == 254)
	}
	return a.IsLoopback() || a.IsUnspecified() || a.IsLinkLocalUnicast()
}

// ResolvedAllowed 는 이름을 풀어 나온 주소가 모두 허용인가다. 못 풀었으면(빈 목록) 요청하지 않는다.
func ResolvedAllowed(addrs []netip.Addr) bool {
	return len(addrs) > 0 && !slices.ContainsFunc(addrs, AddressBlocked)
}

// parseHostIP 는 호스트 글자를 주소로 읽는다. QUrl 과 달리 net/url 은 2130706433·127.1·0x7f.1 을 풀어 주지 않아 직접 푼다.
func parseHostIP(host string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(host); err == nil {
		return a, true
	}
	parts := strings.Split(host, ".")
	if len(parts) > 4 {
		return netip.Addr{}, false
	}
	var ip [4]byte
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 0, 32) // 0x·0 접두 허용
		if err != nil {
			return netip.Addr{}, false
		}
		if i == len(parts)-1 { // 마지막 조각이 남은 바이트를 모두 채운다
			room := 4 - i
			if room < 4 && n >= 1<<(8*room) {
				return netip.Addr{}, false
			}
			for j := 3; j >= i; j-- {
				ip[j] = byte(n)
				n >>= 8
			}
		} else {
			if n > 255 {
				return netip.Addr{}, false
			}
			ip[i] = byte(n)
		}
	}
	return netip.AddrFrom4(ip), true
}

// RemoteRef 는 http/https 주소를 요청해도 되는가다(이름 풀기 전에 볼 수 있는 것). 리디렉션 대상도 이것을 다시 거친다.
func RemoteRef(u *url.URL) Ref {
	drop := Ref{Why: DropScheme}
	if u == nil {
		return drop
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Hostname() == "" {
		return drop
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return Ref{Why: DropBlocked}
	}
	if a, ok := parseHostIP(host); ok && AddressBlocked(a) {
		return Ref{Why: DropBlocked}
	}
	return Ref{Kind: RefRemote, URL: u}
}

// IsDataImage 는 data:image/<허용 형식> 값인가다.
func IsDataImage(v string) bool { return dataImageRe.MatchString(strings.TrimSpace(v)) }

func samePathCase(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// inFolder 는 정리한 경로가 클립 폴더 안인가다.
func inFolder(clean, root string) bool {
	r := path.Clean(root) + "/"
	return len(clean) > len(r) && samePathCase(clean[:len(r)], r)
}

// refValue 는 판정할 값이다 ― 문자 참조를 풀고 주소 안의 탭·줄바꿈을 뗀다(브라우저도 뗀다).
func refValue(raw string) string {
	return strings.TrimSpace(refWSRe.ReplaceAllString(DecodeEntities(raw), ""))
}

// fileURLPath 는 호스트 없는 file: 주소가 가리키는 이 PC 의 보통 경로다. 호스트가 있거나 보통 경로가 아니면 거짓.
// Windows 는 UNC·장치·NT 앞머리를 하나씩 막지 않고 드라이브 경로만 남긴다(원본 plainFilePath) ― file:///%5Chost%5Cs 는 url 이 %5C 를 풀고
// path.Clean 이 \ 를 건드리지 않아 그대로 두면 os.Open 이 UNC 로 연다. 정리하기 전과 뒤를 모두 판정한다 ― 정리가 file:///C:/../%5C%5Chost 의
// 드라이브를 지우고, file:////host/s/../../C:/x 에는 드라이브를 만든다. 뒤의 값은 정리한 경로로는 안전해도 CSS 에 원문이 남아 UNC 로 읽힌다.
func fileURLPath(v string, windows bool) (string, bool) {
	u, err := url.Parse(v)
	if err != nil || u.Host != "" {
		return "", false
	}
	clean := path.Clean(u.Path)
	if driveURLRe.MatchString(u.Path) && driveURLRe.MatchString(clean) {
		return clean[1:], true
	}
	return clean, !windows && strings.HasPrefix(u.Path, "/")
}

// ClassifyRef 는 보내는 쪽 그림 참조 하나를 나눈다. domain 은 복사한 페이지 주소(SourceURL).
func ClassifyRef(raw, domain, clipRoot string) Ref {
	drop := Ref{Why: DropScheme}
	v := refValue(raw)
	if v == "" || strings.HasPrefix(v, `\`) { // UNC(\\host\share)
		return drop
	}
	if IsDataImage(v) {
		return Ref{Kind: RefDataImage}
	}
	v = strings.ReplaceAll(v, `\`, "/")
	m := schemeRe.FindStringSubmatch(v)
	if m != nil && len(m[1]) > 1 { // 한 글자는 드라이브 문자다
		switch strings.ToLower(m[1]) {
		case "file":
			// 호스트를 단 file://host/…·file://localhost/… 는 열지 않는다
			p, ok := fileURLPath(v, runtime.GOOS == "windows")
			if !ok {
				return drop
			}
			return Ref{Kind: RefLocal, Local: path.Clean(p)}
		case "http", "https":
			u, err := url.Parse(v)
			if err != nil {
				return drop
			}
			return RemoteRef(u)
		}
		return drop // qrc:·ftp:·cid:·about:·data:(그림 아님) …
	}
	// 스킴 없는 값 ― 클립 폴더 안의 경로이거나 복사한 페이지 기준의 상대 주소
	clean := path.Clean(v)
	if inFolder(clean, clipRoot) {
		return Ref{Kind: RefLocal, Local: clean}
	}
	if m != nil {
		return drop // 클립 폴더 밖의 드라이브 경로(C:/…)
	}
	base, err := url.Parse(strings.TrimSpace(domain))
	if err != nil || base.Hostname() == "" {
		return drop
	}
	if s := strings.ToLower(base.Scheme); s != "http" && s != "https" {
		return drop
	}
	rel, err := url.Parse(v)
	if err != nil {
		return drop
	}
	return RemoteRef(base.ResolveReference(rel))
}

// ── 태그 훑기 ──

type attr struct {
	begin, end int
	name       string // 소문자
	raw        string // 따옴표를 뗀 값(문자 참조를 풀기 전)
}

type tag struct {
	name                   string // 소문자
	attrs                  []attr
	begin, end             int // 여는 태그 [begin, end)
	contentEnd, elementEnd int // 글자만 담는 요소(style·script …)의 내용 끝·닫는 태그 끝, 없으면 -1
}

type edit struct {
	begin, end int
	text       string
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func indexFrom(s, sub string, from int) int {
	if from > len(s) {
		return -1
	}
	i := strings.Index(s[from:], sub)
	if i < 0 {
		return -1
	}
	return i + from
}

func isRawText(name string) bool {
	switch name {
	case "script", "style", "textarea", "title", "xmp":
		return true
	}
	return false
}

// scanTags 는 html 의 여는 태그마다 fn 을 부른다. 브라우저 토큰화를 흉내 낸다 ― 따옴표 안의 > 는 태그를 끝내지 않고,
// 주석은 --> 에서 끝나 그 안의 따옴표가 뒤 태그를 삼키지 못한다. 주석 안도 훑는다(조건부 주석은 Word 가 읽는다).
func scanTags(html string, fn func(tag)) {
	scanRange(html, asciiLower(html), 0, len(html), fn)
}

func scanRange(s, lower string, from, to int, fn func(tag)) {
	i := from
	for i < to {
		if s[i] != '<' {
			i++
			continue
		}
		if strings.HasPrefix(s[i:to], "<!--") {
			end := indexFrom(s, "-->", i+2) // <!--> 도 주석이다
			if end < 0 || end >= to {
				end = to
			}
			if end > i+4 {
				scanRange(s, lower, i+4, end, fn)
			}
			i = min(end+3, to)
			continue
		}
		var next byte
		if i+1 < to {
			next = s[i+1]
		}
		if !isLetter(next) {
			if next == '!' || next == '?' || next == '/' { // <!DOCTYPE>·<![endif]>·<?xml?>·닫는 태그
				gt := indexFrom(s, ">", i)
				if gt < 0 || gt >= to {
					i = to
				} else {
					i = gt + 1
				}
			} else {
				i++
			}
			continue
		}
		t := readTag(s, i, to)
		if isRawText(t.name) {
			closeAt := indexFrom(lower, "</"+t.name, t.end)
			if closeAt < 0 || closeAt >= to {
				t.contentEnd, t.elementEnd = to, to
			} else {
				t.contentEnd = closeAt
				if gt := indexFrom(s, ">", closeAt); gt < 0 || gt >= to {
					t.elementEnd = to
				} else {
					t.elementEnd = gt + 1
				}
			}
		}
		fn(t)
		if t.elementEnd >= 0 {
			i = t.elementEnd
		} else {
			i = t.end
		}
	}
}

func readTag(s string, i, to int) tag {
	t := tag{begin: i, contentEnd: -1, elementEnd: -1}
	p := i + 1
	for p < to && !isSpace(s[p]) && s[p] != '/' && s[p] != '>' {
		p++
	}
	t.name = asciiLower(s[i+1 : p])
	for {
		for p < to && (isSpace(s[p]) || s[p] == '/') {
			p++
		}
		if p >= to {
			t.end = to
			return t
		}
		if s[p] == '>' {
			t.end = p + 1
			return t
		}
		a := attr{begin: p}
		q := p + 1 // 첫 글자는 '=' 여도 이름이다
		for q < to && !isSpace(s[q]) && s[q] != '/' && s[q] != '>' && s[q] != '=' {
			q++
		}
		a.name = asciiLower(s[p:q])
		a.end = q
		r := q
		for r < to && isSpace(s[r]) {
			r++
		}
		if r < to && s[r] == '=' {
			r++
			for r < to && isSpace(s[r]) {
				r++
			}
			if r < to && (s[r] == '"' || s[r] == '\'') {
				e := indexFrom(s, string(s[r]), r+1)
				if e < 0 || e >= to {
					e = to
				}
				a.raw = s[r+1 : e]
				a.end = min(e+1, to)
			} else {
				e := r
				for e < to && !isSpace(s[e]) && s[e] != '>' {
					e++
				}
				a.raw = s[r:e]
				a.end = e
			}
		}
		t.attrs = append(t.attrs, a)
		p = a.end
	}
}

func applyEdits(s string, edits []edit) string {
	if len(edits) == 0 {
		return s
	}
	slices.SortFunc(edits, func(a, b edit) int { return a.begin - b.begin })
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, e := range edits {
		b.WriteString(s[last:e.begin])
		b.WriteString(e.text)
		last = e.end
	}
	b.WriteString(s[last:])
	return b.String()
}

// cleanCSS 는 CSS 안의 @import 문장과 keep 이 받지 않는 url(…) 을 지운다. 지운 수도 돌려준다.
func cleanCSS(css string, keep func(string) bool) (string, int) {
	removed := len(importRe.FindAllStringIndex(css, -1))
	out := importRe.ReplaceAllLiteralString(css, "")
	var edits []edit
	for _, m := range cssURLRe.FindAllStringSubmatchIndex(out, -1) {
		v := out[m[2]:m[3]]
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
			v = v[1 : len(v)-1]
		}
		if keep(v) {
			continue
		}
		edits = append(edits, edit{m[0], m[1], "url()"})
		removed++
	}
	return applyEdits(out, edits), removed
}

// cleanStyleAttr 는 style 속성(문자 참조를 푼 값으로 CSS 를 읽는다)을 고쳐야 하면 그 편집을 더하고 지운 수를 돌려준다.
func cleanStyleAttr(a attr, keep func(string) bool, edits *[]edit) int {
	v := DecodeEntities(a.raw)
	low := asciiLower(v)
	if !strings.Contains(low, "url(") && !strings.Contains(low, "@import") {
		return 0
	}
	clean, n := cleanCSS(v, keep)
	if n == 0 {
		return 0
	}
	clean = strings.NewReplacer("&", "&amp;", `"`, "&quot;").Replace(clean)
	*edits = append(*edits, edit{a.begin, a.end, a.name + `="` + clean + `"`})
	return n
}

// PrepareSendHTML 은 보내는 HTML 을 고친다. 실을 그림 참조는 자리표시 이름으로, 실지 않을 그림 참조는 속성째 지운다.
// 파일 앞머리·이름 풀기·받은 바이트는 부른 쪽이 본다 ― 거기서 빠진 그림도 HTML 에는 자리표시 이름만 남아 원래 주소가 실려 가지 않는다.
func PrepareSendHTML(html, domain, clipRoot string, stamp int64, maxRefs int) SendHTML {
	var out SendHTML
	nameOf := map[string]string{} // 같은 그림을 여러 번 가리키면 한 번만 싣는다
	var edits []edit
	keepCSS := func(v string) bool { return IsDataImage(refValue(v)) }
	notBundled := 0
	scanTags(html, func(t tag) {
		for _, a := range t.attrs {
			if a.name == "srcset" || (a.name == "src" && t.name == "script") {
				edits = append(edits, edit{a.begin, a.end, ""})
				notBundled++
				continue
			}
			if a.name == "style" {
				notBundled += cleanStyleAttr(a, keepCSS, &edits)
				continue
			}
			if a.name != "src" {
				continue
			}
			r := ClassifyRef(a.raw, domain, clipRoot)
			if r.Kind == RefDataImage {
				continue
			}
			if r.Kind == RefDrop {
				edits = append(edits, edit{a.begin, a.end, ""})
				out.Dropped[r.Why]++
				continue
			}
			remote := r.Kind == RefRemote
			key, src := r.Local, r.Local
			if remote {
				key, src = r.URL.String(), r.URL.Path
			}
			name := nameOf[key]
			if name == "" {
				if len(out.Refs) >= maxRefs {
					edits = append(edits, edit{a.begin, a.end, ""})
					out.Dropped[DropOverLimit]++
					continue
				}
				name = strconv.Itoa(len(out.Refs)+1) + strconv.FormatInt(stamp, 10)
				if suffix := suffixOf(src); IsImageSuffix(suffix) {
					name += "." + suffix
				}
				nameOf[key] = name
				out.Refs = append(out.Refs, SendRef{Remote: remote, Local: r.Local, URL: r.URL, Name: name})
			}
			// 받는 쪽이 자리표시를 공백이 든 폴더 경로로 바꾸므로 따옴표로 감싼다
			edits = append(edits, edit{a.begin, a.end, `src="` + Placeholder + name + `"`})
		}
		if t.name == "style" && t.contentEnd > t.end {
			clean, n := cleanCSS(html[t.end:t.contentEnd], keepCSS)
			if n > 0 {
				edits = append(edits, edit{t.end, t.contentEnd, clean})
			}
			notBundled += n
		}
	})
	out.Dropped[DropNotBundled] += notBundled
	out.HTML = applyEdits(html, edits)
	return out
}

// IsBundleRef 는 받은 HTML 의 참조가 클립 폴더의 묶음 항목을 가리키는가다(날 경로·file:/// 모양 모두).
func IsBundleRef(raw, clipRoot string, names []string) bool {
	_, ok := bundleName(raw, clipRoot, names)
	return ok
}

func bundleName(raw, clipRoot string, names []string) (string, bool) {
	v := refValue(raw)
	if strings.HasPrefix(v, `\`) {
		return "", false
	}
	v = strings.ReplaceAll(v, `\`, "/")
	if len(v) >= 5 && strings.EqualFold(v[:5], "file:") {
		p, ok := fileURLPath(v, runtime.GOOS == "windows")
		if !ok {
			return "", false
		}
		v = p
	}
	clean := path.Clean(v)
	if !inFolder(clean, clipRoot) {
		return "", false
	}
	name := clean[len(path.Clean(clipRoot))+1:]
	for _, n := range names {
		if samePathCase(n, name) {
			return n, true
		}
	}
	return "", false
}

var refAttrs = []string{"src", "background", "poster", "data", "href", "xlink:href", "o:href", "lowsrc", "dynsrc"} // o:href 는 VML 그림의 원본 링크

// CleanReceivedHTML 은 받은 HTML 을 정리한다. 묶음 항목·data:image 를 가리키는 참조와 링크만 남기고, 바깥을 가리키는 참조·
// <script> 요소·이벤트(on…) 속성·<base>·새로 고침 <meta> 는 지운다. 지울 것이 없으면 받은 글자 그대로 돌려준다.
func CleanReceivedHTML(html, clipRoot string, names []string) CleanHTML {
	var out CleanHTML
	var edits []edit
	keep := func(v string) bool { return IsDataImage(refValue(v)) || IsBundleRef(v, clipRoot, names) }
	scanTags(html, func(t tag) {
		if t.name == "script" {
			edits = append(edits, edit{t.begin, t.elementEnd, ""})
			out.Scripts++
			return
		}
		attrValue := func(name string) string {
			for _, a := range t.attrs {
				if a.name == name {
					return refValue(a.raw)
				}
			}
			return ""
		}
		if t.name == "base" || (t.name == "meta" && strings.EqualFold(attrValue("http-equiv"), "refresh")) {
			edits = append(edits, edit{t.begin, t.end, ""})
			out.Refs++
			return
		}
		for _, a := range t.attrs {
			switch {
			case strings.HasPrefix(a.name, "on"):
				edits = append(edits, edit{a.begin, a.end, ""})
				out.Scripts++
			case a.name == "style":
				out.Refs += cleanStyleAttr(a, keep, &edits)
			case a.name == "href" && (t.name == "a" || t.name == "area"):
				// 링크는 사용자가 눌러야 열린다 ― 남긴다. 누르면 스크립트가 도는 값과 그림 아닌 data: 만 지운다
				v := controlsRe.ReplaceAllString(asciiLower(refValue(a.raw)), "")
				if strings.HasPrefix(v, "javascript:") || strings.HasPrefix(v, "vbscript:") ||
					(strings.HasPrefix(v, "data:") && !IsDataImage(v)) {
					edits = append(edits, edit{a.begin, a.end, ""})
					out.Refs++
				}
			case a.name == "srcset" || a.name == "srcdoc" || // srcdoc 은 iframe 이 그리는 문서 자체다 ― 안까지 정리하지 않고 지운다
				(slices.Contains(refAttrs, a.name) && !keep(a.raw)):
				edits = append(edits, edit{a.begin, a.end, ""})
				out.Refs++
			}
		}
		if t.name == "style" && t.contentEnd > t.end {
			clean, n := cleanCSS(html[t.end:t.contentEnd], keep)
			if n > 0 {
				edits = append(edits, edit{t.end, t.contentEnd, clean})
			}
			out.Refs += n
		}
	})
	out.HTML = applyEdits(html, edits)
	return out
}
