package policy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Row 는 [파일 검사 결과] 창의 한 행이다. Top 이면 위 표(파일·폴더 하나), 아니면 아래 [내부 파일 검사 결과] 표다.
// 같은 Key 로 다시 오면 그 행을 바꾼다. 표는 Order 순으로 놓고 # 는 그 순번이다.
type Row struct {
	Key    string `json:"key"`
	Top    bool   `json:"top"`
	Order  int64  `json:"order"`
	Parent string `json:"parent"` // 아래 표: 압축·폴더 이름
	Name   string `json:"name"`   // 위 표: 파일·폴더 이름, 아래 표: 내부 파일 경로
	Result string `json:"result"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

// Hooks 는 검사가 밖에 알리고 묻는 자리다. 모두 nil 이어도 된다(AskPassword 가 nil 이면 입력 취소로 본다).
// 여러 작업자가 부르므로 Rows·Progress·Log 는 동시에 불려도 되어야 한다. AskPassword 는 한 번에 하나만 불린다.
type Hooks struct {
	Rows        func([]Row)
	Progress    func(done, total int, current string)
	AskPassword func(ctx context.Context, name string, retry bool) (string, bool)
	Log         func(string)
}

// CheckOptions 는 업로드 검사 하나의 입력이다.
type CheckOptions struct {
	Paths   []string
	Auto    bool   // 자동 업로드 ― 비밀번호를 묻지 않고, 폴더의 걸린 파일만 빼고 올리지 않는다
	Workers int    // 0 이면 min(8, max(2, CPU 수))
	TempDir string // 안쪽 압축을 잠시 풀 곳(비면 os.TempDir)
	Limits  Limits // 0 값이면 DefaultLimits
}

// PassedItem 은 올릴 항목 하나다. 전송은 Size·ModTime 이 그대로인지 대조한 뒤 올린다(검사한 내용만 올라간다).
type PassedItem struct {
	Path        string    `json:"path"`
	IsDir       bool      `json:"isDir"`
	Excludes    []string  `json:"excludes"` // 폴더에서 빼고 올릴 파일(절대 경로)
	ZipPassword string    `json:"-"`        // 사람이 맞힌 암호 압축 비밀번호(단일 파일만). 로그·화면에 내지 않는다
	Size        int64     `json:"size"`     // 파일: 크기, 폴더: 안 파일 크기 합
	ModTime     time.Time `json:"modTime"`  // 검사가 읽기 전에 잰 수정 시각
}

// CheckOutcome 은 검사 결과다.
type CheckOutcome struct {
	Rows       []Row
	Fails      int    // 오류 파일 개수(창에 보이는 수)
	Blocks     int    // 막는 오류 수 ― 0 보다 크면 올리지 않는다
	Skip       int    // 폴더에서 빼고 올릴 파일 수
	Value      string // "AUTO" · "SKIP:n" · ""
	Items      []PassedItem
	Mismatches []Mismatch
	Canceled   bool
}

// ZipPwAction 은 바깥이 잠긴 압축을 어떻게 할지다(원본 ZipPasswordGate.h).
type ZipPwAction int

const (
	ZipPwPass      ZipPwAction = iota // 안을 검사하지 않고 통과
	ZipPwAsk                          // 비밀번호를 묻는다(두 번까지)
	ZipPwBlock                        // 묻지 않고 막는다
	ZipPwBlockAuto                    // 자동 업로드라 묻지 않고 막는다
)

// ZipAction 은 fm_disable_zip_pass·fm_use_Zip_pwd 와 자동 여부로 고른다. 통과가 묻기·막기보다 앞선다.
func ZipAction(disablePass, useZipPwd int, auto bool) ZipPwAction {
	switch {
	case disablePass == 1:
		return ZipPwPass
	case useZipPwd != 1:
		return ZipPwBlock
	case auto:
		return ZipPwBlockAuto
	}
	return ZipPwAsk
}

// maxZipPasswordRunes 는 전송에 실어 보낼 비밀번호 길이 상한이다(원본 SaveAprvUserInfo 칸).
const maxZipPasswordRunes = 128

// job 은 파일 하나의 검사다. folder 가 0 이상이면 그 폴더 안 파일이다.
type job struct {
	idx    int // 작업 순번(아래 표 순서)
	top    int // 위 표의 몇 번째 항목인가
	folder bool
	path   string
	rel    string // 폴더 안 상대 경로
	// 결과
	fail     int
	keys     []string // 이 파일이 만든 아래 표 행
	password string
}

type checker struct {
	ctx   context.Context
	s     *Set
	o     CheckOptions
	h     Hooks
	depth int
	askMu sync.Mutex
	mu    sync.Mutex
	rows  map[string]Row
	mism  []Mismatch
}

func (c *checker) log(s string) {
	if c.h.Log != nil {
		c.h.Log(s)
	}
}

func (c *checker) put(rows ...Row) {
	c.mu.Lock()
	for _, r := range rows {
		c.rows[r.Key] = r
	}
	c.mu.Unlock()
	if c.h.Rows != nil {
		c.h.Rows(rows)
	}
}

func (c *checker) addMismatch(m Mismatch, ok bool) {
	if !ok {
		return
	}
	c.mu.Lock()
	c.mism = append(c.mism, m)
	c.mu.Unlock()
}

func topKey(i int) string { return fmt.Sprintf("t%d", i) }

// RunUploadCheck 는 업로드할 파일·폴더를 정책 한 벌 s 로 검사한다(원본 checkUploadFiles·verifyUploadFile·verifyUploadDir).
// 파일마다 작업자에게 나눠 동시에 검사하고, ctx 가 끝나면 남은 파일을 건너뛰고 Canceled 로 돌아온다.
func RunUploadCheck(ctx context.Context, s *Set, o CheckOptions, h Hooks) *CheckOutcome {
	if o.Workers <= 0 {
		o.Workers = min(8, max(2, runtime.NumCPU()))
	}
	c := &checker{ctx: ctx, s: s, o: o, h: h, rows: map[string]Row{}}
	depth, clamped, raw := s.DepthFromPolicy()
	if clamped {
		c.log(fmt.Sprintf("압축 중첩 깊이 정책 값을 범위로 자른다 >>> fm_extract_zipdepth[%s] → %d", raw, depth))
	}
	c.depth = depth

	// 위 표 항목과 작업 목록. 기준값(크기·수정 시각)은 읽기 전에 잰다.
	items := make([]PassedItem, len(o.Paths))
	var jobs []*job
	folderJobs := map[int][]*job{}
	for i, p := range o.Paths {
		st, err := os.Stat(p)
		items[i] = PassedItem{Path: p}
		if err == nil {
			items[i].ModTime = st.ModTime()
			items[i].IsDir = st.IsDir()
		}
		if items[i].IsDir {
			var total int64
			_ = filepath.WalkDir(p, func(fp string, d fs.DirEntry, err error) error {
				if err != nil || ctx.Err() != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				fi, err := os.Stat(fp) // 링크는 가리키는 것을 본다. 폴더를 가리키면 건너뛴다
				if err != nil || fi.IsDir() {
					return nil
				}
				total += fi.Size()
				rel, _ := filepath.Rel(p, fp)
				j := &job{top: i, folder: true, path: fp, rel: filepath.ToSlash(rel)}
				jobs = append(jobs, j)
				folderJobs[i] = append(folderJobs[i], j)
				return nil
			})
			items[i].Size = total
			continue
		}
		if err == nil {
			items[i].Size = st.Size()
		}
		// 상세에 크기를 미리 넣어 둔다. 걸리면 사유로 갈아 끼운다(원본과 같다)
		c.put(Row{Key: topKey(i), Top: true, Order: int64(i), Name: filepath.Base(p), Result: PolicyText(OK), Detail: SizeText(items[i].Size), OK: true})
		jobs = append(jobs, &job{top: i, path: p})
	}
	for k, j := range jobs {
		j.idx = k
	}

	// 작업자
	total := len(jobs)
	var done int
	var doneMu sync.Mutex
	ch := make(chan *job)
	var wg sync.WaitGroup
	for w := 0; w < o.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if ctx.Err() == nil {
					c.verify(j)
				}
				doneMu.Lock()
				done++
				d := done
				doneMu.Unlock()
				if c.h.Progress != nil {
					c.h.Progress(d, total, filepath.Base(j.path))
				}
			}
		}()
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		ch <- j
	}
	close(ch)
	wg.Wait()

	out := &CheckOutcome{Canceled: ctx.Err() != nil}
	if out.Canceled {
		return out
	}

	// 집계. 폴더는 다 훑은 뒤에야 뺄지 정한다.
	for i, it := range items {
		if !it.IsDir {
			for _, j := range jobs {
				if j.top == i && !j.folder {
					out.Fails += j.fail
					out.Blocks += j.fail
					if j.fail == 0 {
						it.ZipPassword = j.password
						out.Items = append(out.Items, it)
					}
				}
			}
			continue
		}
		name := filepath.Base(it.Path)
		var failed []*job
		for _, j := range folderJobs[i] {
			if j.fail > 0 {
				failed = append(failed, j)
			}
		}
		errCount := len(failed)
		passed := len(folderJobs[i]) - errCount
		top := Row{Key: topKey(i), Top: true, Order: int64(i), Name: name}
		switch {
		case errCount == 0:
			top.Result, top.OK = "정상 폴더", true
			out.Items = append(out.Items, it)
		case passed > 0 && !o.Auto:
			// 통과한 파일이 남으면 걸린 것만 빼고 올릴 수 있다. 빠지는 행에는 앞에 "(제외됨) " 을 붙인다
			out.Fails += errCount
			out.Skip += errCount
			top.Result = fmt.Sprintf("차단 %d건 제외", errCount)
			top.Detail = fmt.Sprintf("내부 오류 %d건", errCount)
			for _, j := range failed {
				it.Excludes = append(it.Excludes, j.path)
				c.markExcluded(j.keys)
			}
			out.Items = append(out.Items, it)
		default:
			out.Fails += errCount
			out.Blocks += errCount
			top.Result = "오류 파일 존재"
			top.Detail = fmt.Sprintf("내부 오류 %d건", errCount)
		}
		c.put(top)
	}
	switch {
	case o.Auto:
		out.Value = "AUTO"
	case out.Blocks == 0 && out.Skip > 0:
		out.Value = fmt.Sprintf("SKIP:%d", out.Skip)
	}
	out.Rows = make([]Row, 0, len(c.rows))
	for _, r := range c.rows {
		out.Rows = append(out.Rows, r)
	}
	SortRows(out.Rows)
	out.Mismatches = c.mism
	return out
}

// SortRows 는 위 표 행을 앞에, 각 표 안에서는 Order 순으로 놓는다.
func SortRows(rows []Row) {
	sort.SliceStable(rows, func(a, b int) bool {
		if rows[a].Top != rows[b].Top {
			return rows[a].Top
		}
		return rows[a].Order < rows[b].Order
	})
}

func (c *checker) markExcluded(keys []string) {
	var rows []Row
	c.mu.Lock()
	for _, k := range keys {
		if r, ok := c.rows[k]; ok {
			r.Result = "(제외됨) " + r.Result
			r.OK = false
			rows = append(rows, r)
		}
	}
	c.mu.Unlock()
	c.put(rows...)
}

func (c *checker) inspectOpts(pw string) InspectOptions {
	return InspectOptions{Password: pw, MaxDepth: c.depth, TempDir: c.o.TempDir, Limits: c.o.Limits, Log: c.h.Log}
}

// verify 는 파일 하나를 검사하고 행을 낸다(원본 verifyUploadFile).
func (c *checker) verify(j *job) {
	name := filepath.Base(j.path)
	code, size, head := CheckFile(c.ctx, c.s, j.path)
	c.addMismatch(NewMismatch(code, j.path, head))
	zc := ZipOK
	var entries []Entry
	if code == OK && c.s.InsideCheck() && IsArchiveName(name) {
		var stop *Stop
		entries, zc, stop = Inspect(c.ctx, j.path, c.inspectOpts(""))
		if stop != nil {
			c.log(stop.LogLine("업로드 검사", j.path))
		}
		switch zc {
		case ZipPassword: // 바깥이 잠겼다. 안쪽만 잠긴 것(-986)은 묻지 않고 막는다
			entries, zc, j.password = c.lockedZip(j, name)
		case ZipUncheckable: // 풀 수 없는 형식은 암호 압축 통과 정책만 따른다(물어볼 비밀번호가 없다)
			if c.s.Int("fm_disable_zip_pass") == 1 {
				c.log("검사할 수 없는 압축 >>> 안을 검사하지 않고 통과(fm_disable_zip_pass=1): " + name)
				zc, entries = ZipOK, nil
			} else {
				c.log("검사할 수 없는 압축 >>> 막는다: " + name)
			}
		}
		if c.ctx.Err() != nil {
			return
		}
		c.log(fmt.Sprintf("압축파일 검사 : %s, 결과: %d, 내부 파일개수: %d", j.path, zc, len(entries)))
	}
	limit := c.s.SizeLimit(false)
	tk := topKey(j.top)
	parentName := filepath.Base(c.o.Paths[j.top])
	inner := func(k int, parent, nm, result, detail string, ok bool) Row {
		key := fmt.Sprintf("i%d.%d", j.idx, k)
		j.keys = append(j.keys, key)
		return Row{Key: key, Order: int64(j.idx)<<20 | int64(k), Parent: parent, Name: nm, Result: result, Detail: detail, OK: ok}
	}
	switch {
	case zc == ZipOK && len(entries) > 0:
		fails := 0
		rows := make([]Row, 0, len(entries)+1)
		for k, e := range entries {
			ec := CheckEntry(c.s, e.Name, e.Size, e.Head)
			if ec != OK {
				fails++
				c.addMismatch(NewMismatch(ec, j.path+"/"+e.Path, e.Head))
			}
			rows = append(rows, inner(k, name, e.Path, PolicyText(ec), Detail(ec, e.Name, e.Size, limit), ec == OK))
		}
		if !j.folder {
			detail := fmt.Sprintf("내부 파일 %d개", len(entries))
			if fails > 0 {
				detail = fmt.Sprintf("내부 파일 %d개, 오류 %d건", len(entries), fails)
			}
			rows = append(rows, Row{Key: tk, Top: true, Order: int64(j.top), Name: name, Result: ZipText(ZipOK), Detail: detail, OK: fails == 0})
		}
		c.put(rows...)
		j.fail = fails
	case zc < 0: // 압축 검사 실패 ― 암호 압축 막힘·상한·위험 경로도 여기다
		if j.folder {
			c.put(inner(0, parentName, j.rel, ZipText(zc), "", false))
		} else {
			c.put(Row{Key: tk, Top: true, Order: int64(j.top), Name: name, Result: ZipText(zc), Detail: SizeText(size)})
		}
	default:
		ok := code == OK
		if j.folder {
			c.put(inner(0, parentName, j.rel, PolicyText(code), Detail(code, name, size, limit), ok))
		} else {
			c.put(Row{Key: tk, Top: true, Order: int64(j.top), Name: name, Result: PolicyText(code), Detail: Detail(code, name, size, limit), OK: ok})
		}
	}
	if code != OK || zc < 0 {
		if c.o.Auto {
			c.log("[Auto]파일검사 오류로 자동 업로드 불가 : " + name)
		}
		j.fail++
	}
}

// lockedZip 은 바깥이 잠긴 압축을 정책대로 통과·묻기·막기 한다(원본 checkLockedZip). 비밀번호는 길이만 남긴다.
// 맞힌 비밀번호는 사람이 올린 단일 파일이고 128자 이하일 때만 돌려준다(폴더는 압축 하나로 묶여 올라간다).
func (c *checker) lockedZip(j *job, name string) ([]Entry, ZipCode, string) {
	switch ZipAction(c.s.Int("fm_disable_zip_pass"), c.s.Int("fm_use_Zip_pwd"), c.o.Auto) {
	case ZipPwPass:
		c.log("암호 압축 >>> 안을 검사하지 않고 통과(fm_disable_zip_pass=1): " + name)
		return nil, ZipOK, ""
	case ZipPwBlock:
		c.log("암호 압축 >>> 묻지 않고 막는다(fm_use_Zip_pwd != 1): " + name)
		return nil, ZipPassword, ""
	case ZipPwBlockAuto:
		c.log("[Auto]암호 압축 >>> 자동 업로드라 비밀번호를 묻지 않고 막는다: " + name)
		return nil, ZipPassword, ""
	}
	c.askMu.Lock() // 비밀번호 창은 하나다
	defer c.askMu.Unlock()
	for attempt := 0; attempt < 2; attempt++ {
		if c.ctx.Err() != nil || c.h.AskPassword == nil {
			c.log("암호 압축 >>> 비밀번호 입력 취소: " + name)
			return nil, ZipPwCancel, ""
		}
		pw, ok := c.h.AskPassword(c.ctx, name, attempt > 0)
		if !ok || c.ctx.Err() != nil {
			c.log("암호 압축 >>> 비밀번호 입력 취소: " + name)
			return nil, ZipPwCancel, ""
		}
		entries, zc, stop := Inspect(c.ctx, j.path, c.inspectOpts(pw))
		if stop != nil {
			c.log(stop.LogLine("업로드 검사", j.path))
		}
		if zc != ZipPassword {
			c.log(fmt.Sprintf("암호 압축 >>> 비밀번호 확인 결과: %s 결과: %d PW[*** len=%d]", name, zc, utf8.RuneCountInString(pw)))
			keep := ""
			if zc == ZipOK && !j.folder && !c.o.Auto && utf8.RuneCountInString(pw) <= maxZipPasswordRunes {
				keep = pw
			}
			return entries, zc, keep
		}
		c.log(fmt.Sprintf("암호 압축 >>> 비밀번호가 틀렸다(%d/2): %s", attempt+1, name))
	}
	return nil, ZipPwWrong, ""
}

// DownloadCheck 는 결재자가 받은 것의 내부 목록이다(원본 verifyDownloadZip). 폴더면 안 파일을, 압축이면 푼 목록을 보인다.
// show 가 거짓이면 창을 띄우지 않는다(내부 파일 0건, 압축 아님·잠김·손상). 상한으로 멈추면 그 사유 한 행과 오류 1건이다.
func DownloadCheck(ctx context.Context, path string, depth int, tempDir string, limits Limits, log func(string)) (rows []Row, fails int, show bool) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, false
	}
	name := filepath.Base(path)
	type shown struct {
		name string
		size int64
	}
	var list []shown
	if st.IsDir() {
		_ = filepath.WalkDir(path, func(fp string, d fs.DirEntry, err error) error {
			if err != nil || ctx.Err() != nil || d.IsDir() {
				return nil
			}
			fi, err := os.Stat(fp)
			if err != nil || fi.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(path, fp)
			list = append(list, shown{filepath.ToSlash(rel), fi.Size()})
			return nil
		})
	} else {
		entries, zc, stop := Inspect(ctx, path, InspectOptions{MaxDepth: depth, TempDir: tempDir, Limits: limits, Log: log})
		if stop != nil {
			if log != nil {
				log(stop.LogLine("결재 내용 보기", path))
			}
			return []Row{{Key: topKey(0), Top: true, Name: name, Result: ZipText(zc)}}, 1, true
		}
		if zc == ZipOK {
			for _, e := range entries {
				list = append(list, shown{e.Path, e.Size})
			}
		}
	}
	if ctx.Err() != nil || len(list) == 0 {
		return nil, 0, false
	}
	result := "압축 해제 성공"
	if st.IsDir() {
		result = "폴더 확인"
	}
	rows = append(rows, Row{Key: topKey(0), Top: true, Name: name, Result: result, Detail: fmt.Sprintf("내부 파일 %d개", len(list)), OK: true})
	for k, f := range list {
		rows = append(rows, Row{Key: fmt.Sprintf("i0.%d", k), Order: int64(k), Parent: name, Name: f.name, Result: PolicyText(OK), Detail: SizeText(f.size), OK: true})
	}
	return rows, 0, true
}

// ClipSizeOK 는 클립보드 업로드 크기 판정이다(원본 checkUploadFiles 의 isClip). 넘으면 안내 문구를 돌려준다.
func ClipSizeOK(s *Set, size int64) (bool, string) {
	if s.CheckSize(size, true) {
		return true, ""
	}
	return false, PolicyText(SizeOver)
}

// HasClipMarker 는 클립보드 파일명과 헷갈리는 이름(__CLIP__)인가다 ― 업로드에서 뺀다.
func HasClipMarker(path string) bool { return strings.Contains(path, "__CLIP__") }
