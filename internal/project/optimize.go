package project

import (
	"github.com/nekiro/ots-creator/internal/thing"
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
}

// OptimizeSprites removes duplicate, empty and unused sprites, renumbers
// the remaining ones without gaps (keeping their order) and rewrites the
// sprite ids of every thing. It is one undo step.
func (p *Project) OptimizeSprites(o OptimizeOptions) (OptimizeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.sprites.count()
	res := OptimizeResult{Before: n, After: n}

	data := make([][]byte, n+1)
	for id := uint32(1); id <= n; id++ {
		c, err := p.sprites.compressed(id)
		if err != nil {
			return res, err
		}
		data[id] = c
	}

	// canon maps every id to the id that replaces it (0 = empty).
	canon := make([]uint32, n+1)
	first := map[string]uint32{}
	for id := uint32(1); id <= n; id++ {
		canon[id] = id
		c := data[id]
		if len(c) == 0 {
			if o.Empty {
				canon[id] = 0
				res.Empty++
			}
			continue
		}
		if !o.Duplicates {
			continue
		}
		if prev, ok := first[string(c)]; ok {
			canon[id] = prev
			res.Duplicates++
		} else {
			first[string(c)] = id
		}
	}

	used := make([]bool, n+1)
	for _, c := range thing.Categories {
		for _, t := range p.things.Things[c] {
			if t == nil {
				continue
			}
			for _, id := range t.SpriteIDs() {
				if id <= n {
					used[canon[id]] = true
				}
			}
		}
	}

	// Sprites that stay, in order, and their new ids.
	newID := make([]uint32, n+1)
	var next uint32
	for id := uint32(1); id <= n; id++ {
		keep := canon[id] == id && (len(data[id]) > 0 || !o.Empty)
		if keep && o.Unused && !used[id] {
			keep = false
			res.Unused++
		}
		if keep {
			next++
			newID[id] = next
		}
	}
	remap := func(id uint32) uint32 {
		if id == 0 || id > n {
			return id
		}
		return newID[canon[id]]
	}

	r := p.record("Optimize sprites")
	for _, c := range thing.Categories {
		for _, cur := range p.things.Things[c] {
			if cur == nil {
				continue
			}
			var t *thing.Thing
			for gi, g := range cur.FrameGroups {
				for si, id := range g.Sprites {
					if m := remap(id); m != id {
						if t == nil {
							t = cur.Clone()
						}
						t.FrameGroups[gi].Sprites[si] = m
					}
				}
			}
			if t != nil {
				r.setThing(c, t.ID, t)
				res.Things++
			}
		}
	}
	// Move kept sprites down, then record every dropped slot so undo can
	// restore it, then shrink.
	for id := uint32(1); id <= n; id++ {
		if m := newID[id]; m != 0 && m != id {
			r.setSprite(m, data[id])
		}
	}
	for id := next + 1; id <= n; id++ {
		r.setSprite(id, nil)
	}
	r.setSpriteCount(next)
	r.commit()
	res.After = next
	return res, nil
}
