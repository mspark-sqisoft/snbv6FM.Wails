package transfer

import "syscall"

func setHidden(p string) error {
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(u, syscall.FILE_ATTRIBUTE_HIDDEN)
}
