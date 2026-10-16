package workpool

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestRunVisitsEveryIndex(t *testing.T) {
	const count = 1000
	var seen [count]atomic.Int32
	if err := Run(count, func(index int) error {
		seen[index].Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for index := range seen {
		if got := seen[index].Load(); got != 1 {
			t.Fatalf("index %d ran %d times", index, got)
		}
	}
}

func TestRunStopsAtError(t *testing.T) {
	failure := errors.New("failure")
	err := Run(1000, func(index int) error {
		if index == 10 {
			return failure
		}
		return nil
	})
	if !errors.Is(err, failure) {
		t.Fatalf("got %v, want %v", err, failure)
	}
}

func TestRunNothing(t *testing.T) {
	if err := Run(0, func(int) error { t.Fatal("called"); return nil }); err != nil {
		t.Fatal(err)
	}
}
