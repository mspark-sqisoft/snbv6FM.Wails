package transfer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Phase 는 한 전송 안의 단계다(화면 문구를 고른다).
type Phase int32

const (
	PhaseNone    Phase = iota
	PhaseZip           // 폴더 압축 중
	PhaseHash          // 검사값 계산 중
	PhaseSend          // 보내는 중
	PhaseCommit        // 다 보냈고 서버 답을 기다림(「파일 암호화 중...」)
	PhaseRecv          // 받는 중
	PhaseExtract       // 받은 폴더 푸는 중
)

var phaseNames = [...]string{"", "zip", "hash", "send", "commit", "recv", "extract"}

func (p Phase) String() string { return phaseNames[p] }

// Progress 는 워커가 쓰고 시계가 읽는 진행값이다(잠금 없음).
type Progress struct {
	done, total atomic.Int64
	phase       atomic.Int32
	last        atomic.Int64 // 마지막으로 늘어난 때(UnixNano) ― 정지 감시
}

// Set 은 단계를 바꾸고 진행을 done/total 로 둔다.
func (p *Progress) Set(ph Phase, done, total int64) {
	p.total.Store(total)
	p.done.Store(done)
	p.phase.Store(int32(ph))
	p.last.Store(time.Now().UnixNano())
}

// Add 는 진행을 n 늘린다.
func (p *Progress) Add(n int64) {
	p.done.Add(n)
	p.last.Store(time.Now().UnixNano())
}

// Snapshot 은 지금 단계·진행이다.
func (p *Progress) Snapshot() (Phase, int64, int64) {
	return Phase(p.phase.Load()), p.done.Load(), p.total.Load()
}

// Idle 은 마지막으로 늘어난 뒤 지난 시간이다.
func (p *Progress) Idle() time.Duration { return time.Duration(time.Now().UnixNano() - p.last.Load()) }

// Job 은 대기열에 넣는 전송 한 건이다.
//   - Prepare(선택): 압축·검사값 계산처럼 차례가 오기 전에 미리 할 수 있는 일. 동시에 2개까지 먼저 돌고,
//     차례가 왔는데 아직이면 워커가 그 끝을 기다린다(한 번만 돈다)
//   - Run: 실제 전송. ctx 는 취소(행 [취소]·로그아웃)에 끊긴다
//   - Done: 끝날 때 한 번(대기 중 취소 포함). 잠금 밖에서 부른다
type Job struct {
	Dir     Dir
	Name    string
	Size    int64
	Key     string // 같은 파일 중복 금지(빈 값이면 검사 안 함)
	Auto    bool
	Prepare func(ctx context.Context, p *Progress) error
	Run     func(ctx context.Context, p *Progress) error
	Done    func(Result)
}

// Result 는 끝난 전송의 결과다.
type Result struct {
	ID      int64
	State   State
	Err     error // Done 이면 nil, 취소면 ErrCanceled 또는 로그아웃 사유
	Elapsed time.Duration
	Bytes   int64
}

// Item 은 전송 상태 서랍의 한 줄이다(화면에 그대로 간다).
type Item struct {
	ID    int64  `json:"id"`
	Dir   string `json:"dir"`   // "upload" · "download"
	Name  string `json:"name"`  // 화면 이름
	Size  int64  `json:"size"`  // 처음 알려진 크기(폴더는 파일 합)
	State string `json:"state"` // "pending" · "active" · "done" · "failed" · "cancelled"
	Wait  string `json:"wait"`  // 대기 사유: "" · "serial"(대용량 순차 전송)
	Phase string `json:"phase"` // "" · "zip" · "hash" · "send" · "commit" · "recv" · "extract"
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Speed int64  `json:"speed"` // 바이트/초
	Auto  bool   `json:"auto"`
	Error string `json:"error"` // 실패·취소 사유(원본 문구)
	Code  int    `json:"code"`
}

// Tick 은 진행값이 바뀐 행 하나다(200ms 묶음).
type Tick struct {
	ID    int64  `json:"id"`
	Phase string `json:"phase"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Speed int64  `json:"speed"`
}

var stateNames = [...]string{"", "pending", "active", "done", "failed", "cancelled"}

type run struct {
	id       int64
	job      Job
	prog     Progress
	ctx      context.Context
	cancel   context.CancelFunc
	prepOnce sync.Once
	prepErr  error
	state    State
	err      error
	cancelBy error // 진행 중 취소의 사유(nil 이면 취소 아님)
	started  time.Time
	ended    time.Time
	lastDone int64
	lastAt   time.Time
	speed    float64
	sent     Tick
}

func (r *run) prepare() {
	r.prepOnce.Do(func() {
		if r.job.Prepare != nil {
			r.prepErr = r.job.Prepare(r.ctx, &r.prog)
		}
	})
}

// Manager 는 대기열과 워커·진행 시계다. 상태는 Queue 한 곳에 두고 화면은 OnState·OnTick 만 그린다.
type Manager struct {
	mu    sync.Mutex
	q     *Queue
	seq   int64
	runs  map[int64]*run
	order []int64
	dirty bool
	prep  chan struct{}
	h     Hooks
}

// Hooks 는 관리자가 시계 고루틴에서 부르는 것이다(잠금 밖).
type Hooks struct {
	OnState    func([]Item) // 목록이 바뀌면(묶어서 200ms 마다 한 번)
	OnTick     func([]Tick) // 진행값이 바뀐 행
	OnActivity func()       // 바이트가 움직였다(자동 로그아웃 시계 되돌리기)
}

// NewManager 는 방향별 동시 상한 maxConc, 직렬 기준 serialMin 바이트의 관리자를 만들고 시계를 띄운다.
func NewManager(maxConc int, serialMin int64, h Hooks) *Manager {
	m := &Manager{q: NewQueue(maxConc, serialMin), runs: map[int64]*run{}, prep: make(chan struct{}, 2), h: h}
	go m.clock()
	return m
}

// SetSerialMin 은 직렬 기준을 바꾼다(fmenv 를 다시 읽었을 때). 진행 중인 것에는 다음 출발부터 걸린다.
func (m *Manager) SetSerialMin(n int64) {
	m.mu.Lock()
	m.q.SetSerialMin(n)
	m.mu.Unlock()
}

// Add 는 전송을 대기열에 넣고 번호를 준다. 같은 Key 가 대기·진행 중이면 CodeDuplicateQueue 실패.
func (m *Manager) Add(j Job) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	id := m.seq
	if err := m.q.Enqueue(id, j.Dir, j.Size, j.Key); err != nil {
		return 0, fail(CodeDuplicateQueue, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{id: id, job: j, ctx: ctx, cancel: cancel, state: Pending}
	r.prog.total.Store(j.Size)
	m.runs[id] = r
	m.order = append(m.order, id)
	m.dirty = true
	if j.Prepare != nil {
		go func() {
			select {
			case m.prep <- struct{}{}:
				defer func() { <-m.prep }()
				r.prepare()
			case <-ctx.Done():
			}
		}()
	}
	m.pumpLocked()
	return id, nil
}

func (m *Manager) pumpLocked() {
	for _, d := range []Dir{Up, Down} {
		for {
			id, ok := m.q.Next(d)
			if !ok {
				break
			}
			r := m.runs[id]
			r.state, r.started = Active, time.Now()
			m.dirty = true
			go m.work(r)
		}
	}
}

func (m *Manager) work(r *run) {
	r.prepare()
	err := r.prepErr
	if err == nil {
		if err = r.ctx.Err(); err == nil {
			err = r.job.Run(r.ctx, &r.prog)
		}
	}
	m.finish(r, err)
}

func (m *Manager) finish(r *run, err error) {
	m.mu.Lock()
	st := Done
	switch {
	case err == nil: // 취소가 끝난 뒤에 닿았다 ― 파일은 이미 옮겨졌다
	case r.cancelBy != nil:
		st, err = Cancelled, r.cancelBy
	default:
		st = Failed
	}
	m.q.Complete(r.id, st)
	r.state, r.err, r.ended = st, err, time.Now()
	r.cancel()
	m.dirty = true
	m.pumpLocked()
	res := Result{ID: r.id, State: st, Err: err, Elapsed: r.ended.Sub(r.started), Bytes: r.prog.done.Load()}
	m.mu.Unlock()
	if r.job.Done != nil {
		r.job.Done(res)
	}
}

// cancelLocked 는 대기 중이면 곧바로 끝내고(Done 을 돌려준다), 진행 중이면 ctx 를 끊어 워커가 끝내게 한다.
func (m *Manager) cancelLocked(r *run, why error) func() {
	if r.state.terminal() {
		return nil
	}
	if m.q.Cancel(r.id) {
		r.state, r.err, r.ended = Cancelled, why, time.Now()
		r.cancel()
		m.dirty = true
		if r.job.Done != nil {
			return func() { r.job.Done(Result{ID: r.id, State: Cancelled, Err: why}) }
		}
		return nil
	}
	if r.cancelBy == nil {
		r.cancelBy = why
	}
	r.cancel()
	return nil
}

// Cancel 은 전송 하나를 취소한다(대기·진행 모두). 이미 끝났거나 모르면 거짓.
func (m *Manager) Cancel(id int64) bool {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok || r.state.terminal() {
		m.mu.Unlock()
		return false
	}
	done := m.cancelLocked(r, ErrCanceled)
	m.mu.Unlock()
	if done != nil {
		done()
	}
	return true
}

// CancelAll 은 대기를 먼저 모두 빼고 진행 중인 것을 끊는다(로그아웃·끊김). why 가 nil 이면 ErrCanceled.
// filter 가 있으면 그것이 참인 것만(예: 자동 전송만).
func (m *Manager) CancelAll(why error, filter func(Item) bool) int {
	if why == nil {
		why = ErrCanceled
	}
	m.mu.Lock()
	var dones []func()
	n := 0
	for _, st := range []State{Pending, Active} {
		for _, d := range []Dir{Up, Down} {
			for _, id := range m.q.IDs(d, st) {
				r := m.runs[id]
				if filter != nil && !filter(m.itemLocked(r)) {
					continue
				}
				n++
				if f := m.cancelLocked(r, why); f != nil {
					dones = append(dones, f)
				}
			}
		}
	}
	m.mu.Unlock()
	for _, f := range dones {
		f()
	}
	return n
}

// Clear 는 끝난 행을 목록에서 지운다([목록 지우기] ― 진행 중인 전송은 계속된다).
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	keep := m.order[:0]
	for _, id := range m.order {
		if m.runs[id].state.terminal() {
			delete(m.runs, id)
		} else {
			keep = append(keep, id)
		}
	}
	m.order = keep
	m.dirty = true
}

// Items 는 지금 목록이다(넣은 순서).
func (m *Manager) Items() []Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.itemsLocked()
}

// Busy 는 filter 에 맞는 대기·진행 중 전송이 있는가다.
func (m *Manager) Busy(filter func(Item) bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.order {
		r := m.runs[id]
		if !r.state.terminal() && (filter == nil || filter(m.itemLocked(r))) {
			return true
		}
	}
	return false
}

func (m *Manager) itemsLocked() []Item {
	out := make([]Item, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.itemLocked(m.runs[id]))
	}
	return out
}

func (m *Manager) itemLocked(r *run) Item {
	ph, done, total := r.prog.Snapshot()
	it := Item{ID: r.id, Dir: "upload", Name: r.job.Name, Size: r.job.Size, State: stateNames[r.state],
		Phase: ph.String(), Done: done, Total: total, Speed: int64(r.speed), Auto: r.job.Auto}
	if r.job.Dir == Down {
		it.Dir = "download"
	}
	if r.state == Pending && m.q.Large(r.job.Dir) {
		it.Wait = "serial"
	}
	if r.err != nil {
		it.Code = CodeOf(r.err)
		it.Error = r.err.Error()
	}
	return it
}

// clock 은 200ms 마다 목록 변경과 진행값 변경을 묶어 낸다.
func (m *Manager) clock() {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for now := range t.C {
		m.mu.Lock()
		var items []Item
		if m.dirty {
			m.dirty = false
			items = m.itemsLocked()
		}
		var ticks []Tick
		moved := false
		for _, id := range m.order {
			r := m.runs[id]
			if r.state.terminal() {
				continue
			}
			ph, done, total := r.prog.Snapshot()
			if r.state == Active && !r.lastAt.IsZero() {
				if dt := now.Sub(r.lastAt).Seconds(); dt > 0 && (ph == PhaseSend || ph == PhaseRecv) {
					inst := float64(done-r.lastDone) / dt
					if inst < 0 {
						inst = 0
					}
					if r.speed == 0 {
						r.speed = inst
					} else {
						r.speed = r.speed*0.7 + inst*0.3
					}
				}
			}
			if done != r.lastDone {
				moved = true
			}
			r.lastDone, r.lastAt = done, now
			tk := Tick{ID: id, Phase: ph.String(), Done: done, Total: total, Speed: int64(r.speed)}
			if tk != r.sent {
				r.sent = tk
				ticks = append(ticks, tk)
			}
		}
		onState, onTick, onAct := m.h.OnState, m.h.OnTick, m.h.OnActivity
		m.mu.Unlock()
		if items != nil && onState != nil {
			onState(items)
		}
		if len(ticks) > 0 && onTick != nil {
			onTick(ticks)
		}
		if moved && onAct != nil {
			onAct()
		}
	}
}

// IsCanceled 는 err 가 사용자 취소(실패 창을 띄우지 않는 것)인가다.
func IsCanceled(err error) bool { return errors.Is(err, ErrCanceled) }
