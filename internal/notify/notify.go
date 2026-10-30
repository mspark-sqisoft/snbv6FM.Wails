// Package notify 는 서버 알림(NotifyService)이다: 알림 스트림 받기, 알림 종류별 처리기 등록·분배, 판정 규칙.
// 원본은 snBv6FM 의 FileMoverGrpc::startNotify·receivedNotify·sendNotify(imports/FileMoverGrpc/FileMoverGrpc.cpp)다.
// 화면·세션에 닿는 일(목록 갱신, 확인 창, 로그아웃)은 svc 가 처리기로 등록한다 ― 이 패키지는 svc 를 모른다.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/status"

	"snbapp/internal/applog"
	"snbapp/pb/common"
	pb "snbapp/pb/notify"
)

// 전문번호(notify_svcno).
const (
	SvcFileList    int32 = 9001 // 파일 목록 갱신(userid 가 비거나 나일 때)
	SvcApprovalReq int32 = 9002 // 결재 요청(결재자에게)
	SvcKill        int32 = 9003 // 서버의 중복 로그인 종료 ― 로그인 때 cancel_9003=1 로 받지 않는다
	SvcEvent       int32 = 9004 // kind 로 갈린다(아래 Kind*)
	SvcIntegrity   int32 = 9005 // 관리자 수동 무결성 점검(SBM [변조 체크])
	SvcNotice      int32 = 9007 // 전체 공지(msg 제목, dat 본문)
)

// 9004 의 kind(원본 MSG_KIND_*).
const (
	KindFileList int32 = iota // 파일 목록 갱신(결재 목록도 ― 원본의 fallthrough)
	KindFileAprv              // 결재 목록 갱신
	KindMailList              // 메일 목록(쓰지 않음, 로그만)
	KindMailAprv              // 메일 결재(쓰지 않음, 로그만)
	KindURLSend               // 링크 리다이렉트(dat = URL)
	KindEndAprv               // 결재 완료 → 요청자에게(dat = "Y" 승인, 그 밖 반려)
	KindDupFM                 // 다중 로그인 종료
	KindReconFM               // 다중 로그인 알림(지금은 종료와 같게 처리)
)

// SideSame 은 같은 망(원본 MSG_SIDE_SAME)이다.
const SideSame int32 = 0

// AnyKind 로 등록하면 그 전문번호의 모든 kind 에 불린다.
const AnyKind int32 = -1

// ProxyProcessDat 는 9002 의 dat 가 이 값이면 대리결재 처리라 원결재자에게 알림창을 띄우지 않는다.
const ProxyProcessDat = "process_replace_approval"

// Handler 는 알림 하나를 처리한다. 스트림을 받는 goroutine 에서 불리므로 오래 막지 않는다(오래 걸리면 go 로 띄운다).
type Handler func(*pb.NotifyMessage)

// Dispatcher 는 (전문번호, kind) 별 처리기 목록이다. 영역은 서비스 생성자에서 Register 한다(c.Notify().Register(...)).
type Dispatcher struct {
	mu sync.RWMutex
	hs map[[2]int32][]Handler
}

// NewDispatcher 는 빈 Dispatcher 다.
func NewDispatcher() *Dispatcher { return &Dispatcher{hs: map[[2]int32][]Handler{}} }

// Register 는 처리기를 더한다. kind 가 AnyKind 면 그 전문번호의 모든 kind 에 불린다. 등록 순서대로 부른다.
func (d *Dispatcher) Register(svcno, kind int32, h Handler) {
	d.mu.Lock()
	k := [2]int32{svcno, kind}
	d.hs[k] = append(d.hs[k], h)
	d.mu.Unlock()
}

// Dispatch 는 맞는 처리기를 모두 부르고 부른 수를 돌려준다(kind 가 정확히 맞는 것 먼저, 그다음 AnyKind).
func (d *Dispatcher) Dispatch(m *pb.NotifyMessage) int {
	d.mu.RLock()
	hs := append(append([]Handler{}, d.hs[[2]int32{m.NotifySvcno, m.Kind}]...), d.hs[[2]int32{m.NotifySvcno, AnyKind}]...)
	d.mu.RUnlock()
	for _, h := range hs {
		h(m)
	}
	return len(hs)
}

// ReceivedLine 은 모든 알림에 남기는 수신 줄이다(원본과 같은 글자, snb.grpc DEBUG).
func ReceivedLine(m *pb.NotifyMessage) string {
	return fmt.Sprintf("Notify 수신: 전문번호[%d] kind[%d] side[%d] alim[%d] msg[%s] userid[%s] ipaddr[%s] dat[%s]",
		m.NotifySvcno, m.Kind, m.Side, m.Alim, m.Msg, m.Userid, m.Ipaddr, m.Dat)
}

// Ignore 는 알림을 버릴 까닭(로그 줄)이다. 버리지 않으면 "".
//   - 9001·9004: userid 가 남의 것이면 버린다.
//   - 9002: userid 와 msg(요청자)가 같으면 자가 승인이라 버린다.
func Ignore(m *pb.NotifyMessage, myID string) string {
	switch m.NotifySvcno {
	case SvcFileList, SvcEvent:
		if m.Userid != "" && m.Userid != myID {
			return fmt.Sprintf("Notify Ignored: Different UserID ( %s  vs  %s )", m.Userid, myID)
		}
	case SvcApprovalReq:
		if m.Userid != "" && m.Userid == m.Msg {
			return fmt.Sprintf("자가승인은 알림 안띄움 >>> UserID ( %s == %s )", m.Userid, m.Msg)
		}
	}
	return ""
}

// ApprovalResult 는 9004 kind 5 의 결과 글자다.
func ApprovalResult(dat string) string {
	if dat == "Y" {
		return "승인"
	}
	return "반려"
}

// ApprovalDone 은 결재 뒤 요청자에게 보내는 결재 완료 알림(9004 kind 5)이다. 자가 결재(요청자가 나)면 nil ― 보내지 않는다.
func ApprovalDone(reqUser, myID string, approved bool) *pb.NotifyMessage {
	if reqUser == "" || reqUser == myID {
		return nil
	}
	dat := "N"
	if approved {
		dat = "Y"
	}
	return &pb.NotifyMessage{NotifySvcno: SvcEvent, Kind: KindEndAprv, Side: SideSame, Userid: reqUser, Dat: dat}
}

// DupLogin 은 로그인 때 다른 PC 의 같은 계정을 끊게 하는 알림(9004 kind 6)이다(원본 multiLoginClient).
func DupLogin(myID string) *pb.NotifyMessage {
	return &pb.NotifyMessage{NotifySvcno: SvcEvent, Kind: KindDupFM, Side: SideSame, Userid: myID}
}

// Run 은 알림 스트림을 열고 끝날 때까지 받는다. 메시지마다 수신 줄을 남기고 onMsg 를 부른다.
// ctx 가 취소되거나 서버·연결이 끊기면 그 오류를 돌려준다(끊김 판정은 부른 쪽이 세대로 한다).
func Run(ctx context.Context, c pb.NotifyServiceClient, log *slog.Logger, onMsg func(*pb.NotifyMessage)) error {
	log.Debug("Notify Stream 시작!.", "category", applog.CatGrpc)
	st, err := c.NotifyServerStream(ctx, &common.Empty{})
	if err == nil {
		for {
			var m *pb.NotifyMessage
			if m, err = st.Recv(); err != nil {
				break
			}
			log.Debug(ReceivedLine(m), "category", applog.CatGrpc)
			onMsg(m)
		}
	}
	s := status.Convert(err)
	log.Debug(fmt.Sprintf("Notify Stream finished signal. status = %d %s", s.Code(), s.Message()), "category", applog.CatGrpc)
	return err
}

// Debouncer 는 열쇠마다 마지막 요청 d 뒤에 한 번만 부른다(9001 이 짧은 사이 여러 번 와도 목록 조회는 한 번 ― 원본은 매번).
type Debouncer struct {
	mu sync.Mutex
	d  time.Duration
	t  map[string]*time.Timer
}

// NewDebouncer 는 d 지연의 Debouncer 다.
func NewDebouncer(d time.Duration) *Debouncer { return &Debouncer{d: d, t: map[string]*time.Timer{}} }

// Do 는 key 의 앞 예약을 지우고 d 뒤 fn 을 예약한다.
func (b *Debouncer) Do(key string, fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if old := b.t[key]; old != nil {
		old.Stop()
	}
	var me *time.Timer
	me = time.AfterFunc(b.d, func() {
		b.mu.Lock()
		cur := b.t[key] == me
		if cur {
			delete(b.t, key)
		}
		b.mu.Unlock()
		if cur {
			fn()
		}
	})
	b.t[key] = me
}

// Stop 은 예약을 모두 지운다(로그아웃).
func (b *Debouncer) Stop() {
	b.mu.Lock()
	for k, t := range b.t {
		t.Stop()
		delete(b.t, k)
	}
	b.mu.Unlock()
}
