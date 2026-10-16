package project

import (
	"fmt"
	"image"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// SetGroupPixels replaces every sprite of one frame group with new pixels
// (indexed by sprite slot), for example from an imported sprite sheet.
func (p *Project) SetGroupPixels(category thing.Category, id uint32, groupIndex int, pixels [][]byte) error {
	for _, spritePixels := range pixels {
		if err := p.checkPixels(spritePixels); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.things.Get(category, id)
	if current == nil {
		return fmt.Errorf("%s %d does not exist", category, id)
	}
	if groupIndex < 0 || groupIndex >= len(current.FrameGroups) {
		return fmt.Errorf("%s %d has no frame group %d", category, id, groupIndex)
	}
	object := current.Clone()
	group := object.FrameGroups[groupIndex]
	if len(pixels) != len(group.Sprites) {
		return fmt.Errorf("got %d sprites, frame group has %d slots", len(pixels), len(group.Sprites))
	}
	edit := p.record(fmt.Sprintf("Import sheet into %s %d", category, id))
	copy(group.Sprites, p.addSpritesDedup(edit, pixels))
	edit.setThing(category, id, object)
	edit.commit()
	return nil
}

// ImportSheet puts a sprite sheet into a thing and picks the layout from
// the sheet (see imaging.InferLayout). Outfit sheets hold every frame; with
// frame groups the first frame becomes the idle group and the others the
// walking group, unless the thing already splits its frames another way.
func (p *Project) ImportSheet(category thing.Category, id uint32, img *image.NRGBA) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.things.Get(category, id)
	if current == nil {
		return fmt.Errorf("%s %d does not exist", category, id)
	}
	size := p.features.SpriteSize
	merged, err := imaging.InferLayout(img, category, current, size)
	if err != nil {
		return err
	}
	pixels, err := imaging.SliceSheet(img, merged, size)
	if err != nil {
		return err
	}
	edit := p.record(fmt.Sprintf("Import sheet into %s %d", category, id))
	copy(merged.Sprites, p.addSpritesDedup(edit, pixels))
	object := current.Clone()
	object.FrameGroups = splitSheetGroups(merged, current, category == thing.CategoryOutfit && p.features.FrameGroups)
	edit.setThing(category, id, object)
	edit.commit()
	return nil
}

func splitSheetGroups(merged *thing.FrameGroup, current *thing.Thing, groups bool) []*thing.FrameGroup {
	defaultDuration := current.Category.DefaultDuration()
	// Animation settings and durations of the replaced group, when they fit.
	adopt := func(group *thing.FrameGroup, i int) *thing.FrameGroup {
		if i < len(current.FrameGroups) {
			old := current.FrameGroups[i]
			group.Mode, group.LoopCount = old.Mode, old.LoopCount
			if old.Frames == group.Frames {
				group.Durations = slices.Clone(old.Durations)
				group.StartFrame = old.StartFrame
			}
		}
		group.EnsureDurations(defaultDuration)
		return group
	}
	if !groups || merged.Frames < 2 {
		group := merged.Clone()
		group.Type = thing.FrameGroupDefault
		return []*thing.FrameGroup{adopt(group, 0)}
	}
	idleFrames := 1
	if len(current.FrameGroups) == 2 && int(current.FrameGroups[0].Frames)+int(current.FrameGroups[1].Frames) == int(merged.Frames) {
		idleFrames = int(current.FrameGroups[0].Frames)
	}
	idle := merged.Clone()
	idle.Type, idle.Frames = thing.FrameGroupDefault, uint8(idleFrames)
	idle.Sprites = frameBlock(merged, 0, idleFrames)
	walking := merged.Clone()
	walking.Type, walking.Frames = thing.FrameGroupWalking, merged.Frames-uint8(idleFrames)
	walking.Sprites = frameBlock(merged, idleFrames, int(merged.Frames))
	return []*thing.FrameGroup{adopt(idle, 0), adopt(walking, 1)}
}
