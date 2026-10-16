package project

import (
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// OptimizeOptions select what OptimizeSprites removes.
type OptimizeOptions struct {
	// Duplicates merges sprites with identical pixels into one.
	Duplicates bool `json:"duplicates"`
	// Unused drops sprites no thing references.
	Unused bool `json:"unused"`
	// Empty points references to fully transparent sprites at id 0.
	Empty bool `json:"empty"`
}

// OptimizeResult reports what OptimizeSprites did.
type OptimizeResult struct {
	Before     uint32 `json:"before"`
	After      uint32 `json:"after"`
	Duplicates int    `json:"duplicates"`
	Unused     int    `json:"unused"`
	Empty      int    `json:"empty"`
	// Things counts things whose sprite ids were rewritten.
	Things int `json:"things"`
	// Bytes is how much smaller the spr file gets. Asset folders keep their
	// sheets, so it is 0 there.
	Bytes int64 `json:"bytes"`
}

// spritePlan is what OptimizeSprites would change.
type spritePlan struct {
	res OptimizeResult
	// canon maps every id to the id that replaces it (0 = empty).
	canon []uint32
	// newID maps kept ids to their renumbered id (0 = dropped).
	newID []uint32
}

// AnalyzeSprites reports what OptimizeSprites would remove without changing
// anything. Things is not counted. progress may be nil.
func (p *Project) AnalyzeSprites(options OptimizeOptions, progress func(done, total int)) (OptimizeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	plan, err := p.planSprites(options, newCounter(int(p.sprites.count()), progress))
	return plan.res, err
}

// planSprites finds duplicate, empty and unused sprites by content hash,
// ticking progress for every sprite. The caller holds p.mu.
func (p *Project) planSprites(options OptimizeOptions, progress *counter) (spritePlan, error) {
	count := p.sprites.count()
	res := OptimizeResult{Before: count, After: count}

	hashes := make([]spriteHash, count+1)
	err := p.sprites.eachHash(func(id uint32, contentHash spriteHash) {
		hashes[id] = contentHash
		progress.tick()
	})
	if err != nil {
		return spritePlan{res: res}, err
	}

	canon := make([]uint32, count+1)
	first := make(map[spriteHash]uint32, count)
	for id := uint32(1); id <= count; id++ {
		canon[id] = id
		contentHash := hashes[id]
		if contentHash == (spriteHash{}) {
			if options.Empty {
				canon[id] = 0
				res.Empty++
			}
			continue
		}
		if !options.Duplicates {
			continue
		}
		if prev, ok := first[contentHash]; ok {
			canon[id] = prev
			res.Duplicates++
		} else {
			first[contentHash] = id
		}
	}

	used := make([]bool, count+1)
	for _, category := range thing.Categories {
		for _, object := range p.things.Things[category] {
			if object == nil {
				continue
			}
			for _, id := range object.SpriteIDs() {
				if id <= count {
					used[canon[id]] = true
				}
			}
		}
	}

	// Sprites that stay, in order, and their new ids.
	newID := make([]uint32, count+1)
	var next uint32
	for id := uint32(1); id <= count; id++ {
		keep := canon[id] == id && (hashes[id] != (spriteHash{}) || !options.Empty)
		if keep && options.Unused && !used[id] {
			keep = false
			res.Unused++
		}
		if keep {
			next++
			newID[id] = next
			continue
		}
		if p.format == FormatAssets {
			continue
		}
		// A removed sprite frees its spr address and, unless empty, its data.
		res.Bytes += 4
		if hashes[id] != (spriteHash{}) {
			size, err := p.sprites.dataSize(id)
			if err != nil {
				return spritePlan{res: res}, err
			}
			res.Bytes += 5 + int64(size)
		}
	}
	res.After = next
	return spritePlan{res: res, canon: canon, newID: newID}, nil
}

// OptimizeSprites removes duplicate, empty and unused sprites, renumbers
// the remaining ones without gaps (keeping their order) and rewrites the
// sprite ids of every thing. It is one undo step. progress may be nil.
func (p *Project) OptimizeSprites(options OptimizeOptions, progress func(done, total int)) (OptimizeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Hashing every sprite, then moving every sprite.
	ticker := newCounter(2*int(p.sprites.count()), progress)
	plan, err := p.planSprites(options, ticker)
	if err != nil {
		return plan.res, err
	}
	res, canon, newID := plan.res, plan.canon, plan.newID
	before, next := res.Before, res.After
	remap := func(id uint32) uint32 {
		if id == 0 || id > before {
			return id
		}
		return newID[canon[id]]
	}

	edit := p.record("Optimize sprites")
	for _, category := range thing.Categories {
		for _, current := range p.things.Things[category] {
			if current == nil {
				continue
			}
			var object *thing.Thing
			for groupIndex, group := range current.FrameGroups {
				for slot, id := range group.Sprites {
					if mapped := remap(id); mapped != id {
						if object == nil {
							object = current.Clone()
						}
						object.FrameGroups[groupIndex].Sprites[slot] = mapped
					}
				}
			}
			if object != nil {
				edit.setThing(category, object.ID, object)
				res.Things++
			}
		}
	}
	// Move kept sprites down (aliases: nothing is decoded), then record
	// every dropped slot so undo can restore it, then shrink.
	for id := uint32(1); id <= before; id++ {
		if mapped := newID[id]; mapped != 0 && mapped != id {
			edit.moveSprite(mapped, id) // m < id: id is not written yet
		}
		ticker.tick()
	}
	for id := next + 1; id <= before; id++ {
		edit.setSprite(id, nil)
	}
	edit.setSpriteCount(next)
	edit.commit()
	return res, nil
}
