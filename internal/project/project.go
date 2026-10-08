// Package project holds an editing session over one client (dat + spr):
// thing and sprite storage, undo/redo and compilation.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/dat"
	"github.com/nekiro/ots-creator/internal/spr"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ErrUnknownVersion is returned by Open when signatures match no known
// client and no version was given.
var ErrUnknownVersion = errors.New("unknown client version")

// Info describes the loaded client.
type Info struct {
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
	datPath  string
	sprPath  string
	things   *dat.File
	sprites  *spriteStore
	history  history
	// unsaved is set for a client that was never written to disk; edits
	// are tracked by the history instead.
	unsaved bool
	delta   deltaSet
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
func Open(o OpenOptions) (*Project, error) {
	datBytes, err := os.ReadFile(o.DatPath)
	if err != nil {
		return nil, err
	}
	sprBytes, err := os.ReadFile(o.SprPath)
	if err != nil {
		return nil, err
	}
	if len(datBytes) < 4 || len(sprBytes) < 4 {
		return nil, fmt.Errorf("dat or spr file is too small")
	}
	datSig := le32(datBytes)
	sprSig := le32(sprBytes)
	var v client.Version
	if o.Version != nil {
		v = *o.Version
	} else {
		var ok bool
		v, ok = client.FindBySignatures(datSig, sprSig)
		if !ok {
			return nil, fmt.Errorf("%w: dat 0x%X, spr 0x%X", ErrUnknownVersion, datSig, sprSig)
		}
	}
	f := o.Features
	f.ApplyVersionDefaults(v.Value)
	things, err := dat.Decode(datBytes, dat.Options{Format: client.MetadataFormat(v.Value), Features: f})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(o.DatPath), err)
	}
	sf, err := spr.Open(sprBytes, spr.Options{Extended: f.Extended, Transparent: f.Transparency, SpriteSize: f.SpriteSize})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(o.SprPath), err)
	}
	return &Project{
		version:  v,
		features: f,
		datPath:  o.DatPath,
		sprPath:  o.SprPath,
		things:   things,
		sprites:  newSpriteStore(sf, f.SpriteSize, f.Transparency),
	}, nil
}

// New creates an empty client with one item, outfit, effect and missile,
// like ObjectBuilder does.
func New(v client.Version, f client.Features) *Project {
	f.ApplyVersionDefaults(v.Value)
	things := dat.NewFile(v.DatSignature)
	for _, c := range thing.Categories {
		things.Things[c] = []*thing.Thing{thing.New(c.MinID(), c)}
	}
	return &Project{
		version:  v,
		features: f,
		things:   things,
		sprites:  newSpriteStore(nil, f.SpriteSize, f.Transparency),
		unsaved:  true,
	}
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// Info returns a snapshot of the project state.
func (p *Project) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.infoLocked()
}

func (p *Project) infoLocked() Info {
	return Info{
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
		FormatLabel: dat.Table(client.MetadataFormat(p.version.Value)).Name(),
		Supported:   dat.Table(client.MetadataFormat(p.version.Value)).Supported(),
	}
}

// SpriteSize returns the sprite edge length in pixels.
func (p *Project) SpriteSize() int { return p.features.SpriteSize }

// CompileOptions select the output of Compile.
type CompileOptions struct {
	DatPath  string
	SprPath  string
	Version  client.Version
	Features client.Features
	// WriteOTFI also writes an OTClient feature file next to the dat.
	WriteOTFI bool
}

// Compile writes the project to disk. Files are written to temporary files
// first and renamed, so a failure never leaves broken files behind.
// Properties unsupported by the target format are dropped; use Warnings to
// list them beforehand.
func (p *Project) Compile(o CompileOptions) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	f := o.Features
	f.ApplyVersionDefaults(o.Version.Value)
	if f.SpriteSize != p.features.SpriteSize {
		return fmt.Errorf("changing sprite size from %d to %d is not supported", p.features.SpriteSize, f.SpriteSize)
	}
	file := &dat.File{Signature: o.Version.DatSignature, Things: p.things.Things}
	datBytes, err := dat.Encode(file, dat.Options{Format: client.MetadataFormat(o.Version.Value), Features: f})
	if err != nil {
		return err
	}
	var sprBuf bytes.Buffer
	src := p.sprites.source(f.Transparency)
	if size, err := encodedSize(src, f.Extended); err == nil {
		sprBuf.Grow(size) // one allocation instead of repeated doubling
	}
	if err := spr.Encode(&sprBuf, o.Version.SprSignature, src, spr.Options{Extended: f.Extended, Transparent: f.Transparency, SpriteSize: f.SpriteSize}); err != nil {
		return err
	}
	if err := writeAtomic(o.DatPath, datBytes); err != nil {
		return err
	}
	if err := writeAtomic(o.SprPath, sprBuf.Bytes()); err != nil {
		return err
	}
	if o.WriteOTFI {
		otfi := client.OTFI{Features: f, MetadataFile: filepath.Base(o.DatPath), SpritesFile: filepath.Base(o.SprPath)}
		base := filepath.Join(filepath.Dir(o.DatPath), trimExt(filepath.Base(o.DatPath))+".otfi")
		if err := writeAtomic(base, otfi.Marshal()); err != nil {
			return err
		}
	}
	// Reload sprites from the new file so the in-memory overlay is dropped.
	sf, err := spr.Open(sprBuf.Bytes(), spr.Options{Extended: f.Extended, Transparent: f.Transparency, SpriteSize: f.SpriteSize})
	if err != nil {
		return fmt.Errorf("reopen compiled spr: %w", err)
	}
	p.sprites = newSpriteStore(sf, f.SpriteSize, f.Transparency)
	p.version, p.features = o.Version, f
	p.datPath, p.sprPath = o.DatPath, o.SprPath
	p.things.Signature = o.Version.DatSignature
	p.unsaved = false
	p.history.clear()
	// Version and features (transparency) can change how sprites decode.
	p.delta.everything()
	return nil
}

// Warnings lists things whose properties would be lost when compiling to
// the given version.
func (p *Project) Warnings(v client.Version) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	table := dat.Table(client.MetadataFormat(v.Value))
	var out []string
	for _, c := range thing.Categories {
		for _, t := range p.things.Things[c] {
			if u := table.Unsupported(&t.Props); len(u) > 0 {
				out = append(out, fmt.Sprintf("%s %d: %v", c, t.ID, u))
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
	n := src.Count()
	size := header + int(n)*4
	for id := uint32(1); id <= n; id++ {
		c, err := src.Compressed(id)
		if err != nil {
			return 0, err
		}
		if len(c) > 0 {
			size += 5 + len(c)
		}
	}
	return size, nil
}

func trimExt(name string) string { return name[:len(name)-len(filepath.Ext(name))] }

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
