package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 원본 tests/tst_LocalFileListModel.cpp 의 사례를 옮겼다.

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"dirB", "dirA"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, size := range map[string]int{"B.txt": 300, "a.txt": 10, "file2.txt": 2000, "file10.txt": 50, "note.md": 1048576} {
		if err := os.WriteFile(filepath.Join(root, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func names(es []Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Name
	}
	return out
}

func TestListOrderAndCount(t *testing.T) {
	root := fixture(t)
	p, es, err := List(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Clean(root) {
		t.Errorf("path = %q", p)
	}
	// 폴더 먼저, 이름은 대소문자 무시·자연 정렬(file2 < file10)
	want := []string{"dirA", "dirB", "a.txt", "B.txt", "file2.txt", "file10.txt", "note.md"}
	if got := names(es); !equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	byName := map[string]Entry{}
	for _, e := range es {
		byName[e.Name] = e
	}
	if byName["dirA"].SizeText != "" || !byName["dirA"].IsDir {
		t.Errorf("폴더는 크기 글자가 없다: %+v", byName["dirA"])
	}
	if byName["note.md"].Ext != "MD" {
		t.Errorf("ext = %q", byName["note.md"].Ext)
	}
}

func TestListErrors(t *testing.T) {
	root := t.TempDir()
	if _, _, err := List(filepath.Join(root, "존재하지않는폴더"), false); !errors.Is(err, ErrNotFound) {
		t.Errorf("없는 폴더 = %v", err)
	}
	f := filepath.Join(root, "file.txt")
	_ = os.WriteFile(f, nil, 0o644)
	if _, _, err := List(f, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("파일 경로 = %v", err)
	}
	if _, _, err := List("", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("빈 경로 = %v", err)
	}
}

func TestListPermissionDenied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 시스템 폴더로만 본다")
	}
	// 일반 사용자가 읽을 수 없는 폴더(관리자 셸에서는 읽힐 수 있다)
	p := `C:\System Volume Information`
	if _, err := os.Stat(p); err != nil {
		t.Skip("폴더가 없다")
	}
	if _, _, err := List(p, true); err == nil {
		t.Skip("이 셸은 읽을 수 있다(관리자)")
	} else if !errors.Is(err, ErrPermission) {
		t.Errorf("err = %v, want 권한 없음", err)
	}
}

func TestSizeText(t *testing.T) {
	for _, c := range []struct {
		n    int64
		want string
	}{
		{0, "0 byte"}, {10, "10 byte"}, {300, "300 byte"}, {1023, "1023 byte"},
		{1024, "1 KB"}, {2000, "2 KB"}, {1535, "1 KB"}, {1536, "2 KB"},
		{1048576, "1.0 MB"}, {1572864, "1.5 MB"}, {1 << 30, "1.0 GB"}, {1 << 40, "1.0 TB"},
	} {
		if got := SizeText(c.n); got != c.want {
			t.Errorf("SizeText(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestCheckDirName(t *testing.T) {
	for _, c := range []struct {
		in   string
		want error
		out  string
	}{
		{"새 폴더", nil, "새 폴더"},
		{"  여백  ", nil, "여백"},
		{"", ErrEmptyName, ""},
		{"   ", ErrEmptyName, ""},
		{"a/b", ErrBadChars, ""}, {`a\b`, ErrBadChars, ""}, {"a:b", ErrBadChars, ""}, {"a*b", ErrBadChars, ""},
		{"a?b", ErrBadChars, ""}, {`a"b`, ErrBadChars, ""}, {"a<b", ErrBadChars, ""}, {"a>b", ErrBadChars, ""}, {"a|b", ErrBadChars, ""},
		{"..", ErrBadName, ""}, {".", ErrBadName, ""}, {"끝점.", ErrBadName, ""},
		{"CON", ErrBadName, ""}, {"nul", ErrBadName, ""}, {"COM1", ErrBadName, ""}, {"lpt9.txt", ErrBadName, ""},
		{"COM0", nil, "COM0"}, {"CONSOLE", nil, "CONSOLE"},
	} {
		out, err := CheckDirName(c.in)
		if !errors.Is(err, c.want) || out != c.out {
			t.Errorf("CheckDirName(%q) = %q, %v; want %q, %v", c.in, out, err, c.out, c.want)
		}
	}
}

func TestMakeDir(t *testing.T) {
	root := fixture(t)
	p, err := MakeDir(root, "  새 폴더 ")
	if err != nil || p != filepath.Join(root, "새 폴더") {
		t.Fatalf("MakeDir = %q, %v", p, err)
	}
	if _, err := MakeDir(root, "dirA"); !errors.Is(err, ErrExists) {
		t.Errorf("같은 폴더 = %v", err)
	}
	if _, err := MakeDir(root, "a.txt"); !errors.Is(err, ErrExists) {
		t.Errorf("같은 파일 = %v", err)
	}
	if _, err := MakeDir("", "x"); !errors.Is(err, ErrNoParent) {
		t.Errorf("부모 없음 = %v", err)
	}
	if _, err := MakeDir(filepath.Join(root, "없음"), "x"); !errors.Is(err, ErrNoParent) {
		t.Errorf("없는 부모 = %v", err)
	}
}

func TestResolveStart(t *testing.T) {
	root := fixture(t)
	if p, k := ResolveStart(root, ""); p != filepath.Clean(root) || k != StartConfigured {
		t.Errorf("설정 = %q %s", p, k)
	}
	if p, k := ResolveStart(filepath.Join(root, "없음"), filepath.Join(root, "dirA")); p != filepath.Join(root, "dirA") || k != StartLast {
		t.Errorf("마지막 = %q %s", p, k)
	}
	if _, k := ResolveStart(filepath.Join(root, "a.txt"), ""); k == StartConfigured {
		t.Errorf("파일을 시작 폴더로 쓰면 안 된다")
	}
	if p, k := ResolveStart("", ""); p == "" || k == StartConfigured || k == StartLast {
		t.Errorf("기본 = %q %s", p, k)
	}
}

func TestNormPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}
	for in, want := range map[string]string{
		"file:///C:/Users/park/Downloads": `C:\Users\park\Downloads`,
		`C:/Users/park/Desktop\AUTO`:      `C:\Users\park\Desktop\AUTO`,
		"":                                "",
	} {
		if got := NormPath(in); got != want {
			t.Errorf("NormPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTypeName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}
	if s := typeNamer()("TXT"); s == "" {
		t.Errorf("TXT 유형 설명이 비었다")
	} else {
		t.Logf("TXT = %q", s)
	}
}

func TestParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		if Parent(`C:\`) != "" || Parent(`C:\a`) != `C:\` {
			t.Errorf("Parent 드라이브")
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
