package link

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRegisterShell(t *testing.T) {
	shellRoot = `Software\snbapp-test-shell\`
	defer func() {
		shellRoot = `Software\Classes\`
	}()
	defer func() { // 시험 키를 안쪽부터 지운다
		for _, k := range []string{`*\shell`, `*`, `Directory\Background\shell`, `Directory\Background`, `Directory\shell`, `Directory`, ``} {
			registry.DeleteKey(registry.CURRENT_USER, shellRoot+k)
		}
	}()
	exe := `C:\Program Files\snB\snbapp.exe`
	if err := RegisterShell(exe); err != nil {
		t.Fatal(err)
	}
	read := func(key, name string) string {
		k, err := registry.OpenKey(registry.CURRENT_USER, shellRoot+key, registry.QUERY_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		defer k.Close()
		v, _, err := k.GetStringValue(name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for key, want := range map[string]string{
		`*\shell\SNBUpload\command`:                      `"C:\Program Files\snB\snbapp.exe" "--mode=upload" "%1"`,
		`Directory\shell\SNBDownload\command`:            `"C:\Program Files\snB\snbapp.exe" "--mode=download" "%1"`,
		`Directory\Background\shell\SNBDownload\command`: `"C:\Program Files\snB\snbapp.exe" "--mode=download" "%V"`,
	} {
		if got := read(key, ""); got != want {
			t.Errorf("%s: %s", key, got)
		}
	}
	if got := read(`*\shell\SNBUpload`, "Icon"); got != `"C:\Program Files\snB\snbapp.exe",0` {
		t.Errorf("아이콘: %s", got)
	}
	if err := UnregisterShell(); err != nil {
		t.Fatal(err)
	}
	if err := UnregisterShell(); err != nil { // 없어도 오류가 아니다
		t.Fatal(err)
	}
}
