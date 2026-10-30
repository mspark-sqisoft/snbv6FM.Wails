package clip

// Windows 의 "HTML Format"(CF_HTML) 머리 읽기·만들기와, 받은 서식의 평문 추출.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	nhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ParseCFHTML 은 CF_HTML 바이트에서 HTML 과 SourceURL 을 꺼낸다. 오프셋은 바이트 단위다.
// StartHTML~EndHTML 이 맞지 않으면 StartFragment~EndFragment, 그것도 없으면 머리 뒤 전부.
func ParseCFHTML(b []byte) (html, sourceURL string) {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	s := string(b)
	vals := map[string]string{}
	pos := 0
	for pos < len(s) {
		end := strings.IndexByte(s[pos:], '\n')
		if end < 0 {
			break
		}
		line := strings.TrimRight(s[pos:pos+end], "\r")
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, "<") {
			break
		}
		vals[strings.ToLower(k)] = v
		pos += end + 1
	}
	span := func(from, to string) (string, bool) {
		a, err1 := strconv.Atoi(strings.TrimSpace(vals[from]))
		z, err2 := strconv.Atoi(strings.TrimSpace(vals[to]))
		if err1 != nil || err2 != nil || a < 0 || a >= z || z > len(s) {
			return "", false
		}
		return s[a:z], true
	}
	html, ok := span("starthtml", "endhtml")
	if !ok {
		if html, ok = span("startfragment", "endfragment"); !ok {
			html = s[pos:]
		}
	}
	return html, strings.TrimSpace(vals["sourceurl"])
}

const (
	fragStart = "<!--StartFragment-->"
	fragEnd   = "<!--EndFragment-->"
)

// BuildCFHTML 은 HTML 에 CF_HTML 머리를 붙인다. 조각 표식이 없으면 <body> 안에 넣는다.
func BuildCFHTML(html string) []byte {
	low := asciiLower(html)
	if !strings.Contains(low, asciiLower(fragStart)) || !strings.Contains(low, asciiLower(fragEnd)) {
		html = markFragment(html, low)
		low = asciiLower(html)
	}
	const head = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	n := len(fmt.Sprintf(head, 0, 0, 0, 0))
	a := strings.Index(low, asciiLower(fragStart)) + len(fragStart)
	z := strings.Index(low, asciiLower(fragEnd))
	return []byte(fmt.Sprintf(head, n, n+len(html), n+a, n+z) + html)
}

func markFragment(html, low string) string {
	body := strings.Index(low, "<body")
	bodyEnd := strings.LastIndex(low, "</body")
	if body < 0 || bodyEnd < 0 {
		return "<html><body>" + fragStart + html + fragEnd + "</body></html>"
	}
	open := strings.IndexByte(html[body:], '>')
	if open < 0 || body+open+1 > bodyEnd {
		return "<html><body>" + fragStart + html + fragEnd + "</body></html>"
	}
	a := body + open + 1
	return html[:a] + fragStart + html[a:bodyEnd] + fragEnd + html[bodyEnd:]
}

// PlainText 는 받은 서식의 평문이다. 평문을 함께 올리지 않으면 메모장·웹 입력칸에 아무것도 붙지 않는다.
// head·style·script 글자는 빼고, 그림 자리 글자(U+FFFC)는 평문 편집기에 네모로 보이므로 뺀다.
func PlainText(html string) string {
	z := nhtml.NewTokenizer(strings.NewReader(html))
	var b []byte
	skip, pre := 0, 0
	space := false // 줄 앞이거나 공백을 낸 뒤
	nl := func() {
		for len(b) > 0 && b[len(b)-1] == ' ' {
			b = b[:len(b)-1]
		}
		if len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		space = true
	}
	for {
		tt := z.Next()
		switch tt {
		case nhtml.ErrorToken:
			return strings.TrimRight(string(b), " \r\n\t")
		case nhtml.TextToken:
			if skip > 0 {
				continue
			}
			text := strings.NewReplacer(" ", " ", "￼", "").Replace(string(z.Text()))
			if pre > 0 {
				b = append(b, text...)
				space = false
				continue
			}
			for _, r := range text {
				if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
					if !space {
						b = append(b, ' ')
						space = true
					}
					continue
				}
				b = utf8.AppendRune(b, r)
				space = false
			}
		case nhtml.StartTagToken, nhtml.EndTagToken, nhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				skip = 0 // </head> 없이 끝난 머리
			case atom.Head, atom.Style, atom.Script, atom.Title, atom.Noscript, atom.Xmp:
				if tt == nhtml.StartTagToken {
					skip++
				} else if tt == nhtml.EndTagToken && skip > 0 {
					skip--
				}
			case atom.Pre:
				if tt == nhtml.StartTagToken {
					pre++
				} else if tt == nhtml.EndTagToken && pre > 0 {
					pre--
				}
				nl()
			case atom.Br, atom.P, atom.Div, atom.Li, atom.Tr, atom.Table, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
				atom.Ul, atom.Ol, atom.Blockquote, atom.Hr, atom.Section, atom.Article:
				nl()
			case atom.Td, atom.Th:
				if tt == nhtml.EndTagToken {
					b = append(b, '\t')
					space = true
				}
			}
		}
	}
}
