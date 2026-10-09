package project

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// spriteChunk is the number of sprite ids one worker reads at a time: about
// two asset sheets, so all workers together stay within the sheet cache and
// no sheet is decoded twice.
const spriteChunk = 256

// thingChunk returns the number of thing ids per work item for n ids: small
// enough that every core gets work (outfits are few but have many sprites).
func thingChunk(n uint32) uint32 {
	return max(8, n/uint32(runtime.GOMAXPROCS(0)*8))
}

// parallelRange calls fn for consecutive chunks of [first, last] on all
// cores. Chunks run in any order; fn must be safe for concurrent use. It
// stops early and returns the first error.
func parallelRange(first, last, size uint32, fn func(from, to uint32) error) error {
	if last < first {
		return nil
	}
	chunks := int((last-first)/size) + 1
	var next atomic.Int64
	var failed atomic.Bool
	var firstErr error
	var once sync.Once
	var wg sync.WaitGroup
	for range min(chunks, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !failed.Load() {
				i := next.Add(1) - 1
				if i >= int64(chunks) {
					return
				}
				from := first + uint32(i)*size
				if err := fn(from, min(from+size-1, last)); err != nil {
					once.Do(func() { firstErr = err })
					failed.Store(true)
				}
			}
		}()
	}
	wg.Wait()
	return firstErr
}

// eachCompressed calls fn with the compressed data of every sprite, from
// all cores (see parallelRange). Callers hold the project lock.
func (s *spriteStore) eachCompressed(fn func(id uint32, c []byte) error) error {
	return parallelRange(1, s.count(), spriteChunk, func(from, to uint32) error {
		for id := from; id <= to; id++ {
			c, err := s.compressed(id)
			if err != nil {
				return err
			}
			if err := fn(id, c); err != nil {
				return err
			}
		}
		return nil
	})
}
