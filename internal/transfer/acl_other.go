//go:build !windows

package transfer

// inheritACL 은 Windows 만 할 일이 있다(acl_windows.go). 여기서는 권한이 mode 로 정해져 이동해도 그대로다.
func inheritACL(string) {}
