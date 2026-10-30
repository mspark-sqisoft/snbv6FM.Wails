package transfer

import (
	"slices"
	"testing"
)

// 원본 tests/tst_TransferQueue.cpp 를 옮겼다(forceSerialDownload 는 Wails 판에 없다).

const (
	kThreshold = 100 << 20
	kSmall     = 1024
	kLarge     = kThreshold + 1
)

func newQ() *Queue { return NewQueue(5, kThreshold) }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantNext(t *testing.T, q *Queue, d Dir, want int64) {
	t.Helper()
	got, ok := q.Next(d)
	if !ok || got != want {
		t.Fatalf("Next(%d) = %d,%v, want %d", d, got, ok, want)
	}
}

func wantNone(t *testing.T, q *Queue, d Dir) {
	t.Helper()
	if got, ok := q.Next(d); ok {
		t.Fatalf("Next(%d) = %d, want none", d, got)
	}
}

func TestQueueFIFO(t *testing.T) {
	q := newQ()
	for _, id := range []int64{10, 11, 12} {
		must(t, q.Enqueue(id, Up, kSmall, ""))
	}
	if q.PendingCount(Up) != 3 {
		t.Fatal("pending")
	}
	wantNext(t, q, Up, 10)
	wantNext(t, q, Up, 11)
	wantNext(t, q, Up, 12)
	if q.PendingCount(Up) != 0 || q.ActiveCount(Up) != 3 || q.State(11) != Active {
		t.Fatal("counts")
	}
}

func TestQueueSmallFilesRunFiveAtOnce(t *testing.T) {
	q := newQ()
	for i := range int64(7) {
		must(t, q.Enqueue(i, Up, kSmall, ""))
	}
	if q.Limit(Up) != 5 {
		t.Fatal("limit")
	}
	for range 5 {
		if _, ok := q.Next(Up); !ok {
			t.Fatal("next")
		}
	}
	wantNone(t, q, Up)
	if q.PendingCount(Up) != 2 || !q.Complete(0, Done) {
		t.Fatal("complete")
	}
	wantNext(t, q, Up, 5)
	wantNone(t, q, Up)
}

func TestQueueLargePendingForcesSerial(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kSmall, ""))
	must(t, q.Enqueue(2, Up, kLarge, ""))
	must(t, q.Enqueue(3, Up, kSmall, ""))
	if q.Limit(Up) != 1 {
		t.Fatal("limit")
	}
	wantNext(t, q, Up, 1)
	wantNone(t, q, Up)
}

func TestQueueThresholdStaysConcurrent(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kThreshold, ""))
	must(t, q.Enqueue(2, Up, kSmall, ""))
	if q.Limit(Up) != 5 {
		t.Fatal("threshold size must stay concurrent")
	}
	must(t, q.Enqueue(3, Up, kThreshold+1, ""))
	if q.Limit(Up) != 1 {
		t.Fatal("threshold+1 must be serial")
	}
}

func TestQueueLargeActiveForcesSerialUntilDone(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kLarge, ""))
	wantNext(t, q, Up, 1)
	must(t, q.Enqueue(2, Up, kSmall, ""))
	must(t, q.Enqueue(3, Up, kSmall, ""))
	if q.Limit(Up) != 1 {
		t.Fatal("limit")
	}
	wantNone(t, q, Up)
	if !q.Complete(1, Done) || q.Limit(Up) != 5 {
		t.Fatal("after done")
	}
	wantNext(t, q, Up, 2)
	wantNext(t, q, Up, 3)
	if q.State(1) != Done {
		t.Fatal("state")
	}
}

func TestQueueLanesIndependent(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kLarge, ""))
	must(t, q.Enqueue(2, Down, kSmall, ""))
	must(t, q.Enqueue(3, Down, kSmall, ""))
	if q.Limit(Up) != 1 || q.Limit(Down) != 5 {
		t.Fatal("limits")
	}
	wantNext(t, q, Up, 1)
	wantNext(t, q, Down, 2)
	wantNext(t, q, Down, 3)
	if !q.Complete(2, Done) || q.ActiveCount(Up) != 1 || q.ActiveCount(Down) != 1 {
		t.Fatal("counts")
	}
	if d, _ := q.Direction(1); d != Up {
		t.Fatal("dir 1")
	}
	if d, _ := q.Direction(2); d != Down {
		t.Fatal("dir 2")
	}
}

func TestQueueCancelOnlyWhilePending(t *testing.T) {
	q := newQ()
	for _, id := range []int64{1, 2, 3} {
		must(t, q.Enqueue(id, Up, kSmall, ""))
	}
	wantNext(t, q, Up, 1)
	if q.Cancel(1) || q.State(1) != Active {
		t.Fatal("active must not cancel")
	}
	if !q.Cancel(2) || q.State(2) != Cancelled || q.PendingCount(Up) != 1 {
		t.Fatal("cancel pending")
	}
	wantNext(t, q, Up, 3)
	if q.Cancel(2) || q.Complete(2, Done) || q.ActiveCount(Up) != 2 {
		t.Fatal("cancelled is final")
	}
}

func TestQueueCompleteIdempotent(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kSmall, ""))
	wantNext(t, q, Up, 1)
	if !q.Complete(1, Failed) || q.Complete(1, Failed) || q.Complete(1, Done) {
		t.Fatal("idempotent")
	}
	if q.ActiveCount(Up) != 0 || q.State(1) != Failed {
		t.Fatal("state")
	}
	must(t, q.Enqueue(2, Up, kSmall, ""))
	if q.Complete(2, Done) || q.State(2) != Pending {
		t.Fatal("pending must not complete")
	}
}

func TestQueueDuplicateRejected(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kSmall, ""))
	if q.Enqueue(1, Up, kSmall, "") != ErrDupID || q.Enqueue(1, Down, kSmall, "") != ErrDupID {
		t.Fatal("dup id")
	}
	wantNext(t, q, Up, 1)
	q.Complete(1, Done)
	if q.Enqueue(1, Up, kSmall, "") != ErrDupID {
		t.Fatal("id is never reused")
	}
}

// 원본보다 낫게: 같은 파일(경로·크기·수정 시각)은 대기·진행 중에 다시 넣지 않고, 끝나면 다시 받는다.
func TestQueueDuplicateKey(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Up, kSmall, "a|1|2"))
	if q.Enqueue(2, Up, kSmall, "a|1|2") != ErrDupKey {
		t.Fatal("dup key pending")
	}
	wantNext(t, q, Up, 1)
	if q.Enqueue(3, Up, kSmall, "a|1|2") != ErrDupKey {
		t.Fatal("dup key active")
	}
	q.Complete(1, Done)
	must(t, q.Enqueue(4, Up, kSmall, "a|1|2"))
	if !q.Cancel(4) {
		t.Fatal("cancel")
	}
	must(t, q.Enqueue(5, Up, kSmall, "a|1|2"))
}

func TestQueueUnknown(t *testing.T) {
	q := newQ()
	if q.State(99) != Unknown || q.Cancel(99) || q.Complete(99, Done) || q.ActiveCount(Up) != 0 {
		t.Fatal("unknown")
	}
	if _, ok := q.Direction(99); ok {
		t.Fatal("dir")
	}
	wantNone(t, q, Up)
}

func TestQueueIDsByState(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(3, Up, kSmall, ""))
	must(t, q.Enqueue(1, Up, kSmall, ""))
	must(t, q.Enqueue(2, Down, kSmall, ""))
	wantNext(t, q, Up, 3)
	eq := func(got, want []int64) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	eq(q.IDs(Up, Pending), []int64{1})
	eq(q.IDs(Up, Active), []int64{3})
	eq(q.IDs(Down, Pending), []int64{2})
	eq(q.IDs(Down, Active), nil)
	for _, id := range q.IDs(Up, Pending) {
		q.Cancel(id)
	}
	for _, id := range q.IDs(Up, Active) {
		q.Complete(id, Cancelled)
	}
	eq(q.IDs(Up, Pending), []int64{})
	eq(q.IDs(Up, Active), nil)
	eq(q.IDs(Up, Cancelled), []int64{1, 3})
	wantNone(t, q, Up)
}

func TestQueueOverlappingRequestsAllQueued(t *testing.T) {
	q := newQ()
	for _, id := range []int64{1, 2, 3, 4} {
		must(t, q.Enqueue(id, Down, kSmall, ""))
	}
	for _, id := range []int64{1, 2, 3, 4} {
		wantNext(t, q, Down, id)
	}
}

func TestQueueMixedDirections(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Down, kSmall, ""))
	must(t, q.Enqueue(2, Up, kSmall, ""))
	must(t, q.Enqueue(3, Down, kSmall, ""))
	if !slices.Equal(q.IDs(Down, Pending), []int64{1, 3}) || !slices.Equal(q.IDs(Up, Pending), []int64{2}) {
		t.Fatal("lanes")
	}
}

func TestQueueNextHandsOutEachOnce(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Down, kSmall, ""))
	must(t, q.Enqueue(2, Down, kSmall, ""))
	var out []int64
	for range 5 {
		if id, ok := q.Next(Down); ok {
			out = append(out, id)
		}
	}
	if !slices.Equal(out, []int64{1, 2}) {
		t.Fatal(out)
	}
}

func TestQueueCancelPendingLeavesNothing(t *testing.T) {
	q := newQ()
	must(t, q.Enqueue(1, Down, kSmall, ""))
	must(t, q.Enqueue(2, Up, kSmall, ""))
	for _, d := range []Dir{Up, Down} {
		for _, id := range q.IDs(d, Pending) {
			if !q.Cancel(id) {
				t.Fatal("cancel")
			}
		}
	}
	if q.PendingCount(Up)+q.PendingCount(Down) != 0 {
		t.Fatal("left")
	}
	must(t, q.Enqueue(3, Down, kSmall, ""))
	wantNext(t, q, Down, 3)
	wantNone(t, q, Down)
	wantNone(t, q, Up)
}
