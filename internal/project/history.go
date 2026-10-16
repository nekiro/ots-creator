package project

import "github.com/nekiro/ots-creator/internal/thing"

const maxHistory = 200

type thingChange struct {
	category      thing.Category
	id            uint32
	before, after *thing.Thing // nil when the slot does not exist
}

type spriteChange struct {
	id uint32
	// before and after are how the sprite resolved. Restoring a base slot
	// marks the sprite unedited again (compiling then keeps it as it is).
	before, after spriteSlot
}

// historyEntry is one undoable operation. It records list lengths before and after
// plus every slot it touched, so applying it in either direction is exact.
type historyEntry struct {
	label        string
	lenBefore    map[thing.Category]int
	lenAfter     map[thing.Category]int
	things       []thingChange
	sprites      []spriteChange
	spritesCount [2]uint32 // before, after
	// itemMoves maps item ids to the ids the edit moved them to.
	itemMoves map[uint32]uint32
}

type history struct {
	undo []*historyEntry
	redo []*historyEntry
	// clean is the top of the undo stack when the files were last written
	// (nil: empty stack). Undoing back to it means nothing is left to save.
	clean *historyEntry
}

// lost marks a saved state that undo can no longer reach.
var lost = &historyEntry{}

func (h *history) push(entry *historyEntry) {
	h.undo = append(h.undo, entry)
	if len(h.undo) > maxHistory {
		drop := h.undo[:len(h.undo)-maxHistory]
		if h.clean == nil {
			h.clean = lost
		}
		for _, dropped := range drop {
			if dropped == h.clean {
				h.clean = lost
			}
		}
		h.undo = h.undo[len(h.undo)-maxHistory:]
	}
	h.redo = nil
}

func (h *history) top() *historyEntry {
	if len(h.undo) == 0 {
		return nil
	}
	return h.undo[len(h.undo)-1]
}

// atClean reports whether the undo stack is back at the saved state.
func (h *history) atClean() bool { return h.top() == h.clean }

func (h *history) canUndo() bool { return len(h.undo) > 0 }
func (h *history) canRedo() bool { return len(h.redo) > 0 }
func (h *history) clear()        { h.undo, h.redo, h.clean = nil, nil, nil }

// recorder builds an edit while an operation mutates the project. It
// remembers the original state of every slot on first touch.
type recorder struct {
	project     *Project
	entry       *historyEntry
	touched     map[thing.Category]map[uint32]int
	spriteIndex map[uint32]int
}

func (p *Project) record(label string) *recorder {
	entry := &historyEntry{label: label, lenBefore: map[thing.Category]int{}, lenAfter: map[thing.Category]int{}}
	for _, category := range thing.Categories {
		entry.lenBefore[category] = len(p.things.Things[category])
	}
	entry.spritesCount[0] = p.sprites.count()
	return &recorder{project: p, entry: entry, touched: map[thing.Category]map[uint32]int{}, spriteIndex: map[uint32]int{}}
}

// setThing stores object (or removes the slot when object is nil and it is
// the last).
func (r *recorder) setThing(category thing.Category, id uint32, object *thing.Thing) {
	indexByID := r.touched[category]
	if indexByID == nil {
		indexByID = map[uint32]int{}
		r.touched[category] = indexByID
	}
	if _, ok := indexByID[id]; !ok {
		indexByID[id] = len(r.entry.things)
		var before *thing.Thing
		if current := r.project.things.Get(category, id); current != nil {
			before = current
		}
		r.entry.things = append(r.entry.things, thingChange{category: category, id: id, before: before})
	}
	r.entry.things[indexByID[id]].after = object
	setSlot(r.project, category, id, object)
	r.project.delta.thing(category, id)
}

func setSlot(project *Project, category thing.Category, id uint32, object *thing.Thing) {
	list := project.things.Things[category]
	i := int(id - category.MinID())
	for len(list) <= i {
		list = append(list, nil)
	}
	list[i] = object
	// Trim trailing nil slots (removed last things).
	for len(list) > 0 && list[len(list)-1] == nil {
		list = list[:len(list)-1]
	}
	project.things.Things[category] = list
}

func (r *recorder) setSprite(id uint32, compressed []byte) {
	if len(compressed) == 0 {
		compressed = nil
	}
	r.setSpriteSlot(id, spriteSlot{kind: slotData, data: compressed})
}

// moveSprite makes sprite dst read what sprite src reads now, without
// reading the sprite.
func (r *recorder) moveSprite(dst, src uint32) {
	r.setSpriteSlot(dst, r.project.sprites.slotOf(src))
}

func (r *recorder) setSpriteSlot(id uint32, slot spriteSlot) {
	i, ok := r.spriteIndex[id]
	if !ok {
		i = len(r.entry.sprites)
		r.spriteIndex[id] = i
		r.entry.sprites = append(r.entry.sprites, spriteChange{id: id, before: r.project.sprites.slot(id)})
	}
	r.project.sprites.setSlot(id, slot)
	r.entry.sprites[i].after = r.project.sprites.slot(id)
	r.project.delta.sprite(id)
}

func (r *recorder) setSpriteCount(count uint32) {
	r.project.delta.spriteRange(r.project.sprites.count(), count)
	r.project.sprites.setCount(count)
}

func (r *recorder) commit() {
	for _, category := range thing.Categories {
		r.entry.lenAfter[category] = len(r.project.things.Things[category])
	}
	r.entry.spritesCount[1] = r.project.sprites.count()
	if len(r.entry.things) == 0 && len(r.entry.sprites) == 0 && r.entry.spritesCount[0] == r.entry.spritesCount[1] {
		return
	}
	r.project.applyMoves(r.entry.itemMoves)
	r.project.history.push(r.entry)
}

func (p *Project) applyEdit(entry *historyEntry, forward bool) {
	lengths, count := entry.lenBefore, entry.spritesCount[0]
	if forward {
		lengths, count = entry.lenAfter, entry.spritesCount[1]
	}
	for _, category := range thing.Categories {
		list := p.things.Things[category]
		length := lengths[category]
		for len(list) < length {
			list = append(list, nil)
		}
		p.things.Things[category] = list[:length]
	}
	for _, category := range thing.Categories {
		if entry.lenBefore[category] != entry.lenAfter[category] {
			for i := min(entry.lenBefore[category], entry.lenAfter[category]); i < max(entry.lenBefore[category], entry.lenAfter[category]); i++ {
				p.delta.thing(category, category.MinID()+uint32(i))
			}
		}
	}
	p.delta.spriteRange(p.sprites.count(), count)
	for _, change := range entry.things {
		p.delta.thing(change.category, change.id)
		object := change.before
		if forward {
			object = change.after
		}
		i := int(change.id - change.category.MinID())
		if object != nil && i < len(p.things.Things[change.category]) {
			p.things.Things[change.category][i] = object
		}
	}
	if forward {
		p.applyMoves(entry.itemMoves)
	} else {
		p.applyMoves(invertMoves(entry.itemMoves))
	}
	p.sprites.setCount(count)
	for _, change := range entry.sprites {
		p.delta.sprite(change.id)
		if change.id > count {
			continue
		}
		if forward {
			p.sprites.setSlot(change.id, change.after)
		} else {
			p.sprites.setSlot(change.id, change.before)
		}
	}
}

// Undo reverts the last edit. It returns the edit label, or "" when there
// is nothing to undo.
func (p *Project) Undo() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.history.canUndo() {
		return ""
	}
	entry := p.history.undo[len(p.history.undo)-1]
	p.history.undo = p.history.undo[:len(p.history.undo)-1]
	p.applyEdit(entry, false)
	p.history.redo = append(p.history.redo, entry)
	return entry.label
}

// Redo re-applies the last undone edit.
func (p *Project) Redo() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.history.canRedo() {
		return ""
	}
	entry := p.history.redo[len(p.history.redo)-1]
	p.history.redo = p.history.redo[:len(p.history.redo)-1]
	p.applyEdit(entry, true)
	p.history.undo = append(p.history.undo, entry)
	return entry.label
}
