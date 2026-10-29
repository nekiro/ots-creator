package maprender

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// MaxImageTiles limits the tiles of one RenderImage.
const MaxImageTiles = 128 * 128

// imageReach is how many tiles right and below the area are drawn too:
// their sprites reach up and left into it (big sprites and elevation).
const imageReach = 2

// Sizes of the sections of a chunk's geometry (see geometryBuilder.chunk).
const (
	geometryHeaderSize = 24
	quadSize           = 16
	animatedSize       = 12
	composedSize       = 16
)

// imageQuad is one quad of the geometry in image pixels.
type imageQuad struct {
	x, y   int
	sprite uint32
	color  color.NRGBA
	// tileX and tileY are the map position of the tile it belongs to.
	tileX, tileY int
}

// RenderImage draws the tiles of one floor in area (tile coordinates, Max
// excluded) as the editor shows them without view overlays: one sprite
// size per tile, on black. Outfits are drawn without their colors.
func RenderImage(scene *Scene, area image.Rectangle, floor int, options GeometryOptions) (*image.NRGBA, error) {
	if area.Empty() || area.Dx()*area.Dy() > MaxImageTiles {
		return nil, errors.New("the area is empty or too big")
	}
	if floor < 0 || floor > otmap.MaxFloor {
		return nil, errors.New("the floor is out of range")
	}
	spriteSize := scene.SpriteSize
	options.Block = 1
	var keys []otmap.ChunkKey
	for chunkY := area.Min.Y >> otmap.ChunkBits; chunkY <= (area.Max.Y-1+imageReach)>>otmap.ChunkBits; chunkY++ {
		for chunkX := area.Min.X >> otmap.ChunkBits; chunkX <= (area.Max.X-1+imageReach)>>otmap.ChunkBits; chunkX++ {
			if chunkX <= 0xFFFF>>otmap.ChunkBits && chunkY <= 0xFFFF>>otmap.ChunkBits {
				keys = append(keys, otmap.MakeKey(uint16(chunkX), uint16(chunkY), uint8(floor)))
			}
		}
	}
	parts, _, err := BuildGeometryParts(scene, keys, options, false)
	if err != nil {
		return nil, err
	}
	var quads []imageQuad
	for _, part := range parts {
		quads = appendImageQuads(quads, part, area, spriteSize)
	}
	// The client draws tile by tile, diagonal by diagonal from the top-left
	// corner, each diagonal from its bottom-left end; a stable sort keeps
	// the order of the quads of one tile.
	slices.SortStableFunc(quads, func(first, second imageQuad) int {
		if diagonal := (first.tileX + first.tileY) - (second.tileX + second.tileY); diagonal != 0 {
			return diagonal
		}
		return first.tileX - second.tileX
	})
	img := image.NewNRGBA(image.Rect(0, 0, area.Dx()*spriteSize, area.Dy()*spriteSize))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xFF
	}
	sprites := map[uint32][]byte{}
	for _, quad := range quads {
		switch {
		case quad.sprite == spriteHidden:
		case quad.sprite&spriteRect != 0 && quad.sprite&spriteComposed == 0:
			width, height := int(quad.sprite>>rectSizeBits&rectSizeMax), int(quad.sprite&rectSizeMax)
			fillRect(img, image.Rect(quad.x, quad.y, quad.x+width, quad.y+height), quad.color)
		default:
			pixels, ok := sprites[quad.sprite]
			if !ok && scene.SpritePixels != nil {
				pixels, _ = scene.SpritePixels(quad.sprite)
				sprites[quad.sprite] = pixels
			}
			if len(pixels) == spriteSize*spriteSize*4 {
				drawSprite(img, pixels, spriteSize, quad.x, quad.y, quad.color)
			}
		}
	}
	return img, nil
}

// appendImageQuads reads the quads of one chunk's geometry, moved to image
// pixels of area. Composed outfit pieces become their plain sprite.
func appendImageQuads(quads []imageQuad, part []byte, area image.Rectangle, spriteSize int) []imageQuad {
	if len(part) < geometryHeaderSize {
		return quads
	}
	key := otmap.ChunkKey(binary.LittleEndian.Uint32(part))
	quadCount := int(binary.LittleEndian.Uint32(part[4:]))
	animatedCount := int(binary.LittleEndian.Uint32(part[8:]))
	composedCount := int(binary.LittleEndian.Uint32(part[12:]))
	composedStart := geometryHeaderSize + quadCount*quadSize + animatedCount*animatedSize
	if len(part) < composedStart+composedCount*composedSize {
		return quads
	}
	chunkX, chunkY, _ := key.Split()
	originX, originY := int(chunkX)<<otmap.ChunkBits, int(chunkY)<<otmap.ChunkBits
	offsetX, offsetY := (originX-area.Min.X)*spriteSize, (originY-area.Min.Y)*spriteSize
	for index := range quadCount {
		entry := part[geometryHeaderSize+index*quadSize:]
		quad := imageQuad{
			x:      int(int16(binary.LittleEndian.Uint16(entry))) + offsetX,
			y:      int(int16(binary.LittleEndian.Uint16(entry[2:]))) + offsetY,
			sprite: binary.LittleEndian.Uint32(entry[4:]),
			color:  color.NRGBA{entry[8], entry[9], entry[10], entry[11]},
			tileX:  originX + int(binary.LittleEndian.Uint16(entry[12:])),
			tileY:  originY + int(binary.LittleEndian.Uint16(entry[14:])&^tileTop),
		}
		if quad.sprite != spriteHidden && quad.sprite&spriteComposed != 0 {
			composed := int(quad.sprite &^ spriteComposed)
			if composed >= composedCount {
				continue
			}
			quad.sprite = binary.LittleEndian.Uint32(part[composedStart+composed*composedSize+4:])
		}
		quads = append(quads, quad)
	}
	return quads
}

// blend draws a pixel of color red, green, blue with alpha (0-255) over
// the pixel at offset.
func blend(pix []byte, offset int, red, green, blue, alpha uint32) {
	inverse := 255 - alpha
	pix[offset] = uint8((red*alpha + uint32(pix[offset])*inverse) / 255)
	pix[offset+1] = uint8((green*alpha + uint32(pix[offset+1])*inverse) / 255)
	pix[offset+2] = uint8((blue*alpha + uint32(pix[offset+2])*inverse) / 255)
}

// drawSprite draws RGBA sprite pixels multiplied by tint at x, y.
func drawSprite(img *image.NRGBA, pixels []byte, spriteSize, x, y int, tint color.NRGBA) {
	bounds := img.Bounds()
	for row := range spriteSize {
		targetY := y + row
		if targetY < bounds.Min.Y || targetY >= bounds.Max.Y {
			continue
		}
		for column := range spriteSize {
			targetX := x + column
			if targetX < bounds.Min.X || targetX >= bounds.Max.X {
				continue
			}
			source := (row*spriteSize + column) * 4
			alpha := uint32(pixels[source+3]) * uint32(tint.A) / 255
			if alpha == 0 {
				continue
			}
			red := uint32(pixels[source]) * uint32(tint.R) / 255
			green := uint32(pixels[source+1]) * uint32(tint.G) / 255
			blue := uint32(pixels[source+2]) * uint32(tint.B) / 255
			blend(img.Pix, img.PixOffset(targetX, targetY), red, green, blue, alpha)
		}
	}
}

// fillRect blends a color over a rectangle of the image.
func fillRect(img *image.NRGBA, rect image.Rectangle, fill color.NRGBA) {
	rect = rect.Intersect(img.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			blend(img.Pix, img.PixOffset(x, y), uint32(fill.R), uint32(fill.G), uint32(fill.B), uint32(fill.A))
		}
	}
}
