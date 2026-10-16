package maprender

import (
	"bytes"
	"compress/flate"
	"runtime"
	"sync"

	"github.com/nekiro/ots-creator/internal/otmap"
)

// deflateEnd is an empty final deflate block: it ends a stream of sync
// flushed parts.
var deflateEnd = []byte{0x03, 0x00}

// BuildGeometry builds the geometry of chunks on all cores, in the order
// of keys, as one raw deflate stream (see BuildGeometryParts). It also
// returns the size before compression.
//
// On Windows the webview gets a response in one copy on the UI thread, so
// its size is what a load waits for: geometry compresses about 4 times.
func BuildGeometry(scene *Scene, keys []otmap.ChunkKey, options GeometryOptions) ([]byte, int, error) {
	parts, rawSize, err := BuildGeometryParts(scene, keys, options, true)
	if err != nil {
		return nil, 0, err
	}
	size := len(deflateEnd)
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	// Every part ends in a sync flush (no back references cross parts, so
	// they join into one stream).
	return append(out, deflateEnd...), rawSize, nil
}

// BuildGeometryParts builds the geometry of each key on all cores, each
// part compressed (raw deflate, sync flushed) when pack is set. It also
// returns the size before compression.
func BuildGeometryParts(scene *Scene, keys []otmap.ChunkKey, options GeometryOptions, pack bool) ([][]byte, int, error) {
	parts := make([][]byte, len(keys))
	var (
		lock     sync.Mutex
		nextKey  int
		rawSize  int
		firstErr error
		workers  sync.WaitGroup
	)
	// take hands out the next key; done records a built part.
	take := func() (int, bool) {
		lock.Lock()
		defer lock.Unlock()
		index := nextKey
		nextKey++
		return index, index < len(keys) && firstErr == nil
	}
	done := func(raw int, err error) {
		lock.Lock()
		defer lock.Unlock()
		rawSize += raw
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for range min(runtime.NumCPU(), len(keys)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			builder := newGeometryBuilder(scene, options)
			var raw []byte
			var packed bytes.Buffer
			compressor, err := flate.NewWriter(&packed, flate.BestSpeed)
			if err != nil {
				done(0, err)
				return
			}
			for {
				index, ok := take()
				if !ok {
					return
				}
				raw = builder.chunk(keys[index], raw[:0])
				if !pack {
					parts[index] = bytes.Clone(raw)
					done(len(raw), nil)
					continue
				}
				packed.Reset()
				compressor.Reset(&packed)
				_, err := compressor.Write(raw)
				if err == nil {
					err = compressor.Flush()
				}
				parts[index] = bytes.Clone(packed.Bytes())
				done(len(raw), err)
			}
		}()
	}
	workers.Wait()
	return parts, rawSize, firstErr
}
