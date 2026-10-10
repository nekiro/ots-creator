package app

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
)

func TestWindowCloseGuard(t *testing.T) {
	s, ps, _, _, rec := newEnv(t)
	closed := 0
	ws := NewWindowService(s, func() { closed++ })
	block := BlockClose(ws)

	if block() {
		t.Fatal("blocked with no client open")
	}

	ws.SetUnapplied(true)
	if !block() {
		t.Fatal("not blocked with unapplied edits")
	}
	if !slices.Contains(rec.events, EventCloseRequested) {
		t.Fatal("close request not emitted")
	}
	ws.SetUnapplied(false)

	// A running export blocks too.
	err := s.parallel("test", 1, func(int) error {
		if !block() {
			t.Error("not blocked during an export")
		}
		return nil
	})
	if err != nil || block() {
		t.Fatalf("after export: %v, blocked %v", err, block())
	}

	// A new client is not compiled yet.
	ps.New(v1098(), client.Features{})
	if !block() {
		t.Fatal("not blocked with an uncompiled client")
	}

	ws.Quit()
	if closed != 1 {
		t.Fatalf("close called %d times, want 1", closed)
	}
	if block() {
		t.Fatal("blocked after Quit")
	}
}
