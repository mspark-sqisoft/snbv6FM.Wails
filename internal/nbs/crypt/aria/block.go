package aria

import "encoding/binary"

func (c *ariaCipher) cryptBlock(xk [][16]byte, dst, src []byte) {
	n := c.n
	var p [16]byte
	copy(p[:], src[:BlockSize])
	for i := 1; i <= n-1; i++ {
		if i&1 == 1 {
			p = roundOdd(p, xk[i-1])
		} else {
			p = roundEven(p, xk[i-1])
		}
	}
	p = xor(substitute2(xor(p, xk[n-1])), xk[n])
	copy(dst[:BlockSize], p[:])
}

func (c *ariaCipher) expandKey(key []byte) {
	n := c.rounds()
	var kl, kr [16]byte
	for i := 0; i < c.k; i++ {
		if i < 16 {
			kl[i] = key[i]
		} else {
			kr[i-16] = key[i]
		}
	}
	var ck1, ck2, ck3 [16]byte
	switch c.k {
	case 128 / 8:
		ck1, ck2, ck3 = c1, c2, c3
	case 192 / 8:
		ck1, ck2, ck3 = c2, c3, c1
	case 256 / 8:
		ck1, ck2, ck3 = c3, c1, c2
	}
	var w0, w1, w2, w3 [16]byte
	w0 = kl
	w1 = xor(roundOdd(w0, ck1), kr)
	w2 = xor(roundEven(w1, ck2), w0)
	w3 = xor(roundOdd(w2, ck3), w1)

	copyBytes(c.enc, xor(w0, rrot(w1, 19)))
	copyBytes(c.enc[4:], xor(w1, rrot(w2, 19)))
	copyBytes(c.enc[8:], xor(w2, rrot(w3, 19)))
	copyBytes(c.enc[12:], xor(w3, rrot(w0, 19)))
	copyBytes(c.enc[16:], xor(w0, rrot(w1, 31)))
	copyBytes(c.enc[20:], xor(w1, rrot(w2, 31)))
	copyBytes(c.enc[24:], xor(w2, rrot(w3, 31)))
	copyBytes(c.enc[28:], xor(w3, rrot(w0, 31)))
	copyBytes(c.enc[32:], xor(w0, lrot(w1, 61)))
	copyBytes(c.enc[36:], xor(w1, lrot(w2, 61)))
	copyBytes(c.enc[40:], xor(w2, lrot(w3, 61)))
	copyBytes(c.enc[44:], xor(w3, lrot(w0, 61)))
	copyBytes(c.enc[48:], xor(w0, lrot(w1, 31)))
	if n > 12 {
		copyBytes(c.enc[52:], xor(w1, lrot(w2, 31)))
		copyBytes(c.enc[56:], xor(w2, lrot(w3, 31)))
	}
	if n > 14 {
		copyBytes(c.enc[60:], xor(w3, lrot(w0, 31)))
		copyBytes(c.enc[64:], xor(w0, lrot(w1, 19)))
	}

	copy(c.dec, c.enc[n*4:(n+1)*4])
	for i := 1; i <= n-1; i++ {
		t := toBytes(c.enc[(n-i)*4 : (n-i+1)*4])
		t = diffuse(t)
		copyBytes(c.dec[i*4:], t)
	}
	copy(c.dec[n*4:], c.enc[:4])
	for i := 0; i <= n; i++ {
		c.encB[i] = toBytes(c.enc[i*4 : (i+1)*4])
		c.decB[i] = toBytes(c.dec[i*4 : (i+1)*4])
	}
}

func copyBytes(xk []uint32, x [16]byte) {
	xk[0] = binary.BigEndian.Uint32(x[:])
	xk[1] = binary.BigEndian.Uint32(x[4:])
	xk[2] = binary.BigEndian.Uint32(x[8:])
	xk[3] = binary.BigEndian.Uint32(x[12:])
}

func toBytes(u []uint32) (r [16]byte) {
	r[0], r[1], r[2], r[3] = byte(u[0]>>24), byte(u[0]>>16), byte(u[0]>>8), byte(u[0])
	r[4], r[5], r[6], r[7] = byte(u[1]>>24), byte(u[1]>>16), byte(u[1]>>8), byte(u[1])
	r[8], r[9], r[10], r[11] = byte(u[2]>>24), byte(u[2]>>16), byte(u[2]>>8), byte(u[2])
	r[12], r[13], r[14], r[15] = byte(u[3]>>24), byte(u[3]>>16), byte(u[3]>>8), byte(u[3])
	return
}

func roundOdd(d, rk [16]byte) [16]byte  { return diffuse(substitute1(xor(d, rk))) }
func roundEven(d, rk [16]byte) [16]byte { return diffuse(substitute2(xor(d, rk))) }

func substitute1(x [16]byte) (y [16]byte) {
	for i := 0; i < 16; i += 4 {
		y[i+0], y[i+1], y[i+2], y[i+3] = sb1[x[i+0]], sb2[x[i+1]], sb3[x[i+2]], sb4[x[i+3]]
	}
	return
}
func substitute2(x [16]byte) (y [16]byte) {
	for i := 0; i < 16; i += 4 {
		y[i+0], y[i+1], y[i+2], y[i+3] = sb3[x[i+0]], sb4[x[i+1]], sb1[x[i+2]], sb2[x[i+3]]
	}
	return
}

func diffuse(x [16]byte) (y [16]byte) {
	y[0] = x[3] ^ x[4] ^ x[6] ^ x[8] ^ x[9] ^ x[13] ^ x[14]
	y[1] = x[2] ^ x[5] ^ x[7] ^ x[8] ^ x[9] ^ x[12] ^ x[15]
	y[2] = x[1] ^ x[4] ^ x[6] ^ x[10] ^ x[11] ^ x[12] ^ x[15]
	y[3] = x[0] ^ x[5] ^ x[7] ^ x[10] ^ x[11] ^ x[13] ^ x[14]
	y[4] = x[0] ^ x[2] ^ x[5] ^ x[8] ^ x[11] ^ x[14] ^ x[15]
	y[5] = x[1] ^ x[3] ^ x[4] ^ x[9] ^ x[10] ^ x[14] ^ x[15]
	y[6] = x[0] ^ x[2] ^ x[7] ^ x[9] ^ x[10] ^ x[12] ^ x[13]
	y[7] = x[1] ^ x[3] ^ x[6] ^ x[8] ^ x[11] ^ x[12] ^ x[13]
	y[8] = x[0] ^ x[1] ^ x[4] ^ x[7] ^ x[10] ^ x[13] ^ x[15]
	y[9] = x[0] ^ x[1] ^ x[5] ^ x[6] ^ x[11] ^ x[12] ^ x[14]
	y[10] = x[2] ^ x[3] ^ x[5] ^ x[6] ^ x[8] ^ x[13] ^ x[15]
	y[11] = x[2] ^ x[3] ^ x[4] ^ x[7] ^ x[9] ^ x[12] ^ x[14]
	y[12] = x[1] ^ x[2] ^ x[6] ^ x[7] ^ x[9] ^ x[11] ^ x[12]
	y[13] = x[0] ^ x[3] ^ x[6] ^ x[7] ^ x[8] ^ x[10] ^ x[13]
	y[14] = x[0] ^ x[3] ^ x[4] ^ x[5] ^ x[9] ^ x[11] ^ x[14]
	y[15] = x[1] ^ x[2] ^ x[4] ^ x[5] ^ x[8] ^ x[10] ^ x[15]
	return
}

func xor(a, b [16]byte) (r [16]byte) {
	for i := 0; i < 16; i++ {
		r[i] = a[i] ^ b[i]
	}
	return
}
func lrot(x [16]byte, n uint) (y [16]byte) {
	q, r := n/8, n%8
	s := 8 - r
	for i := 0; i < 16; i++ {
		y[i] = x[(uint(i)+q)%16]<<r | x[(uint(i)+q+1)%16]>>s
	}
	return
}
func rrot(x [16]byte, n uint) (y [16]byte) {
	q, r := n/8%16, n%8
	s := 8 - r
	for i := 0; i < 16; i++ {
		y[i] = x[(uint(i)+16-q)%16]>>r | x[(uint(i)+15-q)%16]<<s
	}
	return
}
