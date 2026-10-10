package project

import (
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Diff statuses of one id in two clients.
const (
	DiffChanged = "changed"
	DiffOnlyA   = "onlyA" // the id does not exist in B, or is empty there
	DiffOnlyB   = "onlyB" // the id does not exist in A, or is empty there
)

// Parts of a thing that can differ (DiffEntry.Changes).
const (
	ChangeName      = "name"
	ChangeProps     = "props"
	ChangeNpcSales  = "npcSales"
	ChangeSize      = "size"      // width, height or exact size
	ChangePatterns  = "patterns"  // layers or patterns
	ChangeFrames    = "frames"    // frame count
	ChangeGroups    = "groups"    // frame groups that cannot be compared frame by frame
	ChangeAnimation = "animation" // durations, mode, loop count or start frame
	ChangeSprites   = "sprites"
)

// DiffEntry is one id that differs between two clients.
type DiffEntry struct {
	ID      uint32   `json:"id"`
	Status  string   `json:"status"`
	Changes []string `json:"changes"`
	// Empty marks an id that exists in one client only and has no sprite
	// with pixels there.
	Empty bool `json:"empty,omitempty"`
}

// DiffResult compares one category of two clients. Entries lists only the
// ids that differ, in order; Same counts the others.
type DiffResult struct {
	Entries []DiffEntry `json:"entries"`
	Same    int         `json:"same"`
}

// Diff compares the things of category c in a and b. Sprites are compared
// by their pixels, not by their ids, so the same object stored under other
// sprite ids is equal. Differences that only come from the formats of the
// clients are ignored: an outfit with idle and walking groups equals the
// same frames in one group, animation settings count only when both clients
// store them, and exact size only when both use the same format.
func Diff(a, b *Project, c thing.Category) (DiffResult, error) {
	if !c.Valid() {
		return DiffResult{}, fmt.Errorf("invalid category %d", c)
	}
	if a.SpriteSize() != b.SpriteSize() {
		return DiffResult{}, fmt.Errorf("clients use %dpx and %dpx sprites", a.SpriteSize(), b.SpriteSize())
	}
	// Writers never wait for a second project while they hold their lock
	// (Transfer copies the source first), so holding both read locks is safe.
	a.mu.RLock()
	defer a.mu.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	opts := compareOptions{
		animation: a.storesAnimation() && b.storesAnimation(),
		exactSize: a.format == b.format,
	}
	first, last := c.MinID(), max(a.things.MaxID(c), b.things.MaxID(c))
	if last < first {
		return DiffResult{Entries: []DiffEntry{}}, nil
	}
	// Chunks are compared in parallel (sprite reads may decode asset sheets)
	// and joined in id order.
	size := thingChunk(last - first + 1)
	parts := make([]DiffResult, (last-first)/size+1)
	err := parallelRange(first, last, size, func(from, to uint32) error {
		cmp := thingComparer{opts: opts, sprites: spriteComparer{a: a.sprites, b: b.sprites, cache: map[[2]uint32]bool{}}}
		part, err := diffRange(a, b, c, from, to, &cmp)
		parts[(from-first)/size] = part
		return err
	})
	if err != nil {
		return DiffResult{}, err
	}
	res := DiffResult{Entries: []DiffEntry{}}
	for _, part := range parts {
		res.Entries = append(res.Entries, part.Entries...)
		res.Same += part.Same
	}
	return res, nil
}

func diffRange(a, b *Project, c thing.Category, from, to uint32, cmp *thingComparer) (DiffResult, error) {
	var res DiffResult
	for id := from; id <= to; id++ {
		ta, tb := a.things.Get(c, id), b.things.Get(c, id)
		switch {
		case ta == nil && tb == nil:
		case tb == nil:
			res.Entries = append(res.Entries, DiffEntry{ID: id, Status: DiffOnlyA, Empty: !a.hasPixels(ta)})
		case ta == nil:
			res.Entries = append(res.Entries, DiffEntry{ID: id, Status: DiffOnlyB, Empty: !b.hasPixels(tb)})
		default:
			ch, err := cmp.compare(ta, tb)
			if err != nil {
				return DiffResult{}, fmt.Errorf("%s %d: %w", c, id, err)
			}
			if len(ch) == 0 {
				res.Same++
				continue
			}
			// An empty placeholder against a real object is a new object,
			// not a change.
			ea, eb := !a.hasPixels(ta), !b.hasPixels(tb)
			switch {
			case ea && !eb:
				res.Entries = append(res.Entries, DiffEntry{ID: id, Status: DiffOnlyB})
			case eb && !ea:
				res.Entries = append(res.Entries, DiffEntry{ID: id, Status: DiffOnlyA})
			default:
				res.Entries = append(res.Entries, DiffEntry{ID: id, Status: DiffChanged, Changes: ch})
			}
		}
	}
	return res, nil
}

// storesAnimation reports whether the client stores frame durations, loop
// count and start frame; otherwise they are defaults made up on load.
func (p *Project) storesAnimation() bool {
	return p.format == FormatAssets || p.features.ImprovedAnimations
}

type compareOptions struct {
	animation bool
	exactSize bool
}

type thingComparer struct {
	opts    compareOptions
	sprites spriteComparer
}

func (tc *thingComparer) compare(a, b *thing.Thing) ([]string, error) {
	var out []string
	if a.Name != b.Name || a.Description != b.Description {
		out = append(out, ChangeName)
	}
	if a.Props != b.Props {
		out = append(out, ChangeProps)
	}
	if !slices.Equal(a.NpcSales, b.NpcSales) {
		out = append(out, ChangeNpcSales)
	}
	ga, gb := a.FrameGroups, b.FrameGroups
	if len(ga) != len(gb) {
		// One client splits outfits into idle and walking groups, the other
		// keeps every frame in one: compare all frames in one group.
		sa, okA := a.Stacked()
		sb, okB := b.Stacked()
		if !okA || !okB {
			return append(out, ChangeGroups), nil
		}
		ga, gb = []*thing.FrameGroup{sa}, []*thing.FrameGroup{sb}
	}
	geometry := false
	for i, x := range ga {
		y := gb[i]
		if x.Width != y.Width || x.Height != y.Height || (tc.opts.exactSize && x.ExactSize != y.ExactSize) {
			out = appendOnce(out, ChangeSize)
			geometry = true
		}
		if x.Layers != y.Layers || x.PatternX != y.PatternX || x.PatternY != y.PatternY || x.PatternZ != y.PatternZ {
			out = appendOnce(out, ChangePatterns)
			geometry = true
		}
		if x.Frames != y.Frames {
			out = appendOnce(out, ChangeFrames)
			geometry = true
		}
		if tc.opts.animation && x.Frames == y.Frames && x.Frames > 1 &&
			(x.Mode != y.Mode || x.LoopCount != y.LoopCount || x.StartFrame != y.StartFrame || !slices.Equal(x.Durations, y.Durations)) {
			out = appendOnce(out, ChangeAnimation)
		}
	}
	if geometry {
		return out, nil
	}
	for i, x := range ga {
		y := gb[i]
		for j, id := range x.Sprites {
			eq, err := tc.sprites.equal(id, y.Sprites[j])
			if err != nil {
				return nil, err
			}
			if !eq {
				return append(out, ChangeSprites), nil
			}
		}
	}
	return out, nil
}

func appendOnce(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// spriteComparer compares sprites of two stores by content.
type spriteComparer struct {
	a, b  *spriteStore
	cache map[[2]uint32]bool
}

func (s *spriteComparer) equal(ida, idb uint32) (bool, error) {
	key := [2]uint32{ida, idb}
	if eq, ok := s.cache[key]; ok {
		return eq, nil
	}
	ca, err := s.a.compressedOrEmpty(ida)
	if err != nil {
		return false, err
	}
	cb, err := s.b.compressedOrEmpty(idb)
	if err != nil {
		return false, err
	}
	var eq bool
	if s.a.transparent == s.b.transparent {
		eq = bytes.Equal(ca, cb)
	} else {
		// Different transparency settings encode the same pixels differently.
		pa, err := spr.Decompress(ca, s.a.size, s.a.transparent)
		if err != nil {
			return false, err
		}
		pb, err := spr.Decompress(cb, s.b.size, s.b.transparent)
		if err != nil {
			return false, err
		}
		eq = bytes.Equal(pa, pb)
	}
	s.cache[key] = eq
	return eq, nil
}

// compressedOrEmpty is compressed with id 0 (no sprite) being empty.
func (s *spriteStore) compressedOrEmpty(id uint32) ([]byte, error) {
	if id == 0 {
		return nil, nil
	}
	return s.compressed(id)
}

// TransferResult reports a Transfer.
type TransferResult struct {
	// IDs are the ids of the copied things in the target, in request order.
	IDs []uint32 `json:"ids"`
	// Sprites counts the sprites added to the target.
	Sprites int `json:"sprites"`
	// Before holds the things the copies replaced (nil: the id did not
	// exist), for copies that kept their ids. See RestoreThings.
	Before map[uint32]*thing.Thing `json:"-"`
}

// Transfer copies things of category c from src into dst together with
// their sprites. With appendNew the copies get new ids at the end of the
// category; otherwise they keep their ids, replacing the things in dst
// (ids past the end of dst extend it, filling any gap with empty things).
// A sprite is reused when dst already has the same pixels under the same
// id, or when it was already added by this transfer. Outfits with frame
// groups are merged into one group when dst has no frame groups.
// The whole transfer is one undoable edit in dst.
func Transfer(dst, src *Project, c thing.Category, ids []uint32, appendNew bool) (TransferResult, error) {
	if dst == src {
		return TransferResult{}, fmt.Errorf("cannot copy a client into itself")
	}
	if !c.Valid() {
		return TransferResult{}, fmt.Errorf("invalid category %d", c)
	}
	if dst.SpriteSize() != src.SpriteSize() {
		return TransferResult{}, fmt.Errorf("clients use %dpx and %dpx sprites", dst.SpriteSize(), src.SpriteSize())
	}
	snap, err := src.snapshot(c, ids)
	if err != nil {
		return TransferResult{}, err
	}

	dst.mu.Lock()
	defer dst.mu.Unlock()
	things := snap.things
	groups := dst.features.FrameGroups || dst.format == FormatAssets
	for i, t := range things {
		if len(t.FrameGroups) > 1 && !groups {
			m := mergeGroups(t)
			if m == nil {
				return TransferResult{}, fmt.Errorf("%s %d: idle and walking layouts differ, the target client has no frame groups", c, t.ID)
			}
			things[i] = m
		}
	}

	label := fmt.Sprintf("Copy %d %s(s)", len(things), c)
	if snap.name != "" {
		label += " from " + snap.name
	}
	r := dst.record(label)
	res := TransferResult{IDs: make([]uint32, 0, len(things)), Before: map[uint32]*thing.Thing{}}
	mapped := map[uint32]uint32{}
	added := map[string]uint32{}
	for _, t := range things {
		for _, g := range t.FrameGroups {
			for i, sid := range g.Sprites {
				nid, ok := mapped[sid]
				if !ok {
					if nid, err = dst.importSprite(r, sid, snap.sprites[sid], snap.transparent, added, &res.Sprites); err != nil {
						return TransferResult{}, err
					}
					mapped[sid] = nid
				}
				g.Sprites[i] = nid
			}
		}
		cur := (*thing.Thing)(nil)
		if appendNew {
			t.ID = dst.things.MaxID(c) + 1
		} else {
			cur = dst.things.Get(c, t.ID)
			if _, ok := res.Before[t.ID]; !ok {
				res.Before[t.ID] = cur
			}
			for id := dst.things.MaxID(c) + 1; id < t.ID; id++ {
				r.setThing(c, id, thing.New(id, c))
			}
		}
		if snap.format != dst.format {
			t.Extra = nil
			keepExtra(t, cur)
		}
		if err := t.Validate(); err != nil {
			return TransferResult{}, err
		}
		r.setThing(c, t.ID, t)
		res.IDs = append(res.IDs, t.ID)
	}
	r.commit()
	return res, nil
}

// RestoreThings puts back things of category c as they were before a
// Transfer (see TransferResult.Before), as one undoable edit. A nil thing
// did not exist: it is removed when it is the last one, else emptied.
func (p *Project) RestoreThings(c thing.Category, before map[uint32]*thing.Thing) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := slices.Sorted(maps.Keys(before))
	slices.Reverse(ids)
	for _, id := range ids {
		if t := before[id]; t != nil {
			if err := p.checkSpriteIDs(t); err != nil {
				return fmt.Errorf("%s %d: %w", c, id, err)
			}
		}
	}
	r := p.record(fmt.Sprintf("Revert %d %s(s)", len(ids), c))
	for _, id := range ids {
		t := before[id]
		switch {
		case t != nil:
		case id == p.things.MaxID(c) && id != c.MinID():
		default:
			t = thing.New(id, c)
		}
		if t == nil && p.things.Get(c, id) == nil {
			continue
		}
		r.setThing(c, id, t)
	}
	r.commit()
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
func (p *Project) snapshot(c thing.Category, ids []uint32) (*thingSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := &thingSnapshot{
		things:      make([]*thing.Thing, 0, len(ids)),
		sprites:     map[uint32][]byte{},
		transparent: p.sprites.transparent,
		format:      p.format,
		name:        p.name(),
	}
	for _, id := range ids {
		t := p.things.Get(c, id)
		if t == nil {
			return nil, fmt.Errorf("%s %d does not exist", c, id)
		}
		s.things = append(s.things, t.Clone())
		for _, sid := range t.SpriteIDs() {
			if _, ok := s.sprites[sid]; ok || sid == 0 {
				continue
			}
			c, err := p.sprites.compressed(sid)
			if err != nil {
				return nil, err
			}
			s.sprites[sid] = c
		}
	}
	return s, nil
}

// name is a short name of the client for labels.
func (p *Project) name() string {
	if p.datPath == "" {
		return ""
	}
	if p.format == FormatAssets {
		return filepath.Base(p.datPath)
	}
	return filepath.Base(filepath.Dir(p.datPath)) + "/" + filepath.Base(p.datPath)
}

// importSprite stores source sprite sid (compressed with the source
// transparency) and returns the id to use in p. Empty sprites map to 0.
func (p *Project) importSprite(r *recorder, sid uint32, c []byte, transparent bool, added map[string]uint32, count *int) (uint32, error) {
	if sid == 0 {
		return 0, nil
	}
	if transparent != p.sprites.transparent {
		var err error
		if c, err = spr.Transcode(c, p.sprites.size, transparent, p.sprites.transparent); err != nil {
			return 0, err
		}
	}
	if len(c) == 0 {
		return 0, nil
	}
	if id, ok := added[string(c)]; ok {
		return id, nil
	}
	if sid <= p.sprites.count() {
		cur, err := p.sprites.compressed(sid)
		if err != nil {
			return 0, err
		}
		if bytes.Equal(cur, c) {
			return sid, nil
		}
	}
	id := p.sprites.count() + 1
	r.setSpriteCount(id)
	r.setSprite(id, c)
	added[string(c)] = id
	*count++
	return id, nil
}
