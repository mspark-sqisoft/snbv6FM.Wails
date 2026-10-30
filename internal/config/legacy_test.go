package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApplyLegacy(t *testing.T) {
	// 이 PC 의 실제 reg query·fmusr.ini 모양을 본뜬 값
	reg := map[string]string{
		"isDarkTheme": "false", "layoutIndex": "0", "languageIndex": "1", "navCollapsed": "true",
		"useLogFile": "true", "logoutType": "2", "logoutConfirmPopup": "false",
		"initFolder": `C:\Users\park\Desktop\작업`, "lastExportFolder": "file:///C:/Users/park/Downloads",
		"transferDrawerHeight": "240", "isUserIdSave": "true", "userId": "test1_1", "serverIndex": "1",
		"clipboardHotkeyUse": "true", "clipboardCopyKey": "CTRL+SHIFT", "clipboardCopyChar": "Q",
		"isApprover": "true", "logLevel": "5", // 옮기지 않는 값
	}
	usr := map[string]string{
		"autoUploadUse_0": "true", "autoUploadPeriod_0": "15", "autoUploadFolder_0": `C:/Users/park/Desktop\AUTO_UPLOAD`,
		"autoDownloadUse_1": "true", "autoDownloadFolder_1": "",
		"autoUploadPeriod": "30", "initFolder": "C:/Users/park/Desktop", // 접미사 없는 옛 키·옛 initFolder 는 무시
		"clipboardPasteKey": "ALT+SHIFT", "clipboardPasteChar": "B", // 레지스트리에 없으니 fmusr.ini 값
		"serverIndex": "0", // 레지스트리 값이 이긴다
	}
	s := Defaults()
	applyLegacy(&s, reg, usr)

	if s.Theme != "light" || s.Layout != 0 || s.Language != 1 || !s.NavCollapsed || !s.UseLogFile ||
		s.LogoutType != 2 || s.LogoutConfirm || s.TransferDrawerHeight != 240 || s.UserID != "test1_1" || s.ServerIndex != 1 {
		t.Fatalf("레지스트리 값: %+v", s)
	}
	if runtime.GOOS == "windows" {
		if s.LastExportFolder != `C:\Users\park\Downloads` {
			t.Errorf("lastExportFolder = %q", s.LastExportFolder)
		}
		if got := s.Auto[0].UploadFolder; got != `C:\Users\park\Desktop\AUTO_UPLOAD` {
			t.Errorf("autoUploadFolder_0 = %q", got)
		}
	}
	if s.InitFolder == "" {
		t.Error("initFolder 를 옮기지 않았다")
	}
	h := s.Hotkey
	if !h.Use || h.CopyKey != "CTRL+SHIFT" || h.CopyChar != "Q" || h.PasteKey != "ALT+SHIFT" || h.PasteChar != "B" {
		t.Errorf("단축키: %+v", h)
	}
	if a := s.Auto[0]; !a.UploadUse || a.UploadPeriod != 15 {
		t.Errorf("auto 0: %+v", a)
	}
	if a := s.Auto[1]; !a.DownloadUse || a.DownloadFolder != "" {
		t.Errorf("auto 1: %+v", a)
	}
	if len(s.Auto) != 2 {
		t.Errorf("auto 키: %v", s.Auto)
	}
}

func TestApplyLegacyEmpty(t *testing.T) {
	s := Defaults()
	if n := applyLegacy(&s, map[string]string{}, map[string]string{}); n != 0 {
		t.Fatalf("옮긴 수 %d", n)
	}
	if s.Theme != "system" || s.Layout != 2 || !s.LogoutConfirm {
		t.Fatalf("기본값이 바뀌었다: %+v", s)
	}
	s2 := Defaults()
	applyLegacy(&s2, map[string]string{"isDarkTheme": "true", "layoutIndex": "x"}, nil)
	if s2.Theme != "dark" || s2.Layout != 2 {
		t.Fatalf("다크·잘못된 정수: %+v", s2)
	}
}

func TestMigrateLegacyOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fmusr.ini"), []byte("[USER]\nautoUploadUse_0=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := Open(filepath.Join(dir, SettingsFile))
	_ = MigrateLegacy(st, dir, dir) // 레지스트리는 이 PC 값일 수 있다 ― 표시와 fmusr.ini 값만 본다
	if !st.Get().LegacyImported || !st.Get().Auto[0].UploadUse {
		t.Fatalf("옮기지 않았다: %+v", st.Get())
	}
	_ = st.Update(func(s *Settings) { s.Auto[0] = AutoTransfer{} })
	_ = MigrateLegacy(st, dir, dir)
	if st.Get().Auto[0].UploadUse {
		t.Fatal("두 번 옮겼다")
	}
}

func TestChanges(t *testing.T) {
	a := Defaults()
	b := Defaults()
	b.Theme = "dark"
	b.Auto[0] = AutoTransfer{UploadUse: true}
	cs := Changes(a, b)
	if len(cs) != 2 || cs[0].Key != "auto" || cs[1].Key != "theme" || cs[1].Old != "system" || cs[1].New != "dark" {
		t.Fatalf("changes = %+v", cs)
	}
	if len(Changes(a, a)) != 0 {
		t.Fatal("같은 값인데 바뀜이 있다")
	}
}
