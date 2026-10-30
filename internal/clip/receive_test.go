package clip

import (
	"bytes"
	"errors"
	"image"
	"strings"
	"testing"
)

func TestInterpret(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	p, err := EncodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := BuildBundle([]byte("<p>가<script>x</script></p>"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 앞 5바이트는 PNG 처럼 보여도 끝까지 열리지 않는 그림
	broken := append(append([]byte{}, p[:12]...), 0, 0, 0)
	cases := []struct {
		name string
		data []byte
		ext  string
		kind Kind
		err  error
	}{
		{"글", []byte("가나"), ".txt", KindText, nil},
		{"그림", p, ".png", KindImage, nil},
		{"대문자 확장자", p, ".PNG", KindImage, nil},
		{"fmh 인데 그림", p, ".fmh", KindImage, nil},
		{"묶음", bundle, ".fmh", KindHTML, nil},
		{"망가진 묶음", []byte("0123"), ".fmh", KindNone, ErrBundle},
		{"허용 밖 그림", []byte("<svg></svg>"), ".bmp", KindNone, ErrImage},
		{"손상 그림", broken, ".png", KindNone, ErrImage},
	}
	for _, c := range cases {
		in, err := Interpret(c.data, c.ext)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: 오류 %v", c.name, err)
			continue
		}
		if c.err == nil && in.Kind != c.kind {
			t.Errorf("%s: %v", c.name, in.Kind)
		}
	}
	in, err := Interpret(bundle, ".fmh")
	if err != nil || strings.Contains(in.HTML, "<script") || in.Scripts != 1 || in.Text != "가" {
		t.Fatalf("묶음 정리: %+v %v", in, err)
	}
	if _, err := Interpret(bytes.Repeat([]byte{0xFF}, 8), ".bmp"); err == nil || strings.Contains(err.Error(), "\n") {
		t.Fatalf("사유 문구: %v", err)
	}
}
