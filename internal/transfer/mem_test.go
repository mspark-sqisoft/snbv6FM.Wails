package transfer

// 받기 메모리 측정. 가짜 서버를 다른 프로세스로 띄워(같은 프로세스면 서버 쪽 할당이 함께 잡힌다) 클라이언트 힙만 잰다.
// 실 서버에는 닿지 않는다(127.0.0.1 의 빈 포트). 크기는 SNB_MEM_MB 로 바꾼다.

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"snbapp/internal/nbs"
	"snbapp/pb/serverfile"
)

// TestMemHelperServer 는 측정용 가짜 서버 프로세스다. TestReceiveHeapBelowFile 이 SNB_MEM_SERVER_MB 를 주어 띄운다.
func TestMemHelperServer(t *testing.T) {
	mb, _ := strconv.Atoi(os.Getenv("SNB_MEM_SERVER_MB"))
	if mb == 0 {
		t.Skip("측정 시험이 띄우는 서버다")
	}
	body := make([]byte, mb<<20)
	for i := range body {
		body[i] = byte(i * 31)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer()
	serverfile.RegisterServerFileServiceServer(s, &fakeServer{data: body, checksum: sum(body), dnChunk: 8 << 20})
	fmt.Println("ADDR " + lis.Addr().String())
	s.Serve(lis)
}

// 프로젝트 제약 「전송 메모리 사용량 ≤ 파일 크기」: 서버처럼 8MiB 조각으로 받아도 힙 증가가 파일보다 작다.
// 힙은 파일이 커도 18MB 안팎으로 일정하다(30·100·300MB 측정). 그보다 작은 파일은 gRPC 가 조각을 모은 것과 합친 것 두 벌이라
// 파일의 약 2배가 된다 ― 이 판정은 18MB 넘는 파일에만 맞는다.
func TestReceiveHeapBelowFile(t *testing.T) {
	mb, _ := strconv.Atoi(os.Getenv("SNB_MEM_MB"))
	if mb == 0 {
		mb = 30
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMemHelperServer$")
	cmd.Env = append(os.Environ(), "SNB_MEM_SERVER_MB="+strconv.Itoa(mb))
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr := ""
	for sc := bufio.NewScanner(out); addr == "" && sc.Scan(); {
		addr, _ = strings.CutPrefix(sc.Text(), "ADDR ")
	}
	if addr == "" {
		t.Fatal("서버가 뜨지 않았다")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(nbs.NewCodec()), grpc.MaxCallRecvMsgSize(nbs.MaxRecvMsg)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := serverfile.NewServerFileServiceClient(conn)
	size := int64(mb) << 20
	dest := filepath.Join(t.TempDir(), "a.bin")
	growth, alloc := heapPeak(func() {
		_, err = Receive(context.Background(), c, DownloadSpec{ServerName: "S", Dest: dest, Size: size}, &Progress{})
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%dMB 받기: 힙 증가 최고 %.1fMB, 누적 할당 %.1fMB", mb, float64(growth)/(1<<20), float64(alloc)/(1<<20))
	if mb >= 18 && growth > size {
		t.Errorf("힙 증가 %d 가 파일 크기 %d 보다 크다", growth, size)
	}
}
