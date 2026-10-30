//go:build !windows

package policy

import "golang.org/x/sys/unix"

// diskFree 는 dir 이 있는 디스크에서 이 사용자가 쓸 수 있는 남은 바이트다. 모르면 ok 가 거짓.
func diskFree(dir string) (uint64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true //nolint:gosec
}
