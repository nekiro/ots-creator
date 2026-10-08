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
	r := binio.NewReader(raw)
	v := r.U16()
	if r.Err() != nil {
		return nil, fmt.Errorf("obd: %w", r.Err())
	}
	var d *Data
	var err error
	switch {
	case v == Version3 || v == Version2:
		d, err = decodeV23(r, int(v))
	case v >= 710:
		r.Seek(0)
		d, err = decodeV1(r)
	default:
		return nil, fmt.Errorf("obd: unknown version %d", v)
	}
	if err != nil {
		return nil, fmt.Errorf("obd v%d: %w", max(int(v)/100, 1), err)
	}
	return d, nil
}

func decodeV1(r *binio.Reader) (*Data, error) {
	d := &Data{Version: Version1, ClientVersion: r.U16(), SpriteSize: client.DefaultSpriteSize}
	cat, err := thing.ParseCategory(r.UTF())
	if err != nil {
		return nil, err
	}
	t := &thing.Thing{Category: cat}
	table := dat.Table(client.MetadataFormat(d.ClientVersion))
	if err := table.ReadProperties(r, &t.Props); err != nil {
		return nil, err
	}
	g := &thing.FrameGroup{}
	// v1 always stores pattern Z and never stores durations.
	if err := dat.ReadLayout(r, g, true, false, cat.DefaultDuration()); err != nil {
		return nil, err
	}
	sprites := make([]Sprite, len(g.Sprites))
	for i := range sprites {
		g.Sprites[i] = r.U32()
		n := int(r.U32())
		if r.Err() != nil {
			return nil, r.Err()
		}
		px, err := readARGB(r, n, d)
		if err != nil {
			return nil, fmt.Errorf("sprite %d: %w", i, err)
		}
		sprites[i] = Sprite{ID: g.Sprites[i], Pixels: px}
	}
	t.FrameGroups = []*thing.FrameGroup{g}
	d.Thing = t
	d.Sprites = [][]Sprite{sprites}
	return d, r.Err()
}

func decodeV23(r *binio.Reader, version int) (*Data, error) {
	d := &Data{Version: version, ClientVersion: r.U16(), SpriteSize: client.DefaultSpriteSize}
	cat := thing.Category(r.U8())
	if !cat.Valid() {
		return nil, fmt.Errorf("invalid category %d", cat)
	}
	_ = r.U32() // offset of texture patterns, redundant
	t := &thing.Thing{Category: cat}
	if err := dat.OBDTable.ReadProperties(r, &t.Props); err != nil {
		return nil, err
	}
	groupCount := 1
	multi := version == Version3 && cat == thing.CategoryOutfit
	if multi {
		groupCount = int(r.U8())
	}
	for gi := 0; gi < groupCount; gi++ {
		g := &thing.FrameGroup{}
		if multi {
			g.Type = thing.FrameGroupType(r.U8())
		}
		if err := dat.ReadLayout(r, g, true, true, cat.DefaultDuration()); err != nil {
			return nil, err
		}
		sprites := make([]Sprite, len(g.Sprites))
		for i := range sprites {
			g.Sprites[i] = r.U32()
			n := d.SpriteSize * d.SpriteSize * 4
			if version == Version3 {
				n = int(r.U32())
			}
			if r.Err() != nil {
				return nil, r.Err()
			}
			px, err := readARGB(r, n, d)
			if err != nil {
				return nil, fmt.Errorf("group %d sprite %d: %w", gi, i, err)
			}
			sprites[i] = Sprite{ID: g.Sprites[i], Pixels: px}
		}
		t.FrameGroups = append(t.FrameGroups, g)
		d.Sprites = append(d.Sprites, sprites)
	}
	d.Thing = t
	return d, r.Err()
}

// readARGB reads n bytes of ARGB pixels and returns RGBA. The first sprite
// with a non-default length fixes the sprite size of the file.
func readARGB(r *binio.Reader, n int, d *Data) ([]byte, error) {
	if n == 0 || n%4 != 0 || n > 1<<20 {
		return nil, fmt.Errorf("invalid pixel data size %d", n)
	}
	size := int(math.Sqrt(float64(n / 4)))
	if size*size*4 != n {
		// v1 allowed shorter buffers, pad them to the default size.
		size = d.SpriteSize
		if n > size*size*4 {
			return nil, fmt.Errorf("invalid pixel data size %d", n)
		}
	}
	d.SpriteSize = size
	src := r.Bytes(n)
	if src == nil {
		return nil, r.Err()
	}
	out := make([]byte, size*size*4)
	for i := 0; i+3 < len(src); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = src[i+1], src[i+2], src[i+3], src[i]
	}
	return out, nil
}

// Encode serializes and compresses data with d.Version.
func Encode(d *Data) ([]byte, error) {
	raw, err := EncodeRaw(d)
	if err != nil {
		return nil, err
	}
	return compress(raw)
}

// EncodeRaw serializes data without compression.
func EncodeRaw(d *Data) ([]byte, error) {
	t := d.Thing
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if len(d.Sprites) < len(t.FrameGroups) {
		return nil, fmt.Errorf("obd: pixels for %d groups, thing has %d", len(d.Sprites), len(t.FrameGroups))
	}
	size := d.SpriteSize
	if size == 0 {
		size = client.DefaultSpriteSize
	}
	w := binio.NewWriter(64 << 10)
	groups := t.FrameGroups
	switch d.Version {
	case Version1:
		w.U16(d.ClientVersion)
		w.UTF(t.Category.String())
		dat.Table(client.MetadataFormat(d.ClientVersion)).WriteProperties(w, &t.Props)
		groups = groups[:1]
	case Version2, Version3:
		w.U16(uint16(d.Version))
		w.U16(d.ClientVersion)
		w.U8(uint8(t.Category))
		pos := w.Len()
		w.U32(0)
		dat.OBDTable.WriteProperties(w, &t.Props)
		w.PutU32At(pos, uint32(w.Len()))
		if d.Version == Version2 {
			groups = groups[:1]
		} else if t.Category == thing.CategoryOutfit {
			w.U8(uint8(len(groups)))
		}
	default:
		return nil, fmt.Errorf("obd: unknown version %d", d.Version)
	}
	for gi, g := range groups {
		if d.Version == Version3 && t.Category == thing.CategoryOutfit {
			typ := uint8(gi)
			if len(groups) < 2 {
				typ = 1
			}
			w.U8(typ)
		}
		if err := dat.WriteLayout(w, g, true, d.Version != Version1); err != nil {
			return nil, err
		}
		sprites := d.Sprites[gi]
		if len(sprites) != len(g.Sprites) {
			return nil, fmt.Errorf("obd: group %d has %d sprites, layout needs %d", gi, len(sprites), len(g.Sprites))
		}
		for i, s := range sprites {
			if len(s.Pixels) != size*size*4 {
				return nil, fmt.Errorf("obd: group %d sprite %d has %d bytes, want %d", gi, i, len(s.Pixels), size*size*4)
			}
			w.U32(g.Sprites[i])
			if d.Version != Version2 {
				w.U32(uint32(len(s.Pixels)))
			}
			writeARGB(w, s.Pixels)
		}
	}
	return w.Bytes(), nil
}

func writeARGB(w *binio.Writer, rgba []byte) {
	out := make([]byte, len(rgba))
	for i := 0; i+3 < len(rgba); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = rgba[i+3], rgba[i], rgba[i+1], rgba[i+2]
	}
	w.Write(out)
}

func decompress(file []byte) ([]byte, error) {
	zr, err := lzma.NewReader(bytes.NewReader(file))
	if err != nil {
		return nil, fmt.Errorf("obd: not an LZMA stream: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(zr, maxDecompressed+1))
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
	cfg := lzma.WriterConfig{SizeInHeader: true, Size: int64(len(raw)), EOSMarker: false}
	zw, err := cfg.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
