package svc

// 파일·내역 영역: 전송내역·결재내역, 엑셀 저장(docs/port/04 3.5·5.7·7.9).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"snbapp/internal/applog"
	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/history"
	"snbapp/pb/approval"
)

// HistoryService 는 [전송내역]·[결재내역] 화면이 부른다.
type HistoryService struct {
	base
	exporting atomic.Bool
}

func newHistoryService(c *Core) *HistoryService { return &HistoryService{base: base{c}} }

// HistoryRow 는 내역 표의 한 줄이다(서버 AprvInfo). 코드 칸 옆에 화면 글자를 함께 싣는다.
type HistoryRow struct {
	Dept          string `json:"dept"`
	FileName      string `json:"fileName"` // file_name_pc 의 마지막 경로 조각
	FileSize      uint64 `json:"fileSize"`
	FileSizeText  string `json:"fileSizeText"`
	Requester     string `json:"requester"`
	RequestTime   string `json:"requestTime"`
	RequestCause  string `json:"requestCause"`
	Course        int32  `json:"course"`
	CourseText    string `json:"courseText"` // [전송방향]: 그 LOC 의 fmenv.ini NAME, 없으면 Unknown
	AprvState     int32  `json:"aprvState"`
	AprvStateText string `json:"aprvStateText"`
	AprvType      int32  `json:"aprvType"`
	AprvTypeText  string `json:"aprvTypeText"`
	Approver      string `json:"approver"`    // 서버 값 그대로("-" 포함 ― 화면은 빈칸으로, 엑셀은 그대로)
	ActApprover   string `json:"actApprover"` // 실제 결재자(대리결재). 지정 결재자와 같거나 비면 빈 값
	AprvTime      string `json:"aprvTime"`
	AprvCause     string `json:"aprvCause"`
	DownTime      string `json:"downTime"`
}

// TransferHistory 는 전송내역(내가 요청한 것)이다. from·to 는 화면 날짜 "yyyy-MM-dd". 10000건씩 이어 받아 다 모은다.
func (h *HistoryService) TransferHistory(from, to string) ([]HistoryRow, error) {
	return h.fetch(history.TypeTransfer, from, to)
}

// ApprovalHistory 는 결재내역(내가 결재한 것)이다. 메뉴는 업무망의 결재자에게만 보인다.
func (h *HistoryService) ApprovalHistory(from, to string) ([]HistoryRow, error) {
	return h.fetch(history.TypeApproval, from, to)
}

func (h *HistoryService) fetch(typ int32, from, to string) ([]HistoryRow, error) {
	s, err := h.c.Session()
	if err != nil {
		return nil, err
	}
	start := time.Now()
	list, err := history.Fetch(s, typ, from, to)
	if err != nil {
		return nil, err
	}
	h.c.Log().Debug("[내역] 조회", "category", applog.CatUI, "type", typ, "from", from, "to", to, "rows", len(list), "ms", time.Since(start).Milliseconds())
	return historyRows(list, filesEnvServers(h.c)), nil
}

func historyRows(list []*approval.AprvInfo, servers []config.Server) []HistoryRow {
	rows := make([]HistoryRow, 0, len(list))
	for _, x := range list {
		act := strings.TrimSpace(x.ActAprvUser)
		if act == "-" || act == strings.TrimSpace(x.AprvUser) {
			act = ""
		}
		rows = append(rows, HistoryRow{
			Dept: x.AprvDept, FileName: history.BaseName(x.FileNamePc), FileSize: x.FileSize, FileSizeText: files.SizeText(x.FileSize),
			Requester: x.FileUserName, RequestTime: x.FileDate, RequestCause: x.FileCause,
			Course: x.FileCourse, CourseText: files.CourseName(servers, x.FileCourse),
			AprvState: x.AprvState, AprvStateText: history.AprvStateText(x.AprvState),
			AprvType: x.AprvType, AprvTypeText: history.AprvTypeText(x.AprvType),
			Approver: x.AprvUser, ActApprover: act, AprvTime: x.AprvDate, AprvCause: x.AprvCause, DownTime: x.DownDate,
		})
	}
	return rows
}

// ExportRequest 는 엑셀 저장 요청이다. Rows 는 화면에 보이는 순서(정렬·거르기 반영) 그대로.
// From·To·Word 는 마지막으로 조회·거른 조건(제목에 쓴다).
type ExportRequest struct {
	Screen string       `json:"screen"` // "전송내역" · "결재내역"
	From   string       `json:"from"`
	To     string       `json:"to"`
	Word   string       `json:"word"`
	Rows   []HistoryRow `json:"rows"`
}

// 엑셀 저장 결과 문구(원본 그대로).
var (
	ErrExportEmpty = errors.New("엑셀로 저장할 내역이 없습니다.")
	ErrExportBusy  = errors.New("앞서 저장한 파일을 만드는 중입니다. 잠시 뒤에 다시 눌러 주세요.")
	ErrExportWrite = errors.New("파일을 저장하지 못했습니다.\n같은 이름의 파일이 열려 있는지 확인해 주세요.")
)

var historyHeaders = []string{"부서", "파일명", "파일크기", "요청자", "요청일시", "요청사유", "전송방향", "결재상태", "결재방식", "결재자", "결재일시", "결재사유", "다운로드일시"}

// ExportExcel 은 저장 창(「<화면 이름> 저장」)에서 고른 .xlsx 에 행을 쓰고 경로를 돌려준다. 창을 취소하면 빈 값.
// 처음 폴더는 마지막으로 저장한 폴더(없으면 다운로드 폴더)이고, 고른 폴더를 기억한다.
func (h *HistoryService) ExportExcel(req ExportRequest) (string, error) {
	if len(req.Rows) == 0 {
		return "", ErrExportEmpty
	}
	if !h.exporting.CompareAndSwap(false, true) {
		return "", ErrExportBusy
	}
	defer h.exporting.Store(false)

	dir := ""
	if st := h.c.Settings(); st != nil {
		dir = st.Get().LastExportFolder
	}
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, "Downloads")
		}
	}
	path, err := application.Get().Dialog.SaveFile().
		SetMessage(req.Screen+" 저장").
		AddFilter("엑셀 통합 문서 (*.xlsx)", "*.xlsx").
		SetDirectory(dir).
		SetFilename(history.FileName(req.Screen, time.Now())).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	if st := h.c.Settings(); st != nil {
		_ = st.Update(func(s *config.Settings) { s.LastExportFolder = filepath.Dir(path) })
	}

	rows := make([][]string, len(req.Rows))
	for i, r := range req.Rows {
		rows[i] = []string{r.Dept, r.FileName, r.FileSizeText, r.Requester, r.RequestTime, r.RequestCause, r.CourseText,
			r.AprvStateText, r.AprvTypeText, r.Approver, r.AprvTime, r.AprvCause, r.DownTime}
	}
	start := time.Now()
	if err := history.WriteXLSX(path, history.Sheet{Title: history.Title(req.Screen, req.From, req.To, req.Word), Headers: historyHeaders, Rows: rows}); err != nil {
		h.c.Log().Warn("[내역] 엑셀 저장 실패", "category", applog.CatUI, "path", path, "err", err)
		return "", ErrExportWrite
	}
	h.c.Log().Info("[내역] 엑셀 저장", "category", applog.CatUI, "path", path, "rows", len(rows), "ms", time.Since(start).Milliseconds())
	return path, nil
}
