package config

import (
	"errors"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// legacyRegKey 는 원본 Qt 판 AppSettings 가 쓰는 레지스트리 자리다(설치본·자리 1).
const legacyRegKey = `Software\SQIsoft\snB App`

// readLegacyRegistry 는 원본 레지스트리 값을 이름 → 글자로 읽는다(정수는 10진 글자). 키가 없으면 빈 표·오류 없음.
func readLegacyRegistry() (map[string]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, legacyRegKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return map[string]string{}, err
	}
	defer k.Close()
	names, err := k.ReadValueNames(-1)
	if err != nil {
		return map[string]string{}, err
	}
	out := make(map[string]string, len(names))
	for _, n := range names {
		if s, _, err := k.GetStringValue(n); err == nil {
			out[n] = s
		} else if i, _, err := k.GetIntegerValue(n); err == nil {
			out[n] = strconv.FormatUint(i, 10)
		}
	}
	return out, nil
}

// SystemDark 는 Windows 앱 모드가 어두운가다(HKCU\...\Themes\Personalize AppsUseLightTheme = 0).
// 창 배경색을 첫 그림 전에 테마와 맞춰 흰 번쩍임을 막는 데 쓴다. 못 읽으면 거짓(밝음).
func SystemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}
