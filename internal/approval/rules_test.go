package approval

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	vpb "snbapp/pb/approver"
)

func TestPreMode(t *testing.T) {
	for _, c := range []struct {
		p    map[string]string
		want bool
	}{
		{map[string]string{}, true}, // 키가 없으면 0(사전)
		{map[string]string{"fm_upfile_mode": "0"}, true},
		{map[string]string{"fm_upfile_mode": "1"}, false},
		{map[string]string{"fm_upfile_mode": " 1 "}, false},
	} {
		if got := PreMode(c.p); got != c.want {
			t.Errorf("PreMode(%v) = %v", c.p, got)
		}
	}
}

func TestNeedsApprover(t *testing.T) {
	for typ, want := range map[int32]bool{0: false, 1: true, 2: true, 3: true, 4: true, 5: true, 6: false, 7: true} {
		if NeedsApprover(typ) != want {
			t.Errorf("NeedsApprover(%d)", typ)
		}
	}
}

func TestTexts(t *testing.T) {
	if TypeText(0) != "자동 결재" || TypeText(6) != "자동자가결재" || TypeText(7) != "자동,사전,사후" || TypeText(9) != "Unknown(9)" || TypeText(-1) != "Unknown(-1)" {
		t.Error("TypeText")
	}
	want := map[int32]string{0: "결재 요청", 1: "결재 승인", 2: "결재 반려", 3: "사후 승인", 4: "결재요청 가능", 99: "Unknown"}
	for s, w := range want {
		if StateText(s) != w {
			t.Errorf("StateText(%d) = %s", s, StateText(s))
		}
	}
}

// 원본 tst_FileMoverPolicy getState_*: 백신 → 개인정보 → 결재상태 순서, 두 번째 값 4 만 상신 가능.
func TestCanRequest(t *testing.T) {
	for _, virus := range []int32{-1, 0, 1, 2, 3, 4, 5, 6, 21, 22, 99} {
		for _, priv := range []int32{-1, 0, 1, 2, 3, 99} {
			for _, aprv := range []int32{0, 1, 2, 3, 4, 99} {
				want := (virus == 1 || virus == 4) && (priv == 1 || priv == 3) && aprv == 4
				if got := CanRequest(virus, priv, aprv); got != want {
					t.Errorf("CanRequest(%d,%d,%d) = %v", virus, priv, aprv, got)
				}
			}
		}
	}
}

func TestUploadFields(t *testing.T) {
	for _, c := range []struct {
		clip, pre bool
		typ       int32
		n         int
		resv      string
		kind      int32
	}{
		{false, true, 1, 1, "1", 0},  // 사전결재 + 결재자
		{false, true, 0, 0, "2", 0},  // 자동 결재
		{false, true, 6, 1, "1", 0},  // 자동자가: 결재자 = 나
		{false, false, 3, 0, "2", 3}, // 사후결재 모드, 결재방식 3
		{false, false, 1, 1, "2", 0}, // 사후 모드는 결재자가 있어도 2
		{false, true, 3, 1, "1", 3},
		{true, true, 1, 1, "1", 0},
		{true, true, 0, 0, "1", 0},
		{true, false, 3, 0, "2", 0},
	} {
		resv, kind := UploadFields(c.clip, c.pre, c.typ, c.n)
		if resv != c.resv || kind != c.kind {
			t.Errorf("UploadFields(%v,%v,%d,%d) = %s,%d", c.clip, c.pre, c.typ, c.n, resv, kind)
		}
	}
}

func ai(id, name, dept, deptName string, state int32, rname string) *vpb.ApproverInfo {
	return &vpb.ApproverInfo{UserId: id, UserName: name, UserDeptId: dept, UserDeptName: deptName, AprvState: state, Rname: rname}
}

func TestBuildTree(t *testing.T) {
	list := []*vpb.ApproverInfo{
		ai("u0", "비결재자", "A", "본사", 0, ""), // aprv_state 0 은 넣지 않는다
		ai("u1", "결재자1", "A", "본사", 1, ""),
		ai("u2", "결재자2", "A|B", "연구소", 1, "대리"),
		ai("u3", "결재자3", "X|Y", "고아부서", 1, ""), // 부모 X 가 없다 → 뿌리
		ai("u1", "결재자1", "A", "본사", 1, ""),     // 중복 ID
		ai("u4", "결재자4", "A|B", "연구소", 1, ""),
		ai("u5", "결재자5", "A", "본사", 1, ""),
	}
	roots := BuildTree(list)
	if len(roots) != 2 || roots[0].ID != "A" || roots[1].ID != "X|Y" {
		t.Fatalf("뿌리 %+v", roots)
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].ID != "A|B" {
		t.Fatalf("하위 %+v", roots[0].Children)
	}
	var ids, labels []string
	for _, u := range TreeUsers(roots) {
		ids = append(ids, u.ID)
		labels = append(labels, u.Label)
	}
	if !reflect.DeepEqual(ids, []string{"u1", "u5", "u2", "u4", "u3"}) {
		t.Errorf("트리 순서 %v", ids)
	}
	if labels[2] != "결재자2(대결:대리)" || TreeUsers(roots)[2].Name != "결재자2" {
		t.Errorf("대결 표시 %v", labels)
	}
	if BuildTree([]*vpb.ApproverInfo{ai("u0", "x", "A", "a", 0, "")}) != nil {
		t.Error("결재자 0명이면 nil")
	}
}

func TestCandidates(t *testing.T) {
	got := Candidates([]*vpb.ApproverInfo{ai("me", "나", "A", "a", 1, ""), ai("u1", "1", "A", "a", 0, ""), ai("u1", "1", "A", "a", 0, ""), ai("u2", "2", "A", "a", 1, "")}, "me")
	if len(got) != 2 || got[0].ID != "u1" || got[1].ID != "u2" {
		t.Errorf("%+v", got)
	}
}

// 원본 tst_ZipPasswordGate::groups_byPasswordInTens 의 사례.
func TestGroupByZipPswd(t *testing.T) {
	var names, pws []string
	for i := range 23 {
		names = append(names, fmt.Sprintf("f%d", i))
		pws = append(pws, []string{"A", "", "B"}[i%3])
	}
	g := GroupByZipPswd(names, pws, 10)
	if len(g) != 3 || g[0].Pswd != "A" || g[1].Pswd != "" || g[2].Pswd != "B" {
		t.Fatalf("%+v", g)
	}
	if !reflect.DeepEqual(g[0].Names, []string{"f0", "f3", "f6", "f9", "f12", "f15", "f18", "f21"}) || len(g[2].Names) != 7 {
		t.Errorf("%+v", g)
	}

	var many, same []string
	for i := range 23 {
		many = append(many, fmt.Sprintf("g%d", i))
		same = append(same, "P")
	}
	c := GroupByZipPswd(many, same, 10)
	if len(c) != 3 || len(c[0].Names) != 10 || len(c[1].Names) != 10 || !reflect.DeepEqual(c[2].Names, []string{"g20", "g21", "g22"}) {
		t.Errorf("%+v", c)
	}

	s := GroupByZipPswd([]string{"x", "y"}, nil, 10)
	if len(s) != 1 || s[0].Pswd != "" || !reflect.DeepEqual(s[0].Names, []string{"x", "y"}) {
		t.Errorf("%+v", s)
	}
}

func TestDeputy(t *testing.T) {
	if DeputyLimit(map[string]string{}) != 5 || DeputyLimit(map[string]string{"max_deputy_date": "7"}) != 7 || DeputyLimit(map[string]string{"max_deputy_date": "x"}) != 5 {
		t.Error("DeputyLimit")
	}
	for _, c := range []struct {
		from, to string
		ok       bool
	}{
		{"2026-10-30", "2026-11-03", true},  // 5일
		{"2026-10-30", "2026-11-04", false}, // 6일
		{"2026-10-30", "2026-10-30", true},  // 1일
		{"2026-10-30", "2026-10-29", false}, // 끝 < 시작
		{"2026-10-30", "", false},
		{"2026-12-30", "2027-01-03", true}, // 해 넘김 5일
	} {
		if err := CheckDeputyPeriod(c.from, c.to, 5); (err == nil) != c.ok {
			t.Errorf("CheckDeputyPeriod(%s,%s) = %v", c.from, c.to, err)
		}
	}
	if err := CheckDeputyPeriod("2026-10-30", "2026-11-04", 5); err.Error() != "대체결재자 유효기간이 설정된 값을 초과하였습니다. [5 일]" {
		t.Error(err)
	}
	if ServerDate("2026-10-30") != "20261030" || ScreenDate("20261030") != "2026-10-30" || ScreenDate("20261030235959") != "2026-10-30" || ScreenDate("") != "" {
		t.Error("날짜 변환")
	}
}

func TestFileDesc(t *testing.T) {
	if FileDesc([]string{"a.txt"}, false) != "a.txt" || FileDesc([]string{"a.txt", "b", "c"}, false) != "a.txt 외 2건" || FileDesc([]string{"x"}, true) != "클립보드" {
		t.Error("FileDesc")
	}
}

func TestFreeName(t *testing.T) {
	d := t.TempDir()
	if FreeName(d, "a.txt") != filepath.Join(d, "a.txt") {
		t.Error("빈 폴더")
	}
	os.WriteFile(filepath.Join(d, "a.txt"), nil, 0o644)
	os.WriteFile(filepath.Join(d, "a (1).txt"), nil, 0o644)
	if got := FreeName(d, "a.txt"); got != filepath.Join(d, "a (2).txt") {
		t.Error(got)
	}
}
