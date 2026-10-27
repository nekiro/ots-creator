package project

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// PropsPatch maps JSON property names (as in thing.Properties) to new
// values. Nested objects such as "market" are merged key by key.
type PropsPatch map[string]any

func (p *Project) PatchThings(category thing.Category, ids []uint32, patch PropsPatch) (int, error) {
	if len(patch) == 0 {
		return 0, nil
	}
	if err := checkPatchKeys(patch); err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	edit := p.record(fmt.Sprintf("Edit %d %s(s)", len(ids), category))
	changed := 0
	for _, id := range ids {
		current := p.things.Get(category, id)
		if current == nil {
			return 0, fmt.Errorf("%s %d does not exist", category, id)
		}
		object := current.Clone()
		if err := applyPatch(&object.Props, patch); err != nil {
			return 0, err
		}
		if object.Props == current.Props {
			continue
		}
		edit.setThing(category, id, object)
		changed++
	}
	edit.commit()
	return changed, nil
}

func checkPatchKeys(patch PropsPatch) error {
	known, err := propsMap(&thing.Properties{})
	if err != nil {
		return err
	}
	for key := range patch {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("unknown property %q", key)
		}
	}
	return nil
}

func propsMap(props *thing.Properties) (map[string]any, error) {
	data, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	return values, json.Unmarshal(data, &values)
}

func applyPatch(props *thing.Properties, patch PropsPatch) error {
	values, err := propsMap(props)
	if err != nil {
		return err
	}
	for key, value := range patch {
		if nested, ok := value.(map[string]any); ok {
			if current, ok := values[key].(map[string]any); ok {
				merged := maps.Clone(current)
				maps.Copy(merged, nested)
				value = merged
			}
		}
		values[key] = value
	}
	data, err := json.Marshal(values)
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

func (p *Project) UpdateThings(objects []*thing.Thing) error {
	for _, object := range objects {
		if err := object.Validate(); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, object := range objects {
		if p.things.Get(object.Category, object.ID) == nil {
			return fmt.Errorf("%s %d does not exist", object.Category, object.ID)
		}
		if err := p.checkSpriteIDs(object); err != nil {
			return err
		}
	}
	edit := p.record(fmt.Sprintf("Edit %d object(s)", len(objects)))
	for _, object := range objects {
		edit.setThing(object.Category, object.ID, keepExtra(object.Clone(), p.things.Get(object.Category, object.ID)))
	}
	edit.commit()
	return nil
}

func (p *Project) SetDurations(categories []thing.Category, minMs, maxMs uint32) (int, error) {
	if maxMs < minMs {
		return 0, fmt.Errorf("maximum duration %d is below minimum %d", maxMs, minMs)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	edit := p.record("Set frame durations")
	changed := 0
	want := thing.FrameDuration{Min: minMs, Max: maxMs}
	for _, category := range categories {
		for _, current := range p.things.Things[category] {
			if current == nil || !needsDurations(current, want) {
				continue
			}
			object := current.Clone()
			for _, group := range object.FrameGroups {
				for i := range group.Durations {
					group.Durations[i] = want
				}
			}
			edit.setThing(category, object.ID, object)
			changed++
		}
	}
	edit.commit()
	return changed, nil
}

func needsDurations(object *thing.Thing, want thing.FrameDuration) bool {
	for _, group := range object.FrameGroups {
		for _, duration := range group.Durations {
			if duration != want {
				return true
			}
		}
	}
	return false
}

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
	edit := p.record(label)
	var result ConvertResult
	for _, current := range p.things.Things[thing.CategoryOutfit] {
		if current == nil {
			continue
		}
		var object *thing.Thing
		if toGroups && len(current.FrameGroups) == 1 {
			object = splitGroups(current)
		} else if !toGroups && len(current.FrameGroups) > 1 {
			if object = mergeGroups(current); object == nil {
				result.Skipped++
				continue
			}
		}
		if object != nil {
			edit.setThing(thing.CategoryOutfit, object.ID, object)
			result.Converted++
		}
	}
	edit.commit()
	return result
}

func frameBlock(group *thing.FrameGroup, from, to int) []uint32 {
	spritesPerFrame := group.TotalSprites() / max(int(group.Frames), 1)
	return append([]uint32(nil), group.Sprites[from*spritesPerFrame:to*spritesPerFrame]...)
}

func splitGroups(current *thing.Thing) *thing.Thing {
	object := current.Clone()
	group := object.FrameGroups[0]
	defaultDuration := thing.CategoryOutfit.DefaultDuration()

	idle := group.Clone()
	idle.Type = thing.FrameGroupDefault
	idle.Frames = 1
	idle.Sprites = frameBlock(group, 0, 1)
	idle.StartFrame = 0
	idle.EnsureDurations(defaultDuration)

	walking := idle.Clone()
	walking.Type = thing.FrameGroupWalking
	if group.Frames > 1 {
		walking = group.Clone()
		walking.Type = thing.FrameGroupWalking
		walking.Frames = group.Frames - 1
		walking.Sprites = frameBlock(group, 1, int(group.Frames))
		if len(group.Durations) == int(group.Frames) {
			walking.Durations = append([]thing.FrameDuration(nil), group.Durations[1:]...)
		}
		walking.StartFrame = 0
		walking.EnsureDurations(defaultDuration)
	}
	object.FrameGroups = []*thing.FrameGroup{idle, walking}
	return object
}

func mergeGroups(current *thing.Thing) *thing.Thing {
	idle, walking := current.FrameGroups[0], current.FrameGroups[1]
	if idle.Width != walking.Width || idle.Height != walking.Height || idle.Layers != walking.Layers ||
		idle.PatternX != walking.PatternX || idle.PatternY != walking.PatternY || idle.PatternZ != walking.PatternZ ||
		int(walking.Frames)+1 > 255 {
		return nil
	}
	defaultDuration := thing.CategoryOutfit.DefaultDuration()
	group := walking.Clone()
	group.Type = thing.FrameGroupDefault
	group.Frames = walking.Frames + 1
	group.Sprites = append(frameBlock(idle, 0, 1), walking.Sprites...)
	first := thing.FrameDuration{Min: defaultDuration, Max: defaultDuration}
	if len(idle.Durations) > 0 {
		first = idle.Durations[0]
	}
	rest := walking.Durations
	if len(rest) != int(walking.Frames) {
		rest = nil
		for range walking.Frames {
			rest = append(rest, thing.FrameDuration{Min: defaultDuration, Max: defaultDuration})
		}
	}
	group.Durations = append([]thing.FrameDuration{first}, rest...)
	group.StartFrame = 0
	object := current.Clone()
	object.FrameGroups = []*thing.FrameGroup{group}
	return object
}
