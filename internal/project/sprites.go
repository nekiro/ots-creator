package project

import (
	"fmt"

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
}

func newSpriteStore(base spriteBase, size int, transparent bool) *spriteStore {
	s := &spriteStore{base: base, size: size, transparent: transparent, overlay: map[uint32][]byte{}}
	if base != nil {
		s.n = base.Count()
	}
	return s
}

func sprFile(f *spr.File) spriteBase {
	if f == nil {
		return nil
	}
	return sprFileBase{f}
}

// edited reports whether a sprite differs from the base.
func (s *spriteStore) edited(id uint32) bool {
	_, ok := s.overlay[id]
	return ok || s.base == nil || id > s.base.Count()
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
	if s.base != nil && id <= s.base.Count() {
		return s.base.Compressed(id)
	}
	return nil, nil
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
	s.overlay[id] = c
}

// unset drops an edit: the sprite reads from the base again.
func (s *spriteStore) unset(id uint32) { delete(s.overlay, id) }

// setCount changes the sprite count. Sprites past the new count are
// dropped from the overlay; growing exposes empty sprites.
func (s *spriteStore) setCount(n uint32) {
	// Walk whichever is smaller: the dropped id range or the overlay. Adding
	// sprites one by one must not scan the whole overlay each time.
	if n < s.n {
		if int(s.n-n) < len(s.overlay) {
			for id := n + 1; id <= s.n; id++ {
				delete(s.overlay, id)
			}
		} else {
			for id := range s.overlay {
				if id > n {
					delete(s.overlay, id)
				}
			}
		}
	}
	for id := s.n + 1; id <= n; id++ {
		// Hide base sprites that were removed earlier and are now re-added.
		if s.base != nil && id <= s.base.Count() {
			if _, ok := s.overlay[id]; !ok {
				s.overlay[id] = nil
			}
		}
	}
	s.n = n
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
