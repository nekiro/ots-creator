// Package assets reads and writes the asset folder of Tibia 12+ clients:
// catalog-content.json, the protobuf appearances file and LZMA compressed
// BMP sprite sheets.
package assets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// Sprite sheets are 384x384 pixel images.
const (
	SheetSize   = 384
	sheetPixels = SheetSize * SheetSize * 4
	// TileSize is the edge of one map tile in pixels.
	TileSize = 32
)

// Sprite types of catalog entries (sprite sizes in a sheet).
const (
	Sprite32x32 = 0
	Sprite32x64 = 1
	Sprite64x32 = 2
	Sprite64x64 = 3
)

// SpriteDims returns the sprite width and height of a sprite type in pixels.
func SpriteDims(spriteType int) (w, h int, ok bool) {
	switch spriteType {
	case Sprite32x32:
		return 32, 32, true
	case Sprite32x64:
		return 32, 64, true
	case Sprite64x32:
		return 64, 32, true
	case Sprite64x64:
		return 64, 64, true
	}
	return 0, 0, false
}

// SpriteTypeFor returns the sprite type of a sprite covering w x h tiles.
func SpriteTypeFor(w, h int) (int, bool) {
	switch {
	case w == 1 && h == 1:
		return Sprite32x32, true
	case w == 1 && h == 2:
		return Sprite32x64, true
	case w == 2 && h == 1:
		return Sprite64x32, true
	case w == 2 && h == 2:
		return Sprite64x64, true
	}
	return 0, false
}

// SheetCapacity returns how many sprites of a type fit in one sheet.
func SheetCapacity(spriteType int) int {
	w, h, ok := SpriteDims(spriteType)
	if !ok {
		return 0
	}
	return (SheetSize / w) * (SheetSize / h)
}

// SpriteRect returns the position of the i-th sprite of a sheet.
func SpriteRect(spriteType, i int) (x, y, w, h int) {
	w, h, _ = SpriteDims(spriteType)
	cols := SheetSize / w
	return (i % cols) * w, (i / cols) * h, w, h
}

// cipMagic follows the zero padding of the 32 byte CipSoft header.
var cipMagic = []byte{0x70, 0x0A, 0xFA, 0x80, 0x24}

const cipHeaderSize = 32

// decodeDictCap replaces the 32 MiB dictionary announced by the files: a
// sheet is smaller than 1 MiB, so a 1 MiB window decodes it the same way
// without allocating 32 MiB per sheet.
const decodeDictCap = 1 << 20

// DecodeSheet decompresses a .bmp.lzma sprite sheet into top-down RGBA
// pixels (SheetSize x SheetSize). Transparent pixels are all zero.
func DecodeSheet(data []byte) ([]byte, error) {
	i := 0
	for i < len(data) && data[i] == 0 {
		i++
	}
	if i+len(cipMagic) > len(data) || !bytes.Equal(data[i:i+len(cipMagic)], cipMagic) {
		return nil, errors.New("sprite sheet: missing header")
	}
	i += len(cipMagic)
	for i < len(data) && data[i]&0x80 != 0 {
		i++
	}
	i++ // last byte of the 7-bit encoded size
	if i+13 > len(data) {
		return nil, errors.New("sprite sheet: truncated")
	}
	// LZMA header: properties, dictionary size and a size field that holds
	// the compressed size. Mark the size unknown; the stream ends with an
	// end marker.
	var hdr [13]byte
	hdr[0] = data[i]
	dict := binary.LittleEndian.Uint32(data[i+1:])
	binary.LittleEndian.PutUint32(hdr[1:], min(dict, decodeDictCap))
	binary.LittleEndian.PutUint64(hdr[5:], ^uint64(0))
	r, err := lzma.NewReader(io.MultiReader(bytes.NewReader(hdr[:]), bytes.NewReader(data[i+13:])))
	if err != nil {
		return nil, fmt.Errorf("sprite sheet: %w", err)
	}
	bmp, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("sprite sheet: %w", err)
	}
	return decodeBMP(bmp)
}

func decodeBMP(b []byte) ([]byte, error) {
	if len(b) < 54 || b[0] != 'B' || b[1] != 'M' {
		return nil, errors.New("sprite sheet: not a BMP image")
	}
	off := int(binary.LittleEndian.Uint32(b[10:]))
	w := int(int32(binary.LittleEndian.Uint32(b[18:])))
	h := int(int32(binary.LittleEndian.Uint32(b[22:])))
	bpp := binary.LittleEndian.Uint16(b[28:])
	topDown := h < 0
	if topDown {
		h = -h
	}
	if w != SheetSize || h != SheetSize || bpp != 32 {
		return nil, fmt.Errorf("sprite sheet: unexpected %dx%d %d-bit image", w, h, bpp)
	}
	if off < 0 || off+sheetPixels > len(b) {
		return nil, errors.New("sprite sheet: truncated pixels")
	}
	src := b[off : off+sheetPixels]
	out := make([]byte, sheetPixels)
	row := SheetSize * 4
	for y := range SheetSize {
		sy := SheetSize - 1 - y
		if topDown {
			sy = y
		}
		s := src[sy*row : (sy+1)*row]
		d := out[y*row : (y+1)*row]
		for x := 0; x < row; x += 4 {
			if a := s[x+3]; a != 0 {
				d[x], d[x+1], d[x+2], d[x+3] = s[x+2], s[x+1], s[x], a
			}
		}
	}
	return out, nil
}

// bmpHeader is the 122 byte BITMAPV4HEADER the official sheets use:
// 384x384, 32 bits, BI_BITFIELDS with BGRA masks.
func bmpHeader() []byte {
	h := make([]byte, 122)
	h[0], h[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(h[2:], uint32(122+sheetPixels))
	binary.LittleEndian.PutUint32(h[10:], 122)
	binary.LittleEndian.PutUint32(h[14:], 108)
	binary.LittleEndian.PutUint32(h[18:], SheetSize)
	binary.LittleEndian.PutUint32(h[22:], SheetSize)
	binary.LittleEndian.PutUint16(h[26:], 1)
	binary.LittleEndian.PutUint16(h[28:], 32)
	binary.LittleEndian.PutUint32(h[30:], 3) // BI_BITFIELDS
	binary.LittleEndian.PutUint32(h[54:], 0x00FF0000)
	binary.LittleEndian.PutUint32(h[58:], 0x0000FF00)
	binary.LittleEndian.PutUint32(h[62:], 0x000000FF)
	binary.LittleEndian.PutUint32(h[66:], 0xFF000000)
	copy(h[70:], "BGRs")
	return h
}

// EncodeSheet compresses top-down RGBA pixels (SheetSize x SheetSize) into
// the .bmp.lzma format. Transparent pixels are stored as magenta with zero
// alpha, like the official files.
func EncodeSheet(rgba []byte) ([]byte, error) {
	if len(rgba) != sheetPixels {
		return nil, fmt.Errorf("sprite sheet: got %d bytes, want %d", len(rgba), sheetPixels)
	}
	bmp := append(bmpHeader(), make([]byte, sheetPixels)...)
	px := bmp[122:]
	row := SheetSize * 4
	for y := range SheetSize {
		s := rgba[y*row : (y+1)*row]
		d := px[(SheetSize-1-y)*row : (SheetSize-y)*row]
		for x := 0; x < row; x += 4 {
			if a := s[x+3]; a != 0 {
				d[x], d[x+1], d[x+2], d[x+3] = s[x+2], s[x+1], s[x], a
			} else {
				d[x], d[x+1], d[x+2], d[x+3] = 0xFF, 0x00, 0xFF, 0x00
			}
		}
	}
	var buf bytes.Buffer
	buf.Write(make([]byte, cipHeaderSize))
	w, err := lzma.WriterConfig{DictCap: decodeDictCap, EOSMarker: true}.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(bmp); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	lz := out[cipHeaderSize:]
	// Like the official files, the size field holds the compressed size.
	binary.LittleEndian.PutUint64(lz[5:], uint64(len(lz)-13))
	// Header: zero padding, magic, then the LZMA size as a 7-bit integer,
	// ending exactly at byte 32.
	tail := append([]byte(nil), cipMagic...)
	for n := uint32(len(lz)); ; n >>= 7 {
		if n < 0x80 {
			tail = append(tail, byte(n))
			break
		}
		tail = append(tail, byte(n)|0x80)
	}
	copy(out[cipHeaderSize-len(tail):cipHeaderSize], tail)
	return out, nil
}
