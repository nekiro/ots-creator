package project

import (
	"fmt"

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
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := []uint32{}
	for id := c.MinID(); id <= p.things.MaxID(c); id++ {
		t := p.things.Get(c, id)
		if t == nil {
			continue
		}
		if f.Flag != "" {
			if on, _ := t.Props.FlagByKey(f.Flag); !on {
				continue
			}
		}
		if f.Content != ContentAll && p.hasPixels(t) != (f.Content == ContentUsed) {
			continue
		}
		ids = append(ids, id)
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
