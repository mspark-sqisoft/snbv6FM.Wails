// Package crypt 는 NBS 메시지 암호 모드 1~5 다(protobuf 로 직렬화한 메시지 바이트 전체를 암·복호화한다).
// 서버 snBv6Srv/go/crypt2 와 같은 알고리즘이다(docs/port/01 7.2·7.3). TLS 로 띄운 서버는 늘 모드 0 이라 실제로는
// TLS 를 끈 서버에서만 쓰인다. 서버처럼 상태를 남기지 않는다 ― 메시지마다 같은 키·IV 로 새로 시작한다.
package crypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"errors"
	"fmt"

	"snbapp/internal/nbs/crypt/aria"
)

// 암호 모드(PreLoginResponse.enc_mode).
const (
	ModeNone    = 0
	ModeSEED256 = 1
	ModeARIA256 = 2
	ModeAES256  = 3
	ModeSEED128 = 4
	ModeHIGHT   = 5
	ModeNPARIA  = 6 // 벤더 DLL 이 필요해 지원하지 않는다(docs/port/01 7.4)
)

// ErrNPAria 는 모드 6 이다. 문구는 화면에 그대로 낸다.
var ErrNPAria = errors.New("서버 암호 모드 6 은 지원하지 않습니다.")

// errCipher 는 복호 실패(길이·패딩)다. 그 메시지는 버린다(SEC-023).
var errCipher = errors.New("암호문을 풀지 못했습니다")

// Params 는 키 재료다(PreLoginResponse 의 en_* 6개).
type Params struct {
	SvcID   int
	SvcNo   int
	MyIP    string
	SvcIP   string
	SvcPort int
	SiteAbb string
}

// PreLoginParams 는 TLS 를 끈 서버에 PreLogin 을 보낼 때의 고정 키 재료다(모드 3 으로 쓴다).
func PreLoginParams() Params {
	return Params{SvcID: 22, SvcNo: 1, MyIP: "127.0.0.1", SvcIP: "127.0.0.1", SvcPort: 22, SiteAbb: "SSBR"}
}

// Supported 는 이 앱이 받을 수 있는 모드인가다(0~5).
func Supported(mode int32) bool { return mode >= ModeNone && mode <= ModeHIGHT }

// Cipher 는 모드 하나의 메시지 암·복호다. 여러 고루틴이 함께 써도 된다(상태 없음).
type Cipher struct {
	mode    int32
	key, iv []byte // 32바이트씩. 모드마다 앞부분만 쓴다
}

// New 는 mode 의 Cipher 다. 모드 0 이면 (nil, nil) ― 암호화하지 않는다.
func New(mode int32, p Params) (*Cipher, error) {
	switch {
	case mode == ModeNone:
		return nil, nil
	case mode == ModeNPARIA:
		return nil, ErrNPAria
	case !Supported(mode):
		return nil, fmt.Errorf("서버 암호 모드 %d 은 알 수 없습니다.", mode)
	}
	if err := ensureSeedTables(); err != nil {
		return nil, err
	}
	key, iv, err := derive(p, mode == ModeSEED128)
	if err != nil {
		return nil, err
	}
	return &Cipher{mode: mode, key: key, iv: iv}, nil
}

// Mode 는 암호 모드다.
func (c *Cipher) Mode() int32 { return c.mode }

// Encrypt 는 평문 메시지를 암호화한다. 모드 2·5 는 패딩만큼 늘고 나머지는 길이가 같다.
func (c *Cipher) Encrypt(src []byte) ([]byte, error) {
	switch c.mode {
	case ModeSEED256:
		blk, err := newSeed256Cipher(c.key)
		if err != nil {
			return nil, err
		}
		return seedCbcNoPadEncrypt256(blk.(*seed256Cipher), c.iv, src)
	case ModeARIA256:
		blk, err := aria.NewCipher(c.key)
		if err != nil {
			return nil, err
		}
		return cbcPKCS7Encrypt(blk, c.iv[:16], src), nil
	case ModeAES256:
		blk, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, err
		}
		return cbcTailXOR(blk, c.iv[:16], src, true), nil
	case ModeSEED128:
		return seed128StreamCrypt(src, c.key, c.iv)
	case ModeHIGHT:
		if out := kisaHightCbcEncrypt(c.key[:16], c.iv[:8], src); out != nil {
			return out, nil
		}
		return nil, errHightInvalid
	}
	return nil, fmt.Errorf("암호 모드 %d", c.mode)
}

// Decrypt 는 암호문 메시지를 푼다.
func (c *Cipher) Decrypt(src []byte) ([]byte, error) {
	switch c.mode {
	case ModeSEED256:
		blk, err := newSeed256Cipher(c.key)
		if err != nil {
			return nil, err
		}
		return seedCbcNoPadDecrypt256(blk.(*seed256Cipher), c.iv, src)
	case ModeARIA256:
		blk, err := aria.NewCipher(c.key)
		if err != nil {
			return nil, err
		}
		return cbcPKCS7Decrypt(blk, c.iv[:16], src)
	case ModeAES256:
		blk, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, err
		}
		return cbcTailXOR(blk, c.iv[:16], src, false), nil
	case ModeSEED128:
		return seed128StreamCrypt(src, c.key, c.iv)
	case ModeHIGHT:
		return kisaHightCbcDecrypt(c.key[:16], c.iv[:8], src)
	}
	return nil, fmt.Errorf("암호 모드 %d", c.mode)
}

// derive 는 키·IV 32바이트씩을 만든다(C 원본 sub_pkdf2.cpp·서버 crypt2/core.go). 순서는 SvcID, SvcNo, SvcPort, MyIP, SvcIP.
// PBKDF2 결과를 4바이트마다 뒤집는다 ― C 원본이 uint32 낱말 배열을 리틀엔디언 메모리로 옮기기 때문이다.
func derive(p Params, seed128 bool) (key, iv []byte, err error) {
	site := p.SiteAbb
	if len(site) > 4 {
		site = site[:4]
	}
	base := fmt.Sprintf("SQI%s%d,%d,%d,%s,%s;", site, p.SvcID, p.SvcNo, p.SvcPort, p.MyIP, p.SvcIP)
	vec := "VECTOR"
	if seed128 {
		vec = "IVECTOR"
	}
	if key, err = pbkdf2.Key(sha1.New, base+"KEY", []byte("sqi"), 8, 32); err != nil {
		return nil, nil, err
	}
	if iv, err = pbkdf2.Key(sha1.New, base+vec, []byte("sqi"), 8, 32); err != nil {
		return nil, nil, err
	}
	return reverseWords(key), reverseWords(iv), nil
}

func reverseWords(b []byte) []byte {
	for i := 0; i+3 < len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	return b
}

// cbcPKCS7Encrypt 는 CBC + PKCS#7(모드 2). 길이가 블록의 배수면 한 블록을 더한다.
func cbcPKCS7Encrypt(blk cipher.Block, iv, src []byte) []byte {
	bs := blk.BlockSize()
	pad := bs - len(src)%bs
	out := append(append([]byte{}, src...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	cipher.NewCBCEncrypter(blk, iv).CryptBlocks(out, out)
	return out
}

func cbcPKCS7Decrypt(blk cipher.Block, iv, src []byte) ([]byte, error) {
	bs := blk.BlockSize()
	if len(src) == 0 || len(src)%bs != 0 {
		return nil, errCipher
	}
	out := make([]byte, len(src))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, src)
	pad := int(out[len(out)-1])
	if pad < 1 || pad > bs || !bytes.Equal(out[len(out)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		return nil, errCipher
	}
	return out[:len(out)-pad], nil
}

// cbcTailXOR 는 패딩 없는 CBC(모드 3). 끝 조각(블록 미만)은 직전 암호문 블록(없으면 IV)과 XOR 만 한다(CTS 아님).
func cbcTailXOR(blk cipher.Block, iv, src []byte, enc bool) []byte {
	bs := blk.BlockSize()
	full := len(src) / bs * bs
	out := make([]byte, len(src))
	if full > 0 {
		if enc {
			cipher.NewCBCEncrypter(blk, iv).CryptBlocks(out[:full], src[:full])
		} else {
			cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out[:full], src[:full])
		}
	}
	prev := iv
	if full > 0 {
		if enc {
			prev = out[full-bs : full]
		} else {
			prev = src[full-bs : full]
		}
	}
	for i := full; i < len(src); i++ {
		out[i] = src[i] ^ prev[i-full]
	}
	return out
}
