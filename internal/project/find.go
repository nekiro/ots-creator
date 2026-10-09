package project

import (
	"fmt"
	"strings"

	"github.com/nekiro/ots-creator/internal/thing"
)

// Content filters for FindThings.
const (
	ContentAll   = ""
	ContentUsed  = "used"  // at least one sprite with pixels
	ContentEmpty = "empty" // no sprite with pixels
)

// Filter selects things in a category.
type Filter struct {
	Content string `json:"content"`
	Flag    string `json:"flag"` // JSON name of a boolean property that must be set
	// Name matches things whose name contains it, ignoring case.
	Name string `json:"name"`
}

// FindThings returns the ids of things in c that match f, in order.
func (p *Project) FindThings(c thing.Category, f Filter) ([]uint32, error) {
	switch f.Content {
	case ContentAll, ContentUsed, ContentEmpty:
	default:
		return nil, fmt.Errorf("unknown content filter %q", f.Content)
	}
	if f.Flag != "" {
		if _, ok := (&thing.Properties{}).FlagByKey(f.Flag); !ok {
			return nil, fmt.Errorf("unknown flag %q", f.Flag)
		}
	}
	name := strings.ToLower(strings.TrimSpace(f.Name))
	p.mu.RLock()
	defer p.mu.RUnlock()
	match := func(t *thing.Thing) bool {
		if t == nil {
			return false
		}
		if f.Flag != "" {
			if on, _ := t.Props.FlagByKey(f.Flag); !on {
				return false
			}
		}
		if name != "" && !strings.Contains(strings.ToLower(t.Name), name) {
			return false
		}
		return f.Content == ContentAll || p.hasPixels(t) == (f.Content == ContentUsed)
	}
	first, last := c.MinID(), p.things.MaxID(c)
	keep := make([]bool, max(int(last)-int(first)+1, 0))
	// The content filter reads sprites (asset sheets decode slowly): check
	// the things in parallel, then collect them in order.
	parallelRange(first, last, thingChunk(uint32(len(keep))), func(from, to uint32) error {
		for id := from; id <= to; id++ {
			keep[id-first] = match(p.things.Get(c, id))
		}
		return nil
	})
	ids := []uint32{}
	for i, k := range keep {
		if k {
			ids = append(ids, first+uint32(i))
		}
	}
	return ids, nil
}

func (p *Project) hasPixels(t *thing.Thing) bool {
	for _, g := range t.FrameGroups {
		for _, id := range g.Sprites {
			if id == 0 {
				continue
			}
			if c, err := p.sprites.compressed(id); err == nil && len(c) > 0 {
				return true
			}
		}
	}
	return false
}

// Names returns the names of the named things in c, by id.
func (p *Project) Names(c thing.Category) map[uint32]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := map[uint32]string{}
	for _, t := range p.things.Things[c] {
		if t != nil && t.Name != "" {
			out[t.ID] = t.Name
		}
	}
	return out
}
