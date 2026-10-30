//go:build !windows

package link

import "syscall"

func exclusive(_, _ string, _ syscall.RawConn) error { return nil }

// RegisterShell 은 Windows 전용이다.
func RegisterShell(string) error { return nil }

// UnregisterShell 은 Windows 전용이다.
func UnregisterShell() error { return nil }

// HandleInstallArgs 는 Windows 전용이다.
func HandleInstallArgs([]string) bool { return false }
