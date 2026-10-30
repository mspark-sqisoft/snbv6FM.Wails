package policy

// 원본 tests/tst_FileMoverPolicy.cpp·tst_ZipExtractor.cpp(depthFromPolicy·checkExtension)·tst_ZipPasswordGate.cpp 의 표를 옮겼다.

import (
	"strings"
	"testing"
)

func setWith(profile string) *Set {
	return NewSet(1, 0, 0, 0).WithProfile(ParseProfile(profile))
}

// pattRows 는 원본 표의 "확장자|헤더 사용|패턴|허용" 글자를 행으로 바꾼다.
func pattRows(rows ...string) []PattRow {
	var out []PattRow
	for _, r := range rows {
		f := strings.Split(r, "|")
		out = append(out, PattRow{Ext: f[0], HFlag: f[1] == "1", Patt: f[2], Allow: f[3] == "1"})
	}
	return out
}

func TestParseProfile(t *testing.T) {
	m := ParseProfile("fm_chk_insidezip=1,fm_use_zip_pwd=0,ext_check_method=white")
	if len(m) != 3 || m["fm_chk_insidezip"] != "1" || m["ext_check_method"] != "white" {
		t.Fatalf("parse: %v", m)
	}
	m = ParseProfile("good=1,broken,alsobad=2=3,another=4") // '=' 가 없거나 둘 이상이면 버린다
	if len(m) != 2 || m["good"] != "1" || m["another"] != "4" {
		t.Fatalf("malformed: %v", m)
	}
	if m := ParseProfile("key=first,key=second"); len(m) != 1 || m["key"] != "second" {
		t.Fatalf("last wins: %v", m)
	}
	if m := ParseProfile(""); len(m) != 0 {
		t.Fatalf("empty: %v", m)
	}
	s := setWith("old=1,stale=2").WithProfile(ParseProfile("fresh=9")) // 다시 받으면 앞 값이 남지 않는다
	if _, ok := s.Value("old"); ok {
		t.Fatal("old kept")
	}
	if _, ok := s.Value("없는키"); ok || len(s.Profile()) != 1 { // 읽기만으로 키를 만들지 않는다
		t.Fatal("missing key")
	}
}

func TestCheckSize(t *testing.T) {
	s := NewSet(1, 0, 0, 0)
	if !s.CheckSize(1<<40, false) || !s.CheckSize(1<<40, true) {
		t.Fatal("no limit must allow")
	}
	s = NewSet(1, 100, 5, 10) // 100KB, 10KB
	for _, c := range []struct {
		size int64
		ok   bool
	}{{0, true}, {102399, true}, {102400, false}, {102401, false}, {999999, false}} {
		if got := s.CheckSize(c.size, false); got != c.ok {
			t.Errorf("size %d: got %v", c.size, got)
		}
	}
	if !s.CheckSize(10239, true) || s.CheckSize(10240, true) || !s.CheckSize(20000, false) || s.CheckSize(20000, true) {
		t.Fatal("clip limit")
	}
	if s.SizeLimit(false) != 100*1024 || s.SizeLimit(true) != 10*1024 {
		t.Fatal("KB*1024")
	}
}

func TestCheckExtProfile(t *testing.T) {
	s := NewSet(1, 0, 0, 0)
	if !s.CheckExtProfile("TXT") {
		t.Fatal("unregistered allowed")
	}
	s = s.WithExtInfo([]ExtRow{{Ext: "PDF", AllowUpload: 1, Course: 1}, {Ext: "EXE", AllowUpload: 0, Course: 1},
		{Ext: "ZIP", AllowUpload: 1, Course: 1}, {Ext: "ZIP", AllowUpload: 0, Course: 1}})
	if !s.CheckExtProfile("PDF") || s.CheckExtProfile("EXE") || s.CheckExtProfile("ZIP") {
		t.Fatal("allow/block/block-wins")
	}
	for _, c := range [][2]string{{"EXE", "EXE"}, {"EXE", "exe"}, {"exe", "EXE"}, {"Exe", "exe"}} {
		if NewSet(1, 0, 0, 0).WithExtInfo([]ExtRow{{Ext: c[0], Course: 2}}).CheckExtProfile(c[1]) {
			t.Errorf("case %v", c)
		}
	}
	// 반대 방향 행은 무시한다(⑥)
	rows := []ExtRow{{Ext: "INB", Course: 0}, {Ext: "OUT", Course: 1}, {Ext: "ALL", Course: 2}}
	in, out := NewSet(1, 0, 0, 0).WithExtInfo(rows), NewSet(0, 0, 0, 0).WithExtInfo(rows)
	if !in.CheckExtProfile("INB") || in.CheckExtProfile("OUT") || in.CheckExtProfile("ALL") {
		t.Fatal("inner course")
	}
	if out.CheckExtProfile("INB") || !out.CheckExtProfile("OUT") || out.CheckExtProfile("ALL") {
		t.Fatal("outer course")
	}
}

func TestCheckExtPatternModes(t *testing.T) {
	for _, m := range []string{"black", "white", "gray"} { // 대소문자가 달라도 같은 확장자의 차단 행
		for _, c := range [][2]string{{"exe", "EXE"}, {"EXE", "exe"}} {
			s := setWith("ext_check_method="+m).WithPatterns([]PattRow{{Ext: c[0], HFlag: true, Patt: "4D5A"}}, nil)
			if got := s.CheckExtPattern(c[1], "4D5A90"); got != PattNotAllow {
				t.Errorf("%s %v: %d", m, c, got)
			}
		}
	}
	for _, c := range []struct {
		profile string
		want    PattResult
	}{{"ext_check_method=white", 0}, {"ext_check_method=gray", 0}, {"ext_check_method=black", -3}, {"", -3}, {"ext_check_method=이상한값", -3}} {
		if got := setWith(c.profile).CheckExtPattern("UNKNOWN", "ABCD"); got != c.want {
			t.Errorf("routing %q: %d", c.profile, got)
		}
	}
	cases := []struct {
		name, method string
		rows         []PattRow
		ext, head    string
		want         PattResult
	}{
		{"black-unregistered", "black", pattRows("PDF|1|25504446|1"), "EXE", "4D5A", -3},
		{"black-notallowed", "black", pattRows("EXE|1|4D5A|0"), "EXE", "4D5A", -4},
		{"black-noheader-empty", "black", pattRows("TXT|0||1"), "TXT", "", 0},
		{"black-noheader-hello", "black", pattRows("TXT|0||1"), "TXT", "68656C6C6F", 0},
		{"black-match", "black", pattRows("PDF|1|25504446|1"), "PDF", "255044462D312E37", 0},
		{"black-mismatch", "black", pattRows("PDF|1|25504446|1"), "PDF", "4D5A9000", -6},
		{"black-wild-match", "black", pattRows("DAT|1|xxAB|1"), "DAT", "12AB", 0},
		{"black-wild-mismatch", "black", pattRows("DAT|1|xxAB|1"), "DAT", "12CD", -6},
		{"white-same-allow", "white", pattRows("PDF|1|25504446|1"), "PDF", "255044462D312E37", 0},
		{"white-same-deny", "white", pattRows("EXE|1|4D5A|0"), "EXE", "4D5A90", -4},
		{"white-renamed", "white", pattRows("EXE|1|4D5A|1"), "TXT", "4D5A90", -5},
		{"gray-same-allow", "gray", pattRows("PDF|1|25504446|1"), "PDF", "255044462D312E37", 0},
		{"gray-same-deny", "gray", pattRows("EXE|1|4D5A|0"), "EXE", "4D5A90", -4},
		{"gray-renamed", "gray", pattRows("EXE|1|4D5A|1"), "TXT", "4D5A90", -5},
	}
	for _, c := range cases {
		s := setWith("ext_check_method="+c.method).WithPatterns(c.rows, nil)
		if got := s.CheckExtPattern(c.ext, c.head); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

// 원본 headerPattern_compare(100 SC-001) 표 그대로.
func TestHeaderPatternCompare(t *testing.T) {
	const J, M = "FFD8FFE1123445786966", "4D5A9000"
	const warn = "헤더패턴 행을 버린다 >>> 확장자: EXE, 패턴: 4D5Z, 글자: 'Z'"
	cases := []struct {
		name, method string
		rows         []string
		ext, in      string
		want         PattResult
		warning      string
		stored       int
	}{
		{"01-black-upperX", "black", []string{"JPEG|1|FFD8FFE1XXXX4578|1"}, "JPEG", J, 0, "", -1},
		{"02-white-upperX", "white", []string{"EXE|1|FFD8FFE1XXXX4578|0"}, "EXE", J, -4, "", -1},
		{"03-gray-upperX", "gray", []string{"JPG|1|FFD8FFE1XXXX4578|1"}, "TXT", J, -5, "", -1},
		{"04-black-lower", "black", []string{"JPEG|1|ffd8ffe1xxxx4578|1"}, "JPEG", J, 0, "", -1},
		{"05-black-short", "black", []string{"EXE|1|4D5A9000|1"}, "EXE", "4D5A", -6, "", -1},
		{"06-white-short", "white", []string{"EXE|1|4D5A9000|1"}, "TXT", "4D5A", 0, "", -1},
		{"07-gray-short", "gray", []string{"EXE|1|4D5A9000|1"}, "TXT", "4D5A", 0, "", -1},
		{"08-white-xdeny", "white", []string{"EXE|1|xx5A90|0"}, "EXE", M, -4, "", -1},
		{"09-white-xrename", "white", []string{"EXE|1|xx5A90|1"}, "TXT", M, -5, "", -1},
		{"10-gray-xdeny", "gray", []string{"EXE|1|xx5A90|0"}, "EXE", M, -4, "", -1},
		{"11-gray-xrename", "gray", []string{"EXE|1|xx5A90|1"}, "TXT", M, -5, "", -1},
		{"12-black-Xrename", "black", []string{"TXT|0||1", "EXE|1|XX5A90|1"}, "TXT", M, -5, "", -1},
		{"13-black-allx", "black", []string{"TXT|0||1", "DAT|1|xxxx|1"}, "TXT", M, 0, "", -1},
		{"14-white-allx", "white", []string{"DAT|1|xxxx|1"}, "TXT", M, 0, "", -1},
		{"14-gray-allx", "gray", []string{"DAT|1|xxxx|1"}, "TXT", M, 0, "", -1},
		{"15-black-empty", "black", []string{"EXE|1||1", "PDF|1|25504446|1"}, "EXE", "255044462D312E37", -5, "", -1},
		{"16-white-empty", "white", []string{"EXE|1|4D5A90|1", "TXT|1||1"}, "TXT", M, 0, "", -1},
		{"17-gray-empty", "gray", []string{"TXT|1||1", "EXE|1|4D5A90|1"}, "TXT", M, -5, "", -1},
		{"18-black-badrow", "black", []string{"EXE|1|4D5Z|1"}, "EXE", "4D5A", -3, warn, -1},
		{"19-black-badkeep", "black", []string{"EXE|1|4D5Z|1", "EXE|1|4D5A|1"}, "EXE", "4D5A", 0, warn, 1},
		{"20-black-odd-long", "black", []string{"EXE|1|4D5|1"}, "EXE", "4D5A90", 0, "", -1},
		{"20-black-odd-short", "black", []string{"EXE|1|4D5|1"}, "EXE", "4D", -6, "", -1},
	}
	for _, c := range cases {
		var warns []string
		s := setWith("ext_check_method="+c.method).WithPatterns(pattRows(c.rows...), func(w string) { warns = append(warns, w) })
		if got := s.CheckExtPattern(c.ext, c.in); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
		if c.warning != "" && (len(warns) != 1 || warns[0] != c.warning) {
			t.Errorf("%s: warnings %v", c.name, warns)
		}
		if c.warning == "" && len(warns) != 0 {
			t.Errorf("%s: unexpected warnings %v", c.name, warns)
		}
		if c.stored >= 0 && s.PattStored != c.stored {
			t.Errorf("%s: stored %d", c.name, s.PattStored)
		}
	}
}

// 다시 받으면 통째로 바뀐다(①②③) ― 한 벌은 바뀌지 않으므로 앞 벌로 하던 검사도 그대로다.
func TestReloadReplaces(t *testing.T) {
	a := setWith("ext_check_method=black").WithPatterns(pattRows("EXE|1|4D5A|1"), nil)
	b := a.WithPatterns(pattRows("EXE|1|4D5A|0"), nil)
	if a.CheckExtPattern("EXE", "4D5A") != 0 || b.CheckExtPattern("EXE", "4D5A") != -4 {
		t.Fatal("reload patterns")
	}
	c := b.WithPatterns(pattRows("A|1|01|1", "B|1|02|1", "C|0||1"), nil)
	if d := c.WithPatterns(pattRows("A|1|01|1", "B|1|02|1", "C|0||1"), nil); len(d.Patterns()) != 3 || len(c.Patterns()) != 3 {
		t.Fatal("same twice keeps rows")
	}
	e := NewSet(1, 0, 0, 0).WithExtInfo([]ExtRow{{Ext: "X", Course: 2}})
	if e.CheckExtProfile("X") || !e.WithExtInfo([]ExtRow{{Ext: "X", Course: 2, AllowUpload: 1}}).CheckExtProfile("X") {
		t.Fatal("reload ext profile")
	}
	if NewSet(1, 0, 0, 0).Ready() || !NewSet(1, 0, 0, 0).WithProfile(nil).WithExtInfo(nil).WithPatterns(nil, nil).Ready() {
		t.Fatal("ready")
	}
}

func TestDepthFromPolicy(t *testing.T) {
	for _, c := range []struct {
		profile string
		depth   int
		clamped bool
	}{{"", 2, false}, {"fm_extract_zipdepth=0", 1, true}, {"fm_extract_zipdepth=1", 1, false},
		{"fm_extract_zipdepth=5", 5, false}, {"fm_extract_zipdepth=9", 5, true}, {"fm_extract_zipdepth=3", 3, false}} {
		d, cl, _ := setWith(c.profile).DepthFromPolicy()
		if d != c.depth || cl != c.clamped {
			t.Errorf("%q: %d %v", c.profile, d, cl)
		}
	}
}

func TestInsideCheck(t *testing.T) {
	for p, want := range map[string]bool{"": true, "fm_chk_insidezip=1": true, "fm_chk_insidezip=0": false, "fm_chk_insidezip=2": false} {
		if setWith(p).InsideCheck() != want {
			t.Errorf("%q", p)
		}
	}
}

func TestZipAction(t *testing.T) {
	for _, d := range []int{0, 1, 2} {
		for _, u := range []int{0, 1} {
			for _, auto := range []bool{false, true} {
				var want ZipPwAction
				switch {
				case d == 1:
					want = ZipPwPass
				case u == 1 && auto:
					want = ZipPwBlockAuto
				case u == 1:
					want = ZipPwAsk
				default:
					want = ZipPwBlock
				}
				if got := ZipAction(d, u, auto); got != want {
					t.Errorf("d%d-u%d-auto%v: %d", d, u, auto, got)
				}
			}
		}
	}
}

func TestIsArchiveName(t *testing.T) {
	for name, want := range map[string]bool{"a.zip": true, "a.ZIP": true, "a.ZiP": true, "a.zipx": true, "a.tar": true, "a.gz": true,
		"a.lzh": true, "a.7z": true, "a.xz": true, "a.tgz": true, "a.bz2": true, // tgz·bz2 는 Wails 판에서 더했다(원본은 bz2 거짓)
		"a.txt": false, "README": false, ".zip": false, "": false, "a.tar.gz": true, "a.zip.txt": false, "a.": false} {
		if IsArchiveName(name) != want {
			t.Errorf("%q", name)
		}
	}
}

func TestSizeTextAndDetail(t *testing.T) {
	for n, want := range map[int64]string{0: "0 Byte", 1: "1 byte", 1024: "1024 byte", 1025: "1.0 KB", 1048577: "1.00 MB", 1<<30 + 1: "1.00 GB"} {
		if got := SizeText(n); got != want {
			t.Errorf("%d: %q", n, got)
		}
	}
	if Detail(NotAllowed, "A.EXE", 1, 0) != ".exe" || Detail(SizeOver, "a", 2048, 1024) != "2.0 KB / 상한 1024 byte" || Detail(Renamed, "a.b", 1, 0) != "" {
		t.Fatal("detail")
	}
	if HexHead([]byte{0x4d, 0x5a}) != "4D5A" || len(HexHead(make([]byte, 100))) != 128 {
		t.Fatal("hexhead")
	}
}

func TestCheckEntryOrder(t *testing.T) {
	s := setWith("ext_check_method=black,no_ext_no_permit=1").WithPatterns(pattRows("TXT|0||1", "EXE|1|4D5A|0"), nil).
		WithExtInfo([]ExtRow{{Ext: "TXT", Course: 2, AllowUpload: 0}})
	s.FileSizeKB = 1
	for _, c := range []struct {
		name string
		size int64
		head []byte
		want Code
	}{
		{"a.txt", 0, []byte("x"), ZeroSize},
		{"a.txt", 5, nil, OpenFail},
		{"README", 5, []byte("x"), NoExt},
		{"a.txt", 1024, []byte("x"), SizeOver}, // 상한과 같으면 막힌다
		{"a.exe", 5, []byte{0x4d, 0x5a}, PattNotAllowed},
		{"a.pdf", 5, []byte("%PDF"), NotRegistered},
		{"a.txt", 5, []byte("hello"), NotAllowed},
	} {
		if got := CheckEntry(s, c.name, c.size, c.head); got != c.want {
			t.Errorf("%s/%d: %d want %d", c.name, c.size, got, c.want)
		}
	}
	if m, ok := NewMismatch(Renamed, "C:/x/a.txt", []byte{0x4d}); !ok || m.Ext != "TXT" || m.Patt != "4D" || m.Desc != "Renamed pattern" {
		t.Fatalf("mismatch %+v", m)
	}
	if _, ok := NewMismatch(Canceled, "a", nil); ok {
		t.Fatal("canceled recorded")
	}
}

func TestMismatchRequests(t *testing.T) {
	list := make([]Mismatch, 12)
	list[0].Fname = strings.Repeat("가", 200) // 600 바이트 → 500 바이트 안의 글자 경계
	reqs := MismatchRequests("u1", list)
	if len(reqs) != 3 || reqs[0].MCount != 5 || reqs[2].MCount != 2 || reqs[0].UserId != "u1" {
		t.Fatalf("batches %d", len(reqs))
	}
	if f := reqs[0].List[0].Fname; len(f) != 498 || !strings.HasPrefix(f, "가") {
		t.Fatalf("cut %d", len(f))
	}
}
