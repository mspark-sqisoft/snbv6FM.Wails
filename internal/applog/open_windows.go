package applog

import (
	"os"
	"syscall"
)

// openAppend 는 원본 CreateFileW 와 같게 연다: 쓰기 권한은 FILE_APPEND_DATA 만(여러 프로세스가 같은 파일에 써도
// 줄마다 끝에 붙어 찢어지지 않는다), 공유는 읽기·쓰기·삭제(다른 프로세스가 열어 둬도 폴더를 지울 수 있게).
// Go 의 os.OpenFile(O_APPEND) 은 FILE_SHARE_DELETE 를 주지 않아 직접 연다.
func openAppend(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.FILE_APPEND_DATA|syscall.SYNCHRONIZE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
