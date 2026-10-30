// Package files 는 서버파일 목록·상태 판정·받기 막기 사유·삭제·검출 조회다(파일·내역 영역, docs/port/04).
// 받기 자체는 전송 영역(internal/transfer)이 하고, 그쪽이 쓸 순수 함수(저장 경로·이름 겹침·번호·디스크 여유)는 paths.go 에 있다.
package files

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/status"

	"snbapp/internal/config"
	"snbapp/internal/nbs"
	"snbapp/pb/serverfile"
)

// PageSize 는 목록 한 번에 받을 건수다(원본 from_cnt 10000). 받은 건수가 이만큼이면 다음 쪽을 이어 받는다.
const PageSize = 10000

// maxPages 는 서버가 같은 쪽을 끝없이 주는 일을 막는 상한이다(100만 건).
const maxPages = 100

// List 는 서버 파일 목록 전체다(받을 파일·올린 파일 모두, 최근 것이 위). 10000건씩 이어 받는다.
// 원본은 from_seq 에 응답 건수를 대입해 2만 건이 넘으면 같은 쪽을 다시 받았다(FileTransfer.cpp:466) ― 여기는 누적한다.
func List(s *nbs.Session) ([]*serverfile.FileInfo, error) {
	return listPages(func(from int32) ([]*serverfile.FileInfo, error) {
		ctx, cancel := s.Ctx(nbs.RPCTimeout)
		defer cancel()
		res, err := s.ServerFile().GetServerFileList(ctx, &serverfile.ServerFileListRequest{
			Userid: s.UserID, FromSeq: from, FromCnt: PageSize, Sort: 0,
		})
		if err != nil {
			return nil, fmt.Errorf("서버 파일 목록을 받지 못했습니다: %s", status.Convert(err).Message())
		}
		return res.List, nil
	})
}

func listPages(page func(from int32) ([]*serverfile.FileInfo, error)) ([]*serverfile.FileInfo, error) {
	var all []*serverfile.FileInfo
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

// Scanning 은 검사 결과를 기다리는 행이 있는가다(바이러스 0·APT 3·개인정보 0). 서버는 검사가 끝나도 알리지 않아
// 이런 행이 있으면 스스로 다시 조회한다(원본 schedulePendingScanPoll).
func Scanning(list []*serverfile.FileInfo) bool {
	for _, f := range list {
		if f.VirusFlag == 0 || f.VirusFlag == 3 || f.PrivState == 0 {
			return true
		}
	}
	return false
}

// RecheckDelay 는 n 번째(0부터) 재조회까지 기다릴 초다: 1, 2, 4, 그 뒤 8. MaxRechecks 번을 넘으면 그만 묻는다(약 5분).
func RecheckDelay(n int) int {
	switch n {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 4
	}
	return 8
}

// MaxRechecks 는 검사 중 항목 재조회 상한이다(1+2+4+8x37 = 303초).
const MaxRechecks = 40

// DisplayName 은 표의 [파일명]이다: name_str 의 마지막 경로 조각, 폴더(dir_flag 1)면 끝의 .zip 을 뗀다.
// 원본은 이름 중간의 .zip 까지 지웠다(ViewControllerUiSlot.cpp:72) ― 끝의 것만, 대소문자 없이.
func DisplayName(f *serverfile.FileInfo) string {
	n := f.NameStr
	if i := strings.LastIndexAny(n, `/\`); i >= 0 {
		n = n[i+1:]
	}
	if f.DirFlag == 1 && len(n) > 4 && strings.EqualFold(n[len(n)-4:], ".zip") {
		n = n[:len(n)-4]
	}
	return n
}

// IsClip 은 클립보드로 올린 행인가다(이름에 __CLIP__).
func IsClip(f *serverfile.FileInfo) bool { return strings.Contains(f.NameStr, "__CLIP__") }

// AprvCause 는 결재 사유다: 앞뒤 공백을 지우고 "-" 면 빈 값.
func AprvCause(f *serverfile.FileInfo) string {
	c := strings.TrimSpace(f.AprvCause)
	if c == "-" {
		return ""
	}
	return c
}

// 상태 칩 색(Kind). 화면이 Tone 으로 바꾼다.
const (
	KindDanger  = "danger"  // 검출·실패·반려
	KindPending = "pending" // 검사 중·결재 대기
	KindSuccess = "success" // 다운로드 가능
	KindWarning = "warning" // 동일망 다운로드 불가
	KindAccent  = "accent"  // 결재요청 가능
	KindMuted   = "muted"   // 그 밖
)

// Status 는 상태 칸 하나다. Code: 1 다운로드 가능, 0 결재 대기, 2 반려, 4 결재요청 가능, -1 받을 수 없음.
type Status struct {
	Text string
	Code int
	Kind string
}

// GetState 는 원본 FileMoverPolicy::getState 다(바이러스 → 개인정보 → 결재, 앞에서 걸리면 뒤는 안 본다). 모르는 값은 막는다.
func GetState(aprvState, privState, virusFlag int32) Status {
	bad := func(t string, k string) Status { return Status{t, -1, k} }
	switch virusFlag {
	case -1:
		return bad("백신체크 실패", KindDanger)
	case 0:
		return bad("바이러스 체크 중", KindPending)
	case 2:
		return bad("바이러스 발견", KindDanger)
	case 3:
		return bad("APT 탐지중", KindPending)
	case 5:
		return bad("다운로드 불가", KindDanger)
	case 6: // 원본 색 규칙에서 빠져 흐림이었다 ― 위험으로
		return bad("OLE 검출 파일 차단", KindDanger)
	case 21:
		return bad("CDR 있음", KindDanger)
	case 22:
		return bad("암호설정문서파일 차단", KindDanger)
	case 1, 4:
	default:
		return bad("다운로드 불가", KindMuted)
	}
	switch privState {
	case -1:
		return bad("개인정보 검색 실패", KindDanger)
	case 0:
		return bad("개인정보 검색 중", KindPending)
	case 2:
		return bad("개인정보 검색됨", KindDanger)
	case 1, 3:
	default:
		return bad("다운로드 불가", KindMuted)
	}
	switch aprvState {
	case 0:
		return Status{"결재 대기중", 0, KindPending}
	case 2:
		return Status{"반려됨", 2, KindDanger}
	case 4:
		return Status{"결재요청 가능", 4, KindAccent}
	case 1, 3: // 3 사후승인도 받는다
		return Status{"다운로드 가능", 1, KindSuccess}
	}
	return bad("다운로드 불가", KindMuted)
}

// SameNetworkAllowed 는 정책 down_same_network 가 0 이 아닌가다. 키가 없거나 숫자가 아니면 원본 toInt() 처럼 0 ― 막는다.
func SameNetworkAllowed(profile map[string]string) bool {
	n, _ := strconv.Atoi(strings.TrimSpace(profile["down_same_network"]))
	return n != 0
}

// FileStatus 는 행의 상태 칸이다. 다운로드 가능이어도 지금 망에서 올렸고 동일망 받기를 막는 정책이면 「동일망 다운로드 불가」(-1).
// loc 는 로그인한 망의 LOC(1 업무망, 0 인터넷망).
func FileStatus(f *serverfile.FileInfo, loc int, profile map[string]string) Status {
	st := GetState(f.State, f.PrivState, f.VirusFlag)
	if st.Code == 1 && int(f.Course) == loc && !SameNetworkAllowed(profile) {
		return Status{"동일망 다운로드 불가", -1, KindWarning}
	}
	return st
}

// Block 은 [다운로드] 에서 이 파일을 빼는 사유다. 값의 순서가 안내 창에 사유가 나오는 순서다(원본 DownloadBlock.h).
type Block int

const (
	BlockNone         Block = iota // 받을 수 있다
	BlockClipboard                 // 클립보드로 올린 것
	BlockSameNetwork               // 정책이 막는데 지금 망에서 올린 것
	BlockVirusPending              // 바이러스 검사가 끝나지 않았다(0·3)
	BlockNotReady                  // 받을 수 있는 상태가 아니다(결재 전·반려·검출·검사 오류)
)

// Text 는 사유 문구다(원본 popDownloadBlocked 의 한 건 문구, 앞 공백 없이). 화면은 「<이름> <문구>」로 보인다.
func (b Block) Text() string {
	switch b {
	case BlockClipboard:
		return "클립보드는 파일형태로 다운로드할 수 없습니다."
	case BlockSameNetwork:
		return "동일망에선 다운로드할 수 없습니다."
	case BlockVirusPending:
		return "파일은 바이러스 검사가 끝나지 않아 다운로드할 수 없습니다."
	case BlockNotReady:
		return "파일은 다운로드 가능한 상태가 아닙니다."
	}
	return ""
}

// Classify 는 원본 classifyDownload 다: 클립보드 → 동일망 → 바이러스 미완료 → 상태(GetState 코드가 1 이 아니면).
// 결재 상태는 원시 state 가 아니라 GetState 의 코드로 본다 ― 사후승인(3)은 받고, 개인정보 검출은 막는다.
// 서버 FileDownload 는 결재 상태를 보지 않고 경로만으로 내주므로 여기서 막아야 한다.
func Classify(f *serverfile.FileInfo, loc int, profile map[string]string) Block {
	switch {
	case IsClip(f):
		return BlockClipboard
	case !SameNetworkAllowed(profile) && int(f.Course) == loc:
		return BlockSameNetwork
	case f.VirusFlag == 0 || f.VirusFlag == 3:
		return BlockVirusPending
	case GetState(f.State, f.PrivState, f.VirusFlag).Code != 1:
		return BlockNotReady
	}
	return BlockNone
}

// DownloadBlock 은 받을 수 없으면 그 사유 문구, 받을 수 있으면 빈 값이다(전송 영역이 받기 직전에 부른다).
func DownloadBlock(f *serverfile.FileInfo, loc int, profile map[string]string) string {
	return Classify(f, loc, profile).Text()
}

// Remove 는 서버 파일 하나를 지운다(RemoveFile[3036]). afterDownload 면 flag 1(받은 뒤 삭제), 아니면 0(사용자가 [삭제]).
// 서버는 요청자 소유 행만 바꾼다(svc_removefile.go 의 file_user 조건).
func Remove(s *nbs.Session, serverStr string, afterDownload bool) error {
	flag := int32(0)
	if afterDownload {
		flag = 1
	}
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	if _, err := s.ServerFile().RemoveFile(ctx, &serverfile.FileRemoveRequest{Spath: serverStr, Flag: flag}); err != nil {
		return fmt.Errorf("서버 파일을 지우지 못했습니다: %s", status.Convert(err).Message())
	}
	return nil
}

// 검출 조회 종류(GetDetectionDetail type). 0 개인정보는 서버에 묻지 않고 목록의 priv_detail 을 다듬는다.
const (
	DetectPrivacy = 0
	DetectAPT     = 1
	DetectVirus   = 2
)

// NoDetection 은 검출 내용이 없을 때의 본문이다.
const NoDetection = "검출된 내용이 없습니다."

// DetectionDetail 은 바이러스·APT 검출 상세(GetDetectionDetail[3096])다. 빈 값·"-" 면 NoDetection.
// 원본은 요청 실패도 「없음」으로 보였다 ― 여기는 오류로 돌려 「못 봤다」와 가른다.
func DetectionDetail(s *nbs.Session, serverStr string, kind int32) (string, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.ServerFile().GetDetectionDetail(ctx, &serverfile.DetectionDetailRequest{Spath: serverStr, Type: kind})
	if err != nil {
		return "", fmt.Errorf("검출 상세를 받지 못했습니다: %s", status.Convert(err).Message())
	}
	if d := strings.TrimSpace(res.Detail); d == "" || d == "-" {
		return NoDetection, nil
	}
	return res.Detail, nil
}

// PrivDetailText 는 개인정보 검출 상세를 다듬는다(원본 ViewController.cpp:1680-1707):
// 비었거나 서버 파일 이름과 같으면 없음, "[SI]"·"case detected" 를 지우고, 괄호 안이 비었거나 "0" 인 줄을 버린다.
// 남은 것이 없으면 다듬기 전 글자를 그대로 보인다.
func PrivDetailText(detail, serverStr string) string {
	if detail == "" || detail == serverStr {
		return NoDetection
	}
	msg := strings.ReplaceAll(strings.ReplaceAll(detail, "[SI]", ""), "case detected", "")
	var keep []string
	for _, line := range strings.Split(msg, "\n") {
		a, b := strings.Index(line, "("), strings.Index(line, ")")
		if b-a == 1 || (b-a == 2 && line[a+1] == '0') { // 원본 indexOf 그대로(없으면 -1)
			continue
		}
		keep = append(keep, line)
	}
	if out := strings.TrimSpace(strings.Join(keep, "\n")); out != "" {
		return out
	}
	return msg
}

// CourseName 은 [전송방향] 글자다: fmenv.ini 서버 중 LOC 가 course 와 같은 첫 서버의 NAME, 없으면 Unknown(원본 ViewController.cpp:430).
func CourseName(servers []config.Server, course int32) string {
	for _, s := range servers {
		if s.Loc == int(course) {
			return s.Name
		}
	}
	return "Unknown"
}

// SizeText 는 원본 getFileSizeToText 다(경계는 초과, 단위 글자 대소문자까지 원본 그대로).
func SizeText(n uint64) string {
	const k, m, g = 1024, 1024 * 1024, 1024 * 1024 * 1024
	switch {
	case n < 1:
		return "0 Byte"
	case n > g:
		return fmt.Sprintf("%.2f GB", float64(n)/g)
	case n > m:
		return fmt.Sprintf("%.2f MB", float64(n)/m)
	case n > k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	}
	return fmt.Sprintf("%d byte", n)
}
