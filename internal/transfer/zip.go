package transfer

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"

	"snbapp/internal/files"
)

// ── 스냅샷(원본 imports/FileMoverGrpc/UploadSnapshot.h, 097) ──

// Stamp 는 파일 하나의 기준값이다. 수정 시각은 ms(원본 해상도).
type Stamp struct {
	Size    int64
	MtimeMs int64
}

// StampOf 는 path 의 지금 기준값이다.
func StampOf(path string) (Stamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{st.Size(), st.ModTime().UnixMilli()}, nil
}

// walkFolder 는 폴더 압축·스냅샷이 함께 쓰는 순회다: 숨김 파일·숨김 폴더를 건너뛰고 파일만 rel("/" 구분)로 준다.
func walkFolder(root string, fn func(abs, rel string, info fs.FileInfo) error) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		if hidden(d.Name(), d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() { // 링크·장치 파일은 압축에 넣지 않는다
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		return fn(p, filepath.ToSlash(rel), info)
	})
}

// Snapshot 은 폴더의 rel → 기준값이다(숨김 제외, 빈 폴더는 없다).
func Snapshot(root string) (map[string]Stamp, error) {
	out := map[string]Stamp{}
	err := walkFolder(root, func(_, rel string, info fs.FileInfo) error {
		out[rel] = Stamp{info.Size(), info.ModTime().UnixMilli()}
		return nil
	})
	return out, err
}

// SnapDiff 는 두 스냅샷의 차이다(이름순).
type SnapDiff struct{ Added, Removed, Changed []string }

func (d SnapDiff) Empty() bool { return len(d.Added)+len(d.Removed)+len(d.Changed) == 0 }

// String 은 원본 로그 꼬리 형식이다: 더해짐 N [..] 없어짐 N [..] 바뀜 N [..].
func (d SnapDiff) String() string {
	part := func(label string, l []string) string {
		return fmt.Sprintf("%s %d [%s]", label, len(l), strings.Join(l, ", "))
	}
	return part("더해짐", d.Added) + " " + part("없어짐", d.Removed) + " " + part("바뀜", d.Changed)
}

// DiffSnapshots 는 before→after 차이다. ignore 의 rel(압축에서 빠지는 파일)은 보지 않는다.
func DiffSnapshots(before, after map[string]Stamp, ignore ...string) SnapDiff {
	var d SnapDiff
	for k, a := range after {
		if slices.Contains(ignore, k) {
			continue
		}
		if b, ok := before[k]; !ok {
			d.Added = append(d.Added, k)
		} else if a != b {
			d.Changed = append(d.Changed, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok && !slices.Contains(ignore, k) {
			d.Removed = append(d.Removed, k)
		}
	}
	slices.Sort(d.Added)
	slices.Sort(d.Removed)
	slices.Sort(d.Changed)
	return d
}

// ── 폴더 압축(원본 ZipExtractor::CompressDirectory) ──

// ZipFolder 는 dir 안의 파일을 rel 경로로 zip 에 담는다(Deflate). 숨김·excludes(절대 경로)는 빼고, 빈 폴더는 담지 않는다.
// 한글 이름은 archive/zip 이 UTF-8 플래그를 붙인다. 실패하면 만들던 zip 을 지운다. files 는 담은 파일 수.
func ZipFolder(ctx context.Context, dir, zipPath string, excludes []string, p *Progress) (n int, err error) {
	ex := make(map[string]bool, len(excludes))
	for _, e := range excludes {
		ex[filepath.Clean(e)] = true
	}
	type ent struct {
		abs, rel string
		info     fs.FileInfo
	}
	var list []ent
	var total int64
	if err := walkFolder(dir, func(abs, rel string, info fs.FileInfo) error {
		if !ex[filepath.Clean(abs)] {
			list = append(list, ent{abs, rel, info})
			total += info.Size()
		}
		return nil
	}); err != nil {
		return 0, err
	}
	if p != nil {
		p.Set(PhaseZip, 0, total)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(zipPath)
		}
	}()
	zw := zip.NewWriter(f)
	buf := make([]byte, 256<<10)
	for _, e := range list {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		h, err := zip.FileInfoHeader(e.info)
		if err != nil {
			return 0, err
		}
		h.Name, h.Method = e.rel, zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return 0, err
		}
		src, err := os.Open(e.abs)
		if err != nil {
			return 0, err
		}
		_, err = io.CopyBuffer(w, &countReader{src, p}, buf)
		src.Close()
		if err != nil {
			return 0, err
		}
	}
	return len(list), zw.Close()
}

type countReader struct {
	r io.Reader
	p *Progress
}

func (c *countReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	if c.p != nil && n > 0 {
		c.p.Add(int64(n))
	}
	return n, err
}

// ── 안전 풀기(docs/port/04 7.6) ──

// ExtractLimits 는 풀기 상한이다(원본 098: 10GiB·10만 개·여유 1GiB).
type ExtractLimits struct {
	MaxBytes, MaxItems, ReserveBytes uint64
}

var DefaultExtractLimits = ExtractLimits{MaxBytes: 10 << 30, MaxItems: 100000, ReserveBytes: 1 << 30}

// entryName 은 zip 항목 이름이다. UTF-8 플래그가 없고 UTF-8 로도 유효하지 않으면 CP949 로 읽는다(다른 클라이언트의 zip).
func entryName(f *zip.File) string {
	if f.NonUTF8 && !utf8.ValidString(f.Name) {
		if s, err := korean.EUCKR.NewDecoder().String(f.Name); err == nil {
			return s
		}
	}
	return f.Name
}

// safeRel 은 항목 이름을 풀 폴더 아래 상대 경로로 바꾼다. 절대 경로·".."·드라이브 문자면 거짓.
func safeRel(name string) (string, bool) {
	n := strings.TrimSuffix(strings.ReplaceAll(name, `\`, "/"), "/")
	if n == "" || strings.Contains(n, ":") || !filepath.IsLocal(filepath.FromSlash(n)) {
		return "", false
	}
	return filepath.FromSlash(n), true
}

// ExtractZip 은 zipPath 를 dest 폴더에 푼다. 먼저 전부 검사하고(경로 이탈·링크·항목 수) 문제가 있으면 아무것도 쓰지 않는다.
// 쓴 바이트는 실제로 세어 상한을 넘으면 멈춘다. 실패하면 dest 에 쓴 것을 남기지 않도록 부르는 쪽이 작업 폴더를 지운다.
func ExtractZip(ctx context.Context, zipPath, dest string, lim ExtractLimits, p *Progress) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fail(CodeUnzip, err)
	}
	defer r.Close()
	if uint64(len(r.File)) > lim.MaxItems {
		return fail(CodeUnzipItems, nil, fmt.Sprint(lim.MaxItems))
	}
	rels := make([]string, len(r.File))
	var want uint64
	for i, f := range r.File {
		rel, ok := safeRel(entryName(f))
		if !ok || f.Mode()&fs.ModeSymlink != 0 {
			return fail(CodeUnzipUnsafe, fmt.Errorf("항목 %q", f.Name))
		}
		rels[i] = rel
		want += f.UncompressedSize64
	}
	if free, err := files.FreeSpace(dest); err == nil && free < min(want, lim.MaxBytes)+lim.ReserveBytes {
		return fail(CodeUnzipDisk, nil)
	}
	if p != nil {
		p.Set(PhaseExtract, 0, int64(want))
	}
	var written uint64
	buf := make([]byte, 256<<10)
	for i, f := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		out := filepath.Join(dest, rels[i])
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return fail(CodeUnzip, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return fail(CodeUnzip, err)
		}
		n, err := extractOne(f, out, lim.MaxBytes-written, buf, p)
		written += n
		if f, ok := err.(*Fail); ok && f.Code == CodeUnzipSize {
			f.Arg = files.SizeText(lim.MaxBytes)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func extractOne(f *zip.File, out string, room uint64, buf []byte, p *Progress) (uint64, error) {
	src, err := f.Open()
	if err != nil {
		return 0, fail(CodeUnzip, err)
	}
	defer src.Close()
	dst, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fail(CodeUnzip, err)
	}
	// 헤더의 크기를 믿지 않고 상한 +1 까지만 읽어 넘는지 본다
	n, err := io.CopyBuffer(dst, &countReader{io.LimitReader(src, int64(min(room, 1<<62))+1), p}, buf)
	cerr := dst.Close()
	switch {
	case uint64(n) > room:
		return uint64(n), fail(CodeUnzipSize, nil)
	case err != nil:
		return uint64(n), fail(CodeUnzip, err)
	case cerr != nil:
		return uint64(n), fail(CodeUnzip, cerr)
	}
	if !f.Modified.IsZero() {
		os.Chtimes(out, time.Now(), f.Modified)
	}
	return uint64(n), nil
}

// MergeDir 는 src 폴더의 내용을 dst 로 옮긴다. 같은 상대 경로의 파일은 바꾸고 나머지는 둔다(원본 [덮어쓰기] 병합).
// 같은 디스크면 이름 바꾸기, 아니면 복사 뒤 삭제.
func MergeDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if st, err := os.Lstat(target); err == nil && st.IsDir() {
			return fmt.Errorf("같은 이름의 폴더가 있어 파일을 둘 수 없습니다: %s", target)
		}
		return moveFile(p, target)
	})
}

// moveFile 은 src 를 dst 로 옮긴다(dst 가 있으면 바꾼다). 새 내용을 옆에 다 쓴 뒤에 바꾸고 dst 를 지우지 않으므로 실패해도 dst 의 내용은 그대로다.
func moveFile(src, dst string) error {
	if err := replace(src, dst); err == nil {
		inheritACL(dst)
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := tempPart(dst) // 병합은 파일마다 자리를 잡지 않아 같은 파일을 받는 쪽과 겹칠 수 있다 ― 이름을 나눠 쓴다
	if err != nil {
		return err
	}
	tmp := out.Name()
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := replace(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	in.Close()
	return os.Remove(src)
}
