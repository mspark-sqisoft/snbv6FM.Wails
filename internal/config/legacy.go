package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MigrateLegacy 는 원본 Qt 판의 설정을 처음 한 번 옮긴다(Settings.LegacyImported 가 거짓일 때만).
//   - Windows 레지스트리 HKCU\Software\SQIsoft\snB App(AppSettings). Qt 는 bool·문자열을 REG_SZ("true"/"false"),
//     정수를 REG_DWORD 로 적는다(이 PC 의 reg query 로 확인).
//   - fmusr.ini [USER](UserINI): autoUploadUse_<n>·autoUploadPeriod_<n>·autoUploadFolder_<n>·autoDownloadUse_<n>·
//     autoDownloadFolder_<n>(원본이 실제로 쓰는 키 이름), 레지스트리에 없을 때의 단축키·serverIndex.
//
// 옛 값이 하나도 없어도 표시는 남긴다(다음 실행부터 다시 보지 않는다). 옛 fmusr.ini 의 initFolder 는 옮기지 않는다(원본 SET-044).
// 서버가 주는 값(isApprover 등)·쓰지 않는 값(logLevel·logStorageDays·approvalAlarm*·isAutoLogin)은 옮기지 않는다.
func MigrateLegacy(st *Store, appDir, dataDir string) error {
	if st == nil || st.Get().LegacyImported {
		return nil
	}
	reg, regErr := readLegacyRegistry()
	usr := UserINI(appDir, dataDir)
	err := st.Update(func(s *Settings) {
		applyLegacy(s, reg, usr)
		s.LegacyImported = true
	})
	if err != nil {
		return err
	}
	if regErr != nil {
		return fmt.Errorf("원본 레지스트리 설정을 읽지 못했다(fmusr.ini 값만 옮겼다): %w", regErr)
	}
	return nil
}

// applyLegacy 는 옛 값(레지스트리 reg, fmusr.ini [USER] usr)을 s 에 옮긴다. 있는 키만 바꾼다. 옮긴 키 수를 돌려준다.
func applyLegacy(s *Settings, reg, usr map[string]string) int {
	n := 0
	str := func(m map[string]string, k string, dst *string) {
		if v, ok := m[k]; ok {
			*dst = v
			n++
		}
	}
	boolean := func(m map[string]string, k string, dst *bool) {
		if v, ok := m[k]; ok {
			if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
				*dst = b
				n++
			}
		}
	}
	integer := func(m map[string]string, k string, dst *int) {
		if v, ok := m[k]; ok {
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				*dst = i
				n++
			}
		}
	}
	path := func(m map[string]string, k string, dst *string) {
		if v, ok := m[k]; ok {
			*dst = legacyPath(v)
			n++
		}
	}

	var dark bool
	if _, ok := reg["isDarkTheme"]; ok {
		boolean(reg, "isDarkTheme", &dark)
		if dark {
			s.Theme = "dark"
		} else {
			s.Theme = "light"
		}
	}
	integer(reg, "layoutIndex", &s.Layout)
	integer(reg, "languageIndex", &s.Language)
	boolean(reg, "navCollapsed", &s.NavCollapsed)
	boolean(reg, "useLogFile", &s.UseLogFile)
	integer(reg, "logoutType", &s.LogoutType)
	boolean(reg, "logoutConfirmPopup", &s.LogoutConfirm)
	path(reg, "initFolder", &s.InitFolder)
	path(reg, "lastExportFolder", &s.LastExportFolder)
	integer(reg, "transferDrawerHeight", &s.TransferDrawerHeight)
	boolean(reg, "isUserIdSave", &s.SaveUserID)
	str(reg, "userId", &s.UserID)
	if _, ok := reg["serverIndex"]; ok {
		integer(reg, "serverIndex", &s.ServerIndex)
	} else {
		integer(usr, "serverIndex", &s.ServerIndex)
	}

	// 단축키: 레지스트리, 없으면 fmusr.ini(원본 initHotKey 의 두 출처)
	hk := func(k string, dst *string) {
		if _, ok := reg[k]; ok {
			str(reg, k, dst)
		} else {
			str(usr, k, dst)
		}
	}
	if _, ok := reg["clipboardHotkeyUse"]; ok {
		boolean(reg, "clipboardHotkeyUse", &s.Hotkey.Use)
	} else {
		boolean(usr, "clipboardHotkeyUse", &s.Hotkey.Use)
	}
	hk("clipboardCopyKey", &s.Hotkey.CopyKey)
	hk("clipboardCopyChar", &s.Hotkey.CopyChar)
	hk("clipboardPasteKey", &s.Hotkey.PasteKey)
	hk("clipboardPasteChar", &s.Hotkey.PasteChar)

	// 망별 자동 전송(_<n>, n 은 0 부터 ― 원본 serverIndex)
	if s.Auto == nil {
		s.Auto = map[int]AutoTransfer{}
	}
	for k := range usr {
		base, num, ok := strings.Cut(k, "_")
		if !ok || !strings.HasPrefix(base, "auto") {
			continue
		}
		idx, err := strconv.Atoi(num)
		if err != nil || idx < 0 || idx > 99 {
			continue
		}
		a := s.Auto[idx]
		switch base {
		case "autoUploadUse":
			boolean(usr, k, &a.UploadUse)
		case "autoUploadPeriod":
			integer(usr, k, &a.UploadPeriod)
		case "autoUploadFolder":
			path(usr, k, &a.UploadFolder)
		case "autoDownloadUse":
			boolean(usr, k, &a.DownloadUse)
		case "autoDownloadFolder":
			path(usr, k, &a.DownloadFolder)
		default:
			continue
		}
		s.Auto[idx] = a
	}
	return n
}

// legacyPath 는 옛 경로 값을 정리한다: file:/// 주소를 경로로, Qt 가 섞어 쓴 / 와 \ 를 운영체제 구분자로. 빈 값은 빈 값.
func legacyPath(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || !utf8.ValidString(v) {
		return ""
	}
	if rest, ok := strings.CutPrefix(v, "file:///"); ok {
		v = rest
		if len(v) < 2 || v[1] != ':' {
			v = "/" + v
		}
	}
	return filepath.Clean(v)
}
