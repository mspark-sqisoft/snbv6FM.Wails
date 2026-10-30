package notify

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"snbapp/pb/common"
	pb "snbapp/pb/notify"
)

func TestIgnore(t *testing.T) {
	cases := []struct {
		m    *pb.NotifyMessage
		want string // 앞부분, "" 이면 버리지 않음
	}{
		{&pb.NotifyMessage{NotifySvcno: SvcFileList}, ""},
		{&pb.NotifyMessage{NotifySvcno: SvcFileList, Userid: "me"}, ""},
		{&pb.NotifyMessage{NotifySvcno: SvcFileList, Userid: "other"}, "Notify Ignored: Different UserID ( other  vs  me )"},
		{&pb.NotifyMessage{NotifySvcno: SvcEvent, Kind: KindDupFM, Userid: "other"}, "Notify Ignored"},
		{&pb.NotifyMessage{NotifySvcno: SvcApprovalReq, Userid: "a", Msg: "a"}, "자가승인은 알림 안띄움 >>> UserID ( a == a )"},
		{&pb.NotifyMessage{NotifySvcno: SvcApprovalReq, Userid: "a", Msg: "b"}, ""},
		{&pb.NotifyMessage{NotifySvcno: SvcApprovalReq, Userid: "other", Msg: "b"}, ""}, // 9002 는 userid 로 거르지 않는다(원본)
		{&pb.NotifyMessage{NotifySvcno: SvcNotice, Userid: "other"}, ""},
	}
	for _, c := range cases {
		got := Ignore(c.m, "me")
		if (c.want == "") != (got == "") || !strings.HasPrefix(got, c.want) {
			t.Errorf("%v → %q, want %q", c.m, got, c.want)
		}
	}
}

func TestApprovalDone(t *testing.T) {
	if ApprovalDone("me", "me", true) != nil || ApprovalDone("", "me", true) != nil {
		t.Fatal("자가 결재·빈 요청자는 보내지 않는다")
	}
	m := ApprovalDone("req", "me", false)
	if m.NotifySvcno != 9004 || m.Kind != 5 || m.Side != 0 || m.Userid != "req" || m.Dat != "N" || ApprovalResult(m.Dat) != "반려" {
		t.Fatalf("%v", m)
	}
	if ApprovalResult("Y") != "승인" {
		t.Fatal("Y 는 승인")
	}
}

func TestDispatcher(t *testing.T) {
	d := NewDispatcher()
	var order []string
	d.Register(SvcEvent, AnyKind, func(*pb.NotifyMessage) { order = append(order, "any") })
	d.Register(SvcEvent, KindURLSend, func(m *pb.NotifyMessage) { order = append(order, "url:"+m.Dat) })
	d.Register(SvcEvent, KindURLSend, func(*pb.NotifyMessage) { order = append(order, "url2") })
	if n := d.Dispatch(&pb.NotifyMessage{NotifySvcno: SvcEvent, Kind: KindURLSend, Dat: "http://x"}); n != 3 {
		t.Fatalf("n=%d", n)
	}
	if strings.Join(order, ",") != "url:http://x,url2,any" {
		t.Fatalf("%v", order)
	}
	if n := d.Dispatch(&pb.NotifyMessage{NotifySvcno: SvcNotice}); n != 0 {
		t.Fatalf("등록 없는 전문번호 n=%d", n)
	}
}

func TestDebouncer(t *testing.T) {
	b := NewDebouncer(40 * time.Millisecond)
	var n atomic.Int32
	for range 5 {
		b.Do("files", func() { n.Add(1) })
		time.Sleep(5 * time.Millisecond)
	}
	b.Do("approval", func() { n.Add(10) })
	time.Sleep(150 * time.Millisecond)
	if n.Load() != 11 {
		t.Fatalf("묶여서 한 번씩: %d", n.Load())
	}
	b.Do("files", func() { n.Add(100) })
	b.Stop()
	time.Sleep(80 * time.Millisecond)
	if n.Load() != 11 {
		t.Fatal("Stop 뒤에는 부르지 않는다")
	}
}

type fakeClient struct {
	pb.NotifyServiceClient
	msgs []*pb.NotifyMessage
}

type fakeStream struct {
	grpc.ServerStreamingClient[pb.NotifyMessage]
	ctx  context.Context
	msgs []*pb.NotifyMessage
}

func (s *fakeStream) Recv() (*pb.NotifyMessage, error) {
	if len(s.msgs) == 0 {
		<-s.ctx.Done()
		return nil, io.EOF
	}
	m := s.msgs[0]
	s.msgs = s.msgs[1:]
	return m, nil
}

func (c *fakeClient) NotifyServerStream(ctx context.Context, _ *common.Empty, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.NotifyMessage], error) {
	return &fakeStream{ctx: ctx, msgs: c.msgs}, nil
}

func TestRun(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	var got []int32
	c := &fakeClient{msgs: []*pb.NotifyMessage{{NotifySvcno: 9001, Userid: "u"}, {NotifySvcno: 9007, Msg: "제목", Dat: "본문"}}}
	done := make(chan error)
	go func() {
		done <- Run(ctx, c, log, func(m *pb.NotifyMessage) {
			got = append(got, m.NotifySvcno)
			if len(got) == 2 {
				cancel()
			}
		})
	}()
	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("끝나지 않는다")
	}
	if len(got) != 2 || !strings.Contains(buf.String(), "Notify 수신: 전문번호[9007] kind[0] side[0] alim[0] msg[제목] userid[] ipaddr[] dat[본문]") ||
		!strings.Contains(buf.String(), "Notify Stream finished signal. status = ") {
		t.Fatalf("%v\n%s", got, buf.String())
	}
}
