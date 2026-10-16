package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/workpool"
)

// EventProgress carries a Progress while a long operation runs.
const EventProgress = "app:progress"

// Progress reports how far a long operation is.
type Progress struct {
	Label string `json:"label"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	// Percent shows the progress as a percentage instead of done/total.
	Percent bool `json:"percent"`
}

// ErrCanceled is returned by a long operation stopped with Cancel.
var ErrCanceled = errors.New("canceled")

// progressEvery is the minimum time between progress events.
const progressEvery = 150 * time.Millisecond

// reporter returns a progress callback for label that sends at most one
// event per progressEvery. It is safe for concurrent use.
func (s *Session) reporter(label string) func(done, total int) {
	var mu sync.Mutex
	var last time.Time
	return func(done, total int) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(last) < progressEvery {
			return
		}
		last = time.Now()
		s.notify(EventProgress, Progress{Label: label, Done: done, Total: total, Percent: true})
	}
}

// parallel runs fn for 0..n-1 on all cores and reports progress. It stops
// at the first error and returns it, or ErrCanceled after Session.Cancel.
func (s *Session) parallel(label string, n int, fn func(i int) error) error {
	s.canceled.Store(false)
	s.exporting.Add(1)
	defer s.exporting.Add(-1)
	var done atomic.Int64
	stop, stopped := make(chan struct{}), make(chan struct{})
	ticker := time.NewTicker(progressEvery)
	defer ticker.Stop()
	go func() {
		defer close(stopped)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.notify(EventProgress, Progress{Label: label, Done: int(done.Load()), Total: n})
			}
		}
	}()
	err := workpool.Run(n, func(i int) error {
		if s.canceled.Load() {
			return ErrCanceled
		}
		if err := fn(i); err != nil {
			return err
		}
		done.Add(1)
		return nil
	})
	close(stop)
	<-stopped // events are never sent concurrently
	s.notify(EventProgress, Progress{Label: label, Done: n, Total: n})
	return err
}
