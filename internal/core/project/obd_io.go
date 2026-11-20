package project

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
)

// ImportOBD adds the sprites of an OBD file and appends its thing to the
// matching category. When replaceID is not zero, that thing is replaced
// instead. An outfit gets the frame groups of the client (see
// fitFrameGroups). It returns the id of the imported thing.
func (p *Project) ImportOBD(obdData *obd.Data, replaceID uint32) (uint32, error) {
	object, pixels, err := p.objectFromFile(obdData)
	if err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	category := object.Category
	if replaceID != 0 && p.things.Get(category, replaceID) == nil {
		return 0, fmt.Errorf("%s %d does not exist", category, replaceID)
	}
	fitted, ok := p.fitFrameGroups(object)
	if !ok {
		return 0, errNoFrameGroups(category)
	}
	object = fitted
	edit := p.record(fmt.Sprintf("Import %s", category))
	p.addObjectSprites(edit, object, pixels)
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
// it, filling any gap with empty things. Outfits get the frame groups of
// the client (see fitFrameGroups). The whole import is one undoable edit.
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
	pixels := make([][][]byte, len(files))
	for i, obdData := range files {
		object, objectPixels, err := p.objectFromFile(obdData)
		if err != nil {
			return nil, err
		}
		if object.Category != category {
			return nil, fmt.Errorf("object is a %s, not a %s", object.Category, category)
		}
		things[i], pixels[i] = object, objectPixels
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, object := range things {
		fitted, ok := p.fitFrameGroups(object)
		if !ok {
			return nil, errNoFrameGroups(category)
		}
		things[i] = fitted
	}
	edit := p.record(fmt.Sprintf("Import %d %s(s)", len(things), category))
	for i, object := range things {
		p.addObjectSprites(edit, object, pixels[i])
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

// objectFromFile checks an OBD file and returns a copy of its thing whose
// sprite slots hold the numbers 1..n of the returned pixels, so its frame
// groups can be reshaped before the sprites are added (addObjectSprites).
func (p *Project) objectFromFile(obdData *obd.Data) (*thing.Thing, [][]byte, error) {
	if obdData.SpriteSize != p.features.SpriteSize {
		return nil, nil, fmt.Errorf("object uses %dpx sprites, client uses %dpx", obdData.SpriteSize, p.features.SpriteSize)
	}
	object := obdData.Thing.Clone()
	object.Extra = nil
	if err := object.Validate(); err != nil {
		return nil, nil, err
	}
	if len(obdData.Sprites) < len(object.FrameGroups) {
		return nil, nil, fmt.Errorf("pixels for %d frame groups, %s has %d", len(obdData.Sprites), object.Category, len(object.FrameGroups))
	}
	var pixels [][]byte
	for groupIndex, group := range object.FrameGroups {
		sprites := obdData.Sprites[groupIndex]
		for slot := range group.Sprites {
			group.Sprites[slot] = 0
			if slot < len(sprites) {
				pixels = append(pixels, sprites[slot].Pixels)
				group.Sprites[slot] = uint32(len(pixels))
			}
		}
	}
	return object, pixels, nil
}

// addObjectSprites adds the pixels the frame groups of object use and puts
// their sprite ids in place of the numbers objectFromFile gave. Pixels no
// slot uses any more (frames dropped by fitFrameGroups) are not added.
func (p *Project) addObjectSprites(edit *recorder, object *thing.Thing, pixels [][]byte) {
	var used [][]byte
	usedIndex := map[uint32]int{}
	for _, group := range object.FrameGroups {
		for _, number := range group.Sprites {
			if _, ok := usedIndex[number]; number != 0 && !ok {
				usedIndex[number] = len(used)
				used = append(used, pixels[number-1])
			}
		}
	}
	ids := p.addSpritesDedup(edit, used)
	for _, group := range object.FrameGroups {
		for slot, number := range group.Sprites {
			if number != 0 {
				group.Sprites[slot] = ids[usedIndex[number]]
			}
		}
	}
}

// fitFrameGroups gives an outfit the frame groups the client keeps, as
// Object Builder does: an outfit of an old client (one group: frame 0 is
// idle, the rest is walking) is split into idle and walking groups when the
// client has frame groups, and idle and walking groups are merged into one
// when it has none. ok is false when they cannot be merged because their
// layouts differ. Other things are returned as they are. Call it with p.mu
// held.
func (p *Project) fitFrameGroups(object *thing.Thing) (fitted *thing.Thing, ok bool) {
	if object.Category != thing.CategoryOutfit {
		return object, true
	}
	withGroups := p.features.FrameGroups || p.format == FormatAssets
	switch {
	case withGroups && needsGroupSplit(object):
		return splitGroups(object), true
	case !withGroups && len(object.FrameGroups) > 1:
		merged := mergeGroups(object)
		return merged, merged != nil
	}
	return object, true
}

func errNoFrameGroups(category thing.Category) error {
	return fmt.Errorf("%s: idle and walking layouts differ, the client has no frame groups", category)
}

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
