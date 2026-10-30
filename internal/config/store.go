package config

// 사용자 설정 저장소. 원본은 레지스트리 HKCU\Software\SQIsoft\snB App(AppSettings)와 fmusr.ini [USER] 에 나눠 둔다.
// Wails 판은 사용자 데이터 폴더의 wails-settings.json 하나에 둔다(docs/ARCHITECTURE.md "설정 저장", docs/port/12 4.3·4.4·7.2).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings 는 저장하는 사용자 설정 전부다. 새 키는 여기에 칸을 더하고 Defaults 에 기본값을 둔다.
// 서버가 주는 값(결재자 여부 등)·비밀번호는 저장하지 않는다.
type Settings struct {
	Theme        string `json:"theme"`        // "system"(기본, 운영체제 따름) · "light" · "dark"(원본 isDarkTheme)
	Design       string `json:"design"`       // 디자인 id(frontend/src/designs/registry.ts). "standard" 만 Theme 을 따른다
	Language     int    `json:"language"`     // 0 한국어, 1 English
	Layout       int    `json:"layout"`       // 0 왼쪽 메뉴 + 상단 툴바, 2 상단 탭(원본 기본). 그 밖 값은 0 으로 그린다(저장값은 바꾸지 않는다)
	NavCollapsed bool   `json:"navCollapsed"` // 왼쪽 메뉴 접힘
	UseLogFile   bool   `json:"useLogFile"`   // [로그 저장] 표시값. 실제 스위치는 로그 폴더 유무

	LogoutType    int  `json:"logoutType"`    // 0 로그아웃(로그인 화면), 1 프로그램 종료, 2 트레이로 내리기
	LogoutConfirm bool `json:"logoutConfirm"` // 로그아웃 때 처리 방식 확인 창(원본 logoutConfirmPopup)
	CloseToTray   bool `json:"closeToTray"`   // 로그인 중 창 닫기(X)를 트레이로 내리기로(Wails 판에서 더함, 기본 끔 = 원본처럼 종료)

	// 로컬 탐색기 패널(원본은 저장하지 않았다 ― MAIN-118 개선)
	LocalPanelHeight int    `json:"localPanelHeight"` // 파일전송 화면 아래 패널 높이(0 = 기본)
	LocalTreeWidth   int    `json:"localTreeWidth"`   // 트리 폭(0 = 기본 190)
	LocalSortKey     string `json:"localSortKey"`     // 목록 정렬 컬럼(name·size·type·modTime, 빈 값 = 이름)
	LocalSortDesc    bool   `json:"localSortDesc"`    // 내림차순
	LocalShowHidden  bool   `json:"localShowHidden"`  // 숨김 파일 보기

	// ClipImageFormat 은 클립보드 그림을 보낼 형식이다("bmp" 원본 그대로 · "png"). 상대망 MOSFM 이 PNG 를 여는지 확인 전이라 기본 bmp(docs/port/07 8절)
	ClipImageFormat string `json:"clipImageFormat"`

	InitFolder       string `json:"initFolder"`       // 시작 폴더. 비면 바탕 화면 → 홈 → 루트
	LastFolder       string `json:"lastFolder"`       // 로컬 탐색기에서 마지막으로 연 폴더
	LastExportFolder string `json:"lastExportFolder"` // 엑셀 저장 창의 마지막 폴더

	SaveUserID  bool   `json:"saveUserId"`  // [ID 저장](원본 isUserIdSave, 기본 켬)
	UserID      string `json:"userId"`      // 저장된 ID. [ID 저장] 을 끄면 빈 값
	ServerIndex int    `json:"serverIndex"` // 로그인 콤보의 목록 자리(Server.Index). 목록 밖이면 0
	ServerName  string `json:"serverName"`  // 같은 자리의 망 이름 ― 다음 실행에 같은 이름이 있으면 그것을 고른다(09 5.1)

	TransferDrawerHeight int `json:"transferDrawerHeight"` // 전송 상태 서랍 높이(0 = 기본)

	Hotkey Hotkey `json:"hotkey"` // 클립보드 단축키

	// Auto 는 망별 자동 업로드·다운로드다. 키는 망의 목록 자리(Server.Index, 원본 fmusr.ini 의 `_<n>`).
	Auto map[int]AutoTransfer `json:"auto"`

	// ColumnWidths 는 표 이름 → 칸 폭(px)이다(예: "serverFiles", "transHistory"). 없으면 화면 기본값.
	ColumnWidths map[string][]int `json:"columnWidths"`

	// LegacyImported 는 원본 레지스트리·fmusr.ini 값을 한 번 옮겼는지다(MigrateLegacy).
	LegacyImported bool `json:"legacyImported"`
}

// Hotkey 는 클립보드 전역 단축키다. 조합키는 "ALT"·"SHIFT"·"ALT+SHIFT"·"CTRL+SHIFT", 글자는 하나.
type Hotkey struct {
	Use       bool   `json:"use"`
	CopyKey   string `json:"copyKey"`
	CopyChar  string `json:"copyChar"`
	PasteKey  string `json:"pasteKey"`
	PasteChar string `json:"pasteChar"`
}

// AutoTransfer 는 망 하나의 자동 전송 설정이다.
type AutoTransfer struct {
	UploadUse      bool   `json:"uploadUse"`
	UploadPeriod   int    `json:"uploadPeriod"` // 분. 0(미저장)이면 로그인 때 30 으로 본다
	UploadFolder   string `json:"uploadFolder"`
	DownloadUse    bool   `json:"downloadUse"`
	DownloadFolder string `json:"downloadFolder"`
}

// DefaultUploadPeriod 는 자동 업로드 주기 기본값(분)이다. 원본은 로그인 때 30, [설정] 카드에서 1 로 엇갈린다 ― 30 하나로 한다.
const DefaultUploadPeriod = 30

// Defaults 는 원본 AppSettings 의 기본값이다.
func Defaults() Settings {
	return Settings{
		Theme:           "system",
		Design:          "standard",
		Layout:          2,
		LogoutConfirm:   true,
		SaveUserID:      true,
		ClipImageFormat: "bmp",
		Hotkey:          Hotkey{CopyKey: "ALT", CopyChar: "C", PasteKey: "ALT", PasteChar: "V"},
		Auto:            map[int]AutoTransfer{},
		ColumnWidths:    map[string][]int{},
	}
}

// Store 는 Settings 를 파일 하나에 읽고 쓴다. 여러 고루틴에서 불러도 된다.
type Store struct {
	path string

	mu       sync.Mutex
	s        Settings
	onChange []func(old, cur Settings)
}

// Open 은 path 를 읽는다. 없으면 기본값이다. 깨졌으면 그 파일을 .bad 로 옮겨 두고 기본값으로 열며 오류도 함께 돌려준다
// (설정이 깨져도 앱은 뜬다 ― 원본 SET-105).
func Open(path string) (*Store, error) {
	st := &Store{path: path, s: Defaults()}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return st, nil
	}
	if err == nil {
		err = json.Unmarshal(b, &st.s)
	}
	if err != nil {
		st.s = Defaults()
		_ = os.Rename(path, path+".bad")
		return st, fmt.Errorf("설정 파일을 읽지 못해 기본값으로 시작합니다(%s.bad 로 옮겼습니다): %w", path, err)
	}
	if st.s.Auto == nil {
		st.s.Auto = map[int]AutoTransfer{}
	}
	if st.s.ColumnWidths == nil {
		st.s.ColumnWidths = map[string][]int{}
	}
	return st, nil
}

// Path 는 설정 파일 경로다.
func (st *Store) Path() string { return st.path }

// Get 은 지금 설정의 사본이다(맵도 복사한다 ― 고쳐도 저장소에 반영되지 않는다).
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return clone(st.s)
}

// Set 은 설정 전체를 바꿔 저장한다.
func (st *Store) Set(s Settings) error {
	return st.Update(func(cur *Settings) { *cur = clone(s) })
}

// Update 는 fn 으로 고친 설정을 저장한다. 값이 바뀌었을 때만 파일에 쓰고 OnChange 콜백을 부른다.
// 저장에 실패하면 메모리 값도 되돌리고 오류를 돌려준다(화면이 알림 띠로 알린다 ― SET-104).
func (st *Store) Update(fn func(*Settings)) error {
	st.mu.Lock()
	old := clone(st.s)
	next := clone(st.s)
	fn(&next)
	oldJSON, _ := json.Marshal(old)
	newJSON, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		st.mu.Unlock()
		return err
	}
	compact, _ := json.Marshal(next)
	if bytes.Equal(oldJSON, compact) {
		st.mu.Unlock()
		return nil
	}
	if err := writeAtomic(st.path, newJSON); err != nil {
		st.mu.Unlock()
		return fmt.Errorf("설정을 저장하지 못했습니다: %w", err)
	}
	st.s = next
	hooks := append([]func(old, cur Settings){}, st.onChange...)
	st.mu.Unlock()
	for _, h := range hooks {
		h(clone(old), clone(next))
	}
	return nil
}

// OnChange 는 설정이 바뀌어 저장된 뒤 부를 함수를 더한다(설정 변경 로그·화면 알림·자동 전송 재시작 등).
func (st *Store) OnChange(fn func(old, cur Settings)) {
	st.mu.Lock()
	st.onChange = append(st.onChange, fn)
	st.mu.Unlock()
}

// writeAtomic 은 같은 폴더의 임시 파일에 쓴 뒤 이름을 바꾼다 ― 쓰다가 꺼져도 앞 파일이 남는다.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // 이름을 바꾼 뒤에는 지울 것이 없다
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// clone 은 JSON 왕복으로 깊은 사본을 만든다(맵·조각까지).
func clone(s Settings) Settings {
	b, _ := json.Marshal(s)
	var out Settings
	_ = json.Unmarshal(b, &out)
	if out.Auto == nil {
		out.Auto = map[int]AutoTransfer{}
	}
	if out.ColumnWidths == nil {
		out.ColumnWidths = map[string][]int{}
	}
	return out
}
