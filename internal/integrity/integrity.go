// Package integrity 는 프로그램 무결성 검사[3510/3511]다(docs/port/01 3.6·7.7, 원본 IntegrityVerdict.cpp).
// 검사 대상의 SHA-256(소문자 16진 64자)을 서버에 보내고, 서버가 등록된 값과 대조한 결과를 판정한다.
// 시점(로그인 직후·3시간 주기·관리자 요청 9005)과 실패 때 할 일(종료)은 부르는 쪽(svc)이 정한다.
package integrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"snbapp/internal/config"
	"snbapp/pb/commonservice"
)

// 검사유형(IntegrityRequest.chk_div).
const (
	KindStart    = 0 // 로그인 직후
	KindPeriodic = 1 // 3시간 주기
	KindAdmin    = 2 // 관리자 요청(알림 9005)
)

const (
	typeDiv = "FM"
	// version 은 앱 버전이 아니라 서버 ssbr_version.ini 의 배포 폴더 이름(6.0.0.1/WIN)과 같아야 한다.
	version   = "6.0.0.1"
	notHashed = "not found file"
	envFile   = "fmenv.ini"
)

// Files 는 검사 대상의 PC 경로다. Wails 배포본은 실행 파일 하나에 화면이 들어가므로 실행 파일과 fmenv.ini 둘이다(7.7 추정).
// ponytail: 배포본 구성이 정해지면(15) 여기 목록만 늘린다.
func Files(appDir string) []string {
	exe, err := os.Executable()
	if err != nil {
		exe = filepath.Join(appDir, "snbApp.exe")
	}
	return []string{exe, filepath.Join(appDir, envFile)}
}

// WireName 은 서버에 보낼 이름이다. fmenv.ini 만 망마다 내용이 달라 fmenv.ini.int(LOC 1)·fmenv.ini.ext(그 밖)로 보낸다
// ― 서버는 이름 하나로 첫 행만 대조해 망을 가리지 않는다.
func WireName(file string, loc int) string {
	if file != envFile {
		return file
	}
	if loc == 1 {
		return file + ".int"
	}
	return file + ".ext"
}

// Failures 는 판정이다. 실패 사유 줄(「파일 : 사유」)을 모두 돌려주고, 비면 통과다.
//  1. 서버가 result != 1 로 답한 항목
//  2. 보냈는데 응답에 없는 이름(대소문자 무시)
//  3. 해시를 못 낸 항목(서버가 통과라 해도 ― DB 에 "not found file" 로 등록돼 있으면 일치로 답한다)
func Failures(sent []string, answers []*commonservice.FileChecksumResult, notHashedFiles []string) []string {
	var out []string
	for _, a := range answers {
		if a.GetResult() != 1 {
			out = append(out, a.GetFname()+" : "+a.GetDetail())
		}
	}
	for _, f := range sent {
		found := false
		for _, a := range answers {
			if strings.EqualFold(a.GetFname(), f) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, f+" : 서버 응답에 없어 대조하지 못했습니다")
		}
	}
	for _, f := range notHashedFiles {
		out = append(out, f+" : 파일이 없거나 열 수 없어 해시를 내지 못했습니다")
	}
	return out
}

// SkipUnanswered 는 응답을 받지 못한 검사를 판정하지 않고 넘길지다. 로그인 직후 검사는 넘기지 않는다.
// 주기·관리자 요청 검사는 통신 실패(Unavailable·Canceled·DeadlineExceeded)면 넘긴다 ― 일시적 끊김으로 앱을 끄지 않는다.
// 답이 왔는데 못 읽은 경우(그 밖의 상태)는 판정으로 보내 실패로 만든다.
func SkipUnanswered(kind int, code codes.Code) bool {
	return kind != KindStart && (code == codes.Unavailable || code == codes.Canceled || code == codes.DeadlineExceeded)
}

// hashFile 은 SHA-256 소문자 16진 문자열과 크기다.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path) // Go 는 공유 읽기·쓰기·삭제로 열어 실행 중인 자기 exe 도 읽힌다
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Request 는 보낼 요청과, 판정에 쓸 보낸 이름·해시 못 낸 이름이다.
func Request(paths []string, kind, loc int) (req *commonservice.IntegrityRequest, sent, missing []string) {
	req = &commonservice.IntegrityRequest{ChkDiv: int32(kind)}
	for _, p := range paths {
		name := WireName(filepath.Base(p), loc)
		item := &commonservice.FileChecksumInfo{TypeDiv: typeDiv, Version: version, Fname: name, Location: int32(loc)}
		if sum, size, err := hashFile(p); err == nil {
			item.Fsize, item.Hash = uint64(size), []byte(sum) // 16진 문자열 그대로(원시 32바이트가 아니다)
		} else {
			item.Hash = []byte(notHashed)
			missing = append(missing, name)
		}
		req.List = append(req.List, item)
		sent = append(sent, name)
	}
	req.TotCnt = int32(len(req.List))
	return req, sent, missing
}

// Check 는 검사 한 번이다. 실패 사유가 있으면 failures 에 담는다. skipped 면 판정하지 않았다(Windows 가 아님·통신 실패).
// ctx 에는 auth-token 과 요청 상한을 실어 넘긴다.
func Check(ctx context.Context, cl commonservice.CommonServiceClient, paths []string, kind, loc int, log *slog.Logger) (failures []string, skipped bool) {
	if runtime.GOOS != "windows" {
		log.Info("무결성검사 무시 : 서버에 Windows 배포본 해시만 등록돼 있음")
		return nil, true
	}
	req, sent, missing := Request(paths, kind, loc)
	log.Info("무결성검사 요청[3510] 검사유형: " + strconv.Itoa(kind) + " 건수: " + strconv.Itoa(len(sent)))
	res, err := cl.CheckIntegrity(ctx, req)
	if err != nil {
		code := status.Code(err)
		if SkipUnanswered(kind, code) {
			log.Warn("무결성검사[3510] 응답 없음 ― 서버에 닿지 못해 이번 검사는 판정하지 않는다(검사유형:" + strconv.Itoa(kind) + ")")
			return nil, true
		}
		log.Warn("무결성검사[3510] 실패", "code", code.String(), "err", status.Convert(err).Message())
		failures = Failures(sent, nil, missing)
	} else {
		failures = Failures(sent, res.GetList(), missing)
	}
	for _, f := range failures {
		log.Warn("[3511] 무결성 오류 - " + f)
	}
	if len(failures) == 0 {
		log.Info("무결성검사 응답[3511] => <무결성 오류없음> 검증건수: " + strconv.Itoa(len(sent)))
	}
	return failures, false
}

// Enforced 는 무결성 실패 때 앱을 끝낼지다. SNB_INTEGRITY_ENFORCE=1 이면 늘 켠다.
// 개발 빌드(config.IsDev)만 끈다 ― 다른 작업트리 빌드가 서버 해시를 덮어 내 빌드가 실패하기 때문이다.
func Enforced() bool {
	if os.Getenv("SNB_INTEGRITY_ENFORCE") == "1" {
		return true
	}
	return !config.IsDev()
}

// Period 는 주기 검사 간격이다. 3시간, 개발 빌드만 SNB_INTEGRITY_PERIOD_SEC(1~86400초)로 바꾼다(SEC-003).
func Period() time.Duration {
	if config.IsDev() {
		if n, err := strconv.Atoi(os.Getenv("SNB_INTEGRITY_PERIOD_SEC")); err == nil && n >= 1 && n <= 86400 {
			return time.Duration(n) * time.Second
		}
	}
	return 3 * time.Hour
}
