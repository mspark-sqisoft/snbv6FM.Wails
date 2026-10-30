package policy

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/ulikunitz/xz"
	"github.com/yeka/zip"
)

// ZipCode 는 압축 검사 결과다(원본 ZipExtractor::E_ZIP_ERROR 와 앱 코드). 화면 문구는 ZipText.
type ZipCode int

const (
	ZipOK               ZipCode = 1
	ZipCancel           ZipCode = -999
	ZipNoFile           ZipCode = -995
	ZipNotArchive       ZipCode = -994
	ZipDepth            ZipCode = -991
	ZipPassword         ZipCode = -990 // 바깥이 잠김(또는 넣은 비밀번호가 틀림)
	ZipUnknown          ZipCode = -989
	ZipUnsafe           ZipCode = -987 // 경로 탈출·링크 ― 아무것도 풀지 않았다
	ZipInnerPassword    ZipCode = -986 // 안쪽 압축이 잠김
	ZipLimitSize        ZipCode = -985
	ZipLimitItems       ZipCode = -984
	ZipDisk             ZipCode = -983
	ZipUncheckable      ZipCode = -982 // 순수 Go 로 풀 수 없는 형식·방식(LZH, ZIPX PPMd 등). Wails 판에서 더했다
	ZipInnerUncheckable ZipCode = -981 // 안쪽 압축이 검사할 수 없는 형식. Wails 판에서 더했다
	ZipPwWrong          ZipCode = -1001
	ZipPwCancel         ZipCode = -1002
)

// ZipText 는 압축 결과 문구다(원본 getZipErrText). 원본의 영문 기본값 Unknown 은 '알 수 없는 오류' 로 맞췄다.
func ZipText(c ZipCode) string {
	switch c {
	case ZipOK:
		return "압축 해제 성공"
	case ZipCancel:
		return "사용자 취소"
	case -998, -997:
		return "압축 실패"
	case -996:
		return "압축 모듈 없음"
	case ZipNoFile:
		return "파일 없음"
	case ZipNotArchive:
		return "압축파일 아님"
	case -993:
		return "디렉토리 없음"
	case -992:
		return "압축 대상 중복"
	case ZipDepth:
		return "압축 횟수 초과"
	case ZipPassword, ZipInnerPassword:
		return "압축 패스워드 존재"
	case -988:
		return "압축 모듈 파라미터 오류"
	case ZipUnsafe:
		return "허용되지 않는 경로 포함"
	case ZipLimitSize, ZipLimitItems:
		return "압축 해제 상한 초과"
	case ZipDisk:
		return "압축 해제 공간 부족"
	case ZipUncheckable, ZipInnerUncheckable:
		return "검사할 수 없는 압축 형식"
	case ZipPwWrong:
		return "압축 비밀번호 오류"
	case ZipPwCancel:
		return "압축 비밀번호 입력 취소"
	}
	return "알 수 없는 오류"
}

// archiveExts 는 압축으로 보는 확장자다. 원본 7종(ZIP·ZIPX·TAR·GZ·LZH·7Z·XZ)에 TGZ·BZ2 를 더했다(ARCHITECTURE "압축 검사").
// RAR·CAB·ISO 는 원본처럼 일반 파일로 본다.
var archiveExts = map[string]bool{"ZIP": true, "ZIPX": true, "TAR": true, "GZ": true, "TGZ": true, "BZ2": true, "XZ": true, "7Z": true, "LZH": true}

// IsArchiveName 은 이름의 확장자로 압축인가를 본다(내용은 보지 않는다).
func IsArchiveName(name string) bool { return archiveExts[FileExt(name)] }

// Limits 는 풀기 한 번(중첩 포함)의 상한이다(원본 098 defaultLimits).
type Limits struct {
	MaxBytes     uint64 // 풀린 크기 합
	MaxItems     uint64 // 항목(파일·폴더) 수
	ReserveBytes uint64 // 임시 폴더 디스크에 남길 여유
}

// DefaultLimits 는 10GiB·10만 개·여유 1GiB 다.
var DefaultLimits = Limits{MaxBytes: 10 << 30, MaxItems: 100000, ReserveBytes: 1 << 30}

// Stop 은 상한에 걸려 멈춘 사유다.
type Stop struct {
	Reason string // "크기 상한" · "항목 수 상한" · "디스크 공간"
	Limit  uint64
	Bytes  uint64
	Items  uint64
	Free   uint64
}

// LogLine 은 원본과 같은 형식의 경고 한 줄이다(where: 업로드 검사 · 결재 내용 보기).
func (s *Stop) LogLine(where, archive string) string {
	line := fmt.Sprintf("압축 풀기 중단 >>> 사유[%s], 경로[%s], 압축파일[%s], 상한[%d], 풀린 크기[%d], 풀린 항목[%d]",
		s.Reason, where, filepath.Base(archive), s.Limit, s.Bytes, s.Items)
	if s.Reason == "디스크 공간" {
		line += fmt.Sprintf(", 남은 공간[%d]", s.Free)
	}
	return line
}

// Entry 는 압축 안의 파일 하나다(폴더 항목은 넣지 않는다). 안쪽 압축 파일 자신도 들어간다.
type Entry struct {
	Path string // 압축 안 경로(중첩이면 "안쪽.zip/경로")
	Name string // 파일 이름
	Size int64  // 목록에 적힌 크기(스트림 형식은 실제로 푼 크기)
	Head []byte // 앞 256바이트
}

// InspectOptions 는 압축 검사 설정이다.
type InspectOptions struct {
	Password string
	MaxDepth int    // 0 이면 2
	TempDir  string // 안쪽 압축을 잠시 풀 폴더(비면 os.TempDir)
	Limits   Limits // 0 값이면 DefaultLimits
	Log      func(string)
}

// Inspect 는 압축 path 의 안을 훑어 파일 목록을 만든다. 잎 파일은 디스크에 쓰지 않고 앞 256바이트만 읽는다
// (안쪽 압축만 임시 파일). 비밀번호가 있으면 모든 항목을 끝까지 읽어 CRC·인증값으로 비밀번호를 확인한다(원본 TestPassword).
// 위험한 항목(절대 경로·드라이브·`..`·링크)이 하나라도 있으면 아무것도 읽지 않고 ZipUnsafe.
func Inspect(ctx context.Context, path string, o InspectOptions) ([]Entry, ZipCode, *Stop) {
	if o.MaxDepth <= 0 {
		o.MaxDepth = 2
	}
	if o.Limits == (Limits{}) {
		o.Limits = DefaultLimits
	}
	if o.TempDir == "" {
		o.TempDir = os.TempDir()
	}
	in := &inspector{ctx: ctx, o: o}
	if _, err := os.Stat(path); err != nil {
		return nil, ZipNoFile, nil
	}
	c := in.archive(path, filepath.Base(path), "", 0)
	if c != ZipOK {
		return nil, c, in.stop
	}
	return in.entries, ZipOK, nil
}

type inspector struct {
	ctx      context.Context
	o        InspectOptions
	declared uint64 // 목록 크기 합
	written  uint64 // 실제로 푼 양
	items    uint64
	entries  []Entry
	stop     *Stop
}

func (in *inspector) log(s string) {
	if in.o.Log != nil {
		in.o.Log(s)
	}
}

func (in *inspector) stopFor(reason string, limit uint64) ZipCode {
	in.stop = &Stop{Reason: reason, Limit: limit, Bytes: max(in.written, in.declared), Items: in.items}
	switch reason {
	case "크기 상한":
		return ZipLimitSize
	case "항목 수 상한":
		return ZipLimitItems
	}
	return ZipDisk
}

// declare 는 목록 단계의 항목 수·크기를 센다.
func (in *inspector) declare(items, bytes uint64) ZipCode {
	in.items += items
	in.declared += bytes
	if in.items > in.o.Limits.MaxItems {
		return in.stopFor("항목 수 상한", in.o.Limits.MaxItems)
	}
	if in.declared > in.o.Limits.MaxBytes {
		return in.stopFor("크기 상한", in.o.Limits.MaxBytes)
	}
	return ZipOK
}

var errLimit = errors.New("상한")

// meter 는 실제로 푼 양을 세며 상한·취소를 본다.
type meter struct {
	in *inspector
	r  io.Reader
}

func (m *meter) Read(p []byte) (int, error) {
	if err := m.in.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := m.r.Read(p)
	m.in.written += uint64(n)
	if m.in.written > m.in.o.Limits.MaxBytes {
		m.in.stopFor("크기 상한", m.in.o.Limits.MaxBytes)
		return n, errLimit
	}
	return n, err
}

var (
	dotsOnly = regexp.MustCompile(`^\.{2,} *$`)
	drive    = regexp.MustCompile(`^[A-Za-z]:`)
)

// unsafeName 은 풀 폴더 밖을 가리킬 수 있는 이름이다(원본 findUnsafeEntry). `..` 는 결국 안쪽이어도 거절한다.
func unsafeName(name string) bool {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || drive.MatchString(name) {
		return true
	}
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if dotsOnly.MatchString(seg) {
			return true
		}
	}
	return false
}

func (in *inspector) rejectUnsafe(archive, name string) ZipCode {
	in.log("압축파일 경로 검사 거절(허용되지 않는 경로 또는 링크) >>> 압축파일: " + archive + ", 항목: " + name)
	return ZipUnsafe
}

func pwCode(depth int) ZipCode {
	if depth > 0 {
		return ZipInnerPassword
	}
	return ZipPassword
}

// archive 는 압축 하나를 연다. name 은 형식을 고르는 이름(임시 파일이면 원래 이름), prefix 는 결과 경로 앞에 붙일 것.
func (in *inspector) archive(path, name, prefix string, depth int) ZipCode {
	if depth >= in.o.MaxDepth {
		return ZipDepth
	}
	if in.ctx.Err() != nil {
		return ZipCancel
	}
	switch FileExt(name) {
	case "ZIP", "ZIPX":
		return in.zip(path, prefix, depth)
	case "7Z":
		return in.sevenZip(path, prefix, depth)
	case "TAR":
		return in.tar(path, prefix, depth)
	case "GZ", "TGZ", "BZ2", "XZ":
		return in.stream(path, name, prefix, depth)
	}
	return ZipUncheckable // LZH 등
}

// leaf 는 잎 파일 하나를 넣는다. full 이면 끝까지 읽는다(비밀번호 확인·스트림 크기). 돌려준 크기는 읽은 양(full 일 때).
func (in *inspector) leaf(r io.Reader, path, name string, declared int64, full bool) (int64, error) {
	head := make([]byte, HeadLen)
	mr := &meter{in: in, r: r}
	n, err := io.ReadFull(mr, head)
	size := int64(n)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	} else if err == nil && full {
		var rest int64
		rest, err = io.Copy(io.Discard, mr)
		size += rest
	}
	if err != nil {
		return 0, err
	}
	if !full && declared >= 0 {
		size = declared
	}
	in.entries = append(in.entries, Entry{Path: path, Name: name, Size: size, Head: head[:n]})
	return size, nil
}

// nested 는 안쪽 압축을 임시 파일로 끝까지 풀고(쓴 양·디스크 여유를 센다) 그 안을 훑는다. 안쪽 압축 자신도 목록에 넣는다.
// encrypted 는 바깥 항목이 잠겼는가(읽기 오류를 비밀번호 오류로 볼지), verify 는 다 푼 뒤 바깥 형식이 할 확인(7z CRC, nil 가능)이다.
func (in *inspector) nested(r io.Reader, path, name string, declared int64, depth int, encrypted bool, verify func() ZipCode) ZipCode {
	if err := os.MkdirAll(in.o.TempDir, 0o700); err != nil {
		return ZipUnknown
	}
	if free, ok := diskFree(in.o.TempDir); ok && free < uint64(max(declared, 0))+in.o.Limits.ReserveBytes {
		c := in.stopFor("디스크 공간", in.o.Limits.ReserveBytes)
		in.stop.Free = free
		return c
	}
	tmp, err := os.CreateTemp(in.o.TempDir, "chk-*")
	if err != nil {
		return ZipUnknown
	}
	defer os.Remove(tmp.Name())
	w := &diskGuard{in: in, f: tmp}
	n, err := io.Copy(w, &meter{in: in, r: r})
	tmp.Close()
	if err != nil {
		if in.stop != nil {
			return in.stopCode()
		}
		if in.ctx.Err() != nil {
			return ZipCancel
		}
		return readErr(err, depth, encrypted)
	}
	if verify != nil {
		if c := verify(); c != ZipOK {
			return c
		}
	}
	f, err := os.Open(tmp.Name())
	if err != nil {
		return ZipUnknown
	}
	head := make([]byte, HeadLen)
	hn, _ := io.ReadFull(f, head)
	f.Close()
	in.entries = append(in.entries, Entry{Path: path, Name: name, Size: n, Head: head[:hn]})
	c := in.archive(tmp.Name(), name, path+"/", depth+1)
	if c == ZipUncheckable {
		c = ZipInnerUncheckable // 안쪽은 정책과 상관없이 막는다(안쪽 암호 압축 -986 과 같다)
	}
	return c
}

func (in *inspector) stopCode() ZipCode {
	switch in.stop.Reason {
	case "크기 상한":
		return ZipLimitSize
	case "항목 수 상한":
		return ZipLimitItems
	}
	return ZipDisk
}

// diskGuard 는 64MiB 쓸 때마다 디스크 여유를 본다.
type diskGuard struct {
	in      *inspector
	f       *os.File
	pending int64
}

func (d *diskGuard) Write(p []byte) (int, error) {
	d.pending += int64(len(p))
	if d.pending >= 64<<20 {
		d.pending = 0
		if free, ok := diskFree(d.in.o.TempDir); ok && free < d.in.o.Limits.ReserveBytes {
			d.in.stopFor("디스크 공간", d.in.o.Limits.ReserveBytes)
			d.in.stop.Free = free
			return 0, errLimit
		}
	}
	return d.f.Write(p)
}

// readErr 는 항목을 읽다 난 오류를 결과로 바꾼다. 비밀번호를 쓰는 중이거나 잠긴 항목이면 비밀번호 오류로 본다.
func readErr(err error, depth int, encrypted bool) ZipCode {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ZipCancel
	}
	if errors.Is(err, zip.ErrAlgorithm) {
		return ZipUncheckable
	}
	if errors.Is(err, errWrongPassword) || errors.Is(err, zip.ErrPassword) || errors.Is(err, zip.ErrAuthentication) {
		return pwCode(depth)
	}
	var re *sevenzip.ReadError
	if errors.As(err, &re) && re.Encrypted {
		return pwCode(depth)
	}
	if encrypted {
		return pwCode(depth)
	}
	return ZipUnknown
}

func (in *inspector) zip(path, prefix string, depth int) ZipCode {
	f, err := os.Open(path)
	if err != nil {
		return ZipNoFile
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ZipUnknown
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return ZipNotArchive
		}
		return ZipUnknown
	}
	var total uint64
	encrypted := false
	for _, zf := range zr.File {
		if unsafeName(zf.Name) || zf.Mode()&fs.ModeSymlink != 0 {
			return in.rejectUnsafe(path, zf.Name)
		}
		total += zf.UncompressedSize64
		encrypted = encrypted || zf.IsEncrypted()
	}
	if c := in.declare(uint64(len(zr.File)), total); c != ZipOK {
		return c
	}
	pw := in.o.Password
	if depth > 0 && encrypted {
		return ZipInnerPassword // 안쪽이 잠긴 것은 묻지 않고 막는다(바깥 비밀번호를 안에 다시 쓰지 않는다)
	}
	if encrypted && pw == "" {
		return ZipPassword
	}
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || strings.HasSuffix(zf.Name, "/") {
			continue
		}
		if in.ctx.Err() != nil {
			return ZipCancel
		}
		if _, ok := zipDecomp[zf.Method]; !ok {
			return ZipUncheckable
		}
		rc, err := openZipFile(f, zf, pw)
		if err != nil {
			return readErr(err, depth, zf.IsEncrypted())
		}
		name := filepath.Base(strings.ReplaceAll(zf.Name, "\\", "/"))
		p := prefix + zf.Name
		var c ZipCode = ZipOK
		declared := int64(zf.UncompressedSize64)
		if IsArchiveName(name) {
			c = in.nested(rc, p, name, declared, depth, zf.IsEncrypted(), nil)
		} else if n, err := in.leaf(io.LimitReader(rc, declared+1), p, name, declared, pw != ""); err != nil {
			c = in.failCode(err, depth, zf.IsEncrypted())
		} else if pw != "" && n != declared {
			// 목록보다 많이 풀렸다(yeka 의 확인은 초과를 못 잡는다) ― 잠긴 항목이면 비밀번호 오류, 아니면 손상
			c = readErr(errors.New("zip: 목록 크기와 다르다"), depth, zf.IsEncrypted())
		}
		rc.Close()
		if c != ZipOK {
			return c
		}
	}
	return ZipOK
}

func (in *inspector) failCode(err error, depth int, encrypted bool) ZipCode {
	if in.stop != nil {
		return in.stopCode()
	}
	if in.ctx.Err() != nil {
		return ZipCancel
	}
	return readErr(err, depth, encrypted)
}

func (in *inspector) sevenZip(path, prefix string, depth int) ZipCode {
	pw := in.o.Password
	if depth > 0 {
		pw = "" // 바깥 비밀번호를 안쪽에 다시 쓰지 않는다
	}
	r, err := sevenzip.OpenReaderWithPassword(path, pw)
	if err != nil {
		var re *sevenzip.ReadError
		switch {
		case errors.As(err, &re) && re.Encrypted, strings.Contains(err.Error(), "checksum") && pw != "",
			strings.Contains(strings.ToLower(err.Error()), "password"):
			return pwCode(depth) // 헤더까지 잠긴 7z 는 목록조차 못 읽는다
		case strings.Contains(err.Error(), "not a valid 7-zip"):
			return ZipNotArchive
		}
		return ZipUnknown
	}
	defer r.Close()
	var total uint64
	for _, zf := range r.File {
		if unsafeName(zf.Name) || zf.Mode()&fs.ModeSymlink != 0 {
			return in.rejectUnsafe(path, zf.Name)
		}
		total += zf.UncompressedSize
	}
	if c := in.declare(uint64(len(r.File)), total); c != ZipOK {
		return c
	}
	for _, zf := range r.File {
		if zf.Mode().IsDir() {
			continue
		}
		if in.ctx.Err() != nil {
			return ZipCancel
		}
		rc, err := zf.Open()
		if err != nil {
			return readErr(err, depth, pw != "")
		}
		name := filepath.Base(strings.ReplaceAll(zf.Name, "\\", "/"))
		p := prefix + zf.Name
		// 7z 는 항목 CRC 를 읽는 쪽이 확인해야 한다. 무압축(copy)+AES 는 비밀번호 없이·틀리게 읽어도 오류 없이 엉뚱한 바이트가
		// 나오므로(sevenzip README "Detecting the wrong password") 항상 끝까지 읽어 CRC 를 대조한다. 어긋나면 잠긴 것으로 본다.
		h := crc32.NewIEEE()
		src := io.TeeReader(rc, h)
		declared := int64(zf.UncompressedSize)
		crcOK := func() ZipCode {
			if zf.CRC32 != 0 && h.Sum32() != zf.CRC32 {
				return pwCode(depth)
			}
			return ZipOK
		}
		var c ZipCode = ZipOK
		if IsArchiveName(name) {
			c = in.nested(src, p, name, declared, depth, pw != "", crcOK)
		} else if n, err := in.leaf(src, p, name, declared, true); err != nil {
			c = in.failCode(err, depth, pw != "")
		} else if n != declared {
			c = readErr(errors.New("7z: 목록 크기와 다르다"), depth, pw != "")
		} else {
			c = crcOK()
		}
		rc.Close()
		if c != ZipOK {
			return c
		}
	}
	return ZipOK
}

func (in *inspector) tar(path, prefix string, depth int) ZipCode {
	f, err := os.Open(path)
	if err != nil {
		return ZipNoFile
	}
	defer f.Close()
	// 1차: 머리만 훑어 위험 항목·상한을 본다(파일이라 자료는 Seek 로 건너뛴다)
	tr := tar.NewReader(f)
	var items, total uint64
	for first := true; ; first = false {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if first {
				return ZipNotArchive
			}
			return ZipUnknown
		}
		if unsafeName(h.Name) || h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink {
			return in.rejectUnsafe(path, h.Name)
		}
		items++
		if h.Typeflag == tar.TypeReg {
			total += uint64(max(h.Size, 0))
		}
		if in.ctx.Err() != nil {
			return ZipCancel
		}
	}
	if c := in.declare(items, total); c != ZipOK {
		return c
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ZipUnknown
	}
	tr = tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return ZipOK
		}
		if err != nil {
			return ZipUnknown
		}
		if h.Typeflag != tar.TypeReg {
			continue // 폴더·장치 등은 세지 않는다
		}
		name := filepath.Base(strings.ReplaceAll(h.Name, "\\", "/"))
		p := prefix + h.Name
		var c ZipCode = ZipOK
		if IsArchiveName(name) {
			c = in.nested(tr, p, name, h.Size, depth, false, nil)
		} else if _, err := in.leaf(tr, p, name, h.Size, false); err != nil {
			c = in.failCode(err, depth, false)
		}
		if c != ZipOK {
			return c
		}
	}
}

// innerName 은 단일 스트림 압축이 풀어 낼 이름이다. a.tar.gz → a.tar, a.tgz → a.tar, a.gz → a (gzip 머리의 원래 이름이 있으면 그것).
func innerName(name, header string) string {
	if header != "" {
		if b := filepath.Base(strings.ReplaceAll(header, "\\", "/")); b != "." && b != "/" && !unsafeName(b) {
			return b
		}
	}
	ext := FileExt(name)
	stem := name[:len(name)-len(ext)-1]
	if ext == "TGZ" {
		return stem + ".tar"
	}
	return stem
}

func (in *inspector) stream(path, name, prefix string, depth int) ZipCode {
	f, err := os.Open(path)
	if err != nil {
		return ZipNoFile
	}
	defer f.Close()
	br := bufio.NewReader(f)
	var r io.Reader
	header := ""
	switch FileExt(name) {
	case "GZ", "TGZ":
		gz, err := gzip.NewReader(br)
		if err != nil {
			return ZipNotArchive
		}
		defer gz.Close()
		header, r = gz.Name, gz
	case "BZ2":
		if b, _ := br.Peek(3); string(b) != "BZh" {
			return ZipNotArchive
		}
		r = bzip2.NewReader(br)
	case "XZ":
		x, err := xz.NewReader(br)
		if err != nil {
			return ZipNotArchive
		}
		r = x
	}
	inner := innerName(name, header)
	if c := in.declare(1, 0); c != ZipOK {
		return c
	}
	p := prefix + inner
	if IsArchiveName(inner) {
		return in.nested(r, p, inner, -1, depth, false, nil)
	}
	if _, err := in.leaf(r, p, inner, -1, true); err != nil {
		return in.failCode(err, depth, false)
	}
	return ZipOK
}
