package svc

import (
	"testing"

	"snbapp/internal/config"
	"snbapp/pb/login"
)

// 원본 PasswordChangeDlg.qml:178-206 의 검사 순서·문구.
func TestValidatePasswordChange(t *testing.T) {
	cases := []struct{ cur, next, confirm, want string }{
		{"", "abcd1234!@", "abcd1234!@", MsgPwCurEmpty},
		{"old", "", "", MsgPwNewEmpty},
		{"same1234!a", "same1234!a", "same1234!a", MsgPwSame},
		{"old", "abcd1234!@", "", MsgPwConfirmNone},
		{"old", "abcd1234!@", "abcd1234!#", MsgPwMismatch},
		{"old", "abcd12345", "abcd12345", MsgPwRule},                 // 특수문자 없음
		{"old", "abcdefgh!@", "abcdefgh!@", MsgPwRule},               // 숫자 없음
		{"old", "12345678!@", "12345678!@", MsgPwRule},               // 영문 없음
		{"old", "ab1!", "ab1!", MsgPwRule},                           // 짧음
		{"old", "abcd1234!@abcd123", "abcd1234!@abcd123", MsgPwRule}, // 17자
		{"old", "abcd 1234!@", "abcd 1234!@", MsgPwRule},             // 허용 밖 글자(공백)
		{"old", "abcd1234^@", "abcd1234^@", MsgPwRule},               // 허용 밖 특수문자
		{"old", "가나다1234!@ab", "가나다1234!@ab", MsgPwRule},
		{"old", "abcd1234!@", "abcd1234!@", ""},
		{"old", "Abcdefg1&?12345X", "Abcdefg1&?12345X", ""}, // 16자
	}
	for _, c := range cases {
		if got := ValidatePasswordChange(c.cur, c.next, c.confirm); got != c.want {
			t.Errorf("%q→%q/%q: got %q want %q", c.cur, c.next, c.confirm, got, c.want)
		}
	}
}

func TestAuthorityOf(t *testing.T) {
	env := &config.Env{Servers: []config.Server{
		{Loc: config.LocBusiness, Network: "업무망->인터넷망"},
		{Loc: config.LocInternet, Network: "인터넷망->업무망"},
	}}
	got := authorityOf(&login.LoginResponse{
		FmFileUp: 1, FmFileDown: 0, FmClipDown: 1, FmClipUp: 0,
		FmInflupAprv: 1, FmOtflupAprv: 3, FmInclupAprv: 0, FmOtclupAprv: 6,
		FmFileSize: 1024, FmFileCount: 10, FmClipSize: 0,
	}, env)
	want := Authority{
		InName: "인터넷망->업무망", OutName: "업무망->인터넷망",
		FileOut: true, FileIn: false, ClipOut: true, ClipIn: false,
		FileOutAprv: 1, FileInAprv: 3, ClipOutAprv: 0, ClipInAprv: 6,
		MaxFileSize: 1024 * 1024, MaxFileCount: 10, MaxClipSize: 0,
	}
	if *got != want {
		t.Errorf("got %+v\nwant %+v", *got, want)
	}
}
