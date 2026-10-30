package clip

// Windows 클립보드 입출력과 키 흉내. Wails Clipboard 는 글만 다뤄 Win32 를 직접 부른다.
// OpenClipboard 는 열기와 닫기가 같은 스레드여야 해서 한 호출 동안 OS 스레드를 묶는다.

import (
	"runtime"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	cfDIB         = 8
	cfDIBV5       = 17
	gmemMoveable  = 0x0002
	inputKeyboard = 1
	keyeventfUp   = 2
	vkShift       = 0x10
	vkControl     = 0x11
	vkMenu        = 0x12
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procEmptyClipboard    = user32.NewProc("EmptyClipboard")
	procIsFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procSetClipboardData  = user32.NewProc("SetClipboardData")
	procRegisterFormat    = user32.NewProc("RegisterClipboardFormatW")
	procClipboardSequence = user32.NewProc("GetClipboardSequenceNumber")
	procSendInput         = user32.NewProc("SendInput")
	procGetAsyncKeyState  = user32.NewProc("GetAsyncKeyState")
	procGlobalAlloc       = kernel32.NewProc("GlobalAlloc")
	procGlobalFree        = kernel32.NewProc("GlobalFree")
	procGlobalLock        = kernel32.NewProc("GlobalLock")
	procGlobalUnlock      = kernel32.NewProc("GlobalUnlock")
	procGlobalSize        = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory     = kernel32.NewProc("RtlMoveMemory")
	cfPNG, cfHTML         uintptr
)

func registerFormats() {
	if cfPNG == 0 {
		cfPNG = registerFormat("PNG")
		cfHTML = registerFormat("HTML Format")
	}
}

func registerFormat(name string) uintptr {
	p, _ := windows.UTF16PtrFromString(name)
	r, _, _ := procRegisterFormat.Call(uintptr(unsafe.Pointer(p)))
	return r
}

// withClipboard 는 클립보드를 열고 fn 을 부른 뒤 닫는다. 다른 프로그램이 쥐고 있으면 50ms 간격 3번 다시 시도한다.
func withClipboard(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	registerFormats()
	for i := 0; ; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			break
		}
		if i == 3 {
			return ErrBusy
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer procCloseClipboard.Call()
	return fn()
}

func available(format uintptr) bool {
	if format == 0 {
		return false
	}
	r, _, _ := procIsFormatAvailable.Call(format)
	return r != 0
}

// readGlobal 은 클립보드 형식의 바이트를 복사해 돌려준다. 없으면 nil.
func readGlobal(format uintptr) []byte {
	h, _, _ := procGetClipboardData.Call(format)
	if h == 0 {
		return nil
	}
	size, _, _ := procGlobalSize.Call(h)
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 || size == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(h)
	out := make([]byte, size)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&out[0])), p, size)
	return out
}

// writeGlobal 은 data 를 전역 메모리에 옮겨 클립보드에 넣는다. 성공하면 메모리는 OS 몫이다.
func writeGlobal(format uintptr, data []byte) error {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if h == 0 {
		return err
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return err
	}
	if len(data) > 0 {
		procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	}
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(format, h); r == 0 {
		procGlobalFree.Call(h)
		return err
	}
	return nil
}

func utf16Bytes(s string) []byte {
	u := utf16.Encode([]rune(s))
	u = append(u, 0)
	out := make([]byte, len(u)*2)
	for i, c := range u {
		out[i*2], out[i*2+1] = byte(c), byte(c>>8)
	}
	return out
}

func bytesToString16(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[i*2]) | uint16(b[i*2+1])<<8
		if u[i] == 0 {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u))
}

// Read 는 지금 클립보드를 그림 → HTML → 글 순서로 판정해 읽는다. 쓸 것이 없으면 Kind 가 KindNone 이다.
func Read() (*Content, error) {
	var c Content
	err := withClipboard(func() error {
		switch {
		case available(cfPNG) || available(cfDIBV5) || available(cfDIB):
			c.Kind = KindImage
			if available(cfPNG) {
				c.PNG = readGlobal(cfPNG)
			}
			// CF_DIB 를 먼저 본다 ― 없으면 Windows 가 만들어 주고, V5 보다 옛 앱·Qt 판이 읽기 쉽다
			if available(cfDIB) {
				c.DIB = readGlobal(cfDIB)
			} else if available(cfDIBV5) {
				c.DIB = readGlobal(cfDIBV5)
			}
		case available(cfHTML):
			c.Kind = KindHTML
			c.CFHTML = readGlobal(cfHTML)
		case available(cfUnicodeText):
			c.Kind = KindText
			c.Text = bytesToString16(readGlobal(cfUnicodeText))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Clear 는 클립보드를 비운다. 주인이 응답하지 않으면 Windows 가 오래 기다릴 수 있어 화면 스레드에서 부르지 않는다.
func Clear() error {
	return withClipboard(func() error {
		r, _, err := procEmptyClipboard.Call()
		if r == 0 {
			return err
		}
		return nil
	})
}

func replace(items ...func() error) error {
	return withClipboard(func() error {
		if r, _, err := procEmptyClipboard.Call(); r == 0 {
			return err
		}
		for _, it := range items {
			if err := it(); err != nil {
				return err
			}
		}
		return nil
	})
}

// WriteText 는 글을 클립보드에 넣는다.
func WriteText(s string) error {
	return replace(func() error { return writeGlobal(cfUnicodeText, utf16Bytes(s)) })
}

// WriteImage 는 CF_DIB 와 PNG 등록 형식을 함께 넣는다(Office·브라우저는 PNG 를 먼저 읽어 투명도를 살린다).
func WriteImage(dib, png []byte) error {
	return replace(
		func() error { return writeGlobal(cfDIB, dib) },
		func() error { return writeGlobal(cfPNG, png) },
	)
}

// WriteHTML 은 HTML 을 CF_HTML 로, text 를 평문으로 함께 넣는다.
func WriteHTML(html, text string) error {
	data := append(BuildCFHTML(html), 0)
	return replace(
		func() error { return writeGlobal(cfHTML, data) },
		func() error { return writeGlobal(cfUnicodeText, utf16Bytes(text)) },
	)
}

// Sequence 는 클립보드 변경 번호다(복사 뒤 바뀜 기다리기).
func Sequence() uint32 {
	r, _, _ := procClipboardSequence.Call()
	return uint32(r)
}

func keyDown(vk uint16) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

// keyboardInput 은 INPUT 구조체(type + KEYBDINPUT)다. amd64 에서 40바이트라 union 정렬까지 맞춘다.
type keyboardInput struct {
	kind  uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	extra uintptr
	_     [8]byte
}

func sendKeys(in []keyboardInput) {
	procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
}

// SendCtrl 은 수정키·글자 키를 뗄 때까지 최대 5초 기다린 뒤 Ctrl+글자를 보낸다(복사 'C'·붙여넣기 'V').
// 호출자는 화면 스레드가 아니어야 한다. 관리자 권한 창에는 들어가지 않는다(Windows UIPI, 원본과 같다).
func SendCtrl(letter byte) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (keyDown(vkShift) || keyDown(vkControl) || keyDown(vkMenu) || keyDown(uint16(letter))) {
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	vk := uint16(letter)
	sendKeys([]keyboardInput{
		{kind: inputKeyboard, vk: vkControl},
		{kind: inputKeyboard, vk: vk},
		{kind: inputKeyboard, vk: vk, flags: keyeventfUp},
		{kind: inputKeyboard, vk: vkControl, flags: keyeventfUp},
	})
	time.Sleep(10 * time.Millisecond)
	if keyDown(vkControl) {
		sendKeys([]keyboardInput{{kind: inputKeyboard, vk: vkControl, flags: keyeventfUp}})
	}
}

// WaitChange 는 클립보드 번호가 from 에서 바뀔 때까지 최대 d 만큼 기다린다.
func WaitChange(from uint32, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) && Sequence() == from {
		time.Sleep(20 * time.Millisecond)
	}
}
