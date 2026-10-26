package maprender

import (
	"encoding/binary"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// What the light of the map is built from, as the client (OTClient) does:
// item lights at the center of the item's drawn pixels, tiles that hide
// the light of the floors under them (Tile::canShade) or only dim it (a
// translucent thing on them, Tile::hasTranslucentThing), and tiles with a
// ground, where the light of their floor shines fully (LightView::markGround).

// maxSeeThroughPixels is how many fully transparent pixels a sprite may
// have and still count as opaque (client: Image::loadPNG, sprite sheets).
const maxSeeThroughPixels = 4

// Thing opacity, computed when first needed.
const (
	opacityUnknown = iota
	opacityOpaque
	opacitySeeThrough
)

// lightOrigin is where a thing's light comes from: the center of its drawn
// pixels, from the top-left of its bottom-right tile (before its offset).
type lightOrigin struct{ x, y int }

// light appends the light source of a thing drawn with pattern at, 8
// bytes: int16 x, y (world pixels from the chunk's corner), uint8 level,
// uint8 color (8-bit client color) and two zero bytes (the next chunk
// stays 4-byte aligned). (anchorX, anchorY) is the top-left of its
// bottom-right tile, raised by the elevation under it.
func (b *geometryBuilder) light(look *thingLook, at pattern, anchorX, anchorY int) {
	origin := b.lightOrigin(look, at)
	x := anchorX - look.offsetX + origin.x
	y := anchorY - look.offsetY + origin.y
	b.lights = binary.LittleEndian.AppendUint16(b.lights, uint16(int16(x)))
	b.lights = binary.LittleEndian.AppendUint16(b.lights, uint16(int16(y)))
	b.lights = append(b.lights, look.lightLevel, look.lightColor, 0, 0)
	b.lightCount++
}

// lightOrigin is the center of the pixels of frame 0, layer 0 of a thing
// in pattern at that are not fully transparent (client: the center of
// ThingType::getScreenRect). Without sprite pixels it is the center of the
// whole texture.
func (b *geometryBuilder) lightOrigin(look *thingLook, at pattern) lightOrigin {
	group := look.frameGroup
	spriteSize := b.scene.SpriteSize
	key := (at.z*int(group.PatternY)+at.y)*int(group.PatternX) + at.x
	if origin, ok := look.lightOrigins[key]; ok {
		return origin
	}
	width, height := int(group.Width), int(group.Height)
	// The texture's top-left from the top-left of its bottom-right tile.
	left, top := -(width-1)*spriteSize, -(height-1)*spriteSize
	origin := lightOrigin{left + width*spriteSize/2, top + height*spriteSize/2}
	minX, minY, maxX, maxY := width*spriteSize, height*spriteSize, -1, -1
	for row := 0; row < height && b.scene.SpritePixels != nil; row++ {
		for column := 0; column < width; column++ {
			index := group.SpriteIndex(column, row, 0, at.x, at.y, at.z, 0)
			if index >= len(group.Sprites) || group.Sprites[index] == 0 {
				continue
			}
			pixels, err := b.scene.SpritePixels(group.Sprites[index])
			if err != nil || len(pixels) < spriteSize*spriteSize*4 {
				continue
			}
			// Sprite (column, row) is drawn column tiles left and row tiles up of the bottom-right tile.
			spriteX, spriteY := (width-1-column)*spriteSize, (height-1-row)*spriteSize
			for pixelY := 0; pixelY < spriteSize; pixelY++ {
				for pixelX := 0; pixelX < spriteSize; pixelX++ {
					if pixels[(pixelY*spriteSize+pixelX)*4+3] == 0 {
						continue
					}
					minX, maxX = min(minX, spriteX+pixelX), max(maxX, spriteX+pixelX)
					minY, maxY = min(minY, spriteY+pixelY), max(maxY, spriteY+pixelY)
				}
			}
		}
	}
	if maxX >= 0 {
		// Rect::center of the inclusive rectangle, in whole pixels.
		origin = lightOrigin{floorHalf(2*left + minX + maxX), floorHalf(2*top + minY + maxY)}
	}
	if look.lightOrigins == nil {
		look.lightOrigins = map[int]lightOrigin{}
	}
	look.lightOrigins[key] = origin
	return origin
}

// floorHalf halves rounding down, also below zero (the client's screen
// positions are never negative, chunk positions can be).
func floorHalf(value int) int { return value >> 1 }

// canShade reports whether a tile hides the light of the floors under it
// (client: Tile::canShade): a full ground, an opaque thing, or a ground or
// ground border two tiles a side.
func (b *geometryBuilder) canShade(tile *otmap.Tile) bool {
	return b.isFullyOpaque(tile) || b.hasTopGround(tile)
}

// isFullyOpaque reports whether a tile has a full ground or an opaque
// thing (client: Tile::isFullyOpaque).
func (b *geometryBuilder) isFullyOpaque(tile *otmap.Tile) bool {
	if tile.Ground.ID != 0 {
		if look := b.itemLook(tile.Ground); look != nil && b.isOpaqueOrFull(look) {
			return true
		}
	}
	for _, item := range tile.Items {
		if look := b.itemLook(item); look != nil && b.isOpaqueOrFull(look) {
			return true
		}
	}
	return false
}

// hasTopGround reports whether a tile has a ground or a ground border two
// tiles a side (client: Tile::hasTopGround).
func (b *geometryBuilder) hasTopGround(tile *otmap.Tile) bool {
	if tile.Ground.ID != 0 {
		if look := b.itemLook(tile.Ground); look != nil && look.ground && look.isTopSized() {
			return true
		}
	}
	for _, item := range tile.Items {
		if look := b.itemLook(item); look != nil && look.groundBorder && look.isTopSized() {
			return true
		}
	}
	return false
}

// itemLook is the look of a map item (nil when it has none).
func (b *geometryBuilder) itemLook(item otmap.Item) *thingLook {
	itemType := b.scene.Items.Get(item.ID)
	if itemType == nil || itemType.Flags.Has(otitems.FlagMeta) {
		return nil
	}
	return b.lookOf(thing.CategoryItem, uint32(itemType.ClientID))
}

// isTopSized: two tiles a side (client: Size::dimension() == 4).
func (look *thingLook) isTopSized() bool {
	return int(look.frameGroup.Width)+int(look.frameGroup.Height) == 4
}

func (b *geometryBuilder) isOpaqueOrFull(look *thingLook) bool {
	return look.fullGround || b.isOpaque(look)
}

// isOpaque reports whether a thing covers what is under it (client:
// ThingType::isOpaque): the sprite of its bottom-right tile, frame 0, is
// opaque in every pattern and layer.
func (b *geometryBuilder) isOpaque(look *thingLook) bool {
	if look.opacity != opacityUnknown {
		return look.opacity == opacityOpaque
	}
	look.opacity = opacityOpaque
	group := look.frameGroup
	spriteSize := b.scene.SpriteSize
	for patternZ := 0; patternZ < int(group.PatternZ) && look.opacity == opacityOpaque; patternZ++ {
		for patternY := 0; patternY < int(group.PatternY) && look.opacity == opacityOpaque; patternY++ {
			for patternX := 0; patternX < int(group.PatternX) && look.opacity == opacityOpaque; patternX++ {
				for layer := 0; layer < int(group.Layers); layer++ {
					index := group.SpriteIndex(0, 0, layer, patternX, patternY, patternZ, 0)
					if index >= len(group.Sprites) || !b.isSpriteOpaque(group.Sprites[index], spriteSize) {
						look.opacity = opacitySeeThrough
						break
					}
				}
			}
		}
	}
	return look.opacity == opacityOpaque
}

func (b *geometryBuilder) isSpriteOpaque(id uint32, spriteSize int) bool {
	if id == 0 || b.scene.SpritePixels == nil {
		return false
	}
	pixels, err := b.scene.SpritePixels(id)
	if err != nil || len(pixels) < spriteSize*spriteSize*4 {
		return false
	}
	seeThrough := 0
	for alpha := 3; alpha < len(pixels); alpha += 4 {
		if pixels[alpha] == 0 {
			seeThrough++
			if seeThrough > maxSeeThroughPixels {
				return false
			}
		}
	}
	return true
}

// The bit planes of the shades of a chunk, one bit per tile each (see
// geometryBuilder.shades).
const (
	shadePlane = iota
	translucentPlane
	groundPlane
	shadePlanes
)

// shade sets the bit of the tile being drawn in a plane of the shades.
func (b *geometryBuilder) shade(plane int) {
	size := max(b.options.Block, 1) << otmap.ChunkBits
	bit := plane*size*size + int(b.tileY)*size + int(b.tileX)
	b.shades[bit>>3] |= 1 << (bit & 7)
	b.hasShades = true
}

// hasGround reports whether a tile's first thing is a ground (client:
// Tile::getGround).
func (b *geometryBuilder) hasGround(tile *otmap.Tile) bool {
	if tile.Ground.ID == 0 {
		return false
	}
	look := b.itemLook(tile.Ground)
	return look != nil && look.ground
}

// hasTranslucentThing reports whether a tile has a translucent ground or
// item (client: Tile::hasTranslucentThing).
func (b *geometryBuilder) hasTranslucentThing(tile *otmap.Tile) bool {
	if tile.Ground.ID != 0 {
		if look := b.itemLook(tile.Ground); look != nil && look.translucent {
			return true
		}
	}
	for _, item := range tile.Items {
		if look := b.itemLook(item); look != nil && look.translucent {
			return true
		}
	}
	return false
}
