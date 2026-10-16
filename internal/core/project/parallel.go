package project

import (
	"runtime"

	"github.com/nekiro/ots-creator/internal/workpool"
)

// spriteChunk is the number of sprite ids one worker reads at a time: about
// two asset sheets, so all workers together stay within the sheet cache and
// no sheet is decoded twice.
const spriteChunk = 256

// thingChunk returns the number of thing ids per work item for count ids: small
// enough that every core gets work (outfits are few but have many sprites).
func thingChunk(count uint32) uint32 {
	return max(8, count/uint32(runtime.GOMAXPROCS(0)*8))
}

// parallelRange calls fn for consecutive chunks of [first, last] on all
// cores. Chunks run in any order; fn must be safe for concurrent use. It
// stops early and returns the first error.
func parallelRange(first, last, size uint32, fn func(from, to uint32) error) error {
	if last < first {
		return nil
	}
	chunks := int((last-first)/size) + 1
	return workpool.Run(chunks, func(chunk int) error {
		from := first + uint32(chunk)*size
		return fn(from, min(from+size-1, last))
	})
}
