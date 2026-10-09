package project

import (
	"errors"
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ErrShiftCut is returned by ShiftPixels when visible pixels would be moved
// out of a texture.
var ErrShiftCut = errors.New("visible pixels would be moved out of the texture")

// SetSlotPixels replaces the pixels of sprite slots of one frame group, for
// example after painting. A sprite can be shared by other slots and things,
// so it is never edited in place: changed slots get new sprites (identical
// ones are added once), fully transparent pixels clear the slot and
// unchanged pixels keep their sprite. It is one undoable edit.
func (p *Project) SetSlotPixels(c thing.Category, id uint32, group int, slots []int, pixels [][]byte, label string) error {
	if len(slots) != len(pixels) {
		return fmt.Errorf("%d slots for %d sprites", len(slots), len(pixels))
	}
	for _, px := range pixels {
		if err := p.checkPixels(px); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, g, err := p.groupForEdit(c, id, group)
	if err != nil {
		return err
	}
	for _, s := range slots {
		if s < 0 || s >= len(g.Sprites) {
			return fmt.Errorf("%s %d has no sprite slot %d", c, id, s)
		}
	}
	r := p.record(fmt.Sprintf("%s %s %d", label, c, id))
	if p.setSlotsLocked(r, g, slots, pixels) {
		r.setThing(c, id, t)
	}
	r.commit()
	return nil
}

// groupForEdit returns a copy of a thing and its frame group to change.
func (p *Project) groupForEdit(c thing.Category, id uint32, group int) (*thing.Thing, *thing.FrameGroup, error) {
	cur := p.things.Get(c, id)
	if cur == nil {
		return nil, nil, fmt.Errorf("%s %d does not exist", c, id)
	}
	if group < 0 || group >= len(cur.FrameGroups) {
		return nil, nil, fmt.Errorf("%s %d has no frame group %d", c, id, group)
	}
	t := cur.Clone()
	return t, t.FrameGroups[group], nil
}

// setSlotsLocked stores pixels in slots of g (see SetSlotPixels) and
// reports whether any slot changed.
func (p *Project) setSlotsLocked(r *recorder, g *thing.FrameGroup, slots []int, pixels [][]byte) bool {
	changed := false
	added := map[string]uint32{}
	for i, s := range slots {
		c := spr.Compress(pixels[i], p.features.SpriteSize, p.features.Transparency)
		cur, _ := p.sprites.compressedOrEmpty(g.Sprites[s])
		if string(cur) == string(c) {
			continue
		}
		changed = true
		if len(c) == 0 {
			g.Sprites[s] = 0
			continue
		}
		nid, ok := added[string(c)]
		if !ok {
			nid = p.sprites.count() + 1
			r.setSpriteCount(nid)
			r.setSprite(nid, c)
			added[string(c)] = nid
		}
		g.Sprites[s] = nid
	}
	return changed
}

// ShiftPixels moves the pixels of textures of a frame group by dx, dy
// (positive: right and down). With all every texture is moved, otherwise
// the texture at pos with all its layers (the outfit color mask moves
// along). It fails with ErrShiftCut when visible pixels would leave a
// texture, so nothing is ever lost.
func (p *Project) ShiftPixels(c thing.Category, id uint32, group int, dx, dy int, all bool, pos TexturePos) error {
	if dx == 0 && dy == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, g, err := p.groupForEdit(c, id, group)
	if err != nil {
		return err
	}
	size := p.features.SpriteSize
	var slots []int
	var pixels [][]byte
	for f := range int(g.Frames) {
		for z := range int(g.PatternZ) {
			for y := range int(g.PatternY) {
				for x := range int(g.PatternX) {
					if !all && (f != pos.Frame || z != pos.PatternZ || y != pos.PatternY || x != pos.PatternX) {
						continue
					}
					for l := range int(g.Layers) {
						img, err := p.textureImage(g, l, x, y, z, f)
						if err != nil {
							return err
						}
						moved, ok := shiftImage(img, dx, dy)
						if !ok {
							return fmt.Errorf("%w (frame %d, pattern %d,%d,%d, layer %d)", ErrShiftCut, f+1, x, y, z, l)
						}
						for h := range int(g.Height) {
							for w := range int(g.Width) {
								slots = append(slots, g.SpriteIndex(w, h, l, x, y, z, f))
								pixels = append(pixels, imaging.Tile(moved, (int(g.Width)-1-w)*size, (int(g.Height)-1-h)*size, size))
							}
						}
					}
				}
			}
		}
	}
	r := p.record(fmt.Sprintf("Shift pixels of %s %d", c, id))
	if p.setSlotsLocked(r, g, slots, pixels) {
		r.setThing(c, id, t)
	}
	r.commit()
	return nil
}

// textureImage composes one texture (one layer) of a frame group.
func (p *Project) textureImage(g *thing.FrameGroup, layer, x, y, z, frame int) (*image.NRGBA, error) {
	size := p.features.SpriteSize
	img := image.NewNRGBA(image.Rect(0, 0, int(g.Width)*size, int(g.Height)*size))
	for h := range int(g.Height) {
		for w := range int(g.Width) {
			sid := g.Sprites[g.SpriteIndex(w, h, layer, x, y, z, frame)]
			if sid == 0 {
				continue
			}
			px, err := p.sprites.pixels(sid)
			if err != nil {
				return nil, err
			}
			imaging.PutTile(img, (int(g.Width)-1-w)*size, (int(g.Height)-1-h)*size, size, px)
		}
	}
	return img, nil
}

// shiftImage moves the pixels of img by dx, dy. ok is false when a visible
// pixel would end up outside the image.
func shiftImage(img *image.NRGBA, dx, dy int) (*image.NRGBA, bool) {
	b := img.Rect
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			o := img.PixOffset(x, y)
			if img.Pix[o+3] == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx < b.Min.X || ny < b.Min.Y || nx >= b.Max.X || ny >= b.Max.Y {
				return nil, false
			}
			copy(out.Pix[out.PixOffset(nx, ny):], img.Pix[o:o+4])
		}
	}
	return out, true
}
