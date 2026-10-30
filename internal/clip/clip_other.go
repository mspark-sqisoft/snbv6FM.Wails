//go:build !windows

package clip

// macOS·Linux: 글만 Wails 클립보드로 다룬다. 그림·HTML·키 흉내는 없다(docs/port/07 7.1).
// ponytail: 그림·HTML 은 NSPasteboard(cgo)·xclip 이 필요하다 ― 해당 OS 판을 낼 때 이 파일에 더한다.

import (
	"errors"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func clipboard() *application.ClipboardManager { return application.Get().Clipboard }

// Read 는 글만 읽는다.
func Read() (*Content, error) {
	s, ok := clipboard().Text()
	if !ok || s == "" {
		return &Content{}, nil
	}
	return &Content{Kind: KindText, Text: s}, nil
}

// Clear 는 클립보드를 빈 글로 바꾼다.
func Clear() error { return WriteText("") }

// WriteText 는 글을 넣는다.
func WriteText(s string) error {
	if !clipboard().SetText(s) {
		return ErrBusy
	}
	return nil
}

// WriteImage 는 이 OS 에서 지원하지 않는다.
func WriteImage(dib, png []byte) error {
	return errors.New("이 OS 는 그림 클립보드를 지원하지 않습니다")
}

// WriteHTML 은 평문만 넣는다.
func WriteHTML(html, text string) error { return WriteText(text) }

// Sequence 는 이 OS 에서 늘 0 이다.
func Sequence() uint32 { return 0 }

// SendCtrl 은 하지 않는다(사용자가 이미 복사한 뒤 단축키를 쓴다, 원본 Linux 와 같다).
func SendCtrl(byte) {}

// WaitChange 는 기다리지 않는다.
func WaitChange(uint32, time.Duration) {}
