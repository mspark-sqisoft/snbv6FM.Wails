// Package nbs 는 NBS 서버와의 gRPC 연결·세션·로그인이다. 원본은 snBv6FM 의 imports/FileMoverGrpc(FileMoverGrpc.cpp)이고,
// 같은 흐름을 짧게 옮긴 tools/snb-mcp/server.py 를 함께 대조했다. 서비스별 RPC 감싸기는 영역마다 이 폴더의 다른 파일에 둔다.
package nbs

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"snbapp/internal/config"
	"snbapp/pb/approval"
	"snbapp/pb/approver"
	"snbapp/pb/commonservice"
	"snbapp/pb/fileextension"
	"snbapp/pb/login"
	"snbapp/pb/notify"
	"snbapp/pb/serverfile"
)

// RPCTimeout 은 단일 요청 하나의 상한이다(원본 callRpc 의 RPC_TIMEOUT). 스트림(업로드·받기·알림)에는 걸지 않는다.
const RPCTimeout = 15 * time.Second

// 결재 방식(로그인 응답 fm_*_aprv): 0 자동, 1 사전, 2 확장자, 3 사후, 4 요청시 사전사후결정, 5 요청시 결재생략결정, 6 자동자가, 7 자동·사전·사후.
const (
	AprvAuto = 0
	AprvPre  = 1
	AprvPost = 3
	AprvSelf = 6
)

// Session 은 로그인 한 번의 연결이다. 서버에 Logout RPC 가 없어 Close 로 TCP 를 끊어야 서버가 세션을 회수한다.
// 한 번에 망 하나만 로그인한다(원본과 같다).
type Session struct {
	conn  *grpc.ClientConn
	codec *cryptCodec
	token string

	Server  config.Server        // 접속한 망(fmenv.ini 한 칸)
	Addr    string               // 실제 접속한 host:port
	UserID  string               // 서버가 돌려준 ID(비면 보낸 ID)
	Info    *login.LoginResponse // 로그인 응답 전체(권한·결재 방식 등)
	Profile map[string]string    // 권한 프로파일(GetProfile 'k=v,k=v'). 없는 키는 없는 것으로 둔다(v, ok := Profile[k])

	EncMode         int32  // PreLogin 응답의 암호 모드(TLS 서버는 0)
	IP, MAC         string // 로그인에 실은 단말 식별(MAC 은 구분자 없는 대문자 12자)
	MultiLogin      bool   // fm_login_multi==0 && multi_login_flag!=0 ― 다른 PC 에 이미 로그인(AUTH-024)
	RemainDays      int    // pswd_remain_day: 0 확인 안 함, 양수 만료 예정, 음수 기한 지남
	DefaultPassword bool   // 로그인 비밀번호가 정책 default_pw 와 같다(AUTH-044)
}

// NewTestSession 은 가짜 서버 연결로 세션을 만든다. 다른 패키지의 시험이 쓰고 앱 코드는 쓰지 않는다.
func NewTestSession(conn *grpc.ClientConn) *Session { return &Session{conn: conn} }

// Close 는 연결(TCP)을 닫는다 = 서버 로그아웃. 두 번 불러도 된다.
func (s *Session) Close() error { return s.conn.Close() }

// Ctx 는 auth-token 메타데이터를 실은 컨텍스트다. d 가 0 이면 시간 상한 없이 취소만 된다(스트림용).
func (s *Session) Ctx(d time.Duration) (context.Context, context.CancelFunc) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "auth-token", s.token)
	if d == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// 서비스별 gRPC 클라이언트. 만드는 비용이 거의 없어 부를 때마다 새로 만든다.
func (s *Session) Login() login.LoginServiceClient { return login.NewLoginServiceClient(s.conn) }
func (s *Session) Common() commonservice.CommonServiceClient {
	return commonservice.NewCommonServiceClient(s.conn)
}
func (s *Session) ServerFile() serverfile.ServerFileServiceClient {
	return serverfile.NewServerFileServiceClient(s.conn)
}
func (s *Session) Approval() approval.ApprovalServiceClient {
	return approval.NewApprovalServiceClient(s.conn)
}
func (s *Session) Approver() approver.ApproverServiceClient {
	return approver.NewApproverServiceClient(s.conn)
}
func (s *Session) FileExtension() fileextension.FileExtensionServiceClient {
	return fileextension.NewFileExtensionServiceClient(s.conn)
}
func (s *Session) Notify() notify.NotifyServiceClient { return notify.NewNotifyServiceClient(s.conn) }

// IsBusiness 는 업무망(LOC 1)에 로그인했는가다. 거짓이면 인터넷망.
func (s *Session) IsBusiness() bool { return s.Server.Loc == config.LocBusiness }

// AprvType 은 이 망에서 올릴 때의 결재 방식이다(파일 또는 클립보드).
func (s *Session) AprvType(clip bool) int32 {
	switch {
	case s.IsBusiness() && clip:
		return s.Info.FmInclupAprv
	case s.IsBusiness():
		return s.Info.FmInflupAprv
	case clip:
		return s.Info.FmOtclupAprv
	}
	return s.Info.FmOtflupAprv
}

// UserInfo 는 화면에 보이는 사용자 정보다.
type UserInfo struct {
	UserID          string `json:"userId"`
	UserName        string `json:"userName"`
	Dept            string `json:"dept"`     // 부서 이름
	DeptCode        string `json:"deptCode"` // user_dept2 가 있으면 그것
	Duty            string `json:"duty"`     // 직위
	Rank            string `json:"rank"`     // 직급
	IsApprover      bool   `json:"isApprover"`
	IsProxyApprover bool   `json:"isProxyApprover"`
	LogoutAutoMin   int    `json:"logoutAutoMin"` // 자동 로그아웃(분). 0 = 끄지 않음
}

// User 는 로그인 응답에서 뽑은 사용자 정보다.
func (s *Session) User() UserInfo {
	i := s.Info
	code := i.UserDept2
	if code == "" {
		code = i.UserDept
	}
	return UserInfo{
		UserID: s.UserID, UserName: i.UserName, Dept: i.UserDeptName, DeptCode: code,
		Duty: i.UserDutyName, Rank: i.UserDeptRank,
		IsApprover: i.AprvYn, IsProxyApprover: i.ReplaceYn, LogoutAutoMin: int(i.FmLogoutAuto),
	}
}
