package project

import (
	"fmt"
	"image"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/spr"
)

type TexturePos struct {
	Group    thing.FrameGroupType
	Layer    int
	PatternX int
	PatternY int
	PatternZ int
	Frame    int
	// Addons are extra pattern Y values drawn over the texture (outfit addons).
	Addons []int
	// AllLayers draws every layer over the first one, as the client does
	// with items (outfit layers are color masks instead).
	AllLayers bool
}

// ThumbnailPos returns the texture shown in lists: outfits face south
// with all their addons, items show all their layers, everything else
// uses the first texture.
func ThumbnailPos(object *thing.Thing) TexturePos {
	pos := TexturePos{AllLayers: object.Category == thing.CategoryItem}
	if object.Category == thing.CategoryOutfit {
		if group := object.Group(thing.FrameGroupDefault); group != nil {
			if int(group.PatternX) > int(thing.South) {
				pos.PatternX = int(thing.South)
			}
			for addon := 1; addon <= 2 && addon < int(group.PatternY); addon++ {
				pos.Addons = append(pos.Addons, addon)
			}
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
	patternYs := []int{pos.PatternY}
	for _, addon := range pos.Addons {
		if addon != pos.PatternY && addon >= 0 && addon < int(group.PatternY) {
			patternYs = append(patternYs, addon)
		}
	}
	layers := []int{pos.Layer}
	if pos.AllLayers {
		layers = layers[:0]
		for layer := range int(group.Layers) {
			layers = append(layers, layer)
		}
	}
	type texturePass struct{ layer, patternY int }
	var passes []texturePass
	for _, patternY := range patternYs {
		for _, layer := range layers {
			passes = append(passes, texturePass{layer, patternY})
		}
	}
	for index, pass := range passes {
		for tileY := 0; tileY < int(group.Height); tileY++ {
			for tileX := 0; tileX < int(group.Width); tileX++ {
				spriteID := group.Sprites[group.SpriteIndex(tileX, tileY, pass.layer, pos.PatternX, pass.patternY, pos.PatternZ, pos.Frame)]
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
				x, y := (int(group.Width)-1-tileX)*size, (int(group.Height)-1-tileY)*size
				if index == 0 {
					imaging.PutTile(img, x, y, size, buf)
				} else {
					imaging.BlendTile(img, x, y, size, buf)
				}
			}
		}
	}
	return img, nil
}
