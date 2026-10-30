package history

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"snbapp/pb/approval"
)

func TestRange(t *testing.T) {
	s, e, err := Range("2026-10-23", "2026-10-30")
	if err != nil || s != "20261023" || e != "20261030235959" {
		t.Errorf("%q %q %v", s, e, err)
	}
	if _, _, err := Range("2026-10-30", "2026-10-29"); err != ErrRange {
		t.Errorf("뒤집힌 기간: %v", err)
	}
	if _, _, err := Range("2026-10-30", "2026-10-30"); err != nil {
		t.Errorf("같은 날: %v", err)
	}
	if _, _, err := Range("20261030", "2026-10-30"); err == nil {
		t.Error("형식 오류를 받았다")
	}
}

func TestTexts(t *testing.T) {
	for v, want := range map[int32]string{0: "결재 요청", 1: "결재 승인", 2: "결재 반려", 3: "사후 승인", 4: "결재요청 가능", 9: "Unknown"} {
		if got := AprvStateText(v); got != want {
			t.Errorf("state %d: %q", v, got)
		}
	}
	for v, want := range map[int32]string{0: "자동 결재", 2: "확장자결재", 7: "자동,사전,사후", 8: "Unknown(8)", -1: "Unknown(-1)"} {
		if got := AprvTypeText(v); got != want {
			t.Errorf("type %d: %q", v, got)
		}
	}
	if BaseName(`C:\a\b.txt`) != "b.txt" || BaseName("b.txt") != "b.txt" {
		t.Error("BaseName")
	}
	if Title("전송내역", "2026-10-23", "2026-10-30", "") != "전송내역 (2026-10-23~2026-10-30)" ||
		Title("결재내역", "a", "b", "x") != "결재내역 (a~b) 검색어:x" {
		t.Error("Title")
	}
	if got := FileName("전송내역", time.Date(2026, 10, 30, 9, 5, 7, 0, time.Local)); got != "전송내역_2026-10-30_090507.xlsx" {
		t.Errorf("FileName: %q", got)
	}
}

func TestFetchPages(t *testing.T) {
	total := PageSize + 3
	var starts []int32
	got, err := fetchPages(func(at int32) ([]*approval.AprvInfo, error) {
		starts = append(starts, at)
		return make([]*approval.AprvInfo, min(PageSize, total-int(at))), nil
	})
	if err != nil || len(got) != total || fmt.Sprint(starts) != fmt.Sprint([]int32{0, PageSize}) {
		t.Errorf("%d %v %v", len(got), starts, err)
	}
}

func TestWriteXLSX(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.xlsx")
	err := WriteXLSX(p, Sheet{Title: "전송내역 (a~b)", Headers: []string{"부서", "파일명"}, Rows: [][]string{{"개발", "=1+1"}, {"-", "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for cell, want := range map[string]string{"A1": "전송내역 (a~b)", "A2": "번호", "C2": "파일명", "A3": "1", "C3": "=1+1", "B4": "-"} {
		if v, _ := f.GetCellValue("Sheet1", cell); v != want {
			t.Errorf("%s = %q, want %q", cell, v, want)
		}
	}
	if fo, _ := f.GetCellFormula("Sheet1", "C3"); fo != "" {
		t.Errorf("수식으로 들어갔다: %q", fo)
	}
	if ty, _ := f.GetCellType("Sheet1", "A3"); ty == excelize.CellTypeSharedString || ty == excelize.CellTypeInlineString {
		t.Error("번호가 글자 셀이다")
	}
}
