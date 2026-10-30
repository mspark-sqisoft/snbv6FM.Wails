//go:build !windows

package transfer

import (
	"io/fs"
	"strings"
)

// hidden 은 이름이 "." 으로 시작하는가다(macOS·Linux).
func hidden(name string, _ fs.DirEntry) bool { return strings.HasPrefix(name, ".") }
