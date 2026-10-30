package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// 받은 폴더 안의 파일은 저장 자리의 권한을 따른다. 새 폴더는 저장 폴더의, 병합은 그 폴더의 상속 권한이다.
func TestReceiveFolderInheritsDestACL(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("하위/a.txt")
	w.Write([]byte("x"))
	zw.Close()
	grant := func(dir string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("icacls", dir, "/grant", "*S-1-1-0:(OI)(CI)(RX)").CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	root := t.TempDir()
	save := filepath.Join(root, "save")
	grant(save)
	merged := filepath.Join(root, "other", "폴더")
	grant(merged)
	for dest, overwrite := range map[string]bool{filepath.Join(save, "폴더"): false, merged: true} {
		c := startFake(t, &fakeServer{data: buf.Bytes(), checksum: sum(buf.Bytes())})
		res, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: int64(buf.Len()), Folder: true,
			Overwrite: overwrite, TempDir: filepath.Join(root, "temp")}, &Progress{})
		if err != nil {
			t.Fatal(err)
		}
		sd, err := windows.GetNamedSecurityInfo(filepath.Join(res.Path, "하위", "a.txt"), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		if s := sd.String(); !strings.Contains(s, "0x1200a9;;;WD)") { // WD 는 Everyone, 이 값은 읽기·실행이다
			t.Fatal(dest, " 의 권한을 따르지 않는다: ", s)
		}
	}
}

// 받을 자리가 물려줄 권한이 없으면 상속을 다시 계산하지 않는다 ― 빈 목록으로 확정돼 소유자도 못 연다.
func TestReceiveFolderNoInheritableACL(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("하위/a.txt")
	w.Write([]byte("x"))
	zw.Close()
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	me := "*" + tu.User.Sid.String()
	for _, flags := range []string{"", "(CI)", "(OI)", "(OI)(CI)(NP)"} {
		root := t.TempDir()
		box := filepath.Join(root, "box")
		if err := os.Mkdir(box, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("icacls", box, "/inheritance:r", "/grant:r", me+":"+flags+"(F)").CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
		fresh := filepath.Join(box, "폴더")
		for _, overwrite := range []bool{false, true} { // 새 폴더로 받고, 그 폴더에 다시 병합한다
			c := startFake(t, &fakeServer{data: buf.Bytes(), checksum: sum(buf.Bytes())})
			if _, err := Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: fresh, Size: int64(buf.Len()), Folder: true,
				Overwrite: overwrite, TempDir: filepath.Join(root, "temp")}, &Progress{}); err != nil {
				t.Fatal(flags, overwrite, err)
			}
			if b, err := os.ReadFile(filepath.Join(fresh, "하위", "a.txt")); err != nil || string(b) != "x" {
				t.Fatal(flags, overwrite, err)
			}
		}
	}
}
