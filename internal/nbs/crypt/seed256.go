package crypt

// SEED-256 블록과 32바이트 체인(모드 1). 출처: 서버 snBv6Srv/go/crypt2/seed256.go 사본.
// 바꾼 곳: 환경변수(SEED256_SKIP_ENDIAN_CHANGE·SEED256_DUMP_ROUND_KEYS) 처리를 뺐다 ― 라운드 키를 표준 출력에 찍지 않는다.

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"sync"
)

var (
	seedSS0 []uint32
	seedSS1 []uint32
	seedSS2 []uint32
	seedSS3 []uint32

	seedTablesOnce sync.Once
	seedTablesErr  error
)

const seedApplyEndianChange = true

func ensureSeedTables() error {
	seedTablesOnce.Do(func() {
		seedSS0, seedSS1, seedSS2, seedSS3, seedTablesErr = loadPackedSeedTables()
	})
	return seedTablesErr
}

func rotl32(x uint32, n uint32) uint32 { return (x << n) | (x >> (32 - n)) }
func seedEndianChange(dw uint32) uint32 {
	return (rotl32(dw, 8) & 0x00ff00ff) | (rotl32(dw, 24) & 0xff00ff00)
}
func getB0(a uint32) byte { return byte(a) }
func getB1(a uint32) byte { return byte(a >> 8) }
func getB2(a uint32) byte { return byte(a >> 16) }
func getB3(a uint32) byte { return byte(a >> 24) }

type seed256Cipher struct{ roundKeys [48]uint32 }

func newSeed256Cipher(key []byte) (cipher.Block, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("seed-256: invalid key size %d (need 32)", len(key))
	}
	if err := ensureSeedTables(); err != nil {
		return nil, err
	}
	var c seed256Cipher
	seedRoundKey256(&c.roundKeys, key)
	return &c, nil
}

func (c *seed256Cipher) BlockSize() int { return 16 }
func (c *seed256Cipher) Encrypt(dst, src []byte) {
	if len(src) < 16 || len(dst) < 16 {
		panic("seed-256: input/output not full block")
	}
	var block [16]byte
	copy(block[:], src[:16])
	seedEncrypt256Block(&block, &c.roundKeys)
	copy(dst[:16], block[:])
}
func (c *seed256Cipher) Decrypt(dst, src []byte) {
	if len(src) < 16 || len(dst) < 16 {
		panic("seed-256: input/output not full block")
	}
	var block [16]byte
	copy(block[:], src[:16])
	seedDecrypt256Block(&block, &c.roundKeys)
	copy(dst[:16], block[:])
}

func seedCbcNoPadEncrypt256(c *seed256Cipher, iv32 []byte, in []byte) ([]byte, error) {
	if len(iv32) < 32 {
		return nil, fmt.Errorf("seed256: iv32 too small: %d", len(iv32))
	}
	if len(in) == 0 {
		return []byte{}, nil
	}
	const bs = 32
	out := make([]byte, len(in))
	prev := make([]byte, bs)
	copy(prev, iv32[:bs])
	full := len(in) / bs
	rest := len(in) % bs
	for k := 0; k < full; k++ {
		pos := k * bs
		for j := 0; j < bs; j++ {
			out[pos+j] = in[pos+j] ^ prev[j]
		}
		var block [16]byte
		copy(block[:], out[pos:pos+16])
		seedEncrypt256Block(&block, &c.roundKeys)
		copy(out[pos:pos+16], block[:])
		copy(prev, out[pos:pos+bs])
	}
	if rest != 0 {
		pos := full * bs
		for j := 0; j < rest; j++ {
			out[pos+j] = in[pos+j] ^ prev[j]
		}
	}
	return out, nil
}

func seedCbcNoPadDecrypt256(c *seed256Cipher, iv32 []byte, in []byte) ([]byte, error) {
	if len(iv32) < 32 {
		return nil, fmt.Errorf("seed256: iv32 too small: %d", len(iv32))
	}
	if len(in) == 0 {
		return []byte{}, nil
	}
	const bs = 32
	out := make([]byte, len(in))
	prev := make([]byte, bs)
	copy(prev, iv32[:bs])
	full := len(in) / bs
	rest := len(in) % bs
	for k := 0; k < full; k++ {
		pos := k * bs
		tmp := make([]byte, bs)
		copy(tmp, in[pos:pos+bs])
		copy(out[pos:pos+bs], in[pos:pos+bs])
		var block [16]byte
		copy(block[:], out[pos:pos+16])
		seedDecrypt256Block(&block, &c.roundKeys)
		copy(out[pos:pos+16], block[:])
		for j := 0; j < bs; j++ {
			out[pos+j] ^= prev[j]
		}
		copy(prev, tmp)
	}
	if rest != 0 {
		pos := full * bs
		for j := 0; j < rest; j++ {
			out[pos+j] = in[pos+j] ^ prev[j]
		}
	}
	return out, nil
}

func seedRound(l0, l1, r0, r1 *uint32, k []uint32, kIdx int) {
	t0 := (*r0) ^ k[kIdx]
	t1 := (*r1) ^ k[kIdx+1]
	t1 ^= t0
	t1 = seedSS0[getB0(t1)] ^ seedSS1[getB1(t1)] ^ seedSS2[getB2(t1)] ^ seedSS3[getB3(t1)]
	t0 += t1
	t0 = seedSS0[getB0(t0)] ^ seedSS1[getB1(t0)] ^ seedSS2[getB2(t0)] ^ seedSS3[getB3(t0)]
	t1 += t0
	t1 = seedSS0[getB0(t1)] ^ seedSS1[getB1(t1)] ^ seedSS2[getB2(t1)] ^ seedSS3[getB3(t1)]
	t0 += t1
	*l0 ^= t0
	*l1 ^= t1
}

func seedEncrypt256Block(pbData *[16]byte, roundKeys *[48]uint32) {
	data0 := binary.LittleEndian.Uint32(pbData[0:4])
	data1 := binary.LittleEndian.Uint32(pbData[4:8])
	data2 := binary.LittleEndian.Uint32(pbData[8:12])
	data3 := binary.LittleEndian.Uint32(pbData[12:16])
	L0, L1, R0, R1 := data0, data1, data2, data3
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	keys := roundKeys[:]
	for i := 0; i < 24; i++ {
		kIdx := i * 2
		if i%2 == 0 {
			seedRound(&L0, &L1, &R0, &R1, keys, kIdx)
		} else {
			seedRound(&R0, &R1, &L0, &L1, keys, kIdx)
		}
	}
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	binary.LittleEndian.PutUint32(pbData[0:4], R0)
	binary.LittleEndian.PutUint32(pbData[4:8], R1)
	binary.LittleEndian.PutUint32(pbData[8:12], L0)
	binary.LittleEndian.PutUint32(pbData[12:16], L1)
}

func seedDecrypt256Block(pbData *[16]byte, roundKeys *[48]uint32) {
	data0 := binary.LittleEndian.Uint32(pbData[0:4])
	data1 := binary.LittleEndian.Uint32(pbData[4:8])
	data2 := binary.LittleEndian.Uint32(pbData[8:12])
	data3 := binary.LittleEndian.Uint32(pbData[12:16])
	L0, L1, R0, R1 := data0, data1, data2, data3
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	keys := roundKeys[:]
	for i := 0; i < 24; i++ {
		kIdx := 46 - i*2
		if i%2 == 0 {
			seedRound(&L0, &L1, &R0, &R1, keys, kIdx)
		} else {
			seedRound(&R0, &R1, &L0, &L1, keys, kIdx)
		}
	}
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	binary.LittleEndian.PutUint32(pbData[0:4], R0)
	binary.LittleEndian.PutUint32(pbData[4:8], R1)
	binary.LittleEndian.PutUint32(pbData[8:12], L0)
	binary.LittleEndian.PutUint32(pbData[12:16], L1)
}

func seedRoundKey256(out *[48]uint32, pbUserKey []byte) {
	if len(pbUserKey) < 32 {
		panic("seed-256: key buffer too small")
	}
	user := [8]uint32{
		binary.LittleEndian.Uint32(pbUserKey[0:4]), binary.LittleEndian.Uint32(pbUserKey[4:8]),
		binary.LittleEndian.Uint32(pbUserKey[8:12]), binary.LittleEndian.Uint32(pbUserKey[12:16]),
		binary.LittleEndian.Uint32(pbUserKey[16:20]), binary.LittleEndian.Uint32(pbUserKey[20:24]),
		binary.LittleEndian.Uint32(pbUserKey[24:28]), binary.LittleEndian.Uint32(pbUserKey[28:32]),
	}
	A, B, C, D := user[0], user[1], user[2], user[3]
	E, F, G, H := user[4], user[5], user[6], user[7]
	if seedApplyEndianChange {
		A, B, C, D = seedEndianChange(A), seedEndianChange(B), seedEndianChange(C), seedEndianChange(D)
		E, F, G, H = seedEndianChange(E), seedEndianChange(F), seedEndianChange(G), seedEndianChange(H)
	}
	const (
		KC0  uint32 = 0x9e3779b9
		KC1  uint32 = 0x3c6ef373
		KC2  uint32 = 0x78dde6e6
		KC3  uint32 = 0xf1bbcdcc
		KC4  uint32 = 0xe3779b99
		KC5  uint32 = 0xc6ef3733
		KC6  uint32 = 0x8dde6e67
		KC7  uint32 = 0x1bbcdccf
		KC8  uint32 = 0x3779b99e
		KC9  uint32 = 0x6ef3733c
		KC10 uint32 = 0xdde6e678
		KC11 uint32 = 0xbbcdccf1
		KC12 uint32 = 0x779b99e3
		KC13 uint32 = 0xef3733c6
		KC14 uint32 = 0xde6e678d
		KC15 uint32 = 0xbcdccf1b
		KC16 uint32 = 0x79b99e37
		KC17 uint32 = 0xf3733c6e
		KC18 uint32 = 0xe6e678dd
		KC19 uint32 = 0xcdccf1bb
		KC20 uint32 = 0x9b99e377
		KC21 uint32 = 0x3733c6ef
		KC22 uint32 = 0x6e678dde
		KC23 uint32 = 0xdccf1bbc
	)
	t0 := ((A + C) ^ E) - F
	t0 ^= KC0
	t1 := ((B - D) ^ G) + H
	t1 ^= KC0
	out[0] = seedSS0[getB0(t0)] ^ seedSS1[getB1(t0)] ^ seedSS2[getB2(t0)] ^ seedSS3[getB3(t0)]
	out[1] = seedSS0[getB0(t1)] ^ seedSS1[getB1(t1)] ^ seedSS2[getB2(t1)] ^ seedSS3[getB3(t1)]
	update0 := func(kIdx int, kc uint32, rot uint32) {
		T0 := D
		D = (D >> rot) ^ (C << (32 - rot))
		C = (C >> rot) ^ (B << (32 - rot))
		B = (B >> rot) ^ (A << (32 - rot))
		A = (A >> rot) ^ (T0 << (32 - rot))
		T0 = (((A + C) ^ E) - F) ^ kc
		T1 := (((B - D) ^ G) + H) ^ kc
		out[kIdx+0] = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
		out[kIdx+1] = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
	}
	update1 := func(kIdx int, kc uint32, rot uint32) {
		T0 := E
		E = (E << rot) ^ (F >> (32 - rot))
		F = (F << rot) ^ (G >> (32 - rot))
		G = (G << rot) ^ (H >> (32 - rot))
		H = (H << rot) ^ (T0 >> (32 - rot))
		T0 = (((A + C) ^ E) - F) ^ kc
		T1 := (((B - D) ^ G) + H) ^ kc
		out[kIdx+0] = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
		out[kIdx+1] = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
	}
	update0(2, KC1, 9)
	update1(4, KC2, 9)
	update0(6, KC3, 11)
	update1(8, KC4, 11)
	update0(10, KC5, 12)
	update1(12, KC6, 12)
	update0(14, KC7, 9)
	update1(16, KC8, 9)
	update0(18, KC9, 11)
	update1(20, KC10, 11)
	update0(22, KC11, 12)
	update1(24, KC12, 12)
	update0(26, KC13, 9)
	update1(28, KC14, 9)
	update0(30, KC15, 11)
	update1(32, KC16, 11)
	update0(34, KC17, 12)
	update1(36, KC18, 12)
	update0(38, KC19, 9)
	update1(40, KC20, 9)
	update0(42, KC21, 11)
	update1(44, KC22, 11)
	update0(46, KC23, 12)
}
