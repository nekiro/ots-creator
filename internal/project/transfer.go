package project

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// TransferResult reports a Transfer.
type TransferResult struct {
	// IDs are the ids of the copied things in the target, in request order.
	IDs []uint32 `json:"ids"`
	// Sprites counts the sprites added to the target.
	Sprites int `json:"sprites"`
	// Before holds the things the copies replaced (nil: the id did not
	// exist), for copies that kept their ids. See RestoreThings.
	Before map[uint32]*thing.Thing `json:"-"`
}

// Transfer copies things of category from src into dst together with
// their sprites. With appendNew the copies get new ids at the end of the
// category; otherwise they keep their ids, replacing the things in dst
// (ids past the end of dst extend it, filling any gap with empty things).
// A sprite is reused when dst already has the same pixels under the same
// id, or when it was already added by this transfer. Outfits with frame
// groups are merged into one group when dst has no frame groups.
// The whole transfer is one undoable edit in dst.
func Transfer(dst, src *Project, category thing.Category, ids []uint32, appendNew bool) (TransferResult, error) {
	return transfer(dst, src, category, ids, appendNew, nil)
}

// TransferTo is Transfer that copies ids[i] to targets[i] in dst,
// replacing the things there (targets past the end of dst extend it).
func TransferTo(dst, src *Project, category thing.Category, ids, targets []uint32) (TransferResult, error) {
	if len(targets) != len(ids) {
		return TransferResult{}, fmt.Errorf("%d target ids for %d %s(s)", len(targets), len(ids), category)
	}
	seen := map[uint32]bool{}
	for _, id := range targets {
		if id < category.MinID() {
			return TransferResult{}, fmt.Errorf("%s id %d is below %d", category, id, category.MinID())
		}
		if seen[id] {
			return TransferResult{}, fmt.Errorf("target id %d is used twice", id)
		}
		seen[id] = true
	}
	return transfer(dst, src, category, ids, false, targets)
}

func transfer(dst, src *Project, category thing.Category, ids []uint32, appendNew bool, targets []uint32) (TransferResult, error) {
	if dst == src {
		return TransferResult{}, fmt.Errorf("cannot copy a client into itself")
	}
	if !category.Valid() {
		return TransferResult{}, fmt.Errorf("invalid category %d", category)
	}
	if dst.SpriteSize() != src.SpriteSize() {
		return TransferResult{}, fmt.Errorf("clients use %dpx and %dpx sprites", dst.SpriteSize(), src.SpriteSize())
	}
	snap, err := src.snapshot(category, ids)
	if err != nil {
		return TransferResult{}, err
	}

	dst.mu.Lock()
	defer dst.mu.Unlock()
	things := snap.things
	groups := dst.features.FrameGroups || dst.format == FormatAssets
	for i, object := range things {
		if len(object.FrameGroups) > 1 && !groups {
			merged := mergeGroups(object)
			if merged == nil {
				return TransferResult{}, fmt.Errorf("%s %d: idle and walking layouts differ, the target client has no frame groups", category, object.ID)
			}
			things[i] = merged
		}
	}

	label := fmt.Sprintf("Copy %d %s(s)", len(things), category)
	if snap.name != "" {
		label += " from " + snap.name
	}
	edit := dst.record(label)
	res := TransferResult{IDs: make([]uint32, 0, len(things)), Before: map[uint32]*thing.Thing{}}
	mapped := map[uint32]uint32{}
	added := map[string]uint32{}
	for i, object := range things {
		if targets != nil {
			object.ID = targets[i]
		}
		for _, group := range object.FrameGroups {
			for i, spriteID := range group.Sprites {
				newID, ok := mapped[spriteID]
				if !ok {
					if newID, err = dst.importSprite(edit, spriteID, snap.sprites[spriteID], snap.transparent, added, &res.Sprites); err != nil {
						return TransferResult{}, err
					}
					mapped[spriteID] = newID
				}
				group.Sprites[i] = newID
			}
		}
		current := (*thing.Thing)(nil)
		if appendNew {
			object.ID = dst.things.MaxID(category) + 1
		} else {
			current = dst.things.Get(category, object.ID)
			if _, ok := res.Before[object.ID]; !ok {
				res.Before[object.ID] = current
			}
			for id := dst.things.MaxID(category) + 1; id < object.ID; id++ {
				edit.setThing(category, id, thing.New(id, category))
			}
		}
		if snap.format != dst.format {
			object.Extra = nil
			keepExtra(object, current)
		}
		if err := object.Validate(); err != nil {
			return TransferResult{}, err
		}
		edit.setThing(category, object.ID, object)
		res.IDs = append(res.IDs, object.ID)
	}
	edit.commit()
	return res, nil
}

// name is a short name of the client for labels.
func (p *Project) name() string {
	if p.datPath == "" {
		return ""
	}
	if p.format == FormatAssets {
		return filepath.Base(p.datPath)
	}
	return filepath.Base(filepath.Dir(p.datPath)) + "/" + filepath.Base(p.datPath)
}

// importSprite stores source sprite spriteID (compressed with the source
// transparency) and returns the id to use in p. Empty sprites map to 0.
func (p *Project) importSprite(edit *recorder, spriteID uint32, compressed []byte, transparent bool, added map[string]uint32, count *int) (uint32, error) {
	if spriteID == 0 {
		return 0, nil
	}
	if transparent != p.sprites.transparent {
		var err error
		if compressed, err = spr.Transcode(compressed, p.sprites.size, transparent, p.sprites.transparent); err != nil {
			return 0, err
		}
	}
	if len(compressed) == 0 {
		return 0, nil
	}
	if id, ok := added[string(compressed)]; ok {
		return id, nil
	}
	if spriteID <= p.sprites.count() {
		current, err := p.sprites.compressed(spriteID)
		if err != nil {
			return 0, err
		}
		if bytes.Equal(current, compressed) {
			return spriteID, nil
		}
	}
	id := p.sprites.count() + 1
	edit.setSpriteCount(id)
	edit.setSprite(id, compressed)
	added[string(compressed)] = id
	*count++
	return id, nil
}
