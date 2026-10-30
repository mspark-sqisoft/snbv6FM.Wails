package transfer

import (
	"errors"
	"slices"
)

// Dir 는 전송 방향이다. 방향마다 대기열이 따로다(원본 TransferQueue).
type Dir int

const (
	Up   Dir = 0
	Down Dir = 1
)

// State 는 전송 한 건의 상태다.
type State int

const (
	Unknown State = iota
	Pending
	Active
	Done
	Failed
	Cancelled
)

func (s State) terminal() bool { return s >= Done }

var (
	ErrDupID  = errors.New("이미 넣은 전송 번호")
	ErrDupKey = errors.New("같은 파일이 이미 대기·진행 중")
)

type qent struct {
	dir   Dir
	size  int64
	key   string
	state State
}

// Queue 는 전송 대기열의 순수 규칙이다(원본 imports/FileMoverGrpc/TransferQueue, docs/port/03 7.2). 잠금은 부르는 쪽(Manager)이 한다.
//   - 방향별 FIFO. 동시 상한은 그 방향의 대기·진행 중에 serialMin 보다 큰 것이 하나라도 있으면 1, 아니면 maxConc
//     (기준과 같은 크기는 동시 ― 원본 049)
//   - 아는 번호는 Forget 하기 전까지 상태와 무관하게 다시 받지 않는다. 추가(원본보다 낫게): 같은 Key 가 대기·진행 중이면 거부
//   - Cancel 은 대기 중일 때만, Complete 는 진행 중일 때만 참(두 번 불러도 한 번만 센다)
type Queue struct {
	maxConc   int
	serialMin int64
	ents      map[int64]*qent
	pending   [2][]int64
	active    [2]int
	big       [2]int // 대기·진행 중 serialMin 을 넘는 수(Large 는 시계가 행마다 부른다)
	keys      map[string]int64
}

// NewQueue 는 동시 상한 maxConc, 직렬 기준 serialMin 바이트(0 이하면 직렬 없음)의 대기열이다.
func NewQueue(maxConc int, serialMin int64) *Queue {
	if maxConc < 1 {
		maxConc = 1
	}
	return &Queue{maxConc: maxConc, serialMin: serialMin, ents: map[int64]*qent{}, keys: map[string]int64{}}
}

// Enqueue 는 대기열 끝에 넣는다. key 가 비면 중복 검사를 하지 않는다.
func (q *Queue) Enqueue(id int64, dir Dir, size int64, key string) error {
	if _, ok := q.ents[id]; ok {
		return ErrDupID
	}
	if key != "" {
		if _, ok := q.keys[key]; ok {
			return ErrDupKey
		}
		q.keys[key] = id
	}
	q.ents[id] = &qent{dir: dir, size: size, key: key, state: Pending}
	q.pending[dir] = append(q.pending[dir], id)
	if size > q.serialMin {
		q.big[dir]++
	}
	return nil
}

// SetSerialMin 은 직렬 기준을 바꾸고 대기·진행 중인 것을 새 기준으로 다시 센다.
func (q *Queue) SetSerialMin(n int64) {
	q.serialMin = n
	q.big = [2]int{}
	for _, e := range q.ents {
		if (e.state == Pending || e.state == Active) && e.size > n {
			q.big[e.dir]++
		}
	}
}

// Large 는 그 방향에 직렬 기준을 넘는 것이 대기·진행 중인가다(화면의 「대용량 순차 전송」 사유).
func (q *Queue) Large(dir Dir) bool { return q.serialMin > 0 && q.big[dir] > 0 }

// Limit 은 그 방향의 지금 동시 상한이다.
func (q *Queue) Limit(dir Dir) int {
	if q.Large(dir) {
		return 1
	}
	return q.maxConc
}

// Next 는 상한 안이면 맨 앞 대기를 진행으로 바꿔 준다.
func (q *Queue) Next(dir Dir) (int64, bool) {
	if len(q.pending[dir]) == 0 || q.active[dir] >= q.Limit(dir) {
		return 0, false
	}
	id := q.pending[dir][0]
	q.pending[dir] = q.pending[dir][1:]
	q.ents[id].state = Active
	q.active[dir]++
	return id, true
}

func (q *Queue) release(e *qent) {
	if e.key != "" {
		delete(q.keys, e.key)
	}
	if e.size > q.serialMin {
		q.big[e.dir]--
	}
}

// Complete 는 진행 중인 것을 끝낸다(result 는 Done·Failed·Cancelled).
func (q *Queue) Complete(id int64, result State) bool {
	e, ok := q.ents[id]
	if !ok || e.state != Active || !result.terminal() {
		return false
	}
	e.state = result
	q.active[e.dir]--
	q.release(e)
	return true
}

// Cancel 은 대기 중인 것을 빼고 Cancelled 로 둔다.
func (q *Queue) Cancel(id int64) bool {
	e, ok := q.ents[id]
	if !ok || e.state != Pending {
		return false
	}
	q.pending[e.dir] = slices.DeleteFunc(q.pending[e.dir], func(x int64) bool { return x == id })
	e.state = Cancelled
	q.release(e)
	return true
}

// Forget 은 끝난 번호를 지운다(Manager 가 행을 지울 때).
func (q *Queue) Forget(id int64) {
	if e, ok := q.ents[id]; ok && e.state.terminal() {
		delete(q.ents, id)
	}
}

// State 는 그 번호의 상태다. 모르면 Unknown.
func (q *Queue) State(id int64) State {
	if e, ok := q.ents[id]; ok {
		return e.state
	}
	return Unknown
}

// Direction 은 그 번호의 방향이다.
func (q *Queue) Direction(id int64) (Dir, bool) {
	if e, ok := q.ents[id]; ok {
		return e.dir, true
	}
	return 0, false
}

// Pending·ActiveCount 는 그 방향의 대기·진행 수다.
func (q *Queue) PendingCount(dir Dir) int { return len(q.pending[dir]) }
func (q *Queue) ActiveCount(dir Dir) int  { return q.active[dir] }

// IDs 는 그 방향·상태의 번호다. 대기는 넣은 순서, 그 밖은 번호 순서.
func (q *Queue) IDs(dir Dir, st State) []int64 {
	if st == Pending {
		return slices.Clone(q.pending[dir])
	}
	var out []int64
	for id, e := range q.ents {
		if e.dir == dir && e.state == st {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
