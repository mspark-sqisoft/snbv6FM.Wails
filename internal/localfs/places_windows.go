package localfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DesktopDir 는 바탕 화면이다(OneDrive 로 옮긴 바탕 화면도 따라간다 ― %USERPROFILE%\Desktop 고정은 틀린다).
func DesktopDir() string {
	p, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Desktop")
	}
	return p
}

// DocumentsDir 는 내 문서다.
func DocumentsDir() string {
	p, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Documents")
	}
	return p
}

// Drive 는 내 컴퓨터 아래 드라이브 하나다.
type Drive struct {
	Path  string // C:\
	Label string // 볼륨 이름(고정 드라이브만 읽는다 ― CD·네트워크는 느릴 수 있다)
	Kind  string // fixed·removable·network·cdrom·ram
}

// Drives 는 지금 꽂힌 드라이브다. 부를 때마다 다시 읽는다(원본은 시작 때 한 번 ― 실행 중 꽂은 USB 가 안 보였다).
func Drives() []Drive {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []Drive
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := windows.UTF16PtrFromString(root)
		kind := ""
		switch windows.GetDriveType(p) {
		case windows.DRIVE_FIXED:
			kind = "fixed"
		case windows.DRIVE_REMOVABLE:
			kind = "removable"
		case windows.DRIVE_REMOTE:
			kind = "network"
		case windows.DRIVE_CDROM:
			kind = "cdrom"
		case windows.DRIVE_RAMDISK:
			kind = "ram"
		default:
			continue // 없음·알 수 없음
		}
		d := Drive{Path: root, Kind: kind}
		if kind == "fixed" {
			buf := make([]uint16, windows.MAX_PATH+1)
			if windows.GetVolumeInformation(p, &buf[0], uint32(len(buf)), nil, nil, nil, nil, 0) == nil {
				d.Label = windows.UTF16ToString(buf)
			}
		}
		out = append(out, d)
	}
	return out
}

func rootDir() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return d + `\`
	}
	return `C:\`
}

// isHidden 은 숨김 속성이다(Windows 는 이름이 아니라 파일 속성).
func isHidden(_ string, info fs.FileInfo) bool {
	if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return a.FileAttributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
	}
	return false
}

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procSHGetFileInfo = shell32.NewProc("SHGetFileInfoW")
	typeCache         sync.Map // 확장자 → 유형 설명
)

type shFileInfo struct {
	hIcon         windows.Handle
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [windows.MAX_PATH]uint16
	szTypeName    [80]uint16
}

const (
	shgfiTypeName          = 0x400
	shgfiUseFileAttributes = 0x10
)

// typeNamer 는 확장자의 탐색기 유형 설명(예: 「텍스트 문서」)을 돌려준다. 확장자마다 한 번만 묻는다(파일을 열지 않는다).
func typeNamer() func(ext string) string {
	return func(ext string) string {
		if ext == "" || ext == "LNK" {
			return ""
		}
		if v, ok := typeCache.Load(ext); ok {
			return v.(string)
		}
		name, _ := windows.UTF16PtrFromString("x." + ext)
		var sfi shFileInfo
		r, _, _ := procSHGetFileInfo.Call(uintptr(unsafe.Pointer(name)), windows.FILE_ATTRIBUTE_NORMAL,
			uintptr(unsafe.Pointer(&sfi)), unsafe.Sizeof(sfi), shgfiTypeName|shgfiUseFileAttributes)
		s := ""
		if r != 0 {
			s = windows.UTF16ToString(sfi.szTypeName[:])
		}
		typeCache.Store(ext, s)
		return s
	}
}
