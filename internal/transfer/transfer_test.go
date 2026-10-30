package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// hide 는 시험 파일을 숨긴다(Windows 속성, 그 밖은 이름이 이미 "." 으로 시작한다).
func hide(t *testing.T, p string) {
	t.Helper()
	if err := setHidden(p); err != nil {
		t.Fatal(err)
	}
}

func TestHashFile(t *testing.T) {
	p := write(t, t.TempDir(), "a", "abc")
	var pr Progress
	got, err := HashFile(context.Background(), p, &pr)
	if want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; err != nil || got != want {
		t.Fatalf("got %q %v", got, err)
	}
	if ph, d, tot := pr.Snapshot(); ph != PhaseHash || d != 3 || tot != 3 {
		t.Fatal(ph, d, tot)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := HashFile(ctx, p, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// 원본 tests/tst_UploadGate.cpp
func TestAdmitUpload(t *testing.T) {
	for _, c := range []struct {
		owner Owner
		auto  bool
		want  Admit
	}{
		{OwnerNone, false, Take},
		{OwnerNone, true, Take},
		{OwnerAuto, true, SkipAuto},
		{OwnerHuman, true, SkipAuto},
		{OwnerAuto, false, PreemptAuto},
		{OwnerHuman, false, RejectHuman},
	} {
		if got := AdmitUpload(c.owner, c.auto); got != c.want {
			t.Errorf("AdmitUpload(%d,%v)=%d want %d", c.owner, c.auto, got, c.want)
		}
	}
}

// 원본 tests/tst_UploadSnapshot.cpp
func snapRoot(t *testing.T) string {
	r := t.TempDir()
	write(t, r, "a.pdf", "aaaa")
	write(t, r, "b.pdf", "bbbbbb")
	return r
}

func snap(t *testing.T, r string) map[string]Stamp {
	t.Helper()
	s, err := Snapshot(r)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSnapshotSame(t *testing.T) {
	r := snapRoot(t)
	b := snap(t, r)
	if len(b) != 2 || !DiffSnapshots(b, snap(t, r)).Empty() {
		t.Fatal(b)
	}
}

func TestSnapshotAddedRemoved(t *testing.T) {
	r := snapRoot(t)
	b := snap(t, r)
	write(t, r, "x.txt", "x")
	os.Remove(filepath.Join(r, "b.pdf"))
	d := DiffSnapshots(b, snap(t, r))
	if !slices.Equal(d.Added, []string{"x.txt"}) || !slices.Equal(d.Removed, []string{"b.pdf"}) || d.Changed != nil {
		t.Fatalf("%+v", d)
	}
	if !strings.Contains(d.String(), "더해짐 1 [x.txt] 없어짐 1 [b.pdf] 바뀜 0 []") {
		t.Fatal(d.String())
	}
}

func TestSnapshotSizeOnlyAndMtimeOnly(t *testing.T) {
	r := snapRoot(t)
	b := snap(t, r)
	p := filepath.Join(r, "a.pdf")
	st, _ := os.Stat(p)
	write(t, r, "a.pdf", "aaaaaaaa")
	os.Chtimes(p, st.ModTime(), st.ModTime())
	if d := DiffSnapshots(b, snap(t, r)); !slices.Equal(d.Changed, []string{"a.pdf"}) {
		t.Fatalf("size: %+v", d)
	}
	b = snap(t, r)
	os.Chtimes(p, st.ModTime(), st.ModTime().Add(10*time.Second))
	if d := DiffSnapshots(b, snap(t, r)); !slices.Equal(d.Changed, []string{"a.pdf"}) {
		t.Fatalf("mtime: %+v", d)
	}
}

func TestSnapshotSubfolder(t *testing.T) {
	r := snapRoot(t)
	write(t, r, "sub/deep/c.pdf", "c")
	b := snap(t, r)
	if _, ok := b["sub/deep/c.pdf"]; !ok {
		t.Fatal(b)
	}
	write(t, r, "sub/deep/c.pdf", "cc")
	write(t, r, "sub/d.pdf", "d")
	d := DiffSnapshots(b, snap(t, r))
	if !slices.Equal(d.Changed, []string{"sub/deep/c.pdf"}) || !slices.Equal(d.Added, []string{"sub/d.pdf"}) {
		t.Fatalf("%+v", d)
	}
}

func hiddenNames() (file, dir string) {
	if runtime.GOOS == "windows" {
		return "Thumbs.db", "hid"
	}
	return ".hidden", ".hid"
}

func TestSnapshotHiddenAndIgnoredOnlyIsClean(t *testing.T) {
	r := snapRoot(t)
	hf, hd := hiddenNames()
	write(t, r, "deny.txt", "blocked")
	b := snap(t, r)
	hide(t, write(t, r, hf, "h"))
	write(t, r, hd+"/in.pdf", "i")
	hide(t, filepath.Join(r, hd))
	write(t, r, "deny.txt", "blocked-and-changed")
	a := snap(t, r)
	if _, ok := a[hf]; ok {
		t.Fatal("hidden file in snapshot")
	}
	if _, ok := a[hd+"/in.pdf"]; ok {
		t.Fatal("hidden dir in snapshot")
	}
	if !DiffSnapshots(b, a, "deny.txt").Empty() {
		t.Fatal("ignored must be clean")
	}
	if d := DiffSnapshots(b, a); !slices.Equal(d.Changed, []string{"deny.txt"}) {
		t.Fatalf("%+v", d)
	}
}

func TestSnapshotEmptyFolder(t *testing.T) {
	r := t.TempDir()
	os.Mkdir(filepath.Join(r, "emptysub"), 0o755)
	b := snap(t, r)
	if len(b) != 0 {
		t.Fatal(b)
	}
	write(t, r, "emptysub/new.pdf", "n")
	if d := DiffSnapshots(b, snap(t, r)); !slices.Equal(d.Added, []string{"emptysub/new.pdf"}) {
		t.Fatalf("%+v", d)
	}
}

func TestStampOf(t *testing.T) {
	p := write(t, t.TempDir(), "a.pdf", "aaaa")
	s, err := StampOf(p)
	if err != nil || s.Size != 4 || s.MtimeMs <= 0 {
		t.Fatal(s, err)
	}
	if s2, _ := StampOf(p); s2 != s {
		t.Fatal("same")
	}
	st, _ := os.Stat(p)
	os.Chtimes(p, st.ModTime(), st.ModTime().Add(-5*time.Second))
	if s2, _ := StampOf(p); s2 == s {
		t.Fatal("mtime change must differ")
	}
}

// 폴더 압축: 숨김·제외·빈 폴더는 빠지고, 한글 이름은 UTF-8 플래그로, 상대 경로는 그대로.
func TestZipFolder(t *testing.T) {
	r := filepath.Join(t.TempDir(), "보고서")
	write(t, r, "한글/문서.txt", "본문")
	write(t, r, "a.txt", "a")
	deny := write(t, r, "deny.exe", "x")
	hf, hd := hiddenNames()
	hide(t, write(t, r, hf, "h"))
	write(t, r, hd+"/in.txt", "i")
	hide(t, filepath.Join(r, hd))
	os.MkdirAll(filepath.Join(r, "empty"), 0o755)
	zp := filepath.Join(t.TempDir(), "보고서.zip")
	var pr Progress
	n, err := ZipFolder(context.Background(), r, zp, []string{deny}, &pr)
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	zr, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Name == "한글/문서.txt" && f.NonUTF8 {
			t.Fatal("한글 이름은 UTF-8 플래그")
		}
		if f.Method != zip.Deflate {
			t.Fatal("deflate")
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"a.txt", "한글/문서.txt"}) {
		t.Fatal(names)
	}
	if ph, d, tot := pr.Snapshot(); ph != PhaseZip || d != tot || tot != int64(len("본문")+1) {
		t.Fatal(ph, d, tot)
	}

	// 풀어서 그대로 돌아오는가
	out := filepath.Join(t.TempDir(), "out")
	if err := ExtractZip(context.Background(), zp, out, DefaultExtractLimits, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "한글", "문서.txt")); string(b) != "본문" {
		t.Fatal(string(b))
	}
}

func TestZipFolderCanceledRemovesZip(t *testing.T) {
	r := t.TempDir()
	write(t, r, "a.txt", "a")
	zp := filepath.Join(t.TempDir(), "x.zip")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ZipFolder(ctx, r, zp, nil, nil); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(zp); err == nil {
		t.Fatal("temp zip must be removed")
	}
}

func makeZip(t *testing.T, ents map[string]string, mod func(*zip.FileHeader)) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range ents {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if mod != nil {
			mod(h)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	p := filepath.Join(t.TempDir(), "t.zip")
	os.WriteFile(p, buf.Bytes(), 0o644)
	return p
}

func TestExtractZipRejectsUnsafeBeforeWriting(t *testing.T) {
	for _, bad := range []string{"../evil.txt", "/abs.txt", `..\evil.txt`, "C:/x.txt", "a/../../x"} {
		zp := makeZip(t, map[string]string{"ok.txt": "ok", bad: "x"}, nil)
		out := filepath.Join(t.TempDir(), "out")
		err := ExtractZip(context.Background(), zp, out, DefaultExtractLimits, nil)
		if CodeOf(err) != CodeUnzipUnsafe {
			t.Fatalf("%s: %v", bad, err)
		}
		if _, err := os.Stat(filepath.Join(out, "ok.txt")); err == nil {
			t.Fatalf("%s: nothing must be written", bad)
		}
	}
}

func TestExtractZipLimits(t *testing.T) {
	zp := makeZip(t, map[string]string{"a": strings.Repeat("x", 100), "b": "y"}, nil)
	out := t.TempDir()
	if err := ExtractZip(context.Background(), zp, out, ExtractLimits{MaxBytes: 1 << 30, MaxItems: 1}, nil); CodeOf(err) != CodeUnzipItems {
		t.Fatal(err)
	}
	// 헤더 크기를 거짓으로 적어도 실제로 센다
	err := ExtractZip(context.Background(), zp, t.TempDir(), ExtractLimits{MaxBytes: 50, MaxItems: 10}, nil)
	if CodeOf(err) != CodeUnzipSize || !strings.Contains(err.Error(), "상한") {
		t.Fatal(err)
	}
}

func TestExtractZipCP949Name(t *testing.T) {
	// "가.txt" 의 CP949 바이트(UTF-8 플래그 없음)
	name := string([]byte{0xB0, 0xA1}) + ".txt"
	zp := makeZip(t, map[string]string{name: "x"}, func(h *zip.FileHeader) { h.NonUTF8 = true })
	out := t.TempDir()
	if err := ExtractZip(context.Background(), zp, out, DefaultExtractLimits, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "가.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestMergeDir(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	write(t, src, "same.txt", "new")
	write(t, src, "sub/n.txt", "n")
	write(t, dst, "same.txt", "old")
	write(t, dst, "keep.txt", "k")
	if err := MergeDir(src, dst); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{"same.txt": "new", "keep.txt": "k", "sub/n.txt": "n"} {
		if b, _ := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel))); string(b) != want {
			t.Fatalf("%s = %q", rel, b)
		}
	}
}

func TestErrorTexts(t *testing.T) {
	for _, c := range []struct {
		code      int
		arg, arg2 string
		want      string
	}{
		{CodeUpBizNoPerm, "", "", "업무망 파일 업로드 권한이 없습니다."},
		{CodeUpNetSize, "100 MB", "", "파일사이즈가 전송가능사이즈를(100 MB) 초과하였습니다."},
		{CodeDnBizSize, "1 GB", "", "파일사이즈가 전송사이즈(1 GB)를 초과하였습니다."},
		{CodeChecksum, "AA", "bb", "파일이 변조되었거나 손상되었습니다. Original(AA), Download_file(bb)"},
		{CodeSizeMismatch, "1", "2", "받은 크기(1 바이트)가 서버 파일 크기(2 바이트)와 달라 저장하지 않았습니다."},
		{CodeChanged, "", "", "검사한 뒤 파일이 바뀌어 올리지 않았습니다. 다시 올려 주세요."},
		{CodeStall, "30", "", "서버 응답이 없어 전송을 중단했습니다."},
		{CodeStorage, "", "", "저장소의 용량이 부족합니다."},
		{-2, "", "", "확장자가 없는 파일은 전송할 수 없습니다."},
		{-12345, "", "", "알 수 없는 오류 [-12345]"},
	} {
		if got := Text(c.code, c.arg, c.arg2); got != c.want {
			t.Errorf("Text(%d) = %q want %q", c.code, got, c.want)
		}
	}
	if CodeOf(nil) != 0 || CodeOf(ErrCanceled) != CodeCanceled || CodeOf(errors.New("x")) != CodeFailed || CodeOf(fail(-45, nil)) != -45 {
		t.Fatal("CodeOf")
	}
}

func TestAutoCandidatesAndMoveToDone(t *testing.T) {
	r := t.TempDir()
	a := write(t, r, "a.txt", "a")
	write(t, r, "folder/x.txt", "x")
	write(t, r, "done/old.txt", "o")
	hf, _ := hiddenNames()
	hide(t, write(t, r, hf, "h"))
	part, err := createPart(filepath.Join(r, "받는 중.txt")) // 받는 폴더와 같은 폴더여도 쓰는 중 파일은 올리지 않는다
	if err != nil {
		t.Fatal(err)
	}
	part.Close()
	if part, err = tempPart(filepath.Join(r, "긴 이름.txt")); err != nil {
		t.Fatal(err)
	}
	part.Close()
	got, err := AutoCandidates(r)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, p := range got {
		names = append(names, filepath.Base(p))
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"a.txt", "folder"}) {
		t.Fatal(names)
	}
	write(t, r, "done/a.txt", "prev")
	dst, err := MoveToDone(a, r)
	if err != nil || filepath.Base(dst) != "a_00000.txt" {
		t.Fatal(dst, err)
	}
	if FolderProblem(r) != "" || FolderProblem("") == "" || FolderProblem(filepath.Join(r, "none")) == "" || FolderProblem(dst) == "" {
		t.Fatal("FolderProblem")
	}
}
