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

type sprFileBase struct{ file *spr.File }

func (b sprFileBase) Count() uint32                        { return b.file.Count }
func (b sprFileBase) Compressed(id uint32) ([]byte, error) { return b.file.Compressed(id) }

// spriteStore overlays edited sprites on top of a read-only base.
// All data is kept compressed with the store's transparency setting.
type spriteStore struct {
	base        spriteBase
	size        int
	transparent bool
	spriteCount uint32
	overlay     map[uint32][]byte // edited sprites; nil value = empty sprite
	// alias maps a sprite to the base sprite it reads (moved sprites), so
	// moving a sprite never decodes it.
	alias map[uint32]uint32

	// Content hashes and compressed sizes of base sprites, filled on first
	// use (the base never changes). first == 0: not hashed yet; first == 1:
	// empty.
	seeds  [2]maphash.Seed
	hashes []baseInfo
}

type baseInfo struct {
	first, second atomic.Uint64
	size          atomic.Uint32
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
	store := &spriteStore{base: base, size: size, transparent: transparent, overlay: map[uint32][]byte{}, alias: map[uint32]uint32{}}
	store.seeds = [2]maphash.Seed{maphash.MakeSeed(), maphash.MakeSeed()}
	if base != nil {
		store.spriteCount = base.Count()
		store.hashes = make([]baseInfo, store.spriteCount+1)
	}
	return store
}

func sprFile(file *spr.File) spriteBase {
	if file == nil {
		return nil
	}
	return sprFileBase{file}
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

func (s *spriteStore) count() uint32 { return s.spriteCount }

// compressed returns sprite data; empty sprites return nil.
func (s *spriteStore) compressed(id uint32) ([]byte, error) {
	if id == 0 || id > s.spriteCount {
		return nil, fmt.Errorf("sprite id %d out of range [1,%d]", id, s.spriteCount)
	}
	if data, ok := s.overlay[id]; ok {
		return data, nil
	}
	if aliasID, ok := s.alias[id]; ok {
		return s.base.Compressed(aliasID)
	}
	if id <= s.baseCount() {
		return s.base.Compressed(id)
	}
	return nil, nil
}

// baseID returns the base sprite that id reads, if it reads one.
func (s *spriteStore) baseID(id uint32) (uint32, bool) {
	if id == 0 || id > s.spriteCount {
		return 0, false
	}
	if _, ok := s.overlay[id]; ok {
		return 0, false
	}
	if aliasID, ok := s.alias[id]; ok {
		return aliasID, true
	}
	return id, id <= s.baseCount()
}

// hash returns the content hash of a sprite. Base sprites are read once;
// later calls are free.
func (s *spriteStore) hash(id uint32) (spriteHash, error) {
	if id == 0 || id > s.spriteCount {
		return spriteHash{}, fmt.Errorf("sprite id %d out of range [1,%d]", id, s.spriteCount)
	}
	if data, ok := s.overlay[id]; ok {
		return s.hashData(data), nil
	}
	if baseSprite, ok := s.baseID(id); ok {
		return s.baseHash(baseSprite)
	}
	return spriteHash{}, nil
}

// baseHash returns the content hash of a base sprite, reading it only the
// first time.
func (s *spriteStore) baseHash(baseSprite uint32) (spriteHash, error) {
	if baseSprite == 0 || baseSprite > s.baseCount() {
		return spriteHash{}, fmt.Errorf("base sprite id %d out of range [1,%d]", baseSprite, s.baseCount())
	}
	entry := &s.hashes[baseSprite]
	if first := entry.first.Load(); first != 0 {
		if first == 1 {
			return spriteHash{}, nil
		}
		return spriteHash{first, entry.second.Load()}, nil
	}
	data, err := s.base.Compressed(baseSprite)
	if err != nil {
		return spriteHash{}, err
	}
	contentHash := s.hashData(data)
	// first is stored last: it marks the entry complete.
	entry.size.Store(uint32(len(data)))
	entry.second.Store(contentHash[1])
	if contentHash == (spriteHash{}) {
		entry.first.Store(1)
	} else {
		entry.first.Store(contentHash[0])
	}
	return contentHash, nil
}

// dataSize returns the compressed size of a sprite in bytes.
func (s *spriteStore) dataSize(id uint32) (int, error) {
	if data, ok := s.overlay[id]; ok {
		return len(data), nil
	}
	baseSprite, ok := s.baseID(id)
	if !ok {
		return 0, nil
	}
	if _, err := s.baseHash(baseSprite); err != nil {
		return 0, err
	}
	return int(s.hashes[baseSprite].size.Load()), nil
}

func (s *spriteStore) hashData(data []byte) spriteHash {
	if len(data) == 0 {
		return spriteHash{}
	}
	return spriteHash{maphash.Bytes(s.seeds[0], data) | 2, maphash.Bytes(s.seeds[1], data)} // first is never 0 or 1
}

func (s *spriteStore) pixels(id uint32) ([]byte, error) {
	data, err := s.compressed(id)
	if err != nil {
		return nil, err
	}
	return spr.Decompress(data, s.size, s.transparent)
}

func (s *spriteStore) set(id uint32, data []byte) {
	if len(data) == 0 {
		data = nil
	}
	delete(s.alias, id)
	s.overlay[id] = data
}

// unset drops an edit: the sprite reads from the base again.
func (s *spriteStore) unset(id uint32) {
	delete(s.overlay, id)
	delete(s.alias, id)
}

// slot returns how a sprite resolves now.
func (s *spriteStore) slot(id uint32) spriteSlot {
	if data, ok := s.overlay[id]; ok {
		return spriteSlot{kind: slotData, data: data}
	}
	if aliasID, ok := s.alias[id]; ok {
		return spriteSlot{kind: slotAlias, alias: aliasID}
	}
	return spriteSlot{kind: slotBase}
}

// setSlot makes id resolve as slot.
func (s *spriteStore) setSlot(id uint32, slot spriteSlot) {
	switch {
	case slot.kind == slotData:
		s.set(id, slot.data)
	case slot.kind == slotAlias && slot.alias != id:
		delete(s.overlay, id)
		s.alias[id] = slot.alias
	default:
		s.unset(id)
	}
}

// slotOf returns a slot that reads what sprite sourceID reads now, for use at
// another id.
func (s *spriteStore) slotOf(sourceID uint32) spriteSlot {
	slot := s.slot(sourceID)
	if slot.kind == slotBase {
		if sourceID > s.baseCount() {
			return spriteSlot{kind: slotData}
		}
		return spriteSlot{kind: slotAlias, alias: sourceID}
	}
	return slot
}

// setCount changes the sprite count. Sprites past the new count are
// dropped from the overlay; growing exposes empty sprites.
func (s *spriteStore) setCount(newCount uint32) {
	// Walk whichever is smaller: the dropped id range or the edits. Adding
	// sprites one by one must not scan all edits each time.
	if newCount < s.spriteCount {
		if int(s.spriteCount-newCount) < len(s.overlay)+len(s.alias) {
			for id := newCount + 1; id <= s.spriteCount; id++ {
				delete(s.overlay, id)
				delete(s.alias, id)
			}
		} else {
			for id := range s.overlay {
				if id > newCount {
					delete(s.overlay, id)
				}
			}
			for id := range s.alias {
				if id > newCount {
					delete(s.alias, id)
				}
			}
		}
	}
	for id := s.spriteCount + 1; id <= newCount; id++ {
		// Hide base sprites that were removed earlier and are now re-added.
		if id <= s.baseCount() {
			if _, ok := s.overlay[id]; !ok {
				s.overlay[id] = nil
			}
		}
	}
	s.spriteCount = newCount
}

// eachHash calls fn with the content hash of every sprite, from all cores
// (see parallelRange). Callers hold the project lock.
func (s *spriteStore) eachHash(fn func(id uint32, contentHash spriteHash)) error {
	return parallelRange(1, s.count(), spriteChunk, func(from, to uint32) error {
		for id := from; id <= to; id++ {
			contentHash, err := s.hash(id)
			if err != nil {
				return err
			}
			fn(id, contentHash)
		}
		return nil
	})
}

// source returns an spr.Source that encodes with the given transparency.
func (s *spriteStore) source(transparent bool) spr.Source {
	if transparent == s.transparent {
		return storeSource{s}
	}
	return &transcodeSource{store: s, transparent: transparent, cache: map[uint32][]byte{}}
}

type storeSource struct{ store *spriteStore }

func (source storeSource) Count() uint32 { return source.store.spriteCount }
func (source storeSource) Compressed(id uint32) ([]byte, error) {
	return source.store.compressed(id)
}

// transcodeSource reads the store re-encoded with another transparency.
type transcodeSource struct {
	store       *spriteStore
	transparent bool
	cache       map[uint32][]byte
}

func (source *transcodeSource) Count() uint32 { return source.store.spriteCount }

func (source *transcodeSource) Compressed(id uint32) ([]byte, error) {
	if data, ok := source.cache[id]; ok {
		return data, nil
	}
	data, err := source.store.compressed(id)
	if err != nil {
		return nil, err
	}
	data, err = spr.Transcode(data, source.store.size, source.store.transparent, source.transparent)
	if err != nil {
		return nil, err
	}
	source.cache[id] = data
	return data, nil
}
