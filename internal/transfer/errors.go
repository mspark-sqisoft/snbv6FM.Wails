// Package transfer 는 전송 대기열·업로드·다운로드·폴더 압축·안전 풀기·자동 전송의 순수 부분이다(docs/port/03·04).
// 세션·설정·화면 사건은 모른다 ― 그것은 internal/svc/transfer.go 가 잇는다.
package transfer

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 원본 오류 코드(FileMoverGrpc.cpp getTransFileErrorString, docs/port/03 3.4·3.5, docs/port/04 3.7).
const (
	CodeUpBizNoPerm    = -21 // 업무망 업로드 권한 없음(서버)
	CodeUpBizSize      = -22 // 업무망 크기 초과(서버)
	CodeUpNetNoPerm    = -23
	CodeUpNetSize      = -24
	CodeDnBizNoPerm    = -31
	CodeDnBizSize      = -32
	CodeDnNetNoPerm    = -33
	CodeDnNetSize      = -34
	CodeChecksum       = -45  // 받은 파일 검사값 불일치
	CodeOpen           = -123 // 파일 열기 실패
	CodeRead           = -125 // 보낼 파일을 읽지 못함
	CodeFailed         = -970 // 그 밖 전송 실패
	CodeStreamOpen     = -972 // 업로드 스트림을 열지 못함
	CodeBadAnswer      = -973
	CodeBroken         = -974 // 다 보내기 전에 연결이 끊김
	CodeStall          = -975 // 정지 감시
	CodeNoData         = -976 // 받기: 서버 데이터 없음
	CodeSizeMismatch   = -977 // 받기: 크기 불일치
	CodeChanged        = -978 // 검사 뒤 파일이 바뀜
	CodeCanceled       = -980 // 사용자 취소
	CodeZip            = -990 // 폴더 압축 실패(원본 로그 문구만 있고 코드 없음 ― Wails 판에서 붙였다)
	CodeUnzip          = -991 // FOLDER_UNZIP_FAILED
	CodeUnzipUnsafe    = -992
	CodeUnzipSize      = -993
	CodeUnzipItems     = -994
	CodeUnzipDisk      = -995
	CodeStorage        = -9999 // 저장소 용량 부족
	CodeLoggedOut      = -9998 // 로그아웃으로 정리(Wails 판)
	CodeFolderChanged  = -9978 // 검사 뒤 폴더가 바뀜(창 문구가 달라 따로 둔다)
	CodeDuplicateQueue = -9997 // 같은 파일이 이미 대기·진행 중(Wails 판)
)

// Fail 은 전송 실패 하나다. Arg 는 문구의 %1(크기 상한 등), Arg2 는 %2.
type Fail struct {
	Code      int
	Arg, Arg2 string
	Cause     error // 로그용 원인(화면에 내지 않는다)
}

func (f *Fail) Error() string { return Text(f.Code, f.Arg, f.Arg2) }
func (f *Fail) Unwrap() error { return f.Cause }

func fail(code int, cause error, args ...string) *Fail {
	f := &Fail{Code: code, Cause: cause}
	if len(args) > 0 {
		f.Arg = args[0]
	}
	if len(args) > 1 {
		f.Arg2 = args[1]
	}
	return f
}

// CodeOf 는 err 의 원본 코드다. 취소면 CodeCanceled, 모르는 오류는 CodeFailed.
func CodeOf(err error) int {
	var f *Fail
	switch {
	case err == nil:
		return 0
	case errors.As(err, &f):
		return f.Code
	case errors.Is(err, ErrCanceled):
		return CodeCanceled
	}
	return CodeFailed
}

// Text 는 원본 문구다(tr 원문 그대로). arg 는 %1, arg2 는 %2.
func Text(code int, arg, arg2 string) string {
	r := strings.NewReplacer("%1", arg, "%2", arg2)
	if s, ok := texts[code]; ok {
		return r.Replace(s)
	}
	if code >= -6 && code <= -1 {
		return extTexts[-code-1]
	}
	return fmt.Sprintf("알 수 없는 오류 [%d]", code)
}

var extTexts = [...]string{
	"등록된 확장자 패턴이 없습니다.",
	"확장자가 없는 파일은 전송할 수 없습니다.",
	"등록되지 않은 확장자입니다..",
	"허가된 확장자가 아닙니다.",
	"확장자가 변경된 파일은 전송할 수 없습니다.",
	"새로운 패턴의 확장자입니다.\n 동일현상 발생 시, 관리자에게 문의하세요.",
}

var texts = map[int]string{
	CodeUpBizNoPerm:    "업무망 파일 업로드 권한이 없습니다.",
	CodeUpBizSize:      "파일사이즈가 전송가능사이즈를(%1) 초과하였습니다.",
	CodeUpNetNoPerm:    "인터넷망 파일 업로드 권한이 없습니다.",
	CodeUpNetSize:      "파일사이즈가 전송가능사이즈를(%1) 초과하였습니다.",
	CodeDnBizNoPerm:    "업무망 파일 다운로드 권한이 없습니다.",
	CodeDnBizSize:      "파일사이즈가 전송사이즈(%1)를 초과하였습니다.",
	CodeDnNetNoPerm:    "인터넷망 파일 다운로드 권한이 없습니다.",
	CodeDnNetSize:      "파일사이즈가 전송사이즈(%1)를 초과하였습니다.",
	CodeChecksum:       "파일이 변조되었거나 손상되었습니다. Original(%1), Download_file(%2)",
	CodeOpen:           "파일 열기 실패",
	CodeRead:           "보낼 파일을 읽지 못했습니다.",
	CodeFailed:         "전송 실패",
	-971:               "재전송 횟수 초과",
	CodeStreamOpen:     "서버와 전송 연결을 열지 못했습니다.",
	CodeBadAnswer:      "서버 응답을 해석하지 못했습니다.",
	CodeBroken:         "파일을 다 보내기 전에 연결이 끊겼습니다.",
	CodeStall:          "서버 응답이 없어 전송을 중단했습니다.",
	CodeNoData:         "서버에서 파일 데이터를 받지 못했습니다.",
	CodeSizeMismatch:   "받은 크기(%1 바이트)가 서버 파일 크기(%2 바이트)와 달라 저장하지 않았습니다.",
	CodeChanged:        "검사한 뒤 파일이 바뀌어 올리지 않았습니다. 다시 올려 주세요.",
	CodeCanceled:       "전송 취소",
	CodeZip:            "폴더를 압축하는데 오류가 발생했습니다.",
	CodeUnzip:          "폴더 압축해제 실패",
	CodeUnzipUnsafe:    "받은 폴더를 풀지 못했습니다.\n압축파일 안에 허용되지 않는 경로가 있습니다.",
	CodeUnzipSize:      "받은 폴더를 풀지 않았습니다.\n풀린 크기가 상한(%1)을 넘습니다.",
	CodeUnzipItems:     "받은 폴더를 풀지 않았습니다.\n파일·폴더 수가 상한(%1개)을 넘습니다.",
	CodeUnzipDisk:      "받은 폴더를 풀지 못했습니다.\n풀 디스크의 남은 공간이 부족합니다.",
	CodeStorage:        "저장소의 용량이 부족합니다.",
	CodeLoggedOut:      "로그아웃되어 전송을 멈췄습니다.",
	CodeFolderChanged:  "검사한 뒤 폴더 '%1' 의 내용이 바뀌어 올리지 않았습니다. 다시 올려 주세요.",
	CodeDuplicateQueue: "같은 파일이 이미 전송 대기열에 있습니다.",
	-950:               "데이타 암호화 실패",
	-951:               "데이타 복호화 실패",
}

// ErrCanceled 는 사용자가 취소한 전송이다(화면 행 「취소됨」, 실패 창 없음).
var ErrCanceled = errors.New("전송 취소")

// downloadStatusFail 은 받기 스트림의 gRPC 오류를 원본 코드로 바꾼다. 지금 Go 서버는 받기 거절을 answer 가 아니라
// gRPC 오류로 보낸다(서버 svc_filedownload.go:88-110, docs/port/04 3.3). biz 는 업무망 로그인인가.
func downloadStatusFail(err error, biz bool, limit string) *Fail {
	st := status.Convert(err)
	msg := st.Message()
	switch {
	case strings.Contains(msg, "invalid Downloadfile name format"):
		return fail(CodeNoData, err)
	case strings.Contains(msg, "not allowed to download file to Inner PC"):
		return fail(CodeDnBizNoPerm, err)
	case strings.Contains(msg, "not allowed to download file to Internet PC"):
		return fail(CodeDnNetNoPerm, err)
	case strings.Contains(msg, "file size exceeds limit"):
		if biz {
			return fail(CodeDnBizSize, err, limit)
		}
		return fail(CodeDnNetSize, err, limit)
	case st.Code() == codes.Unavailable:
		return fail(CodeBroken, err)
	}
	return fail(CodeFailed, err)
}
