package project

import (
	"encoding/binary"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/assets"
)

type OptimizeOptions struct {
	Duplicates bool `json:"duplicates"`
	Unused     bool `json:"unused"`
	// Empty points references to fully transparent sprites at id 0.
	Empty bool `json:"empty"`
}

type OptimizeResult struct {
	Before     uint32 `json:"before"`
	After      uint32 `json:"after"`
	Duplicates int    `json:"duplicates"`
	Unused     int    `json:"unused"`
	Empty      int    `json:"empty"`
	Things     int    `json:"things"`
	// Bytes is how much smaller the spr file gets. Asset folders keep their
	// sheets, so it is 0 there.
	Bytes int64 `json:"bytes"`
}

type spritePlan struct {
	result OptimizeResult
	// canon maps every id to the id that replaces it (0 = empty).
	canon []uint32
	// newID maps kept ids to their renumbered id (0 = dropped).
	newID []uint32
}

// AnalyzeSprites reports what OptimizeSprites would remove without changing
// anything. Things is not counted. progress may be nil.
func (p *Project) AnalyzeSprites(options OptimizeOptions, progress func(done, total int)) (OptimizeResult, error) {
	// planSprites only reads: hashes are cached with atomics.
	p.mu.RLock()
	defer p.mu.RUnlock()
	plan, err := p.planSprites(options, newCounter(int(p.sprites.count()), progress))
	return plan.result, err
}

// planSprites finds duplicate, empty and unused sprites by content hash,
// ticking progress for every sprite. The caller holds p.mu (a read lock is
// enough).
func (p *Project) planSprites(options OptimizeOptions, progress *counter) (spritePlan, error) {
	count := p.sprites.count()
	result := OptimizeResult{Before: count, After: count}

	hashes := make([]spriteHash, count+1)
	err := p.sprites.eachHash(func(id uint32, contentHash spriteHash) {
		hashes[id] = contentHash
		progress.tick()
	})
	if err != nil {
		return spritePlan{result: result}, err
	}

	var duplicates []uint32
	if options.Duplicates {
		duplicates = firstOfEachHash(hashes)
	}
	// Pieces of whole sheet sprites go only with their sprite (see
	// sheetSprites): their empty or repeated pieces stay.
	runs := p.sheetSprites()
	whole := make([]bool, count+1)
	for _, run := range runs {
		for i := range run.count {
			whole[run.first+i] = true
		}
	}
	canon := make([]uint32, count+1)
	for id := uint32(1); id <= count; id++ {
		canon[id] = id
		if whole[id] {
			continue
		}
		if hashes[id] == (spriteHash{}) {
			if options.Empty {
				canon[id] = 0
				result.Empty++
			}
			continue
		}
		if options.Duplicates && duplicates[id] != 0 {
			canon[id] = duplicates[id]
			result.Duplicates++
		}
	}
	if options.Duplicates && len(runs) > 0 {
		result.Duplicates += mergeSheetSprites(runs, hashes, canon)
		// A piece may point at a piece of a merged sprite.
		for id := uint32(1); id <= count; id++ {
			if !whole[id] && canon[id] != 0 {
				canon[id] = canon[canon[id]]
			}
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
	// A sheet sprite stays whole when any of its pieces is used.
	for _, run := range runs {
		pieces := used[run.first : run.first+run.count]
		if slices.Contains(pieces, true) {
			for i := range pieces {
				pieces[i] = true
			}
		}
	}

	newID := make([]uint32, count+1)
	var next uint32
	for id := uint32(1); id <= count; id++ {
		keep := canon[id] == id && (hashes[id] != (spriteHash{}) || !options.Empty || whole[id])
		if keep && options.Unused && !used[id] {
			keep = false
			result.Unused++
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
		result.Bytes += 4
		if hashes[id] != (spriteHash{}) {
			size, err := p.sprites.dataSize(id)
			if err != nil {
				return spritePlan{result: result}, err
			}
			result.Bytes += 5 + int64(size)
		}
	}
	result.After = next
	return spritePlan{result: result, canon: canon, newID: newID}, nil
}

type pieceRun struct {
	first, count uint32
	// width is the sprite width in pieces (2 pieces are 2x1 or 1x2).
	width int
}

// sheetSprites lists the runs of pieces that read a whole sprite of the
// asset sheets, in order; nil outside asset folders. Compiling writes such a
// run back as an existing sprite (see assetWriter.sameContent), empty and
// repeated pieces included, so it can only be dropped or merged as a whole.
func (p *Project) sheetSprites() []pieceRun {
	if p.format != FormatAssets || p.assets == nil {
		return nil
	}
	pieceMap, store := p.assets.pieces, p.sprites
	count := store.count()
	var runs []pieceRun
	for id := uint32(1); id <= count; {
		run, ok := p.sheetSpriteAt(pieceMap, id, count)
		if !ok {
			id++
			continue
		}
		runs = append(runs, run)
		id += run.count
	}
	return runs
}

func (p *Project) sheetSpriteAt(pieceMap *assets.PieceMap, id, count uint32) (pieceRun, bool) {
	baseID, ok := p.sprites.baseID(id)
	if !ok {
		return pieceRun{}, false
	}
	location, ok := pieceMap.Locate(baseID)
	if !ok || location.Sprite == 0 {
		return pieceRun{}, false
	}
	first, width, height, ok := pieceMap.Pieces(location.Sprite)
	n := uint32(width * height)
	if !ok || first != baseID || id+n-1 > count {
		return pieceRun{}, false
	}
	for i := uint32(1); i < n; i++ {
		if piece, ok := p.sprites.baseID(id + i); !ok || piece != first+i {
			return pieceRun{}, false
		}
	}
	return pieceRun{first: id, count: n, width: width}, true
}

func mergeSheetSprites(runs []pieceRun, hashes []spriteHash, canon []uint32) int {
	merged := 0
	firstRun := make(map[string]uint32, len(runs))
	key := make([]byte, 0, 4*2*8+1)
	for _, run := range runs {
		key = append(key[:0], byte(run.width))
		for _, contentHash := range hashes[run.first : run.first+run.count] {
			key = binary.LittleEndian.AppendUint64(key, contentHash[0])
			key = binary.LittleEndian.AppendUint64(key, contentHash[1])
		}
		original, ok := firstRun[string(key)]
		if !ok {
			firstRun[string(key)] = run.first
			continue
		}
		for i := range run.count {
			canon[run.first+i] = original + i
		}
		merged += int(run.count)
	}
	return merged
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
		return plan.result, err
	}
	result, canon, newID := plan.result, plan.canon, plan.newID
	before, next := result.Before, result.After
	remap := func(id uint32) uint32 {
		if id == 0 || id > before {
			return id
		}
		return newID[canon[id]]
	}

	edit := p.record("Optimize sprites")
	result.Things = p.remapSpriteRefs(edit, remap)
	// Move kept sprites down (aliases: nothing is decoded), then record
	// every dropped slot so undo can restore it, then shrink.
	for id := uint32(1); id <= before; id++ {
		if mapped := newID[id]; mapped != 0 && mapped != id {
			edit.moveSprite(mapped, id) // mapped < id: id is not written yet
		}
		ticker.tick()
	}
	for id := next + 1; id <= before; id++ {
		edit.setSprite(id, nil)
	}
	edit.setSpriteCount(next)
	edit.commit()
	return result, nil
}

func (p *Project) remapSpriteRefs(edit *recorder, remap func(id uint32) uint32) int {
	changed := 0
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
				changed++
			}
		}
	}
	return changed
}
