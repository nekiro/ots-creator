package project

import (
	"fmt"
	"image"
	"slices"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Thing returns a copy of a thing.
func (p *Project) Thing(c thing.Category, id uint32) (*thing.Thing, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	t := p.things.Get(c, id)
	if t == nil {
		return nil, fmt.Errorf("%s %d does not exist", c, id)
	}
	return t.Clone(), nil
}

// ThingRange returns copies of things with ids in [from, to].
func (p *Project) ThingRange(c thing.Category, from, to uint32) []*thing.Thing {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*thing.Thing
	for id := max(from, c.MinID()); id <= to; id++ {
		t := p.things.Get(c, id)
		if t == nil {
			break
		}
		out = append(out, t.Clone())
	}
	return out
}

// UpdateThing replaces an existing thing with t (matched by category and id).
func (p *Project) UpdateThing(t *thing.Thing) error {
	if err := t.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cur := p.things.Get(t.Category, t.ID)
	if cur == nil {
		return fmt.Errorf("%s %d does not exist", t.Category, t.ID)
	}
	if err := p.checkSpriteIDs(t); err != nil {
		return err
	}
	r := p.record(fmt.Sprintf("Edit %s %d", t.Category, t.ID))
	r.setThing(t.Category, t.ID, keepExtra(t.Clone(), cur))
	r.commit()
	return nil
}

// keepExtra gives an edited copy of a thing (that came from the UI or a
// file) the format data of the thing it replaces.
func keepExtra(t, cur *thing.Thing) *thing.Thing {
	if cur != nil {
		t.Extra = cur.Extra
	}
	return t
}

func (p *Project) checkSpriteIDs(t *thing.Thing) error {
	n := p.sprites.count()
	for _, id := range t.SpriteIDs() {
		if id > n {
			return fmt.Errorf("sprite %d does not exist (max %d)", id, n)
		}
	}
	return nil
}

// AddThing appends a new empty thing and returns its id.
func (p *Project) AddThing(c thing.Category) (uint32, error) {
	if !c.Valid() {
		return 0, fmt.Errorf("invalid category %d", c)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.things.MaxID(c) + 1
	r := p.record(fmt.Sprintf("New %s", c))
	r.setThing(c, id, thing.New(id, c))
	r.commit()
	return id, nil
}

// DuplicateThings appends copies of the given things and returns new ids.
func (p *Project) DuplicateThings(c thing.Category, ids []uint32) ([]uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.record(fmt.Sprintf("Duplicate %d %s(s)", len(ids), c))
	var out []uint32
	for _, id := range ids {
		src := p.things.Get(c, id)
		if src == nil {
			return nil, fmt.Errorf("%s %d does not exist", c, id)
		}
		n := src.Clone()
		n.ID = p.things.MaxID(c) + 1
		r.setThing(c, n.ID, n)
		out = append(out, n.ID)
	}
	r.commit()
	return out, nil
}

// RemoveThings removes things. The last thing of a category is deleted;
// others are replaced with empty things so ids stay stable. The first id of
// each category always remains.
func (p *Project) RemoveThings(c thing.Category, ids []uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	slices.Reverse(sorted)
	r := p.record(fmt.Sprintf("Remove %d %s(s)", len(ids), c))
	for _, id := range sorted {
		if p.things.Get(c, id) == nil {
			return fmt.Errorf("%s %d does not exist", c, id)
		}
		if id == p.things.MaxID(c) && id != c.MinID() {
			r.setThing(c, id, nil)
		} else {
			r.setThing(c, id, thing.New(id, c))
		}
	}
	r.commit()
	return nil
}

// SpritePixels returns RGBA pixels of a sprite.
func (p *Project) SpritePixels(id uint32) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sprites.pixels(id)
}

// IsSpriteEmpty reports whether a sprite has no colored pixels.
func (p *Project) IsSpriteEmpty(id uint32) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, err := p.sprites.compressed(id)
	return len(c) == 0, err
}

func (p *Project) checkPixels(px []byte) error {
	want := p.features.SpriteSize * p.features.SpriteSize * 4
	if len(px) != want {
		return fmt.Errorf("sprite pixels have %d bytes, want %d", len(px), want)
	}
	return nil
}

// AddSprites appends sprites and returns their ids.
func (p *Project) AddSprites(pixels [][]byte) ([]uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, px := range pixels {
		if err := p.checkPixels(px); err != nil {
			return nil, err
		}
	}
	r := p.record(fmt.Sprintf("Add %d sprite(s)", len(pixels)))
	ids := p.addSpritesLocked(r, pixels)
	r.commit()
	return ids, nil
}

func (p *Project) addSpritesLocked(r *recorder, pixels [][]byte) []uint32 {
	ids := make([]uint32, len(pixels))
	for i, px := range pixels {
		id := p.sprites.count() + 1
		r.setSpriteCount(id)
		r.setSprite(id, spr.Compress(px, p.features.SpriteSize, p.features.Transparency))
		ids[i] = id
	}
	return ids
}

// ReplaceSprite overwrites one sprite.
func (p *Project) ReplaceSprite(id uint32, px []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkPixels(px); err != nil {
		return err
	}
	if id == 0 || id > p.sprites.count() {
		return fmt.Errorf("sprite %d does not exist", id)
	}
	r := p.record(fmt.Sprintf("Replace sprite %d", id))
	r.setSprite(id, spr.Compress(px, p.features.SpriteSize, p.features.Transparency))
	r.commit()
	return nil
}

// RemoveSprites clears sprites. The last sprite is deleted, others become
// empty so references stay valid.
func (p *Project) RemoveSprites(ids []uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	slices.Reverse(sorted)
	r := p.record(fmt.Sprintf("Remove %d sprite(s)", len(ids)))
	for _, id := range sorted {
		if id == 0 || id > p.sprites.count() {
			return fmt.Errorf("sprite %d does not exist", id)
		}
		r.setSprite(id, nil)
		if id == p.sprites.count() {
			r.setSpriteCount(id - 1)
		}
	}
	r.commit()
	return nil
}

// ImportOBD adds the sprites of an OBD file and appends its thing to the
// matching category. When replaceID is not zero, that thing is replaced
// instead. It returns the id of the imported thing.
func (p *Project) ImportOBD(d *obd.Data, replaceID uint32) (uint32, error) {
	if d.SpriteSize != p.features.SpriteSize {
		return 0, fmt.Errorf("object uses %dpx sprites, client uses %dpx", d.SpriteSize, p.features.SpriteSize)
	}
	t := d.Thing.Clone()
	t.Extra = nil
	if err := t.Validate(); err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	c := t.Category
	if replaceID != 0 && p.things.Get(c, replaceID) == nil {
		return 0, fmt.Errorf("%s %d does not exist", c, replaceID)
	}
	r := p.record(fmt.Sprintf("Import %s", c))
	for gi, g := range t.FrameGroups {
		pixels := make([][]byte, len(d.Sprites[gi]))
		for i, s := range d.Sprites[gi] {
			pixels[i] = s.Pixels
		}
		ids := p.addSpritesDedup(r, pixels)
		copy(g.Sprites, ids)
	}
	if replaceID != 0 {
		t.ID = replaceID
		keepExtra(t, p.things.Get(c, replaceID))
	} else {
		t.ID = p.things.MaxID(c) + 1
	}
	r.setThing(c, t.ID, t)
	r.commit()
	return t.ID, nil
}

// SetGroupPixels replaces every sprite of one frame group with new pixels
// (indexed by sprite slot), for example from an imported sprite sheet.
func (p *Project) SetGroupPixels(c thing.Category, id uint32, group int, pixels [][]byte) error {
	for _, px := range pixels {
		if err := p.checkPixels(px); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cur := p.things.Get(c, id)
	if cur == nil {
		return fmt.Errorf("%s %d does not exist", c, id)
	}
	if group < 0 || group >= len(cur.FrameGroups) {
		return fmt.Errorf("%s %d has no frame group %d", c, id, group)
	}
	t := cur.Clone()
	g := t.FrameGroups[group]
	if len(pixels) != len(g.Sprites) {
		return fmt.Errorf("got %d sprites, frame group has %d slots", len(pixels), len(g.Sprites))
	}
	r := p.record(fmt.Sprintf("Import sheet into %s %d", c, id))
	copy(g.Sprites, p.addSpritesDedup(r, pixels))
	r.setThing(c, id, t)
	r.commit()
	return nil
}

// addSpritesDedup adds sprites, mapping fully transparent ones to id 0 and
// reusing identical sprites within the same import.
func (p *Project) addSpritesDedup(r *recorder, pixels [][]byte) []uint32 {
	ids := make([]uint32, len(pixels))
	seen := map[string]uint32{}
	for i, px := range pixels {
		c := spr.Compress(px, p.features.SpriteSize, p.features.Transparency)
		if len(c) == 0 {
			continue
		}
		if id, ok := seen[string(c)]; ok {
			ids[i] = id
			continue
		}
		id := p.sprites.count() + 1
		r.setSpriteCount(id)
		r.setSprite(id, c)
		seen[string(c)] = id
		ids[i] = id
	}
	return ids
}

// ExportOBD builds OBD data for a thing.
func (p *Project) ExportOBD(c thing.Category, id uint32, version int) (*obd.Data, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	t := p.things.Get(c, id)
	if t == nil {
		return nil, fmt.Errorf("%s %d does not exist", c, id)
	}
	d := &obd.Data{Version: version, ClientVersion: p.version.Value, Thing: t.Clone(), SpriteSize: p.features.SpriteSize}
	for _, g := range t.FrameGroups {
		sprites := make([]obd.Sprite, len(g.Sprites))
		for i, sid := range g.Sprites {
			var px []byte
			var err error
			if sid == 0 {
				px = make([]byte, p.features.SpriteSize*p.features.SpriteSize*4)
			} else if px, err = p.sprites.pixels(sid); err != nil {
				return nil, err
			}
			sprites[i] = obd.Sprite{ID: sid, Pixels: px}
		}
		d.Sprites = append(d.Sprites, sprites)
	}
	return d, nil
}

// ImportSheet puts a sprite sheet into a thing and picks the layout from
// the sheet (see imaging.InferLayout). Outfit sheets hold every frame; with
// frame groups the first frame becomes the idle group and the others the
// walking group, unless the thing already splits its frames another way.
func (p *Project) ImportSheet(c thing.Category, id uint32, img *image.NRGBA) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	cur := p.things.Get(c, id)
	if cur == nil {
		return fmt.Errorf("%s %d does not exist", c, id)
	}
	size := p.features.SpriteSize
	merged, err := imaging.InferLayout(img, c, cur, size)
	if err != nil {
		return err
	}
	pixels, err := imaging.SliceSheet(img, merged, size)
	if err != nil {
		return err
	}
	r := p.record(fmt.Sprintf("Import sheet into %s %d", c, id))
	copy(merged.Sprites, p.addSpritesDedup(r, pixels))
	t := cur.Clone()
	t.FrameGroups = splitSheetGroups(merged, cur, c == thing.CategoryOutfit && p.features.FrameGroups)
	r.setThing(c, id, t)
	r.commit()
	return nil
}

func splitSheetGroups(merged *thing.FrameGroup, cur *thing.Thing, groups bool) []*thing.FrameGroup {
	def := cur.Category.DefaultDuration()
	// Animation settings and durations of the replaced group, when they fit.
	adopt := func(g *thing.FrameGroup, i int) *thing.FrameGroup {
		if i < len(cur.FrameGroups) {
			old := cur.FrameGroups[i]
			g.Mode, g.LoopCount = old.Mode, old.LoopCount
			if old.Frames == g.Frames {
				g.Durations = slices.Clone(old.Durations)
				g.StartFrame = old.StartFrame
			}
		}
		g.EnsureDurations(def)
		return g
	}
	if !groups || merged.Frames < 2 {
		g := merged.Clone()
		g.Type = thing.FrameGroupDefault
		return []*thing.FrameGroup{adopt(g, 0)}
	}
	idleFrames := 1
	if len(cur.FrameGroups) == 2 && int(cur.FrameGroups[0].Frames)+int(cur.FrameGroups[1].Frames) == int(merged.Frames) {
		idleFrames = int(cur.FrameGroups[0].Frames)
	}
	idle := merged.Clone()
	idle.Type, idle.Frames = thing.FrameGroupDefault, uint8(idleFrames)
	idle.Sprites = frameBlock(merged, 0, idleFrames)
	walk := merged.Clone()
	walk.Type, walk.Frames = thing.FrameGroupWalking, merged.Frames-uint8(idleFrames)
	walk.Sprites = frameBlock(merged, idleFrames, int(merged.Frames))
	return []*thing.FrameGroup{adopt(idle, 0), adopt(walk, 1)}
}
