package nbs

import (
	"errors"
	"strings"

	"google.golang.org/grpc/status"

	"snbapp/pb/commonservice"
)

// 비밀번호 변경 문구(원본 PasswordChangeDlg.qml·ViewControllerNotifySlot.cpp).
const (
	MsgPasswordChanged = "비밀번호가 정상적으로 변경되었습니다."
	// MsgPasswordChangeFail 은 gRPC 오류일 때다. 원본은 로그만 남겨 화면이 반응하지 않았다(AUTH-049, docs/port/01 8장).
	MsgPasswordChangeFail = "비밀번호를 변경하지 못했습니다."
)

// ChangePassword 는 비밀번호 변경[3002]이다. 계정 칸에는 세션의 실제 ID 를 싣는다(원본은 [ID 저장] 을 끄면 빈 값이었다, AUTH-048).
// 공백을 뺀 값으로 보낸다(원본과 같다). 서버 거절(res<=0)은 서버 사유 그대로, gRPC 오류는 MsgPasswordChangeFail + 사유다.
func (s *Session) ChangePassword(cur, next string) error {
	ctx, cancel := s.Ctx(RPCTimeout)
	defer cancel()
	res, err := s.Common().ChangePassword(ctx, &commonservice.ChangePasswordRequest{
		UserId: s.UserID, CurPw: strings.TrimSpace(cur), NewPw: strings.TrimSpace(next),
	})
	if err != nil {
		if m := status.Convert(err).Message(); m != "" {
			return errors.New(MsgPasswordChangeFail + "\n" + m)
		}
		return errors.New(MsgPasswordChangeFail)
	}
	if res.Res <= 0 {
		if res.Reason == "" {
			return errors.New(MsgPasswordChangeFail)
		}
		return errors.New(res.Reason)
	}
	return nil
}
