package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"snbapp/internal/nbs"
	"snbapp/pb/common"
	"snbapp/pb/serverfile"
)

// Chunk 는 업로드 조각 크기다(원본 UPLOAD_STREAM_CHUNK_SIZE). 워커마다 버퍼 하나를 다시 쓴다.
const Chunk = 512 * 1024

// SessionCtx 는 parent(대기열의 취소)에 끊기면서 세션 토큰을 실은 스트림용 컨텍스트다.
func SessionCtx(parent context.Context, s *nbs.Session) (context.Context, context.CancelFunc) {
	ctx, cancel := s.Ctx(0)
	stop := context.AfterFunc(parent, cancel)
	return ctx, func() { stop(); cancel() }
}

// HashFile 은 SHA-256 소문자 16진이다(원본 calculateFileHash). 진행은 PhaseHash 로 p 에 쓴다.
func HashFile(ctx context.Context, path string, p *Progress) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fail(CodeOpen, err)
	}
	defer f.Close()
	if p != nil {
		st, _ := f.Stat()
		if st != nil {
			p.Set(PhaseHash, 0, st.Size())
		}
	}
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, rerr := f.Read(buf)
		h.Write(buf[:n])
		if p != nil && n > 0 {
			p.Add(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", fail(CodeRead, rerr)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// UploadSpec 은 보낼 파일 하나다.
//   - Info: 첫 메시지. Fsize 는 Send 가 연 파일 크기로 채운다
//   - Stamp: 검사 때 기준값. 영값이 아니면 첫 조각 전·끝 알림 전에 대조해 다르면 -978(원본 097)
//   - Stall: 보낸 바이트가 이만큼 늘지 않으면 -975(0 이면 끔, fmenv UPLOAD_STALL_SEC)
//   - Limit: 서버 크기 거절(-22·-24) 문구의 %1
type UploadSpec struct {
	Path  string
	Info  *serverfile.FileTransInfo
	Stamp Stamp
	Stall time.Duration
	Limit string
}

func (sp *UploadSpec) changed() bool {
	if sp.Stamp == (Stamp{}) {
		return false
	}
	now, err := StampOf(sp.Path)
	return err != nil || now != sp.Stamp
}

// Send 는 파일 하나를 FileUpload 스트림으로 보낸다(docs/port/03 7.3).
// 서버 거절(res≠1)은 받는 즉시 보내기를 멈춘다. 실패하면 CloseSend 하지 않고 ctx 를 끊는다 ―
// CloseSend 로 끝내면 서버가 잘린 파일을 정상으로 커밋한다(SR002).
func Send(ctx context.Context, c serverfile.ServerFileServiceClient, sp UploadSpec, p *Progress) error {
	f, err := os.Open(sp.Path)
	if err != nil {
		return fail(CodeOpen, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return fail(CodeOpen, err)
	}
	size := st.Size()
	if sp.changed() {
		return fail(CodeChanged, nil)
	}
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := c.FileUpload(ctx)
	if err != nil {
		return fail(CodeStreamOpen, err)
	}

	var rejected atomic.Pointer[common.AnswerResponse]
	recvDone := make(chan error, 1)
	go func() {
		for {
			a, err := stream.Recv()
			if err != nil {
				recvDone <- err
				return
			}
			if a.Res != 1 {
				rejected.Store(a)
				cancel()
				recvDone <- nil
				return
			}
		}
	}()
	var stalled, sending atomic.Bool
	sending.Store(true)
	p.Set(PhaseSend, 0, size)
	if sp.Stall > 0 {
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if sending.Load() && p.Idle() > sp.Stall {
						stalled.Store(true)
						cancel()
						return
					}
				}
			}
		}()
	}

	verdict := func(code int, cause error) error {
		if a := rejected.Load(); a != nil {
			if a.Code == 0 {
				return fail(CodeFailed, errors.New(a.Reason))
			}
			return fail(int(a.Code), errors.New(a.Reason), sp.Limit)
		}
		if stalled.Load() {
			return fail(CodeStall, cause, fmt.Sprint(int(sp.Stall.Seconds())))
		}
		if parent.Err() != nil {
			return ErrCanceled
		}
		return fail(code, cause)
	}
	// 보내기 실패는 서버가 스트림을 닫았을 수 있다 ― 진짜 사유는 Recv 에 있으니 잠깐 기다린다
	sendFailed := func(err error) error {
		select {
		case <-recvDone:
		case <-time.After(5 * time.Second):
		}
		cancel()
		return verdict(CodeBroken, err)
	}

	info := sp.Info
	info.Fsize = uint64(size)
	if err := stream.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Info{Info: info}}); err != nil {
		return sendFailed(err)
	}
	buf := make([]byte, Chunk)
	var sent int64
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			if err := stream.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Chunk{Chunk: buf[:n]}}); err != nil {
				return sendFailed(err)
			}
			sent += int64(n)
			p.Add(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			cancel()
			return verdict(CodeRead, rerr)
		}
	}
	sending.Store(false)
	if sp.changed() {
		cancel()
		return verdict(CodeChanged, nil)
	}
	if sent != size {
		cancel()
		return verdict(CodeBroken, fmt.Errorf("보낸 %d / %d 바이트", sent, size))
	}
	p.Set(PhaseCommit, sent, size)
	if err := stream.CloseSend(); err != nil {
		cancel()
		return verdict(CodeBroken, err)
	}
	err = <-recvDone
	if err == io.EOF {
		err = nil
	}
	if rejected.Load() != nil || err != nil {
		code := CodeFailed
		if status.Code(err) == codes.Unavailable {
			code = CodeBroken
		}
		return verdict(code, err)
	}
	return nil
}
