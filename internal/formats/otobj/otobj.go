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

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/formats/obd"
	"github.com/nekiro/ots-creator/internal/core/thing"
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
func Encode(data *obd.Data, options Options) ([]byte, error) {
	object := data.Thing
	if err := object.Validate(); err != nil {
		return nil, err
	}
	if len(data.Sprites) != len(object.FrameGroups) {
		return nil, fmt.Errorf("otobj: %d sprite lists for %d frame groups", len(data.Sprites), len(object.FrameGroups))
	}
	flags, err := encodeFlags(&object.Props)
	if err != nil {
		return nil, err
	}
	meta := manifest{
		Format:      formatName,
		Version:     Version,
		Generator:   options.Generator,
		Category:    object.Category.String(),
		SpriteSize:  data.SpriteSize,
		Name:        object.Name,
		Description: object.Description,
		Flags:       flags,
		NpcSales:    object.NpcSales,
	}
	if data.ClientVersion != 0 || object.ID != 0 {
		meta.Source = &source{Client: data.ClientVersion, ID: object.ID}
	}
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for groupIndex, frameGroup := range object.FrameGroups {
		name := "sprites/" + groupType(object, groupIndex) + ".png"
		meta.FrameGroups = append(meta.FrameGroups, encodeGroup(object, groupIndex, name))
		if len(data.Sprites[groupIndex]) != len(frameGroup.Sprites) {
			return nil, fmt.Errorf("otobj: frame group %d has %d slots and %d sprites", groupIndex, len(frameGroup.Sprites), len(data.Sprites[groupIndex]))
		}
		sprites := data.Sprites[groupIndex]
		sheet := imaging.Sheet(frameGroup, data.SpriteSize, color.NRGBA{}, func(slot int) []byte { return sprites[slot].Pixels })
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return nil, err
		}
		if err := writePNG(entry, sheet); err != nil {
			return nil, err
		}
	}
	manifestJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestEntry, err := archive.CreateHeader(&zip.FileHeader{Name: manifestName, Method: zip.Deflate})
	if err != nil {
		return nil, err
	}
	if _, err := manifestEntry.Write(manifestJSON); err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func groupType(object *thing.Thing, groupIndex int) string {
	if object.Category == thing.CategoryOutfit && len(object.FrameGroups) > 1 && groupIndex == 1 {
		return "walking"
	}
	return "idle"
}

func encodeGroup(object *thing.Thing, groupIndex int, sheet string) group {
	frameGroup := object.FrameGroups[groupIndex]
	out := group{
		Type: groupType(object, groupIndex), Width: frameGroup.Width, Height: frameGroup.Height, ExactSize: frameGroup.ExactSize, Layers: frameGroup.Layers,
		PatternX: frameGroup.PatternX, PatternY: frameGroup.PatternY, PatternZ: frameGroup.PatternZ, Frames: frameGroup.Frames, Sheet: sheet,
	}
	if frameGroup.Frames > 1 {
		mode := "async"
		if frameGroup.Mode == thing.AnimationSync {
			mode = "sync"
		}
		out.Animation = &animation{Mode: mode, LoopCount: frameGroup.LoopCount, StartFrame: frameGroup.StartFrame, Durations: frameGroup.Durations}
	}
	return out
}

// encodeFlags writes the properties that differ from their defaults.
func encodeFlags(props *thing.Properties) (json.RawMessage, error) {
	raw, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	var all map[string]any
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	for key, value := range all {
		if isZero(value) {
			delete(all, key)
		}
	}
	if !props.IsMarket {
		delete(all, "market")
	}
	if !props.HasBones {
		delete(all, "bones")
	}
	return json.Marshal(all) // map keys are sorted: stable, diffable output
}

// isZero reports whether a decoded JSON value is false, 0, "" or only made
// of such values.
func isZero(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case bool:
		return !typed
	case float64:
		return typed == 0
	case string:
		return typed == ""
	case []any:
		for _, element := range typed {
			if !isZero(element) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, element := range typed {
			if !isZero(element) {
				return false
			}
		}
		return true
	}
	return false
}

// writePNG writes a sheet, with a palette when it has at most 256 colors.
func writePNG(dest io.Writer, img *image.NRGBA) error {
	var out image.Image = img
	if palettedImg := paletted(img); palettedImg != nil {
		out = palettedImg
	}
	return (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(dest, out)
}

// paletted returns img with a palette, or nil when it has more than 256
// colors. Fully transparent pixels share one entry.
func paletted(img *image.NRGBA) *image.Paletted {
	index := map[[4]byte]uint8{}
	var palette color.Palette
	pixels := img.Pix
	key := func(i int) [4]byte {
		if pixels[i+3] == 0 {
			return [4]byte{}
		}
		return [4]byte{pixels[i], pixels[i+1], pixels[i+2], pixels[i+3]}
	}
	for i := 0; i < len(pixels); i += 4 {
		pixelKey := key(i)
		if _, ok := index[pixelKey]; ok {
			continue
		}
		if len(palette) == 256 {
			return nil
		}
		index[pixelKey] = uint8(len(palette))
		palette = append(palette, color.NRGBA{pixelKey[0], pixelKey[1], pixelKey[2], pixelKey[3]})
	}
	out := image.NewPaletted(img.Rect, palette)
	for i, j := 0, 0; i < len(pixels); i, j = i+4, j+1 {
		out.Pix[j] = index[key(i)]
	}
	return out
}

// Decode reads an OTOBJ file. Sprite ids of the returned thing are 0.
func Decode(file []byte) (*obd.Data, error) {
	archive, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotOTOBJ, err)
	}
	manifestJSON, err := readEntry(archive, manifestName)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotOTOBJ, err)
	}
	var meta manifest
	if err := json.Unmarshal(manifestJSON, &meta); err != nil {
		return nil, fmt.Errorf("otobj: manifest: %w", err)
	}
	if meta.Format != formatName {
		return nil, ErrNotOTOBJ
	}
	if meta.Version < 1 || meta.Version > Version {
		return nil, fmt.Errorf("otobj: version %d is not supported (up to %d), update OTS Creator", meta.Version, Version)
	}
	category, err := thing.ParseCategory(meta.Category)
	if err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	if meta.SpriteSize != 32 && meta.SpriteSize != 64 {
		return nil, fmt.Errorf("otobj: sprite size %d is not supported", meta.SpriteSize)
	}
	object := &thing.Thing{Category: category, Name: meta.Name, Description: meta.Description, NpcSales: meta.NpcSales}
	if len(meta.Flags) > 0 && string(meta.Flags) != "null" {
		if err := json.Unmarshal(meta.Flags, &object.Props); err != nil {
			return nil, fmt.Errorf("otobj: flags: %w", err)
		}
	}
	data := &obd.Data{Version: Version, Thing: object, SpriteSize: meta.SpriteSize}
	if meta.Source != nil {
		data.ClientVersion, object.ID = meta.Source.Client, meta.Source.ID
	}
	for groupIndex, groupManifest := range meta.FrameGroups {
		frameGroup, err := decodeGroup(groupManifest, groupIndex, category.DefaultDuration())
		if err != nil {
			return nil, err
		}
		width, height := frameGroup.SheetSize(meta.SpriteSize)
		sheet, err := readSheet(archive, groupManifest.Sheet, width, height)
		if err != nil {
			return nil, err
		}
		pixels, err := imaging.SliceSheetExact(sheet, frameGroup, meta.SpriteSize)
		if err != nil {
			return nil, fmt.Errorf("otobj: %s: %w", groupManifest.Sheet, err)
		}
		sprites := make([]obd.Sprite, len(pixels))
		for i, spritePixels := range pixels {
			sprites[i] = obd.Sprite{Pixels: spritePixels}
		}
		frameGroup.Sprites = make([]uint32, len(pixels))
		object.FrameGroups = append(object.FrameGroups, frameGroup)
		data.Sprites = append(data.Sprites, sprites)
	}
	if err := object.Validate(); err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	return data, nil
}

func decodeGroup(groupManifest group, groupIndex int, defaultDuration uint32) (*thing.FrameGroup, error) {
	frameGroup := &thing.FrameGroup{
		Width: groupManifest.Width, Height: groupManifest.Height, ExactSize: groupManifest.ExactSize, Layers: groupManifest.Layers,
		PatternX: groupManifest.PatternX, PatternY: groupManifest.PatternY, PatternZ: groupManifest.PatternZ, Frames: groupManifest.Frames,
	}
	switch groupManifest.Type {
	case "idle", "":
	case "walking":
		frameGroup.Type = thing.FrameGroupWalking
	default:
		return nil, fmt.Errorf("otobj: frame group %d: unknown type %q", groupIndex, groupManifest.Type)
	}
	if frameGroup.Width < 1 || frameGroup.Height < 1 || frameGroup.Width > 8 || frameGroup.Height > 8 || frameGroup.Layers < 1 || frameGroup.PatternX < 1 || frameGroup.PatternY < 1 || frameGroup.PatternZ < 1 || frameGroup.Frames < 1 {
		return nil, fmt.Errorf("otobj: frame group %d has an invalid layout", groupIndex)
	}
	if total := frameGroup.TotalSprites(); total > maxSprites {
		return nil, fmt.Errorf("otobj: frame group %d has %d sprites (at most %d)", groupIndex, total, maxSprites)
	}
	if anim := groupManifest.Animation; anim != nil && frameGroup.Frames > 1 {
		if anim.Mode == "sync" {
			frameGroup.Mode = thing.AnimationSync
		}
		frameGroup.LoopCount, frameGroup.StartFrame, frameGroup.Durations = anim.LoopCount, anim.StartFrame, anim.Durations
	}
	if frameGroup.Frames > 1 && len(frameGroup.Durations) != int(frameGroup.Frames) {
		frameGroup.Durations = nil
		frameGroup.EnsureDurations(defaultDuration)
	}
	return frameGroup, nil
}

// readSheet reads a PNG sheet that must be w x h pixels. The size is checked
// before decoding, so a bad file cannot make it allocate a huge image.
func readSheet(archive *zip.Reader, name string, width, height int) (*image.NRGBA, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".png") || path.Clean(name) != name || strings.HasPrefix(name, "/") {
		return nil, fmt.Errorf("otobj: bad sheet name %q", name)
	}
	data, err := readEntry(archive, name)
	if err != nil {
		return nil, fmt.Errorf("otobj: %w", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("otobj: %s: %w", name, err)
	}
	if config.Width != width || config.Height != height {
		return nil, fmt.Errorf("otobj: %s is %dx%d, its frame group needs %dx%d", name, config.Width, config.Height, width, height)
	}
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("otobj: %s: %w", name, err)
	}
	return img, nil
}

func readEntry(archive *zip.Reader, name string) ([]byte, error) {
	entry, err := archive.Open(name)
	if err != nil {
		return nil, fmt.Errorf("missing %s", name)
	}
	defer entry.Close()
	data, err := io.ReadAll(io.LimitReader(entry, maxEntry+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(data) > maxEntry {
		return nil, fmt.Errorf("%s is larger than %d MiB", name, maxEntry>>20)
	}
	return data, nil
}
