package svc

import (
	"errors"
	"log/slog"
	"testing"

	"snbapp/internal/config"
)

// 배포본은 환경변수를 주어도 시험 진입점을 열지 않는다.
func TestInjectNotifyNeedsDevBuild(t *testing.T) {
	t.Setenv("SNB_TEST_HOOKS", "1")
	t.Setenv("SNB_DEV", "")
	if config.IsDev() {
		t.Skip("개발 빌드 태그로 돌렸다")
	}
	l := &LogService{base: base{c: NewCore("test", slog.Default())}}
	if err := l.InjectNotify(0, 0, 0, "", "", ""); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
}
