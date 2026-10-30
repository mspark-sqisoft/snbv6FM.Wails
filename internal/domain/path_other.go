//go:build !windows

package domain

// HostsPath 는 hosts 파일 경로다.
func HostsPath() (string, error) { return "/etc/hosts", nil }
