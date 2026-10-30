package domain

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// HostsPath 는 hosts 파일 경로다.
func HostsPath() (string, error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil || dir == "" {
		return "", ErrNoDir
	}
	return filepath.Join(dir, "drivers", "etc", "hosts"), nil
}
