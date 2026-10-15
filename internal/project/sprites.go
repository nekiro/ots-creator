package project

import (
	"fmt"
	"hash/maphash"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/spr"
)

// spriteBase is the read-only sprite source of a store: an spr file or the
// sprite sheets of an asset folder.
type spriteBase interface {
	Count() uint32
	Compressed(id uint32) ([]byte, error)
}

type sprFileBase struct{ f *spr.File }

func (b sprFileBase) Count() uint32                        { return b.f.Count }
func (b sprFileBase) Compressed(id uint32) ([]byte, error) { return b.f.Compressed(id) }

// spriteStore overlays edited sprites on top of a read-only base.
// All data is kept compressed with the store's transparency setting.
type spriteStore struct {
	base        spriteBase
	size        int
	transparent bool
	n           uint32
	overlay     map[uint32][]byte // edited sprites; nil value = empty sprite
	// alias maps a sprite to the base sprite it reads (moved sprites), so
	// moving a sprite never decodes it.
	alias map[uint32]uint32

	// Content hashes and compressed sizes of base sprites, filled on first
	// use (the base never changes). a == 0: not hashed yet; a == 1: empty.
	seeds  [2]maphash.Seed
	hashes []baseInfo
}

type baseInfo struct {
	a, b atomic.Uint64
	size atomic.Uint32
}

// spriteHash identifies sprite content; the zero value is an empty sprite.
type spriteHash [2]uint64

// Kinds of spriteSlot.
const (
	slotBase  = iota // reads the base sprite of the same id
	slotData         // reads data
	slotAlias        // reads base sprite alias
)

// spriteSlot is how one sprite id resolves, kept by undo without reading
// the sprite.
type spriteSlot struct {
	kind  uint8
	alias uint32
	data  []byte
}

func newSpriteStore(base spriteBase, size int, transparent bool) *spriteStore {
	s := &spriteStore{base: base, size: size, transparent: transparent, overlay: map[uint32][]byte{}, alias: map[uint32]uint32{}}
	s.seeds = [2]maphash.Seed{maphash.MakeSeed(), maphash.MakeSeed()}
	if base != nil {
		s.n = base.Count()
		s.hashes = make([]baseInfo, s.n+1)
	}
	return s
}

func sprFile(f *spr.File) spriteBase {
	if f == nil {
		return nil
	}
	return sprFileBase{f}
}

// baseCount is the number of sprites of the base.
func (s *spriteStore) baseCount() uint32 {
	if s.base == nil {
		return 0
	}
	return s.base.Count()
}

// edited reports whether a sprite differs from the base.
func (s *spriteStore) edited(id uint32) bool {
	_, ok := s.overlay[id]
	_, moved := s.alias[id]
	return ok || moved || id > s.baseCount()
}

func (s *spriteStore) count() uint32 { return s.n }

// compressed returns sprite data; empty sprites return nil.
func (s *spriteStore) compressed(id uint32) ([]byte, error) {
	if id == 0 || id > s.n {
		return nil, fmt.Errorf("sprite id %d out of range [1,%d]", id, s.n)
	}
	if c, ok := s.overlay[id]; ok {
		return c, nil
	}
	if a, ok := s.alias[id]; ok {
		return s.base.Compressed(a)
	}
	if id <= s.baseCount() {
		return s.base.Compressed(id)
	}
	return nil, nil
}

// baseID returns the base sprite that id reads, if it reads one.
func (s *spriteStore) baseID(id uint32) (uint32, bool) {
	if id == 0 || id > s.n {
		return 0, false
	}
	if _, ok := s.overlay[id]; ok {
		return 0, false
	}
	if a, ok := s.alias[id]; ok {
		return a, true
	}
	return id, id <= s.baseCount()
}

// hash returns the content hash of a sprite. Base sprites are read once;
// later calls are free.
func (s *spriteStore) hash(id uint32) (spriteHash, error) {
	if id == 0 || id > s.n {
		return spriteHash{}, fmt.Errorf("sprite id %d out of range [1,%d]", id, s.n)
	}
	if c, ok := s.overlay[id]; ok {
		return s.hashData(c), nil
	}
	if b, ok := s.baseID(id); ok {
		return s.baseHash(b)
	}
	return spriteHash{}, nil
}

// baseHash returns the content hash of base sprite b, reading it only the
// first time.
func (s *spriteStore) baseHash(b uint32) (spriteHash, error) {
	if b == 0 || b > s.baseCount() {
		return spriteHash{}, fmt.Errorf("base sprite id %d out of range [1,%d]", b, s.baseCount())
	}
	e := &s.hashes[b]
	if a := e.a.Load(); a != 0 {
		if a == 1 {
			return spriteHash{}, nil
		}
		return spriteHash{a, e.b.Load()}, nil
	}
	c, err := s.base.Compressed(b)
	if err != nil {
		return spriteHash{}, err
	}
	h := s.hashData(c)
	// a last: it marks the entry complete.
	e.size.Store(uint32(len(c)))
	e.b.Store(h[1])
	if h == (spriteHash{}) {
		e.a.Store(1)
	} else {
		e.a.Store(h[0])
	}
	return h, nil
}

// dataSize returns the compressed size of a sprite in bytes.
func (s *spriteStore) dataSize(id uint32) (int, error) {
	if c, ok := s.overlay[id]; ok {
		return len(c), nil
	}
	b, ok := s.baseID(id)
	if !ok {
		return 0, nil
	}
	if _, err := s.baseHash(b); err != nil {
		return 0, err
	}
	return int(s.hashes[b].size.Load()), nil
}

func (s *spriteStore) hashData(c []byte) spriteHash {
	if len(c) == 0 {
		return spriteHash{}
	}
	return spriteHash{maphash.Bytes(s.seeds[0], c) | 2, maphash.Bytes(s.seeds[1], c)} // a is never 0 or 1
}

func (s *spriteStore) pixels(id uint32) ([]byte, error) {
	c, err := s.compressed(id)
	if err != nil {
		return nil, err
	}
	return spr.Decompress(c, s.size, s.transparent)
}

func (s *spriteStore) set(id uint32, c []byte) {
	if len(c) == 0 {
		c = nil
	}
	delete(s.alias, id)
	s.overlay[id] = c
}

// unset drops an edit: the sprite reads from the base again.
func (s *spriteStore) unset(id uint32) {
	delete(s.overlay, id)
	delete(s.alias, id)
}

// slot returns how a sprite resolves now.
func (s *spriteStore) slot(id uint32) spriteSlot {
	if c, ok := s.overlay[id]; ok {
		return spriteSlot{kind: slotData, data: c}
	}
	if a, ok := s.alias[id]; ok {
		return spriteSlot{kind: slotAlias, alias: a}
	}
	return spriteSlot{kind: slotBase}
}

// setSlot makes id resolve as sl.
func (s *spriteStore) setSlot(id uint32, sl spriteSlot) {
	switch {
	case sl.kind == slotData:
		s.set(id, sl.data)
	case sl.kind == slotAlias && sl.alias != id:
		delete(s.overlay, id)
		s.alias[id] = sl.alias
	default:
		s.unset(id)
	}
}

// slotOf returns a slot that reads what sprite src reads now, for use at
// another id.
func (s *spriteStore) slotOf(src uint32) spriteSlot {
	sl := s.slot(src)
	if sl.kind == slotBase {
		if src > s.baseCount() {
			return spriteSlot{kind: slotData}
		}
		return spriteSlot{kind: slotAlias, alias: src}
	}
	return sl
}

// setCount changes the sprite count. Sprites past the new count are
// dropped from the overlay; growing exposes empty sprites.
func (s *spriteStore) setCount(n uint32) {
	// Walk whichever is smaller: the dropped id range or the edits. Adding
	// sprites one by one must not scan all edits each time.
	if n < s.n {
		if int(s.n-n) < len(s.overlay)+len(s.alias) {
			for id := n + 1; id <= s.n; id++ {
				delete(s.overlay, id)
				delete(s.alias, id)
			}
		} else {
			for id := range s.overlay {
				if id > n {
					delete(s.overlay, id)
				}
			}
			for id := range s.alias {
				if id > n {
					delete(s.alias, id)
				}
			}
		}
	}
	for id := s.n + 1; id <= n; id++ {
		// Hide base sprites that were removed earlier and are now re-added.
		if id <= s.baseCount() {
			if _, ok := s.overlay[id]; !ok {
				s.overlay[id] = nil
			}
		}
	}
	s.n = n
}

// eachHash calls fn with the content hash of every sprite, from all cores
// (see parallelRange). Callers hold the project lock.
func (s *spriteStore) eachHash(fn func(id uint32, h spriteHash)) error {
	return parallelRange(1, s.count(), spriteChunk, func(from, to uint32) error {
		for id := from; id <= to; id++ {
			h, err := s.hash(id)
			if err != nil {
				return err
			}
			fn(id, h)
		}
		return nil
	})
}

// source returns an spr.Source that encodes with the given transparency.
func (s *spriteStore) source(transparent bool) spr.Source {
	if transparent == s.transparent {
		return storeSource{s}
	}
	return &transcodeSource{s: s, to: transparent, cache: map[uint32][]byte{}}
}

type storeSource struct{ s *spriteStore }

func (x storeSource) Count() uint32                        { return x.s.n }
func (x storeSource) Compressed(id uint32) ([]byte, error) { return x.s.compressed(id) }

type transcodeSource struct {
	s     *spriteStore
	to    bool
	cache map[uint32][]byte
}

func (x *transcodeSource) Count() uint32 { return x.s.n }

func (x *transcodeSource) Compressed(id uint32) ([]byte, error) {
	if c, ok := x.cache[id]; ok {
		return c, nil
	}
	c, err := x.s.compressed(id)
	if err != nil {
		return nil, err
	}
	c, err = spr.Transcode(c, x.s.size, x.s.transparent, x.to)
	if err != nil {
		return nil, err
	}
	x.cache[id] = c
	return c, nil
}
