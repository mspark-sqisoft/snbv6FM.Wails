//go:build !windows

package transfer

func setHidden(string) error { return nil } // 이름이 "." 으로 시작하면 숨김이다
