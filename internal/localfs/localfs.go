// Package localfs 는 로컬 탐색기(파일전송 화면 아래 패널)의 목록·드라이브·새 폴더·시작 폴더를 맡는다(docs/port/12 5.7·7.3~7.6).
// 원본 src/model/LocalFileListModel.cpp·LocalDirTreeModel.cpp 를 옮겼다. 화면 문구는 원본 한국어 그대로 오류 글자로 낸다.
package localfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// 목록 읽기 오류(원본 문구 그대로).
var (
	ErrNotFound   = errors.New("폴더를 찾을 수 없습니다.")
	ErrPermission = errors.New("폴더를 읽을 권한이 없습니다.")
)

// Entry 는 폴더 안 항목 하나다.
type Entry struct {
	Name     string
	Path     string
	IsDir    bool
	IsLink   bool   // .lnk 바로 가기(대상이 아니라 링크 파일 자체 크기)
	Size     int64  // 폴더는 0
	SizeText string // 탐색기 크기 글자(폴더는 빈 값)
	Ext      string // 확장자 대문자(점 없음)
	Type     string // 운영체제의 유형 설명(Windows 탐색기와 같은 글자). 모르면 빈 값 ― 화면이 「%1 파일」·「파일」로 채운다
	ModTime  time.Time
}

// List 는 dir 의 항목을 폴더 먼저·이름 자연 정렬(숫자는 수로, 대소문자 무시, 한국어 순)로 돌려준다.
// 숨김 파일은 showHidden 일 때만, 시스템 파일은 늘 포함한다(원본 필터 System). dir 은 절대 경로로 정리해 돌려준다.
// 읽기 거부 폴더는 ErrPermission 이다(원본은 빈 목록이 보일 수 있었다 ― 읽기 결과가 있으면 거부로 보지 않았다).
func List(dir string, showHidden bool) (string, []Entry, error) {
	if strings.TrimSpace(dir) == "" {
		return "", nil, ErrNotFound
	}
	dir = normPath(dir)
	if len(dir) == 2 && dir[1] == ':' { // "C:" 는 그 드라이브의 현재 폴더가 아니라 뿌리로 본다
		dir += string(filepath.Separator)
	}
	clean, err := filepath.Abs(dir)
	if err != nil {
		return dir, nil, ErrNotFound
	}
	st, err := os.Stat(clean)
	switch {
	case errors.Is(err, fs.ErrPermission):
		return clean, nil, ErrPermission
	case err != nil || !st.IsDir():
		return clean, nil, ErrNotFound
	}
	des, err := os.ReadDir(clean)
	if err != nil && len(des) == 0 {
		if errors.Is(err, fs.ErrPermission) {
			return clean, nil, ErrPermission
		}
		if errors.Is(err, fs.ErrNotExist) {
			return clean, nil, ErrNotFound
		}
		return clean, nil, fmt.Errorf("%w (%v)", ErrPermission, err)
	}
	types := typeNamer()
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		info, ierr := de.Info()
		if ierr != nil { // 읽는 사이 지워짐 등
			continue
		}
		if !showHidden && isHidden(de.Name(), info) {
			continue
		}
		e := Entry{Name: de.Name(), Path: filepath.Join(clean, de.Name()), IsDir: info.IsDir(), ModTime: info.ModTime()}
		if !e.IsDir {
			e.Size = info.Size()
			e.SizeText = SizeText(e.Size)
			if ext := filepath.Ext(e.Name); len(ext) > 1 {
				e.Ext = strings.ToUpper(ext[1:])
				e.IsLink = e.Ext == "LNK"
				e.Type = types(e.Ext)
			}
		}
		out = append(out, e)
	}
	SortEntries(out)
	return clean, out, nil
}

// SortEntries 는 폴더 먼저, 그다음 이름 자연 정렬(원본 QCollator numericMode·CaseInsensitive)이다.
func SortEntries(es []Entry) {
	c := collate.New(language.Korean, collate.Numeric, collate.IgnoreCase)
	slices.SortStableFunc(es, func(a, b Entry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		if r := c.CompareString(a.Name, b.Name); r != 0 {
			return r
		}
		return strings.Compare(a.Name, b.Name)
	})
}

// SizeText 는 탐색기 크기 글자다: 1024 미만 「N byte」, KB 는 정수, 그 위는 소수 한 자리(원본 sizeTextOf).
// 권한 팝업의 크기 글자(getFileSizeToText)와 규칙이 다르다 ― 합치지 않는다.
func SizeText(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d byte", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n) / 1024
	u := 0
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	if u == 0 {
		return fmt.Sprintf("%.0f KB", v+0.0000001) // qRound 와 같게 .5 는 올린다
	}
	return fmt.Sprintf("%.1f %s", v, units[u])
}

// Parent 는 dir 의 상위 폴더다. 뿌리(드라이브·/)면 빈 값.
func Parent(dir string) string {
	p := filepath.Dir(dir)
	if p == dir {
		return ""
	}
	return p
}

// 새 폴더 오류(원본 makeDir 문구 그대로, 검사 순서도 같다).
var (
	ErrNoParent    = errors.New("먼저 폴더를 선택해 주세요.")
	ErrEmptyName   = errors.New("폴더 이름을 입력해 주세요.")
	ErrBadChars    = errors.New("폴더 이름에 다음 문자는 쓸 수 없습니다.\n\\ / : * ? \" < > |")
	ErrBadName     = errors.New("사용할 수 없는 폴더 이름입니다.")
	ErrExists      = errors.New("같은 이름의 폴더나 파일이 이미 있습니다.")
	ErrCannotWrite = errors.New("폴더를 만들 수 없습니다. 쓰기 권한을 확인해 주세요.")
)

// CheckDirName 은 새 폴더 이름을 검사하고 앞뒤 공백을 걷은 이름을 돌려준다(존재·쓰기는 MakeDir 가 본다).
// 원본보다 더 막는 것: Windows 예약 이름(CON·NUL·COM1…)과 끝의 점 ― 원본은 만들기에서 실패하거나 다른 이름으로 만들어졌다.
func CheckDirName(name string) (string, error) {
	n := strings.TrimSpace(name)
	switch {
	case n == "":
		return "", ErrEmptyName
	case strings.ContainsAny(n, `\/:*?"<>|`):
		return "", ErrBadChars
	case n == "." || n == ".." || strings.HasSuffix(n, ".") || isReserved(n):
		return "", ErrBadName
	}
	for _, r := range n {
		if r < 0x20 {
			return "", ErrBadName
		}
	}
	return n, nil
}

func isReserved(n string) bool {
	base := strings.ToUpper(n)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return true
	}
	return false
}

// MakeDir 는 parent 아래에 name 폴더를 만들고 그 경로를 돌려준다.
func MakeDir(parent, name string) (string, error) {
	if strings.TrimSpace(parent) == "" {
		return "", ErrNoParent
	}
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		return "", ErrNoParent
	}
	n, err := CheckDirName(name)
	if err != nil {
		return "", err
	}
	p := filepath.Join(parent, n)
	if _, err := os.Lstat(p); err == nil {
		return "", ErrExists
	}
	if err := os.Mkdir(p, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", ErrExists
		}
		return "", ErrCannotWrite
	}
	return p, nil
}

// usable 은 dir 이 읽을 수 있는 폴더인가다.
func usable(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == nil || errors.Is(err, io.EOF)
}

// StartKind 는 시작 폴더가 어디서 왔는지다(설정 화면의 안내 글자 「바탕 화면 (%1)」·「홈 (%1)」).
type StartKind string

const (
	StartConfigured StartKind = "configured"
	StartLast       StartKind = "last"
	StartDesktop    StartKind = "desktop"
	StartHome       StartKind = "home"
	StartRoot       StartKind = "root"
)

// ResolveStart 는 탐색기·폴더 선택 창의 첫 폴더다: 설정 시작 폴더 → 마지막 폴더 → 바탕 화면 → 홈 → 루트(원본 resolveStartFolder,
// 마지막 폴더는 Wails 판에서 더했다). 쓸 수 없는 값은 건너뛰고 저장값은 지우지 않는다.
func ResolveStart(configured, last string) (string, StartKind) {
	if p := normPath(configured); usable(p) {
		return p, StartConfigured
	}
	if p := normPath(last); usable(p) {
		return p, StartLast
	}
	return DefaultStart()
}

// DefaultStart 는 설정이 없을 때의 첫 폴더다: 바탕 화면 → 홈 → 루트.
func DefaultStart() (string, StartKind) {
	if p := DesktopDir(); usable(p) {
		return p, StartDesktop
	}
	if p, _ := os.UserHomeDir(); usable(p) {
		return filepath.Clean(p), StartHome
	}
	return rootDir(), StartRoot
}

// normPath 는 file:/// 주소(원본 QML 이 남긴 값)를 경로로 바꾸고 정리한다.
func normPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(p, "file:///"); ok {
		p = rest
		if len(p) < 2 || p[1] != ':' { // 유닉스 file:///home/x
			p = "/" + p
		}
	} else if rest, ok := strings.CutPrefix(p, "file://"); ok {
		p = rest
	}
	return filepath.Clean(p)
}

// NormPath 는 normPath 의 공개판이다(옛 설정 옮기기·화면 경로 입력).
func NormPath(p string) string { return normPath(p) }
