package clip

// 그림 형식 판정과 변환. 받은 그림은 내용 앞머리로 형식을 고르고 그 디코더만 부른다(원본 107) ―
// image.Decode 는 등록된 처리기를 모두 시험하므로 쓰지 않는다. 보내는 쪽 판정과 받는 쪽 판정은 ImageFormat 하나다.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// ImageFormat 은 앞머리가 허용 그림 형식이면 그 이름(png·jpeg·gif·bmp·webp·tiff), 아니면 "" 다.
func ImageFormat(h []byte) string {
	if len(h) < 12 {
		return ""
	}
	switch {
	case bytes.HasPrefix(h, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case h[0] == 0xff && h[1] == 0xd8 && h[2] == 0xff:
		return "jpeg"
	case bytes.HasPrefix(h, []byte("GIF87a")), bytes.HasPrefix(h, []byte("GIF89a")):
		return "gif"
	case bytes.HasPrefix(h, []byte("BM")) && len(h) >= 14:
		return "bmp"
	case bytes.HasPrefix(h, []byte("RIFF")) && string(h[8:12]) == "WEBP":
		return "webp"
	case bytes.HasPrefix(h, []byte("II*\x00")), bytes.HasPrefix(h, []byte("MM\x00*")):
		return "tiff"
	}
	return ""
}

// LooksLikeImage 는 앞머리가 허용 그림인가다.
func LooksLikeImage(h []byte) bool { return ImageFormat(h) != "" }

// maxImageBytes 는 풀었을 때의 그림 크기 상한이다(가로×세로×4). 작은 파일이 머리에 큰 크기를 적어 메모리를 다 쓰지 못하게 한다.
const maxImageBytes = 256 << 20

var codecs = map[string]struct {
	decode func(io.Reader) (image.Image, error)
	config func(io.Reader) (image.Config, error)
}{
	"png":  {png.Decode, png.DecodeConfig},
	"jpeg": {jpeg.Decode, jpeg.DecodeConfig},
	"gif":  {gif.Decode, gif.DecodeConfig},
	"bmp":  {bmp.Decode, bmp.DecodeConfig},
	"webp": {webp.Decode, webp.DecodeConfig},
	"tiff": {tiff.Decode, tiff.DecodeConfig},
}

// DecodeImage 는 앞머리로 고른 형식의 디코더로 연다. 머리의 크기부터 읽어 상한을 넘으면 풀지 않는다.
// 돌려준 글자는 형식 이름이다(판정 실패면 "").
func DecodeImage(b []byte) (image.Image, string, error) {
	f := ImageFormat(b)
	c, ok := codecs[f]
	if !ok {
		return nil, "", errors.New("허용 그림 형식이 아니다")
	}
	cfg, err := c.config(bytes.NewReader(b))
	if err != nil {
		return nil, f, err
	}
	if int64(cfg.Width)*int64(cfg.Height)*4 > maxImageBytes {
		return nil, f, fmt.Errorf("그림이 너무 크다: %dx%d", cfg.Width, cfg.Height)
	}
	img, err := c.decode(bytes.NewReader(b))
	if err == nil && (img == nil || img.Bounds().Empty()) {
		err = errors.New("빈 그림이다")
	}
	return img, f, err
}

// DIBToBMP 는 CF_DIB·CF_DIBV5 바이트에 BMP 파일 머리(14바이트)를 붙인다.
// 32비트 BI_BITFIELDS + 40바이트 머리(웹 브라우저가 흔히 준다)는 마스크가 기본값이면 BI_RGB 로 바꿔 마스크를 뗀다 ― 받는 쪽 디코더가 읽게 한다.
func DIBToBMP(dib []byte) ([]byte, error) {
	if len(dib) < 40 {
		return nil, fmt.Errorf("DIB 가 짧습니다: %d 바이트", len(dib))
	}
	le := binary.LittleEndian
	hdr := int(le.Uint32(dib))
	if hdr < 40 || hdr > len(dib) {
		return nil, fmt.Errorf("DIB 머리 크기가 맞지 않습니다: %d", hdr)
	}
	bits := int(le.Uint16(dib[14:]))
	comp := le.Uint32(dib[16:])
	colors := int(le.Uint32(dib[32:]))
	palette := 0
	switch {
	case bits <= 8:
		if colors == 0 {
			colors = 1 << bits
		}
		palette = colors * 4
	case comp == 3 && hdr == 40:
		palette = 12
	default:
		palette = colors * 4
	}
	if hdr+palette > len(dib) {
		return nil, fmt.Errorf("DIB 색표가 데이터를 넘습니다: %d", palette)
	}
	if comp == 3 && hdr == 40 && bits == 32 && len(dib) >= 52 &&
		le.Uint32(dib[40:]) == 0xff0000 && le.Uint32(dib[44:]) == 0xff00 && le.Uint32(dib[48:]) == 0xff {
		fixed := make([]byte, 0, len(dib)-12)
		fixed = append(fixed, dib[:40]...)
		fixed = append(fixed, dib[52:]...)
		le.PutUint32(fixed[16:], 0)
		dib, palette = fixed, 0
	}
	out := make([]byte, 14, 14+len(dib))
	out[0], out[1] = 'B', 'M'
	le.PutUint32(out[2:], uint32(14+len(dib)))
	le.PutUint32(out[10:], uint32(14+hdr+palette))
	return append(out, dib...), nil
}

// ImageToDIB 는 그림을 32비트 CF_DIB(40바이트 머리, 아래에서 위로, BGRA)로 만든다.
func ImageToDIB(img image.Image) []byte {
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(nrgba, nrgba.Rect, img, r.Min, draw.Src)
	out := make([]byte, 40+w*h*4)
	le := binary.LittleEndian
	le.PutUint32(out, 40)
	le.PutUint32(out[4:], uint32(w))
	le.PutUint32(out[8:], uint32(h))
	le.PutUint16(out[12:], 1)
	le.PutUint16(out[14:], 32)
	le.PutUint32(out[20:], uint32(w*h*4))
	for y := 0; y < h; y++ {
		src := nrgba.Pix[y*nrgba.Stride : y*nrgba.Stride+w*4]
		dst := out[40+(h-1-y)*w*4:]
		for x := 0; x < w*4; x += 4 {
			dst[x], dst[x+1], dst[x+2], dst[x+3] = src[x+2], src[x+1], src[x], src[x+3]
		}
	}
	return out
}

// EncodePNG 는 그림을 PNG 로 쓴다.
func EncodePNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// EncodeBMP 는 그림을 BMP 로 쓴다.
func EncodeBMP(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := bmp.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
