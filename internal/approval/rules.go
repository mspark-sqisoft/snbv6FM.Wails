// Package approval 은 결재 영역이다: 결재방식 판정, 결재자 트리, 결재요청[4127]·결재처리[3390/3092], 대리결재자[3174/3272/3276].
// 원본은 snBv6FM 의 ViewController(결재 부분)·FileMoverGrpc 이고, 이식 명세는 docs/port/06-approval.md 다.
// 이 파일은 서버 없이 시험하는 순수 판정 함수만 둔다(rules_test.go).
package approval

import (
	"errors"
	"fmt"
	"strings"
	"time"

	vpb "snbapp/pb/approver"
)

// 결재 흐름의 공통 오류. 부르는 쪽은 errors.Is 로 가른다.
var (
	ErrCanceled   = errors.New("사용자에 의해 결재요청이 취소되었습니다.")
	ErrNoApprover = errors.New("결재자가 없습니다.")
	ErrBusy       = errors.New("앞 업로드를 마친 뒤 다시 시도하세요.")
)

// PreMode 는 사전결재 모드(fm_upfile_mode 0 ― 검사 통과 직후 결재자를 고른다)인가다. 키가 없으면 0(사전)이다.
// 거짓이면 사후결재 모드(1 ― 결재자 없이 올린 뒤 [결재요청]).
func PreMode(profile map[string]string) bool {
	return strings.TrimSpace(profile["fm_upfile_mode"]) != "1"
}

// NeedsApprover 는 결재자 선택 창이 필요한 결재방식인가다(0 자동·6 자동자가는 창 없이 진행, APR-031).
func NeedsApprover(aprvType int32) bool { return aprvType != 0 && aprvType != 6 }

// SelfApproveAllowed 는 자기를 결재자로 고를 수 있는가다(정책 self_approve == "on", APR-030).
func SelfApproveAllowed(profile map[string]string) bool { return profile["self_approve"] == "on" }

// TypeText 는 결재방식 문구다(원본 ViewController::getAprvTypeText, 결재자 창 콤보와 같은 순서).
func TypeText(t int32) string {
	if t >= 0 && int(t) < len(typeTexts) {
		return typeTexts[t]
	}
	return fmt.Sprintf("Unknown(%d)", t)
}

var typeTexts = []string{"자동 결재", "사전 결재", "확장자결재", "사후 결재", "요청시 사전사후결정", "요청시 결재생략결정", "자동자가결재", "자동,사전,사후"}

// TypeOptions 는 결재자 창의 잠긴 콤보 항목이다(값 0~7).
func TypeOptions() []string { return append([]string(nil), typeTexts...) }

// StateText 는 결재상태 문구다(원본 getAprvStateText). 4 는 서버 스키마 주석에 없지만 사후결재 상신 대기다.
func StateText(s int32) string {
	switch s {
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

// CanRequest 는 사후결재 [결재요청] 으로 상신할 수 있는 행인가다(원본 FileMoverPolicy::getState 의 두 번째 값이 4).
// 백신(1·4 통과) → 개인정보(1·3 통과) → 결재상태 4 순서로 본다.
func CanRequest(virus, priv, aprv int32) bool { return RequestBlock(virus, priv, aprv) == "" }

// RequestBlock 은 상신할 수 없는 까닭(원본 getState 의 글자)이다. 상신할 수 있으면 "".
func RequestBlock(virus, priv, aprv int32) string {
	switch virus {
	case 1, 4:
	case -1:
		return "백신체크 실패"
	case 0:
		return "바이러스 체크 중"
	case 2:
		return "바이러스 발견"
	case 3:
		return "APT 탐지중"
	case 21:
		return "CDR 있음"
	case 22:
		return "암호설정문서파일 차단"
	case 6:
		return "OLE 검출 파일 차단"
	default:
		return "다운로드 불가"
	}
	switch priv {
	case 1, 3:
	case -1:
		return "개인정보 검색 실패"
	case 0:
		return "개인정보 검색 중"
	case 2:
		return "개인정보 검색됨"
	default:
		return "다운로드 불가"
	}
	switch aprv {
	case 4:
		return ""
	case 0:
		return "결재 대기중"
	case 2:
		return "반려됨"
	case 1, 3:
		return "다운로드 가능"
	}
	return "다운로드 불가"
}

// UploadFields 는 업로드 요청(FileTransInfo)의 resv·aprv_kind 다(원본 FileTransfer.cpp:1139-1160).
//   - 클립보드: resv = 사후결재 모드면 "2", 아니면 "1". aprv_kind 없음
//   - 파일: 결재자가 있고 사전결재 모드면 "1", 그 밖 "2". 결재방식 3 이면 aprv_kind 3
func UploadFields(clip, pre bool, aprvType int32, approvers int) (resv string, kind int32) {
	if clip {
		if pre {
			return "1", 0
		}
		return "2", 0
	}
	resv = "2"
	if approvers > 0 && pre {
		resv = "1"
	}
	if aprvType == 3 {
		kind = 3
	}
	return resv, kind
}

// User 는 결재자 트리의 사람 한 명이다.
type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`  // 서버로 보내는 실제 이름(대결 표시 없음)
	Label  string `json:"label"` // 화면 글자: 이름 또는 "이름(대결:대리결재자)"
	DeptID string `json:"deptId"`
	Dept   string `json:"dept"`
	Rank   string `json:"rank"` // 직급
	Duty   string `json:"duty"` // 직위
}

// Dept 는 결재자 트리의 부서 한 마디다.
type Dept struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Users    []User  `json:"users"`
	Children []*Dept `json:"children"`
}

func toUser(a *vpb.ApproverInfo) User {
	label := a.UserName
	if a.Rname != "" {
		label += "(대결:" + a.Rname + ")"
	}
	return User{ID: a.UserId, Name: a.UserName, Label: label, DeptID: a.UserDeptId, Dept: a.UserDeptName, Rank: a.UserRank, Duty: a.UserDutyName}
}

// BuildTree 는 결재자 목록[3370]으로 트리를 만든다(원본 makeAprvTree, APR-020·026).
//   - aprv_state == 1 인 사람만, 서버 순서대로. 같은 ID 는 처음 것만
//   - 부서 ID 는 "|" 로 나눈 계층. 부모 = 마지막 "|" 앞부분. 부모를 못 찾으면 뿌리에 단다(원본은 빈 포인터로 죽을 수 있다)
//
// 결재자가 0명이면 nil.
func BuildTree(list []*vpb.ApproverInfo) []*Dept {
	var roots []*Dept
	byID := map[string]*Dept{}
	seen := map[string]bool{}
	for _, a := range list {
		if a.AprvState != 1 || a.UserId == "" || seen[a.UserId] {
			continue
		}
		seen[a.UserId] = true
		d := byID[a.UserDeptId]
		if d == nil {
			d = &Dept{ID: a.UserDeptId, Name: a.UserDeptName}
			byID[a.UserDeptId] = d
			parent := (*Dept)(nil)
			if i := strings.LastIndex(a.UserDeptId, "|"); i >= 0 {
				parent = byID[a.UserDeptId[:i]]
			}
			if parent != nil {
				parent.Children = append(parent.Children, d)
			} else {
				roots = append(roots, d)
			}
		}
		d.Users = append(d.Users, toUser(a))
	}
	return roots
}

// TreeUsers 는 트리를 앞에서부터(부서 안의 사람 → 하위 부서) 훑은 사람 목록이다 ― 고른 결재자의 순서가 이것이다.
func TreeUsers(roots []*Dept) []User {
	var out []User
	var walk func([]*Dept)
	walk = func(ds []*Dept) {
		for _, d := range ds {
			out = append(out, d.Users...)
			walk(d.Children)
		}
	}
	walk(roots)
	return out
}

// Candidates 는 [대리결재자] 후보다: 목록 전체에서 나만 뺀다(aprv_state 로 거르지 않는다, 원본 ViewControllerUiSlot.cpp:519).
func Candidates(list []*vpb.ApproverInfo, myID string) []User {
	var out []User
	seen := map[string]bool{}
	for _, a := range list {
		if a.UserId == "" || a.UserId == myID || seen[a.UserId] {
			continue
		}
		seen[a.UserId] = true
		out = append(out, toUser(a))
	}
	return out
}

// ZipGroup 은 결재요청[4127] 하나에 실을 묶음이다.
type ZipGroup struct {
	Pswd  string
	Names []string
}

// GroupByZipPswd 는 (서버 파일명, 압축 비밀번호) 를 같은 비밀번호끼리, 그 안에서 chunk 개씩 묶는다(원본 ZipPswdGroups.h).
// 비밀번호가 처음 나온 순서, 묶음 안은 들어온 순서. pswds 가 짧으면 모자란 자리는 빈 값이다(빈 값도 하나의 열쇠).
func GroupByZipPswd(names, pswds []string, chunk int) []ZipGroup {
	if chunk <= 0 {
		chunk = 10
	}
	var order []string
	by := map[string][]string{}
	for i, n := range names {
		pw := ""
		if i < len(pswds) {
			pw = pswds[i]
		}
		if _, ok := by[pw]; !ok {
			order = append(order, pw)
		}
		by[pw] = append(by[pw], n)
	}
	var out []ZipGroup
	for _, pw := range order {
		all := by[pw]
		for j := 0; j < len(all); j += chunk {
			out = append(out, ZipGroup{Pswd: pw, Names: all[j:min(j+chunk, len(all))]})
		}
	}
	return out
}

// DeputyLimit 은 대리 기간 상한(일)이다. 정책 max_deputy_date, 없거나 숫자가 아니면 5.
func DeputyLimit(profile map[string]string) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(profile["max_deputy_date"]), "%d", &n); err != nil || n <= 0 {
		return 5
	}
	return n
}

// CheckDeputyPeriod 는 대리 기간("YYYY-MM-DD")을 판정한다. 일수 = (끝 - 시작) + 1, 일수 > 상한이면 막는다(원본 saveProxyApprover).
// 원본은 끝 < 시작 과 형식 오류를 막지 않았다 ― 여기서는 막는다(docs/port/06 6절).
func CheckDeputyPeriod(from, to string, limit int) error {
	f, err1 := time.Parse(time.DateOnly, from)
	t, err2 := time.Parse(time.DateOnly, to)
	if err1 != nil || err2 != nil {
		return errors.New("대리 기간을 확인해 주세요.")
	}
	if t.Before(f) {
		return errors.New("종료일이 시작일보다 앞섭니다.")
	}
	if days := int(t.Sub(f).Hours()/24) + 1; days > limit {
		return fmt.Errorf("대체결재자 유효기간이 설정된 값을 초과하였습니다. [%d 일]", limit)
	}
	return nil
}

// ServerDate 는 "YYYY-MM-DD" → "YYYYMMDD" 다. ScreenDate 는 그 반대("YYYYMMDD…" 앞 8자리, 형식이 틀리면 그대로).
func ServerDate(d string) string { return strings.ReplaceAll(d, "-", "") }

func ScreenDate(d string) string {
	if len(d) < 8 {
		return d
	}
	if t, err := time.Parse("20060102", d[:8]); err == nil {
		return t.Format(time.DateOnly)
	}
	return d
}

// FileDesc 는 결재자 창의 파일정보 글자다: 클립보드면 "클립보드", 1건 = 이름, 여럿 = "%1 외 %2건"(APR-022).
func FileDesc(names []string, clip bool) string {
	switch {
	case clip:
		return "클립보드"
	case len(names) == 0:
		return ""
	case len(names) == 1:
		return names[0]
	}
	return fmt.Sprintf("%s 외 %d건", names[0], len(names)-1)
}

// PickFile 은 결재자 창의 파일 표 한 줄이다.
type PickFile struct {
	Name string `json:"name"` // 화면에 보일 이름(클립보드면 무시하고 "클립보드")
	Size int64  `json:"size"`
}

// PickRequest 는 결재자를 정해 달라는 요청이다(svc.PickApprover).
//   - Clip: 모든 항목이 클립보드면 참(클립보드 결재방식 fm_*clup_aprv 를 쓴다)
//   - After: 이미 올린 파일의 사후결재 [결재요청](4127)이면 참 ― 사후결재 모드여도 창을 띄운다.
//     거짓(업로드 직전)이면 사후결재 모드에서는 창 없이 결재자 없는 값을 준다.
type PickRequest struct {
	Files []PickFile
	Clip  bool
	After bool
}

// PickResult 는 업로드 요청(FileTransInfo)·결재요청[4127] 에 실을 결재 칸이다.
// 업로드: approver_id = ApproverIDs, fcause = Cause, aprv_type = AprvType, aprv_dept = AprvDept, resv = Resv, aprv_kind = AprvKind.
// pkg_no·zip_pswd 는 전송 쪽이 채운다.
type PickResult struct {
	ApproverIDs   []string
	ApproverNames []string // 4127 approver[] ― 대결 표시 없는 실제 이름(ID 와 같은 자리)
	Cause         string
	AprvType      int32
	AprvDept      string // 내 부서명(로그인 응답 user_dept_name)
	Resv          string
	AprvKind      int32
	Dialog        bool // 결재자 창을 거쳤는가(창을 띄우지 않은 결재방식 0·6, 사후결재 모드 업로드는 거짓)
}

// IsClipName 은 클립보드로 올린 서버 파일(이름에 __CLIP__)인가다.
func IsClipName(name string) bool { return strings.Contains(name, "__CLIP__") }
