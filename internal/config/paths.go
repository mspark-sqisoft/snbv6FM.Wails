// Package config 는 경로, fmenv.ini·fmlocal.ini·fmusr.ini 읽기, 사용자 설정 저장소(wails-settings.json)를 맡는다.
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// AppName 은 사용자 데이터 폴더 이름이다. 원본 Qt 판과 같은 폴더를 쓴다(로그 파서·E2E 하네스가 그대로 읽는다).
const AppName = "snB App"

// SettingsFile 은 사용자 데이터 폴더 안의 설정 파일 이름이다. 원본(레지스트리·fmusr.ini)과 겹치지 않게 이름을 달리한다.
const SettingsFile = "wails-settings.json"

// devBuild·sealed 는 빌드 태그가 켠다(build_dev.go·build_sealed.go).
// 태그로 정하므로 Taskfile·Docker·맨 go build 어느 경로로 만들어도 같다. -tags production 을 풀려면 snb_noenforce 태그를 더한다.
var devBuild, sealed bool

// IsDev 는 개발 빌드인가다(-tags dev 또는 SNB_DEV=1). 배포 빌드(-tags production)는 환경변수로 개발 빌드가 되지 않는다.
// 무결성 강제·인스턴스·원격 디버깅이 모두 이 판정을 쓴다.
func IsDev() bool { return devBuild || (!sealed && os.Getenv("SNB_DEV") == "1") }

// Instance 는 개발용 인스턴스 이름이다(개발 빌드의 SNB_INSTANCE, 영숫자·-·_ 만). 없으면 "".
// 데이터 폴더(「snB App <이름>」)와 단일 실행 ID 를 나눠 같은 PC 에서 여럿을 띄운다(원본 SET-082·LOG-002).
func Instance() string {
	n := os.Getenv("SNB_INSTANCE")
	if !IsDev() || !instanceName.MatchString(n) {
		return ""
	}
	return n
}

var instanceName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// DataDir 는 사용자 데이터 폴더다. Windows 는 %LOCALAPPDATA%\SQIsoft\snB App(원본 Qt AppLocalDataLocation 과 같은 자리).
func DataDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		if base = os.Getenv("LOCALAPPDATA"); base == "" {
			base, _ = os.UserCacheDir() // Windows 에서는 %LOCALAPPDATA% 다
		}
	case "darwin":
		base, _ = os.UserConfigDir() // ~/Library/Application Support
	default:
		if base = os.Getenv("XDG_DATA_HOME"); base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
	}
	name := AppName
	if n := Instance(); n != "" {
		name += " " + n
	}
	return filepath.Join(base, "SQIsoft", name)
}

// AppDir 는 fmenv.ini·fmlocal.ini·root_ca.pem 이 있는 폴더다. SNB_APP_DIR 가 있으면 그것, 없으면 실행 파일 폴더다.
// 앱은 이 폴더에 쓰지 않는다(설치 폴더, 무결성 검사 대상).
func AppDir() string {
	if d := os.Getenv("SNB_APP_DIR"); d != "" {
		return d
	}
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}
