package transfer

import (
	"bufio"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"snbapp/internal/files"
	"snbapp/pb/serverfile"
)

// PartSuffix 는 받는 중인 파일의 꼬리다. 크기·검사값이 맞은 뒤에만 원래 이름으로 바꾼다.
const PartSuffix = ".snbpart"

// createPart 는 dest 옆에 쓰는 중 파일을 만든다. 보통은 dest+PartSuffix 이고, 꼬리를 붙이면 파일 이름 상한(255자)을 넘는 등 그 이름을 못 쓰면
// 짧은 임시 이름이다. ponytail: 임시 이름은 claim 의 찌꺼기 청소가 찾지 못한다 ― 긴 이름을 받다 강제 종료될 때만 남는다.
func createPart(dest string) (*os.File, error) {
	f, err := os.Create(dest + PartSuffix)
	if err == nil {
		return f, nil
	}
	// dest 자체를 못 만드는 이름이면 임시 이름으로 다 받아도 마지막 이름 바꾸기에서 실패한다 ― 받기 전에 끝낸다.
	probe, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		probe.Close()
		os.Remove(dest)
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if f, err = os.CreateTemp(filepath.Dir(dest), "snb*"+PartSuffix); err == nil {
		f.Chmod(0o644) // CreateTemp 는 0600 이다
	}
	return f, err
}

// 클립보드 받기 첫 응답의 code(원본 FileTransfer.cpp): 1 글, 2 그림, 3 묶음(.fmh).
const (
	ClipText  = 1
	ClipImage = 2
	ClipFmh   = 3
)

// DownloadSpec 은 받을 서버 파일 하나다.
//   - ServerName: spath(서버 이름). Fpath: 클립보드 받기면 "__CLIP__"
//   - Dest: 저장 경로(폴더면 풀 폴더, 클립보드면 확장자 없는 경로 ― 형식 응답, 없으면 서버 이름의 확장자로 .txt·.png·.bmp·.fmh 를 붙인다)
//   - Overwrite: 거짓이면 받기 직전에 자리를 다시 보고 겹치면 번호(_00000)를 붙인다. 참이면 바꾸고, 폴더는 병합
//   - Size: 서버 목록의 크기. 클립보드는 첫 응답의 length(암호화된 크기)로 채워 진행 표시에만 쓴다
//   - TempDir: 폴더 zip·작업 폴더 자리. Biz·Limit: 서버 거절 문구
type DownloadSpec struct {
	ServerName string
	Fpath      string
	Dest       string
	Size       int64
	Folder     bool
	Clip       bool
	Overwrite  bool
	TempDir    string
	Biz        bool
	Limit      string
	Limits     ExtractLimits // 영값이면 DefaultExtractLimits
}

// DownloadResult 는 받은 결과다. Format 은 클립보드 형식, ServerPath 는 클립보드 첫 응답의 서버 경로(받은 뒤 지울 때 쓴다).
type DownloadResult struct {
	Path       string
	ServerPath string
	Format     int32
	Checksum   string
}

// reserved 는 지금 받는 중인 저장 경로다 ― 같은 요청 안의 같은 이름이 동시에 같은 번호를 고르지 않게.
var reserved sync.Map

func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

func taken(p string) bool {
	_, busy := reserved.Load(pathKey(p))
	return busy || files.Exists(p)
}

// claim 은 dest(덮어쓰기가 아니면 번호를 붙인 빈 자리)를 잡는다. 놓기는 돌려준 함수.
func claim(dest string, overwrite bool) (string, func(), error) {
	// 아무도 받지 않는 자리의 .snbpart 는 지난 강제 종료의 찌꺼기다 ― 번호를 밀어내지 않게 치운다.
	if _, busy := reserved.LoadOrStore(pathKey(dest), true); !busy {
		os.Remove(dest + PartSuffix)
		reserved.Delete(pathKey(dest))
	}
	for range 100 {
		p := dest
		if !overwrite && taken(p) {
			var err error
			if p, err = files.NumberedPath(dest, taken); err != nil {
				return "", nil, fail(CodeOpen, err)
			}
		}
		if _, busy := reserved.LoadOrStore(pathKey(p), true); !busy {
			return p, func() { reserved.Delete(pathKey(p)) }, nil
		}
		if overwrite { // 같은 자리를 다른 받기가 덮어쓰는 중 ― 번호로 비킨다
			overwrite = false
		}
	}
	return "", nil, fail(CodeOpen, errors.New("저장 자리를 잡지 못했다"))
}

func isHexLen(s string, lo, hi int) bool {
	if len(s) < lo || len(s) > hi {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// clipExt 는 형식 응답으로, 없으면(spath 받기) 서버 경로 글자로 확장자를 고른다(원본 FileTransfer.cpp:1555-1563).
func clipExt(format int32, serverPath string) string {
	lower := strings.ToLower(serverPath)
	switch format {
	case ClipText:
		return ".txt"
	case ClipImage:
		if strings.Contains(lower, ".png") {
			return ".png"
		}
		return ".bmp"
	case ClipFmh:
		return ".fmh"
	}
	for _, ext := range []string{".txt", ".png", ".bmp"} {
		if strings.Contains(lower, ext) {
			return ext
		}
	}
	return ".fmh"
}

// Receive 는 서버 파일 하나를 받는다(docs/port/04 7.4·7.6). .snbpart 에 쓰며 검사값을 함께 계산하고,
// 크기(-977)·검사값(-45)이 맞을 때만 원래 이름으로 바꾼다. 실패·취소면 받던 파일을 지운다.
func Receive(ctx context.Context, c serverfile.ServerFileServiceClient, sp DownloadSpec, p *Progress) (*DownloadResult, error) {
	if sp.Limits == (ExtractLimits{}) {
		sp.Limits = DefaultExtractLimits
	}
	parent := ctx
	canceled := func(err error) error {
		if parent.Err() != nil {
			return ErrCanceled
		}
		return err
	}
	if sp.Size > 0 {
		if err := files.CheckSpace(filepath.Dir(sp.Dest), uint64(sp.Size)); err != nil {
			return nil, fail(CodeStorage, err)
		}
	}
	stream, err := c.FileDownload(ctx, &serverfile.FileDownloadRequest{Fpath: sp.Fpath, Spath: sp.ServerName})
	if err != nil {
		return nil, canceled(downloadStatusFail(err, sp.Biz, sp.Limit))
	}

	res := &DownloadResult{}
	var (
		part    string // 쓰는 중인 파일
		out     *os.File
		bw      *bufio.Writer
		h       hash.Hash
		got     int64
		release func()
		dest    string
	)
	cleanup := func() {
		if out != nil {
			out.Close()
			out = nil
		}
		if part != "" {
			os.Remove(part)
		}
		if release != nil {
			release()
			release = nil
		}
	}
	defer cleanup()
	open := func() error {
		target := sp.Dest
		if sp.Clip {
			target += clipExt(res.Format, cmp.Or(res.ServerPath, sp.ServerName))
		}
		if sp.Folder {
			if err := os.MkdirAll(sp.TempDir, 0o755); err != nil {
				return fail(CodeOpen, err)
			}
			f, err := os.CreateTemp(sp.TempDir, "dn-*.zip")
			if err != nil {
				return fail(CodeOpen, err)
			}
			out, part = f, f.Name()
		} else {
			d, rel, err := claim(target, sp.Overwrite)
			if err != nil {
				return err
			}
			dest, release = d, rel
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return fail(CodeOpen, err)
			}
			if out, err = createPart(dest); err != nil {
				return fail(CodeOpen, err)
			}
			part = out.Name()
		}
		bw = bufio.NewWriterSize(out, 1<<20)
		h = sha256.New()
		p.Set(PhaseRecv, 0, sp.Size)
		return nil
	}
	if !sp.Clip {
		if err := open(); err != nil {
			return nil, err
		}
	}
	w := func(b []byte) error {
		if _, err := bw.Write(b); err != nil {
			return err
		}
		h.Write(b)
		return nil
	}
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, canceled(downloadStatusFail(err, sp.Biz, sp.Limit))
		}
		if a := r.GetAnswer(); a != nil {
			switch {
			case a.Res != 1:
				return nil, fail(int(a.Code), errors.New(a.Reason), sp.Limit)
			case sp.Clip && out == nil && res.Format == 0 && !isHexLen(a.Reason, 32, 128): // 클립보드 첫 응답: 형식·서버 경로·크기(서버는 검사값을 먼저 보내기도 한다)
				res.Format, res.ServerPath = a.Code, a.Reason
				if sp.Size <= 0 && a.Length > 0 {
					sp.Size = int64(a.Length)
				}
			case sp.Clip:
				if isHexLen(a.Reason, 32, 128) {
					res.Checksum = a.Reason
				}
			case files.AcceptChecksum(a.Code, a.Reason):
				res.Checksum = a.Reason
			}
			continue
		}
		if out == nil {
			if err := open(); err != nil {
				return nil, err
			}
		}
		chunk := r.GetChunk()
		if err := w(chunk); err != nil {
			return nil, fail(CodeStorage, err)
		}
		got += int64(len(chunk))
		p.Add(int64(len(chunk)))
		// ponytail: 조각마다 GC 한 번(수 ms 이하). 서버의 8MiB 조각이 쓰레기로 쌓이면 30MB 받기에 힙이 파일보다 커진다
		// (측정: 34~40MB → 18MB, TestReceiveHeapBelowFile). 동시 받기가 많아 GC 시간이 보이면 조각 버퍼 재사용으로 바꾼다.
		if len(chunk) >= 1<<20 {
			runtime.GC()
		}
	}
	if out == nil { // 데이터 없이 끝났다(빈 파일이거나 클립보드 형식만)
		if sp.Clip && res.Format == 0 {
			return nil, fail(CodeNoData, nil)
		}
		if err := open(); err != nil {
			return nil, err
		}
	}
	if err := bw.Flush(); err != nil {
		return nil, fail(CodeStorage, err)
	}
	if err := out.Close(); err != nil {
		out = nil
		return nil, fail(CodeStorage, err)
	}
	out = nil
	if !sp.Clip && got != sp.Size { // 클립보드의 length 는 암호화된 크기라 대조하지 않는다(docs/port/07 3.2)
		return nil, fail(CodeSizeMismatch, nil, fmt.Sprint(got), fmt.Sprint(sp.Size))
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if res.Checksum != "" && !strings.EqualFold(res.Checksum, actual) {
		return nil, fail(CodeChecksum, nil, res.Checksum, actual)
	}
	if res.Checksum == "" {
		res.Checksum = actual
	}
	if sp.Folder {
		if err := ctx.Err(); err != nil {
			return nil, ErrCanceled
		}
		path, err := placeFolder(ctx, part, sp, p)
		if err != nil {
			return nil, canceled(err)
		}
		res.Path = path
		return res, nil
	}
	saved, err := commit(part, dest, sp.Overwrite)
	if err != nil {
		return nil, err
	}
	part = ""
	res.Path = saved
	return res, nil
}

// commit 은 .snbpart 를 dest 로 바꾸고 실제 저장 경로를 돌려준다. 덮어쓰기면 원래 파일을 지운 뒤 바꾸고, 못 바꾸면 번호 붙인 이름으로 남긴다.
func commit(part, dest string, overwrite bool) (string, error) {
	if overwrite {
		os.Remove(dest)
	}
	err := os.Rename(part, dest)
	if err == nil {
		return dest, nil
	}
	alt, nerr := files.NumberedPath(dest, taken)
	if nerr != nil {
		return "", fail(CodeOpen, nerr)
	}
	if err = os.Rename(part, alt); err != nil {
		return "", fail(CodeOpen, err)
	}
	return alt, nil
}

// placeFolder 는 받은 zip 을 작업 폴더에 안전하게 푼 뒤 저장 자리로 옮긴다.
// 덮어쓰기면 있는 폴더에 병합하고, 아니면 빈 자리(번호)에 새 폴더로 둔다. 새 폴더였는데 옮기다 실패하면 지운다.
func placeFolder(ctx context.Context, zipPath string, sp DownloadSpec, p *Progress) (string, error) {
	work, err := os.MkdirTemp(sp.TempDir, "x")
	if err != nil {
		return "", fail(CodeUnzip, err)
	}
	defer os.RemoveAll(work)
	if err := ExtractZip(ctx, zipPath, work, sp.Limits, p); err != nil {
		return "", err
	}
	dest, release, err := claim(sp.Dest, sp.Overwrite)
	if err != nil {
		return "", err
	}
	defer release()
	fresh := !files.Exists(dest)
	if fresh && os.Rename(work, dest) == nil {
		return dest, nil
	}
	if err := MergeDir(work, dest); err != nil {
		if fresh {
			os.RemoveAll(dest)
		}
		return "", fail(CodeUnzip, err)
	}
	return dest, nil
}
