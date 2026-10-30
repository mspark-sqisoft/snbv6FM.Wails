package approval

// 결재 RPC 감싸기(docs/port/06 3장). 서버 approval·approver 서비스는 실패를 res=0 이 아니라 gRPC 오류로 준다(3.3절) ―
// answer() 하나로 판정을 모은다.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/status"

	"snbapp/internal/nbs"
	apb "snbapp/pb/approval"
	vpb "snbapp/pb/approver"
	"snbapp/pb/common"
	npb "snbapp/pb/notify"
)

// rpcErr 는 gRPC 오류를 "원본 실패 문구\n서버 사유" 로 바꾼다.
func rpcErr(base string, err error) error {
	msg := status.Convert(err).Message()
	if base == "" {
		return errors.New(msg)
	}
	return fmt.Errorf("%s\n%s", base, msg)
}

// answer 는 AnswerResponse 응답 판정이다: err → 실패(서버 사유), res != 1 → 실패(reason), 그 밖 성공.
func answer(base string, a *common.AnswerResponse, err error) error {
	if err != nil {
		return rpcErr(base, err)
	}
	if a.GetRes() != 1 {
		if base == "" {
			return errors.New(a.GetReason())
		}
		return fmt.Errorf("%s\n%s", base, a.GetReason())
	}
	return nil
}

// Approvers 는 결재자 목록[3370]이다. type 1(전사 사용자) 고정 ― 원본은 초기화 안 된 값으로 2 를 보낼 수 있었다(서버 거절).
// 빈 user_id 행에서 멈춘다(원본 :2308).
func Approvers(s *nbs.Session) ([]*vpb.ApproverInfo, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Approver().GetApproverList(ctx, &vpb.ApproverListRequest{ReqUserId: s.UserID, Type: 1})
	if err != nil {
		return nil, rpcErr("결재자 목록을 받지 못했습니다.", err)
	}
	list := res.GetList()
	for i, a := range list {
		if a.GetUserId() == "" {
			return list[:i], nil
		}
	}
	return list, nil
}

// AfterRequest 는 사후결재 결재요청[4127] 의 내용이다. Pswds 는 Names 와 같은 자리의 서버파일 목록 zip_pswd(그대로 되돌려 보낸다 ―
// 비우면 서버가 업로드 때 남긴 비밀번호를 지운다).
type AfterRequest struct {
	Names         []string
	Pswds         []string
	ApproverIDs   []string
	ApproverNames []string
	AprvType      int32
	Cause         string
}

// RequestAfter 는 결재요청[4127] 을 비밀번호별 10개 묶음으로 보낸다. 성공한 묶음의 파일 수와, 실패한 묶음 사유들을 준다.
// 이미 성공한 묶음은 되돌리지 않는다(서버에 취소 RPC 가 없다).
func RequestAfter(s *nbs.Session, r AfterRequest) (sent int, fails []string) {
	names := r.ApproverNames
	if len(names) != len(r.ApproverIDs) {
		names = r.ApproverIDs // 원본 setApproverAndCause: 개수가 다르면 ID 를 이름으로
	}
	pkg := int32(time.Now().Unix())
	for _, g := range GroupByZipPswd(r.Names, r.Pswds, 10) {
		ctx, cancel := s.Ctx(nbs.RPCTimeout)
		a, err := s.Approval().ApprovalAfterUpload(ctx, &apb.ApprovalAfterUploadRequest{
			FnameSvr: g.Names, Approver: names, ApproverId: r.ApproverIDs, AprvType: r.AprvType,
			AprvDept: s.Info.GetUserDeptName(), ZipPswd: g.Pswd, Fcause: r.Cause, PkgNo: pkg,
		})
		cancel()
		if err := answer("", a, err); err != nil {
			fails = append(fails, err.Error())
			continue
		}
		sent += len(g.Names)
	}
	return sent, fails
}

// Pending 은 결재대상목록[3390] 이다.
func Pending(s *nbs.Session) ([]*apb.AprvFileInfo, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Approval().GetAprovalList(ctx, &apb.AprovalListRequest{Userid: s.UserID})
	if err != nil {
		return nil, rpcErr("결재 목록을 받지 못했습니다.", err)
	}
	return res.GetList(), nil
}

// Target 은 결재[3092] 한 건에 필요한 칸이다(결재대상목록 행에서 온다).
type Target struct {
	Seqno      int32  `json:"seqno"`
	ServerName string `json:"serverName"`
	Requester  string `json:"requester"`
	AprvRank   int32  `json:"aprvRank"`
}

// Outcome 은 결재 한 건의 결과다.
type Outcome struct {
	Seqno  int32  `json:"seqno"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason"`
}

// approveWorkers 는 결재[3092] 동시 건수다.
// ponytail: 동시 4건 고정(원본은 직렬 + 20건마다 1.5초 쉼). 서버 부하가 보이면 정책 키로 뺀다.
const approveWorkers = 4

// Approve 는 건마다 결재[3092] 를 보내고(동시 4건) 결과를 모아 준다. 성공하면 요청자에게 9004 kind 5 를 NotifyOnce 로 보낸다
// (요청자가 비거나 나 자신이면 안 보낸다, 응답은 버린다 ― 서버가 지금은 넘기지 않지만 고쳐지면 앱은 그대로 된다, 3.5절).
func Approve(s *nbs.Session, items []Target, approve bool, cause string) []Outcome {
	state, dat := int32(2), "N"
	if approve {
		state, dat = 1, "Y"
	}
	out := make([]Outcome, len(items))
	sem := make(chan struct{}, approveWorkers)
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			ctx, cancel := s.Ctx(nbs.RPCTimeout)
			defer cancel()
			res, err := s.Approval().Approve(ctx, &apb.ApprovalRequest{
				Seqno: it.Seqno, FileNameSvr: it.ServerName, FileUser: it.Requester, AprvCause: cause,
				AprvId: s.UserID, AprvUser: s.Info.GetUserName(), AprvState: state, AprvRank: it.AprvRank,
			})
			o := Outcome{Seqno: it.Seqno}
			switch {
			case err != nil:
				o.Reason = status.Convert(err).Message()
			case res.GetRes() != 1:
				o.Reason = res.GetReason()
			default:
				o.OK = true
				if it.Requester != "" && it.Requester != s.UserID {
					nctx, ncancel := s.Ctx(nbs.RPCTimeout)
					_, _ = s.Notify().NotifyOnce(nctx, &npb.NotifyMessage{NotifySvcno: 9004, Kind: 5, Side: 0, Userid: it.Requester, Dat: dat})
					ncancel()
				}
			}
			out[i] = o
		}()
	}
	wg.Wait()
	return out
}

// Proxy 는 등록된 대리결재자[3174] 다. alt_userid 가 비면 등록 없음.
func Proxy(s *nbs.Session) (*vpb.ProxyApproverResponse, error) {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	res, err := s.Approver().GetProxyApprover(ctx, &vpb.ProxyApproverRequest{Userid: s.UserID})
	if err != nil {
		return nil, rpcErr("대리결재자를 조회하지 못했습니다.", err)
	}
	return res, nil
}

// RegisterProxy 는 대리결재자 등록[3272] 이다. from·to 는 "YYYY-MM-DD".
func RegisterProxy(s *nbs.Session, altID, altName, from, to, cause string) error {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	a, err := s.Approver().RegisterProxyApprover(ctx, &vpb.ProxyApproverRegRequest{
		Userid: s.UserID, Username: s.Info.GetUserName(), AltUserid: altID, AltUsername: altName,
		ExpireFrom: ServerDate(from) + "000000", ExpireTo: ServerDate(to) + "235959", Cause: cause,
	})
	return answer("대리결재자를 등록하지 못했습니다.", a, err)
}

// DeleteProxy 는 대리결재자 삭제[3276] 다. 원본 QML 은 인자 순서가 바뀌어 사유 칸에 망 번호가 갔다 ― 여기서는 사유를 바로 싣는다.
func DeleteProxy(s *nbs.Session, altID, altName string) error {
	ctx, cancel := s.Ctx(nbs.RPCTimeout)
	defer cancel()
	a, err := s.Approver().DeleteProxyApprover(ctx, &vpb.ProxyApproverRegRequest{
		Userid: s.UserID, Username: s.Info.GetUserName(), AltUserid: altID, AltUsername: altName, Cause: "대리결재자 삭제",
	})
	return answer("대리결재자를 삭제하지 못했습니다.", a, err)
}

// FreeName 은 dir 안에서 겹치지 않는 이름이다: "a.txt" 가 있으면 "a (1).txt", "a (2).txt" …
func FreeName(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return p
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Stat(p); err != nil {
			return p
		}
	}
}
