package svc

import (
	"context"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"snbapp/internal/approval"
	"snbapp/internal/config"
	"snbapp/internal/nbs"
)

// 실 서버로 결재자 창 왕복(PickApprover → approval:select → ApproverReply)을 본다. SNB_LIVE_REQ_ID·SNB_LIVE_APR_ID 가 없으면 SKIP.
//
//	go test -run TestLivePick -v -count=1 ./internal/svc
func TestLivePick(t *testing.T) {
	id, aprID := os.Getenv("SNB_LIVE_REQ_ID"), os.Getenv("SNB_LIVE_APR_ID")
	if id == "" || aprID == "" {
		t.Skip("SNB_LIVE_REQ_ID·SNB_LIVE_APR_ID 가 없다")
	}
	dir := config.AppDir()
	env, err := config.LoadEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	var s *nbs.Session
	for _, srv := range env.Servers {
		if srv.Loc == config.LocBusiness {
			if s, err = nbs.Login(dir, srv, id, os.Getenv("SNB_LIVE_REQ_PW"), "0.1.0-test", slog.Default()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if s == nil {
		t.Fatal("업무망 서버가 없다")
	}
	defer s.Close()
	a := newApprovalService(NewCore("test", slog.Default()))

	// 업로드 직전: 사후결재 모드면 창 없이 결재자 없음
	if !approval.PreMode(s.Profile) {
		r, err := a.pick(context.Background(), s, approval.PickRequest{Files: []approval.PickFile{{Name: "a.txt", Size: 1}}})
		if err != nil || r.Dialog || len(r.ApproverIDs) != 0 || r.Resv != "2" {
			t.Fatalf("사후결재 모드 업로드: %+v %v", r, err)
		}
	}

	// [결재요청](After): 창을 띄우고 답을 받는다
	type out struct {
		r   *approval.PickResult
		err error
	}
	ch := make(chan out, 1)
	go func() {
		r, err := a.pick(context.Background(), s, approval.PickRequest{Files: []approval.PickFile{{Name: "a.txt", Size: 10}, {Name: "b.txt", Size: 20}}, After: true})
		ch <- out{r, err}
	}()
	var gen int64
	for i := 0; i < 100 && gen == 0; i++ {
		a.mu.Lock()
		if a.ask != nil {
			gen = a.ask.gen
		}
		a.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	if gen == 0 {
		o := <-ch
		t.Fatalf("창이 뜨지 않았다: %+v %v", o.r, o.err)
	}
	if err := a.ApproverReply(gen, []string{aprID}, "시험 사유", false); err != nil {
		t.Fatal(err)
	}
	o := <-ch
	if o.err != nil || !o.r.Dialog || !reflect.DeepEqual(o.r.ApproverIDs, []string{aprID}) || len(o.r.ApproverNames) != 1 || o.r.Cause != "시험 사유" || o.r.AprvDept != s.Info.UserDeptName {
		t.Fatalf("%+v %v", o.r, o.err)
	}
	t.Logf("결재자 창 답: %+v", *o.r)

	// ctx 가 끝나면 창을 닫고 ctx 오류
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := a.pick(ctx, s, approval.PickRequest{Files: []approval.PickFile{{Name: "a.txt"}}, After: true}); err != context.DeadlineExceeded || a.ask != nil {
		t.Fatalf("ctx 끝: %v", err)
	}
}

// ApproverReply: 트리 순서·중복 제거, 결재자 없음·사유 없음·자기 지정은 창을 닫지 않고 오류, 지난 gen 은 버림, 취소.
func TestApproverReply(t *testing.T) {
	a := newApprovalService(NewCore("test", slog.Default()))
	order := []approval.User{{ID: "u1", Name: "일"}, {ID: "me", Name: "나"}, {ID: "u2", Name: "이"}}
	arm := func(selfOK bool) *pendingAsk {
		p := &pendingAsk{gen: 7, myID: "me", selfOK: selfOK, order: order, ch: make(chan askAnswer, 1)}
		a.ask = p
		return p
	}

	p := arm(false)
	for _, c := range []struct {
		ids   []string
		cause string
		want  string
	}{
		{nil, "사유", "결재자를 선택해 주세요."},
		{[]string{"zz"}, "사유", "결재자를 선택해 주세요."}, // 트리에 없는 ID
		{[]string{"u1"}, "  ", "결재요청 사유를 입력해 주세요."},
		{[]string{"u1", "me"}, "사유", "자신을 결재자로 선택할 수 없습니다."},
	} {
		err := a.ApproverReply(7, c.ids, c.cause, false)
		if err == nil || err.Error() != c.want {
			t.Errorf("%v %q → %v", c.ids, c.cause, err)
		}
		if a.ask != p {
			t.Fatal("판정 오류인데 창이 닫혔다")
		}
	}
	if err := a.ApproverReply(6, []string{"u1"}, "x", false); err != nil || a.ask != p {
		t.Error("지난 gen 은 조용히 버린다")
	}
	if err := a.ApproverReply(7, []string{"u2", "u1", "u2"}, " 사유 ", false); err != nil {
		t.Fatal(err)
	}
	got := <-p.ch
	if !reflect.DeepEqual(got.ids, []string{"u1", "u2"}) || !reflect.DeepEqual(got.names, []string{"일", "이"}) || got.cause != "사유" || a.ask != nil {
		t.Errorf("%+v", got)
	}

	p = arm(true) // self_approve on 이면 나를 골라도 된다
	if err := a.ApproverReply(7, []string{"me"}, "x", false); err != nil {
		t.Fatal(err)
	}
	if got := <-p.ch; !reflect.DeepEqual(got.ids, []string{"me"}) {
		t.Errorf("%+v", got)
	}

	p = arm(false)
	if err := a.ApproverReply(7, nil, "", true); err != nil || !(<-p.ch).cancel || a.ask != nil {
		t.Error("취소")
	}

	p = arm(false)
	a.onLogout() // 로그아웃은 기다리던 창을 취소로 정리한다
	if !(<-p.ch).cancel || a.ask != nil {
		t.Error("로그아웃 취소")
	}
}
