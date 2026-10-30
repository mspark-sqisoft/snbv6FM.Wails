// Package policy 는 업로드 파일 검사(확장자·헤더 패턴·크기·압축 내부)와 정책 한 벌이다(docs/port/02).
// 원본은 snBv6FM 의 imports/FileMoverPolicy(extpolicy.cpp·filemoverpolicy.cpp), imports/ZipExtractor,
// FileMoverGrpc.cpp 의 checkExtPolicy·checkZipFile, ViewController.cpp 의 verifyUploadFile·checkLockedZip 이다.
package policy

import (
	"sort"
	"strconv"
	"strings"
)

// PattRow 는 헤더 패턴 응답[3015] 한 행이다.
type PattRow struct {
	Ext   string
	HFlag bool   // 헤더 패턴 사용
	Patt  string // 16진 패턴, X 는 반 바이트 와일드카드
	Allow bool
}

// ExtRow 는 확장자 허용 목록 응답[3005] 한 행이다. 판정에는 AllowUpload 만 쓴다(원본과 같다).
type ExtRow struct {
	Ext          string
	AFlag        int32
	Mode         int32
	Course       int32 // 0 인터넷망→, 1 업무망→, 2 양방향
	AllowUpload  int32
	AprvUserFlag int32
	AprvType     int32
}

type pattEntry struct {
	patt  string // 빈 값 = 헤더 없는 행
	allow bool
}

type pattAll struct {
	patt, ext string
	allow     bool
}

// Set 은 망 하나의 정책 한 벌이다. 만든 뒤에는 바뀌지 않는다 ― 응답이 오면 With* 로 그 부분만 바꾼 새 한 벌을 만든다.
// 검사 하나는 시작 때 잡은 한 벌로 끝까지 판정한다(원본 091).
type Set struct {
	Loc        int   // 1 업무망, 0 인터넷망
	FileSizeKB int64 // 로그인 응답 fm_file_size(KB). 0 = 제한 없음
	FileCount  int   // fm_file_count. 0 = 제한 없음
	ClipSizeKB int64 // fm_clip_size(KB). 0 = 제한 없음

	profile    map[string]string
	byPatt     []pattAll
	byName     map[string][]pattEntry
	extProfile map[string][]ExtRow
	extRows    []ExtRow // 정책 화면용(방향을 거른 행)

	HasProfile, HasExt, HasPatt bool
	PattResp, PattStored        int // 헤더 패턴 응답 건수와 버리지 않고 담은 행 수(로그·화면)
}

// NewSet 은 로그인 응답 값으로 빈 한 벌을 만든다. 정책을 받기 전에는 black·빈 표라 모든 파일이 막힌다(원본 POL-060).
func NewSet(loc int, fileSizeKB int64, fileCount int, clipSizeKB int64) *Set {
	return &Set{Loc: loc, FileSizeKB: fileSizeKB, FileCount: fileCount, ClipSizeKB: clipSizeKB,
		profile: map[string]string{}, byName: map[string][]pattEntry{}, extProfile: map[string][]ExtRow{}}
}

func (s *Set) clone() *Set {
	c := *s
	return &c
}

// Ready 는 정책 세 가지(프로파일·확장자 목록·헤더 패턴)를 다 받았는가다.
func (s *Set) Ready() bool { return s.HasProfile && s.HasExt && s.HasPatt }

// ParseProfile 은 권한 프로파일 'k=v,k=v' 를 읽는다. `=` 로 나눠 정확히 두 조각인 항목만 받는다(원본 setFmProfile2 ― `a=b=c` 는 버린다).
// 같은 키가 또 나오면 뒤 값이 이긴다.
func ParseProfile(p string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(p, ",") {
		parts := strings.Split(kv, "=")
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

// WithProfile 은 프로파일을 통째로 바꾼 새 한 벌이다.
func (s *Set) WithProfile(m map[string]string) *Set {
	c := s.clone()
	c.profile = make(map[string]string, len(m))
	for k, v := range m {
		c.profile[k] = v
	}
	c.HasProfile = true
	return c
}

// WithPatterns 는 헤더 패턴 표를 통째로 바꾼 새 한 벌이다(원본 setPattData).
// 확장자는 대문자로, 패턴은 대문자 16진과 X 로 맞춘다. 16진·X 밖 글자가 든 행은 warn 으로 알리고 버린다.
// 원본은 확장자를 7바이트·패턴을 127자로 잘랐지만 여기서는 자르지 않는다.
func (s *Set) WithPatterns(rows []PattRow, warn func(string)) *Set {
	c := s.clone()
	c.byPatt = nil
	c.byName = map[string][]pattEntry{}
	stored := 0
	for _, r := range rows {
		ext := strings.ToUpper(r.Ext)
		patt := ""
		if r.HFlag && r.Patt != "" {
			v := []byte(r.Patt)
			bad := -1
			for i, ch := range v {
				if (ch >= 'a' && ch <= 'f') || ch == 'x' {
					ch -= 'a' - 'A'
					v[i] = ch
				}
				if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || ch == 'X') {
					bad = i
					break
				}
			}
			if bad >= 0 {
				if warn != nil {
					warn("헤더패턴 행을 버린다 >>> 확장자: " + ext + ", 패턴: " + r.Patt + ", 글자: '" + string(r.Patt[bad]) + "'")
				}
				continue
			}
			patt = string(v)
			c.byPatt = append(c.byPatt, pattAll{patt: patt, ext: ext, allow: r.Allow})
		}
		c.byName[ext] = append(c.byName[ext], pattEntry{patt: patt, allow: r.Allow})
		stored++
	}
	c.HasPatt = true
	c.PattResp, c.PattStored = len(rows), stored
	return c
}

// WithExtInfo 는 확장자 허용 목록을 통째로 바꾼 새 한 벌이다. 이 망에서 보내는 방향(course == Loc)과 양방향(2) 행만 담는다.
func (s *Set) WithExtInfo(rows []ExtRow) *Set {
	c := s.clone()
	c.extProfile = map[string][]ExtRow{}
	c.extRows = nil
	for _, r := range rows {
		if int(r.Course) == s.Loc || r.Course == 2 {
			k := strings.ToUpper(r.Ext)
			c.extProfile[k] = append(c.extProfile[k], r)
			c.extRows = append(c.extRows, r)
		}
	}
	c.HasExt = true
	return c
}

// Value 는 프로파일 값이다. 없는 키는 ok 가 거짓(읽기만으로 키를 만들지 않는다).
func (s *Set) Value(k string) (string, bool) {
	v, ok := s.profile[k]
	return v, ok
}

// Int 는 프로파일 값을 정수로 읽는다. 없거나 숫자가 아니면 0(원본 QVariant::toInt).
func (s *Set) Int(k string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s.profile[k]))
	return n
}

// 판정 방식(원본 checkExtPattern 의 type).
const (
	MethodWhite = 0
	MethodBlack = 1
	MethodGray  = 2
)

// Method 는 ext_check_method 다. "white"·"gray" 는 그대로(대소문자 가림), 그 밖·없음은 black.
func (s *Set) Method() int {
	switch s.profile["ext_check_method"] {
	case "white":
		return MethodWhite
	case "gray":
		return MethodGray
	}
	return MethodBlack
}

// MethodName 은 화면용 이름이다.
func (s *Set) MethodName() string {
	return [...]string{"white", "black", "gray"}[s.Method()]
}

// PattResult 는 헤더 패턴 계층의 판정이다. 파일 검사 결과(Code)와 숫자가 달라 타입을 나눴다(원본 6.1 주의).
type PattResult int

const (
	PattOK        PattResult = 0
	PattNotReg    PattResult = -3 // 등록되지 않은 확장자 → Code -6
	PattNotAllow  PattResult = -4 // 허용되지 않은 확장자 헤더패턴 → Code -9
	PattRenamed   PattResult = -5 // 변경된 확장자 → Code -7
	PattNoMatched PattResult = -6 // 존재하지 않는 헤더패턴 → Code -8
)

// comparePatt 는 head(대문자 16진)가 patt 와 맞는가다. patt 길이만큼 대조하고 head 가 짧으면 다르다.
func comparePatt(head, patt string) bool {
	if len(head) < len(patt) {
		return false
	}
	for i := 0; i < len(patt); i++ {
		if patt[i] != 'X' && head[i] != patt[i] {
			return false
		}
	}
	return true
}

// allX 는 비어 있지 않고 전부 X 인가다 ― 이런 행은 이름 바꿈(-5) 판정에 쓰지 않는다.
func allX(p string) bool {
	return p != "" && strings.Trim(p, "X") == ""
}

// CheckExtPattern 은 확장자 ext 와 파일 앞부분 16진 head 로 헤더 패턴을 판정한다(원본 extpolicy.cpp 를 그대로 옮김).
func (s *Set) CheckExtPattern(ext, head string) PattResult {
	ext = strings.ToUpper(ext)
	switch s.Method() {
	case MethodWhite:
		return s.white(ext, head)
	case MethodGray:
		return s.gray(ext, head)
	}
	return s.black(ext, head)
}

func (s *Set) black(ext, head string) PattResult {
	rows, ok := s.byName[ext]
	if !ok || len(rows) == 0 {
		return PattNotReg
	}
	anyAllow := false
	for _, r := range rows {
		if !r.allow {
			continue
		}
		anyAllow = true
		if r.patt != "" {
			if comparePatt(head, r.patt) {
				return PattOK
			}
			continue
		}
		// 헤더 없는 허용 행: 다른 확장자 패턴과 맞으면 이름 바꾼 파일, 아니면 허용(그 자리에서 결론)
		for _, q := range s.byPatt {
			if comparePatt(head, q.patt) && !allX(q.patt) {
				return PattRenamed
			}
		}
		return PattOK
	}
	if !anyAllow {
		return PattNotAllow
	}
	return PattNoMatched
}

func (s *Set) white(ext, head string) PattResult {
	hasSame, renamed := false, false
	for _, q := range s.byPatt {
		if !comparePatt(head, q.patt) {
			continue
		}
		if q.ext == ext {
			if !q.allow {
				return PattNotAllow
			}
			hasSame = true
		} else if !allX(q.patt) {
			renamed = true
		}
	}
	for _, r := range s.byName[ext] {
		if r.patt == "" && r.allow {
			hasSame = true
		}
	}
	if hasSame {
		return PattOK
	}
	if renamed {
		return PattRenamed
	}
	return PattOK
}

func (s *Set) gray(ext, head string) PattResult {
	notAllow, other := false, false
	for _, q := range s.byPatt {
		if !comparePatt(head, q.patt) {
			continue
		}
		if q.ext == ext {
			if q.allow {
				return PattOK
			}
			notAllow = true
		} else if !allX(q.patt) {
			other = true
		}
	}
	if notAllow {
		return PattNotAllow
	}
	if other {
		return PattRenamed
	}
	return PattOK
}

// CheckExtProfile 은 확장자 허용 목록 판정이다. 그 확장자의 행 중 allow_upload==0 이 하나라도 있으면 거짓, 없거나 전부 허용이면 참.
func (s *Set) CheckExtProfile(ext string) bool {
	for _, r := range s.extProfile[strings.ToUpper(ext)] {
		if r.AllowUpload == 0 {
			return false
		}
	}
	return true
}

// SizeLimit 은 크기 상한(바이트)이다. 0 = 제한 없음.
func (s *Set) SizeLimit(clip bool) int64 {
	if clip {
		return s.ClipSizeKB * 1024
	}
	return s.FileSizeKB * 1024
}

// CheckSize 는 상한 판정이다. 상한이 크기보다 클 때만 통과 ― 같으면 막힌다(원본 checkSize).
func (s *Set) CheckSize(size int64, clip bool) bool {
	l := s.SizeLimit(clip)
	return l <= 0 || l > size
}

// InsideCheck 는 압축 내부를 검사하는가다. fm_chk_insidezip 이 없거나 1 이면 검사(원본 verifyUploadFile).
func (s *Set) InsideCheck() bool {
	if _, ok := s.profile["fm_chk_insidezip"]; !ok {
		return true
	}
	return s.Int("fm_chk_insidezip") == 1
}

// DepthFromPolicy 는 압축 중첩 깊이다. 없으면 2, 1~5 로 자른다. clamped 면 부르는 쪽이 경고를 남긴다.
func (s *Set) DepthFromPolicy() (depth int, clamped bool, raw string) {
	raw, ok := s.profile["fm_extract_zipdepth"]
	if !ok {
		return 2, false, ""
	}
	asked := s.Int("fm_extract_zipdepth")
	depth = min(max(asked, 1), 5)
	return depth, depth != asked, raw
}

// ProfileKV 는 프로파일 한 쌍이다(정책 화면).
type ProfileKV struct{ Key, Value string }

// Profile 은 프로파일을 키 순으로 돌려준다.
func (s *Set) Profile() []ProfileKV {
	out := make([]ProfileKV, 0, len(s.profile))
	for k, v := range s.profile {
		out = append(out, ProfileKV{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// PattView 는 헤더 패턴 표 한 행이다(정책 화면).
type PattView struct {
	Ext   string
	Patt  string
	Allow bool
}

// Patterns 는 담긴 헤더 패턴 행을 확장자 순으로 돌려준다.
func (s *Set) Patterns() []PattView {
	exts := make([]string, 0, len(s.byName))
	for e := range s.byName {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	var out []PattView
	for _, e := range exts {
		for _, r := range s.byName[e] {
			out = append(out, PattView{e, r.patt, r.allow})
		}
	}
	return out
}

// ExtRows 는 이 망에 적용되는 확장자 허용 목록 행이다(정책 화면).
func (s *Set) ExtRows() []ExtRow { return s.extRows }
