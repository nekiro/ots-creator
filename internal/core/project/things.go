package project

import (
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// Thing returns a copy of a thing.
func (p *Project) Thing(category thing.Category, id uint32) (*thing.Thing, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	object := p.things.Get(category, id)
	if object == nil {
		return nil, fmt.Errorf("%s %d does not exist", category, id)
	}
	return object.Clone(), nil
}

// ThingRange returns copies of things with ids in [from, to].
func (p *Project) ThingRange(category thing.Category, from, to uint32) []*thing.Thing {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*thing.Thing
	for id := max(from, category.MinID()); id <= to; id++ {
		object := p.things.Get(category, id)
		if object == nil {
			break
		}
		out = append(out, object.Clone())
	}
	return out
}

// UpdateThing replaces an existing thing with object (matched by category
// and id).
func (p *Project) UpdateThing(object *thing.Thing) error {
	if err := object.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.things.Get(object.Category, object.ID)
	if current == nil {
		return fmt.Errorf("%s %d does not exist", object.Category, object.ID)
	}
	if err := p.checkSpriteIDs(object); err != nil {
		return err
	}
	edit := p.record(fmt.Sprintf("Edit %s %d", object.Category, object.ID))
	edit.setThing(object.Category, object.ID, keepExtra(object.Clone(), current))
	edit.commit()
	return nil
}

// keepExtra gives an edited copy of a thing (that came from the UI or a
// file) the format data of the thing it replaces.
func keepExtra(object, current *thing.Thing) *thing.Thing {
	if current != nil {
		object.Extra = current.Extra
	}
	return object
}

func (p *Project) checkSpriteIDs(object *thing.Thing) error {
	count := p.sprites.count()
	for _, id := range object.SpriteIDs() {
		if id > count {
			return fmt.Errorf("sprite %d does not exist (max %d)", id, count)
		}
	}
	return nil
}

// AddThing appends a new empty thing and returns its id.
func (p *Project) AddThing(category thing.Category) (uint32, error) {
	if !category.Valid() {
		return 0, fmt.Errorf("invalid category %d", category)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.things.MaxID(category) + 1
	edit := p.record(fmt.Sprintf("New %s", category))
	edit.setThing(category, id, thing.New(id, category))
	edit.commit()
	return id, nil
}

// DuplicateThings appends copies of the given things and returns new ids.
func (p *Project) DuplicateThings(category thing.Category, ids []uint32) ([]uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	edit := p.record(fmt.Sprintf("Duplicate %d %s(s)", len(ids), category))
	var out []uint32
	for _, id := range ids {
		src := p.things.Get(category, id)
		if src == nil {
			return nil, fmt.Errorf("%s %d does not exist", category, id)
		}
		duplicate := src.Clone()
		duplicate.ID = p.things.MaxID(category) + 1
		edit.setThing(category, duplicate.ID, duplicate)
		out = append(out, duplicate.ID)
	}
	edit.commit()
	return out, nil
}

// PasteThings puts copies of the given things at the ids from at on, in
// order, in one undo step: things already there are replaced and ids past
// the end of the category extend it. It returns the ids written.
func (p *Project) PasteThings(category thing.Category, at uint32, objects []*thing.Thing) ([]uint32, error) {
	if !category.Valid() {
		return nil, fmt.Errorf("invalid category %d", category)
	}
	if at < category.MinID() {
		return nil, fmt.Errorf("%s id %d is below %d", category, at, category.MinID())
	}
	for _, object := range objects {
		if object.Category != category {
			return nil, fmt.Errorf("a %s cannot be pasted as a %s", object.Category, category)
		}
		if err := object.Validate(); err != nil {
			return nil, err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if at > p.things.MaxID(category)+1 {
		return nil, fmt.Errorf("%s id %d is past the end (%d)", category, at, p.things.MaxID(category)+1)
	}
	for _, object := range objects {
		if err := p.checkSpriteIDs(object); err != nil {
			return nil, err
		}
	}
	edit := p.record(fmt.Sprintf("Paste %d %s(s)", len(objects), category))
	out := make([]uint32, 0, len(objects))
	for i, object := range objects {
		pasted := object.Clone()
		pasted.ID = at + uint32(i)
		edit.setThing(category, pasted.ID, keepExtra(pasted, p.things.Get(category, pasted.ID)))
		out = append(out, pasted.ID)
	}
	edit.commit()
	return out, nil
}

// RemoveThings removes things. The last thing of a category is deleted;
// others are replaced with empty things so ids stay stable. The first id of
// each category always remains.
func (p *Project) RemoveThings(category thing.Category, ids []uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	slices.Reverse(sorted)
	edit := p.record(fmt.Sprintf("Remove %d %s(s)", len(ids), category))
	for _, id := range sorted {
		if p.things.Get(category, id) == nil {
			return fmt.Errorf("%s %d does not exist", category, id)
		}
		if id == p.things.MaxID(category) && id != category.MinID() {
			edit.setThing(category, id, nil)
		} else {
			edit.setThing(category, id, thing.New(id, category))
		}
	}
	edit.commit()
	return nil
}
