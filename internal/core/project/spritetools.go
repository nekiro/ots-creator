package project

import (
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

const (
	SpritesUnused    = "unused"    // no thing references the sprite
	SpritesEmpty     = "empty"     // no visible pixels
	SpritesDuplicate = "duplicate" // same pixels as a sprite with a lower id
)

func (p *Project) FindSprites(filter string) ([]uint32, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := p.sprites.count()
	out := []uint32{}
	switch filter {
	case SpritesUnused:
		used := p.usedSpritesLocked()
		for id := uint32(1); id <= count; id++ {
			if !used[id] {
				out = append(out, id)
			}
		}
	case SpritesEmpty:
		empty := make([]bool, count+1)
		err := p.sprites.eachHash(func(id uint32, contentHash spriteHash) { empty[id] = contentHash == spriteHash{} })
		if err != nil {
			return nil, err
		}
		for id := uint32(1); id <= count; id++ {
			if empty[id] {
				out = append(out, id)
			}
		}
	case SpritesDuplicate:
		duplicates, err := p.duplicateSprites()
		if err != nil {
			return nil, err
		}
		for id := uint32(1); id <= count; id++ {
			if duplicates[id] != 0 {
				out = append(out, id)
			}
		}
	default:
		return nil, fmt.Errorf("unknown sprite filter %q", filter)
	}
	return out, nil
}

func (p *Project) usedSpritesLocked() []bool {
	used := make([]bool, p.sprites.count()+1)
	for _, category := range thing.Categories {
		for _, object := range p.things.Things[category] {
			if object == nil {
				continue
			}
			for _, group := range object.FrameGroups {
				for _, id := range group.Sprites {
					if int(id) < len(used) {
						used[id] = true
					}
				}
			}
		}
	}
	return used
}

func (p *Project) SpriteUsers(id uint32) []ThingRef {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := []ThingRef{}
	for _, category := range thing.Categories {
		for _, object := range p.things.Things[category] {
			if object != nil && slices.Contains(object.SpriteIDs(), id) {
				out = append(out, ThingRef{Category: category, ID: object.ID})
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
	edit := p.record(fmt.Sprintf("Replace %d sprite reference(s) with %d", len(replace), to))
	changed := p.remapSpriteRefs(edit, func(id uint32) uint32 {
		if replace[id] {
			return to
		}
		return id
	})
	edit.commit()
	return changed, nil
}

// duplicateSprites maps every sprite with the same pixels as a sprite with
// a lower id to the first of them (0: not a duplicate; empty sprites are
// never duplicates). Sprites are hashed in parallel with a 128-bit hash, so
// a collision is practically impossible and nothing is read twice.
func (p *Project) duplicateSprites() ([]uint32, error) {
	count := p.sprites.count()
	hashes := make([]spriteHash, count+1)
	if err := p.sprites.eachHash(func(id uint32, contentHash spriteHash) { hashes[id] = contentHash }); err != nil {
		return nil, err
	}
	return firstOfEachHash(hashes), nil
}

// firstOfEachHash maps every id of hashes (indexed by sprite id, 0 unused)
// whose hash an earlier id has to the first such id (0: not a duplicate;
// empty sprites are never duplicates).
func firstOfEachHash(hashes []spriteHash) []uint32 {
	duplicates := make([]uint32, len(hashes))
	first := make(map[spriteHash]uint32, len(hashes))
	for id := 1; id < len(hashes); id++ {
		contentHash := hashes[id]
		if contentHash == (spriteHash{}) {
			continue
		}
		if firstID, ok := first[contentHash]; ok {
			duplicates[id] = firstID
		} else {
			first[contentHash] = uint32(id)
		}
	}
	return duplicates
}
