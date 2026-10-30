package svc

import (
	"testing"

	"snbapp/internal/config"
	"snbapp/pb/serverfile"
)

func TestClipCount(t *testing.T) {
	ready := func(name string, state, course int32) *serverfile.FileInfo {
		return &serverfile.FileInfo{NameStr: name, State: state, PrivState: 1, VirusFlag: 1, Course: course}
	}
	allow := map[string]string{"down_same_network": "1"}
	list := []*serverfile.FileInfo{
		ready("__CLIP__.txt", 1, 0),
		ready("__CLIP__.bmp", 3, 0), // 사후승인도 받는다
		ready("__CLIP__.fmh", 0, 0), // 결재 대기
		ready("a.txt", 1, 0),
		ready("__CLIP__.png", 1, config.LocBusiness), // 같은 망에서 올린 것
	}
	if got := clipCount(list, config.LocBusiness, nil); got != 2 {
		t.Fatalf("동일망 받기를 막으면 2건이어야 한다: %d", got)
	}
	if got := clipCount(list, config.LocBusiness, allow); got != 3 {
		t.Fatalf("동일망 받기를 허용하면 3건이어야 한다: %d", got)
	}
}
