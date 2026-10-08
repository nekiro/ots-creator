package project

import "github.com/nekiro/ots-creator/internal/thing"

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

func (d *deltaSet) thing(c thing.Category, id uint32) {
	if d.all {
		return
	}
	if d.things == nil {
		d.things = map[ThingRef]struct{}{}
	}
	d.things[ThingRef{c, id}] = struct{}{}
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

// spriteRange marks ids in (a, b] or (b, a].
func (d *deltaSet) spriteRange(a, b uint32) {
	lo, hi := min(a, b), max(a, b)
	if hi-lo > deltaLimit {
		d.everything()
		return
	}
	for id := lo + 1; id <= hi; id++ {
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
	d := p.delta
	p.delta = deltaSet{}
	out := Delta{All: d.all, Things: []ThingRef{}, Sprites: []uint32{}}
	if d.all {
		return out
	}
	for id := range d.sprites {
		out.Sprites = append(out.Sprites, id)
	}
	if len(d.sprites) > 0 {
		for _, c := range thing.Categories {
			for _, t := range p.things.Things[c] {
				if t == nil {
					continue
				}
				if usesAny(t, d.sprites) {
					d.thing(c, t.ID)
				}
			}
		}
	}
	for r := range d.things {
		out.Things = append(out.Things, r)
	}
	return out
}

func usesAny(t *thing.Thing, ids map[uint32]struct{}) bool {
	for _, g := range t.FrameGroups {
		for _, id := range g.Sprites {
			if _, ok := ids[id]; ok {
				return true
			}
		}
	}
	return false
}
