//go:build !windows

package config

// readLegacyRegistry 는 Windows 밖에서 빈 표다(원본 macOS·Linux 판의 plist·conf 는 옮기지 않는다 ― 배포 대상이 Windows 먼저).
func readLegacyRegistry() (map[string]string, error) { return map[string]string{}, nil }

// SystemDark 는 Windows 밖에서 거짓이다(화면이 prefers-color-scheme 으로 곧 맞춘다).
func SystemDark() bool { return false }
