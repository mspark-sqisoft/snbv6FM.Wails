package approval

// 실 서버 시험(업무망). 환경변수가 없으면 SKIP. 비밀번호는 환경변수로만 받는다.
//   SNB_APP_DIR                    fmenv.ini·root_ca.pem 폴더(예: E:\git\snBv6FM\build\release)
//   SNB_LIVE_REQ_ID / _PW          요청자(test<N>_1)
//   SNB_LIVE_APR_ID / _PW          결재자(test<N>_2, 요청자의 결재선)
//   SNB_LIVE_PROXY_ID              대리결재자 후보(test<N>_3) ― 있으면 대리결재 등록·삭제도 본다
//
//   go test -run Live -v -count=1 ./internal/approval
//
// 흐름: 요청자가 작은 글자 파일 둘을 업무망에 올려 결재요청(사전결재 모드면 업로드에 결재자를 싣고, 사후결재 모드면 4127) →
// 결재자 [3390] 에 보이는지 → 하나 승인·하나 반려 → 요청자 서버파일 목록에서 상태 1·2 → 올린 파일을 지운다.

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/pb/serverfile"
)

func liveLogin(t *testing.T, idKey, pwKey string) *nbs.Session {
	t.Helper()
	id := os.Getenv(idKey)
	if id == "" {
		t.Skipf("%s 가 없다", idKey)
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, srv := range env.Servers {
		if srv.Loc == config.LocBusiness {
			s, err := nbs.Login(dir, srv, id, os.Getenv(pwKey), "0.1.0-test", slog.Default())
			if err != nil {
				t.Fatalf("%s 로그인: %v", id, err)
			}
			t.Cleanup(func() { s.Close() })
			return s
		}
	}
	t.Fatal("업무망 서버가 없다")
	return nil
}

// liveUpload 는 결재 칸을 실은 업로드다(전송 담당의 Upload 가 PickResult 를 받기 전까지 시험에서만 쓴다).
func liveUpload(t *testing.T, s *nbs.Session, name string, data []byte, p *PickResult) {
	t.Helper()
	sum := sha256.Sum256(data)
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	st, err := s.ServerFile().FileUpload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info := &serverfile.FileTransInfo{
		Fsize: uint64(len(data)), Fpath: name, ApproverId: p.ApproverIDs, AprvKind: p.AprvKind, AprvType: p.AprvType,
		AprvDept: p.AprvDept, Fcause: p.Cause, Resv: p.Resv, Checksum: hex.EncodeToString(sum[:]),
	}
	if err := st.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Info{Info: info}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Chunk{Chunk: data}}); err != nil {
		t.Fatal(err)
	}
	if err := st.CloseSend(); err != nil {
		t.Fatal(err)
	}
	for {
		a, err := st.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("업로드: %v", err)
		}
		if a.Res != 1 {
			t.Fatalf("업로드 거부: %s", a.Reason)
		}
	}
}

func findFile(t *testing.T, s *nbs.Session, name string) *serverfile.FileInfo {
	t.Helper()
	list, err := files.List(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range list {
		if f.NameStr == name {
			return f
		}
	}
	return nil
}

// waitFile 은 서버가 검사(백신·개인정보)를 끝낼 때까지 cond 를 기다린다.
func waitFile(t *testing.T, s *nbs.Session, name string, cond func(*serverfile.FileInfo) bool) *serverfile.FileInfo {
	t.Helper()
	var f *serverfile.FileInfo
	for i := 0; i < 30; i++ {
		if f = findFile(t, s, name); f != nil && cond(f) {
			return f
		}
		time.Sleep(time.Second)
	}
	if f == nil {
		t.Fatalf("%s 가 서버 목록에 없다", name)
	}
	t.Fatalf("%s 상태가 바뀌지 않았다: state %d virus %d priv %d", name, f.State, f.VirusFlag, f.PrivState)
	return nil
}

func TestLiveApproval(t *testing.T) {
	req := liveLogin(t, "SNB_LIVE_REQ_ID", "SNB_LIVE_REQ_PW")
	apr := liveLogin(t, "SNB_LIVE_APR_ID", "SNB_LIVE_APR_PW")
	pre := PreMode(req.Profile)
	typ := req.AprvType(false)
	t.Logf("요청자 %s(%s) 부서 %s fm_upfile_mode=%q 사전결재모드=%v 파일결재방식=%d(%s) 클립결재방식=%d self_approve=%q",
		req.UserID, req.Info.UserName, req.Info.UserDeptName, req.Profile["fm_upfile_mode"], pre, typ, TypeText(typ), req.AprvType(true), req.Profile["self_approve"])

	// 결재자 목록[3370] 과 트리
	start := time.Now()
	list, err := Approvers(req)
	if err != nil {
		t.Fatal(err)
	}
	tree := BuildTree(list)
	var me *User
	for _, u := range TreeUsers(tree) {
		if u.ID == apr.UserID {
			u := u
			me = &u
		}
	}
	t.Logf("결재자 목록 %d건(%v), 트리 뿌리 %d개, 트리 사람 %d명", len(list), time.Since(start).Round(time.Millisecond), len(tree), len(TreeUsers(tree)))
	if len(tree) == 0 || me == nil {
		t.Fatalf("트리에 결재자 %s 가 없다", apr.UserID)
	}
	if !NeedsApprover(typ) {
		t.Skipf("결재방식 %d 는 결재자가 필요 없어 결재 흐름을 볼 수 없다", typ)
	}

	stamp := time.Now().Format("20060102_150405")
	names := []string{"WAILS_APRV_" + stamp + "_ok.txt", "WAILS_APRV_" + stamp + "_no.txt"}
	cause := "WAILS 결재 시험"
	uploaded := []string{}
	t.Cleanup(func() { // 올린 시험 파일을 지운다
		for _, n := range uploaded {
			if f := findFile(t, req, n); f != nil {
				ctx, cancel := req.Ctx(nbs.RPCTimeout)
				_, err := req.ServerFile().RemoveFile(ctx, &serverfile.FileRemoveRequest{Spath: f.ServerStr, Flag: 0})
				cancel()
				t.Logf("지움 %s: %v", n, err)
			}
		}
		for _, n := range uploaded {
			if f := findFile(t, req, n); f != nil {
				t.Errorf("%s 가 지워지지 않았다", n)
			}
		}
	})

	var pick *PickResult
	if pre {
		pick = &PickResult{ApproverIDs: []string{me.ID}, ApproverNames: []string{me.Name}, Cause: cause, AprvType: typ, AprvDept: req.Info.UserDeptName}
	} else {
		pick = &PickResult{AprvType: typ, AprvDept: req.Info.UserDeptName}
	}
	pick.Resv, pick.AprvKind = UploadFields(false, pre, typ, len(pick.ApproverIDs))
	for _, n := range names {
		liveUpload(t, req, n, []byte("snbApp Wails approval test "+n+"\n"), pick)
		uploaded = append(uploaded, n)
	}

	if !pre { // 사후결재: 상태 4(결재요청 가능)를 기다려 4127
		var svr, pws []string
		for _, n := range names {
			f := waitFile(t, req, n, func(f *serverfile.FileInfo) bool { return CanRequest(f.VirusFlag, f.PrivState, f.State) })
			svr, pws = append(svr, f.ServerStr), append(pws, f.ZipPswd)
		}
		sent, fails := RequestAfter(req, AfterRequest{Names: svr, Pswds: pws, ApproverIDs: []string{me.ID}, ApproverNames: []string{me.Name}, AprvType: typ, Cause: cause})
		if sent != 2 || len(fails) > 0 {
			t.Fatalf("4127 sent %d fails %v", sent, fails)
		}
	}
	srv := map[string]string{}
	for _, n := range names {
		f := waitFile(t, req, n, func(f *serverfile.FileInfo) bool { return f.State == 0 })
		srv[n] = f.ServerStr
		t.Logf("요청자 목록: %s state %d(%s) 요청사유 %q", n, f.State, StateText(f.State), f.ReqCause)
	}

	// 결재자 [3390]
	var targets []Target
	for i := 0; i < 10 && len(targets) < 2; i++ {
		targets = nil
		pend, err := Pending(apr)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pend {
			for _, n := range names {
				if p.FileNameSvr == srv[n] {
					targets = append(targets, Target{Seqno: p.Seqno, ServerName: p.FileNameSvr, Requester: p.FileUser, AprvRank: p.AprvRank})
					t.Logf("결재대상: seq %d %s 요청자 %s(%s) 사유 %q 상태 %d", p.Seqno, p.FileNamePc, p.FileUser, p.UserName, p.FileCause, p.AprvState)
				}
			}
		}
		if len(targets) < 2 {
			time.Sleep(time.Second)
		}
	}
	if len(targets) != 2 {
		t.Fatalf("결재대상 %d건(2건이어야)", len(targets))
	}
	byName := map[string]Target{}
	for _, tg := range targets {
		for n, s := range srv {
			if tg.ServerName == s {
				byName[n] = tg
			}
		}
	}
	okRes := Approve(apr, []Target{byName[names[0]]}, true, "WAILS 승인")
	noRes := Approve(apr, []Target{byName[names[1]]}, false, "WAILS 반려")
	if !okRes[0].OK || !noRes[0].OK {
		t.Fatalf("결재 결과 승인 %+v 반려 %+v", okRes, noRes)
	}
	pend, err := Pending(apr)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pend {
		if p.FileNameSvr == srv[names[0]] || p.FileNameSvr == srv[names[1]] {
			t.Errorf("결재 뒤에도 목록에 있다: %s", p.FileNameSvr)
		}
	}
	f1 := waitFile(t, req, names[0], func(f *serverfile.FileInfo) bool { return f.State == 1 || f.State == 3 })
	f2 := waitFile(t, req, names[1], func(f *serverfile.FileInfo) bool { return f.State == 2 })
	t.Logf("요청자 목록 뒤: %s state %d / %s state %d", names[0], f1.State, names[1], f2.State)
}

func TestLiveProxy(t *testing.T) {
	alt := os.Getenv("SNB_LIVE_PROXY_ID")
	if alt == "" {
		t.Skip("SNB_LIVE_PROXY_ID 가 없다")
	}
	apr := liveLogin(t, "SNB_LIVE_APR_ID", "SNB_LIVE_APR_PW")
	cur, err := Proxy(apr)
	if err != nil {
		t.Fatal(err)
	}
	if cur.AltUserid != "" {
		t.Skipf("이미 대리결재자 %s 가 등록돼 있어 건드리지 않는다", cur.AltUserid)
	}
	list, err := Approvers(apr)
	if err != nil {
		t.Fatal(err)
	}
	name := ""
	for _, u := range Candidates(list, apr.UserID) {
		if u.ID == alt {
			name = u.Name
		}
	}
	if name == "" {
		t.Fatalf("후보에 %s 가 없다", alt)
	}
	from := time.Now().Format(time.DateOnly)
	to := time.Now().AddDate(0, 0, 4).Format(time.DateOnly)
	if err := CheckDeputyPeriod(from, to, DeputyLimit(apr.Profile)); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProxy(apr, alt, name, from, to, "WAILS 대리결재 시험"); err != nil {
		t.Fatal(err)
	}
	got, err := Proxy(apr)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("등록 뒤: %s(%s) %s ~ %s", got.AltUserid, got.AltUsername, ScreenDate(got.ExpireFrom), ScreenDate(got.ExpireTo))
	delErr := DeleteProxy(apr, got.AltUserid, got.AltUsername)
	after, err := Proxy(apr)
	if err != nil {
		t.Fatal(err)
	}
	if got.AltUserid != alt || delErr != nil || after.AltUserid != "" {
		t.Fatalf("등록 %q 삭제 %v 삭제 뒤 %q", got.AltUserid, delErr, after.AltUserid)
	}
	if !strings.HasPrefix(ScreenDate(got.ExpireFrom), from[:4]) {
		t.Errorf("기간 %s", got.ExpireFrom)
	}
}
