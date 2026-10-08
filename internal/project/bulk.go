package project

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/nekiro/ots-creator/internal/thing"
)

// PropsPatch maps JSON property names (as in thing.Properties) to new
// values. Nested objects such as "market" are merged key by key.
type PropsPatch map[string]any

// PatchThings applies the same property changes to many things in one undo
// step and returns how many things changed.
func (p *Project) PatchThings(c thing.Category, ids []uint32, patch PropsPatch) (int, error) {
	if len(patch) == 0 {
		return 0, nil
	}
	if err := checkPatchKeys(patch); err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.record(fmt.Sprintf("Edit %d %s(s)", len(ids), c))
	n := 0
	for _, id := range ids {
		cur := p.things.Get(c, id)
		if cur == nil {
			return 0, fmt.Errorf("%s %d does not exist", c, id)
		}
		t := cur.Clone()
		if err := applyPatch(&t.Props, patch); err != nil {
			return 0, err
		}
		if t.Props == cur.Props {
			continue
		}
		r.setThing(c, id, t)
		n++
	}
	r.commit()
	return n, nil
}

func checkPatchKeys(patch PropsPatch) error {
	known, err := propsMap(&thing.Properties{})
	if err != nil {
		return err
	}
	for k := range patch {
		if _, ok := known[k]; !ok {
			return fmt.Errorf("unknown property %q", k)
		}
	}
	return nil
}

func propsMap(props *thing.Properties) (map[string]any, error) {
	data, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(data, &m)
}

func applyPatch(props *thing.Properties, patch PropsPatch) error {
	m, err := propsMap(props)
	if err != nil {
		return err
	}
	for k, v := range patch {
		if sub, ok := v.(map[string]any); ok {
			if cur, ok := m[k].(map[string]any); ok {
				merged := maps.Clone(cur)
				maps.Copy(merged, sub)
				v = merged
			}
		}
		m[k] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var out thing.Properties
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("invalid property value: %w", err)
	}
	*props = out
	return nil
}

// UpdateThings replaces several things in one undo step.
func (p *Project) UpdateThings(ts []*thing.Thing) error {
	for _, t := range ts {
		if err := t.Validate(); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range ts {
		if p.things.Get(t.Category, t.ID) == nil {
			return fmt.Errorf("%s %d does not exist", t.Category, t.ID)
		}
		if err := p.checkSpriteIDs(t); err != nil {
			return err
		}
	}
	r := p.record(fmt.Sprintf("Edit %d object(s)", len(ts)))
	for _, t := range ts {
		r.setThing(t.Category, t.ID, t.Clone())
	}
	r.commit()
	return nil
}

// SetDurations gives every frame of every animated thing in the given
// categories the same duration range. It returns how many things changed.
func (p *Project) SetDurations(cats []thing.Category, minMs, maxMs uint32) (int, error) {
	if maxMs < minMs {
		return 0, fmt.Errorf("maximum duration %d is below minimum %d", maxMs, minMs)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.record("Set frame durations")
	n := 0
	want := thing.FrameDuration{Min: minMs, Max: maxMs}
	for _, c := range cats {
		for _, cur := range p.things.Things[c] {
			if cur == nil || !needsDurations(cur, want) {
				continue
			}
			t := cur.Clone()
			for _, g := range t.FrameGroups {
				for i := range g.Durations {
					g.Durations[i] = want
				}
			}
			r.setThing(c, t.ID, t)
			n++
		}
	}
	r.commit()
	return n, nil
}

func needsDurations(t *thing.Thing, want thing.FrameDuration) bool {
	for _, g := range t.FrameGroups {
		for _, d := range g.Durations {
			if d != want {
				return true
			}
		}
	}
	return false
}

// ConvertResult reports a frame group conversion.
type ConvertResult struct {
	Converted int `json:"converted"`
	// Skipped counts outfits whose idle and walking layouts differ, so they
	// cannot be merged into one group.
	Skipped int `json:"skipped"`
}

// ConvertFrameGroups converts every outfit between one frame group (old
// clients: frame 0 is idle, the rest is walking) and separate idle and
// walking groups (clients with the frame groups feature).
func (p *Project) ConvertFrameGroups(toGroups bool) ConvertResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	label := "Split outfits into frame groups"
	if !toGroups {
		label = "Merge outfit frame groups"
	}
	r := p.record(label)
	var res ConvertResult
	for _, cur := range p.things.Things[thing.CategoryOutfit] {
		if cur == nil {
			continue
		}
		var t *thing.Thing
		if toGroups && len(cur.FrameGroups) == 1 {
			t = splitGroups(cur)
		} else if !toGroups && len(cur.FrameGroups) > 1 {
			if t = mergeGroups(cur); t == nil {
				res.Skipped++
				continue
			}
		}
		if t != nil {
			r.setThing(thing.CategoryOutfit, t.ID, t)
			res.Converted++
		}
	}
	r.commit()
	return res
}

// frameBlock returns the sprites of frames [from, to) of g.
func frameBlock(g *thing.FrameGroup, from, to int) []uint32 {
	per := g.TotalSprites() / max(int(g.Frames), 1)
	return append([]uint32(nil), g.Sprites[from*per:to*per]...)
}

func splitGroups(cur *thing.Thing) *thing.Thing {
	t := cur.Clone()
	g := t.FrameGroups[0]
	def := thing.CategoryOutfit.DefaultDuration()

	idle := g.Clone()
	idle.Type = thing.FrameGroupDefault
	idle.Frames = 1
	idle.Sprites = frameBlock(g, 0, 1)
	idle.StartFrame = 0
	idle.EnsureDurations(def)

	walk := idle.Clone()
	walk.Type = thing.FrameGroupWalking
	if g.Frames > 1 {
		walk = g.Clone()
		walk.Type = thing.FrameGroupWalking
		walk.Frames = g.Frames - 1
		walk.Sprites = frameBlock(g, 1, int(g.Frames))
		if len(g.Durations) == int(g.Frames) {
			walk.Durations = append([]thing.FrameDuration(nil), g.Durations[1:]...)
		}
		walk.StartFrame = 0
		walk.EnsureDurations(def)
	}
	t.FrameGroups = []*thing.FrameGroup{idle, walk}
	return t
}

func mergeGroups(cur *thing.Thing) *thing.Thing {
	idle, walk := cur.FrameGroups[0], cur.FrameGroups[1]
	if idle.Width != walk.Width || idle.Height != walk.Height || idle.Layers != walk.Layers ||
		idle.PatternX != walk.PatternX || idle.PatternY != walk.PatternY || idle.PatternZ != walk.PatternZ ||
		int(walk.Frames)+1 > 255 {
		return nil
	}
	def := thing.CategoryOutfit.DefaultDuration()
	g := walk.Clone()
	g.Type = thing.FrameGroupDefault
	g.Frames = walk.Frames + 1
	g.Sprites = append(frameBlock(idle, 0, 1), walk.Sprites...)
	first := thing.FrameDuration{Min: def, Max: def}
	if len(idle.Durations) > 0 {
		first = idle.Durations[0]
	}
	rest := walk.Durations
	if len(rest) != int(walk.Frames) {
		rest = nil
		for range walk.Frames {
			rest = append(rest, thing.FrameDuration{Min: def, Max: def})
		}
	}
	g.Durations = append([]thing.FrameDuration{first}, rest...)
	g.StartFrame = 0
	t := cur.Clone()
	t.FrameGroups = []*thing.FrameGroup{g}
	return t
}
