//go:build !windows

package files

import "golang.org/x/sys/unix"

func freeSpace(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:unconvert // Bsize 형이 OS 마다 다르다
}
