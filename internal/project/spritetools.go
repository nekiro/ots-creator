package project

import (
	"fmt"
	"hash/maphash"
	"slices"

	"github.com/nekiro/ots-creator/internal/thing"
)

// Sprite filters for FindSprites.
const (
	SpritesUnused    = "unused"    // no thing references the sprite
	SpritesEmpty     = "empty"     // no visible pixels
	SpritesDuplicate = "duplicate" // same pixels as a sprite with a lower id
)

// FindSprites returns the ids of the sprites that match filter, in order.
func (p *Project) FindSprites(filter string) ([]uint32, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := p.sprites.count()
	out := []uint32{}
	switch filter {
	case SpritesUnused:
		used := p.usedSpritesLocked()
		for id := uint32(1); id <= n; id++ {
			if !used[id] {
				out = append(out, id)
			}
		}
	case SpritesEmpty:
		empty := make([]bool, n+1)
		err := p.sprites.eachCompressed(func(id uint32, c []byte) error {
			empty[id] = len(c) == 0
			return nil
		})
		if err != nil {
			return nil, err
		}
		for id := uint32(1); id <= n; id++ {
			if empty[id] {
				out = append(out, id)
			}
		}
	case SpritesDuplicate:
		dups, err := p.duplicateSprites()
		if err != nil {
			return nil, err
		}
		for id := uint32(1); id <= n; id++ {
			if dups[id] != 0 {
				out = append(out, id)
			}
		}
	default:
		return nil, fmt.Errorf("unknown sprite filter %q", filter)
	}
	return out, nil
}

// usedSpritesLocked marks every sprite id some thing references.
func (p *Project) usedSpritesLocked() []bool {
	used := make([]bool, p.sprites.count()+1)
	for _, c := range thing.Categories {
		for _, t := range p.things.Things[c] {
			if t == nil {
				continue
			}
			for _, g := range t.FrameGroups {
				for _, id := range g.Sprites {
					if int(id) < len(used) {
						used[id] = true
					}
				}
			}
		}
	}
	return used
}

// SpriteUsers lists the things that reference sprite id.
func (p *Project) SpriteUsers(id uint32) []ThingRef {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := []ThingRef{}
	for _, c := range thing.Categories {
		for _, t := range p.things.Things[c] {
			if t != nil && slices.Contains(t.SpriteIDs(), id) {
				out = append(out, ThingRef{Category: c, ID: t.ID})
			}
		}
	}
	return out
}

// ReplaceSpriteRefs points every reference to a sprite in from at sprite to
// (0 clears them) in all things. It returns the number of changed things;
// the sprites themselves stay. It is one undoable edit.
func (p *Project) ReplaceSpriteRefs(from []uint32, to uint32) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if to > p.sprites.count() {
		return 0, fmt.Errorf("sprite %d does not exist (max %d)", to, p.sprites.count())
	}
	replace := map[uint32]bool{}
	for _, id := range from {
		if id != 0 && id != to {
			replace[id] = true
		}
	}
	if len(replace) == 0 {
		return 0, nil
	}
	r := p.record(fmt.Sprintf("Replace %d sprite reference(s) with %d", len(replace), to))
	changed := 0
	for _, c := range thing.Categories {
		for _, cur := range p.things.Things[c] {
			if cur == nil {
				continue
			}
			var t *thing.Thing
			for gi, g := range cur.FrameGroups {
				for si, id := range g.Sprites {
					if replace[id] {
						if t == nil {
							t = cur.Clone()
						}
						t.FrameGroups[gi].Sprites[si] = to
					}
				}
			}
			if t != nil {
				r.setThing(c, t.ID, t)
				changed++
			}
		}
	}
	r.commit()
	return changed, nil
}

// duplicateSprites maps every sprite with the same pixels as a sprite with
// a lower id to the first of them (0: not a duplicate; empty sprites are
// never duplicates). Sprites are hashed in parallel with a 128-bit hash, so
// a collision is practically impossible and nothing is read twice.
func (p *Project) duplicateSprites() ([]uint32, error) {
	n := p.sprites.count()
	s1, s2 := maphash.MakeSeed(), maphash.MakeSeed()
	hashes := make([][2]uint64, n+1)
	err := p.sprites.eachCompressed(func(id uint32, c []byte) error {
		if len(c) > 0 {
			hashes[id] = [2]uint64{maphash.Bytes(s1, c) | 1, maphash.Bytes(s2, c)} // 0 marks empty
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dups := make([]uint32, n+1)
	first := make(map[[2]uint64]uint32, n)
	for id := uint32(1); id <= n; id++ {
		h := hashes[id]
		if h[0] == 0 {
			continue
		}
		if f, ok := first[h]; ok {
			dups[id] = f
		} else {
			first[h] = id
		}
	}
	return dups, nil
}
