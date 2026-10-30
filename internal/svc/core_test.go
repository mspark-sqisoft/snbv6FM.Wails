package svc

import (
	"log/slog"
	"testing"

	"snbapp/internal/nbs"
)

// 늦게 온 끊김(앞 세대)은 새 세션을 끊지 않는다. 실제 내림(연결 닫기)은 앱 시험에서 본다.
func TestLogoutIfIgnoresStaleGen(t *testing.T) {
	c := NewCore("test", slog.Default())
	if c.LogoutIf(c.Gen(), ReasonLogout) {
		t.Fatal("세션이 없는데 로그아웃했다")
	}
	c.sess = &nbs.Session{UserID: "u"}
	if c.LogoutIf(c.Gen()+1, ReasonLogout) || c.sess == nil {
		t.Fatal("다른 세대가 세션을 내렸다")
	}
}

// 권한 변경은 다음 Session() 부터 보이고, 이미 잡은 세션의 값은 그대로이며, 지난 세대의 응답은 버린다.
func TestSetProfile(t *testing.T) {
	c := NewCore("test", slog.Default())
	old := &nbs.Session{UserID: "u", Profile: map[string]string{"a": "1"}}
	c.sess = old
	c.setProfile(c.Gen(), map[string]string{"a": "2"})
	s, err := c.Session()
	if err != nil || s.Profile["a"] != "2" || s.UserID != "u" {
		t.Fatal(s, err)
	}
	if old.Profile["a"] != "1" {
		t.Fatal("이미 잡은 세션의 값이 바뀌었다")
	}
	c.setProfile(c.Gen()+1, map[string]string{"a": "3"})
	if s, _ := c.Session(); s.Profile["a"] != "2" {
		t.Fatal("지난 세대의 응답이 들어갔다")
	}
}
