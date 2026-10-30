package clip

// 클립보드 묶음(.fmh): 원본 stFmClipHeader 를 메모리 그대로 앞에 붙이고 항목 바이트를 0번부터 이어 붙인 파일이다.
//
//	오프셋 0      int32 nCnt             항목 수(1~51)
//	오프셋 4      int32 nSize[51]        항목별 바이트 수
//	오프셋 208    byte  szName[51][256]  항목 이름(NUL 로 끝남, UTF-8)
//	오프셋 13264  항목 바이트(크기 0 인 항목은 없음)
//
// 상대망 사용자가 만들 수 있으므로 앞머리를 다 본 뒤에만 항목을 꺼낸다(원본 ClipFmh.h, C-003·C-011).
// Wails 판은 항목을 파일로 풀지 않고 메모리에서 HTML 의 data URI 로 넣는다 ― 받은 그림이 PC 에 남지 않는다.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// ImgMaxCnt 는 묶음에 싣는 그림 수 상한이다.
	ImgMaxCnt = 50
	// Placeholder 는 보내는 HTML 의 그림 자리표시다(받는 쪽이 자기 경로로 바꾼다. Qt 판과 같은 글자).
	Placeholder = "MOSFM_CLIP_FOLDER"
	// HeaderSize 는 묶음 앞머리 크기다.
	HeaderSize = 4 + (ImgMaxCnt+1)*4 + (ImgMaxCnt+1)*maxName

	maxName  = 256
	htmlDst  = "dst.htm"
	htmlSrc  = "src.htm"
	clipFm   = "__CLIP__.fmh"
	nameBase = 4 + (ImgMaxCnt+1)*4
	// virtualRoot 는 묶음 항목을 가리키는 가상 폴더다. 정리 판정에만 쓰고 파일을 만들지 않는다.
	virtualRoot = "C:/__snb_clip__/"
)

var deviceNameRe = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com\d|lpt\d)(\..*)?$`)

// EntryName 은 이름 칸 하나에서 파일명만 꺼낸다. 쓸 수 없으면 "". 보내는 쪽은 늘 NUL 로 끝내므로 NUL 없는 칸은 받지 않는다.
func EntryName(raw []byte) string {
	n := bytes.IndexByte(raw, 0)
	if n < 0 || !utf8.Valid(raw[:n]) { // 잘못된 UTF-8 은 바꾸지 않고 거부한다 ― 경로가 달라지는 일을 막는다
		return ""
	}
	name := strings.ReplaceAll(string(raw[:n]), `\`, "/")
	name = name[strings.LastIndexByte(name, '/')+1:]
	// 경로 조각·드라이브 문자를 버린다. "C:x" 는 Windows 에서 드라이브 상대 경로다
	if len(name) >= 2 && name[1] == ':' && isLetter(name[0]) {
		name = name[2:]
	}
	name = strings.TrimSpace(name)
	// Windows 는 열 때 이름 끝의 점·공백을 떼므로 "dst.htm." 이 dst.htm 을 연다 ― 비교 전에 똑같이 뗀다
	name = strings.TrimRight(name, ". ")
	if name == "" || strings.Contains(name, ":") || deviceNameRe.MatchString(name) {
		return ""
	}
	return name
}

type fmhHeader struct {
	cnt   int
	sizes [ImgMaxCnt + 1]int32
	names [ImgMaxCnt + 1]string
}

// headerProblem 은 앞머리를 읽고 검사한다. 거부하면 사유(로그용), 받으면 "".
// 0번은 보내는 쪽이 늘 채우는 dst.htm 이어야 하고, 1번부터 크기가 0 이하인 항목은 건너뛰므로 이름을 보지 않는다.
func headerProblem(head []byte, fileSize int64) (fmhHeader, string) {
	var h fmhHeader
	if len(head) < HeaderSize {
		return h, fmt.Sprintf("헤더보다 짧다(%d / %d 바이트)", len(head), HeaderSize)
	}
	cnt := int32(binary.LittleEndian.Uint32(head))
	if cnt < 1 || cnt > ImgMaxCnt+1 {
		return h, fmt.Sprintf("항목 수 %d(1~%d)", cnt, ImgMaxCnt+1)
	}
	h.cnt = int(cnt)
	for i := range h.sizes {
		h.sizes[i] = int32(binary.LittleEndian.Uint32(head[4+i*4:]))
	}
	if h.sizes[0] <= 0 || EntryName(head[nameBase:nameBase+maxName]) != htmlDst {
		return h, fmt.Sprintf("0번 항목이 %s 이 아니다", htmlDst)
	}
	var total int64
	for i := 0; i < h.cnt; i++ {
		if h.sizes[i] <= 0 {
			continue
		}
		name := EntryName(head[nameBase+i*maxName : nameBase+(i+1)*maxName])
		if name == "" {
			return h, fmt.Sprintf("%d번 항목 이름에 파일명이 없다", i)
		}
		// 작업 파일 이름은 Qt 판이 덮어쓰지 않게 막던 것이다 ― 호환을 위해 같게 거부한다
		if i > 0 && (strings.EqualFold(name, clipFm) || strings.EqualFold(name, htmlSrc) || strings.EqualFold(name, htmlDst)) {
			return h, fmt.Sprintf("%d번 항목 이름 %s 는 작업 파일이다", i, name)
		}
		h.names[i] = name
		total += int64(h.sizes[i])
	}
	if total > fileSize-HeaderSize {
		return h, fmt.Sprintf("항목 크기 합 %d > 남은 %d 바이트", total, fileSize-HeaderSize)
	}
	return h, ""
}

// Entry 는 묶음의 그림 항목 하나다.
type Entry struct {
	Name string
	Data []byte
}

// BuildBundle 은 묶음 바이트를 만든다. images 는 ImgMaxCnt 이하, 이름은 255 바이트 이하여야 한다.
func BuildBundle(html []byte, images []Entry) ([]byte, error) {
	if len(html) == 0 {
		return nil, errors.New("클립보드 묶음에 넣을 HTML 이 없습니다")
	}
	if len(images) > ImgMaxCnt {
		return nil, fmt.Errorf("클립보드 묶음의 그림 수 %d 가 상한 %d 를 넘습니다", len(images), ImgMaxCnt)
	}
	size := HeaderSize + len(html)
	for _, im := range images {
		size += len(im.Data)
	}
	out := make([]byte, HeaderSize, size)
	binary.LittleEndian.PutUint32(out, uint32(len(images)+1))
	binary.LittleEndian.PutUint32(out[4:], uint32(len(html)))
	copy(out[nameBase:nameBase+maxName-1], htmlDst)
	out = append(out, html...)
	for i, im := range images {
		if len(im.Name) >= maxName {
			return nil, fmt.Errorf("클립보드 묶음 항목 이름이 깁니다: %d 바이트", len(im.Name))
		}
		binary.LittleEndian.PutUint32(out[4+(i+1)*4:], uint32(len(im.Data)))
		copy(out[nameBase+(i+1)*maxName:], im.Name)
		out = append(out, im.Data...)
	}
	return out, nil
}

// Received 는 받은 서식 묶음을 푼 결과다.
type Received struct {
	HTML    string // 정리한 HTML. 그림은 data URI 다
	Text    string // 평문
	Refs    int    // 지운 바깥 참조
	Scripts int    // 지운 스크립트
}

// ParseBundle 은 받은 묶음을 풀어 정리한다. 앞머리가 맞지 않으면 "클립보드 수신 >>> 묶음 거부:" 뒤에 적을 사유를 오류로 돌려준다.
func ParseBundle(data []byte) (*Received, error) {
	h, problem := headerProblem(data, int64(len(data)))
	if problem != "" {
		return nil, errors.New(problem)
	}
	off := HeaderSize
	var html string
	imgs := map[string][]byte{}
	var names []string
	for i := 0; i < h.cnt; i++ {
		n := int(h.sizes[i])
		if n <= 0 {
			continue
		}
		body := data[off : off+n]
		off += n
		if i == 0 {
			html = string(body)
			continue
		}
		// 그림으로 판정되지 않는 항목은 싣지 않는다 ― 그 항목을 가리키는 참조도 정리에서 지워진다.
		// 원본은 확장자만 맞으면 파일로 풀었다. 판정 함수를 하나로 두어 보낼 수 있는 그림만 받는다(107)
		if !LooksLikeImage(body) {
			continue
		}
		imgs[h.names[i]] = body
		names = append(names, h.names[i])
	}
	html = strings.ReplaceAll(html, Placeholder, virtualRoot)
	html = rewriteRefs(html, names)
	clean := CleanReceivedHTML(html, virtualRoot, names)
	return &Received{
		HTML: inlineBundle(clean.HTML, imgs, names), Text: PlainText(clean.HTML),
		Refs: clean.Refs, Scripts: clean.Scripts,
	}, nil
}

// rewriteRefs 는 다른 OS 가 만든 HTML 에 자리표시 대신 절대 경로·file:// 가 남은 src 를 가상 폴더 경로로 바꾼다.
func rewriteRefs(html string, names []string) string {
	for _, name := range names {
		rep := strings.ReplaceAll(virtualRoot+name, "$", "$$")
		esc := regexp.QuoteMeta(name)
		for _, q := range []string{`"`, `'`} {
			re := regexp.MustCompile(`(?i)(\bsrc\s*=\s*)` + q + `(?:[^"']*[/\\])?` + esc + q)
			html = re.ReplaceAllString(html, "${1}"+q+rep+q)
		}
	}
	return html
}

// inlineBundle 은 묶음 항목을 가리키는 참조를 data URI 로 바꾸고, 앞머리가 허용 그림이 아닌 data:image 는 지운다.
func inlineBundle(html string, imgs map[string][]byte, names []string) string {
	var edits []edit
	scanTags(html, func(t tag) {
		for _, a := range t.attrs {
			if !slices.Contains(refAttrs, a.name) {
				continue
			}
			if name, ok := bundleName(a.raw, virtualRoot, names); ok {
				b := imgs[name]
				uri := "data:image/" + ImageFormat(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
				edits = append(edits, edit{a.begin, a.end, a.name + `="` + uri + `"`})
			} else if v := refValue(a.raw); IsDataImage(v) && !dataImageAllowed(v) {
				edits = append(edits, edit{a.begin, a.end, ""})
			}
		}
	})
	return applyEdits(html, edits)
}

// dataImageAllowed 는 base64 data:image 의 앞머리가 허용 그림인가다. base64 가 아니면 앞머리를 볼 수 없어 지운다.
func dataImageAllowed(v string) bool {
	i := strings.Index(v, ";base64,")
	if i < 0 {
		return false
	}
	body := v[i+len(";base64,"):]
	body = body[:min(len(body), 32)]
	head, err := base64.StdEncoding.DecodeString(body[:len(body)/4*4])
	return err == nil && LooksLikeImage(head)
}
