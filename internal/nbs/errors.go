package nbs

// 로그인 화면 문구. 원본 FileMoverGrpc.cpp:582-601(PreLogin)·867-914(Login) 의 한국어 원문 그대로다(docs/port/01 3.2·3.3).

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MsgTooLong        = "아이디 또는 비밀번호가 너무 깁니다."
	MsgNoCert         = "서버 인증서를 검증할 수 없습니다."
	MsgUntrusted      = "서버를 신뢰할 수 없어 접속하지 않았습니다. 관리자에게 문의해 주세요."
	MsgPreLoginFail   = "서버에 연결할 수 없습니다. 접속 서버 설정을 확인해 주세요."
	MsgConnect        = "서버에 연결할 수 없습니다."
	MsgLicense        = "라이선스가 만료되었거나 사용할 수 없습니다."
	MsgSide           = "접속한 서버의 망 구분이 설정과 다릅니다."
	MsgAccount        = "로그인에 실패하였습니다."
	MsgNoServer       = "서버정보가 없어 로그인에 실패했습니다."
	msgChannelFailFmt = "서버 접속에 실패했습니다. - %d" // %d = 1 부터 센 서버 순번
)

// LoginError 는 로그인 실패다. Msg 는 화면에 그대로 낸다. Exit 이면 안내 창을 닫은 뒤 앱을 끝낸다(미등록 ID, AUTH-021).
type LoginError struct {
	Msg  string
	Exit bool
}

func (e *LoginError) Error() string { return e.Msg }

func loginErr(msg string) *LoginError { return &LoginError{Msg: msg} }

func channelErr(index int) *LoginError { return loginErr(fmt.Sprintf(msgChannelFailFmt, index+1)) }

// loginFailure 는 Login RPC 오류를 화면 문구로 바꾼다. 계정 실패는 사유를 가리지 않고 한 문구다(없는 ID 를 알아낼 수 없게).
// 판정 순서는 원본과 같다: 문구를 고른 뒤 미등록 여부를 따로 본다 ― 서버가 DB 오류에도 OTHER_ERROR 를 보내 등록된 ID 도 종료된다.
func loginFailure(err error) *LoginError {
	st := status.Convert(err)
	code, m := st.Code(), st.Message()
	// 인터넷망 서버는 업무망의 거절을 Internal 로 감싸 보낸다(실측: "failed to route to inner server: rpc error: code = Internal
	// desc = inner server error: rpc error: code = Unauthenticated desc = OTHER_ERROR"). 원본은 이것을 연결 실패로 냈다 ― 안쪽 거절을 꺼내 쓴다.
	if code == codes.Internal {
		const inner = "code = Unauthenticated desc = "
		if i := strings.LastIndex(m, inner); i >= 0 {
			code, m = codes.Unauthenticated, m[i+len(inner):]
		}
	}
	switch {
	case code != codes.Unauthenticated:
		return loginErr(MsgConnect)
	case strings.Contains(m, "LICENSE_ERROR"):
		return loginErr(MsgLicense)
	case strings.Contains(m, "CONNECTION_SIDE_ERROR"):
		return loginErr(MsgSide)
	case strings.Contains(m, "OTHER_ERROR") || strings.Contains(m, "USER_NOT_FOUND"):
		return &LoginError{Msg: MsgAccount, Exit: true}
	}
	return loginErr(MsgAccount) // PASSWORD_ERROR·PASSWORD_ERROR_COUNT_OVER·OTP_ERROR·IP_ERROR·MAC_ERROR 등
}

// isHandshakeErr 는 TLS 핸드셰이크(인증서 검증) 실패인가다. grpc-go 는 Unavailable 하나에
// "transport: authentication handshake failed: tls: ..." 를 실어 보낸다(docs/port/01 7.6 의 1안).
func isHandshakeErr(err error) bool {
	m := status.Convert(err).Message()
	for _, k := range []string{"authentication handshake failed", "x509:", "tls:", "certificate"} {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}
