package project

import (
	"errors"
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/spr"
)

// ErrShiftCut is returned by ShiftPixels when visible pixels would be moved
// out of a texture.
var ErrShiftCut = errors.New("visible pixels would be moved out of the texture")

// SetSlotPixels replaces the pixels of sprite slots of one frame group, for
// example after painting. A sprite can be shared by other slots and things,
// so it is never edited in place: changed slots get new sprites (identical
// ones are added once), fully transparent pixels clear the slot and
// unchanged pixels keep their sprite. It is one undoable edit.
func (p *Project) SetSlotPixels(category thing.Category, id uint32, groupIndex int, slots []int, pixels [][]byte, label string) error {
	if len(slots) != len(pixels) {
		return fmt.Errorf("%d slots for %d sprites", len(slots), len(pixels))
	}
	for _, spritePixels := range pixels {
		if err := p.checkPixels(spritePixels); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	object, group, err := p.groupForEdit(category, id, groupIndex)
	if err != nil {
		return err
	}
	for _, slot := range slots {
		if slot < 0 || slot >= len(group.Sprites) {
			return fmt.Errorf("%s %d has no sprite slot %d", category, id, slot)
		}
	}
	edit := p.record(fmt.Sprintf("%s %s %d", label, category, id))
	if p.setSlotsLocked(edit, group, slots, pixels) {
		edit.setThing(category, id, object)
	}
	edit.commit()
	return nil
}

// groupForEdit returns a copy of a thing and its frame group to change.
func (p *Project) groupForEdit(category thing.Category, id uint32, groupIndex int) (*thing.Thing, *thing.FrameGroup, error) {
	current := p.things.Get(category, id)
	if current == nil {
		return nil, nil, fmt.Errorf("%s %d does not exist", category, id)
	}
	if groupIndex < 0 || groupIndex >= len(current.FrameGroups) {
		return nil, nil, fmt.Errorf("%s %d has no frame group %d", category, id, groupIndex)
	}
	object := current.Clone()
	return object, object.FrameGroups[groupIndex], nil
}

// setSlotsLocked stores pixels in slots of group (see SetSlotPixels) and
// reports whether any slot changed.
func (p *Project) setSlotsLocked(edit *recorder, group *thing.FrameGroup, slots []int, pixels [][]byte) bool {
	changed := false
	added := map[string]uint32{}
	for i, slot := range slots {
		compressed := spr.Compress(pixels[i], p.features.SpriteSize, p.features.Transparency)
		current, _ := p.sprites.compressedOrEmpty(group.Sprites[slot])
		if string(current) == string(compressed) {
			continue
		}
		changed = true
		if len(compressed) == 0 {
			group.Sprites[slot] = 0
			continue
		}
		newID, ok := added[string(compressed)]
		if !ok {
			newID = p.sprites.count() + 1
			edit.setSpriteCount(newID)
			edit.setSprite(newID, compressed)
			added[string(compressed)] = newID
		}
		group.Sprites[slot] = newID
	}
	return changed
}

// ShiftPixels moves the pixels of textures of a frame group by offsetX, offsetY
// (positive: right and down). With all every texture is moved, otherwise
// the texture at pos with all its layers (the outfit color mask moves
// along). It fails with ErrShiftCut when visible pixels would leave a
// texture, so nothing is ever lost.
func (p *Project) ShiftPixels(category thing.Category, id uint32, groupIndex int, offsetX, offsetY int, all bool, pos TexturePos) error {
	if offsetX == 0 && offsetY == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	object, group, err := p.groupForEdit(category, id, groupIndex)
	if err != nil {
		return err
	}
	size := p.features.SpriteSize
	var slots []int
	var pixels [][]byte
	for frame := range int(group.Frames) {
		for z := range int(group.PatternZ) {
			for y := range int(group.PatternY) {
				for x := range int(group.PatternX) {
					if !all && (frame != pos.Frame || z != pos.PatternZ || y != pos.PatternY || x != pos.PatternX) {
						continue
					}
					for layer := range int(group.Layers) {
						img, err := p.textureImage(group, layer, x, y, z, frame)
						if err != nil {
							return err
						}
						moved, ok := imaging.Shift(img, offsetX, offsetY)
						if !ok {
							return fmt.Errorf("%w (frame %d, pattern %d,%d,%d, layer %d)", ErrShiftCut, frame+1, x, y, z, layer)
						}
						for tileY := range int(group.Height) {
							for tileX := range int(group.Width) {
								slots = append(slots, group.SpriteIndex(tileX, tileY, layer, x, y, z, frame))
								pixels = append(pixels, imaging.Tile(moved, (int(group.Width)-1-tileX)*size, (int(group.Height)-1-tileY)*size, size))
							}
						}
					}
				}
			}
		}
	}
	edit := p.record(fmt.Sprintf("Shift pixels of %s %d", category, id))
	if p.setSlotsLocked(edit, group, slots, pixels) {
		edit.setThing(category, id, object)
	}
	edit.commit()
	return nil
}

// textureImage composes one texture (one layer) of a frame group.
func (p *Project) textureImage(group *thing.FrameGroup, layer, x, y, z, frame int) (*image.NRGBA, error) {
	size := p.features.SpriteSize
	img := image.NewNRGBA(image.Rect(0, 0, int(group.Width)*size, int(group.Height)*size))
	for tileY := range int(group.Height) {
		for tileX := range int(group.Width) {
			spriteID := group.Sprites[group.SpriteIndex(tileX, tileY, layer, x, y, z, frame)]
			if spriteID == 0 {
				continue
			}
			pixels, err := p.sprites.pixels(spriteID)
			if err != nil {
				return nil, err
			}
			imaging.PutTile(img, (int(group.Width)-1-tileX)*size, (int(group.Height)-1-tileY)*size, size, pixels)
		}
	}
	return img, nil
}
