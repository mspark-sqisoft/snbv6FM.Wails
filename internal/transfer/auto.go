package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"snbapp/internal/files"
)

// DoneDir 는 자동 업로드가 보낸 파일을 옮기는 하위 폴더 이름이다(원본 AUTO_TRANS_DONE_PATH).
const DoneDir = "done"

// FolderProblem 은 자동 전송 폴더를 쓸 수 없는 까닭이다(원본 autoTransferFolderProblem, 045 D6). 쓸 수 있으면 "".
// 업로드도 보낸 파일을 완료 폴더로 옮기므로 쓰기를 본다.
func FolderProblem(dir string) string {
	if dir == "" {
		return "폴더가 지정되지 않았다"
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "폴더가 없거나 폴더가 아니다: " + dir
	}
	if _, err := os.ReadDir(dir); err != nil {
		return "폴더를 읽을 수 없다: " + dir
	}
	if err := files.UsableFolder(dir); err != nil {
		return fmt.Sprintf("폴더에 쓸 수 없다: %s (%v)", dir, err)
	}
	return ""
}

// AutoCandidates 는 자동 업로드 폴더 바로 아래의 파일·폴더다(완료 폴더·숨김·앱이 잠깐 만든 파일 제외, 원본 checkAutoUploadFiles).
func AutoCandidates(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if (e.IsDir() && strings.EqualFold(n, DoneDir)) || strings.HasPrefix(n, files.WritePrefix) || hidden(n, e) {
			continue
		}
		out = append(out, filepath.Join(dir, n))
	}
	return out, nil
}

// MoveToDone 은 자동 업로드로 보낸 path 를 dir/done 아래로 옮긴다. 같은 이름이 있으면 번호(_00000)를 붙인다
// (원본은 rename 이 실패해 원래 자리에 남아 다음 주기에 다시 올라갔다).
func MoveToDone(path, dir string) (string, error) {
	done := filepath.Join(dir, DoneDir)
	if err := os.MkdirAll(done, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(done, filepath.Base(path))
	if files.Exists(dst) {
		var err error
		if dst, err = files.NumberedPath(dst, nil); err != nil {
			return "", err
		}
	}
	return dst, os.Rename(path, dst)
}
