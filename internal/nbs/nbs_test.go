package nbs

import (
	"bytes"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"snbapp/internal/nbs/crypt"
	"snbapp/pb/common"
	"snbapp/pb/login"
	"snbapp/pb/serverfile"
)

func TestLoginFailure(t *testing.T) {
	cases := []struct {
		code codes.Code
		msg  string
		want string
		exit bool
	}{
		{codes.Unavailable, "connection refused", MsgConnect, false},
		{codes.DeadlineExceeded, "", MsgConnect, false},
		{codes.Unauthenticated, "LICENSE_ERROR", MsgLicense, false},
		{codes.Unauthenticated, "CONNECTION_SIDE_ERROR", MsgSide, false},
		{codes.Unauthenticated, "USER_NOT_FOUND", MsgAccount, true},
		{codes.Unauthenticated, "OTHER_ERROR", MsgAccount, true},
		{codes.Unauthenticated, "PASSWORD_ERROR", MsgAccount, false},
		{codes.Unauthenticated, "PASSWORD_ERROR_COUNT_OVER", MsgAccount, false},
		{codes.Unauthenticated, "MAC_ERROR", MsgAccount, false},
		// 인터넷망 서버가 업무망 거절을 감싼 것(158 실측 문구)
		{codes.Internal, "failed to route to inner server: rpc error: code = Internal desc = inner server error: rpc error: code = Unauthenticated desc = OTHER_ERROR", MsgAccount, true},
		{codes.Internal, "failed to route to inner server: rpc error: code = Internal desc = inner server error: rpc error: code = Unauthenticated desc = PASSWORD_ERROR", MsgAccount, false},
		{codes.Internal, "failed to route to inner server: rpc error: code = Unavailable desc = connection refused", MsgConnect, false},
	}
	for _, c := range cases {
		got := loginFailure(status.Error(c.code, c.msg))
		if got.Msg != c.want || got.Exit != c.exit {
			t.Errorf("%s %q: got %+v, want %q exit=%v", c.code, c.msg, got, c.want, c.exit)
		}
	}
	var le *LoginError
	if !errors.As(error(channelErr(1)), &le) || le.Msg != "서버 접속에 실패했습니다. - 2" {
		t.Errorf("channelErr: %v", le)
	}
}

func TestIsHandshakeErr(t *testing.T) {
	if !isHandshakeErr(status.Error(codes.Unavailable, `connection error: desc = "transport: authentication handshake failed: tls: failed to verify certificate: x509: certificate signed by unknown authority"`)) {
		t.Error("핸드셰이크 실패를 못 가렸다")
	}
	if isHandshakeErr(status.Error(codes.Unavailable, "connection error: desc = \"transport: Error while dialing: dial tcp 10.0.0.1:29002: connectex: No connection could be made\"")) {
		t.Error("연결 실패를 핸드셰이크로 봤다")
	}
}

func TestCheckLength(t *testing.T) {
	cases := []struct {
		id, pw string
		ok     bool
	}{
		{"abc", "pw", true},
		{strings.Repeat("a", 31), strings.Repeat("b", 31), true},
		{strings.Repeat("a", 32), "pw", false},
		{"abc", strings.Repeat("b", 32), false},
		{strings.Repeat("가", 10), "pw", true},  // 30바이트
		{strings.Repeat("가", 11), "pw", false}, // 33바이트 ― 한글 11자(9.2 시나리오 4)
	}
	for _, c := range cases {
		err := CheckLength(c.id, c.pw)
		if (err == nil) != c.ok || (err != nil && err.Error() != MsgTooLong) {
			t.Errorf("%q/%q: %v", c.id, c.pw, err)
		}
	}
}

func TestDefaultPasswordAndRemainDays(t *testing.T) {
	if isDefaultPassword("", "") || isDefaultPassword("", "x") || isDefaultPassword("a", "b") || !isDefaultPassword("init1234!", "init1234!") {
		t.Error("isDefaultPassword")
	}
	for in, want := range map[string]int{"": 0, "0": 0, "15": 15, " -3 ": -3, "abc": 0} {
		if got := remainDays(in); got != want {
			t.Errorf("remainDays(%q)=%d", in, got)
		}
	}
}

func TestPickIdentity(t *testing.T) {
	up := net.FlagUp | net.FlagRunning
	mac := func(s string) net.HardwareAddr { m, _ := net.ParseMAC(s); return m }
	ip := net.ParseIP
	eth := nic{name: "이더넷", flags: up, mac: mac("aa:bb:cc:dd:ee:01"), ips: []net.IP{ip("fe80::1"), ip("192.168.0.10")}}
	wifi := nic{name: "Wi-Fi", flags: up, mac: mac("aa:bb:cc:dd:ee:02"), ips: []net.IP{ip("10.0.0.5")}}
	vm := nic{name: "VMware Network Adapter VMnet8", flags: up, mac: mac("00:50:56:c0:00:08"), ips: []net.IP{ip("192.168.56.1")}}
	virt := nic{name: "VirtualBox Host-Only", flags: up, mac: mac("0a:00:27:00:00:01"), ips: []net.IP{ip("192.168.99.1")}}
	down := nic{name: "이더넷 2", flags: net.FlagUp, mac: mac("aa:bb:cc:dd:ee:03"), ips: []net.IP{ip("172.16.0.1")}}
	tun := nic{name: "WireGuard", flags: up, ips: []net.IP{ip("10.8.0.2")}} // MAC 없음
	lo := nic{name: "Loopback", flags: up | net.FlagLoopback, mac: mac("00:00:00:00:00:01"), ips: []net.IP{ip("127.0.0.1")}}
	ll := nic{name: "이더넷 3", flags: up, mac: mac("aa:bb:cc:dd:ee:04"), ips: []net.IP{ip("169.254.1.2")}}
	cases := []struct {
		name    string
		route   net.IP
		nics    []nic
		ip, mac string
	}{
		{"1순위: 서버 경로의 어댑터", ip("10.0.0.5"), []nic{eth, wifi}, "10.0.0.5", "AABBCCDDEE02"},
		{"경로 어댑터가 가상이면 2순위", ip("192.168.56.1"), []nic{vm, eth}, "192.168.0.10", "AABBCCDDEE01"},
		{"경로 없음: 첫 실물 어댑터", nil, []nic{lo, tun, vm, virt, down, ll, wifi, eth}, "10.0.0.5", "AABBCCDDEE02"},
		{"루프백 경로는 무시", ip("127.0.0.1"), []nic{lo, eth}, "192.168.0.10", "AABBCCDDEE01"},
		{"쓸 만한 어댑터 없음", ip("10.8.0.2"), []nic{lo, tun, vm, down, ll}, "", ""},
	}
	for _, c := range cases {
		gotIP, gotMAC, _ := pickIdentity(c.route, c.nics)
		if gotIP != c.ip || gotMAC != c.mac {
			t.Errorf("%s: %s %s", c.name, gotIP, gotMAC)
		}
	}
}

// 받기 응답은 조각을 복사하지 않고 풀되 결과는 proto 와 같다. 조각 하나뿐이 아니면 proto 로 돌린다.
func TestCodecAliasChunk(t *testing.T) {
	k := newCryptCodec()
	decode := func(m *serverfile.FileDownloadResponse) {
		t.Helper()
		data, err := k.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		got := &serverfile.FileDownloadResponse{}
		if err := k.Unmarshal(data, got); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(m, got) {
			t.Fatalf("%v != %v", m, got)
		}
	}
	decode(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Chunk{Chunk: []byte("abc")}})
	decode(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Chunk{Chunk: bytes.Repeat([]byte{7}, 1<<20)}})
	decode(&serverfile.FileDownloadResponse{Data: &serverfile.FileDownloadResponse_Answer{Answer: &common.AnswerResponse{Res: 1, Reason: "x"}}})
	decode(&serverfile.FileDownloadResponse{})

	buf := []byte{0x12, 2, 'a', 'b'} // 필드 2(chunk), 길이 2
	r := &serverfile.FileDownloadResponse{}
	if !aliasChunk(buf, r) || &r.GetChunk()[0] != &buf[2] {
		t.Fatal("조각을 복사했다")
	}
	if aliasChunk(append(buf, 0x12, 0), &serverfile.FileDownloadResponse{}) || aliasChunk(buf[:3], &serverfile.FileDownloadResponse{}) {
		t.Fatal("조각 하나가 아닌데 받았다")
	}
}

// 코덱: 모드 0 은 기본 proto 코덱과 같은 바이트, 모드 1~5 는 암호문으로 실어 왕복한다.
func TestCryptCodec(t *testing.T) {
	msg := &login.PreLoginRequest{Userid: "test9_3", Msg: "[START]"}
	k := newCryptCodec()
	plain, err := k.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := proto.Marshal(msg)
	if !bytes.Equal(plain.Materialize(), want) {
		t.Error("모드 0 은 그대로여야 한다")
	}
	for m := int32(1); m <= 5; m++ {
		c, _ := crypt.New(m, crypt.PreLoginParams())
		k.c.Store(c)
		enc, err := k.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(enc.Materialize(), want) {
			t.Errorf("mode%d: 암호화되지 않았다", m)
		}
		var back login.PreLoginRequest
		if err := k.Unmarshal(enc, &back); err != nil || back.Userid != "test9_3" || back.Msg != "[START]" {
			t.Errorf("mode%d 왕복: %v %v", m, &back, err)
		}
	}
	var empty login.PreLoginRequest // 빈 메시지는 암호화하지 않는다
	if b, _ := k.Marshal(&empty); b.Len() != 0 {
		t.Error("빈 메시지")
	}
	if err := k.Unmarshal(mem.BufferSlice{mem.SliceBuffer([]byte{1, 2, 3})}, &empty); err == nil { // 마지막 모드 5(HIGHT): 8의 배수가 아닌 암호문
		t.Error("깨진 암호문을 받았다")
	}
}

func TestParseProfile(t *testing.T) {
	got := ParseProfile("fm_upfile_mode=1,fm_checksum=0,broken,remove_logfile=")
	want := map[string]string{"fm_upfile_mode": "1", "fm_checksum": "0", "remove_logfile": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
