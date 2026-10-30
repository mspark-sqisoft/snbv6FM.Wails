package crypt

import "errors"

// KISA HIGHT CBC (PKCS 스타일 패딩): crypt/sub_hight_256.c 와 동일 동작.
// 출처: 서버 snBv6Srv/go/crypt2/hight_kisa.go 사본. 바꾼 곳: 복호 패딩 검사(kisaHightCbcClose) ― 서버는 패딩 값 8
// (평문 길이가 8의 배수)을 거절하지만 C 원본처럼 1~8 을 받는다(docs/port/01 7.3).

var errHightInvalid = errors.New("hight: invalid ciphertext or padding")

var hightF0 = [...]byte{
	0, 134, 13, 139, 26, 156, 23, 145, 52, 178, 57, 191, 46, 168, 35, 165,
	104, 238, 101, 227, 114, 244, 127, 249, 92, 218, 81, 215, 70, 192, 75, 205,
	208, 86, 221, 91, 202, 76, 199, 65, 228, 98, 233, 111, 254, 120, 243, 117,
	184, 62, 181, 51, 162, 36, 175, 41, 140, 10, 129, 7, 150, 16, 155, 29,
	161, 39, 172, 42, 187, 61, 182, 48, 149, 19, 152, 30, 143, 9, 130, 4,
	201, 79, 196, 66, 211, 85, 222, 88, 253, 123, 240, 118, 231, 97, 234, 108,
	113, 247, 124, 250, 107, 237, 102, 224, 69, 195, 72, 206, 95, 217, 82, 212,
	25, 159, 20, 146, 3, 133, 14, 136, 45, 171, 32, 166, 55, 177, 58, 188,
	67, 197, 78, 200, 89, 223, 84, 210, 119, 241, 122, 252, 109, 235, 96, 230,
	43, 173, 38, 160, 49, 183, 60, 186, 31, 153, 18, 148, 5, 131, 8, 142,
	147, 21, 158, 24, 137, 15, 132, 2, 167, 33, 170, 44, 189, 59, 176, 54,
	251, 125, 246, 112, 225, 103, 236, 106, 207, 73, 194, 68, 213, 83, 216, 94,
	226, 100, 239, 105, 248, 126, 245, 115, 214, 80, 219, 93, 204, 74, 193, 71,
	138, 12, 135, 1, 144, 22, 157, 27, 190, 56, 179, 53, 164, 34, 169, 47,
	50, 180, 63, 185, 40, 174, 37, 163, 6, 128, 11, 141, 28, 154, 17, 151,
	90, 220, 87, 209, 64, 198, 77, 203, 110, 232, 99, 229, 116, 242, 121, 255,
}
var hightF1 = [...]byte{
	0, 88, 176, 232, 97, 57, 209, 137, 194, 154, 114, 42, 163, 251, 19, 75,
	133, 221, 53, 109, 228, 188, 84, 12, 71, 31, 247, 175, 38, 126, 150, 206,
	11, 83, 187, 227, 106, 50, 218, 130, 201, 145, 121, 33, 168, 240, 24, 64,
	142, 214, 62, 102, 239, 183, 95, 7, 76, 20, 252, 164, 45, 117, 157, 197,
	22, 78, 166, 254, 119, 47, 199, 159, 212, 140, 100, 60, 181, 237, 5, 93,
	147, 203, 35, 123, 242, 170, 66, 26, 81, 9, 225, 185, 48, 104, 128, 216,
	29, 69, 173, 245, 124, 36, 204, 148, 223, 135, 111, 55, 190, 230, 14, 86,
	152, 192, 40, 112, 249, 161, 73, 17, 90, 2, 234, 178, 59, 99, 139, 211,
	44, 116, 156, 196, 77, 21, 253, 165, 238, 182, 94, 6, 143, 215, 63, 103,
	169, 241, 25, 65, 200, 144, 120, 32, 107, 51, 219, 131, 10, 82, 186, 226,
	39, 127, 151, 207, 70, 30, 246, 174, 229, 189, 85, 13, 132, 220, 52, 108,
	162, 250, 18, 74, 195, 155, 115, 43, 96, 56, 208, 136, 1, 89, 177, 233,
	58, 98, 138, 210, 91, 3, 235, 179, 248, 160, 72, 16, 153, 193, 41, 113,
	191, 231, 15, 87, 222, 134, 110, 54, 125, 37, 205, 149, 28, 68, 172, 244,
	49, 105, 129, 217, 80, 8, 224, 184, 243, 171, 67, 27, 146, 202, 34, 122,
	180, 236, 4, 92, 213, 141, 101, 61, 118, 46, 198, 158, 23, 79, 167, 255,
}
var hightDelta = [...]byte{
	90, 109, 54, 27, 13, 6, 3, 65, 96, 48, 24, 76, 102, 51, 89, 44,
	86, 43, 21, 74, 101, 114, 57, 28, 78, 103, 115, 121, 60, 94, 111, 55,
	91, 45, 22, 11, 5, 66, 33, 80, 40, 84, 42, 85, 106, 117, 122, 125,
	62, 95, 47, 23, 75, 37, 82, 41, 20, 10, 69, 98, 49, 88, 108, 118,
	59, 29, 14, 71, 99, 113, 120, 124, 126, 127, 63, 31, 15, 7, 67, 97,
	112, 56, 92, 110, 119, 123, 61, 30, 79, 39, 83, 105, 52, 26, 77, 38,
	19, 73, 36, 18, 9, 4, 2, 1, 64, 32, 16, 8, 68, 34, 17, 72,
	100, 50, 25, 12, 70, 35, 81, 104, 116, 58, 93, 46, 87, 107, 53, 90,
}

const (
	hightBlockSize = 8
	hightEncrypt   = 1
	hightDecrypt   = 0
)

type kisaHightKey struct {
	userKey [16]byte
	keyData [128]byte
}

var hightEncPerm = [32][8]int{
	{7, 6, 5, 4, 3, 2, 1, 0},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{0, 7, 6, 5, 4, 3, 2, 1},
}

// DRound(t7..t0) 인자 순서 — sub_hight_256.c decrypt (Encrypt Round 역순과 다른 순서).
var hightDecPerm = [32][8]int{
	{7, 6, 5, 4, 3, 2, 1, 0},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{6, 5, 4, 3, 2, 1, 0, 7},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{0, 7, 6, 5, 4, 3, 2, 1},
	{1, 0, 7, 6, 5, 4, 3, 2},
	{2, 1, 0, 7, 6, 5, 4, 3},
	{3, 2, 1, 0, 7, 6, 5, 4},
	{4, 3, 2, 1, 0, 7, 6, 5},
	{5, 4, 3, 2, 1, 0, 7, 6},
	{6, 5, 4, 3, 2, 1, 0, 7},
}

func kisaHightInit(userkey []byte, ks *kisaHightKey) {
	copy(ks.userKey[:], userkey[:16])
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			ks.keyData[16*i+j] = ks.userKey[(j-i)&7] + hightDelta[16*i+j]
		}
		for j := 0; j < 8; j++ {
			ks.keyData[16*i+j+8] = ks.userKey[((j-i)&7)+8] + hightDelta[16*i+j+8]
		}
	}
}

func hightRoundEnc(t *[8]byte, perm [8]int, key []byte) {
	x7 := &t[perm[0]]
	x6 := &t[perm[1]]
	x5 := &t[perm[2]]
	x4 := &t[perm[3]]
	x3 := &t[perm[4]]
	x2 := &t[perm[5]]
	x1 := &t[perm[6]]
	x0 := &t[perm[7]]
	*x1 += hightF1[*x0] ^ key[0]
	*x3 ^= hightF0[*x2] + key[1]
	*x5 += hightF1[*x4] ^ key[2]
	*x7 ^= hightF0[*x6] + key[3]
}

func hightRoundDec(t *[8]byte, perm [8]int, key []byte) {
	x7 := &t[perm[0]]
	x6 := &t[perm[1]]
	x5 := &t[perm[2]]
	x4 := &t[perm[3]]
	x3 := &t[perm[4]]
	x2 := &t[perm[5]]
	x1 := &t[perm[6]]
	x0 := &t[perm[7]]
	*x1 = *x1 - (hightF1[*x0] ^ key[0])
	*x3 ^= hightF0[*x2] + key[1]
	*x5 = *x5 - (hightF1[*x4] ^ key[2])
	*x7 ^= hightF0[*x6] + key[3]
}

func kisaHightEncryptBlock(in []byte, out []byte, ks *kisaHightKey) {
	var t [8]byte
	t[1] = in[1]
	t[3] = in[3]
	t[5] = in[5]
	t[7] = in[7]
	t[0] = in[0] + ks.userKey[12]
	t[2] = in[2] ^ ks.userKey[13]
	t[4] = in[4] + ks.userKey[14]
	t[6] = in[6] ^ ks.userKey[15]
	k := 0
	for r := 0; r < 32; r++ {
		hightRoundEnc(&t, hightEncPerm[r], ks.keyData[k:k+4])
		k += 4
	}
	out[0] = t[1] + ks.userKey[0]
	out[2] = t[3] ^ ks.userKey[1]
	out[4] = t[5] + ks.userKey[2]
	out[6] = t[7] ^ ks.userKey[3]
	out[1] = t[2]
	out[3] = t[4]
	out[5] = t[6]
	out[7] = t[0]
}

func kisaHightDecryptBlock(in []byte, out []byte, ks *kisaHightKey) {
	var t [8]byte
	t[1] = in[1]
	t[3] = in[3]
	t[5] = in[5]
	t[7] = in[7]
	t[0] = in[0] - ks.userKey[0]
	t[2] = in[2] ^ ks.userKey[1]
	t[4] = in[4] - ks.userKey[2]
	t[6] = in[6] ^ ks.userKey[3]
	k := 124
	for r := 0; r < 32; r++ {
		hightRoundDec(&t, hightDecPerm[r], ks.keyData[k:k+4])
		k -= 4
	}
	// DecFin_Transformation(t7, t1, t3, t5, key2[12..15])
	out[0] = t[7] - ks.userKey[12]
	out[2] = t[1] ^ ks.userKey[13]
	out[4] = t[3] - ks.userKey[14]
	out[6] = t[5] ^ ks.userKey[15]
	out[1] = t[0]
	out[3] = t[2]
	out[5] = t[4]
	out[7] = t[6]
}

func internalHightProcessBlocks(hightKey *kisaHightKey, encrypt int, ivec []byte, in []byte, inl int, out []byte) {
	inlength := inl
	iv := ivec
	var tmp [hightBlockSize]byte

	if encrypt != 0 {
		for inlength >= hightBlockSize {
			for i := 0; i < hightBlockSize; i++ {
				out[i] = in[i] ^ iv[i]
			}
			kisaHightEncryptBlock(out[:hightBlockSize], out[:hightBlockSize], hightKey)
			iv = out[:hightBlockSize]
			inlength -= hightBlockSize
			in = in[hightBlockSize:]
			out = out[hightBlockSize:]
		}
		if inlength > 0 {
			for i := 0; i < inlength; i++ {
				out[i] = in[i] ^ iv[i]
			}
			for i := inlength; i < hightBlockSize; i++ {
				out[i] = iv[i]
			}
			kisaHightEncryptBlock(out[:hightBlockSize], out[:hightBlockSize], hightKey)
			iv = out[:hightBlockSize]
		}
		copy(ivec, iv[:hightBlockSize])
		return
	}

	// decrypt, in != out (별도 버퍼)
	for inlength >= hightBlockSize {
		kisaHightDecryptBlock(in[:hightBlockSize], out[:hightBlockSize], hightKey)
		for i := 0; i < hightBlockSize; i++ {
			out[i] ^= iv[i]
		}
		iv = in[:hightBlockSize]
		inlength -= hightBlockSize
		in = in[hightBlockSize:]
		out = out[hightBlockSize:]
	}
	if inlength > 0 {
		kisaHightDecryptBlock(in[:hightBlockSize], tmp[:], hightKey)
		for i := 0; i < inlength; i++ {
			out[i] = tmp[i] ^ iv[i]
		}
		iv = in[:hightBlockSize]
	}
	copy(ivec, iv[:hightBlockSize])
}

type kisaHightCbcInfo struct {
	encrypt       int
	ivec          [hightBlockSize]byte
	hightKey      kisaHightKey
	cbcBuffer     [hightBlockSize]byte
	bufferLength  int
	cbcLastBlock  [hightBlockSize]byte
	lastBlockFlag int
}

func kisaHightCbcInit(info *kisaHightCbcInfo, enc int, userKey, iv []byte) bool {
	if info == nil || len(userKey) < 16 || len(iv) < hightBlockSize {
		return false
	}
	kisaHightInit(userKey[:16], &info.hightKey)
	copy(info.ivec[:], iv[:hightBlockSize])
	info.encrypt = enc
	info.lastBlockFlag = 0
	info.bufferLength = 0
	return true
}

func internalHightCbcProcessEnc(info *kisaHightCbcInfo, in []byte, inLen int, out []byte) (outLen int) {
	templen := inLen & 0x07

	if info.bufferLength == 0 && templen == 0 {
		internalHightProcessBlocks(&info.hightKey, info.encrypt, info.ivec[:], in, inLen, out)
		return inLen
	}

	if info.bufferLength != 0 {
		if info.bufferLength+inLen < hightBlockSize {
			copy(info.cbcBuffer[info.bufferLength:], in[:inLen])
			info.bufferLength += inLen
			return 0
		}
		length := hightBlockSize - info.bufferLength
		copy(info.cbcBuffer[info.bufferLength:], in[:length])
		internalHightProcessBlocks(&info.hightKey, info.encrypt, info.ivec[:], info.cbcBuffer[:], hightBlockSize, out)
		inLen -= length
		in = in[length:]
		out = out[hightBlockSize:]
		outLen = hightBlockSize
		templen = inLen & 0x07
	} else {
		outLen = 0
	}

	inLen -= templen
	if inLen > 0 {
		internalHightProcessBlocks(&info.hightKey, info.encrypt, info.ivec[:], in, inLen, out)
		outLen += inLen
	}

	if templen > 0 {
		copy(info.cbcBuffer[:], in[inLen:inLen+templen])
	}
	info.bufferLength = templen
	return outLen
}

func internalHightCbcProcessDec(info *kisaHightCbcInfo, in []byte, inLen int, out []byte) (outLen int) {
	outOff := 0
	updatedLen := 0
	if info.lastBlockFlag != 0 {
		copy(out, info.cbcLastBlock[:])
		outOff = hightBlockSize
		updatedLen = 1
	}
	outLen = internalHightCbcProcessEnc(info, in, inLen, out[outOff:])
	if info.bufferLength == 0 {
		outLen -= hightBlockSize
		copy(info.cbcLastBlock[:], out[outOff+outLen:outOff+outLen+hightBlockSize])
		info.lastBlockFlag = 1
	} else {
		info.lastBlockFlag = 0
	}
	if updatedLen != 0 {
		outLen += hightBlockSize
	}
	return outLen
}

func kisaHightCbcProcess(info *kisaHightCbcInfo, in []byte, inLen int, out []byte) int {
	if inLen <= 0 {
		return 0
	}
	if info.encrypt != 0 {
		return internalHightCbcProcessEnc(info, in, inLen, out)
	}
	return internalHightCbcProcessDec(info, in, inLen, out)
}

func kisaHightCbcClose(info *kisaHightCbcInfo, out []byte) (outLen int, ok bool) {
	if info.encrypt != 0 {
		padlen := hightBlockSize - info.bufferLength
		for i := info.bufferLength; i < hightBlockSize; i++ {
			info.cbcBuffer[i] = byte(padlen)
		}
		internalHightProcessBlocks(&info.hightKey, info.encrypt, info.ivec[:], info.cbcBuffer[:], hightBlockSize, out)
		return hightBlockSize, true
	}
	if info.lastBlockFlag == 0 { // 암호문이 8의 배수가 아니거나 비었다
		return 0, false
	}
	padVal := int(info.cbcLastBlock[hightBlockSize-1])
	if padVal < 1 || padVal > hightBlockSize {
		return 0, false
	}
	for i := hightBlockSize - padVal; i < hightBlockSize; i++ {
		if info.cbcLastBlock[i] != byte(padVal) {
			return 0, false
		}
	}
	padlen := hightBlockSize - padVal // 마지막 블록에 남은 평문 길이(0~7)
	copy(out, info.cbcLastBlock[:padlen])
	return padlen, true
}

func kisaHightCbcEncrypt(userKey, iv, in []byte) []byte {
	var cbc kisaHightCbcInfo
	if !kisaHightCbcInit(&cbc, hightEncrypt, userKey, iv) {
		return nil
	}
	out := make([]byte, len(in)+hightBlockSize)
	interlen := kisaHightCbcProcess(&cbc, in, len(in), out)
	padlen, ok := kisaHightCbcClose(&cbc, out[interlen:])
	if !ok {
		return nil
	}
	return out[:interlen+padlen]
}

func kisaHightCbcDecrypt(userKey, iv, in []byte) ([]byte, error) {
	var cbc kisaHightCbcInfo
	if !kisaHightCbcInit(&cbc, hightDecrypt, userKey, iv) {
		return nil, errHightInvalid
	}
	out := make([]byte, len(in)+hightBlockSize)
	interlen := kisaHightCbcProcess(&cbc, in, len(in), out)
	padlen, ok := kisaHightCbcClose(&cbc, out[interlen:])
	if !ok {
		return nil, errHightInvalid
	}
	return out[:interlen+padlen], nil
}
