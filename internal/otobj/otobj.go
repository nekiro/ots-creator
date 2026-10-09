// Package otobj reads and writes OTOBJ files (*.otobj), the native object
// format of OTS Creator: a ZIP archive with a JSON manifest and one PNG
// sprite sheet per frame group. See docs/otobj.md for the specification.
//
// The in-memory form is obd.Data, shared with the OBD codec, so both
// formats import and export through the same code.
package otobj

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"path"
	"strings"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Version is the format version this package writes and the highest it
// reads.
const Version = 1

// Ext is the file extension, with the dot.
const Ext = ".otobj"

const (
	formatName   = "otobj"
	manifestName = "manifest.json"
	// maxEntry limits one decompressed entry (64 MiB).
	maxEntry = 64 << 20
	// maxSprites limits the sprites of one frame group.
	maxSprites = 4096
)

// ErrNotOTOBJ is returned for files that are not OTOBJ archives.
var ErrNotOTOBJ = errors.New("not an otobj file")

type manifest struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Generator   string          `json:"generator,omitempty"`
	Category    string          `json:"category"`
	SpriteSize  int             `json:"spriteSize"`
	Source      *source         `json:"source,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Flags       json.RawMessage `json:"flags"`
	NpcSales    []thing.NpcSale `json:"npcSales,omitempty"`
	FrameGroups []group         `json:"frameGroups"`
}

type source struct {
	Client uint16 `json:"client,omitempty"`
	ID     uint32 `json:"id,omitempty"`
}

type group struct {
	Type      string     `json:"type"`
	Width     uint8      `json:"width"`
	Height    uint8      `json:"height"`
	ExactSize uint8      `json:"exactSize"`
	Layers    uint8      `json:"layers"`
	PatternX  uint8      `json:"patternX"`
	PatternY  uint8      `json:"patternY"`
	PatternZ  uint8      `json:"patternZ"`
	Frames    uint8      `json:"frames"`
	Animation *animation `json:"animation,omitempty"`
	Sheet     string     `json:"sheet"`
}

type animation struct {
	Mode       string                `json:"mode"`
	LoopCount  int32                 `json:"loopCount"`
	StartFrame int8                  `json:"startFrame"`
	Durations  []thing.FrameDuration `json:"durations"`
}

// Options control Encode.
type Options struct {
	// Generator names the writing tool, e.g. "OTS Creator 1.4.0".
	Generator string
}

// Encode writes d as an OTOBJ file. Sprite ids in d are only kept as the
// source of the object.
func Encode(d *obd.Data, o Options) ([]byte, error) {
	t := d.Thing
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if len(d.Sprites) != len(t.FrameGroups) {
		return nil, fmt.Errorf("otobj: %d sprite lists for %d frame groups", len(d.Sprites), len(t.FrameGroups))
	}
	flags, err := encodeFlags(&t.Props)
	if err != nil {
		return nil, err
	}
	m := manifest{
		Format:      formatName,
		Version:     Version,
		Generator:   o.Generator,
		Category:    t.Category.String(),
		SpriteSize:  d.SpriteSize,
		Name:        t.Name,
		Description: t.Description,
		Flags:       flags,
		NpcSales:    t.NpcSales,
	}
	if d.ClientVersion != 0 || t.ID != 0 {
		m.Source = &source{Client: d.ClientVersion, ID: t.ID}
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for gi, g := range t.FrameGroups {
		name := "sprites/" + groupType(t, gi) + ".png"
		m.FrameGroups = append(m.FrameGroups, encodeGroup(t, gi, name))
		if len(d.Sprites[gi]) != len(g.Sprites) {
			return nil, fmt.Errorf("otobj: frame group %d has %d slots and %d sprites", gi, len(g.Sprites), len(d.Sprites[gi]))
		}
		sprites := d.Sprites[gi]
		sheet := imaging.Sheet(g, d.SpriteSize, color.NRGBA{}, func(slot int) []byte { return sprites[slot].Pixels })
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return nil, err
		}
		if err := writePNG(w, sheet); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	w, err := z.CreateHeader(&zip.FileHeader{Name: manifestName, Method: zip.Deflate})
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func groupType(t *thing.Thing, i int) string {
	if t.Category == thing.CategoryOutfit && len(t.FrameGroups) > 1 && i == 1 {
		return "walking"
	}
	return "idle"
}

func encodeGroup(t *thing.Thing, i int, sheet string) group {
	g := t.FrameGroups[i]
	out := group{
		Type: groupType(t, i), Width: g.Width, Height: g.Height, ExactSize: g.ExactSize, Layers: g.Layers,
		PatternX: g.PatternX, PatternY: g.PatternY, PatternZ: g.PatternZ, Frames: g.Frames, Sheet: sheet,
	}
	if g.Frames > 1 {
		mode := "async"
		if g.Mode == thing.AnimationSync {
			mode = "sync"
		}
		out.Animation = &animation{Mode: mode, LoopCount: g.LoopCount, StartFrame: g.StartFrame, Durations: g.Durations}
	}
	return out
}

// encodeFlags writes the properties that differ from their defaults.
func encodeFlags(p *thing.Properties) (json.RawMessage, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var all map[string]any
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	for k, v := range all {
		if isZero(v) {
			delete(all, k)
		}
	}
	if !p.IsMarket {
		delete(all, "market")
	}
	if !p.HasBones {
		delete(all, "bones")
	}
	return json.Marshal(all) // map keys are sorted: stable, diffable output
}

// isZero reports whether a decoded JSON value is false, 0, "" or only made
// of such values.
func isZero(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case float64:
		return x == 0
	case string:
		return x == ""
	case []any:
		for _, e := range x {
			if !isZero(e) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, e := range x {
			if !isZero(e) {
				return false
			}
		}
		return true
	}
	return false
}

// writePNG writes a sheet, with a palette when it has at most 256 colors.
func writePNG(w io.Writer, img *image.NRGBA) error {
	var out image.Image = img
	if pal := paletted(img); pal != nil {
		out = pal
	}
	return (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(w, out)
}

// paletted returns img with a palette, or nil when it has more than 256
// colors. Fully transparent pixels share one entry.
func paletted(img *image.NRGBA) *image.Paletted {
	index := map[[4]byte]uint8{}
	var pal color.Palette
	pix := img.Pix
	key := func(i int) [4]byte {
		if pix[i+3] == 0 {
			return [4]byte{}
		}
		return [4]byte{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	for i := 0; i < len(pix); i += 4 {
		k := key(i)
		if _, ok := index[k]; ok {
			continue
		}
		if len(pal) == 256 {
			return nil
		}
		index[k] = uint8(len(pal))
		pal = append(pal, color.NRGBA{k[0], k[1], k[2], k[3]})
	}
	out := image.NewPaletted(img.Rect, pal)
	for i, j := 0, 0; i < len(pix); i, j = i+4, j+1 {
		out.Pix[j] = index[key(i)]
	}
	return out
}

// Decode reads an OTOBJ file. Sprite ids of the returned thing are 0.
func Decode(file []byte) (*obd.Data, error) {
	z, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotOTOBJ, err)
	}
	data, err := readEntry(z, manifestName)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotOTOBJ, err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("otobj: manifest: %w", err)
	}
	if m.Format != formatName {
		return nil, ErrNotOTOBJ
	}
	if m.Version < 1 || m.Version > Version {
		return nil, fmt.Errorf("otobj: version %d is not supported (up to %d), update OTS Creator", m.Version, Version)
	}
	c, err := thing.ParseCategory(m.Category)
	if err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	if m.SpriteSize != 32 && m.SpriteSize != 64 {
		return nil, fmt.Errorf("otobj: sprite size %d is not supported", m.SpriteSize)
	}
	t := &thing.Thing{Category: c, Name: m.Name, Description: m.Description, NpcSales: m.NpcSales}
	if len(m.Flags) > 0 && string(m.Flags) != "null" {
		if err := json.Unmarshal(m.Flags, &t.Props); err != nil {
			return nil, fmt.Errorf("otobj: flags: %w", err)
		}
	}
	d := &obd.Data{Version: Version, Thing: t, SpriteSize: m.SpriteSize}
	if m.Source != nil {
		d.ClientVersion, t.ID = m.Source.Client, m.Source.ID
	}
	for gi, mg := range m.FrameGroups {
		g, err := decodeGroup(mg, gi, c.DefaultDuration())
		if err != nil {
			return nil, err
		}
		w, h := g.SheetSize(m.SpriteSize)
		sheet, err := readSheet(z, mg.Sheet, w, h)
		if err != nil {
			return nil, err
		}
		pixels, err := imaging.SliceSheetExact(sheet, g, m.SpriteSize)
		if err != nil {
			return nil, fmt.Errorf("otobj: %s: %w", mg.Sheet, err)
		}
		sprites := make([]obd.Sprite, len(pixels))
		for i, px := range pixels {
			sprites[i] = obd.Sprite{Pixels: px}
		}
		g.Sprites = make([]uint32, len(pixels))
		t.FrameGroups = append(t.FrameGroups, g)
		d.Sprites = append(d.Sprites, sprites)
	}
	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	return d, nil
}

func decodeGroup(mg group, i int, defDuration uint32) (*thing.FrameGroup, error) {
	g := &thing.FrameGroup{
		Width: mg.Width, Height: mg.Height, ExactSize: mg.ExactSize, Layers: mg.Layers,
		PatternX: mg.PatternX, PatternY: mg.PatternY, PatternZ: mg.PatternZ, Frames: mg.Frames,
	}
	switch mg.Type {
	case "idle", "":
	case "walking":
		g.Type = thing.FrameGroupWalking
	default:
		return nil, fmt.Errorf("otobj: frame group %d: unknown type %q", i, mg.Type)
	}
	if g.Width < 1 || g.Height < 1 || g.Width > 8 || g.Height > 8 || g.Layers < 1 || g.PatternX < 1 || g.PatternY < 1 || g.PatternZ < 1 || g.Frames < 1 {
		return nil, fmt.Errorf("otobj: frame group %d has an invalid layout", i)
	}
	if n := g.TotalSprites(); n > maxSprites {
		return nil, fmt.Errorf("otobj: frame group %d has %d sprites (at most %d)", i, n, maxSprites)
	}
	if a := mg.Animation; a != nil && g.Frames > 1 {
		if a.Mode == "sync" {
			g.Mode = thing.AnimationSync
		}
		g.LoopCount, g.StartFrame, g.Durations = a.LoopCount, a.StartFrame, a.Durations
	}
	if g.Frames > 1 && len(g.Durations) != int(g.Frames) {
		g.Durations = nil
		g.EnsureDurations(defDuration)
	}
	return g, nil
}

// readSheet reads a PNG sheet that must be w x h pixels. The size is checked
// before decoding, so a bad file cannot make it allocate a huge image.
func readSheet(z *zip.Reader, name string, w, h int) (*image.NRGBA, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".png") || path.Clean(name) != name || strings.HasPrefix(name, "/") {
		return nil, fmt.Errorf("otobj: bad sheet name %q", name)
	}
	data, err := readEntry(z, name)
	if err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("otobj: %s: %w", name, err)
	}
	if cfg.Width != w || cfg.Height != h {
		return nil, fmt.Errorf("otobj: %s is %dx%d, its frame group needs %dx%d", name, cfg.Width, cfg.Height, w, h)
	}
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("otobj: %s: %w", name, err)
	}
	return img, nil
}

func readEntry(z *zip.Reader, name string) ([]byte, error) {
	f, err := z.Open(name)
	if err != nil {
		return nil, fmt.Errorf("missing %s", name)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEntry+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(data) > maxEntry {
		return nil, fmt.Errorf("%s is larger than %d MiB", name, maxEntry>>20)
	}
	return data, nil
}
