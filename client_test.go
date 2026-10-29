package main

import (
	"os"
	"path/filepath"
	"testing"

	"snbapp/pb/serverfile"
)

func TestReadServers(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fmenv.ini")
	ini := "\xEF\xBB\xBF# 주석\n[SERVER01]\r\nIP=10.0.0.1\r\nNAME=업무망\r\nLOC=1\r\n\n[CONFIG]\nIP=9.9.9.9\n[SERVER02]\nIP = 10.0.0.2\nPORT=1234\nLOC=0\n"
	if err := os.WriteFile(p, []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readServers(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []Server{{"업무망", "10.0.0.1", "29002", "1"}, {"", "10.0.0.2", "1234", "0"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDownloadBlock(t *testing.T) {
	ok := func() *serverfile.FileInfo { return &serverfile.FileInfo{NameStr: "a.txt", State: 1, VirusFlag: 1, Course: 1} }
	cases := []struct {
		name    string
		edit    func(*serverfile.FileInfo)
		loc     string
		profile map[string]string
		blocked bool
	}{
		{"승인·검사 끝·반대 망", func(*serverfile.FileInfo) {}, "0", nil, false},
		{"동일망 금지", func(*serverfile.FileInfo) {}, "1", map[string]string{"down_same_network": "0"}, true},
		{"동일망 허용", func(*serverfile.FileInfo) {}, "1", map[string]string{"down_same_network": "1"}, false},
		{"결재 전", func(f *serverfile.FileInfo) { f.State = 4 }, "0", nil, true},
		{"검사 중", func(f *serverfile.FileInfo) { f.VirusFlag = 0 }, "0", nil, true},
		{"검사 4 는 받는다", func(f *serverfile.FileInfo) { f.VirusFlag = 4 }, "0", nil, false},
		{"클립보드", func(f *serverfile.FileInfo) { f.NameStr = "x__CLIP__.txt" }, "0", nil, true},
		{"폴더", func(f *serverfile.FileInfo) { f.DirFlag = 1 }, "0", nil, true},
	}
	for _, c := range cases {
		f := ok()
		c.edit(f)
		if got := downloadBlock(f, c.loc, c.profile) != ""; got != c.blocked {
			t.Errorf("%s: blocked=%v, want %v (%q)", c.name, got, c.blocked, downloadBlock(f, c.loc, c.profile))
		}
	}
}
