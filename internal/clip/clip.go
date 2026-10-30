package clip

import (
	"errors"
	"strings"
	"time"
)

// ErrBusy 는 다른 프로그램이 클립보드를 쥐고 놓지 않을 때다.
var ErrBusy = errors.New("클립보드를 다른 프로그램이 쓰고 있습니다. 잠시 뒤 다시 시도하세요.")

// Kind 는 클립보드 내용의 갈래다.
type Kind int

const (
	KindNone Kind = iota
	KindImage
	KindHTML
	KindText
)

// Content 는 Read 가 읽은 내용이다.
type Content struct {
	Kind   Kind
	PNG    []byte // 등록 형식 PNG(있으면)
	DIB    []byte // CF_DIB 또는 CF_DIBV5
	CFHTML []byte
	Text   string
}

// ImageFile 은 그림을 format("png" 면 PNG, 그 밖은 BMP) 파일 바이트로 만든다.
func (c *Content) ImageFile(format string) ([]byte, string, error) {
	if format == "png" {
		if len(c.PNG) > 0 {
			return c.PNG, ".png", nil
		}
		bmp, err := DIBToBMP(c.DIB)
		if err != nil {
			return nil, "", err
		}
		img, _, err := DecodeImage(bmp)
		if err != nil {
			return nil, "", err
		}
		b, err := EncodePNG(img)
		return b, ".png", err
	}
	if len(c.DIB) > 0 {
		b, err := DIBToBMP(c.DIB)
		return b, ".bmp", err
	}
	img, _, err := DecodeImage(c.PNG)
	if err != nil {
		return nil, "", err
	}
	b, err := EncodeBMP(img)
	return b, ".bmp", err
}

// CopySelection 은 Ctrl+C 를 흉내 내고 클립보드가 바뀌기를 최대 500ms 기다린다.
func CopySelection() {
	from := Sequence()
	SendCtrl('C')
	WaitChange(from, 500*time.Millisecond)
}

// Accelerator 는 설정의 조합키("ALT"·"SHIFT"·"ALT+SHIFT"·"CTRL+SHIFT")와 글자를 Wails 가속기 글자로 바꾼다. 글자가 영문·숫자 하나가 아니면 "".
func Accelerator(mod, ch string) string {
	ch = strings.ToUpper(strings.TrimSpace(ch))
	if len(ch) != 1 || !(ch[0] >= 'A' && ch[0] <= 'Z' || ch[0] >= '0' && ch[0] <= '9') {
		return ""
	}
	parts := strings.Split(strings.ToUpper(mod), "+")
	for i, p := range parts {
		switch p {
		case "ALT":
			parts[i] = "Alt"
		case "SHIFT":
			parts[i] = "Shift"
		case "CTRL":
			parts[i] = "Ctrl"
		default:
			return ""
		}
	}
	return strings.Join(append(parts, ch), "+")
}

// AllowClip 은 클립보드 권한 칸을 고른다: 업무망 올리기·인터넷망 내리기는 fm_clip_down 칸, 그 반대는 fm_clip_up 칸이다(서버 칸 이름이 망 방향 기준이다).
func AllowClip(business, up bool, fmClipUp, fmClipDown int32) bool {
	if business == up {
		return fmClipDown != 0
	}
	return fmClipUp != 0
}
