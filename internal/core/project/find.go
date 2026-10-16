package project

import (
	"fmt"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/thing"
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

// FindThings returns the ids of things in category that match filter, in
// order.
func (p *Project) FindThings(category thing.Category, filter Filter) ([]uint32, error) {
	switch filter.Content {
	case ContentAll, ContentUsed, ContentEmpty:
	default:
		return nil, fmt.Errorf("unknown content filter %q", filter.Content)
	}
	if filter.Flag != "" {
		if _, ok := (&thing.Properties{}).FlagByKey(filter.Flag); !ok {
			return nil, fmt.Errorf("unknown flag %q", filter.Flag)
		}
	}
	name := strings.ToLower(strings.TrimSpace(filter.Name))
	p.mu.RLock()
	defer p.mu.RUnlock()
	match := func(object *thing.Thing) bool {
		if object == nil {
			return false
		}
		if filter.Flag != "" {
			if set, _ := object.Props.FlagByKey(filter.Flag); !set {
				return false
			}
		}
		if name != "" && !strings.Contains(strings.ToLower(object.Name), name) {
			return false
		}
		return filter.Content == ContentAll || p.hasPixels(object) == (filter.Content == ContentUsed)
	}
	first, last := category.MinID(), p.things.MaxID(category)
	keep := make([]bool, max(int(last)-int(first)+1, 0))
	// The content filter reads sprites (asset sheets decode slowly): check
	// the things in parallel, then collect them in order.
	parallelRange(first, last, thingChunk(uint32(len(keep))), func(from, to uint32) error {
		for id := from; id <= to; id++ {
			keep[id-first] = match(p.things.Get(category, id))
		}
		return nil
	})
	ids := []uint32{}
	for i, kept := range keep {
		if kept {
			ids = append(ids, first+uint32(i))
		}
	}
	return ids, nil
}

func (p *Project) hasPixels(object *thing.Thing) bool {
	for _, group := range object.FrameGroups {
		for _, id := range group.Sprites {
			if id == 0 {
				continue
			}
			if compressed, err := p.sprites.compressed(id); err == nil && len(compressed) > 0 {
				return true
			}
		}
	}
	return false
}

// Names returns the names of the named things in category, by id.
func (p *Project) Names(category thing.Category) map[uint32]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := map[uint32]string{}
	for _, object := range p.things.Things[category] {
		if object != nil && object.Name != "" {
			out[object.ID] = object.Name
		}
	}
	return out
}
