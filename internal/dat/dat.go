// Package dat reads and writes Tibia.dat metadata files (7.10 - 13.x
// legacy format).
package dat

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// maxSpritesPerGroup guards against corrupted layouts allocating huge slices.
const maxSpritesPerGroup = 1 << 16

// Options select the encoding of a dat file.
type Options struct {
	// Format is the flag table (1..6), see client.MetadataFormat.
	Format   int
	Features client.Features
}

// OptionsFor returns the options mandated by a client version combined with
// user selected features.
func OptionsFor(version uint16, f client.Features) Options {
	f.ApplyVersionDefaults(version)
	return Options{Format: client.MetadataFormat(version), Features: f}
}

func (o Options) hasPatternZ() bool { return o.Format >= 3 }

// File is a decoded Tibia.dat.
type File struct {
	Signature uint32
	// Things holds things of each category ordered by id, starting at
	// Category.MinID().
	Things map[thing.Category][]*thing.Thing
}

// NewFile returns an empty file.
func NewFile(signature uint32) *File {
	f := &File{Signature: signature, Things: map[thing.Category][]*thing.Thing{}}
	return f
}

// Get returns a thing by category and id.
func (f *File) Get(c thing.Category, id uint32) *thing.Thing {
	list := f.Things[c]
	i := int(id) - int(c.MinID())
	if i < 0 || i >= len(list) {
		return nil
	}
	return list[i]
}

// MaxID returns the highest id of a category, or MinID-1 when it is empty.
func (f *File) MaxID(c thing.Category) uint32 {
	return c.MinID() + uint32(len(f.Things[c])) - 1
}

// Decode parses a Tibia.dat file.
func Decode(data []byte, opts Options) (*File, error) {
	r := binio.NewReader(data)
	f := NewFile(r.U32())
	var counts [4]uint16
	for i := range counts {
		counts[i] = r.U16()
	}
	if r.Err() != nil {
		return nil, fmt.Errorf("dat header: %w", r.Err())
	}
	table := Table(opts.Format)
	for i, c := range thing.Categories {
		maxID := uint32(counts[i])
		if maxID < c.MinID() {
			continue
		}
		list := make([]*thing.Thing, 0, maxID-c.MinID()+1)
		for id := c.MinID(); id <= maxID; id++ {
			t := &thing.Thing{ID: id, Category: c}
			if err := table.ReadProperties(r, &t.Props); err != nil {
				return nil, fmt.Errorf("%s %d: %w", c, id, err)
			}
			if err := readGroups(r, t, opts); err != nil {
				return nil, fmt.Errorf("%s %d: %w", c, id, err)
			}
			list = append(list, t)
		}
		f.Things[c] = list
	}
	return f, nil
}

// ReadThing decodes one thing (flags + texture patterns) from r. OBD v1
// uses this layout.
func ReadThing(r *binio.Reader, t *thing.Thing, opts Options) error {
	if err := Table(opts.Format).ReadProperties(r, &t.Props); err != nil {
		return err
	}
	return readGroups(r, t, opts)
}

func readGroups(r *binio.Reader, t *thing.Thing, opts Options) error {
	groupCount := 1
	withGroups := opts.Features.FrameGroups && t.Category == thing.CategoryOutfit
	if withGroups {
		groupCount = int(r.U8())
	}
	t.FrameGroups = make([]*thing.FrameGroup, 0, groupCount)
	for i := 0; i < groupCount; i++ {
		g := &thing.FrameGroup{}
		if withGroups {
			g.Type = thing.FrameGroupType(r.U8())
		}
		if err := ReadGroup(r, g, opts.hasPatternZ(), opts.Features.ImprovedAnimations, opts.Features.Extended, t.Category.DefaultDuration()); err != nil {
			return err
		}
		t.FrameGroups = append(t.FrameGroups, g)
	}
	return r.Err()
}

// ReadGroup reads one frame group layout followed by its sprite ids.
func ReadGroup(r *binio.Reader, g *thing.FrameGroup, patternZ, durations, extended bool, defDuration uint32) error {
	if err := ReadLayout(r, g, patternZ, durations, defDuration); err != nil {
		return err
	}
	for i := range g.Sprites {
		if extended {
			g.Sprites[i] = r.U32()
		} else {
			g.Sprites[i] = uint32(r.U16())
		}
	}
	return r.Err()
}

// ReadLayout reads the dimensions and animation of a frame group and
// allocates g.Sprites without reading sprite ids. Shared with the OBD codec.
func ReadLayout(r *binio.Reader, g *thing.FrameGroup, patternZ, durations bool, defDuration uint32) error {
	g.Width = r.U8()
	g.Height = r.U8()
	if g.Width > 1 || g.Height > 1 {
		g.ExactSize = r.U8()
	} else {
		g.ExactSize = 32
	}
	g.Layers = r.U8()
	g.PatternX = r.U8()
	g.PatternY = r.U8()
	if patternZ {
		g.PatternZ = r.U8()
	} else {
		g.PatternZ = 1
	}
	g.Frames = r.U8()
	if r.Err() != nil {
		return r.Err()
	}
	if g.Frames > 1 {
		if durations {
			g.Mode = thing.AnimationMode(r.U8())
			g.LoopCount = r.I32()
			g.StartFrame = r.I8()
			g.Durations = make([]thing.FrameDuration, g.Frames)
			for i := range g.Durations {
				g.Durations[i].Min = r.U32()
				g.Durations[i].Max = r.U32()
			}
		} else {
			g.EnsureDurations(defDuration)
		}
	}
	total := g.TotalSprites()
	if total == 0 || total > maxSpritesPerGroup {
		return fmt.Errorf("invalid sprite count %d (%dx%d layers=%d patterns=%dx%dx%d frames=%d)",
			total, g.Width, g.Height, g.Layers, g.PatternX, g.PatternY, g.PatternZ, g.Frames)
	}
	g.Sprites = make([]uint32, total)
	return r.Err()
}

// Encode serializes a dat file.
func Encode(f *File, opts Options) ([]byte, error) {
	w := binio.NewWriter(1 << 20)
	w.U32(f.Signature)
	for _, c := range thing.Categories {
		maxID := f.MaxID(c)
		if maxID > 0xFFFF {
			return nil, fmt.Errorf("too many %ss: max id %d exceeds 65535", c, maxID)
		}
		w.U16(uint16(maxID))
	}
	for _, c := range thing.Categories {
		for i, t := range f.Things[c] {
			id := c.MinID() + uint32(i)
			if t == nil {
				return nil, fmt.Errorf("%s %d is missing", c, id)
			}
			if err := WriteThing(w, t, opts); err != nil {
				return nil, fmt.Errorf("%s %d: %w", c, id, err)
			}
		}
	}
	return w.Bytes(), nil
}

// WriteThing encodes one thing (flags + texture patterns).
func WriteThing(w *binio.Writer, t *thing.Thing, opts Options) error {
	if err := t.Validate(); err != nil {
		return err
	}
	Table(opts.Format).WriteProperties(w, &t.Props)
	withGroups := opts.Features.FrameGroups && t.Category == thing.CategoryOutfit
	groups := t.FrameGroups
	if !withGroups {
		groups = groups[:1]
	} else {
		w.U8(uint8(len(groups)))
	}
	for i, g := range groups {
		if withGroups {
			// ObjectBuilder marks a lone outfit group as "walking" (1).
			typ := uint8(i)
			if len(groups) < 2 {
				typ = 1
			}
			w.U8(typ)
		}
		if err := WriteGroup(w, g, opts.hasPatternZ(), opts.Features.ImprovedAnimations, opts.Features.Extended); err != nil {
			return err
		}
	}
	return nil
}

// WriteGroup writes one frame group layout followed by its sprite ids.
func WriteGroup(w *binio.Writer, g *thing.FrameGroup, patternZ, durations, extended bool) error {
	if err := WriteLayout(w, g, patternZ, durations); err != nil {
		return err
	}
	for _, id := range g.Sprites {
		if extended {
			w.U32(id)
		} else {
			if id > 0xFFFF {
				return fmt.Errorf("sprite id %d needs the extended feature", id)
			}
			w.U16(uint16(id))
		}
	}
	return nil
}

// WriteLayout writes the dimensions and animation of a frame group without
// sprite ids. Shared with the OBD codec.
func WriteLayout(w *binio.Writer, g *thing.FrameGroup, patternZ, durations bool) error {
	w.U8(g.Width)
	w.U8(g.Height)
	if g.Width > 1 || g.Height > 1 {
		w.U8(g.ExactSize)
	}
	w.U8(g.Layers)
	w.U8(g.PatternX)
	w.U8(g.PatternY)
	if patternZ {
		w.U8(g.PatternZ)
	} else if g.PatternZ > 1 {
		return fmt.Errorf("pattern Z %d is not supported by this format", g.PatternZ)
	}
	w.U8(g.Frames)
	if durations && g.IsAnimation() {
		w.U8(uint8(g.Mode))
		w.I32(g.LoopCount)
		w.I8(g.StartFrame)
		for _, d := range g.Durations {
			w.U32(d.Min)
			w.U32(d.Max)
		}
	}
	return nil
}
