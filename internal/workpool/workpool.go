package workpool

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Run calls fn for every index in [0, count) on all cores. Indexes run in
// any order; fn must be safe for concurrent use. It stops at the first
// error and returns it.
func Run(count int, fn func(index int) error) error {
	var next atomic.Int64
	var failed atomic.Bool
	var firstErr error
	var errOnce sync.Once
	var wg sync.WaitGroup
	for range min(count, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !failed.Load() {
				index := int(next.Add(1) - 1)
				if index >= count {
					return
				}
				if err := fn(index); err != nil {
					errOnce.Do(func() { firstErr = err })
					failed.Store(true)
					return
				}
			}
		}()
	}
	wg.Wait()
	return firstErr
}
