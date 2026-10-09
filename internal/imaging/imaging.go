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

	"github.com/nekiro/ots-creator/internal/thing"
)

// Magenta is the background ObjectBuilder uses for exported sheets. It is
// treated as transparent on import.
var Magenta = color.NRGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 0xFF}

// Decode reads an image (PNG, or any registered format) as NRGBA.
func Decode(r io.Reader) (*image.NRGBA, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return ToNRGBA(img), nil
}

// Load reads an image file as NRGBA.
func Load(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

// EncodePNG encodes an image as PNG.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
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
// transparent pixels flattened onto bg.
func Encode(img *image.NRGBA, format string, bg color.NRGBA) ([]byte, error) {
	if format == "png" || format == "" {
		return EncodePNG(img)
	}
	flat := image.NewNRGBA(img.Rect)
	bg.A = 0xFF
	draw.Draw(flat, flat.Rect, &image.Uniform{C: bg}, image.Point{}, draw.Src)
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
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	return out
}

// ContentBounds returns the smallest rectangle holding every pixel with
// alpha > 0, or an empty rectangle for a fully transparent image.
func ContentBounds(img *image.NRGBA) image.Rectangle {
	b := img.Rect
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y):]
		for x := b.Min.X; x < b.Max.X; x++ {
			if row[(x-b.Min.X)*4+3] == 0 {
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
	r := ContentBounds(img)
	if r.Empty() || r == img.Rect {
		return img
	}
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Rect, img, r.Min, draw.Src)
	return out
}

// RemoveMagenta makes opaque magenta pixels fully transparent in place.
func RemoveMagenta(img *image.NRGBA) {
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		if p[i] == 0xFF && p[i+1] == 0 && p[i+2] == 0xFF && p[i+3] == 0xFF {
			p[i], p[i+1], p[i+2], p[i+3] = 0, 0, 0, 0
		}
	}
}

// Normalize clears the color of fully transparent pixels so identical
// sprites compress to identical bytes.
func Normalize(img *image.NRGBA) {
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		if p[i+3] == 0 {
			p[i], p[i+1], p[i+2] = 0, 0, 0
		}
	}
}

// Tile copies a size x size block at (x, y) into RGBA bytes. Pixels outside
// the image are transparent.
func Tile(img *image.NRGBA, x, y, size int) []byte {
	out := make([]byte, size*size*4)
	b := img.Rect
	for row := 0; row < size; row++ {
		sy := y + row
		if sy < b.Min.Y || sy >= b.Max.Y {
			continue
		}
		x0, x1 := max(x, b.Min.X), min(x+size, b.Max.X)
		if x0 >= x1 {
			continue
		}
		src := img.Pix[img.PixOffset(x0, sy) : img.PixOffset(x1-1, sy)+4]
		copy(out[(row*size+(x0-x))*4:], src)
	}
	return out
}

// PutTile draws RGBA sprite bytes at (x, y), replacing destination pixels.
func PutTile(img *image.NRGBA, x, y, size int, px []byte) {
	for row := 0; row < size; row++ {
		o := img.PixOffset(x, y+row)
		copy(img.Pix[o:o+size*4], px[row*size*4:(row+1)*size*4])
	}
}

// SliceTiles cuts an image into size x size tiles, row by row. Partial
// tiles at the right/bottom edge are padded with transparency.
func SliceTiles(img *image.NRGBA, size int) [][]byte {
	cols := (img.Rect.Dx() + size - 1) / size
	rows := (img.Rect.Dy() + size - 1) / size
	out := make([][]byte, 0, cols*rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out = append(out, Tile(img, c*size, r*size, size))
		}
	}
	return out
}

// textureOrigin returns the pixel position of a texture in a group sheet.
func textureOrigin(g *thing.FrameGroup, size, layer, px, py, pz, frame int) (int, int) {
	totalX := int(g.PatternZ) * int(g.PatternX) * int(g.Layers)
	idx := g.TextureIndex(layer, px, py, pz, frame)
	return (idx % totalX) * int(g.Width) * size, (idx / totalX) * int(g.Height) * size
}

// eachSprite calls fn for every sprite slot with its sheet position.
func eachSprite(g *thing.FrameGroup, size int, fn func(slot, x, y int)) {
	for f := 0; f < int(g.Frames); f++ {
		for z := 0; z < int(g.PatternZ); z++ {
			for py := 0; py < int(g.PatternY); py++ {
				for px := 0; px < int(g.PatternX); px++ {
					for l := 0; l < int(g.Layers); l++ {
						fx, fy := textureOrigin(g, size, l, px, py, z, f)
						for w := 0; w < int(g.Width); w++ {
							for h := 0; h < int(g.Height); h++ {
								slot := g.SpriteIndex(w, h, l, px, py, z, f)
								fn(slot, fx+(int(g.Width)-w-1)*size, fy+(int(g.Height)-h-1)*size)
							}
						}
					}
				}
			}
		}
	}
}

// Sheet renders a frame group as a sprite sheet. pixels returns RGBA bytes
// for a slot (nil for empty). bg fills the background; use Magenta for
// ObjectBuilder compatible output or a transparent color.
func Sheet(g *thing.FrameGroup, size int, bg color.NRGBA, pixels func(slot int) []byte) *image.NRGBA {
	w, h := g.SheetSize(size)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Rect, &image.Uniform{C: bg}, image.Point{}, draw.Src)
	eachSprite(g, size, func(slot, x, y int) {
		px := pixels(slot)
		if px == nil {
			return
		}
		DrawOver(img, x, y, size, px)
	})
	return img
}

// DrawOver draws RGBA sprite bytes at (x, y), keeping the destination where
// the sprite is fully transparent (so a background color stays).
func DrawOver(img *image.NRGBA, x, y, size int, px []byte) {
	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			s := px[(row*size+col)*4:]
			if s[3] == 0 {
				continue
			}
			o := img.PixOffset(x+col, y+row)
			copy(img.Pix[o:o+4], s[:4])
		}
	}
}

// SliceSheet cuts a group sheet into sprites, returning RGBA bytes indexed
// by sprite slot. Magenta is removed. The image must match g.SheetSize.
func SliceSheet(img *image.NRGBA, g *thing.FrameGroup, size int) ([][]byte, error) {
	w, h := g.SheetSize(size)
	if img.Rect.Dx() != w || img.Rect.Dy() != h {
		return nil, fmt.Errorf("sheet is %dx%d, frame group needs %dx%d", img.Rect.Dx(), img.Rect.Dy(), w, h)
	}
	clean := image.NewNRGBA(img.Rect)
	copy(clean.Pix, img.Pix)
	RemoveMagenta(clean)
	Normalize(clean)
	out := make([][]byte, g.TotalSprites())
	eachSprite(g, size, func(slot, x, y int) {
		out[slot] = Tile(clean, x, y, size)
	})
	return out, nil
}
