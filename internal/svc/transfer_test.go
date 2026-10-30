package svc

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"snbapp/internal/config"
	"snbapp/internal/files"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
)

func newTestTransfers(ctx context.Context) *Transfers {
	return &Transfers{c: NewCore("test", slog.Default()), login: ctx}
}

// 검사를 지난 자동 업로드는 사람의 답을 기다리므로 사람의 업로드가 밀어내지 못한다(원본 ViewController.cpp:2404).
func TestHandToHuman(t *testing.T) {
	ts := newTestTransfers(context.Background())
	ctx, cancel, token, err := ts.claim(true)
	if err != nil || ctx == nil {
		t.Fatal(err)
	}
	ts.handToHuman(token)
	if _, _, _, err := ts.claim(false); !errors.Is(err, ErrUploadBusy) {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("답을 기다리는 업로드가 밀려났다")
	}
	ts.release(token, cancel)
	if _, cancel, _, err := ts.claim(false); err != nil {
		t.Fatal("놓은 뒤에는 받아야 한다", err)
	} else {
		cancel()
	}
}

func TestQuietFail(t *testing.T) {
	loggedOut := &transfer.Fail{Code: transfer.CodeLoggedOut}
	changed := &transfer.Fail{Code: transfer.CodeChanged}
	folder := &transfer.Fail{Code: transfer.CodeFolderChanged}
	zip := &transfer.Fail{Code: transfer.CodeZip}
	if !quietFail(loggedOut, false) || !quietFail(changed, true) || !quietFail(folder, true) || !quietFail(zip, true) {
		t.Fatal("로그아웃·자동 업로드의 검사 뒤 바뀜·압축 실패는 창을 띄우지 않는다")
	}
	if quietFail(changed, false) || quietFail(folder, false) || quietFail(zip, false) || quietFail(&transfer.Fail{Code: transfer.CodeOpen}, true) {
		t.Fatal("나머지 실패는 창을 띄운다")
	}
}

// 실패 표시가 있으면 한 번만 다시 걸고, 조회가 이어져도 시계를 밀지 않는다.
func TestRetryAutoDownload(t *testing.T) {
	st, err := config.Open(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	cur := st.Get()
	cur.Auto = map[int]config.AutoTransfer{0: {DownloadUse: true}}
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	ts := newTestTransfers(context.Background())
	ts.c.settings = st
	ts.c.sess = &nbs.Session{}
	ts.retryAutoDownload()
	if ts.autoDn != nil {
		t.Fatal("실패가 없으면 걸지 않는다")
	}
	ts.autoRetry = true
	ts.retryAutoDownload()
	first := ts.autoDn
	if first == nil {
		t.Fatal("실패가 있으면 건다")
	}
	defer first.Stop()
	ts.retryAutoDownload()
	if ts.autoDn != first {
		t.Fatal("시계가 다시 걸렸다")
	}
}

// 파일·폴더 선택 창은 탐색기의 마지막 폴더를 따르지 않는다.
func TestDialogFolder(t *testing.T) {
	st, err := config.Open(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	cur := st.Get()
	cur.LastFolder = t.TempDir()
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	c := NewCore("test", slog.Default())
	c.settings = st
	if c.dialogFolder() == cur.LastFolder || c.startFolder().Path != cur.LastFolder {
		t.Fatal(c.dialogFolder(), c.startFolder().Path)
	}
	cur.InitFolder = t.TempDir()
	if err := st.Set(cur); err != nil {
		t.Fatal(err)
	}
	if c.dialogFolder() != cur.InitFolder {
		t.Fatal(c.dialogFolder())
	}
}

func TestUploadStartText(t *testing.T) {
	if got := uploadStartText([]string{"a"}); got != "a  업로드 시작" {
		t.Fatal(got)
	}
	if got := uploadStartText([]string{"a", "b", "c"}); got != "a 외 2 건 업로드 시작" {
		t.Fatal(got)
	}
}

// 겹치는 이름만 묻고, 답대로 덮어쓰기·건너뛰기를 돌려준다. 결재 받기도 이 함수를 쓴다.
func TestAskConflicts(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ts := newTestTransfers(context.Background())
	type out struct {
		over, skip map[int]bool
		ok         bool
	}
	ch := make(chan out, 1)
	go func() {
		over, skip, ok := ts.askConflicts([]files.Candidate{{Dir: dir, Name: "a.txt"}, {Dir: dir, Name: "new.txt"}, {Dir: dir, Name: "b.txt"}},
			func(int) bool { return false })
		ch <- out{over, skip, ok}
	}()
	answer := func(gen int64, choice int) {
		t.Helper()
		for range 500 {
			ts.mu.Lock()
			a := ts.ask
			ts.mu.Unlock()
			if a != nil && a.gen == gen {
				a.ch <- conflictAnswer{choice: choice}
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("질문이 오지 않았다")
	}
	answer(1, ConflictOverwrite)
	answer(2, ConflictCancel)
	r := <-ch
	if !r.ok || len(r.over) != 1 || !r.over[0] || len(r.skip) != 1 || !r.skip[2] {
		t.Fatalf("%+v", r)
	}
}

// 로그아웃이면 묻기를 멈춘다.
func TestAskConflictsLogout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ts := newTestTransfers(ctx)
	if _, _, ok := ts.askConflicts([]files.Candidate{{Dir: dir, Name: "a.txt"}}, func(int) bool { return false }); ok {
		t.Fatal("로그아웃인데 끝까지 물었다")
	}
}
