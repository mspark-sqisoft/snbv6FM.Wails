package crypt

import (
	"bytes"
	"encoding/hex"
	"testing"

	"snbapp/internal/nbs/crypt/aria"
)

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// 원본 tests/tst_FMEncryption.cpp:513-567 의 KAT(docs/port/01 7.3). 평문 34바이트, 키 재료 셋 × 모드 1~5.
func TestKnownAnswer(t *testing.T) {
	plain := []byte("087 known answer 0123456789abcdef!")
	keys := []Params{
		PreLoginParams(),
		{SvcID: 99, SvcNo: 7, MyIP: "10.0.0.1", SvcIP: "10.0.0.2", SvcPort: 1234, SiteAbb: "OTHER"},
		{SvcID: 31, SvcNo: 2, MyIP: "192.168.0.10", SvcIP: "192.168.0.20", SvcPort: 5000, SiteAbb: "SQI"},
	}
	want := [3][5]string{
		{
			"b7cc90b33093bd74173a382e368177dc184288309291f81042230abda21c1948d1ed",
			"0e0fe5528baf3dd52ea223e1e61f5ea1559e0126b285946dcf6380a9b7cc7a7d370924e010dc99733b462c0a87b115f1",
			"64ad3cee9eee0d600905cff798e47fcffcd35d0a70641b837a50859cc5100c409af2",
			"a5985e51430337580af087f2a687fd12207b569af532dd6e4a4345cc63c64db73912",
			"9a023cf33cbac1d5925e6352eed73a143b2197514975fae3a8417228123c351c72aff2628ec07c6c",
		},
		{
			"ff789fb547a701008a9b25c728ddb0176183120523301874f10c291c650224789959",
			"c7c3d93bcd796d54eb71b123f86a8e4317524a2c517eba79e2b909c5a12d570a724a945aea1f59ed07cbbe9ecc9bb48b",
			"5cbbe3fd6f0f5777d58ff391c84e1e70ed1f24053fedb7da640582cce4409d088b3e",
			"d3f40d881e56f81eff4a9b47adbee568ebd6fbbd7149b7fcc560a5dfd3f58c6e7eb3",
			"6a6260b51c522708c2ff4b803d249799e2db621a7beed4ec10dbd600bf2ecd04b40c0dc31bb9e3e9",
		},
		{
			"35d15b2fbcf14e0d247c1b4dfbc251dd834fd681068ea82172708425c965b82753f0",
			"2a43a119280ee414827189fc367c2ad619db1f655219bcce5796af746038536ae5ffafac2505ff7948687e32ae066154",
			"94bf72ca1eccba0cad490cb326f6362e6c2aeb6deb395a258be68c73285831c80a0b",
			"944654337b0576e1d909c1bb287efc05990c7b42c3ac53926abf87da0e03c13201dc",
			"81fd81a350a1c236245bb79917764361e1a6873772b8904aa89c4c202d6d6662dfaad7c0c43d8284",
		},
	}
	for ki, p := range keys {
		for m := int32(1); m <= 5; m++ {
			c, err := New(m, p)
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Encrypt(plain)
			if err != nil {
				t.Fatalf("key%d mode%d: %v", ki, m, err)
			}
			if h := hex.EncodeToString(got); h != want[ki][m-1] {
				t.Errorf("key%d mode%d\n got %s\nwant %s", ki, m, h, want[ki][m-1])
			}
			back, err := c.Decrypt(got)
			if err != nil || !bytes.Equal(back, plain) {
				t.Errorf("key%d mode%d 복호: %q %v", ki, m, back, err)
			}
			// 상태 없음: 두 번째 메시지도 같은 암호문(원본 SEED-128 의 IV 덮어쓰기를 옮기지 않았다)
			again, _ := c.Encrypt(plain)
			if !bytes.Equal(again, got) {
				t.Errorf("key%d mode%d: 두 번째 암호문이 다르다", ki, m)
			}
		}
	}
}

// 모드별 길이와 경계 길이 왕복(0·1·7·8·15·16·31·32·33·1000). HIGHT 8의 배수 길이도 풀려야 한다(서버 결함을 옮기지 않음).
func TestRoundTripLengths(t *testing.T) {
	wantLen := map[int32]func(n int) int{
		1: func(n int) int { return n },
		2: func(n int) int { return n/16*16 + 16 },
		3: func(n int) int { return n },
		4: func(n int) int { return n },
		5: func(n int) int { return n/8*8 + 8 },
	}
	for m := int32(1); m <= 5; m++ {
		c, _ := New(m, PreLoginParams())
		for _, n := range []int{1, 7, 8, 15, 16, 31, 32, 33, 34, 1000} {
			src := bytes.Repeat([]byte{byte(n)}, n)
			enc, err := c.Encrypt(src)
			if err != nil {
				t.Fatalf("mode%d len%d: %v", m, n, err)
			}
			if len(enc) != wantLen[m](n) {
				t.Errorf("mode%d len%d: 암호문 길이 %d", m, n, len(enc))
			}
			dec, err := c.Decrypt(enc)
			if err != nil || !bytes.Equal(dec, src) {
				t.Errorf("mode%d len%d: 왕복 실패 %v", m, n, err)
			}
		}
	}
}

func TestBadCiphertext(t *testing.T) {
	for _, m := range []int32{ModeARIA256, ModeHIGHT} {
		c, _ := New(m, PreLoginParams())
		if _, err := c.Decrypt([]byte{1, 2, 3}); err == nil {
			t.Errorf("mode%d: 블록 배수가 아닌 암호문을 받았다", m)
		}
	}
}

func TestModes(t *testing.T) {
	if c, err := New(0, Params{}); c != nil || err != nil {
		t.Error("모드 0 은 (nil, nil)")
	}
	if _, err := New(6, Params{}); err != ErrNPAria || err.Error() != "서버 암호 모드 6 은 지원하지 않습니다." {
		t.Errorf("모드 6: %v", err)
	}
	if _, err := New(9, Params{}); err == nil {
		t.Error("모드 9 를 받았다")
	}
	if !Supported(5) || Supported(6) || Supported(-1) {
		t.Error("Supported")
	}
}

// 공개 시험 벡터(블록 함수 단독, docs/port/01 7.3).
func TestPublicVectors(t *testing.T) {
	t.Run("ARIA-256 RFC5794 A.3", func(t *testing.T) {
		blk, err := aria.NewCipher(unhex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))
		if err != nil {
			t.Fatal(err)
		}
		out := make([]byte, 16)
		blk.Encrypt(out, unhex(t, "00112233445566778899aabbccddeeff"))
		if h := hex.EncodeToString(out); h != "f92bd7c79fb72e2f2b8f80c1972d24fc" {
			t.Errorf("got %s", h)
		}
	})
	_ = ensureSeedTables()
	for _, v := range []struct{ name, key, in, out string }{
		{"SEED-128 RFC4269 B.1", "00000000000000000000000000000000", "000102030405060708090a0b0c0d0e0f", "5ebac6e0054e166819aff1cc6d346cdb"},
		{"SEED-128 RFC4269 B.2", "000102030405060708090a0b0c0d0e0f", "00000000000000000000000000000000", "c11f22f20140505084483597e4370f43"},
	} {
		t.Run(v.name, func(t *testing.T) {
			var rk [32]uint32
			seedRoundKeyV4(&rk, unhex(t, v.key))
			var blk [16]byte
			copy(blk[:], unhex(t, v.in))
			seedEncryptV4(&blk, &rk)
			if h := hex.EncodeToString(blk[:]); h != v.out {
				t.Errorf("got %s", h)
			}
		})
	}
	// KISA HIGHT 구현은 키·평문·암호문 바이트 순서를 뒤집어 다룬다 ― 논문 벡터를 뒤집은 값으로 시험한다.
	for _, v := range []struct{ key, in, out string }{
		{"ffeeddccbbaa99887766554433221100", "0000000000000000", "f2034fd9ae18f400"},
		{"00112233445566778899aabbccddeeff", "7766554433221100", "d8e643e5729fce23"},
	} {
		var ks kisaHightKey
		kisaHightInit(unhex(t, v.key), &ks)
		out := make([]byte, 8)
		kisaHightEncryptBlock(unhex(t, v.in), out, &ks)
		if h := hex.EncodeToString(out); h != v.out {
			t.Errorf("HIGHT key %s: got %s want %s", v.key, h, v.out)
		}
	}
}
