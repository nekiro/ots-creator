package project

import "github.com/nekiro/ots-creator/internal/core/thing"

// deltaLimit is the number of touched sprites after which a delta just
// reports "everything changed": cheaper than listing them.
const deltaLimit = 4096

// ThingRef names one thing.
type ThingRef struct {
	Category thing.Category `json:"category"`
	ID       uint32         `json:"id"`
}

// Delta lists what changed since the previous TakeDelta, so views can
// refresh only those things and sprites. Things includes every thing that
// uses a changed sprite.
type Delta struct {
	All     bool       `json:"all"`
	Things  []ThingRef `json:"things"`
	Sprites []uint32   `json:"sprites"`
}

type deltaSet struct {
	all     bool
	things  map[ThingRef]struct{}
	sprites map[uint32]struct{}
}

func (d *deltaSet) thing(category thing.Category, id uint32) {
	if d.all {
		return
	}
	if d.things == nil {
		d.things = map[ThingRef]struct{}{}
	}
	d.things[ThingRef{category, id}] = struct{}{}
}

func (d *deltaSet) sprite(id uint32) {
	if d.all {
		return
	}
	if d.sprites == nil {
		d.sprites = map[uint32]struct{}{}
	}
	d.sprites[id] = struct{}{}
	if len(d.sprites) > deltaLimit {
		d.everything()
	}
}

// spriteRange marks ids in (from, to] or (to, from].
func (d *deltaSet) spriteRange(from, to uint32) {
	low, high := min(from, to), max(from, to)
	if high-low > deltaLimit {
		d.everything()
		return
	}
	for id := low + 1; id <= high; id++ {
		d.sprite(id)
	}
}

func (d *deltaSet) everything() {
	*d = deltaSet{all: true}
}

// TakeDelta returns the changes since the last call and resets them.
func (p *Project) TakeDelta() Delta {
	p.mu.Lock()
	defer p.mu.Unlock()
	delta := p.delta
	p.delta = deltaSet{}
	out := Delta{All: delta.all, Things: []ThingRef{}, Sprites: []uint32{}}
	if delta.all {
		return out
	}
	for id := range delta.sprites {
		out.Sprites = append(out.Sprites, id)
	}
	if len(delta.sprites) > 0 {
		for _, category := range thing.Categories {
			for _, object := range p.things.Things[category] {
				if object == nil {
					continue
				}
				if usesAny(object, delta.sprites) {
					delta.thing(category, object.ID)
				}
			}
		}
	}
	for ref := range delta.things {
		out.Things = append(out.Things, ref)
	}
	return out
}

func usesAny(object *thing.Thing, ids map[uint32]struct{}) bool {
	for _, group := range object.FrameGroups {
		for _, id := range group.Sprites {
			if _, ok := ids[id]; ok {
				return true
			}
		}
	}
	return false
}
