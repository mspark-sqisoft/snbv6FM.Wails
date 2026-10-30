package domain

// 원본 tests/tst_DomainPolicy.cpp 를 옮겼다. 실제 hosts 는 건드리지 않는다.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

var kRows = [][]byte{[]byte("10.0.0.1\tintra.example.com\t\t#profile1"), []byte("fe80::1\tv6.example.com\t\t#p2")}

func block(eol string, rows [][]byte) []byte {
	b := []byte(startMarker + eol + cautionMarker + eol)
	for _, r := range rows {
		b = append(append(b, r...), eol...)
	}
	return append(b, endMarker+eol...)
}

func concat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func removed(t *testing.T, in []byte) []byte {
	t.Helper()
	out, _ := RemoveBlocks(in)
	return out
}

func TestRoundTripRestoresBytes(t *testing.T) {
	for name, orig := range map[string][]byte{
		"lf":        []byte("127.0.0.1\tlocalhost\n# end\n"),
		"crlf":      []byte("127.0.0.1\tlocalhost\r\n# end\r\n"),
		"empty":     nil,
		"blankTail": []byte("127.0.0.1\tlocalhost\r\n\r\n"),
		"cp949":     []byte("# \xC7\xD1\xB1\xDB \xC1\xD6\xBC\xAE\r\n10.1.1.1\tsvr\r\n"),
	} {
		with := AppendBlock(orig, kRows)
		if !HasBlock(with) || !bytes.HasPrefix(with, orig) {
			t.Errorf("%s: 붙이기 %q", name, with)
		}
		out, partial := RemoveBlocks(with)
		if !bytes.Equal(out, orig) || partial {
			t.Errorf("%s: %q partial=%v", name, out, partial)
		}
	}
}

func TestAppendFollowsLineEnding(t *testing.T) {
	if got := AppendBlock([]byte("a\r\n"), kRows); !bytes.Equal(got, concat([]byte("a\r\n\r\n"), block("\r\n", kRows))) {
		t.Errorf("crlf: %q", got)
	}
	if got := AppendBlock([]byte("a\n"), kRows); !bytes.Equal(got, concat([]byte("a\n\n"), block("\n", kRows))) {
		t.Errorf("lf: %q", got)
	}
	if got := AppendBlock([]byte("a\r\nb"), kRows); !bytes.HasPrefix(got, []byte("a\r\nb\r\n\r\n")) {
		t.Errorf("끝 줄바꿈 없음: %q", got)
	}
}

func TestRemove(t *testing.T) {
	other := []byte("# \xC7\xD1\xB1\xDB\r\n\xFF\xFE bad\r\n")
	if got := removed(t, concat(other, []byte("\r\n"), block("\r\n", kRows), []byte("9.9.9.9\tafter\r\n"))); !bytes.Equal(got, concat(other, []byte("9.9.9.9\tafter\r\n"))) {
		t.Errorf("비UTF-8: %q", got)
	}
	legacy := []byte("a\r\n\r\n" + startMarker + "\r\n" + cautionMarker + "\r\n" + string(kRows[0]) + "\r\n" + endMarker)
	if got := removed(t, legacy); string(got) != "a\r\n" {
		t.Errorf("옛 구역: %q", got)
	}
	half := []byte("a\n\n" + startMarker + "\n" + cautionMarker + "\n" + string(kRows[0]) + "\n192.168.0.9 other-app.local\n# other\n")
	got, partial := RemoveBlocks(half)
	if string(got) != "a\n192.168.0.9 other-app.local\n# other\n" || !partial {
		t.Errorf("반쪽: %q %v", got, partial)
	}
	if got := removed(t, concat(block("\n", kRows), []byte("b\n"))); string(got) != "b\n" {
		t.Errorf("첫 줄: %q", got)
	}
	p := []byte("\n" + startMarker + "\n" + cautionMarker + "\n")
	if got := removed(t, concat([]byte("a\n"), p, p, []byte("\n"), block("\n", kRows))); string(got) != "a\n" {
		t.Errorf("여러 구역: %q", got)
	}
	inside := []byte("a\n\n" + startMarker + "\n" + cautionMarker + "\n1.2.3.4 someone-added\n" + endMarker + "\nz\n")
	if got := removed(t, inside); string(got) != "a\nz\n" {
		t.Errorf("구역 안 남의 줄: %q", got)
	}
}

func TestHasBlock(t *testing.T) {
	if HasBlock([]byte("a\nb\n")) || !HasBlock([]byte("a\n"+startMarker+"\n")) || !HasBlock([]byte(startMarker+"\r\n")) {
		t.Fatal("HasBlock")
	}
}

func TestRows(t *testing.T) {
	var skipped []string
	rows := Rows([]Info{
		{"p1", "intra.example.com", "10.0.0.1"},
		{"p\n2", "v6.example.com", "fe80::1"},
		{"p", "bad name", "10.0.0.2"},
		{"p", "ok.example", "10.0.0.3\n1.1.1.1 evil"},
		{"p", "ok.example", "fe80::1%eth0\nx"},
		{"p", "ok.example", "not-ip"},
	}, func(ip, name string) { skipped = append(skipped, name) })
	if len(rows) != 2 || len(skipped) != 4 {
		t.Fatalf("%q %v", rows, skipped)
	}
	if string(rows[0]) != "10.0.0.1\tintra.example.com\t\t#p1" || string(rows[1]) != "fe80::1\tv6.example.com\t\t#p2" {
		t.Fatalf("%q", rows)
	}
}

func TestApplyRemove(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	orig := []byte("# \xC7\xD1\xB1\xDB\r\n127.0.0.1 x\r\n")
	if err := os.WriteFile(p, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(p, kRows); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(p, kRows); err != nil { // 두 번 써도 구역은 하나다
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if bytes.Count(got, []byte(startMarker)) != 1 {
		t.Fatalf("%q", got)
	}
	if _, err := Remove(p); err != nil {
		t.Fatal(err)
	}
	if got, _ = os.ReadFile(p); !bytes.Equal(got, orig) {
		t.Fatalf("%q", got)
	}
	if _, err := Remove(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("없는 파일")
	}
}
