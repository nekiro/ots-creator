// Package obd reads and writes ObjectBuilder Data files (*.obd), versions
// 1, 2 and 3. Files are LZMA ("LZMA alone" format, as produced by Flash
// ByteArray.compress) wrapped binary data.
package obd

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/ulikunitz/xz/lzma"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/dat"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Format versions as stored in the file.
const (
	Version1 = 100
	Version2 = 200
	Version3 = 300
)

// maxDecompressed guards against decompression bombs (256 MiB).
const maxDecompressed = 256 << 20

// Sprite is one sprite embedded in an OBD file.
type Sprite struct {
	ID     uint32 // id in the source client, informative only
	Pixels []byte // RGBA, non-premultiplied, size*size*4 bytes
}

// Data is a decoded OBD file.
type Data struct {
	Version       int
	ClientVersion uint16
	Thing         *thing.Thing
	SpriteSize    int
	// Sprites holds the pixels of every slot of every frame group, in the
	// same order as Thing.FrameGroups[i].Sprites.
	Sprites [][]Sprite
}

// Decode decompresses and parses an OBD file.
func Decode(file []byte) (*Data, error) {
	raw, err := decompress(file)
	if err != nil {
		return nil, err
	}
	return DecodeRaw(raw)
}

// DecodeRaw parses decompressed OBD data.
func DecodeRaw(raw []byte) (*Data, error) {
	reader := binio.NewReader(raw)
	version := reader.U16()
	if reader.Err() != nil {
		return nil, fmt.Errorf("obd: %w", reader.Err())
	}
	var data *Data
	var err error
	switch {
	case version == Version3 || version == Version2:
		data, err = decodeV23(reader, int(version))
	case version >= 710:
		reader.Seek(0)
		data, err = decodeV1(reader)
	default:
		return nil, fmt.Errorf("obd: unknown version %d", version)
	}
	if err != nil {
		return nil, fmt.Errorf("obd v%d: %w", max(int(version)/100, 1), err)
	}
	return data, nil
}

func decodeV1(reader *binio.Reader) (*Data, error) {
	data := &Data{Version: Version1, ClientVersion: reader.U16(), SpriteSize: client.DefaultSpriteSize}
	category, err := thing.ParseCategory(reader.UTF())
	if err != nil {
		return nil, err
	}
	object := &thing.Thing{Category: category}
	table := dat.Table(client.MetadataFormat(data.ClientVersion))
	if err := table.ReadProperties(reader, &object.Props); err != nil {
		return nil, err
	}
	group := &thing.FrameGroup{}
	// v1 always stores pattern Z and never stores durations.
	if err := dat.ReadLayout(reader, group, true, false, category.DefaultDuration()); err != nil {
		return nil, err
	}
	sprites := make([]Sprite, len(group.Sprites))
	for i := range sprites {
		group.Sprites[i] = reader.U32()
		length := int(reader.U32())
		if reader.Err() != nil {
			return nil, reader.Err()
		}
		pixels, err := readARGB(reader, length, data)
		if err != nil {
			return nil, fmt.Errorf("sprite %d: %w", i, err)
		}
		sprites[i] = Sprite{ID: group.Sprites[i], Pixels: pixels}
	}
	object.FrameGroups = []*thing.FrameGroup{group}
	data.Thing = object
	data.Sprites = [][]Sprite{sprites}
	return data, reader.Err()
}

func decodeV23(reader *binio.Reader, version int) (*Data, error) {
	data := &Data{Version: version, ClientVersion: reader.U16(), SpriteSize: client.DefaultSpriteSize}
	category := thing.Category(reader.U8())
	if !category.Valid() {
		return nil, fmt.Errorf("invalid category %d", category)
	}
	_ = reader.U32() // offset of texture patterns, redundant
	object := &thing.Thing{Category: category}
	if err := dat.OBDTable.ReadProperties(reader, &object.Props); err != nil {
		return nil, err
	}
	groupCount := 1
	hasGroupTypes := version == Version3 && category == thing.CategoryOutfit
	if hasGroupTypes {
		groupCount = int(reader.U8())
	}
	for groupIndex := 0; groupIndex < groupCount; groupIndex++ {
		group := &thing.FrameGroup{}
		if hasGroupTypes {
			group.Type = thing.FrameGroupType(reader.U8())
		}
		if err := dat.ReadLayout(reader, group, true, true, category.DefaultDuration()); err != nil {
			return nil, err
		}
		sprites := make([]Sprite, len(group.Sprites))
		for i := range sprites {
			group.Sprites[i] = reader.U32()
			length := data.SpriteSize * data.SpriteSize * 4
			if version == Version3 {
				length = int(reader.U32())
			}
			if reader.Err() != nil {
				return nil, reader.Err()
			}
			pixels, err := readARGB(reader, length, data)
			if err != nil {
				return nil, fmt.Errorf("group %d sprite %d: %w", groupIndex, i, err)
			}
			sprites[i] = Sprite{ID: group.Sprites[i], Pixels: pixels}
		}
		object.FrameGroups = append(object.FrameGroups, group)
		data.Sprites = append(data.Sprites, sprites)
	}
	data.Thing = object
	return data, reader.Err()
}

// readARGB reads n bytes of ARGB pixels and returns RGBA. The first sprite
// with a non-default length fixes the sprite size of the file.
func readARGB(reader *binio.Reader, length int, data *Data) ([]byte, error) {
	if length == 0 || length%4 != 0 || length > 1<<20 {
		return nil, fmt.Errorf("invalid pixel data size %d", length)
	}
	size := int(math.Sqrt(float64(length / 4)))
	if size*size*4 != length {
		// v1 allowed shorter buffers, pad them to the default size.
		size = data.SpriteSize
		if length > size*size*4 {
			return nil, fmt.Errorf("invalid pixel data size %d", length)
		}
	}
	data.SpriteSize = size
	argb := reader.Bytes(length)
	if argb == nil {
		return nil, reader.Err()
	}
	out := make([]byte, size*size*4)
	for i := 0; i+3 < len(argb); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = argb[i+1], argb[i+2], argb[i+3], argb[i]
	}
	return out, nil
}

// Encode serializes and compresses data with d.Version.
func Encode(data *Data) ([]byte, error) {
	raw, err := EncodeRaw(data)
	if err != nil {
		return nil, err
	}
	return compress(raw)
}

// EncodeRaw serializes data without compression.
func EncodeRaw(data *Data) ([]byte, error) {
	object := data.Thing
	if err := object.Validate(); err != nil {
		return nil, err
	}
	if len(data.Sprites) < len(object.FrameGroups) {
		return nil, fmt.Errorf("obd: pixels for %d groups, thing has %d", len(data.Sprites), len(object.FrameGroups))
	}
	size := data.SpriteSize
	if size == 0 {
		size = client.DefaultSpriteSize
	}
	writer := binio.NewWriter(64 << 10)
	groups := object.FrameGroups
	switch data.Version {
	case Version1:
		writer.U16(data.ClientVersion)
		writer.UTF(object.Category.String())
		dat.Table(client.MetadataFormat(data.ClientVersion)).WriteProperties(writer, &object.Props)
		groups = groups[:1]
	case Version2, Version3:
		writer.U16(uint16(data.Version))
		writer.U16(data.ClientVersion)
		writer.U8(uint8(object.Category))
		offsetPos := writer.Len()
		writer.U32(0)
		dat.OBDTable.WriteProperties(writer, &object.Props)
		writer.PutU32At(offsetPos, uint32(writer.Len()))
		if data.Version == Version2 {
			groups = groups[:1]
		} else if object.Category == thing.CategoryOutfit {
			writer.U8(uint8(len(groups)))
		}
	default:
		return nil, fmt.Errorf("obd: unknown version %d", data.Version)
	}
	for groupIndex, group := range groups {
		if data.Version == Version3 && object.Category == thing.CategoryOutfit {
			groupType := uint8(groupIndex)
			if len(groups) < 2 {
				groupType = 1
			}
			writer.U8(groupType)
		}
		if err := dat.WriteLayout(writer, group, true, data.Version != Version1); err != nil {
			return nil, err
		}
		sprites := data.Sprites[groupIndex]
		if len(sprites) != len(group.Sprites) {
			return nil, fmt.Errorf("obd: group %d has %d sprites, layout needs %d", groupIndex, len(sprites), len(group.Sprites))
		}
		for i, sprite := range sprites {
			if len(sprite.Pixels) != size*size*4 {
				return nil, fmt.Errorf("obd: group %d sprite %d has %d bytes, want %d", groupIndex, i, len(sprite.Pixels), size*size*4)
			}
			writer.U32(group.Sprites[i])
			if data.Version != Version2 {
				writer.U32(uint32(len(sprite.Pixels)))
			}
			writeARGB(writer, sprite.Pixels)
		}
	}
	return writer.Bytes(), nil
}

func writeARGB(writer *binio.Writer, rgba []byte) {
	out := make([]byte, len(rgba))
	for i := 0; i+3 < len(rgba); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = rgba[i+3], rgba[i], rgba[i+1], rgba[i+2]
	}
	writer.Write(out)
}

func decompress(file []byte) ([]byte, error) {
	decompressor, err := lzma.NewReader(bytes.NewReader(file))
	if err != nil {
		return nil, fmt.Errorf("obd: not an LZMA stream: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(decompressor, maxDecompressed+1))
	if err != nil {
		return nil, fmt.Errorf("obd: decompress: %w", err)
	}
	if len(raw) > maxDecompressed {
		return nil, fmt.Errorf("obd: decompressed data exceeds %d bytes", maxDecompressed)
	}
	return raw, nil
}

func compress(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	config := lzma.WriterConfig{SizeInHeader: true, Size: int64(len(raw)), EOSMarker: false}
	compressor, err := config.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := compressor.Write(raw); err != nil {
		return nil, err
	}
	if err := compressor.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
