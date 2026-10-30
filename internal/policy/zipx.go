package policy

import (
	"bufio"
	"compress/bzip2"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
	"github.com/yeka/zip"
)

// ZIP·ZIPX 압축 방식. 표준은 Store(0)·Deflate(8) 만이라 ZIPX 가 쓰는 방식을 더한다.
// 98 PPMd·97 WavPack·96 JPEG·9 Deflate64 는 순수 Go 해독기가 없어 "검사할 수 없는 압축" 으로 판정한다.
const (
	methodBzip2 = 12
	methodLZMA  = 14
	methodZstd  = 93
	methodXZ    = 95
)

// zipDecomp 는 이 패키지가 읽을 수 있는 방식이다. yeka/zip 의 전역 등록부에도 같은 것을 넣는다(init).
var zipDecomp = map[uint16]func(io.Reader) io.ReadCloser{
	zip.Store:   io.NopCloser,
	zip.Deflate: flate.NewReader,
	methodBzip2: func(r io.Reader) io.ReadCloser { return io.NopCloser(bzip2.NewReader(r)) },
	methodLZMA:  newZipLZMA,
	methodZstd:  newZipZstd,
	methodXZ:    newZipXZ,
}

// init 은 yeka/zip 의 전역 등록부에 ZIPX 방식을 더한다. 다른 패키지가 같은 방식을 먼저 넣었으면 그것을 쓴다(중복 등록은 패닉이다).
func init() {
	for _, m := range []uint16{methodBzip2, methodLZMA, methodZstd, methodXZ} {
		func() {
			defer func() { _ = recover() }()
			zip.RegisterDecompressor(m, zip.Decompressor(zipDecomp[m]))
		}()
	}
}

// errReader 는 열 때 난 오류를 첫 읽기에서 돌려준다.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
func (e errReader) Close() error             { return nil }

// newZipLZMA 는 ZIP 의 LZMA(14): 판 2바이트 + 속성 길이 2바이트 + 속성 5바이트 뒤에 압축 자료가 온다.
// 크기를 모르는 .lzma 머리를 만들어 ulikunitz/lzma 에 넘기고, 끝 표시 없이 끝난 자료의 '예기치 않은 끝' 은 끝으로 본다
// (실제 크기·CRC 는 zip 의 checksumReader 가 확인한다).
func newZipLZMA(r io.Reader) io.ReadCloser {
	var h [9]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return errReader{err}
	}
	if binary.LittleEndian.Uint16(h[2:4]) != 5 {
		return errReader{errors.New("zip lzma: 속성 길이가 5 가 아니다")}
	}
	var hdr [13]byte
	copy(hdr[:5], h[4:9])
	for i := 5; i < 13; i++ {
		hdr[i] = 0xFF // 크기 모름
	}
	lr, err := lzma.ReaderConfig{DictCap: 1 << 27}.NewReader(io.MultiReader(bytesReader(hdr[:]), r))
	if err != nil {
		return errReader{err}
	}
	return io.NopCloser(eofTolerant{lr})
}

type eofTolerant struct{ r io.Reader }

func (e eofTolerant) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

func newZipZstd(r io.Reader) io.ReadCloser {
	d, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(1<<30))
	if err != nil {
		return errReader{err}
	}
	return d.IOReadCloser()
}

func newZipXZ(r io.Reader) io.ReadCloser {
	x, err := xz.NewReader(bufio.NewReader(r))
	if err != nil {
		return errReader{err}
	}
	return io.NopCloser(x)
}

func bytesReader(b []byte) io.Reader { return &sliceReader{b} }

type sliceReader struct{ b []byte }

func (s *sliceReader) Read(p []byte) (int, error) {
	if len(s.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.b)
	s.b = s.b[n:]
	return n, nil
}

// zipCryptoReader 는 전통 ZIP 암호(ZipCrypto)를 흘려 읽으며 푼다. yeka/zip 의 ZipCryptoDecryptor 는 항목 하나를 통째로
// 메모리에 올리므로(압축 폭탄·큰 파일에서 위험) 쓰지 않는다. 키 갱신은 yeka/zip 의 ZipCrypto 를 그대로 쓴다.
type zipCryptoReader struct {
	r io.Reader
	z *zip.ZipCrypto
}

func (c *zipCryptoReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		copy(p, c.z.Decrypt(p[:n]))
	}
	return n, err
}

// errWrongPassword 는 비밀번호가 틀렸다는 뜻이다(머리 12바이트 확인 바이트 또는 CRC 불일치).
var errWrongPassword = errors.New("zip: 비밀번호가 틀렸다")

// hasAESExtra 는 WinZip AES 추가 칸(0x9901)이 있는가다.
func hasAESExtra(extra []byte) bool {
	for len(extra) >= 4 {
		id := binary.LittleEndian.Uint16(extra[0:2])
		n := int(binary.LittleEndian.Uint16(extra[2:4]))
		if id == 0x9901 {
			return true
		}
		if 4+n > len(extra) {
			return false
		}
		extra = extra[4+n:]
	}
	return false
}

// openZipFile 은 항목 하나를 연다(ra 는 zip 파일 자체). 암호 항목이면 password 로 푼다. 다 읽으면 CRC·인증값을 확인한다.
func openZipFile(ra io.ReaderAt, f *zip.File, password string) (io.ReadCloser, error) {
	dec, ok := zipDecomp[f.Method]
	if !ok {
		return nil, zip.ErrAlgorithm
	}
	if !f.IsEncrypted() {
		return f.Open()
	}
	if password == "" {
		return nil, errWrongPassword
	}
	f.SetPassword(password)
	if hasAESExtra(f.Extra) {
		// WinZip AES: 확인값으로 틀린 비밀번호를 곧바로 안다(yeka 의 ErrPassword). 인증값은 다 읽은 끝에서 본다 ―
		// DeferAuth 가 거짓이면 yeka 가 항목을 통째로 메모리에 올린다.
		f.DeferAuth = true
		return f.Open()
	}
	off, err := f.DataOffset()
	if err != nil {
		return nil, err
	}
	sr := io.NewSectionReader(ra, off, int64(f.CompressedSize64))
	z := zip.NewZipCrypto([]byte(password))
	var hdr [12]byte
	if _, err := io.ReadFull(sr, hdr[:]); err != nil {
		return nil, err
	}
	plain := z.Decrypt(hdr[:])
	check := byte(f.CRC32 >> 24)
	if f.Flags&0x8 != 0 { // 자료 기술자가 있으면 수정 시각 높은 바이트로 확인한다
		check = byte(f.ModifiedTime >> 8)
	}
	if plain[11] != check {
		return nil, errWrongPassword
	}
	rc := dec(&zipCryptoReader{r: sr, z: z})
	return &crcReader{rc: rc, want: f.CRC32, size: f.UncompressedSize64, h: crc32.NewIEEE()}, nil
}

type crcReader struct {
	rc   io.ReadCloser
	h    hash.Hash32
	want uint32
	size uint64
	n    uint64
}

func (c *crcReader) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.h.Write(p[:n])
	c.n += uint64(n)
	if err == io.EOF {
		if c.n != c.size {
			return n, io.ErrUnexpectedEOF
		}
		if c.h.Sum32() != c.want {
			return n, errWrongPassword
		}
	}
	return n, err
}

func (c *crcReader) Close() error { return c.rc.Close() }
