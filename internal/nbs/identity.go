package nbs

// 단말 식별(로그인 요청의 IP·MAC). 원본 FileMoverGrpc.cpp pickDeviceIdentity(docs/port/01 7.5).
// 서버가 계정에 등록된 IP·MAC 을 한 쌍으로 대조하므로(login_ip_check·login_mac_check) 둘을 같은 어댑터에서 고른다.

import (
	"net"
	"strings"
	"time"
)

// nic 은 어댑터 하나다(시험에서 가짜 목록을 넣으려고 net.Interface 에서 필요한 것만 옮긴다).
type nic struct {
	name  string
	flags net.Flags
	mac   net.HardwareAddr
	ips   []net.IP
}

// usable 은 단말 식별에 쓸 만한 어댑터인가다 ― 루프백이 아니고, 켜져 있고(Up·Running), MAC 이 있고, 이름이 가상(vm·virtual)이 아닌 것.
// MAC 이 없는 VPN 터널은 여기서 걸린다. Go 의 어댑터 이름은 Windows 에서 연결 이름("이더넷"·"VMware Network Adapter VMnet8")이다.
func usable(n nic) bool {
	lower := strings.ToLower(n.name)
	return n.flags&net.FlagLoopback == 0 && n.flags&net.FlagUp != 0 && n.flags&net.FlagRunning != 0 &&
		len(n.mac) > 0 && !strings.HasPrefix(lower, "vm") && !strings.HasPrefix(lower, "virtual")
}

func macText(m net.HardwareAddr) string {
	return strings.ToUpper(strings.NewReplacer(":", "", "-", "").Replace(m.String()))
}

// pickIdentity 는 1순위 서버로 나가는 경로의 IP(route)를 가진 쓸 만한 어댑터, 2순위 쓸 만한 어댑터 중 첫 IPv4(루프백·링크로컬 제외)다.
// 못 고르면 빈 값 둘. adapter 는 로그에 남길 어댑터 이름이다.
func pickIdentity(route net.IP, nics []nic) (ip, mac, adapter string) {
	if route != nil && !route.IsLoopback() {
		for _, n := range nics {
			if !usable(n) {
				continue
			}
			for _, a := range n.ips {
				if a.Equal(route) {
					return route.String(), macText(n.mac), n.name
				}
			}
		}
	}
	for _, n := range nics {
		if !usable(n) {
			continue
		}
		for _, a := range n.ips {
			if v4 := a.To4(); v4 != nil && !v4.IsLoopback() && !v4.IsLinkLocalUnicast() {
				return v4.String(), macText(n.mac), n.name
			}
		}
	}
	return "", "", ""
}

// routeIP 는 서버로 나갈 때 쓰는 지역 주소다. UDP 연결은 패킷을 보내지 않고 경로만 정한다. 0.2초 상한(원본과 같다).
func routeIP(addr string) net.IP {
	c, err := net.DialTimeout("udp", addr, 200*time.Millisecond)
	if err != nil {
		return nil
	}
	defer c.Close()
	if u, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return u.IP
	}
	return nil
}

func systemNICs() []nic {
	ifs, _ := net.Interfaces()
	out := make([]nic, 0, len(ifs))
	for _, it := range ifs {
		n := nic{name: it.Name, flags: it.Flags, mac: it.HardwareAddr}
		addrs, _ := it.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				n.ips = append(n.ips, ipn.IP)
			}
		}
		out = append(out, n)
	}
	return out
}

// deviceIdentity 는 addr(서버 host:port)로 가는 단말의 IP·MAC 이다.
func deviceIdentity(addr string) (ip, mac, adapter string) {
	return pickIdentity(routeIP(addr), systemNICs())
}
