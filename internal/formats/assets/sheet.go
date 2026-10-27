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
	TileSize    = 32
)

const (
	Sprite32x32 = 0
	Sprite32x64 = 1
	Sprite64x32 = 2
	Sprite64x64 = 3
)

func SpriteDims(spriteType int) (width, height int, ok bool) {
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

func SpriteTypeFor(width, height int) (int, bool) {
	switch {
	case width == 1 && height == 1:
		return Sprite32x32, true
	case width == 1 && height == 2:
		return Sprite32x64, true
	case width == 2 && height == 1:
		return Sprite64x32, true
	case width == 2 && height == 2:
		return Sprite64x64, true
	}
	return 0, false
}

func SheetCapacity(spriteType int) int {
	width, height, ok := SpriteDims(spriteType)
	if !ok {
		return 0
	}
	return (SheetSize / width) * (SheetSize / height)
}

func SpriteRect(spriteType, i int) (x, y, width, height int) {
	width, height, _ = SpriteDims(spriteType)
	columns := SheetSize / width
	return (i % columns) * width, (i / columns) * height, width, height
}

// cipMagic follows the zero padding of the 32 byte CipSoft header.
var cipMagic = []byte{0x70, 0x0A, 0xFA, 0x80, 0x24}

const cipHeaderSize = 32

// lzmaHeaderSize is the size of the LZMA header: properties, dictionary
// size and the size field.
const lzmaHeaderSize = 13

const bmpHeaderSize = 122

// decodeDictCap replaces the 32 MiB dictionary announced by the files: a
// sheet is smaller than 1 MiB, so a 1 MiB window decodes it the same way
// without allocating 32 MiB per sheet.
const decodeDictCap = 1 << 20

// DecodeSheet decompresses a .bmp.lzma sprite sheet into top-down RGBA
// pixels (SheetSize x SheetSize). Transparent pixels are all zero.
func DecodeSheet(data []byte) ([]byte, error) {
	pos := 0
	for pos < len(data) && data[pos] == 0 {
		pos++
	}
	if pos+len(cipMagic) > len(data) || !bytes.Equal(data[pos:pos+len(cipMagic)], cipMagic) {
		return nil, errors.New("sprite sheet: missing header")
	}
	pos += len(cipMagic)
	for pos < len(data) && data[pos]&0x80 != 0 {
		pos++
	}
	pos++ // last byte of the 7-bit encoded size
	if pos+lzmaHeaderSize > len(data) {
		return nil, errors.New("sprite sheet: truncated")
	}
	// LZMA header: properties, dictionary size and a size field that holds
	// the compressed size. Mark the size unknown; the stream ends with an
	// end marker.
	var lzmaHeader [lzmaHeaderSize]byte
	lzmaHeader[0] = data[pos]
	dictSize := binary.LittleEndian.Uint32(data[pos+1:])
	binary.LittleEndian.PutUint32(lzmaHeader[1:], min(dictSize, decodeDictCap))
	binary.LittleEndian.PutUint64(lzmaHeader[5:], ^uint64(0))
	reader, err := lzma.NewReader(io.MultiReader(bytes.NewReader(lzmaHeader[:]), bytes.NewReader(data[pos+lzmaHeaderSize:])))
	if err != nil {
		return nil, fmt.Errorf("sprite sheet: %w", err)
	}
	bmp, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("sprite sheet: %w", err)
	}
	return decodeBMP(bmp)
}

func decodeBMP(bmp []byte) ([]byte, error) {
	if len(bmp) < 54 || bmp[0] != 'B' || bmp[1] != 'M' {
		return nil, errors.New("sprite sheet: not a BMP image")
	}
	pixelOffset := int(binary.LittleEndian.Uint32(bmp[10:]))
	width := int(int32(binary.LittleEndian.Uint32(bmp[18:])))
	height := int(int32(binary.LittleEndian.Uint32(bmp[22:])))
	bitsPerPixel := binary.LittleEndian.Uint16(bmp[28:])
	topDown := height < 0
	if topDown {
		height = -height
	}
	if width != SheetSize || height != SheetSize || bitsPerPixel != 32 {
		return nil, fmt.Errorf("sprite sheet: unexpected %dx%d %d-bit image", width, height, bitsPerPixel)
	}
	if pixelOffset < 0 || pixelOffset+sheetPixels > len(bmp) {
		return nil, errors.New("sprite sheet: truncated pixels")
	}
	pixels := bmp[pixelOffset : pixelOffset+sheetPixels]
	out := make([]byte, sheetPixels)
	rowSize := SheetSize * 4
	for y := range SheetSize {
		sourceY := SheetSize - 1 - y
		if topDown {
			sourceY = y
		}
		source := pixels[sourceY*rowSize : (sourceY+1)*rowSize]
		dest := out[y*rowSize : (y+1)*rowSize]
		for x := 0; x < rowSize; x += 4 {
			if alpha := source[x+3]; alpha != 0 {
				dest[x], dest[x+1], dest[x+2], dest[x+3] = source[x+2], source[x+1], source[x], alpha
			}
		}
	}
	return out, nil
}

// bmpHeader is the 122 byte BITMAPV4HEADER the official sheets use:
// 384x384, 32 bits, BI_BITFIELDS with BGRA masks.
func bmpHeader() []byte {
	header := make([]byte, bmpHeaderSize)
	header[0], header[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(header[2:], uint32(bmpHeaderSize+sheetPixels))
	binary.LittleEndian.PutUint32(header[10:], bmpHeaderSize)
	binary.LittleEndian.PutUint32(header[14:], 108)
	binary.LittleEndian.PutUint32(header[18:], SheetSize)
	binary.LittleEndian.PutUint32(header[22:], SheetSize)
	binary.LittleEndian.PutUint16(header[26:], 1)
	binary.LittleEndian.PutUint16(header[28:], 32)
	binary.LittleEndian.PutUint32(header[30:], 3) // BI_BITFIELDS
	binary.LittleEndian.PutUint32(header[54:], 0x00FF0000)
	binary.LittleEndian.PutUint32(header[58:], 0x0000FF00)
	binary.LittleEndian.PutUint32(header[62:], 0x000000FF)
	binary.LittleEndian.PutUint32(header[66:], 0xFF000000)
	copy(header[70:], "BGRs")
	return header
}

// EncodeSheet compresses top-down RGBA pixels (SheetSize x SheetSize) into
// the .bmp.lzma format. Transparent pixels are stored as magenta with zero
// alpha, like the official files.
func EncodeSheet(rgba []byte) ([]byte, error) {
	if len(rgba) != sheetPixels {
		return nil, fmt.Errorf("sprite sheet: got %d bytes, want %d", len(rgba), sheetPixels)
	}
	bmp := append(bmpHeader(), make([]byte, sheetPixels)...)
	pixels := bmp[bmpHeaderSize:]
	rowSize := SheetSize * 4
	for y := range SheetSize {
		source := rgba[y*rowSize : (y+1)*rowSize]
		dest := pixels[(SheetSize-1-y)*rowSize : (SheetSize-y)*rowSize]
		for x := 0; x < rowSize; x += 4 {
			if alpha := source[x+3]; alpha != 0 {
				dest[x], dest[x+1], dest[x+2], dest[x+3] = source[x+2], source[x+1], source[x], alpha
			} else {
				dest[x], dest[x+1], dest[x+2], dest[x+3] = 0xFF, 0x00, 0xFF, 0x00
			}
		}
	}
	var buf bytes.Buffer
	buf.Write(make([]byte, cipHeaderSize))
	writer, err := lzma.WriterConfig{DictCap: decodeDictCap, EOSMarker: true}.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(bmp); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	compressed := out[cipHeaderSize:]
	// Like the official files, the size field holds the compressed size.
	binary.LittleEndian.PutUint64(compressed[5:], uint64(len(compressed)-lzmaHeaderSize))
	// Header: zero padding, magic, then the LZMA size as a 7-bit integer,
	// ending exactly at byte 32.
	tail := append([]byte(nil), cipMagic...)
	for size := uint32(len(compressed)); ; size >>= 7 {
		if size < 0x80 {
			tail = append(tail, byte(size))
			break
		}
		tail = append(tail, byte(size)|0x80)
	}
	copy(out[cipHeaderSize-len(tail):cipHeaderSize], tail)
	return out, nil
}
