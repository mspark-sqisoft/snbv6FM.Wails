// Package domain 은 도메인 정책을 hosts 파일의 ssBridge 전용 구역에 반영한다(원본 src/domain, docs/port/07 7.9).
//
// hosts 는 바이트 그대로 다룬다 ― 글자로 읽어 다시 쓰면 메모장(ANSI)으로 단 한글 주석 같은 다른 줄이 깨진다. 줄 끝도 원본 그대로 둔다.
// 구역의 모양: <빈 줄> · 시작 표식 · 주의 표식 · "IP\t이름\t\t#설명"(정책 수만큼) · 끝 표식.
package domain

import (
	"bytes"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"snbapp/internal/nbs"
	"snbapp/pb/commonservice"
)

const (
	startMarker   = "# 본영역은 ssBridge의 도메인 정책이 등록됩니다."
	cautionMarker = "# 삭제될 수 있는 영역이므로 다른 내용을 넣지 마세요."
	endMarker     = "# ssBridge 도메인 정책 등록 영역 끝"
)

type line struct{ raw, text []byte }

func splitLines(data []byte) []line {
	var out []line
	for len(data) > 0 {
		end := len(data)
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			end = i + 1
		}
		l := line{raw: data[:end]}
		l.text = bytes.TrimSuffix(bytes.TrimSuffix(l.raw, []byte("\n")), []byte("\r"))
		out = append(out, l)
		data = data[end:]
	}
	return out
}

func isMarker(l line, m string) bool { return string(l.text) == m }

// isPolicyRow 는 우리가 쓰는 정책 줄의 모양이다: 주석이 아니고 "\t\t#" 이 있다.
func isPolicyRow(t []byte) bool {
	return !bytes.HasPrefix(t, []byte("#")) && bytes.Contains(t, []byte("\t\t#"))
}

// HasBlock 은 시작 표식이 있는가다(반쪽 구역도 센다).
func HasBlock(hosts []byte) bool {
	for _, l := range splitLines(hosts) {
		if isMarker(l, startMarker) {
			return true
		}
	}
	return false
}

// RemoveBlocks 는 우리 구역을 모두 지운 결과다. 구역 바로 앞의 빈 줄 하나(붙일 때 넣은 것)도 함께 지우고, 다른 줄은 바이트 그대로다.
// partial 은 끝 표식 없이 지운 반쪽 구역이 있었는가다 ― 반쪽은 우리 모양의 줄까지만 지워 뒤의 남의 줄을 남긴다.
func RemoveBlocks(hosts []byte) (out []byte, partial bool) {
	lines := splitLines(hosts)
	var kept []line
	for i := 0; i < len(lines); i++ {
		if !isMarker(lines[i], startMarker) {
			kept = append(kept, lines[i])
			continue
		}
		// 다음 시작 표식 전에 끝 표식이 있으면 그 사이는 전부 우리 구역이다(안내문이 "삭제될 수 있는 영역" 이라 적는다)
		end := -1
		for k := i + 1; k < len(lines) && !isMarker(lines[k], startMarker); k++ {
			if isMarker(lines[k], endMarker) {
				end = k
				break
			}
		}
		if end >= 0 {
			i = end
		} else {
			partial = true
			j := i + 1
			for j < len(lines) && (isMarker(lines[j], cautionMarker) || isPolicyRow(lines[j].text)) {
				j++
			}
			i = j - 1
		}
		if n := len(kept); n > 0 && len(kept[n-1].text) == 0 {
			kept = kept[:n-1] // 붙일 때 넣은 빈 줄
		}
	}
	out = make([]byte, 0, len(hosts))
	for _, l := range kept {
		out = append(out, l.raw...)
	}
	return out, partial
}

// AppendBlock 은 구역을 지운 hosts 끝에 새 구역을 붙인다. 줄 끝은 원본이 쓰는 것을 따른다(줄바꿈이 없으면 Windows 는 CRLF).
func AppendBlock(hosts []byte, rows [][]byte) []byte {
	first := bytes.IndexByte(hosts, '\n')
	crlf := first > 0 && hosts[first-1] == '\r'
	if first < 0 && runtime.GOOS == "windows" {
		crlf = true
	}
	eol := []byte("\n")
	if crlf {
		eol = []byte("\r\n")
	}
	out := bytes.Clone(hosts)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, eol...)
	}
	out = append(out, eol...) // 구역 앞 빈 줄
	add := func(b []byte) { out = append(append(out, b...), eol...) }
	add([]byte(startMarker))
	add([]byte(cautionMarker))
	for _, r := range rows {
		add(r)
	}
	add([]byte(endMarker))
	return out
}

// Info 는 도메인 정책 한 행이다.
type Info struct{ Profile, Name, IP string }

var (
	ipCharsRe  = regexp.MustCompile(`^[0-9A-Fa-f:.]+$`)
	hostNameRe = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
)

// Rows 는 서버 값을 검사해 hosts 줄로 만든다. 형식이 틀린 행은 건너뛰고 skip 에 알린다 ―
// 줄바꿈이 섞이면 관리 구역 밖에 해제로 지워지지 않는 줄이 남아 PC 전체의 이름 풀이가 바뀐다.
func Rows(list []Info, skip func(ip, name string)) [][]byte {
	var rows [][]byte
	for _, d := range list {
		ip, name := strings.TrimSpace(d.IP), strings.TrimSpace(d.Name)
		_, err := netip.ParseAddr(ip)
		if !ipCharsRe.MatchString(ip) || err != nil || !hostNameRe.MatchString(name) {
			if skip != nil {
				skip(ip, name)
			}
			continue
		}
		profile := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, d.Profile)
		rows = append(rows, []byte(ip+"\t"+name+"\t\t#"+profile))
	}
	return rows
}

// Fetch 는 [3230] 도메인 정책을 받는다.
func Fetch(s *nbs.Session) ([]Info, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Common().GetDomainPolicy(ctx, &commonservice.DomainPolicyRequest{Userid: s.UserID})
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(res.List))
	for _, d := range res.List {
		out = append(out, Info{Profile: d.ProfileName, Name: d.DomainName, IP: d.DomainIp})
	}
	return out, nil
}

// ErrNoDir 는 hosts 자리를 찾지 못했을 때다.
var ErrNoDir = errors.New("hosts 디렉토리를 찾는데 실패했습니다.")

// Apply 는 남아 있던 구역을 지우고 새 구역을 한 번에 쓴다. partial 은 끝 표식 없는 옛 구역을 지웠는가다.
func Apply(path string, rows [][]byte) (partial bool, err error) {
	hosts, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	hosts, partial = RemoveBlocks(hosts)
	return partial, write(path, AppendBlock(hosts, rows))
}

// Remove 는 우리 구역만 지운다. 구역이 없으면 쓰지 않는다.
func Remove(path string) (partial bool, err error) {
	hosts, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !HasBlock(hosts) {
		return false, nil
	}
	cleaned, partial := RemoveBlocks(hosts)
	return partial, write(path, cleaned)
}

// write 는 임시 파일에 다 쓴 뒤 한 번에 바꿔치기한다 ― 쓰는 중에 끊겨도 hosts 가 비지 않는다.
// 폴더에는 못 쓰고 hosts 에만 쓰기 권한을 준 PC 가 있어 임시 파일을 못 만들면 직접 쓴다.
func write(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "hosts*.tmp")
	if err != nil {
		return os.WriteFile(path, data, mode)
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Chmod(name, mode)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
		return os.WriteFile(path, data, mode)
	}
	return nil
}
