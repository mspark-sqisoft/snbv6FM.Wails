package clip

// 서버에서 받은 클립보드 파일을 이 PC 클립보드에 넣을 내용으로 바꾼다(원본 ViewControllerNotifySlot 의 받기 완료, specs/107).

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

var (
	// ErrBundle 은 묶음을 거부했을 때다 ― 클립보드를 건드리지 않고 로그만 남긴다.
	ErrBundle = errors.New("묶음 거부")
	// ErrImage 는 받은 그림을 열 수 없을 때다 ― 안내 창을 띄운다.
	ErrImage = errors.New("그림을 열 수 없다")
)

// Incoming 은 받은 내용 한 건이다. Kind 에 맞는 칸만 찬다.
type Incoming struct {
	Kind    Kind
	Text    string // KindText 는 글, KindHTML 은 평문 대체 글
	HTML    string
	Image   image.Image
	Format  string // 그림 형식 이름
	Refs    int    // 묶음의 그림 참조 수
	Scripts int    // 지운 스크립트 수
}

// Interpret 은 받은 파일 바이트를 확장자(.txt·.png·.bmp·.fmh)대로 해석한다. .fmh 인데 앞머리가 그림이면 그림이다.
// 그림은 허용 형식만 그 형식의 디코더로 연다. ErrImage 오류 문구가 로그에 남길 사유다(내용은 넣지 않는다).
func Interpret(data []byte, ext string) (*Incoming, error) {
	ext = strings.ToLower(ext)
	if ext == ".fmh" && !LooksLikeImage(data) {
		r, err := ParseBundle(data)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBundle, err)
		}
		return &Incoming{Kind: KindHTML, HTML: r.HTML, Text: r.Text, Refs: r.Refs, Scripts: r.Scripts}, nil
	}
	if ext == ".txt" {
		return &Incoming{Kind: KindText, Text: string(data)}, nil
	}
	f := ImageFormat(data)
	if f == "" {
		return nil, fmt.Errorf("%w: 허용 밖 앞머리[%x] 크기[%d]", ErrImage, data[:min(len(data), 16)], len(data))
	}
	img, _, err := DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s 손상: %v", ErrImage, f, err)
	}
	return &Incoming{Kind: KindImage, Image: img, Format: f}, nil
}

// Put 은 내용을 이 PC 클립보드에 넣는다.
func (in *Incoming) Put() error {
	switch in.Kind {
	case KindHTML:
		return WriteHTML(in.HTML, in.Text)
	case KindImage:
		p, err := EncodePNG(in.Image)
		if err != nil {
			return err
		}
		return WriteImage(ImageToDIB(in.Image), p)
	}
	return WriteText(in.Text)
}
