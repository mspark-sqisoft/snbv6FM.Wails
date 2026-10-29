package main

// NBS 서버와의 gRPC 통신. 원본은 snBv6FM 의 imports/FileMoverGrpc(FileMoverGrpc.cpp·FileTransfer.cpp)이고,
// 같은 흐름을 짧게 옮긴 tools/snb-mcp/server.py 를 함께 대조했다.

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"snbapp/pb/commonservice"
	"snbapp/pb/login"
	"snbapp/pb/serverfile"
)

const (
	rpcTimeout  = 15 * time.Second // 원본 callRpc 의 RPC_TIMEOUT
	uploadChunk = 512 * 1024       // 원본 UPLOAD_STREAM_CHUNK_SIZE
	partSuffix  = ".snbpart"
)

// Server 는 fmenv.ini 의 [SERVERxx] 한 칸이다.
type Server struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port string `json:"port"`
	Loc  string `json:"loc"` // "1" 업무망, "0" 인터넷망
}

// readServers 는 fmenv.ini 에서 [SERVERxx] 섹션만 읽는다.
func readServers(path string) ([]Server, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Server
	cur := -1
	for _, line := range strings.Split(strings.TrimPrefix(string(b), "\xEF\xBB\xBF"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || line[0] == '#' || line[0] == ';':
		case line[0] == '[':
			cur = -1
			if strings.HasPrefix(line, "[SERVER") {
				out = append(out, Server{Port: "29002"})
				cur = len(out) - 1
			}
		case cur >= 0:
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "IP":
				out[cur].IP = v
			case "PORT":
				out[cur].Port = v
			case "NAME":
				out[cur].Name = v
			case "LOC":
				out[cur].Loc = v
			}
		}
	}
	return out, nil
}

// Session 은 로그인 한 번의 연결이다. 서버에 Logout RPC 가 없어 Close 로 TCP 를 끊어야 세션이 회수된다.
type Session struct {
	conn    *grpc.ClientConn
	token   string
	UserID  string
	Server  Server
	Info    *login.LoginResponse
	Profile map[string]string // 권한 프로파일(GetProfile) 'k=v,k=v'
}

func (s *Session) Close() { s.conn.Close() }

func (s *Session) ctx(d time.Duration) (context.Context, context.CancelFunc) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "auth-token", s.token)
	if d == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// Login 은 PreLogin → Login → GetProfile 을 부른다. 화면에 내는 오류 문구는 원본 FileMoverGrpc::login() 과 같다 ―
// 계정 실패는 사유를 가리지 않고 한 문구로 알리고(없는 ID 를 알아낼 수 없게), 사유는 로그에만 남긴다.
func Login(appDir string, srv Server, id, pw, version string) (*Session, error) {
	pem, err := os.ReadFile(filepath.Join(appDir, "root_ca.pem"))
	if err != nil {
		return nil, fmt.Errorf("root_ca.pem 을 읽지 못했습니다: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("root_ca.pem 에 인증서가 없습니다")
	}
	addr := net.JoinHostPort(srv.IP, srv.Port)
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: pool})),
		// 서버 keepalive MinTime 이 10초라 30초 ping 은 허용된다(snb-mcp 와 같다).
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}))
	if err != nil {
		return nil, err
	}
	s := &Session{conn: conn, Server: srv}
	if err := s.login(id, pw, version); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) login(id, pw, version string) error {
	ctx, cancel := s.ctx(rpcTimeout)
	defer cancel()
	pre, err := login.NewLoginServiceClient(s.conn).PreLogin(ctx, &login.PreLoginRequest{Userid: id, Msg: "[START]"})
	if err != nil {
		log.Printf("사전로그인 실패: %v", err)
		return errors.New("서버에 연결할 수 없습니다.")
	}
	if pre.EncMode != 0 {
		// ponytail: TLS 만 쓰는 서버(암호 모드 0)만 지원한다. 다른 모드는 FMEncryption(npcf DLL)의 메시지 암호화를 옮겨야 한다.
		return fmt.Errorf("서버 암호 모드 %d 는 아직 지원하지 않습니다(0 만 지원)", pre.EncMode)
	}
	s.token = pre.Token
	if pre.Id != "" {
		id = pre.Id
	}

	ip, mac := deviceIdentity(net.JoinHostPort(s.Server.IP, s.Server.Port))
	if mac == "" {
		log.Printf("단말 식별 실패 ― 서버 정책에 따라 로그인이 거부될 수 있다(IP %q)", ip)
	}
	inOut := "2"
	if s.Server.Loc == "1" {
		inOut = "1"
	}
	ctx, cancel = s.ctx(rpcTimeout)
	defer cancel()
	res, err := login.NewLoginServiceClient(s.conn).Login(ctx, &login.LoginRequest{
		UserId: id, Passwd: pw, InOut: inOut,
		UserIpIn: ip, UserIpOut: ip, UserMacIn: mac, UserMacOut: mac,
		Otp: 8888, FirstLogin: 337, Cancel_9003: "1", // 원본과 같은 값(최초로그인 표시, 9003 알림 안 씀)
		AppVersion: version,
	})
	if err != nil {
		st := status.Convert(err)
		log.Printf("로그인 실패: %s %s", st.Code(), st.Message())
		switch {
		case st.Code() != codes.Unauthenticated:
			return errors.New("서버에 연결할 수 없습니다.")
		case strings.Contains(st.Message(), "LICENSE_ERROR"):
			return errors.New("라이선스가 만료되었거나 사용할 수 없습니다.")
		case strings.Contains(st.Message(), "CONNECTION_SIDE_ERROR"):
			return errors.New("접속한 서버의 망 구분이 설정과 다릅니다.")
		}
		return errors.New("로그인에 실패하였습니다.")
	}
	s.Info = res
	s.UserID = res.Userid
	if s.UserID == "" {
		s.UserID = id
	}
	// 업로드 방식(fm_upfile_mode)·검사값 사용(fm_checksum)은 로그인 응답이 아니라 프로파일에 있다(원본 setProfile2).
	ctx, cancel = s.ctx(rpcTimeout)
	defer cancel()
	prof, err := commonservice.NewCommonServiceClient(s.conn).GetProfile(ctx, &commonservice.ProfileRequest{Userid: s.UserID})
	if err != nil {
		return fmt.Errorf("권한 프로파일을 받지 못했습니다: %s", status.Convert(err).Message())
	}
	s.Profile = map[string]string{}
	for _, kv := range strings.Split(prof.Profile, ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			s.Profile[k] = v
		}
	}
	log.Printf("로그인 성공: %s %s @ %s (IP %s MAC %s)", s.UserID, res.UserName, s.Server.IP, ip, mac)
	return nil
}

// deviceIdentity 는 서버로 나가는 경로의 어댑터에서 IP 와 MAC 을 함께 고른다. 서버가 계정에 등록된 IP·MAC 쌍을
// 대조하므로 둘을 다른 어댑터에서 고르면 거부된다(원본 pickDeviceIdentity 의 1순위).
// ponytail: 경로를 못 정할 때의 2순위(가상 어댑터를 거른 첫 실물 어댑터)는 옮기지 않았다 ― 서버에 못 붙으면 어차피 로그인이 안 된다.
func deviceIdentity(addr string) (ip, mac string) {
	c, err := net.Dial("udp", addr) // UDP 는 패킷을 보내지 않고 경로만 정한다
	if err != nil {
		return "", ""
	}
	defer c.Close()
	ip = c.LocalAddr().(*net.UDPAddr).IP.String()
	ifs, _ := net.Interfaces()
	for _, it := range ifs {
		addrs, _ := it.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.String() == ip {
				return ip, strings.ToUpper(strings.ReplaceAll(it.HardwareAddr.String(), ":", ""))
			}
		}
	}
	return ip, ""
}

// Files 는 서버 파일 목록이다(받을 파일·올린 파일 모두).
func (s *Session) Files() ([]*serverfile.FileInfo, error) {
	ctx, cancel := s.ctx(rpcTimeout)
	defer cancel()
	res, err := serverfile.NewServerFileServiceClient(s.conn).GetServerFileList(ctx, &serverfile.ServerFileListRequest{
		Userid: s.UserID, FromCnt: 1000})
	if err != nil {
		return nil, fmt.Errorf("서버 파일 목록을 받지 못했습니다: %s", status.Convert(err).Message())
	}
	return res.List, nil
}

// aprvType 은 이 망에서 파일을 올릴 때의 결재 방식이다(0 자동승인, 1 사전결재, 3 사후결재, 6 자가승인).
func (s *Session) aprvType() int32 {
	if s.Server.Loc == "1" {
		return s.Info.FmInflupAprv
	}
	return s.Info.FmOtflupAprv
}

// Upload 는 파일 하나를 올린다. 결재자를 업로드 때 정해야 하는 계정(fm_upfile_mode 0, 자동·자가승인이 아님)은 아직 지원하지 않는다.
func (s *Session) Upload(path string, progress func(done, total int64)) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return errors.New("폴더 업로드는 아직 지원하지 않습니다")
	}
	aprv := s.aprvType()
	var approvers []string
	switch {
	case aprv == 6:
		approvers = []string{s.UserID} // 자가승인(원본 ViewControllerNotifySlot)
	case s.Profile["fm_upfile_mode"] != "1" && aprv != 0:
		return fmt.Errorf("이 계정은 업로드할 때 결재자를 정해야 합니다(결재 방식 %d) ― 결재자 선택은 아직 지원하지 않습니다", aprv)
	}
	resv, kind := "2", int32(0)
	if len(approvers) > 0 {
		resv = "1"
	}
	if aprv == 3 {
		kind = 3
	}
	sum := ""
	if s.Profile["fm_checksum"] != "0" { // 키가 없으면 켠 것(원본 PolicySet::useChecksum)
		if sum, err = sha256File(path); err != nil {
			return err
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// 업로드는 크기에 따라 오래 걸리므로 시간 상한을 두지 않는다(원본도 스트림에는 RPC_TIMEOUT 을 걸지 않는다).
	ctx, cancel := s.ctx(0)
	defer cancel() // 도중에 실패하면 취소한다 ― CloseSend 로 끝내면 서버가 잘린 파일을 정상으로 커밋한다(서버요청 SR002)
	stream, err := serverfile.NewServerFileServiceClient(s.conn).FileUpload(ctx)
	if err != nil {
		return err
	}
	info := &serverfile.FileTransInfo{
		Fsize: uint64(st.Size()), Fpath: filepath.Base(path), ApproverId: approvers,
		AprvKind: kind, AprvType: aprv, AprvDept: s.Info.UserDeptName, Checksum: sum, Resv: resv,
	}
	if err := stream.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Info{Info: info}}); err != nil {
		return uploadErr(stream, err)
	}
	buf := make([]byte, uploadChunk)
	var done int64
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			if err := stream.Send(&serverfile.FileUploadRequest{Data: &serverfile.FileUploadRequest_Chunk{Chunk: buf[:n]}}); err != nil {
				return uploadErr(stream, err)
			}
			done += int64(n)
			progress(done, st.Size())
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr // defer cancel() 이 스트림을 취소한다
		}
	}
	if done != st.Size() {
		return fmt.Errorf("읽은 크기가 다릅니다: %d / %d 바이트 ― 올리는 중에 파일이 바뀌었습니다", done, st.Size())
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	for {
		a, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("업로드 실패: %s", status.Convert(err).Message())
		}
		if a.Res != 1 {
			return fmt.Errorf("서버가 업로드를 거부했습니다: %s (code %d)", a.Reason, a.Code)
		}
	}
}

// uploadErr 는 Send 실패의 진짜 까닭을 꺼낸다 ― 서버가 스트림을 닫으면 Send 는 io.EOF 만 주고 사유는 Recv 에 있다.
func uploadErr(stream serverfile.ServerFileService_FileUploadClient, err error) error {
	if err == io.EOF {
		for {
			a, rerr := stream.Recv()
			if rerr != nil {
				err = rerr
				break
			}
			if a.Res != 1 {
				return fmt.Errorf("서버가 업로드를 거부했습니다: %s (code %d)", a.Reason, a.Code)
			}
		}
	}
	return fmt.Errorf("업로드 실패: %s", status.Convert(err).Message())
}

// downloadBlock 은 [다운로드] 를 막아야 하면 그 까닭을 준다(원본 src/DownloadBlock.cpp). 서버 FileDownload 는 결재 상태를
// 보지 않고 경로만으로 내주므로, 여기서 막지 않으면 결재 전 파일을 받고 RemoveFile 로 결재 요청까지 지운다.
func downloadBlock(f *serverfile.FileInfo, loc string, profile map[string]string) string {
	switch {
	case f.DirFlag == 1:
		return "폴더(압축으로 올라온 것)는 아직 받지 않습니다"
	case strings.Contains(f.NameStr, "__CLIP__"):
		return "클립보드로 올린 것은 파일로 받을 수 없습니다"
	case profile["down_same_network"] == "0" && strconv.Itoa(int(f.Course)) == loc:
		return "동일망에서 올린 파일은 받을 수 없습니다 ― 반대 망으로 로그인하세요"
	case f.VirusFlag == 0 || f.VirusFlag == 3:
		return "바이러스 검사가 끝나지 않았습니다"
	case (f.VirusFlag != 1 && f.VirusFlag != 4) || f.State != 1:
		return "받을 수 있는 상태가 아닙니다(승인된 파일만 받습니다)"
	}
	return ""
}

// Download 는 서버 파일 하나를 dir 에 PC 파일 이름으로 받고, 받은 뒤 서버 목록에서 지운다(원본 [다운로드] 와 같다).
func (s *Session) Download(serverName, dir string, progress func(done, total int64)) (string, error) {
	files, err := s.Files()
	if err != nil {
		return "", err
	}
	var f *serverfile.FileInfo
	for _, x := range files {
		if x.ServerStr == serverName {
			f = x
		}
	}
	if f == nil {
		return "", fmt.Errorf("서버 파일 목록에 없습니다: %s", serverName)
	}
	if why := downloadBlock(f, s.Server.Loc, s.Profile); why != "" {
		return "", errors.New(why)
	}
	// ponytail: 같은 이름이 있으면 받지 않는다. 원본의 덮어쓰기·이름 바꾸기 선택(DownloadConflict)은 다음 단계.
	dest := filepath.Join(dir, filepath.Base(f.NameStr))
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("같은 이름이 이미 있습니다: %s", dest)
	}
	part := dest + partSuffix
	defer os.Remove(part) // 성공하면 이미 이름이 바뀌어 지울 것이 없다

	ctx, cancel := s.ctx(0)
	defer cancel()
	stream, err := serverfile.NewServerFileServiceClient(s.conn).FileDownload(ctx, &serverfile.FileDownloadRequest{Spath: serverName})
	if err != nil {
		return "", err
	}
	out, err := os.Create(part)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	var got int64
	checksum := ""
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			out.Close()
			return "", fmt.Errorf("받기 실패: %s", status.Convert(err).Message())
		}
		if a := r.GetAnswer(); a != nil {
			if a.Res != 1 {
				out.Close()
				return "", fmt.Errorf("서버가 받기를 거부했습니다: %s (code %d)", a.Reason, a.Code)
			}
			if a.Code == 3 { // 받기의 검사값(원본 ChecksumVerdict.h). 옛 서버는 code 0 으로 다른 글을 싣는다
				checksum = a.Reason
			}
			continue
		}
		n, err := w.Write(r.GetChunk())
		if err != nil {
			out.Close()
			return "", err
		}
		got += int64(n)
		progress(got, int64(f.FileSize))
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	// 스트림은 서버 원본이 비었거나 도중에 끊겨도 정상 종료한다(원본 DOWNLOAD_SIZE_MISMATCH)
	if got != int64(f.FileSize) {
		return "", fmt.Errorf("받은 크기가 다릅니다: %d / %d 바이트", got, f.FileSize)
	}
	if actual := hex.EncodeToString(h.Sum(nil)); checksum != "" && !strings.EqualFold(checksum, actual) {
		return "", fmt.Errorf("검사값이 다릅니다: 서버 %s / 받은 것 %s", checksum, actual)
	}
	if err := os.Rename(part, dest); err != nil {
		return "", err
	}
	rctx, rcancel := s.ctx(rpcTimeout)
	defer rcancel()
	if _, err := serverfile.NewServerFileServiceClient(s.conn).RemoveFile(rctx, &serverfile.FileRemoveRequest{Spath: serverName, Flag: 1}); err != nil {
		log.Printf("받은 뒤 서버 목록에서 지우지 못했다: %s %v", serverName, err)
	}
	return dest, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil // 원본 calculateFileHash(...).toHex() 와 같은 소문자 16진
}
