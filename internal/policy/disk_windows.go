package policy

import "golang.org/x/sys/windows"

// diskFree 는 dir 이 있는 디스크에서 이 사용자가 쓸 수 있는 남은 바이트다. 모르면 ok 가 거짓.
func diskFree(dir string) (uint64, bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, nil, nil); err != nil {
		return 0, false
	}
	return free, true
}
