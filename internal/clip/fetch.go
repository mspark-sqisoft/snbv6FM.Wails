package clip

// 서식 클립보드의 그림 모으기: 로컬 파일과 원격 그림을 받아 묶음 항목으로 만든다(원본 GetImage·GetFile).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	fetchWorkers  = 6
	fetchTimeout  = 15 * time.Second // 전체 시한. 원본은 그림마다 5초를 차례로 기다렸다
	fetchImageMax = 20 << 20
	redirectMax   = 5
)

var errBlocked = errors.New("막는 주소다")

// fetchClient 는 이름을 풀어 주소마다 본 뒤 그 주소로 직접 연결한다 ― 판정과 연결 사이에 이름이 바뀌는 틈(DNS 재바인딩)을 없앤다.
func fetchClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil, // 프록시로 나가면 막는 주소 판정을 비켜 간다
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				var addrs []netip.Addr
				if a, ok := parseHostIP(host); ok {
					addrs = []netip.Addr{a}
				} else if addrs, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host); err != nil {
					return nil, err
				}
				if !ResolvedAllowed(addrs) {
					return nil, errBlocked
				}
				var last error
				for _, a := range addrs {
					c, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
					if err == nil {
						return c, nil
					}
					last = err
				}
				return nil, last
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= redirectMax {
				return errors.New("리디렉션이 너무 많다")
			}
			if RemoteRef(req.URL).Kind != RefRemote {
				return errBlocked
			}
			return nil
		},
	}
}

func fetchRemote(ctx context.Context, cl *http.Client, u *url.URL, limit int64) ([]byte, Drop) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, DropNotFetched
	}
	res, err := cl.Do(req)
	if err != nil {
		if errors.Is(err, errBlocked) {
			return nil, DropBlocked
		}
		return nil, DropNotFetched
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, DropNotFetched
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, DropNotFetched
	}
	if !LooksLikeImage(data) { // 받은 바이트도 그림이어야 싣는다
		return nil, DropNotImage
	}
	return data, 0
}

func readLocal(p string, limit int64) ([]byte, Drop) {
	f, err := os.Open(p)
	if err != nil {
		return nil, DropNotFetched
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	if !LocalImageOK(p, head[:n]) {
		return nil, DropNotImage
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, DropNotFetched
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, DropNotFetched
	}
	return data, 0
}

// CollectImages 는 refs 의 그림을 동시에 모은다. 못 실은 것은 dropped 에 사유별로 더한다. limit 은 그림 한 장 상한(0 이면 20MB).
func CollectImages(ctx context.Context, refs []SendRef, limit int64, dropped *DropCounts) []Entry {
	if limit <= 0 || limit > fetchImageMax {
		limit = fetchImageMax
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	cl := fetchClient()
	defer cl.CloseIdleConnections()
	got := make([][]byte, len(refs))
	why := make([]Drop, len(refs))
	sem := make(chan struct{}, fetchWorkers)
	var wg sync.WaitGroup
	for i, r := range refs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if r.Remote {
				got[i], why[i] = fetchRemote(ctx, cl, r.URL, limit)
			} else {
				got[i], why[i] = readLocal(r.Local, limit)
			}
		}()
	}
	wg.Wait()
	var out []Entry
	for i, b := range got {
		if b == nil {
			dropped[why[i]]++
			continue
		}
		out = append(out, Entry{Name: refs[i].Name, Data: b})
	}
	return out
}

// DropLog 는 보낼 때 뺀 참조를 사유별로 적은 로그 줄이다. 없으면 "". 주소·경로·파일 이름은 넣지 않는다.
func DropLog(d DropCounts) string {
	n := 0
	for _, v := range d {
		n += v
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("클립보드 HTML 보내기 >>> 참조 제외 %d건(그림 아님 %d, 막는 주소 %d, 허용 안 된 스킴 %d, 개수 초과 %d, 묶지 않는 속성 %d, 받지 못함 %d)",
		n, d[DropNotImage], d[DropBlocked], d[DropScheme], d[DropOverLimit], d[DropNotBundled], d[DropNotFetched])
}

// PackHTML 은 CF_HTML 바이트로 서식 묶음을 만든다(원본 EditHtml·GetImage·CombineFiles). stamp 는 묶음 항목 이름에 들어가는 시각(초)이다.
// 돌려준 로그 줄이 비어 있지 않으면 부른 쪽이 앱 로그에 남긴다.
func PackHTML(ctx context.Context, cfhtml []byte, stamp int64) ([]byte, string, error) {
	html, source := ParseCFHTML(cfhtml)
	return PackHTMLText(ctx, html, source, stamp)
}

// PackHTMLText 는 PackHTML 의 HTML 글자 판이다(시험과 실 서버 확인이 부른다).
func PackHTMLText(ctx context.Context, html, source string, stamp int64) ([]byte, string, error) {
	html = strings.ReplaceAll(html, "&#xA;", "")
	sent := PrepareSendHTML(html, source, virtualRoot, stamp, ImgMaxCnt)
	images := CollectImages(ctx, sent.Refs, 0, &sent.Dropped)
	b, err := BuildBundle([]byte(sent.HTML), images)
	return b, DropLog(sent.Dropped), err
}
