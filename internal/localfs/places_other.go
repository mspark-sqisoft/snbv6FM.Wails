//go:build !windows

package localfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DesktopDir 는 ~/Desktop 이다(XDG 설정의 다른 이름은 따르지 않는다 ― 없으면 홈으로 내려간다).
func DesktopDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

// DocumentsDir 는 ~/Documents 다.
func DocumentsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

// Drive 는 볼륨 하나다.
type Drive struct {
	Path  string
	Label string
	Kind  string
}

// Drives 는 / 와 볼륨(macOS /Volumes/*, Linux /media·/mnt 아래)이다.
func Drives() []Drive {
	out := []Drive{{Path: "/", Kind: "fixed"}}
	var globs []string
	if runtime.GOOS == "darwin" {
		globs = []string{"/Volumes/*"}
	} else {
		user := os.Getenv("USER")
		globs = []string{"/media/" + user + "/*", "/run/media/" + user + "/*", "/mnt/*"}
	}
	for _, g := range globs {
		ms, _ := filepath.Glob(g)
		for _, m := range ms {
			if st, err := os.Stat(m); err == nil && st.IsDir() {
				out = append(out, Drive{Path: m, Label: filepath.Base(m), Kind: "removable"})
			}
		}
	}
	return out
}

func rootDir() string { return "/" }

func isHidden(name string, _ fs.FileInfo) bool { return strings.HasPrefix(name, ".") }

func typeNamer() func(string) string { return func(string) string { return "" } }
