package transfer

import (
	"io/fs"
	"syscall"
)

// hidden 은 Windows 숨김 속성이다(Qt QDir::Hidden 이 Windows 에서 보는 것 ― 이름의 "." 은 보지 않는다).
func hidden(_ string, d fs.DirEntry) bool {
	info, err := d.Info()
	if err != nil {
		return false
	}
	a, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && a.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
