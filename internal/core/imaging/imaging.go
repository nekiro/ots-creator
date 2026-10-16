// Package imaging converts between sprites and images: sprite sheets of
// frame groups (ObjectBuilder compatible layout) and tiles of arbitrary
// images.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // decoder registration
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// Magenta is the background ObjectBuilder uses for exported sheets. It is
// treated as transparent on import.
var Magenta = color.NRGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 0xFF}

// Decode reads an image (PNG, or any registered format) as NRGBA.
func Decode(reader io.Reader) (*image.NRGBA, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return ToNRGBA(img), nil
}

// Load reads an image file as NRGBA.
func Load(path string) (*image.NRGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Decode(file)
}

// EncodePNG encodes an image as PNG.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Formats lists the image formats Encode writes, by file extension.
var Formats = []string{"png", "bmp", "jpg"}

// FormatOf returns the export format for a file name ("png" when unknown).
func FormatOf(path string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "bmp":
		return "bmp"
	case "jpg", "jpeg":
		return "jpg"
	}
	return "png"
}

// Encode writes img as png, bmp or jpg. Formats without alpha get
// transparent pixels flattened onto background.
func Encode(img *image.NRGBA, format string, background color.NRGBA) ([]byte, error) {
	if format == "png" || format == "" {
		return EncodePNG(img)
	}
	flat := image.NewNRGBA(img.Rect)
	background.A = 0xFF
	draw.Draw(flat, flat.Rect, &image.Uniform{C: background}, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Rect, img, img.Rect.Min, draw.Over)
	var buf bytes.Buffer
	var err error
	switch format {
	case "bmp":
		err = bmp.Encode(&buf, flat)
	case "jpg":
		err = jpeg.Encode(&buf, flat, &jpeg.Options{Quality: 95})
	default:
		return nil, fmt.Errorf("unknown image format %q", format)
	}
	return buf.Bytes(), err
}

// ToNRGBA converts any image to NRGBA with origin (0,0).
func ToNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok && nrgba.Rect.Min == (image.Point{}) {
		return nrgba
	}
	bounds := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(out, out.Rect, img, bounds.Min, draw.Src)
	return out
}

// ContentBounds returns the smallest rectangle holding every pixel with
// alpha > 0, or an empty rectangle for a fully transparent image.
func ContentBounds(img *image.NRGBA) image.Rectangle {
	bounds := img.Rect
	minX, minY, maxX, maxY := bounds.Max.X, bounds.Max.Y, bounds.Min.X-1, bounds.Min.Y-1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := img.Pix[img.PixOffset(bounds.Min.X, y):]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if row[(x-bounds.Min.X)*4+3] == 0 {
				continue
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if maxX < minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// CropToContent returns a copy cropped to ContentBounds. Fully transparent
// images are returned unchanged.
func CropToContent(img *image.NRGBA) *image.NRGBA {
	content := ContentBounds(img)
	if content.Empty() || content == img.Rect {
		return img
	}
	out := image.NewNRGBA(image.Rect(0, 0, content.Dx(), content.Dy()))
	draw.Draw(out, out.Rect, img, content.Min, draw.Src)
	return out
}

// RemoveMagenta makes opaque magenta pixels fully transparent in place.
func RemoveMagenta(img *image.NRGBA) {
	pixels := img.Pix
	for i := 0; i+3 < len(pixels); i += 4 {
		if pixels[i] == 0xFF && pixels[i+1] == 0 && pixels[i+2] == 0xFF && pixels[i+3] == 0xFF {
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 0, 0, 0, 0
		}
	}
}

// Normalize clears the color of fully transparent pixels so identical
// sprites compress to identical bytes.
func Normalize(img *image.NRGBA) {
	pixels := img.Pix
	for i := 0; i+3 < len(pixels); i += 4 {
		if pixels[i+3] == 0 {
			pixels[i], pixels[i+1], pixels[i+2] = 0, 0, 0
		}
	}
}

// Tile copies a size x size block at (x, y) into RGBA bytes. Pixels outside
// the image are transparent.
func Tile(img *image.NRGBA, x, y, size int) []byte {
	out := make([]byte, size*size*4)
	bounds := img.Rect
	for row := 0; row < size; row++ {
		sourceY := y + row
		if sourceY < bounds.Min.Y || sourceY >= bounds.Max.Y {
			continue
		}
		startX, endX := max(x, bounds.Min.X), min(x+size, bounds.Max.X)
		if startX >= endX {
			continue
		}
		src := img.Pix[img.PixOffset(startX, sourceY) : img.PixOffset(endX-1, sourceY)+4]
		copy(out[(row*size+(startX-x))*4:], src)
	}
	return out
}

// PutTile draws RGBA sprite bytes at (x, y), replacing destination pixels.
func PutTile(img *image.NRGBA, x, y, size int, pixels []byte) {
	for row := 0; row < size; row++ {
		offset := img.PixOffset(x, y+row)
		copy(img.Pix[offset:offset+size*4], pixels[row*size*4:(row+1)*size*4])
	}
}

// SliceTiles cuts an image into size x size tiles, row by row. Partial
// tiles at the right/bottom edge are padded with transparency.
func SliceTiles(img *image.NRGBA, size int) [][]byte {
	cols := (img.Rect.Dx() + size - 1) / size
	rows := (img.Rect.Dy() + size - 1) / size
	out := make([][]byte, 0, cols*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			out = append(out, Tile(img, col*size, row*size, size))
		}
	}
	return out
}

// Shift moves the pixels of img by offsetX, offsetY. ok is false when a
// visible pixel would end up outside the image.
func Shift(img *image.NRGBA, offsetX, offsetY int) (*image.NRGBA, bool) {
	bounds := img.Rect
	out := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			offset := img.PixOffset(x, y)
			if img.Pix[offset+3] == 0 {
				continue
			}
			movedX, movedY := x+offsetX, y+offsetY
			if movedX < bounds.Min.X || movedY < bounds.Min.Y || movedX >= bounds.Max.X || movedY >= bounds.Max.Y {
				return nil, false
			}
			copy(out.Pix[out.PixOffset(movedX, movedY):], img.Pix[offset:offset+4])
		}
	}
	return out, true
}

// textureOrigin returns the pixel position of a texture in a group sheet.
func textureOrigin(group *thing.FrameGroup, size, layer, patternX, patternY, patternZ, frame int) (int, int) {
	totalX := int(group.PatternZ) * int(group.PatternX) * int(group.Layers)
	index := group.TextureIndex(layer, patternX, patternY, patternZ, frame)
	return (index % totalX) * int(group.Width) * size, (index / totalX) * int(group.Height) * size
}

// eachSprite calls fn for every sprite slot with its sheet position.
func eachSprite(group *thing.FrameGroup, size int, fn func(slot, x, y int)) {
	for frame := 0; frame < int(group.Frames); frame++ {
		for z := 0; z < int(group.PatternZ); z++ {
			for patternY := 0; patternY < int(group.PatternY); patternY++ {
				for patternX := 0; patternX < int(group.PatternX); patternX++ {
					for layer := 0; layer < int(group.Layers); layer++ {
						originX, originY := textureOrigin(group, size, layer, patternX, patternY, z, frame)
						for tileX := 0; tileX < int(group.Width); tileX++ {
							for tileY := 0; tileY < int(group.Height); tileY++ {
								slot := group.SpriteIndex(tileX, tileY, layer, patternX, patternY, z, frame)
								fn(slot, originX+(int(group.Width)-tileX-1)*size, originY+(int(group.Height)-tileY-1)*size)
							}
						}
					}
				}
			}
		}
	}
}

// Sheet renders a frame group as a sprite sheet. pixels returns RGBA bytes
// for a slot (nil for empty). background fills the sheet; use Magenta for
// ObjectBuilder compatible output or a transparent color.
func Sheet(group *thing.FrameGroup, size int, background color.NRGBA, pixels func(slot int) []byte) *image.NRGBA {
	width, height := group.SheetSize(size)
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Rect, &image.Uniform{C: background}, image.Point{}, draw.Src)
	eachSprite(group, size, func(slot, x, y int) {
		spritePixels := pixels(slot)
		if spritePixels == nil {
			return
		}
		DrawOver(img, x, y, size, spritePixels)
	})
	return img
}

// DrawOver draws RGBA sprite bytes at (x, y), keeping the destination where
// the sprite is fully transparent (so a background color stays).
func DrawOver(img *image.NRGBA, x, y, size int, pixels []byte) {
	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			source := pixels[(row*size+col)*4:]
			if source[3] == 0 {
				continue
			}
			offset := img.PixOffset(x+col, y+row)
			copy(img.Pix[offset:offset+4], source[:4])
		}
	}
}

// SliceSheet cuts a group sheet into sprites, returning RGBA bytes indexed
// by sprite slot. Magenta is removed. The image must match group.SheetSize.
func SliceSheet(img *image.NRGBA, group *thing.FrameGroup, size int) ([][]byte, error) {
	return sliceSheet(img, group, size, true)
}

// SliceSheetExact is SliceSheet for sheets with real transparency: magenta
// is kept as a color.
func SliceSheetExact(img *image.NRGBA, group *thing.FrameGroup, size int) ([][]byte, error) {
	return sliceSheet(img, group, size, false)
}

func sliceSheet(img *image.NRGBA, group *thing.FrameGroup, size int, magenta bool) ([][]byte, error) {
	width, height := group.SheetSize(size)
	if img.Rect.Dx() != width || img.Rect.Dy() != height {
		return nil, fmt.Errorf("sheet is %dx%d, frame group needs %dx%d", img.Rect.Dx(), img.Rect.Dy(), width, height)
	}
	clean := image.NewNRGBA(img.Rect)
	copy(clean.Pix, img.Pix)
	if magenta {
		RemoveMagenta(clean)
	}
	Normalize(clean)
	out := make([][]byte, group.TotalSprites())
	eachSprite(group, size, func(slot, x, y int) {
		out[slot] = Tile(clean, x, y, size)
	})
	return out, nil
}
