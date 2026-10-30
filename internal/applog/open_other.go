//go:build !windows

package applog

import "os"

// openAppend 는 O_APPEND 로 연다 ― 줄마다 write 한 번이라 여러 프로세스가 같은 파일에 써도 줄이 찢어지지 않는다.
func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
}
