package nbs

// 메시지 암호화 코덱. 서버가 PreLogin 응답으로 암호 모드 1~5 를 주면 그 뒤 모든 메시지(직렬화한 protobuf 바이트 전체)를
// 암·복호화한다(docs/port/01 7.2). 모드 0(TLS 서버의 늘 그 값)이면 기본 proto 코덱에 그대로 넘겨 비용이 없다.
// 연결마다 하나다 ― 전역 RegisterCodec 을 쓰지 않는다.

import (
	"sync/atomic"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/encoding/protowire"
	gproto "google.golang.org/protobuf/proto"

	"snbapp/internal/nbs/crypt"
	"snbapp/pb/serverfile"
)

type cryptCodec struct {
	c    atomic.Pointer[crypt.Cipher] // nil = 모드 0
	base encoding.CodecV2
}

func newCryptCodec() *cryptCodec {
	return &cryptCodec{base: encoding.GetCodecV2(proto.Name)}
}

// NewCodec 은 앱 채널이 쓰는 코덱(모드 0)이다. 가짜 서버 시험이 앱과 같은 수신 길을 타게 한다.
func NewCodec() encoding.CodecV2 { return newCryptCodec() }

// Name 은 content-type application/grpc+proto 를 유지한다(서버는 이 이름의 우회 코덱을 등록해 둔다).
func (k *cryptCodec) Name() string { return proto.Name }

func (k *cryptCodec) Marshal(v any) (mem.BufferSlice, error) {
	c := k.c.Load()
	if c == nil {
		return k.base.Marshal(v)
	}
	b, err := gproto.Marshal(v.(gproto.Message))
	if err != nil || len(b) == 0 { // 빈 메시지는 암호화하지 않는다(원본 EncryptSerializer)
		return mem.BufferSlice{mem.SliceBuffer(b)}, err
	}
	enc, err := c.Encrypt(b)
	if err != nil {
		return nil, err
	}
	return mem.BufferSlice{mem.SliceBuffer(enc)}, nil
}

func (k *cryptCodec) Unmarshal(data mem.BufferSlice, v any) error {
	c := k.c.Load()
	down, isDown := v.(*serverfile.FileDownloadResponse)
	if c == nil && !isDown {
		return k.base.Unmarshal(data, v)
	}
	b := data.Materialize()
	if c != nil && len(b) > 0 {
		var err error
		if b, err = c.Decrypt(b); err != nil {
			return err // 그 메시지는 버린다(SEC-023) ― gRPC 가 Internal 오류로 돌려준다
		}
	}
	if isDown && aliasChunk(b, down) {
		return nil
	}
	return gproto.Unmarshal(b, v.(gproto.Message))
}

// aliasChunk 는 파일 받기 응답이 조각(oneof chunk, 번호 2) 하나뿐이면 복사하지 않고 b 를 가리키게 한다.
// proto.Unmarshal 은 8MiB 조각마다 같은 크기를 한 벌 더 만든다 ― 전송 메모리 ≤ 파일 크기를 지키려고 뺀다. b 는 이 메시지만의 버퍼다.
func aliasChunk(b []byte, r *serverfile.FileDownloadResponse) bool {
	num, typ, n := protowire.ConsumeTag(b)
	if n <= 0 || num != 2 || typ != protowire.BytesType {
		return false
	}
	chunk, m := protowire.ConsumeBytes(b[n:])
	if m <= 0 || n+m != len(b) {
		return false
	}
	r.Data = &serverfile.FileDownloadResponse_Chunk{Chunk: chunk}
	return true
}
