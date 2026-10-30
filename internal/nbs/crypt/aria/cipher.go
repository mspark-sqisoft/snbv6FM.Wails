// Package aria 는 ARIA 블록 암호(RFC 5794)다. 출처: 서버 snBv6Srv/go/crypt2/internalaria 사본(그대로, 패키지 이름만 바꿈).
package aria

import (
	"crypto/cipher"
	"fmt"
)

const BlockSize = 16

type ariaCipher struct {
	k    int
	enc  []uint32
	dec  []uint32
	encB [][16]byte
	decB [][16]byte
	n    int
}

type KeySizeError int

func (k KeySizeError) Error() string {
	return fmt.Sprintf("aria: invalid key size %d", int(k))
}

func NewCipher(key []byte) (cipher.Block, error) {
	k := len(key)
	switch k {
	default:
		return nil, KeySizeError(k)
	case 128 / 8, 192 / 8, 256 / 8:
	}
	n := k + 36
	r := k/4 + 8
	c := ariaCipher{
		k:    k,
		enc:  make([]uint32, n),
		dec:  make([]uint32, n),
		encB: make([][16]byte, r+1),
		decB: make([][16]byte, r+1),
		n:    r,
	}
	c.expandKey(key)
	return &c, nil
}

func (c *ariaCipher) BlockSize() int { return BlockSize }

func (c *ariaCipher) Encrypt(dst, src []byte) {
	if len(src) < BlockSize {
		panic("aria: input not full block")
	}
	if len(dst) < BlockSize {
		panic("aria: output not full block")
	}
	if inexactOverlap(dst[:BlockSize], src[:BlockSize]) {
		panic("aria: invalid buffer overlap")
	}
	c.cryptBlock(c.encB, dst, src)
}

func (c *ariaCipher) Decrypt(dst, src []byte) {
	if len(src) < BlockSize {
		panic("aria: input not full block")
	}
	if len(dst) < BlockSize {
		panic("aria: output not full block")
	}
	if inexactOverlap(dst[:BlockSize], src[:BlockSize]) {
		panic("aria: invalid buffer overlap")
	}
	c.cryptBlock(c.decB, dst, src)
}

func (c *ariaCipher) rounds() int { return c.k/4 + 8 }
