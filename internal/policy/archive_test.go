package policy

// 압축 검사 시험. testdata 의 7z·zip 은 7-Zip 26 으로 만들었다(내용: a.txt·sub/b.pdf, big.txt = "line N: the quick brown fox…" 300줄 16092바이트,
// 비밀번호 Secret1). 나머지는 시험 안에서 만든다. 원본 tst_ZipExtractor 의 위험 항목·암호·상한 경우를 옮겼다.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixturePW = "Secret1"

type ent struct {
	name string
	data []byte
}

func writeZip(t *testing.T, path string, entries ...ent) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(e.data)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func zipBytes(t *testing.T, entries ...ent) []byte {
	p := filepath.Join(t.TempDir(), "x.zip")
	writeZip(t, p, entries...)
	b, _ := os.ReadFile(p)
	return b
}

func writeTar(t *testing.T, path string, hdrs ...tar.Header) {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, h := range hdrs {
		data := []byte(h.Linkname)
		if h.Typeflag == tar.TypeReg {
			data = []byte("content of " + h.Name)
			h.Size = int64(len(data))
		} else {
			data = nil
		}
		if err := w.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	w.Close()
	os.WriteFile(path, buf.Bytes(), 0o600)
}

func names(es []Entry) string {
	var s []string
	for _, e := range es {
		s = append(s, e.Path)
	}
	return strings.Join(s, ",")
}

func inspect(path, pw string) ([]Entry, ZipCode, *Stop) {
	return Inspect(context.Background(), path, InspectOptions{Password: pw})
}

func TestInspectZipPlain(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.zip")
	writeZip(t, p, ent{"dir/", nil}, ent{"dir/x.pdf", []byte("%PDF-1.7 body")}, ent{"top.txt", []byte("TOP")})
	es, c, _ := inspect(p, "")
	if c != ZipOK || names(es) != "dir/x.pdf,top.txt" {
		t.Fatalf("%d %s", c, names(es))
	}
	if string(es[0].Head) != "%PDF-1.7 body" || es[0].Name != "x.pdf" || es[1].Size != 3 {
		t.Fatalf("%+v", es[0])
	}
}

func TestInspectFixtures(t *testing.T) {
	for _, c := range []struct {
		file, pw string
		want     ZipCode
		names    string
	}{
		{"plain.7z", "", ZipOK, "a.txt,sub/b.pdf"},
		{"aes_copy.7z", "", ZipPassword, ""}, // 무압축+AES ― 비밀번호 없이 읽으면 오류 없이 엉뚱한 바이트, CRC 로 잡는다
		{"aes_copy.7z", fixturePW, ZipOK, "a.txt"},
		{"aes_copy.7z", "nope", ZipPassword, ""},
		{"aes_lzma.7z", "", ZipPassword, ""},
		{"aes_lzma.7z", fixturePW, ZipOK, "big.txt"},
		{"aes_lzma.7z", "nope", ZipPassword, ""},
		{"aes_hdr.7z", "", ZipPassword, ""}, // 머리까지 잠겨 목록조차 못 읽는다
		{"aes_hdr.7z", fixturePW, ZipOK, "a.txt"},
		{"aes_hdr.7z", "nope", ZipPassword, ""},
		{"zipcrypto.zip", "", ZipPassword, ""},
		{"zipcrypto.zip", fixturePW, ZipOK, "big.txt,sub/b.pdf"},
		{"zipcrypto.zip", "nope", ZipPassword, ""},
		{"zipaes.zip", "", ZipPassword, ""},
		{"zipaes.zip", fixturePW, ZipOK, "big.txt,sub/b.pdf"},
		{"zipaes.zip", "nope", ZipPassword, ""},
		{"bzip2.zip", "", ZipOK, "big.txt"},
		{"lzma.zip", "", ZipOK, "big.txt"},
		{"ppmd.zip", "", ZipUncheckable, ""},
		{"nested_locked.zip", "", ZipInnerPassword, ""},
		{"big.txt.bz2", "", ZipOK, "big.txt"},
		{"big.txt.xz", "", ZipOK, "big.txt"},
	} {
		es, got, _ := inspect(filepath.Join("testdata", c.file), c.pw)
		if got != c.want || (got == ZipOK && names(es) != c.names) {
			t.Errorf("%s pw=%v: %d %q", c.file, c.pw != "", got, names(es))
			continue
		}
		for _, e := range es {
			if e.Name == "big.txt" && (!bytes.HasPrefix(e.Head, []byte("line 1: the quick")) || e.Size != 16092) {
				t.Errorf("%s: big.txt head %q size %d", c.file, e.Head[:min(len(e.Head), 20)], e.Size)
			}
		}
	}
}

func TestInspectUncheckableLZH(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.lzh")
	os.WriteFile(p, []byte("-lh5- whatever"), 0o600)
	if _, c, _ := inspect(p, ""); c != ZipUncheckable {
		t.Fatal(c)
	}
	// 안쪽이면 정책과 상관없이 막는 코드
	outer := filepath.Join(t.TempDir(), "o.zip")
	writeZip(t, outer, ent{"in.lzh", []byte("-lh5-")})
	if _, c, _ := inspect(outer, ""); c != ZipInnerUncheckable {
		t.Fatal(c)
	}
}

func TestInspectStreamsAndTar(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "a.tar")
	writeTar(t, tarPath, tar.Header{Name: "d/", Typeflag: tar.TypeDir, Mode: 0o755}, tar.Header{Name: "d/x.txt", Typeflag: tar.TypeReg, Mode: 0o644})
	es, c, _ := inspect(tarPath, "")
	if c != ZipOK || names(es) != "d/x.txt" || string(es[0].Head) != "content of d/x.txt" {
		t.Fatalf("tar %d %s", c, names(es))
	}
	tb, _ := os.ReadFile(tarPath)
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write(tb)
	zw.Close()
	tgz := filepath.Join(dir, "b.tgz")
	os.WriteFile(tgz, gz.Bytes(), 0o600)
	if es, c, _ := inspect(tgz, ""); c != ZipOK || names(es) != "b.tar,b.tar/d/x.txt" {
		t.Fatalf("tgz %d %s", c, names(es))
	}
	tgz2 := filepath.Join(dir, "c.tar.gz")
	os.WriteFile(tgz2, gz.Bytes(), 0o600)
	if es, c, _ := inspect(tgz2, ""); c != ZipOK || names(es) != "c.tar,c.tar/d/x.txt" {
		t.Fatalf("tar.gz %d %s", c, names(es))
	}
	notgz := filepath.Join(dir, "d.gz")
	os.WriteFile(notgz, []byte("plain"), 0o600)
	if _, c, _ := inspect(notgz, ""); c != ZipNotArchive {
		t.Fatalf("not gz %d", c)
	}
	notzip := filepath.Join(dir, "e.zip")
	os.WriteFile(notzip, []byte("plain text"), 0o600)
	if _, c, _ := inspect(notzip, ""); c != ZipNotArchive {
		t.Fatalf("not zip %d", c)
	}
	if _, c, _ := inspect(filepath.Join(dir, "missing.zip"), ""); c != ZipNoFile {
		t.Fatalf("missing %d", c)
	}
}

// 풀 폴더 밖을 가리키거나 링크인 항목이 하나라도 있으면 거절한다(원본 extract_unsafeEntries_isRejected).
func TestInspectUnsafe(t *testing.T) {
	dir := t.TempDir()
	var logs []string
	cases := map[string]func(p string){
		"escape.zip": func(p string) {
			writeZip(t, p, ent{"ok/inner.txt", []byte("I")}, ent{"../escape1.txt", []byte("E")}, ent{"../../escape2.txt", []byte("E")})
		},
		"absolute.zip":    func(p string) { writeZip(t, p, ent{"ok.txt", []byte("O")}, ent{"/abs.txt", []byte("A")}) },
		"drive.zip":       func(p string) { writeZip(t, p, ent{"C:/abs.txt", []byte("A")}) },
		"backslash.zip":   func(p string) { writeZip(t, p, ent{"a\\..\\..\\b.txt", []byte("A")}) },
		"dotdotinner.zip": func(p string) { writeZip(t, p, ent{"a/../b.txt", []byte("B")}) },
		"dots.zip":        func(p string) { writeZip(t, p, ent{"a/... /b.txt", []byte("B")}) },
		"escape.tar": func(p string) {
			writeTar(t, p, tar.Header{Name: "ok.txt", Typeflag: tar.TypeReg}, tar.Header{Name: "../e.txt", Typeflag: tar.TypeReg})
		},
		"symlink.tar": func(p string) {
			writeTar(t, p, tar.Header{Name: "ok.txt", Typeflag: tar.TypeReg}, tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../../../outside"})
		},
		"hardlink.tar": func(p string) {
			writeTar(t, p, tar.Header{Name: "ok.txt", Typeflag: tar.TypeReg}, tar.Header{Name: "hl", Typeflag: tar.TypeLink, Linkname: "ok.txt"})
		},
		"nested.zip": func(p string) {
			writeZip(t, p, ent{"ok.txt", []byte("O")}, ent{"inner.zip", zipBytes(t, ent{"../../../escape3.txt", []byte("E")})})
		},
	}
	for name, mk := range cases {
		p := filepath.Join(dir, name)
		mk(p)
		_, c, _ := Inspect(context.Background(), p, InspectOptions{Log: func(s string) { logs = append(logs, s) }})
		if c != ZipUnsafe {
			t.Errorf("%s: %d", name, c)
		}
	}
	if len(logs) != len(cases) || !strings.HasPrefix(logs[0], "압축파일 경로 검사 거절(허용되지 않는 경로 또는 링크) >>> 압축파일: ") {
		t.Fatalf("logs %d %v", len(logs), logs)
	}
}

func TestInspectDepth(t *testing.T) {
	dir := t.TempDir()
	l3 := zipBytes(t, ent{"deep.txt", []byte("D")})
	l2 := zipBytes(t, ent{"l3.zip", l3})
	p := filepath.Join(dir, "l1.zip")
	writeZip(t, p, ent{"l2.zip", l2})
	if _, c, _ := Inspect(context.Background(), p, InspectOptions{MaxDepth: 2}); c != ZipDepth {
		t.Fatalf("depth 2: %d", c)
	}
	es, c, _ := Inspect(context.Background(), p, InspectOptions{MaxDepth: 3})
	if c != ZipOK || names(es) != "l2.zip,l2.zip/l3.zip,l2.zip/l3.zip/deep.txt" {
		t.Fatalf("depth 3: %d %s", c, names(es))
	}
	// 임시 파일을 남기지 않는다
	tmp := t.TempDir()
	Inspect(context.Background(), p, InspectOptions{MaxDepth: 3, TempDir: tmp})
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("temp left: %d", len(left))
	}
}

// bombZip 은 목록에는 10바이트라 적고 실제로는 n 바이트(0)가 풀리는 안쪽 zip 을 담은 zip 이다.
func bombZip(t *testing.T, path string, n int) {
	var raw bytes.Buffer
	fw, _ := flate.NewWriter(&raw, flate.BestCompression)
	zeros := make([]byte, n)
	fw.Write(zeros)
	fw.Close()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "bomb.zip", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(zeros), CompressedSize64: uint64(raw.Len()), UncompressedSize64: 10}
	f, err := w.CreateRaw(h)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(raw.Bytes())
	w.Close()
	os.WriteFile(path, buf.Bytes(), 0o600)
}

func TestInspectLimits(t *testing.T) {
	dir := t.TempDir()
	many := filepath.Join(dir, "many.zip")
	var es []ent
	for i := 0; i < 20; i++ {
		es = append(es, ent{strings.Repeat("x", i+1) + ".txt", []byte("1234567890")})
	}
	writeZip(t, many, es...)
	if _, c, st := Inspect(context.Background(), many, InspectOptions{Limits: Limits{MaxItems: 10, MaxBytes: 1 << 30}}); c != ZipLimitItems || st == nil || st.Reason != "항목 수 상한" {
		t.Fatalf("items %d %+v", c, st)
	}
	if _, c, st := Inspect(context.Background(), many, InspectOptions{Limits: Limits{MaxItems: 100, MaxBytes: 100}}); c != ZipLimitSize || st.Reason != "크기 상한" {
		t.Fatalf("declared %d %+v", c, st)
	}
	// 목록을 속인 압축 폭탄: 실제로 푼 양으로 멈춘다
	bomb := filepath.Join(dir, "bomb.zip")
	bombZip(t, bomb, 4<<20)
	_, c, st := Inspect(context.Background(), bomb, InspectOptions{Limits: Limits{MaxItems: 100, MaxBytes: 1 << 20}, TempDir: t.TempDir()})
	if c != ZipLimitSize || st == nil || st.Bytes <= 1<<20 || st.Bytes > 2<<20 {
		t.Fatalf("bomb %d %+v", c, st)
	}
	if line := st.LogLine("업로드 검사", bomb); !strings.HasPrefix(line, "압축 풀기 중단 >>> 사유[크기 상한], 경로[업로드 검사], 압축파일[bomb.zip], 상한[1048576]") {
		t.Fatal(line)
	}
	// 단일 스트림 폭탄(gz)
	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	zw.Write(make([]byte, 4<<20))
	zw.Close()
	gzp := filepath.Join(dir, "z.gz")
	os.WriteFile(gzp, gz.Bytes(), 0o600)
	if _, c, _ := Inspect(context.Background(), gzp, InspectOptions{Limits: Limits{MaxItems: 10, MaxBytes: 1 << 20}}); c != ZipLimitSize {
		t.Fatalf("gz bomb %d", c)
	}
	// 디스크 여유가 상한보다 작으면 안쪽 압축을 풀지 않는다
	if _, c, st := Inspect(context.Background(), bomb, InspectOptions{Limits: Limits{MaxItems: 10, MaxBytes: 1 << 40, ReserveBytes: 1 << 62}, TempDir: t.TempDir()}); c != ZipDisk || st.Free == 0 {
		t.Fatalf("disk %d %+v", c, st)
	}
}

func TestInspectCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, c, _ := Inspect(ctx, filepath.Join("testdata", "plain.7z"), InspectOptions{}); c != ZipCancel {
		t.Fatal(c)
	}
}
