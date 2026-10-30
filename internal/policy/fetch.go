package policy

import (
	"context"
	"unicode/utf8"

	"snbapp/pb/fileextension"
)

// FetchExtInfo 는 확장자 허용 목록[3005]을 받는다(type 2: 결재·결재스킵 모두).
func FetchExtInfo(ctx context.Context, c fileextension.FileExtensionServiceClient) ([]ExtRow, error) {
	res, err := c.GetExtInfo(ctx, &fileextension.ExtInfoRequest{Type: 2})
	if err != nil {
		return nil, err
	}
	rows := make([]ExtRow, 0, len(res.List))
	for _, e := range res.List {
		rows = append(rows, ExtRow{Ext: e.ExtName, AFlag: e.AFlag, Mode: e.Mode, Course: e.Course,
			AllowUpload: e.AllowUpload, AprvUserFlag: e.AprvUserFlag, AprvType: e.AprvType})
	}
	return rows, nil
}

// FetchPatterns 는 헤더 패턴[3015]을 받는다.
func FetchPatterns(ctx context.Context, c fileextension.FileExtensionServiceClient) ([]PattRow, error) {
	res, err := c.GetExtPattern(ctx, &fileextension.ExtPatternRequest{ReqMsg: ""})
	if err != nil {
		return nil, err
	}
	rows := make([]PattRow, 0, len(res.List))
	for _, p := range res.List {
		rows = append(rows, PattRow{Ext: p.ExtName, HFlag: p.HFlag, Patt: p.ExtPatt, Allow: p.AFlag})
	}
	return rows, nil
}

// MismatchBatch 는 불일치 기록[3316] 한 번에 싣는 건수다(원본과 같다). 원본은 앞 5건만 보내고 나머지를 버렸지만 여기서는 5건씩 나눠 다 보낸다.
const MismatchBatch = 5

// maxFnameBytes 는 불일치 기록의 파일 이름 상한이다(서버 칸 fname). 넘으면 UTF-8 글자 경계에서 자른다.
const maxFnameBytes = 500

// MismatchRequests 는 기록 목록을 5건씩 요청으로 나눈다.
func MismatchRequests(userID string, list []Mismatch) []*fileextension.MismatchRegRequest {
	var out []*fileextension.MismatchRegRequest
	for len(list) > 0 {
		n := min(len(list), MismatchBatch)
		req := &fileextension.MismatchRegRequest{MCount: int32(n), UserId: userID}
		for _, m := range list[:n] {
			req.List = append(req.List, &fileextension.FileMismatchInfo{
				ExtName: m.Ext, ExtPatt: m.Patt, Fname: cutUTF8(m.Fname, maxFnameBytes), Desc1: m.Desc})
		}
		out = append(out, req)
		list = list[n:]
	}
	return out
}

func cutUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
