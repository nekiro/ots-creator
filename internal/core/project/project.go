// Package project holds an editing session over one client (dat + spr):
// thing and sprite storage, undo/redo and compilation.
package project

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/assets"
	"github.com/nekiro/ots-creator/internal/formats/dat"
	"github.com/nekiro/ots-creator/internal/formats/spr"
)

// ErrUnknownVersion is returned by Open when signatures match no known
// client and no version was given.
var ErrUnknownVersion = errors.New("unknown client version")

// Info describes the loaded client.
type Info struct {
	Format      Format          `json:"format"`
	Version     client.Version  `json:"version"`
	Features    client.Features `json:"features"`
	DatPath     string          `json:"datPath"`
	SprPath     string          `json:"sprPath"`
	Counts      Counts          `json:"counts"`
	Changed     bool            `json:"changed"`
	CanUndo     bool            `json:"canUndo"`
	CanRedo     bool            `json:"canRedo"`
	FormatLabel string          `json:"formatLabel"`
	// Supported lists property names the current format can store.
	Supported []string `json:"supported"`
	// ItemsOTB is the attached items.otb, kept in sync when compiling.
	ItemsOTB *OTBInfo `json:"itemsOtb"`
}

// Counts holds the highest id of each category and the sprite count.
type Counts struct {
	Items    uint32 `json:"items"`
	Outfits  uint32 `json:"outfits"`
	Effects  uint32 `json:"effects"`
	Missiles uint32 `json:"missiles"`
	Sprites  uint32 `json:"sprites"`
}

// Project is safe for concurrent use.
type Project struct {
	mu       sync.RWMutex
	version  client.Version
	features client.Features
	format   Format
	// datPath is the asset folder for FormatAssets.
	datPath string
	sprPath string
	things  *dat.File
	sprites *spriteStore
	history history
	// unsaved is set for a client that was never written to disk; edits
	// are tracked by the history instead.
	unsaved bool
	delta   deltaSet
	assets  *assetState // FormatAssets only
	otb     *itemsOTB   // attached items.otb, nil when none
	// base holds the items as they are on disk and moves maps their ids
	// to the ids they were moved to since (see otb.go).
	base  []*thing.Thing
	moves map[uint32]uint32
}

// OpenOptions control how a client is opened.
type OpenOptions struct {
	DatPath string
	SprPath string
	// Version overrides signature detection when not nil.
	Version *client.Version
	// Features are combined with the defaults of the version.
	Features client.Features
}

// Open loads a dat/spr pair.
func Open(options OpenOptions) (*Project, error) {
	datBytes, err := os.ReadFile(options.DatPath)
	if err != nil {
		return nil, err
	}
	sprBytes, err := os.ReadFile(options.SprPath)
	if err != nil {
		return nil, err
	}
	if len(datBytes) < 4 || len(sprBytes) < 4 {
		return nil, fmt.Errorf("dat or spr file is too small")
	}
	datSignature := binary.LittleEndian.Uint32(datBytes)
	sprSignature := binary.LittleEndian.Uint32(sprBytes)
	var version client.Version
	if options.Version != nil {
		version = *options.Version
	} else {
		var ok bool
		version, ok = client.FindBySignatures(datSignature, sprSignature)
		if !ok {
			return nil, fmt.Errorf("%w: dat 0x%X, spr 0x%X", ErrUnknownVersion, datSignature, sprSignature)
		}
	}
	features := options.Features
	features.ApplyVersionDefaults(version.Value)
	things, err := dat.Decode(datBytes, dat.Options{Format: client.MetadataFormat(version.Value), Features: features})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(options.DatPath), err)
	}
	spriteFile, err := spr.Open(sprBytes, spr.Options{Extended: features.Extended, Transparent: features.Transparency, SpriteSize: features.SpriteSize})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(options.SprPath), err)
	}
	project := &Project{
		version:  version,
		features: features,
		format:   FormatDat,
		datPath:  options.DatPath,
		sprPath:  options.SprPath,
		things:   things,
		sprites:  newSpriteStore(sprFile(spriteFile), features.SpriteSize, features.Transparency),
	}
	project.resetBase()
	return project, nil
}

// New creates an empty client with one item, outfit, effect and missile,
// like ObjectBuilder does.
func New(version client.Version, features client.Features) *Project {
	features.ApplyVersionDefaults(version.Value)
	things := dat.NewFile(version.DatSignature)
	for _, category := range thing.Categories {
		things.Things[category] = []*thing.Thing{thing.New(category.MinID(), category)}
	}
	return &Project{
		version:  version,
		features: features,
		format:   FormatDat,
		things:   things,
		sprites:  newSpriteStore(nil, features.SpriteSize, features.Transparency),
		unsaved:  true,
	}
}

// Info returns a snapshot of the project state.
func (p *Project) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.infoLocked()
}

func (p *Project) infoLocked() Info {
	var otb *OTBInfo
	if p.otb != nil {
		info := p.otb.info
		otb = &info
	}
	label, supported := dat.Table(client.MetadataFormat(p.version.Value)).Name(), dat.Table(client.MetadataFormat(p.version.Value)).Supported()
	if p.format == FormatAssets {
		label, supported = "Protobuf appearances", assets.Supported()
	}
	return Info{
		Format:   p.format,
		Version:  p.version,
		Features: p.features,
		DatPath:  p.datPath,
		SprPath:  p.sprPath,
		Counts: Counts{
			Items:    p.things.MaxID(thing.CategoryItem),
			Outfits:  p.things.MaxID(thing.CategoryOutfit),
			Effects:  p.things.MaxID(thing.CategoryEffect),
			Missiles: p.things.MaxID(thing.CategoryMissile),
			Sprites:  p.sprites.count(),
		},
		Changed:     p.unsaved || !p.history.atClean(),
		CanUndo:     p.history.canUndo(),
		CanRedo:     p.history.canRedo(),
		FormatLabel: label,
		Supported:   supported,
		ItemsOTB:    otb,
	}
}

// SpriteSize returns the sprite edge length in pixels.
func (p *Project) SpriteSize() int { return p.features.SpriteSize }

// CompileOptions select the output of Compile.
type CompileOptions struct {
	// Format is the output format; empty means FormatDat. For FormatAssets
	// DatPath is the output folder and the other fields are ignored.
	Format   Format
	DatPath  string
	SprPath  string
	Version  client.Version
	Features client.Features
	// WriteOTFI also writes an OTClient feature file next to the dat.
	WriteOTFI bool
	// Progress, when set, is called with the work done so far, possibly
	// from several goroutines and very often.
	Progress func(done, total int)
	// Stats, when set, receives what an asset compilation did to the sheets
	// and what happened to the attached items.otb.
	Stats *CompileStats
}

// CompileStats reports the sheets of an asset compilation.
type CompileStats struct {
	// SheetsRemoved counts sheets of the folder that no longer hold a used
	// sprite as they were (dropped, or repacked into new sheets).
	SheetsRemoved int `json:"sheetsRemoved"`
	// Freed is the size of the removed sheets minus the new ones, in bytes.
	Freed int64 `json:"freed"`
	// OTB reports the update of the attached items.otb (nil when none).
	OTB *OTBReport `json:"otb"`
}

// Compile writes the project to disk. Files are written to temporary files
// first and renamed, so a failure never leaves broken files behind.
// Properties unsupported by the target format are dropped; use Warnings to
// list them beforehand.
func (p *Project) Compile(options CompileOptions) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// The items.otb is synced against the items on disk, before they change.
	otbFile, otbData, otbRep := p.prepareOTB()
	if options.Stats != nil {
		options.Stats.OTB = otbRep
	}
	if options.Format == FormatAssets {
		if err := p.compileAssets(options.DatPath, options.Progress, options.Stats); err != nil {
			return err
		}
		p.writeOTB(otbFile, otbData, otbRep)
		p.saved()
		return nil
	}
	features := options.Features
	features.ApplyVersionDefaults(options.Version.Value)
	if features.SpriteSize != p.features.SpriteSize {
		return fmt.Errorf("changing sprite size from %d to %d is not supported", p.features.SpriteSize, features.SpriteSize)
	}
	file := &dat.File{Signature: options.Version.DatSignature, Things: p.things.Things}
	datBytes, err := dat.Encode(file, dat.Options{Format: client.MetadataFormat(options.Version.Value), Features: features})
	if err != nil {
		return err
	}
	var sprBuf bytes.Buffer
	src := p.sprites.source(features.Transparency)
	// Every sprite is read three times: for the size and by both passes of
	// spr.Encode.
	src = countingSource{src, newCounter(3*int(src.Count()), options.Progress)}
	if size, err := encodedSize(src, features.Extended); err == nil {
		sprBuf.Grow(size) // one allocation instead of repeated doubling
	}
	if err := spr.Encode(&sprBuf, options.Version.SprSignature, src, spr.Options{Extended: features.Extended, Transparent: features.Transparency, SpriteSize: features.SpriteSize}); err != nil {
		return err
	}
	if err := fsutil.WriteAtomic(options.DatPath, datBytes); err != nil {
		return err
	}
	if err := fsutil.WriteAtomic(options.SprPath, sprBuf.Bytes()); err != nil {
		return err
	}
	if options.WriteOTFI {
		otfi := client.OTFI{Features: features, MetadataFile: filepath.Base(options.DatPath), SpritesFile: filepath.Base(options.SprPath)}
		base := filepath.Join(filepath.Dir(options.DatPath), fsutil.TrimExt(filepath.Base(options.DatPath))+".otfi")
		if err := fsutil.WriteAtomic(base, otfi.Marshal()); err != nil {
			return err
		}
	}
	p.writeOTB(otbFile, otbData, otbRep)
	// Reload sprites from the new file so the in-memory overlay is dropped.
	spriteFile, err := spr.Open(sprBuf.Bytes(), spr.Options{Extended: features.Extended, Transparent: features.Transparency, SpriteSize: features.SpriteSize})
	if err != nil {
		return fmt.Errorf("reopen compiled spr: %w", err)
	}
	p.sprites = newSpriteStore(sprFile(spriteFile), features.SpriteSize, features.Transparency)
	p.format, p.assets = FormatDat, nil
	p.version, p.features = options.Version, features
	p.datPath, p.sprPath = options.DatPath, options.SprPath
	p.things.Signature = options.Version.DatSignature
	p.saved()
	return nil
}

// saved resets change tracking after the files were written.
func (p *Project) saved() {
	p.unsaved = false
	p.history.clear()
	p.resetBase()
	// Version and features (transparency) can change how sprites decode.
	p.delta.everything()
}

// Warnings lists things whose properties would be lost when compiling to
// the given format and version.
func (p *Project) Warnings(format Format, version client.Version) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if format == FormatAssets {
		return p.assetWarnings()
	}
	table := dat.Table(client.MetadataFormat(version.Value))
	var out []string
	for _, category := range thing.Categories {
		for _, object := range p.things.Things[category] {
			unsupported := table.Unsupported(&object.Props)
			if len(object.NpcSales) > 0 {
				unsupported = append(unsupported, "npcSales")
			}
			if len(unsupported) > 0 {
				out = append(out, fmt.Sprintf("%s %d: %v", category, object.ID, unsupported))
			}
		}
	}
	return out
}

// encodedSize returns the size of the spr file Encode writes for src.
func encodedSize(src spr.Source, extended bool) (int, error) {
	header := 6
	if extended {
		header = 8
	}
	count := src.Count()
	size := header + int(count)*4
	for id := uint32(1); id <= count; id++ {
		compressed, err := src.Compressed(id)
		if err != nil {
			return 0, err
		}
		if len(compressed) > 0 {
			size += 5 + len(compressed)
		}
	}
	return size, nil
}
