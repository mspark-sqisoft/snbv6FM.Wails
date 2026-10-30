package link

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// exclusive 는 포트를 SO_EXCLUSIVEADDRUSE 로 연다 ― 다른 프로세스가 같은 포트에 겹쳐 붙어 URL 을 가로채지 못하게 한다.
func exclusive(_, _ string, c syscall.RawConn) error {
	var opt error
	if err := c.Control(func(fd uintptr) {
		const soExclusiveAddrUse = ^windows.SO_REUSEADDR // WinSock2 의 정의, x/sys 에 없다
		opt = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, soExclusiveAddrUse, 1)
	}); err != nil {
		return err
	}
	return opt
}

type shellEntry struct {
	key, title, mode, arg string
}

// shellRoot 는 시험이 임시 키로 바꾼다 ― 이 PC 에 깔린 앱의 메뉴와 키 이름이 같다.
var shellRoot = `Software\Classes\`

var shellEntries = []shellEntry{
	{`*\shell\SNBUpload`, "snB App 파일 업로드", "upload", "%1"},
	{`Directory\shell\SNBDownload`, "snB App 파일 다운로드", "download", "%1"},
	{`Directory\Background\shell\SNBDownload`, "snB App 파일 다운로드", "download", "%V"},
}

// RegisterShell 은 탐색기 오른쪽 메뉴 셋을 HKCU 에 등록한다(관리자 권한 없이). Windows 11 새 메뉴에서는 「추가 옵션 표시」 아래에 있다.
// 원본이 쓰던 옛 메뉴 되돌리기 CLSID 값은 다른 프로그램의 메뉴 모양까지 바꾸므로 쓰지 않는다.
func RegisterShell(exe string) error {
	for _, e := range shellEntries {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, shellRoot+e.key, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = k.SetStringValue("", e.title)
		if err == nil {
			err = k.SetStringValue("Icon", fmt.Sprintf(`"%s",0`, exe))
		}
		k.Close()
		if err != nil {
			return err
		}
		c, _, err := registry.CreateKey(registry.CURRENT_USER, shellRoot+e.key+`\command`, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = c.SetStringValue("", fmt.Sprintf(`"%s" "--mode=%s" "%s"`, exe, e.mode, e.arg))
		c.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// UnregisterShell 은 메뉴를 지운다. 없으면 그냥 지나간다.
func UnregisterShell() error {
	var errs []error
	for _, e := range shellEntries {
		for _, key := range []string{shellRoot + e.key + `\command`, shellRoot + e.key} {
			if err := registry.DeleteKey(registry.CURRENT_USER, key); err != nil && !errors.Is(err, registry.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// HandleInstallArgs 는 설치본이 부르는 인자(--register-shell·--unregister-shell)를 처리한다. 처리했으면 참 ― main 은 곧바로 끝낸다.
func HandleInstallArgs(args []string) bool {
	for _, a := range args {
		switch a {
		case "--register-shell":
			exe, err := os.Executable()
			if err == nil {
				err = RegisterShell(exe)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return true
		case "--unregister-shell":
			if err := UnregisterShell(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return true
		}
	}
	return false
}
