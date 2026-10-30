package policy

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Code 는 파일 하나의 검사 결과다(원본 ENUM_EXT_CHECK_TYPE, checkExtPolicy 의 반환값).
type Code int

const (
	OK             Code = 0
	ZeroSize       Code = -1
	OpenFail       Code = -2
	NoExt          Code = -3
	NotAllowed     Code = -4  // 확장자 허용 목록[3005]
	NotRegistered  Code = -6  // 패턴 -3
	Renamed        Code = -7  // 패턴 -5
	NoMatched      Code = -8  // 패턴 -6, 그리고 모르는 값
	PattNotAllowed Code = -9  // 패턴 -4
	Canceled       Code = -10 // 사용자 취소
	SizeOver       Code = -11
)

// PolicyText 는 결과 문구다(원본 getPolicyErrText).
func PolicyText(c Code) string {
	switch c {
	case OK:
		return "정상 파일"
	case ZeroSize:
		return "파일 사이즈 0"
	case OpenFail:
		return "파일 열기 실패"
	case NoExt:
		return "확장자 없음"
	case NotAllowed:
		return "허용되지 않은 확장자"
	case -5, Renamed:
		return "변경된 확장자"
	case NotRegistered:
		return "등록되지 않은 확장자"
	case NoMatched:
		return "존재하지 않는 헤더패턴"
	case PattNotAllowed:
		return "허용되지 않은 확장자 헤더패턴"
	case Canceled:
		return "사용자 취소"
	case SizeOver:
		return "전송 가능한 파일 크기 초과"
	}
	return "알 수 없는 오류"
}

// Detail 은 [상세] 칸이다(원본 getPolicyErrDetail): 정상=크기, -11=크기와 상한, -4·-6=.확장자(소문자). 나머지는 비운다.
func Detail(c Code, name string, size, limit int64) string {
	switch c {
	case OK:
		return SizeText(size)
	case SizeOver:
		if limit <= 0 {
			return SizeText(size)
		}
		return SizeText(size) + " / 상한 " + SizeText(limit)
	case NotAllowed, NotRegistered:
		if e := FileExt(name); e != "" {
			return "." + strings.ToLower(e)
		}
	}
	return ""
}

// mismatchDesc 는 불일치 기록[3316]의 영문 사유다. 서버 DB 에 그대로 쌓이므로 글자를 원본과 맞춘다.
func mismatchDesc(c Code) string {
	switch c {
	case NotRegistered:
		return "No Extension in DB"
	case PattNotAllowed:
		return "Not Allowed Extension"
	case Renamed:
		return "Renamed pattern"
	case NoMatched:
		return "Not exist matched pattern in DB"
	case ZeroSize:
		return "File Size Zero"
	case NotAllowed:
		return "EXT_PROFILE NOT ALLOWED"
	case SizeOver:
		return "File Size Over Than Profile"
	case NoExt:
		return "File has no extension"
	case OpenFail:
		return "File Open Error"
	}
	return "Unknown"
}

// Mismatch 는 불일치 기록[3316] 한 건이다.
type Mismatch struct {
	Fname string // 파일 경로(압축 안이면 <압축 경로>/<내부 경로>)
	Ext   string // 대문자
	Patt  string // 앞 64바이트의 대문자 16진
	Desc  string
}

// NewMismatch 는 실패 하나의 기록이다. 취소(-10)·통과는 기록하지 않으므로 ok 가 거짓.
func NewMismatch(c Code, fname string, head []byte) (Mismatch, bool) {
	if c == OK || c == Canceled {
		return Mismatch{}, false
	}
	return Mismatch{Fname: fname, Ext: FileExt(fname), Patt: HexHead(head), Desc: mismatchDesc(c)}, true
}

// FileExt 는 확장자(대문자)다. 마지막 점 뒤이고, 점이 맨 앞이거나 맨 끝이면 없다(.bashrc·a. 는 없음).
func FileExt(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return strings.ToUpper(name[i+1:])
}

// SizeText 는 크기 표기다(원본 getFileSizeToText, 1024 진법, 경계는 '>').
func SizeText(n int64) string {
	const k, m, g = 1024, 1024 * 1024, 1024 * 1024 * 1024
	switch {
	case n < 1:
		return "0 Byte"
	case n > g:
		return fmt.Sprintf("%.2f GB", float64(n)/g)
	case n > m:
		return fmt.Sprintf("%.2f MB", float64(n)/m)
	case n > k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	}
	return fmt.Sprintf("%d byte", n)
}

// HeadLen 은 읽는 앞부분 크기다. 대조는 앞 64바이트(16진 128자)만 쓴다.
const HeadLen = 256

// HexHead 는 앞 64바이트의 대문자 16진이다(원본 %02X).
func HexHead(head []byte) string {
	return strings.ToUpper(hex.EncodeToString(head[:min(len(head), 64)]))
}

// CheckEntry 는 이름·크기·앞부분으로 판정한다(압축 안 파일). 순서는 원본과 같다:
// 크기 0 → 앞부분 없음(열기 실패) → 확장자 없음 → 크기 상한 → 헤더 패턴 → 확장자 허용 목록. 첫 실패 하나만.
func CheckEntry(s *Set, name string, size int64, head []byte) Code {
	if size == 0 {
		return ZeroSize
	}
	if len(head) == 0 {
		return OpenFail
	}
	ext := FileExt(name)
	if s.Int("no_ext_no_permit") != 0 && ext == "" {
		return NoExt
	}
	if !s.CheckSize(size, false) {
		return SizeOver
	}
	switch s.CheckExtPattern(ext, HexHead(head)) {
	case PattOK:
	case PattNotReg:
		return NotRegistered
	case PattNotAllow:
		return PattNotAllowed
	case PattRenamed:
		return Renamed
	default:
		return NoMatched
	}
	if !s.CheckExtProfile(ext) {
		return NotAllowed
	}
	return OK
}

// CheckFile 은 PC 파일 하나를 판정한다(원본 checkExtPolicy): 취소 → 존재 → 크기 0 → 열기·앞 256바이트 → 나머지는 CheckEntry.
// 독점으로 잠긴 파일은 열기 실패(-2)다. 돌려준 head 는 불일치 기록에 쓴다.
func CheckFile(ctx context.Context, s *Set, path string) (c Code, size int64, head []byte) {
	if ctx.Err() != nil {
		return Canceled, 0, nil
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return OpenFail, 0, nil
	}
	size = st.Size()
	if size == 0 {
		return ZeroSize, 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return OpenFail, size, nil
	}
	buf := make([]byte, HeadLen)
	n, _ := io.ReadFull(f, buf)
	f.Close()
	if n <= 0 {
		return OpenFail, size, nil
	}
	head = buf[:n]
	return CheckEntry(s, filepath.Base(path), size, head), size, head
}
