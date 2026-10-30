package nbs

import (
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"snbapp/internal/config"
	"snbapp/internal/nbs/crypt"
	"snbapp/pb/commonservice"
	"snbapp/pb/login"
)

const maxIDPWBytes = 31 // 원본 고정 칸 32바이트(끝 NUL 포함). UTF-8 바이트로 센다

// MaxRecvMsg 는 모든 RPC 의 응답 한 개 상한이다. 서버는 파일을 8MiB 조각으로 보내고 gRPC 기본 상한은 4MiB 라,
// 올리지 않으면 4MiB 넘는 파일은 첫 조각에서 ResourceExhausted 로 끊긴다(실 서버에서 확인).
const MaxRecvMsg = 64 << 20

// CheckLength 는 ID·비밀번호가 UTF-8 31바이트 안인가다(넘으면 MsgTooLong, 서버에 보내지 않는다).
func CheckLength(id, pw string) error {
	if len(id) > maxIDPWBytes || len(pw) > maxIDPWBytes {
		return loginErr(MsgTooLong)
	}
	return nil
}

// Login 은 PreLogin → Login → GetProfile 을 부르고 세션을 돌려준다(docs/port/01 3.1~3.4). 실패는 *LoginError(화면 문구)다.
// 사유는 로그에만 남긴다. 비밀번호는 로그에 길이만 남기고, 기본 비밀번호 판단(AUTH-044)에만 쓴 뒤 쥐지 않는다.
func Login(appDir string, srv config.Server, id, pw, version string, log *slog.Logger) (*Session, error) {
	if err := CheckLength(id, pw); err != nil {
		return nil, err
	}
	pem, err := os.ReadFile(filepath.Join(appDir, "root_ca.pem"))
	if err != nil {
		log.Error("root_ca.pem 을 읽지 못했다", "err", err)
		return nil, loginErr(MsgNoCert)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) { // PEM 블록을 전부 읽는다(업무망·인터넷망 CA 둘)
		log.Error("root_ca.pem 에 인증서가 없다")
		return nil, loginErr(MsgNoCert)
	}
	if len(srv.IPs) == 0 || srv.Port <= 0 {
		log.Error("서버 주소가 없다", "index", srv.Index, "ip", srv.IP, "port", srv.Port)
		return nil, channelErr(srv.Index)
	}
	// ponytail: `_` 로 적힌 여러 IP 중 첫 것만 쓴다. 대체 접속(09 8.3)은 그 영역이 넣는다.
	addr := net.JoinHostPort(srv.IPs[0], strconv.Itoa(srv.Port))
	codec := newCryptCodec()
	// TLS 만 쓴다(SEC-015·018) ― 운영체제 신뢰 목록을 쓰지 않고, 검증을 끄는 길이 없다.
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})),
		// 서버 keepalive MinTime 이 10초라 30초 ping 은 허용된다(snb-mcp 와 같다).
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(codec), grpc.MaxCallRecvMsgSize(MaxRecvMsg)))
	if err != nil {
		log.Error("gRPC 채널을 만들지 못했다", "addr", addr, "err", err)
		return nil, channelErr(srv.Index)
	}
	s := &Session{conn: conn, codec: codec, Server: srv, Addr: addr}
	if err := s.login(id, pw, version, log); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) login(id, pw, version string, log *slog.Logger) error {
	log.Info("LOGIN TRY >>> ID[" + id + "] PW[*** len=" + strconv.Itoa(len(pw)) + "]")

	ctx, cancel := s.Ctx(RPCTimeout)
	defer cancel()
	pre, err := s.Login().PreLogin(ctx, &login.PreLoginRequest{Userid: id, Msg: "[START]"})
	if err != nil {
		if isHandshakeErr(err) {
			log.Error("[gRPC] 서버 인증서 검증 실패: " + status.Convert(err).Message() + " | host:" + s.Addr)
			return loginErr(MsgUntrusted)
		}
		log.Error("사전로그인 실패", "err", status.Convert(err).Message())
		return loginErr(MsgPreLoginFail)
	}
	log.Info("사전로그인 응답 >>> 암호모드: " + strconv.Itoa(int(pre.EncMode)))
	c, err := crypt.New(pre.EncMode, crypt.Params{
		SvcID: int(pre.EnSvcId), SvcNo: int(pre.EnSvcNo), SvcPort: int(pre.EnSvcPort),
		MyIP: pre.EnMyIp, SvcIP: pre.EnSvcIp, SiteAbb: pre.EnSiteAbb,
	})
	if err != nil {
		log.Error("암호 키를 만들지 못했다", "mode", pre.EncMode, "err", err)
		return loginErr(err.Error())
	}
	s.codec.c.Store(c) // 이후 모든 전문을 이 모드로(모드 0 이면 nil)
	s.EncMode = pre.EncMode
	s.token = pre.Token
	if pre.Id != "" {
		id = pre.Id // 서버가 공백을 뺀 실제 ID
	}

	ip, mac, adapter := deviceIdentity(s.Addr)
	if ip == "" || mac == "" {
		log.Warn("단말 식별 실패 >>> 서버 정책에 따라 로그인이 거부될 수 있다", "ip", ip)
	} else {
		log.Info("단말 식별 >>> " + adapter + " " + ip + " " + mac)
	}
	s.IP, s.MAC = ip, mac
	inOut := "2"
	if s.IsBusiness() {
		inOut = "1"
	}
	ctx, cancel = s.Ctx(RPCTimeout)
	defer cancel()
	res, err := s.Login().Login(ctx, &login.LoginRequest{
		UserId: id, Passwd: pw, InOut: inOut,
		UserIpIn: ip, UserIpOut: ip, UserMacIn: mac, UserMacOut: mac,
		Otp: 8888, FirstLogin: 337, Cancel_9003: "1", // 원본과 같은 값(최초로그인 표시, 9003 알림 안 씀)
		AppVersion: version,
	})
	if err != nil {
		st := status.Convert(err)
		log.Warn("로그인 실패 상세(화면에는 내지 않는다) >>> " + st.Code().String() + " " + st.Message())
		return loginFailure(err)
	}
	s.Info = res
	s.UserID = res.Userid
	if s.UserID == "" {
		s.UserID = id
	}
	s.MultiLogin = res.FmLoginMulti == 0 && res.MultiLoginFlag != 0
	s.RemainDays = remainDays(res.PswdRemainDay)

	// 원본은 정책 실패 때 `접속 중..` 에 멈춘다 ― 여기서는 15초 상한 뒤 연결 실패로 돌린다(docs/port/01 8장).
	ctx, cancel = s.Ctx(RPCTimeout)
	defer cancel()
	prof, err := s.Common().GetProfile(ctx, &commonservice.ProfileRequest{Userid: s.UserID})
	if err != nil {
		log.Error("fm_profile2 목록 조회[3007] 실패!!!", "err", status.Convert(err).Message())
		return loginErr(MsgConnect)
	}
	s.Profile = ParseProfile(prof.Profile)
	s.DefaultPassword = isDefaultPassword(s.Profile["default_pw"], pw)
	_, set := s.Profile["default_pw"]
	log.Info("기본PW[" + pick(set, "set", "none") + "] PW[*** len=" + strconv.Itoa(len(pw)) +
		"] 같음[" + pick(s.DefaultPassword, "Y", "N") + "]")
	log.Info("비밀번호 변경 잔여일 >>> " + strconv.Itoa(s.RemainDays) + " (0:체크안함, 음수:기한초과)")
	log.Info("로그인 완료 >>> msg[LOGIN SUCCESS]", "user", s.UserID, "name", res.UserName, "server", s.Addr, "multi", s.MultiLogin)
	return nil
}

// isDefaultPassword 는 정책 default_pw 와 로그인 비밀번호가 같은가다. 빈 값끼리는 같다고 보지 않는다.
func isDefaultPassword(def, pw string) bool {
	return def != "" && subtle.ConstantTimeCompare([]byte(def), []byte(pw)) == 1
}

func pick(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

// remainDays 는 로그인 응답 pswd_remain_day(문자열)다. 숫자가 아니면 0(확인 안 함).
func remainDays(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// ParseProfile 은 'k=v,k=v' 를 쉼표로 나누고 첫 `=` 로 가른다(값에 쉼표가 있으면 잘린다 ― 원본과 같다).
func ParseProfile(p string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(p, ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}
