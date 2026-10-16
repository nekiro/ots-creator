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
func ThumbnailPos(object *thing.Thing) TexturePos {
	pos := TexturePos{}
	if object.Category == thing.CategoryOutfit {
		if group := object.Group(thing.FrameGroupDefault); group != nil && int(group.PatternX) > int(thing.South) {
			pos.PatternX = int(thing.South)
		}
	}
	return pos
}

// Render composes one texture of a thing into an image of
// width*size x height*size pixels. Tile (0,0) is the bottom-right one, as
// in the client.
func (p *Project) Render(category thing.Category, id uint32, pos TexturePos) (*image.NRGBA, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	object := p.things.Get(category, id)
	if object == nil {
		return nil, fmt.Errorf("%s %d does not exist", category, id)
	}
	group := object.Group(pos.Group)
	if pos.Layer >= int(group.Layers) || pos.PatternX >= int(group.PatternX) || pos.PatternY >= int(group.PatternY) || pos.PatternZ >= int(group.PatternZ) {
		return nil, fmt.Errorf("texture position %+v out of range", pos)
	}
	size := p.features.SpriteSize
	img := image.NewNRGBA(image.Rect(0, 0, int(group.Width)*size, int(group.Height)*size))
	buf := make([]byte, size*size*4)
	for tileY := 0; tileY < int(group.Height); tileY++ {
		for tileX := 0; tileX < int(group.Width); tileX++ {
			spriteID := group.Sprites[group.SpriteIndex(tileX, tileY, pos.Layer, pos.PatternX, pos.PatternY, pos.PatternZ, pos.Frame)]
			if spriteID == 0 {
				continue
			}
			compressed, err := p.sprites.compressed(spriteID)
			if err != nil || len(compressed) == 0 {
				continue // missing sprites render as empty
			}
			if err := spr.DecompressInto(buf, compressed, size, p.features.Transparency); err != nil {
				continue
			}
			originX := (int(group.Width) - 1 - tileX) * size
			originY := (int(group.Height) - 1 - tileY) * size
			for y := 0; y < size; y++ {
				copy(img.Pix[(originY+y)*img.Stride+originX*4:(originY+y)*img.Stride+(originX+size)*4], buf[y*size*4:(y+1)*size*4])
			}
		}
	}
	return img, nil
}
