// Package history 는 전송내역·결재내역 조회와 엑셀 저장이다(파일·내역 영역, docs/port/04 3.5·5.7·7.9).
package history

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"google.golang.org/grpc/status"

	"snbapp/internal/nbs"
	"snbapp/pb/approval"
)

// 조회 구분(AprovalHistoryRequest.type).
const (
	TypeApproval int32 = 0 // 결재내역(내가 결재한 것)
	TypeTransfer int32 = 1 // 전송내역(내가 요청한 것)
)

// PageSize 는 한 번에 받을 건수다. 받은 건수가 이만큼이면 이어 받는다.
const PageSize = 10000

const maxPages = 100

// ErrRange 는 시작일이 종료일보다 늦을 때다(원본은 그대로 보냈다 ― HIST-005).
var ErrRange = errors.New("시작일이 종료일보다 늦습니다.")

// Range 는 화면 날짜 "yyyy-MM-dd" 둘을 서버 형식으로 바꾼다: 시작 YYYYMMDD, 종료 YYYYMMDD235959(원본과 같다 ―
// proto 주석의 형식과 다르지만 서버가 문자열 그대로 BETWEEN 에 넣고 이 형식으로 동작한다).
func Range(from, to string) (start, end string, err error) {
	f, err1 := time.Parse(time.DateOnly, from)
	t, err2 := time.Parse(time.DateOnly, to)
	if err1 != nil || err2 != nil {
		return "", "", fmt.Errorf("날짜 형식이 맞지 않습니다: %s ~ %s", from, to)
	}
	if f.After(t) {
		return "", "", ErrRange
	}
	return f.Format("20060102"), t.Format("20060102") + "235959", nil
}

// Fetch 는 기간 안의 내역 전체다(GetAprovalHistory[3360]). 10000건씩 이어 받아 다 모은 뒤 돌려준다.
func Fetch(s *nbs.Session, typ int32, from, to string) ([]*approval.AprvInfo, error) {
	start, end, err := Range(from, to)
	if err != nil {
		return nil, err
	}
	return fetchPages(func(at int32) ([]*approval.AprvInfo, error) {
		ctx, cancel := s.Ctx(nbs.RPCTimeout)
		defer cancel()
		res, err := s.Approval().GetAprovalHistory(ctx, &approval.AprovalHistoryRequest{
			UserId: s.UserID, StartDate: start, EndDate: end, Type: typ, Start: at, Count: PageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("내역을 받지 못했습니다: %s", status.Convert(err).Message())
		}
		return res.List, nil
	})
}

func fetchPages(page func(start int32) ([]*approval.AprvInfo, error)) ([]*approval.AprvInfo, error) {
	var all []*approval.AprvInfo
	for i := 0; i < maxPages; i++ {
		got, err := page(int32(len(all)))
		if err != nil {
			return nil, err
		}
		all = append(all, got...)
		if len(got) < PageSize {
			break
		}
	}
	return all, nil
}

// AprvStateText 는 [결재상태] 글자다(원본 ViewController.cpp:430-514).
func AprvStateText(v int32) string {
	switch v {
	case 0:
		return "결재 요청"
	case 1:
		return "결재 승인"
	case 2:
		return "결재 반려"
	case 3:
		return "사후 승인"
	case 4:
		return "결재요청 가능"
	}
	return "Unknown"
}

// AprvTypeText 는 [결재방식] 글자다.
func AprvTypeText(v int32) string {
	names := []string{"자동 결재", "사전 결재", "확장자결재", "사후 결재", "요청시 사전사후결정", "요청시 결재생략결정", "자동자가결재", "자동,사전,사후"}
	if v >= 0 && int(v) < len(names) {
		return names[v]
	}
	return fmt.Sprintf("Unknown(%d)", v)
}

// BaseName 은 경로의 마지막 조각이다(file_name_pc 에 경로가 붙어 올 수 있다).
func BaseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Sheet 는 엑셀로 쓸 표다. Rows 의 칸 수는 Headers 와 같다. 1칸에는 번호(1부터)를 숫자로 붙인다.
type Sheet struct {
	Title   string
	Headers []string
	Rows    [][]string
}

// Title 은 엑셀 1행 제목이다: "<화면 이름> (<시작일>~<종료일>)" + 단어가 있으면 " 검색어:<단어>".
func Title(screen, from, to, word string) string {
	t := fmt.Sprintf("%s (%s~%s)", screen, from, to)
	if word != "" {
		t += " 검색어:" + word
	}
	return t
}

// FileName 은 저장 창의 처음 이름이다: "<화면 이름>_yyyy-MM-dd_hhmmss.xlsx".
func FileName(screen string, now time.Time) string {
	return screen + "_" + now.Format("2006-01-02_150405") + ".xlsx"
}

// WriteXLSX 는 원본 TransHistoryModel 의 엑셀 형식으로 path 에 쓴다: 1행 제목(병합·가운데·굵게 20pt),
// 2행 「번호」+머리글(흰 글씨·남색 바탕·굵게·가운데), 3행부터 자료. 자료 칸은 모두 글자 셀(수식·링크로 해석되지 않게, 원본 C-020).
// 칸 폭은 그 칸 가장 긴 글자 수 × 1.2.
func WriteXLSX(path string, sh Sheet) error {
	f := excelize.NewFile()
	defer f.Close()
	const name = "Sheet1"
	cols := len(sh.Headers) + 1
	cell := func(c, r int) string { s, _ := excelize.CoordinatesToCellName(c, r); return s }

	titleStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 20, Family: "Pretendard Black"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	headStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"000080"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}

	widths := make([]int, cols)
	set := func(c, r int, v string) error {
		if n := utf8.RuneCountInString(v); n > widths[c-1] {
			widths[c-1] = n
		}
		return f.SetCellStr(name, cell(c, r), v)
	}
	if err := f.SetCellStr(name, "A1", sh.Title); err != nil {
		return err
	}
	if err := f.MergeCell(name, "A1", cell(cols, 1)); err != nil {
		return err
	}
	if err := f.SetCellStyle(name, "A1", cell(cols, 1), titleStyle); err != nil {
		return err
	}
	_ = f.SetRowHeight(name, 1, 30)
	for c, h := range append([]string{"번호"}, sh.Headers...) {
		if err := set(c+1, 2, h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(name, "A2", cell(cols, 2), headStyle); err != nil {
		return err
	}
	for i, row := range sh.Rows {
		r := i + 3
		if err := f.SetCellInt(name, cell(1, r), int64(i+1)); err != nil {
			return err
		}
		if n := len(fmt.Sprint(i + 1)); n > widths[0] {
			widths[0] = n
		}
		for c, v := range row {
			if c+2 > cols {
				break
			}
			if err := set(c+2, r, v); err != nil {
				return err
			}
		}
	}
	for c, w := range widths {
		col, _ := excelize.ColumnNumberToName(c + 1)
		if err := f.SetColWidth(name, col, col, max(float64(w)*1.2, 4)); err != nil {
			return err
		}
	}
	return f.SaveAs(path)
}
