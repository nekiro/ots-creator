package project

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
)

// ImportOBD adds the sprites of an OBD file and appends its thing to the
// matching category. When replaceID is not zero, that thing is replaced
// instead. It returns the id of the imported thing.
func (p *Project) ImportOBD(obdData *obd.Data, replaceID uint32) (uint32, error) {
	if obdData.SpriteSize != p.features.SpriteSize {
		return 0, fmt.Errorf("object uses %dpx sprites, client uses %dpx", obdData.SpriteSize, p.features.SpriteSize)
	}
	object := obdData.Thing.Clone()
	object.Extra = nil
	if err := object.Validate(); err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	category := object.Category
	if replaceID != 0 && p.things.Get(category, replaceID) == nil {
		return 0, fmt.Errorf("%s %d does not exist", category, replaceID)
	}
	edit := p.record(fmt.Sprintf("Import %s", category))
	for groupIndex, group := range object.FrameGroups {
		pixels := make([][]byte, len(obdData.Sprites[groupIndex]))
		for i, sprite := range obdData.Sprites[groupIndex] {
			pixels[i] = sprite.Pixels
		}
		ids := p.addSpritesDedup(edit, pixels)
		copy(group.Sprites, ids)
	}
	if replaceID != 0 {
		object.ID = replaceID
		keepExtra(object, p.things.Get(category, replaceID))
	} else {
		object.ID = p.things.MaxID(category) + 1
	}
	edit.setThing(category, object.ID, object)
	edit.commit()
	return object.ID, nil
}

// ImportOBDsTo imports OBD files of category, files[i] to targets[i],
// replacing the things there. Targets past the end of the category extend
// it, filling any gap with empty things. The whole import is one undoable
// edit.
func (p *Project) ImportOBDsTo(category thing.Category, files []*obd.Data, targets []uint32) ([]uint32, error) {
	if len(targets) != len(files) {
		return nil, fmt.Errorf("%d target ids for %d %s(s)", len(targets), len(files), category)
	}
	seen := map[uint32]bool{}
	for _, id := range targets {
		if id < category.MinID() {
			return nil, fmt.Errorf("%s id %d is below %d", category, id, category.MinID())
		}
		if seen[id] {
			return nil, fmt.Errorf("target id %d is used twice", id)
		}
		seen[id] = true
	}
	things := make([]*thing.Thing, len(files))
	for i, obdData := range files {
		if obdData.SpriteSize != p.features.SpriteSize {
			return nil, fmt.Errorf("object uses %dpx sprites, client uses %dpx", obdData.SpriteSize, p.features.SpriteSize)
		}
		object := obdData.Thing.Clone()
		object.Extra = nil
		if object.Category != category {
			return nil, fmt.Errorf("object is a %s, not a %s", object.Category, category)
		}
		if err := object.Validate(); err != nil {
			return nil, err
		}
		things[i] = object
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	edit := p.record(fmt.Sprintf("Import %d %s(s)", len(things), category))
	for i, object := range things {
		for groupIndex, group := range object.FrameGroups {
			pixels := make([][]byte, len(files[i].Sprites[groupIndex]))
			for j, sprite := range files[i].Sprites[groupIndex] {
				pixels[j] = sprite.Pixels
			}
			copy(group.Sprites, p.addSpritesDedup(edit, pixels))
		}
		object.ID = targets[i]
		for id := p.things.MaxID(category) + 1; id < object.ID; id++ {
			edit.setThing(category, id, thing.New(id, category))
		}
		keepExtra(object, p.things.Get(category, object.ID))
		edit.setThing(category, object.ID, object)
	}
	edit.commit()
	return targets, nil
}

// ExportOBD builds OBD data for a thing.
func (p *Project) ExportOBD(category thing.Category, id uint32, version int) (*obd.Data, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	object := p.things.Get(category, id)
	if object == nil {
		return nil, fmt.Errorf("%s %d does not exist", category, id)
	}
	obdData := &obd.Data{Version: version, ClientVersion: p.version.Value, Thing: object.Clone(), SpriteSize: p.features.SpriteSize}
	for _, group := range object.FrameGroups {
		sprites := make([]obd.Sprite, len(group.Sprites))
		for i, spriteID := range group.Sprites {
			var pixels []byte
			var err error
			if spriteID == 0 {
				pixels = make([]byte, p.features.SpriteSize*p.features.SpriteSize*4)
			} else if pixels, err = p.sprites.pixels(spriteID); err != nil {
				return nil, err
			}
			sprites[i] = obd.Sprite{ID: spriteID, Pixels: pixels}
		}
		obdData.Sprites = append(obdData.Sprites, sprites)
	}
	return obdData, nil
}
