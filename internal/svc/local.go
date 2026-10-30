package svc

// 메인 영역: 로컬 탐색기(docs/port/12 5.7·7.3~7.6). 읽기·정렬은 internal/localfs, 여기는 화면용 모양으로 바꾸기만 한다.

import (
	"snbapp/internal/applog"
	"snbapp/internal/localfs"
)

// LocalService 는 [파일전송] 화면의 탐색기 패널(views/local/LocalPanel.svelte)이 부른다.
type LocalService struct{ base }

func newLocalService(c *Core) *LocalService { return &LocalService{base{c}} }

// LocalEntry 는 탐색기 목록의 한 항목이다.
type LocalEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	IsLink   bool   `json:"isLink"` // .lnk 바로 가기
	Size     int64  `json:"size"`
	SizeText string `json:"sizeText"` // 「10 byte」·「2 KB」·「1.0 MB」(폴더는 빈 값)
	Ext      string `json:"ext"`      // 확장자 대문자
	Type     string `json:"type"`     // 운영체제 유형 설명(예: 텍스트 문서). 빈 값이면 화면이 「%1 파일」·「파일」로
	ModTime  string `json:"modTime"`  // yyyy-MM-dd hh:mm(지역 시각)
	ModUnix  int64  `json:"modUnix"`  // 정렬용 초
}

// LocalList 는 폴더 하나의 목록이다. Req 는 화면이 보낸 요청 번호 그대로(늦게 온 옛 결과를 화면이 버린다).
type LocalList struct {
	Req     int64        `json:"req"`
	Path    string       `json:"path"`    // 정리한 절대 경로
	Parent  string       `json:"parent"`  // 상위 폴더(뿌리면 빈 값)
	Entries []LocalEntry `json:"entries"` // 폴더 먼저, 이름 자연 정렬
}

// List 는 dir 의 목록이다. 숨김 파일은 설정 localShowHidden 일 때만.
// 오류 문구(원본 그대로): 「폴더를 찾을 수 없습니다.」·「폴더를 읽을 권한이 없습니다.」
func (l *LocalService) List(req int64, dir string) (*LocalList, error) {
	hidden := false
	if st := l.c.Settings(); st != nil {
		hidden = st.Get().LocalShowHidden
	}
	p, es, err := localfs.List(dir, hidden)
	if err != nil {
		return nil, err
	}
	out := &LocalList{Req: req, Path: p, Parent: localfs.Parent(p), Entries: make([]LocalEntry, len(es))}
	for i, e := range es {
		out.Entries[i] = LocalEntry{
			Name: e.Name, Path: e.Path, IsDir: e.IsDir, IsLink: e.IsLink, Size: e.Size, SizeText: e.SizeText,
			Ext: e.Ext, Type: e.Type, ModTime: e.ModTime.Format("2006-01-02 15:04"), ModUnix: e.ModTime.Unix(),
		}
	}
	return out, nil
}

// LocalPlace 는 트리 뿌리 하나다. Kind: desktop(바탕 화면)·documents(내 문서)·drive(내 컴퓨터 아래 드라이브).
// Label 은 드라이브만 채운다(볼륨 이름) ― 바탕 화면·내 문서 글자는 화면이 번역한다.
type LocalPlace struct {
	Kind  string `json:"kind"`
	Path  string `json:"path"`
	Label string `json:"label"`
	Drive string `json:"drive"` // fixed·removable·network·cdrom·ram
}

// Places 는 트리 뿌리다: 바탕 화면, 내 문서, 드라이브들. 부를 때마다 드라이브를 다시 읽는다(원본은 시작 때 한 번).
func (l *LocalService) Places() ([]LocalPlace, error) {
	out := []LocalPlace{
		{Kind: "desktop", Path: localfs.DesktopDir()},
		{Kind: "documents", Path: localfs.DocumentsDir()},
	}
	for _, d := range localfs.Drives() {
		out = append(out, LocalPlace{Kind: "drive", Path: d.Path, Label: d.Label, Drive: d.Kind})
	}
	return out, nil
}

// LocalStart 는 시작 폴더와 그 출처다. Kind: configured(설정 시작 폴더)·last(마지막 폴더)·desktop·home·root.
type LocalStart struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// StartFolder 는 탐색기 첫 폴더다: 설정 initFolder → 마지막 폴더(lastFolder) → 바탕 화면 → 홈 → 루트.
// 쓸 수 없는 저장값은 건너뛰되 지우지 않는다.
func (l *LocalService) StartFolder() (*LocalStart, error) { return l.c.startFolder(), nil }

// startFolder 는 탐색기 첫 폴더다.
func (c *Core) startFolder() *LocalStart {
	var init, last string
	if st := c.Settings(); st != nil {
		s := st.Get()
		init, last = s.InitFolder, s.LastFolder
	}
	p, k := localfs.ResolveStart(init, last)
	return &LocalStart{Path: p, Kind: string(k)}
}

// dialogFolder 는 파일·폴더 선택 창이 처음 열릴 폴더다. 탐색기와 달리 마지막 폴더를 따르지 않는다(원본 HomePage.qml:304-309).
func (c *Core) dialogFolder() string {
	init := ""
	if st := c.Settings(); st != nil {
		init = st.Get().InitFolder
	}
	p, _ := localfs.ResolveStart(init, "")
	return p
}

// DefaultFolder 는 설정 시작 폴더가 없을 때 열릴 폴더다(바탕 화면 → 홈 → 루트) ― [설정] 의 안내 글자 「바탕 화면 (%1)」 용.
func (l *LocalService) DefaultFolder() *LocalStart {
	p, k := localfs.DefaultStart()
	return &LocalStart{Path: p, Kind: string(k)}
}

// MakeDir 는 parent 아래에 새 폴더를 만들고 경로를 돌려준다. 오류 문구는 원본 그대로(docs/port/12 3.4·7.5).
func (l *LocalService) MakeDir(parent, name string) (string, error) {
	p, err := localfs.MakeDir(parent, name)
	if err == nil {
		l.c.Log().Info("새 폴더 >>> "+p, "category", applog.CatUI)
	}
	return p, err
}
