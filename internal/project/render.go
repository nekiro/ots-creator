package project

import (
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// TexturePos selects one texture of a frame group.
type TexturePos struct {
	Group    thing.FrameGroupType
	Layer    int
	PatternX int
	PatternY int
	PatternZ int
	Frame    int
}

// ThumbnailPos returns the texture shown in lists: outfits face south,
// everything else uses the first texture.
func ThumbnailPos(t *thing.Thing) TexturePos {
	pos := TexturePos{}
	if t.Category == thing.CategoryOutfit {
		if g := t.Group(thing.FrameGroupDefault); g != nil && int(g.PatternX) > int(thing.South) {
			pos.PatternX = int(thing.South)
		}
	}
	return pos
}

// Render composes one texture of a thing into an image of
// width*size x height*size pixels. Tile (0,0) is the bottom-right one, as
// in the client.
func (p *Project) Render(c thing.Category, id uint32, pos TexturePos) (*image.NRGBA, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	t := p.things.Get(c, id)
	if t == nil {
		return nil, fmt.Errorf("%s %d does not exist", c, id)
	}
	g := t.Group(pos.Group)
	if pos.Layer >= int(g.Layers) || pos.PatternX >= int(g.PatternX) || pos.PatternY >= int(g.PatternY) || pos.PatternZ >= int(g.PatternZ) {
		return nil, fmt.Errorf("texture position %+v out of range", pos)
	}
	size := p.features.SpriteSize
	img := image.NewNRGBA(image.Rect(0, 0, int(g.Width)*size, int(g.Height)*size))
	buf := make([]byte, size*size*4)
	for h := 0; h < int(g.Height); h++ {
		for w := 0; w < int(g.Width); w++ {
			sid := g.Sprites[g.SpriteIndex(w, h, pos.Layer, pos.PatternX, pos.PatternY, pos.PatternZ, pos.Frame)]
			if sid == 0 {
				continue
			}
			comp, err := p.sprites.compressed(sid)
			if err != nil || len(comp) == 0 {
				continue // missing sprites render as empty
			}
			if err := spr.DecompressInto(buf, comp, size, p.features.Transparency); err != nil {
				continue
			}
			ox := (int(g.Width) - 1 - w) * size
			oy := (int(g.Height) - 1 - h) * size
			for y := 0; y < size; y++ {
				copy(img.Pix[(oy+y)*img.Stride+ox*4:(oy+y)*img.Stride+(ox+size)*4], buf[y*size*4:(y+1)*size*4])
			}
		}
	}
	return img, nil
}
