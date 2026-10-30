package files

// 원본 단위 시험(tests/tst_DownloadBlock·tst_FileMoverPolicy getState_*·tst_FileMoverUtil·tst_DownloadConflict·
// tst_AutoDownloadTarget·tst_ChecksumVerdict)의 사례를 표로 옮겼다.

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"snbapp/pb/serverfile"
)

const here, there = 1, 0 // 지금 접속한 망, 다른 망

func ok() *serverfile.FileInfo {
	return &serverfile.FileInfo{NameStr: "a.pdf", State: 1, VirusFlag: 1, PrivState: 1, Course: there}
}

func TestClassify(t *testing.T) {
	deny := map[string]string{"down_same_network": "0"}
	allow := map[string]string{"down_same_network": "1"}
	cases := []struct {
		name string
		edit func(*serverfile.FileInfo)
		prof map[string]string
		want Block
	}{
		{"받을 수 있다", func(*serverfile.FileInfo) {}, deny, BlockNone},
		{"클립보드", func(f *serverfile.FileInfo) { f.NameStr = "20260911__CLIP__.txt" }, deny, BlockClipboard},
		{"동일망 정책이 막음", func(f *serverfile.FileInfo) { f.Course = here }, deny, BlockSameNetwork},
		{"동일망 키 없음도 막음(toInt 0)", func(f *serverfile.FileInfo) { f.Course = here }, nil, BlockSameNetwork},
		{"동일망 허용", func(f *serverfile.FileInfo) { f.Course = here }, allow, BlockNone},
		{"virus -1", func(f *serverfile.FileInfo) { f.VirusFlag = -1 }, deny, BlockNotReady},
		{"virus 0", func(f *serverfile.FileInfo) { f.VirusFlag = 0 }, deny, BlockVirusPending},
		{"virus 2", func(f *serverfile.FileInfo) { f.VirusFlag = 2 }, deny, BlockNotReady},
		{"virus 3", func(f *serverfile.FileInfo) { f.VirusFlag = 3 }, deny, BlockVirusPending},
		{"virus 4", func(f *serverfile.FileInfo) { f.VirusFlag = 4 }, deny, BlockNone},
		{"virus 5", func(f *serverfile.FileInfo) { f.VirusFlag = 5 }, deny, BlockNotReady},
		{"virus 21", func(f *serverfile.FileInfo) { f.VirusFlag = 21 }, deny, BlockNotReady},
		{"virus 22", func(f *serverfile.FileInfo) { f.VirusFlag = 22 }, deny, BlockNotReady},
		{"결재 대기", func(f *serverfile.FileInfo) { f.State = 0 }, deny, BlockNotReady},
		{"반려", func(f *serverfile.FileInfo) { f.State = 2 }, deny, BlockNotReady},
		{"사후승인(3)은 받는다 ― 옛 Wails 판 결함", func(f *serverfile.FileInfo) { f.State = 3 }, deny, BlockNone},
		{"결재요청 가능", func(f *serverfile.FileInfo) { f.State = 4 }, deny, BlockNotReady},
		{"개인정보 검출", func(f *serverfile.FileInfo) { f.PrivState = 2 }, deny, BlockNotReady},
		{"순서: 클립보드 > 동일망", func(f *serverfile.FileInfo) { f.NameStr = "x__CLIP__"; f.Course = here }, deny, BlockClipboard},
		{"순서: 동일망 > 바이러스", func(f *serverfile.FileInfo) { f.Course = here; f.VirusFlag = 0 }, deny, BlockSameNetwork},
		{"순서: 바이러스 > 결재", func(f *serverfile.FileInfo) { f.VirusFlag = 0; f.State = 0 }, deny, BlockVirusPending},
		{"폴더도 받는다(풀기는 전송 영역)", func(f *serverfile.FileInfo) { f.DirFlag = 1 }, deny, BlockNone},
	}
	for _, c := range cases {
		f := ok()
		c.edit(f)
		if got := Classify(f, here, c.prof); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
		if (DownloadBlock(f, here, c.prof) == "") != (c.want == BlockNone) {
			t.Errorf("%s: DownloadBlock 글자와 판정이 어긋난다", c.name)
		}
	}
}

func TestGetState(t *testing.T) {
	for _, v := range []int32{-1, 0, 2, 3, 5, 6, 21, 22, 99} { // 바이러스가 먼저 이긴다
		if s := GetState(1, 1, v); s.Code != -1 || s.Text == "" {
			t.Errorf("virus %d: %+v", v, s)
		}
	}
	for _, c := range []struct{ priv, code int32 }{{2, -1}, {0, -1}, {-1, -1}, {1, 1}, {3, 1}, {99, -1}} {
		if s := GetState(1, c.priv, 1); int32(s.Code) != c.code {
			t.Errorf("priv %d: %+v", c.priv, s)
		}
	}
	for _, c := range []struct{ aprv, code int32 }{{0, 0}, {1, 1}, {2, 2}, {3, 1}, {4, 4}, {99, -1}} {
		if s := GetState(c.aprv, 1, 1); int32(s.Code) != c.code || s.Text == "" {
			t.Errorf("aprv %d: %+v", c.aprv, s)
		}
	}
	if s := GetState(1, 1, 6); s.Text != "OLE 검출 파일 차단" || s.Kind != KindDanger {
		t.Errorf("OLE: %+v", s)
	}
	// 코드 1 과 「다운로드 가능」은 정확히 같은 경우다
	for _, v := range []int32{-1, 0, 1, 2, 3, 4, 5, 6, 21, 22, 99} {
		for _, p := range []int32{-1, 0, 1, 2, 3, 99} {
			for _, a := range []int32{0, 1, 2, 3, 4, 99} {
				s := GetState(a, p, v)
				if (s.Code == 1) != (s.Text == "다운로드 가능") || (s.Code == 1) != (s.Kind == KindSuccess) {
					t.Errorf("virus=%d priv=%d aprv=%d → %+v", v, p, a, s)
				}
			}
		}
	}
}

func TestFileStatusSameNetwork(t *testing.T) {
	f := ok()
	f.Course = here
	if s := FileStatus(f, here, nil); s.Text != "동일망 다운로드 불가" || s.Code != -1 || s.Kind != KindWarning {
		t.Errorf("동일망: %+v", s)
	}
	if s := FileStatus(f, here, map[string]string{"down_same_network": "1"}); s.Code != 1 {
		t.Errorf("허용: %+v", s)
	}
	f.State = 0 // 다운로드 가능이 아니면 바꾸지 않는다
	if s := FileStatus(f, here, nil); s.Text != "결재 대기중" {
		t.Errorf("대기: %+v", s)
	}
}

func TestSizeText(t *testing.T) {
	for _, c := range []struct {
		n    uint64
		want string
	}{
		{0, "0 Byte"}, {1, "1 byte"}, {1023, "1023 byte"}, {1024, "1024 byte"}, {1025, "1.0 KB"},
		{1048576, "1024.0 KB"}, {1048577, "1.00 MB"}, {1073741824, "1024.00 MB"}, {1073741825, "1.00 GB"}, {10737418240, "10.00 GB"},
	} {
		if got := SizeText(c.n); got != c.want {
			t.Errorf("%d: %q, want %q", c.n, got, c.want)
		}
	}
}

func TestPrivDetailText(t *testing.T) {
	for _, c := range []struct{ in, srv, want string }{
		{"", "s", NoDetection},
		{"/data/a.txt", "/data/a.txt", NoDetection},
		{"[SI]주민번호(3)case detected\n전화번호()\n카드번호(0)", "s", "주민번호(3)"},
		{"주민번호(0)", "s", "주민번호(0)"}, // 다 버려지면 다듬기 전 글자
		{"  앞뒤 공백  \n", "s", "앞뒤 공백"},
	} {
		if got := PrivDetailText(c.in, c.srv); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  int32
		want string
	}{
		{`C:\a\b\보고서.pdf`, 0, "보고서.pdf"}, {"/x/y.zip", 0, "y.zip"}, {"사진모음.zip", 1, "사진모음"},
		{"a.zip.b.ZIP", 1, "a.zip.b"}, {"zip", 1, "zip"}, {".zip", 1, ".zip"},
	} {
		if got := DisplayName(&serverfile.FileInfo{NameStr: c.name, DirFlag: c.dir}); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitizeFor("windows", `a?b\c/d*e:f"g<h>i|j`+"\x01"); got != "a b c d e f g h i j " {
		t.Errorf("windows: %q", got)
	}
	if got := sanitizeFor("linux", `a:b/c`+"\x00"); got != "a:b c " {
		t.Errorf("linux: %q", got)
	}
	for _, bad := range []string{"", ".", "..", " ", ".snb_write_a.txt.snbpart", ".SNB_WRITE_A.TXT"} {
		if _, err := SavePath("d", bad); err == nil {
			t.Errorf("%q 를 받았다", bad)
		}
	}
	if p, _ := SavePath("d", "../x"); p != filepath.Join("d", ".. x") {
		t.Errorf("경로 이탈: %q", p)
	}
}

func existsIn(paths ...string) func(string) bool {
	m := map[string]bool{}
	for _, p := range paths {
		m[filepath.Clean(p)] = true
	}
	return func(p string) bool { return m[filepath.Clean(p)] }
}

func TestDuplicates(t *testing.T) {
	at := func(names ...string) []Candidate {
		var c []Candidate
		for _, n := range names {
			c = append(c, Candidate{"D:/down", n})
		}
		return c
	}
	p := func(n string) string { return filepath.Join("D:/down", n) }
	colon := p("a b.pdf")
	if runtime.GOOS != "windows" {
		colon = p("a:b.pdf")
	}
	cases := []struct {
		name   string
		cands  []Candidate
		exists func(string) bool
		want   string
	}{
		{"겹침 없음", at("a.pdf", "b.pdf"), existsIn(p("c.pdf")), "[]"},
		{"겹치는 것만 순서대로", at("a.pdf", "b.pdf", "c.pdf"), existsIn(p("a.pdf"), p("c.pdf")), "[0 2]"},
		{"같은 요청 안 동명은 한 번", at("a.pdf", "a.pdf"), existsIn(p("a.pdf")), "[0]"},
		{"디스크에 없으면 동명이라도 안 묻는다", at("a.pdf", "a.pdf"), existsIn(), "[]"},
		{"금지 글자 정리 뒤 비교", at("a:b.pdf"), existsIn(colon), "[0]"},
		{"빈 목록", nil, existsIn(p("a.pdf")), "[]"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(Duplicates(c.cands, c.exists)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestNumberedPath(t *testing.T) {
	d := t.TempDir()
	p := func(n string) string { return filepath.Join(d, n) }
	cases := []struct {
		in, want string
		taken    []string
	}{
		{"보고서.txt", "보고서_00000.txt", nil},
		{"보고서.txt", "보고서_00002.txt", []string{"보고서_00000.txt", "보고서_00001.txt"}},
		{"README", "README_00000", nil},
		{"문서.PDF", "문서_00000.PDF", nil},
		{"표.XlSx", "표_00000.XlSx", nil},
		{"file.maß", "file_00000.maß", nil},
		{"archive.tar.gz", "archive.tar_00000.gz", nil},
		{".gitignore", ".gitignore_00000", nil},
		{"name.", "name._00000", nil},
	}
	for _, c := range cases {
		var taken []string
		for _, x := range c.taken {
			taken = append(taken, p(x))
		}
		got, err := NumberedPath(p(c.in), existsIn(taken...))
		if err != nil || got != p(c.want) {
			t.Errorf("%s: %q %v, want %q", c.in, got, err, c.want)
		}
	}
	if _, err := NumberedPath(p("a.txt"), func(string) bool { return true }); err != ErrNoNumber {
		t.Errorf("다 찼을 때: %v", err)
	}
}

func TestRemoveAfterDownload(t *testing.T) {
	for _, c := range []struct {
		prof map[string]string
		want bool
	}{{nil, false}, {map[string]string{"max_downlod_count": "1"}, true}, {map[string]string{"max_downlod_count": "0"}, false},
		{map[string]string{"max_downlod_count": "3"}, false}, {map[string]string{"max_downlod_count": ""}, false}} {
		if got := RemoveAfterDownload(c.prof); got != c.want {
			t.Errorf("%v: %v", c.prof, got)
		}
	}
}

func TestAcceptChecksum(t *testing.T) {
	if !AcceptChecksum(3, "abc") || AcceptChecksum(3, "") || AcceptChecksum(0, "Successfully downloaded file") {
		t.Error("code 3 이고 비어 있지 않을 때만 받는다")
	}
}

func TestAutoTarget(t *testing.T) {
	base := AutoPolicy{AutoTransAll: true, PrintRelayUsed: true, UserFolder: "U", UserFolderUsable: true, ServerPath: "S", PrintFolder: "P"}
	with := func(f func(*AutoPolicy)) AutoPolicy { p := base; f(&p); return p }
	cases := []struct {
		name   string
		typ    int32
		p      AutoPolicy
		folder string
		skip   AutoSkip
	}{
		{"사용자 폴더", 0, base, "U", AutoNone},
		{"사용자 폴더 빈 값", 0, with(func(p *AutoPolicy) { p.UserFolder = "" }), "", AutoEmptyFolder},
		{"사용자 폴더 못 씀", 0, with(func(p *AutoPolicy) { p.UserFolderUsable = false }), "", AutoUserFolderUnusable},
		{"모르는 유형도 0", 4, with(func(p *AutoPolicy) { p.UserFolderUsable = false }), "", AutoUserFolderUnusable},
		{"모르는 유형 -1", -1, base, "U", AutoNone},
		{"사용자 폴더 못 써도 서버 경로는", 1, with(func(p *AutoPolicy) { p.UserFolderUsable = false }), "S", AutoNone},
		{"사용자 폴더 못 써도 인쇄는", 3, with(func(p *AutoPolicy) { p.UserFolderUsable = false }), "P", AutoNone},
		{"서버 경로 전체 아님", 1, with(func(p *AutoPolicy) { p.AutoTransAll = false }), "", AutoNotAutoTransAll},
		{"서버 경로 빈 값", 1, with(func(p *AutoPolicy) { p.ServerPath = "" }), "", AutoEmptyFolder},
		{"UILess", 2, base, "", AutoUiLess},
		{"인쇄 중계 끔", 3, with(func(p *AutoPolicy) { p.PrintRelayUsed = false }), "", AutoPrintRelayOff},
		{"인쇄 폴더 빈 값", 3, with(func(p *AutoPolicy) { p.PrintFolder = "" }), "", AutoEmptyFolder},
	}
	for _, c := range cases {
		if f, s := AutoTarget(c.typ, c.p); f != c.folder || s != c.skip {
			t.Errorf("%s: %q %q", c.name, f, s)
		}
	}
	if p := AutoPolicyOf(map[string]string{"autotrans_step": "2", "print_relay": "3", "autodown_path": "X"}); !p.AutoTransAll || !p.PrintRelayUsed || p.ServerPath != "X" {
		t.Errorf("AutoPolicyOf: %+v", p)
	}
}

func TestUsableFolderAndSpace(t *testing.T) {
	d := t.TempDir()
	if err := UsableFolder(d); err != nil {
		t.Fatal(err)
	}
	if UsableFolder(filepath.Join(d, "없음")) == nil {
		t.Error("없는 폴더를 쓸 수 있다고 했다")
	}
	free, err := FreeSpace(filepath.Join(d, "아직", "없는", "폴더"))
	if err != nil || free == 0 {
		t.Errorf("FreeSpace: %d %v", free, err)
	}
	if CheckSpace(d, free+1<<40) != ErrStorageNotEnough || CheckSpace(d, 1) != nil {
		t.Error("CheckSpace")
	}
}

func TestListPages(t *testing.T) {
	var froms []int32
	total := 2*PageSize + 5
	got, err := listPages(func(from int32) ([]*serverfile.FileInfo, error) {
		froms = append(froms, from)
		n := min(PageSize, total-int(from))
		return make([]*serverfile.FileInfo, n), nil
	})
	if err != nil || len(got) != total || fmt.Sprint(froms) != fmt.Sprint([]int32{0, PageSize, 2 * PageSize}) {
		t.Errorf("%d건, from=%v, %v", len(got), froms, err)
	}
}

func TestScanning(t *testing.T) {
	a := []*serverfile.FileInfo{ok()}
	if Scanning(a) {
		t.Error("검사 끝난 목록")
	}
	for _, edit := range []func(*serverfile.FileInfo){
		func(f *serverfile.FileInfo) { f.VirusFlag = 0 },
		func(f *serverfile.FileInfo) { f.VirusFlag = 3 },
		func(f *serverfile.FileInfo) { f.PrivState = 0 },
	} {
		b := []*serverfile.FileInfo{ok(), ok()}
		edit(b[1])
		if !Scanning(b) {
			t.Error("검사 중 행을 못 봤다")
		}
	}
	if RecheckDelay(0) != 1 || RecheckDelay(2) != 4 || RecheckDelay(39) != 8 {
		t.Error("RecheckDelay")
	}
}
