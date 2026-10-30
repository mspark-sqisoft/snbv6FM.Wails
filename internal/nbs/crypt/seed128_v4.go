package crypt

// 출처: 서버 snBv6Srv/go/crypt2/seed128_v4.go 사본(그대로). 서버처럼 메시지마다 같은 키·IV 로 새로 시작한다(상태 없음).

import (
	"encoding/binary"
	"fmt"
)

// SEED-128 (KISA v4): C sub_seed_128.c 의 SeedRoundKey_v4 / SeedEncrypt_v4 / SEED_ENCRYPT_128 와 동일.

func seedRoundKeyV4(out *[32]uint32, pbUserKey []byte) {
	if len(pbUserKey) < 16 {
		panic("seed128: user key too short")
	}
	A := binary.LittleEndian.Uint32(pbUserKey[0:4])
	B := binary.LittleEndian.Uint32(pbUserKey[4:8])
	C := binary.LittleEndian.Uint32(pbUserKey[8:12])
	D := binary.LittleEndian.Uint32(pbUserKey[12:16])
	if seedApplyEndianChange {
		A, B, C, D = seedEndianChange(A), seedEndianChange(B), seedEndianChange(C), seedEndianChange(D)
	}
	K := out[:]
	var T0, T1 uint32
	roundKeyUpdate0 := func(kIdx int, kc uint32) {
		T0 = A + C - kc
		T1 = B + kc - D
		K[kIdx] = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
		K[kIdx+1] = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
		T0 = A
		A = (A >> 8) ^ (B << 24)
		B = (B >> 8) ^ (T0 << 24)
	}
	roundKeyUpdate1 := func(kIdx int, kc uint32) {
		T0 = A + C - kc
		T1 = B + kc - D
		K[kIdx] = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
		K[kIdx+1] = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
		T0 = C
		C = (C << 8) ^ (D >> 24)
		D = (D << 8) ^ (T0 >> 24)
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
	)
	roundKeyUpdate0(0, KC0)
	roundKeyUpdate1(2, KC1)
	roundKeyUpdate0(4, KC2)
	roundKeyUpdate1(6, KC3)
	roundKeyUpdate0(8, KC4)
	roundKeyUpdate1(10, KC5)
	roundKeyUpdate0(12, KC6)
	roundKeyUpdate1(14, KC7)
	roundKeyUpdate0(16, KC8)
	roundKeyUpdate1(18, KC9)
	roundKeyUpdate0(20, KC10)
	roundKeyUpdate1(22, KC11)
	roundKeyUpdate0(24, KC12)
	roundKeyUpdate1(26, KC13)
	roundKeyUpdate0(28, KC14)
	T0 = A + C - KC15
	T1 = B - D + KC15
	K[30] = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
	K[31] = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
}

func seedRoundV4(l0, l1, r0, r1 *uint32, k []uint32, kIdx int) {
	var T0, T1 uint32
	T0 = *r0 ^ k[kIdx]
	T1 = *r1 ^ k[kIdx+1]
	T1 ^= T0
	T1 = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
	T0 += T1
	T0 = seedSS0[getB0(T0)] ^ seedSS1[getB1(T0)] ^ seedSS2[getB2(T0)] ^ seedSS3[getB3(T0)]
	T1 += T0
	T1 = seedSS0[getB0(T1)] ^ seedSS1[getB1(T1)] ^ seedSS2[getB2(T1)] ^ seedSS3[getB3(T1)]
	T0 += T1
	*l0 ^= T0
	*l1 ^= T1
}

func seedEncryptV4(pbData *[16]byte, pdwRoundKey *[32]uint32) {
	L0 := binary.LittleEndian.Uint32(pbData[0:4])
	L1 := binary.LittleEndian.Uint32(pbData[4:8])
	R0 := binary.LittleEndian.Uint32(pbData[8:12])
	R1 := binary.LittleEndian.Uint32(pbData[12:16])
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	K := pdwRoundKey[:]
	seedRoundV4(&L0, &L1, &R0, &R1, K, 0)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 2)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 4)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 6)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 8)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 10)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 12)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 14)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 16)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 18)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 20)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 22)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 24)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 26)
	seedRoundV4(&L0, &L1, &R0, &R1, K, 28)
	seedRoundV4(&R0, &R1, &L0, &L1, K, 30)
	if seedApplyEndianChange {
		L0, L1, R0, R1 = seedEndianChange(L0), seedEndianChange(L1), seedEndianChange(R0), seedEndianChange(R1)
	}
	binary.LittleEndian.PutUint32(pbData[0:4], R0)
	binary.LittleEndian.PutUint32(pbData[4:8], R1)
	binary.LittleEndian.PutUint32(pbData[8:12], L0)
	binary.LittleEndian.PutUint32(pbData[12:16], L1)
}

// seed128StreamCrypt: SEED_ENCRYPT_128 / SEED_DECRYPT_128 과 동일 (CTR 유사 스트림).
func seed128StreamCrypt(in []byte, key32, iv32 []byte) ([]byte, error) {
	if len(key32) < 16 || len(iv32) < 16 {
		return nil, fmt.Errorf("seed128: key/iv too short")
	}
	if err := ensureSeedTables(); err != nil {
		return nil, err
	}
	var roundKeys [32]uint32
	seedRoundKeyV4(&roundKeys, key32[:16])

	var ti [16]byte
	v0 := binary.BigEndian.Uint32(iv32[0:4])
	v1 := binary.BigEndian.Uint32(iv32[4:8])
	v2 := binary.BigEndian.Uint32(iv32[8:12])
	v3 := binary.BigEndian.Uint32(iv32[12:16])
	binary.LittleEndian.PutUint32(ti[0:4], v0)
	binary.LittleEndian.PutUint32(ti[4:8], v1)
	binary.LittleEndian.PutUint32(ti[8:12], v2)
	binary.LittleEndian.PutUint32(ti[12:16], v3)

	out := make([]byte, len(in))
	var d [16]byte
	n := 0
	for i := 0; i < len(in); i++ {
		if n == 0 {
			seedEncryptV4(&ti, &roundKeys)
			w0 := binary.LittleEndian.Uint32(ti[0:4])
			w1 := binary.LittleEndian.Uint32(ti[4:8])
			w2 := binary.LittleEndian.Uint32(ti[8:12])
			w3 := binary.LittleEndian.Uint32(ti[12:16])
			binary.BigEndian.PutUint32(d[0:4], w0)
			binary.BigEndian.PutUint32(d[4:8], w1)
			binary.BigEndian.PutUint32(d[8:12], w2)
			binary.BigEndian.PutUint32(d[12:16], w3)
		}
		out[i] = in[i] ^ d[n]
		n = (n + 1) & 0x0f
	}
	return out, nil
}
