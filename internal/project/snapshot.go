package project

import (
	"fmt"
	"maps"
	"slices"

	"github.com/nekiro/ots-creator/internal/thing"
)

// RestoreThings puts back things of category as they were before a
// Transfer (see TransferResult.Before), as one undoable edit. A nil thing
// did not exist: it is removed when it is the last one, else emptied.
func (p *Project) RestoreThings(category thing.Category, before map[uint32]*thing.Thing) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := slices.Sorted(maps.Keys(before))
	slices.Reverse(ids)
	for _, id := range ids {
		if object := before[id]; object != nil {
			if err := p.checkSpriteIDs(object); err != nil {
				return fmt.Errorf("%s %d: %w", category, id, err)
			}
		}
	}
	edit := p.record(fmt.Sprintf("Revert %d %s(s)", len(ids), category))
	for _, id := range ids {
		object := before[id]
		switch {
		case object != nil:
		case id == p.things.MaxID(category) && id != category.MinID():
		default:
			object = thing.New(id, category)
		}
		if object == nil && p.things.Get(category, id) == nil {
			continue
		}
		edit.setThing(category, id, object)
	}
	edit.commit()
	return nil
}

// thingSnapshot holds copies of things and their sprites, compressed with
// the transparency setting of the source.
type thingSnapshot struct {
	things      []*thing.Thing
	sprites     map[uint32][]byte
	transparent bool
	format      Format
	name        string
}

// snapshot copies things and their sprites, so a transfer
// does not hold the source lock while it writes the target.
func (p *Project) snapshot(category thing.Category, ids []uint32) (*thingSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := &thingSnapshot{
		things:      make([]*thing.Thing, 0, len(ids)),
		sprites:     map[uint32][]byte{},
		transparent: p.sprites.transparent,
		format:      p.format,
		name:        p.name(),
	}
	for _, id := range ids {
		object := p.things.Get(category, id)
		if object == nil {
			return nil, fmt.Errorf("%s %d does not exist", category, id)
		}
		snapshot.things = append(snapshot.things, object.Clone())
		for _, spriteID := range object.SpriteIDs() {
			if _, ok := snapshot.sprites[spriteID]; ok || spriteID == 0 {
				continue
			}
			compressed, err := p.sprites.compressed(spriteID)
			if err != nil {
				return nil, err
			}
			snapshot.sprites[spriteID] = compressed
		}
	}
	return snapshot, nil
}
