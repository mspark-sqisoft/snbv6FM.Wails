package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// testSet 은 black 판정에 TXT(헤더 없음)·PDF·ZIP·7Z 허용, EXE 차단을 둔 한 벌이다.
func testSet(profile string) *Set {
	return setWith("ext_check_method=black,"+profile).WithExtInfo(nil).WithPatterns(pattRows(
		"TXT|0||1", "PDF|1|25504446|1", "ZIP|1|504B0304|1", "7Z|1|377ABCAF271C|1", "EXE|1|4D5A|0"), nil)
}

type recorder struct {
	mu   sync.Mutex
	logs []string
	asks []string
	pws  []string // AskPassword 가 차례로 돌려줄 답("" 이면 취소)
}

func (r *recorder) hooks() Hooks {
	return Hooks{
		Log: func(s string) { r.mu.Lock(); r.logs = append(r.logs, s); r.mu.Unlock() },
		AskPassword: func(_ context.Context, name string, retry bool) (string, bool) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.asks = append(r.asks, name+map[bool]string{true: ":retry", false: ""}[retry])
			if len(r.pws) == 0 {
				return "", false
			}
			pw := r.pws[0]
			r.pws = r.pws[1:]
			return pw, pw != ""
		},
	}
}

func (r *recorder) joined() string { return strings.Join(r.logs, "\n") }

func rowByName(rows []Row, top bool, name string) (Row, bool) {
	for _, r := range rows {
		if r.Top == top && r.Name == name {
			return r, true
		}
	}
	return Row{}, false
}

func mkfile(t *testing.T, path string, data []byte) string {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunUploadCheckFiles(t *testing.T) {
	dir := t.TempDir()
	ok := mkfile(t, filepath.Join(dir, "ok.txt"), []byte("hello"))
	bad := mkfile(t, filepath.Join(dir, "bad.exe"), []byte("MZ\x90\x00"))
	ren := mkfile(t, filepath.Join(dir, "ren.txt"), []byte("%PDF-1.7"))
	zero := mkfile(t, filepath.Join(dir, "zero.txt"), nil)
	out := RunUploadCheck(context.Background(), testSet(""), CheckOptions{Paths: []string{ok, bad, ren, zero, filepath.Join(dir, "missing.txt")}}, Hooks{})
	if out.Fails != 4 || out.Blocks != 4 || len(out.Items) != 1 || out.Items[0].Path != ok || out.Value != "" {
		t.Fatalf("%+v", out)
	}
	for name, want := range map[string]string{"ok.txt": "정상 파일", "bad.exe": "허용되지 않은 확장자 헤더패턴", "ren.txt": "변경된 확장자",
		"zero.txt": "파일 사이즈 0", "missing.txt": "파일 열기 실패"} {
		if r, _ := rowByName(out.Rows, true, name); r.Result != want {
			t.Errorf("%s: %q", name, r.Result)
		}
	}
	if len(out.Mismatches) != 4 {
		t.Fatalf("mismatches %d", len(out.Mismatches))
	}
	if out.Items[0].Size != 5 || out.Items[0].ModTime.IsZero() {
		t.Fatal("snapshot")
	}
}

func TestRunUploadCheckFolder(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "F")
	mkfile(t, filepath.Join(folder, "ok.txt"), []byte("hello"))
	bad := mkfile(t, filepath.Join(folder, "sub", "bad.exe"), []byte("MZ\x90"))
	mkfile(t, filepath.Join(folder, "in.zip"), zipBytes(t, ent{"x.exe", []byte("MZ")}, ent{"y.txt", []byte("Y")}))
	clean := filepath.Join(dir, "C")
	mkfile(t, filepath.Join(clean, "a.txt"), []byte("A"))

	out := RunUploadCheck(context.Background(), testSet(""), CheckOptions{Paths: []string{folder, clean}}, Hooks{})
	if out.Fails != 2 || out.Blocks != 0 || out.Skip != 2 || out.Value != "SKIP:2" || len(out.Items) != 2 {
		t.Fatalf("human %+v", out)
	}
	if ex := out.Items[0].Excludes; len(ex) != 2 || !(ex[0] == bad || ex[1] == bad) {
		t.Fatalf("excludes %v", ex)
	}
	if r, _ := rowByName(out.Rows, true, "F"); r.Result != "차단 2건 제외" || r.Detail != "내부 오류 2건" {
		t.Fatalf("top %+v", r)
	}
	if r, _ := rowByName(out.Rows, true, "C"); r.Result != "정상 폴더" || !r.OK {
		t.Fatalf("clean %+v", r)
	}
	if r, _ := rowByName(out.Rows, false, "sub/bad.exe"); r.Result != "(제외됨) 허용되지 않은 확장자 헤더패턴" || r.Parent != "F" {
		t.Fatalf("inner %+v", r)
	}
	// 압축째 빠지면 속의 정상 파일 행에도 표시가 붙는다
	if r, _ := rowByName(out.Rows, false, "y.txt"); r.Result != "(제외됨) 정상 파일" || r.Parent != "in.zip" {
		t.Fatalf("zip child %+v", r)
	}
	if r, _ := rowByName(out.Rows, false, "ok.txt"); r.Result != "정상 파일" || !r.OK {
		t.Fatalf("ok %+v", r)
	}

	auto := RunUploadCheck(context.Background(), testSet(""), CheckOptions{Paths: []string{folder}, Auto: true}, Hooks{})
	if auto.Blocks != 2 || auto.Value != "AUTO" || len(auto.Items) != 0 {
		t.Fatalf("auto %+v", auto)
	}
	if r, _ := rowByName(auto.Rows, true, "F"); r.Result != "오류 파일 존재" {
		t.Fatalf("auto top %+v", r)
	}
}

func TestRunUploadCheckArchive(t *testing.T) {
	dir := t.TempDir()
	z := mkfile(t, filepath.Join(dir, "a.zip"), zipBytes(t, ent{"x.exe", []byte("MZ")}, ent{"y.txt", []byte("Y")}))
	out := RunUploadCheck(context.Background(), testSet(""), CheckOptions{Paths: []string{z}}, Hooks{})
	r, _ := rowByName(out.Rows, true, "a.zip")
	if out.Blocks != 1 || r.Result != "압축 해제 성공" || r.Detail != "내부 파일 2개, 오류 1건" || r.OK {
		t.Fatalf("%+v %+v", out, r)
	}
	if len(out.Mismatches) != 1 || out.Mismatches[0].Fname != z+"/x.exe" {
		t.Fatalf("mismatch %+v", out.Mismatches)
	}
	// 내부 검사를 끄면 압축 자신만 본다
	off := RunUploadCheck(context.Background(), testSet("fm_chk_insidezip=0"), CheckOptions{Paths: []string{z}}, Hooks{})
	if off.Blocks != 0 || len(off.Items) != 1 {
		t.Fatalf("inside off %+v", off)
	}
}

func TestRunUploadCheckLockedZip(t *testing.T) {
	locked, _ := filepath.Abs(filepath.Join("testdata", "zipcrypto.zip"))
	run := func(profile string, auto bool, pws ...string) (*CheckOutcome, *recorder) {
		rec := &recorder{pws: pws}
		return RunUploadCheck(context.Background(), testSet(profile), CheckOptions{Paths: []string{locked}, Auto: auto}, rec.hooks()), rec
	}
	out, rec := run("fm_use_Zip_pwd=1", false, "wrong", fixturePW)
	if out.Blocks != 0 || len(out.Items) != 1 || out.Items[0].ZipPassword != fixturePW {
		t.Fatalf("retry ok %+v", out)
	}
	if strings.Join(rec.asks, ",") != "zipcrypto.zip,zipcrypto.zip:retry" {
		t.Fatalf("asks %v", rec.asks)
	}
	logs := rec.joined()
	if strings.Contains(logs, fixturePW) || strings.Contains(logs, "wrong") || !strings.Contains(logs, "PW[*** len=7]") ||
		!strings.Contains(logs, "암호 압축 >>> 비밀번호가 틀렸다(1/2): zipcrypto.zip") {
		t.Fatalf("logs:\n%s", logs)
	}
	for _, c := range []struct {
		profile string
		auto    bool
		pws     []string
		result  string
		blocks  int
		asks    int
		log     string
	}{
		{"fm_use_Zip_pwd=1", false, []string{"a", "b"}, "압축 비밀번호 오류", 1, 2, ""},
		{"fm_use_Zip_pwd=1", false, nil, "압축 비밀번호 입력 취소", 1, 1, "암호 압축 >>> 비밀번호 입력 취소: zipcrypto.zip"},
		{"fm_use_Zip_pwd=0", false, nil, "압축 패스워드 존재", 1, 0, "암호 압축 >>> 묻지 않고 막는다(fm_use_Zip_pwd != 1): zipcrypto.zip"},
		{"fm_use_zip_pwd=1", false, nil, "압축 패스워드 존재", 1, 0, ""}, // 키 이름은 대소문자를 가린다
		{"fm_use_Zip_pwd=1", true, nil, "압축 패스워드 존재", 1, 0, "[Auto]암호 압축 >>> 자동 업로드라 비밀번호를 묻지 않고 막는다: zipcrypto.zip"},
		{"fm_disable_zip_pass=1,fm_use_Zip_pwd=1", false, nil, "정상 파일", 0, 0, "암호 압축 >>> 안을 검사하지 않고 통과(fm_disable_zip_pass=1): zipcrypto.zip"},
	} {
		out, rec := run(c.profile, c.auto, c.pws...)
		r, _ := rowByName(out.Rows, true, "zipcrypto.zip")
		if r.Result != c.result || out.Blocks != c.blocks || len(rec.asks) != c.asks || (c.log != "" && !strings.Contains(rec.joined(), c.log)) {
			t.Errorf("%s auto=%v: %q blocks=%d asks=%d\n%s", c.profile, c.auto, r.Result, out.Blocks, len(rec.asks), rec.joined())
		}
	}
	// 폴더 안의 암호 압축은 맞혀도 비밀번호를 돌려주지 않는다
	dir := t.TempDir()
	data, _ := os.ReadFile(locked)
	mkfile(t, filepath.Join(dir, "F", "z.zip"), data)
	rec = &recorder{pws: []string{fixturePW}}
	out = RunUploadCheck(context.Background(), testSet("fm_use_Zip_pwd=1"), CheckOptions{Paths: []string{filepath.Join(dir, "F")}}, rec.hooks())
	if out.Blocks != 0 || len(out.Items) != 1 || out.Items[0].ZipPassword != "" {
		t.Fatalf("folder %+v", out)
	}
}

func TestRunUploadCheckUncheckable(t *testing.T) {
	ppmd, _ := filepath.Abs(filepath.Join("testdata", "ppmd.zip"))
	out := RunUploadCheck(context.Background(), testSet(""), CheckOptions{Paths: []string{ppmd}}, Hooks{})
	if r, _ := rowByName(out.Rows, true, "ppmd.zip"); r.Result != "검사할 수 없는 압축 형식" || out.Blocks != 1 {
		t.Fatalf("%+v", r)
	}
	out = RunUploadCheck(context.Background(), testSet("fm_disable_zip_pass=1"), CheckOptions{Paths: []string{ppmd}}, Hooks{})
	if out.Blocks != 0 {
		t.Fatalf("pass %+v", out)
	}
}

func TestRunUploadCheckDepthClampLogAndCancel(t *testing.T) {
	dir := t.TempDir()
	f := mkfile(t, filepath.Join(dir, "a.txt"), []byte("A"))
	rec := &recorder{}
	RunUploadCheck(context.Background(), testSet("fm_extract_zipdepth=9"), CheckOptions{Paths: []string{f}}, rec.hooks())
	if !strings.Contains(rec.joined(), "압축 중첩 깊이 정책 값을 범위로 자른다 >>> fm_extract_zipdepth[9] → 5") {
		t.Fatal(rec.joined())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out := RunUploadCheck(ctx, testSet(""), CheckOptions{Paths: []string{f}}, Hooks{}); !out.Canceled {
		t.Fatal("not canceled")
	}
}

func TestDownloadCheck(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "F")
	mkfile(t, filepath.Join(folder, "a.txt"), []byte("A"))
	mkfile(t, filepath.Join(folder, "s", "b.txt"), []byte("BB"))
	rows, fails, show := DownloadCheck(context.Background(), folder, 2, "", Limits{}, nil)
	if !show || fails != 0 || len(rows) != 3 || rows[0].Result != "폴더 확인" || rows[0].Detail != "내부 파일 2개" || rows[2].Name != "s/b.txt" {
		t.Fatalf("%+v", rows)
	}
	z, _ := filepath.Abs(filepath.Join("testdata", "plain.7z"))
	if rows, _, show := DownloadCheck(context.Background(), z, 2, "", Limits{}, nil); !show || rows[0].Result != "압축 해제 성공" || rows[1].Result != "정상 파일" {
		t.Fatalf("7z %+v", rows)
	}
	if _, _, show := DownloadCheck(context.Background(), filepath.Join("testdata", "zipcrypto.zip"), 2, "", Limits{}, nil); show {
		t.Fatal("locked shown")
	}
	bomb := filepath.Join(dir, "bomb.zip")
	bombZip(t, bomb, 4<<20)
	var logs []string
	rows, fails, show = DownloadCheck(context.Background(), bomb, 2, t.TempDir(), Limits{MaxItems: 10, MaxBytes: 1 << 20}, func(s string) { logs = append(logs, s) })
	if !show || fails != 1 || len(rows) != 1 || rows[0].Result != "압축 해제 상한 초과" || len(logs) != 1 || !strings.Contains(logs[0], "경로[결재 내용 보기]") {
		t.Fatalf("stop %+v %v", rows, logs)
	}
}

func TestClip(t *testing.T) {
	s := NewSet(1, 0, 0, 10)
	if ok, _ := ClipSizeOK(s, 10239); !ok {
		t.Fatal("under")
	}
	if ok, text := ClipSizeOK(s, 10240); ok || text != "전송 가능한 파일 크기 초과" {
		t.Fatal("over")
	}
	if !HasClipMarker(`C:\x\__CLIP__.txt`) || HasClipMarker("a.txt") {
		t.Fatal("marker")
	}
}
