package project

import "github.com/nekiro/ots-creator/internal/thing"

const maxHistory = 200

type thingChange struct {
	cat           thing.Category
	id            uint32
	before, after *thing.Thing // nil when the slot does not exist
}

type spriteChange struct {
	id            uint32
	before, after []byte
}

// edit is one undoable operation. It records list lengths before and after
// plus every slot it touched, so applying it in either direction is exact.
type edit struct {
	label        string
	lenBefore    map[thing.Category]int
	lenAfter     map[thing.Category]int
	things       []thingChange
	sprites      []spriteChange
	spritesCount [2]uint32 // before, after
}

type history struct {
	undo []*edit
	redo []*edit
	// clean is the top of the undo stack when the files were last written
	// (nil: empty stack). Undoing back to it means nothing is left to save.
	clean *edit
}

// lost marks a saved state that undo can no longer reach.
var lost = &edit{}

func (h *history) push(e *edit) {
	h.undo = append(h.undo, e)
	if len(h.undo) > maxHistory {
		drop := h.undo[:len(h.undo)-maxHistory]
		if h.clean == nil {
			h.clean = lost
		}
		for _, d := range drop {
			if d == h.clean {
				h.clean = lost
			}
		}
		h.undo = h.undo[len(h.undo)-maxHistory:]
	}
	h.redo = nil
}

func (h *history) top() *edit {
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
	p       *Project
	e       *edit
	touched map[thing.Category]map[uint32]int
	sprIdx  map[uint32]int
}

func (p *Project) record(label string) *recorder {
	e := &edit{label: label, lenBefore: map[thing.Category]int{}, lenAfter: map[thing.Category]int{}}
	for _, c := range thing.Categories {
		e.lenBefore[c] = len(p.things.Things[c])
	}
	e.spritesCount[0] = p.sprites.count()
	return &recorder{p: p, e: e, touched: map[thing.Category]map[uint32]int{}, sprIdx: map[uint32]int{}}
}

// setThing stores t (or removes the slot when t is nil and it is the last).
func (r *recorder) setThing(c thing.Category, id uint32, t *thing.Thing) {
	m := r.touched[c]
	if m == nil {
		m = map[uint32]int{}
		r.touched[c] = m
	}
	if _, ok := m[id]; !ok {
		m[id] = len(r.e.things)
		var before *thing.Thing
		if cur := r.p.things.Get(c, id); cur != nil {
			before = cur
		}
		r.e.things = append(r.e.things, thingChange{cat: c, id: id, before: before})
	}
	r.e.things[m[id]].after = t
	setSlot(r.p, c, id, t)
	r.p.delta.thing(c, id)
}

func setSlot(p *Project, c thing.Category, id uint32, t *thing.Thing) {
	list := p.things.Things[c]
	i := int(id - c.MinID())
	for len(list) <= i {
		list = append(list, nil)
	}
	list[i] = t
	// Trim trailing nil slots (removed last things).
	for len(list) > 0 && list[len(list)-1] == nil {
		list = list[:len(list)-1]
	}
	p.things.Things[c] = list
}

func (r *recorder) setSprite(id uint32, c []byte) {
	if _, ok := r.sprIdx[id]; !ok {
		before, _ := r.p.sprites.compressed(id)
		r.sprIdx[id] = len(r.e.sprites)
		r.e.sprites = append(r.e.sprites, spriteChange{id: id, before: before})
	}
	r.e.sprites[r.sprIdx[id]].after = c
	r.p.sprites.set(id, c)
	r.p.delta.sprite(id)
}

func (r *recorder) setSpriteCount(n uint32) {
	r.p.delta.spriteRange(r.p.sprites.count(), n)
	r.p.sprites.setCount(n)
}

func (r *recorder) commit() {
	for _, c := range thing.Categories {
		r.e.lenAfter[c] = len(r.p.things.Things[c])
	}
	r.e.spritesCount[1] = r.p.sprites.count()
	if len(r.e.things) == 0 && len(r.e.sprites) == 0 && r.e.spritesCount[0] == r.e.spritesCount[1] {
		return
	}
	r.p.history.push(r.e)
}

func (p *Project) applyEdit(e *edit, forward bool) {
	lens, count := e.lenBefore, e.spritesCount[0]
	if forward {
		lens, count = e.lenAfter, e.spritesCount[1]
	}
	for _, c := range thing.Categories {
		list := p.things.Things[c]
		n := lens[c]
		for len(list) < n {
			list = append(list, nil)
		}
		p.things.Things[c] = list[:n]
	}
	for _, c := range thing.Categories {
		if e.lenBefore[c] != e.lenAfter[c] {
			for i := min(e.lenBefore[c], e.lenAfter[c]); i < max(e.lenBefore[c], e.lenAfter[c]); i++ {
				p.delta.thing(c, c.MinID()+uint32(i))
			}
		}
	}
	p.delta.spriteRange(p.sprites.count(), count)
	for _, ch := range e.things {
		p.delta.thing(ch.cat, ch.id)
		t := ch.before
		if forward {
			t = ch.after
		}
		i := int(ch.id - ch.cat.MinID())
		if t != nil && i < len(p.things.Things[ch.cat]) {
			p.things.Things[ch.cat][i] = t
		}
	}
	p.sprites.setCount(count)
	for _, ch := range e.sprites {
		p.delta.sprite(ch.id)
		if ch.id > count {
			continue
		}
		if forward {
			p.sprites.set(ch.id, ch.after)
		} else {
			p.sprites.set(ch.id, ch.before)
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
	e := p.history.undo[len(p.history.undo)-1]
	p.history.undo = p.history.undo[:len(p.history.undo)-1]
	p.applyEdit(e, false)
	p.history.redo = append(p.history.redo, e)
	return e.label
}

// Redo re-applies the last undone edit.
func (p *Project) Redo() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.history.canRedo() {
		return ""
	}
	e := p.history.redo[len(p.history.redo)-1]
	p.history.redo = p.history.redo[:len(p.history.redo)-1]
	p.applyEdit(e, true)
	p.history.undo = append(p.history.undo, e)
	return e.label
}
