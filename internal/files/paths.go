package files

// 받기 전·중에 전송 영역이 쓰는 순수 함수(docs/port/04 7.4·7.5·7.8, 3.3). 서버·화면을 모른다.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ErrStorageNotEnough 는 받을 자리의 남은 공간이 파일보다 작을 때다(원본 STORAGE_NOT_ENOUGH 문구).
var ErrStorageNotEnough = errors.New("저장소의 용량이 부족합니다.")

// ErrBadName 은 정리한 뒤의 이름이 비었거나 "."·".." 일 때다(서버 이름이 받을 폴더를 벗어나지 못하게).
var ErrBadName = errors.New("저장할 수 없는 파일 이름입니다.")

// ErrNoNumber 는 _00000~_99999 가 다 찼을 때다. 원본은 원래 이름에 덮어썼다 ― 여기는 실패(원본 FILE_OPEN_FAILED 문구).
var ErrNoNumber = errors.New("파일 열기 실패")

// SanitizeName 은 저장할 파일 이름에서 OS 가 막는 글자를 공백으로 바꾼다(원본 sanitizeFileName).
// Windows 는 ? \ / * : " < > | 9개와 제어 글자, 그 밖 OS 도 / 와 NUL(경로 이탈 방지 ― 원본에 없음).
func SanitizeName(name string) string { return sanitizeFor(runtime.GOOS, name) }

func sanitizeFor(goos, name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == 0 || (goos == "windows" && (r < 32 || strings.ContainsRune(`?\*:"<>|`, r))) {
			return ' '
		}
		return r
	}, name)
}

// SavePath 는 받을 자리 dir 와 표의 [파일명](DisplayName)으로 저장 경로를 만든다.
func SavePath(dir, name string) (string, error) {
	n := SanitizeName(name)
	if strings.TrimSpace(n) == "" || n == "." || n == ".." {
		return "", ErrBadName
	}
	return filepath.Join(dir, n), nil
}

// Candidate 는 받기 대상 하나의 받을 폴더와 표의 [파일명]이다.
type Candidate struct {
	Dir  string
	Name string
}

// Duplicates 는 「파일 중복 확인」 창으로 물어야 할 대상의 색인이다(원본 downloadDuplicates).
// 같은 요청 안에서 이름이 겹치는 뒤 항목은 묻지 않는다 ― 앞 항목이 자리를 차지하고 받을 때 번호가 붙는다.
// Windows·macOS 는 이름 대소문자를 가리지 않아 소문자로 비교한다(원본은 가림). exists 가 nil 이면 디스크를 본다(os.Lstat).
// 이름이 저장할 수 없는 것(ErrBadName)은 묻지 않는다 ― 받기에서 실패로 끝난다.
func Duplicates(cands []Candidate, exists func(path string) bool) []int {
	if exists == nil {
		exists = Exists
	}
	fold := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	taken := map[string]bool{}
	var asking []int
	for i, c := range cands {
		p, err := SavePath(c.Dir, c.Name)
		if err != nil {
			continue
		}
		key := p
		if fold {
			key = strings.ToLower(p)
		}
		if taken[key] {
			continue
		}
		taken[key] = true
		if exists(p) {
			asking = append(asking, i)
		}
	}
	return asking
}

// Exists 는 그 자리에 파일이든 폴더든 무엇이 있는가다.
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// NumberedPath 는 path 의 이름 끝에 _00000 부터 처음 빈 번호를 붙인 경로다(원본 addFileNumbering, [이름 변경]).
// 확장자는 마지막 점 뒤(점이 맨 앞이거나 맨 끝이면 확장자 없음), 글자는 원래 대소문자 그대로. 폴더도 같다(a.b → a_00000.b).
// 받기 직전에 다시 불러 물은 뒤 생긴 겹침도 피한다. exists 가 nil 이면 디스크를 본다.
func NumberedPath(path string, exists func(string) bool) (string, error) {
	if exists == nil {
		exists = Exists
	}
	dir, file := filepath.Split(path)
	stem, ext := file, ""
	if i := strings.LastIndexByte(file, '.'); i > 0 && i < len(file)-1 {
		stem, ext = file[:i], file[i:]
	}
	for n := 0; n <= 99999; n++ {
		p := filepath.Join(dir, fmt.Sprintf("%s_%05d%s", stem, n, ext))
		if !exists(p) {
			return p, nil
		}
	}
	return "", ErrNoNumber
}

// RemoveAfterDownload 는 받기 성공 뒤 서버 원본을 지울지(RemoveFile flag 1)다: 정책 max_downlod_count 가 있고 1 일 때만.
// 키가 없으면 지우지 않는다(원본은 isValid() 로 먼저 본다). 철자 downlod 는 서버 키 그대로.
func RemoveAfterDownload(profile map[string]string) bool {
	v, ok := profile["max_downlod_count"]
	if !ok {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	return err == nil && n == 1
}

// AcceptChecksum 은 받기 마지막 응답의 reason 을 검사값으로 대조할지다(원본 acceptChecksum, 파일 받기).
// code 3 이고 비어 있지 않을 때만 ― 옛 서버는 code 0 에 「Successfully downloaded file」을 싣는다.
// 16진이 아닌 값도 받는다(받은 파일 해시와 같을 수 없어 불일치로 막힌다). 비교는 대소문자를 가리지 않는다(strings.EqualFold).
func AcceptChecksum(code int32, reason string) bool { return code == 3 && reason != "" }

// FreeSpace 는 dir 가 있는 디스크에서 이 사용자가 쓸 수 있는 바이트다(원본 QStorageInfo::bytesAvailable, 할당량 반영).
// dir 가 아직 없으면 있는 상위 폴더로 올라가 잰다.
func FreeSpace(dir string) (uint64, error) {
	p, err := filepath.Abs(dir)
	if err != nil {
		return 0, err
	}
	for {
		if _, err := os.Stat(p); err == nil {
			return freeSpace(p)
		}
		up := filepath.Dir(p)
		if up == p {
			return 0, fmt.Errorf("남은 공간을 잴 폴더가 없습니다: %s", dir)
		}
		p = up
	}
}

// CheckSpace 는 dir 에 need 바이트를 받을 수 있는지다. 모자라면 ErrStorageNotEnough.
// 잴 수 없으면(네트워크 경로 등) 막지 않는다 ― 쓰다가 실패하면 그 오류로 끝난다.
func CheckSpace(dir string, need uint64) error {
	free, err := FreeSpace(dir)
	if err == nil && free < need {
		return ErrStorageNotEnough
	}
	return nil
}

// AutoSkip 은 자동 다운로드에서 이 파일을 받지 않는 사유다(원본 AutoDownloadSkip). 글자는 로그에만 쓴다.
type AutoSkip string

const (
	AutoNone               AutoSkip = ""
	AutoUiLess             AutoSkip = "UILess 유형이라 다른 경로에서 처리한다"
	AutoNotAutoTransAll    AutoSkip = "서버 경로 파일인데 자동전송 정책이 전체가 아니다"
	AutoPrintRelayOff      AutoSkip = "인쇄 파일인데 인쇄 중계를 쓰지 않는다"
	AutoEmptyFolder        AutoSkip = "받을 폴더가 지정되지 않았다"
	AutoUserFolderUnusable AutoSkip = "받을 폴더가 없거나 폴더가 아니거나 쓸 수 없다"
)

// AutoPolicy 는 자동 다운로드 판정에 쓰는 정책·설정 값이다.
type AutoPolicy struct {
	AutoTransAll     bool   // 정책 autotrans_step == 2
	PrintRelayUsed   bool   // 정책 print_relay > 0
	UserFolder       string // 설정 망별 자동 다운로드 폴더(유형 0)
	UserFolderUsable bool   // UsableFolder(UserFolder) == nil ― 부르는 쪽이 실행마다 한 번 잰다
	ServerPath       string // 정책 autodown_path(유형 1)
	PrintFolder      string // <사용자 데이터>/print_Download(유형 3)
}

// AutoPolicyOf 는 프로파일에서 자동 다운로드 정책 값을 읽는다(폴더 칸은 부르는 쪽이 채운다).
func AutoPolicyOf(profile map[string]string) AutoPolicy {
	n := func(k string) int { v, _ := strconv.Atoi(strings.TrimSpace(profile[k])); return v }
	return AutoPolicy{AutoTransAll: n("autotrans_step") == 2, PrintRelayUsed: n("print_relay") > 0, ServerPath: profile["autodown_path"]}
}

// AutoTarget 은 자동 다운로드가 파일 하나를 받을 폴더다(원본 resolveAutoDownloadTarget). 유형 autoDown:
// 0(그 밖 값 포함) 사용자 폴더 · 1 서버 경로 · 2 UILess(받지 않음) · 3 인쇄 폴더. 폴더를 정하지 못하면 skip 이 빈 값이 아니다.
func AutoTarget(autoDown int32, p AutoPolicy) (folder string, skip AutoSkip) {
	switch autoDown {
	case 1:
		if !p.AutoTransAll {
			return "", AutoNotAutoTransAll
		}
		folder = p.ServerPath
	case 2:
		return "", AutoUiLess
	case 3:
		if !p.PrintRelayUsed {
			return "", AutoPrintRelayOff
		}
		folder = p.PrintFolder
	default:
		if p.UserFolder != "" && !p.UserFolderUsable { // 없는 사용자 폴더는 만들지 않는다
			return "", AutoUserFolderUnusable
		}
		folder = p.UserFolder
	}
	if folder == "" {
		return "", AutoEmptyFolder
	}
	return folder, AutoNone
}

// UsableFolder 는 dir 가 있고 폴더이며 실제로 파일을 만들 수 있는지다(원본: 읽기 전용 속성은 ACL 을 못 봐 임시 파일을 만들어 본다).
func UsableFolder(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("폴더가 아닙니다: %s", dir)
	}
	f, err := os.CreateTemp(dir, ".snb_write_*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
