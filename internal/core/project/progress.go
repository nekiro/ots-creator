package project

import (
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/formats/spr"
)

// counter reports work done to a CompileOptions.Progress.
type counter struct {
	done   atomic.Int64
	total  int
	report func(done, total int)
}

func newCounter(total int, report func(done, total int)) *counter {
	if report == nil {
		report = func(int, int) {}
	}
	return &counter{total: total, report: report}
}

func (c *counter) tick() { c.report(int(c.done.Add(1)), c.total) }

// countingSource ticks a counter for every sprite read.
type countingSource struct {
	spr.Source
	counter *counter
}

func (s countingSource) Compressed(id uint32) ([]byte, error) {
	s.counter.tick()
	return s.Source.Compressed(id)
}
