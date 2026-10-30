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
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"snbapp/internal/config"
	"snbapp/pb/commonservice"
)

// 원본 tests/tst_IntegrityVerdict.cpp 를 옮겼다.
var sent = []string{"FileMoverApp.exe", "FileMoverGrpc.dll", "lang_en.qm"}

func ok(n string) *commonservice.FileChecksumResult {
	return &commonservice.FileChecksumResult{Fname: n, Result: 1, Detail: "success checksum !!!"}
}

func allOk() []*commonservice.FileChecksumResult {
	var a []*commonservice.FileChecksumResult
	for _, n := range sent {
		a = append(a, ok(n))
	}
	return a
}

func mentions(fs []string, n string) bool {
	for _, f := range fs {
		if strings.HasPrefix(f, n) {
			return true
		}
	}
	return false
}

func TestFailures(t *testing.T) {
	mismatch := allOk()
	mismatch[1] = &commonservice.FileChecksumResult{Fname: "FileMoverGrpc.dll", Result: 3, Detail: "mismatched file hash !!!"}
	cases := []struct {
		name      string
		sent      []string
		answers   []*commonservice.FileChecksumResult
		notHashed []string
		want      []string // 실패에 들어야 할 이름(개수도 같아야 한다)
	}{
		{"전부 일치하면 통과", sent, allOk(), nil, nil},
		{"서버가 실패로 답한 항목", sent, mismatch, nil, []string{"FileMoverGrpc.dll"}},
		{"실패가 여럿이면 전부", sent, []*commonservice.FileChecksumResult{
			{Fname: "FileMoverApp.exe", Result: 4, Detail: "mismatched file size & hash !!!"},
			{Fname: "FileMoverGrpc.dll", Result: 3, Detail: "mismatched file hash !!!"},
			ok("lang_en.qm"),
		}, nil, []string{"FileMoverApp.exe", "FileMoverGrpc.dll"}},
		{"응답에 없는 항목", sent, []*commonservice.FileChecksumResult{ok("FileMoverApp.exe"), ok("lang_en.qm")}, nil, []string{"FileMoverGrpc.dll"}},
		{"응답이 비면 전부", sent, nil, nil, sent},
		{"해시 못 낸 항목은 서버가 통과라 해도", sent, allOk(), []string{"FileMoverGrpc.dll"}, []string{"FileMoverGrpc.dll"}},
		{"대소문자 무시", sent, []*commonservice.FileChecksumResult{ok("filemoverapp.exe"), ok("FILEMOVERGRPC.DLL"), ok("lang_en.qm")}, nil, nil},
		{"보낸 것도 응답도 없으면 통과", nil, nil, nil, nil},
		{"실패 사유는 보낸 이름으로", []string{"FileMoverApp.exe", WireName("fmenv.ini", 1)}, []*commonservice.FileChecksumResult{
			ok("FileMoverApp.exe"), {Fname: "fmenv.ini.int", Result: 3, Detail: "mismatched file hash !!!"},
		}, nil, []string{"fmenv.ini.int"}},
	}
	for _, c := range cases {
		got := Failures(c.sent, c.answers, c.notHashed)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v", c.name, got)
			continue
		}
		for _, n := range c.want {
			if !mentions(got, n) {
				t.Errorf("%s: %s 가 없다 %v", c.name, n, got)
			}
		}
	}
	if f := Failures(sent, mismatch, nil); !strings.Contains(f[0], "mismatched file hash !!!") {
		t.Errorf("사유: %v", f)
	}
}

func TestSkipUnanswered(t *testing.T) {
	cases := []struct {
		kind int
		code codes.Code
		want bool
	}{
		{KindStart, codes.Unavailable, false}, // 로그인 직후 검사는 넘기지 않는다
		{KindStart, codes.Internal, false},
		{KindPeriodic, codes.Unavailable, true},
		{KindAdmin, codes.DeadlineExceeded, true},
		{KindPeriodic, codes.Canceled, true},  // 닫힌 연결로 보낸 요청(Go gRPC)
		{KindPeriodic, codes.Internal, false}, // 답이 왔는데 못 읽음 → 판정
		{KindAdmin, codes.Unknown, false},
	}
	for _, c := range cases {
		if got := SkipUnanswered(c.kind, c.code); got != c.want {
			t.Errorf("%d %s: %v", c.kind, c.code, got)
		}
	}
}

func TestWireName(t *testing.T) {
	for _, c := range []struct {
		f    string
		loc  int
		want string
	}{
		{"fmenv.ini", 1, "fmenv.ini.int"}, {"fmenv.ini", 0, "fmenv.ini.ext"}, {"fmenv.ini", -1, "fmenv.ini.ext"},
		{"snbApp.exe", 1, "snbApp.exe"}, {"lang_en.qm", 0, "lang_en.qm"},
	} {
		if got := WireName(c.f, c.loc); got != c.want {
			t.Errorf("%s %d: %s", c.f, c.loc, got)
		}
	}
}

func TestRequest(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fmenv.ini")
	if err := os.WriteFile(p, []byte("[SERVER01]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req, s, missing := Request([]string{p, filepath.Join(dir, "없는.exe")}, KindAdmin, 1)
	sum := sha256.Sum256([]byte("[SERVER01]\n"))
	it := req.List[0]
	if it.Fname != "fmenv.ini.int" || string(it.Hash) != hex.EncodeToString(sum[:]) || it.Fsize != 11 ||
		it.TypeDiv != "FM" || it.Version != "6.0.0.1" || it.Location != 1 {
		t.Errorf("항목: %+v", it)
	}
	if string(req.List[1].Hash) != "not found file" || req.List[1].Fsize != 0 || req.List[1].Location != 1 {
		t.Errorf("못 낸 항목: %+v", req.List[1])
	}
	if req.TotCnt != 2 || req.ChkDiv != KindAdmin || len(s) != 2 || len(missing) != 1 || missing[0] != "없는.exe" {
		t.Errorf("req=%v sent=%v missing=%v", req, s, missing)
	}
}

type fakeCommon struct {
	commonservice.CommonServiceClient
	res *commonservice.IntegrityResponse
	err error
}

func (f fakeCommon) CheckIntegrity(context.Context, *commonservice.IntegrityRequest, ...grpc.CallOption) (*commonservice.IntegrityResponse, error) {
	return f.res, f.err
}

func TestCheck(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows 에서만 검사한다")
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	p := filepath.Join(dir, "a.exe")
	_ = os.WriteFile(p, []byte("x"), 0o600)
	pass := fakeCommon{res: &commonservice.IntegrityResponse{List: []*commonservice.FileChecksumResult{ok("a.exe")}}}
	if f, skip := Check(context.Background(), pass, []string{p}, KindStart, 1, log); len(f) != 0 || skip {
		t.Errorf("통과: %v %v", f, skip)
	}
	down := fakeCommon{err: status.Error(codes.Unavailable, "down")}
	if f, skip := Check(context.Background(), down, []string{p}, KindPeriodic, 1, log); len(f) != 0 || !skip {
		t.Errorf("주기 검사 통신 실패는 넘긴다: %v %v", f, skip)
	}
	if f, skip := Check(context.Background(), down, []string{p}, KindStart, 1, log); len(f) != 1 || skip {
		t.Errorf("로그인 직후 통신 실패는 실패: %v %v", f, skip)
	}
}

func TestEnforced(t *testing.T) {
	t.Setenv("SNB_INTEGRITY_ENFORCE", "")
	t.Setenv("SNB_DEV", "")
	if Enforced() == config.IsDev() {
		t.Error("기본값")
	}
	t.Setenv("SNB_DEV", "1")
	if !config.IsDev() {
		t.Skip("배포 빌드는 SNB_DEV 를 무시한다(config.TestIsDevBuilds)")
	}
	if Enforced() {
		t.Error("개발 빌드는 끈다")
	}
	t.Setenv("SNB_INTEGRITY_ENFORCE", "1")
	if !Enforced() {
		t.Error("SNB_INTEGRITY_ENFORCE=1 은 켠다")
	}
	t.Setenv("SNB_INTEGRITY_PERIOD_SEC", "40")
	if Period().Seconds() != 40 {
		t.Errorf("개발 빌드 주기: %v", Period())
	}
	t.Setenv("SNB_DEV", "")
	if !config.IsDev() && Period().Hours() != 3 {
		t.Errorf("설치본 주기: %v", Period())
	}
}
