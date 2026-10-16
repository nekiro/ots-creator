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
func OptionsFor(version uint16, features client.Features) Options {
	features.ApplyVersionDefaults(version)
	return Options{Format: client.MetadataFormat(version), Features: features}
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
	file := &File{Signature: signature, Things: map[thing.Category][]*thing.Thing{}}
	return file
}

// Get returns a thing by category and id.
func (f *File) Get(category thing.Category, id uint32) *thing.Thing {
	list := f.Things[category]
	i := int(id) - int(category.MinID())
	if i < 0 || i >= len(list) {
		return nil
	}
	return list[i]
}

// MaxID returns the highest id of a category, or MinID-1 when it is empty.
func (f *File) MaxID(category thing.Category) uint32 {
	return category.MinID() + uint32(len(f.Things[category])) - 1
}

// Decode parses a Tibia.dat file.
func Decode(data []byte, opts Options) (*File, error) {
	r := binio.NewReader(data)
	file := NewFile(r.U32())
	var counts [4]uint16
	for i := range counts {
		counts[i] = r.U16()
	}
	if r.Err() != nil {
		return nil, fmt.Errorf("dat header: %w", r.Err())
	}
	table := Table(opts.Format)
	for i, category := range thing.Categories {
		maxID := uint32(counts[i])
		if maxID < category.MinID() {
			continue
		}
		list := make([]*thing.Thing, 0, maxID-category.MinID()+1)
		for id := category.MinID(); id <= maxID; id++ {
			object := &thing.Thing{ID: id, Category: category}
			if err := table.ReadProperties(r, &object.Props); err != nil {
				return nil, fmt.Errorf("%s %d: %w", category, id, err)
			}
			if err := readGroups(r, object, opts); err != nil {
				return nil, fmt.Errorf("%s %d: %w", category, id, err)
			}
			list = append(list, object)
		}
		file.Things[category] = list
	}
	return file, nil
}

// ReadThing decodes one thing (flags + texture patterns) from r. OBD v1
// uses this layout.
func ReadThing(r *binio.Reader, object *thing.Thing, opts Options) error {
	if err := Table(opts.Format).ReadProperties(r, &object.Props); err != nil {
		return err
	}
	return readGroups(r, object, opts)
}

func readGroups(r *binio.Reader, object *thing.Thing, opts Options) error {
	groupCount := 1
	withGroups := opts.Features.FrameGroups && object.Category == thing.CategoryOutfit
	if withGroups {
		groupCount = int(r.U8())
	}
	object.FrameGroups = make([]*thing.FrameGroup, 0, groupCount)
	for i := 0; i < groupCount; i++ {
		group := &thing.FrameGroup{}
		if withGroups {
			group.Type = thing.FrameGroupType(r.U8())
		}
		if err := ReadGroup(r, group, opts.hasPatternZ(), opts.Features.ImprovedAnimations, opts.Features.Extended, object.Category.DefaultDuration()); err != nil {
			return err
		}
		object.FrameGroups = append(object.FrameGroups, group)
	}
	return r.Err()
}

// ReadGroup reads one frame group layout followed by its sprite ids.
func ReadGroup(r *binio.Reader, group *thing.FrameGroup, patternZ, durations, extended bool, defaultDuration uint32) error {
	if err := ReadLayout(r, group, patternZ, durations, defaultDuration); err != nil {
		return err
	}
	for i := range group.Sprites {
		if extended {
			group.Sprites[i] = r.U32()
		} else {
			group.Sprites[i] = uint32(r.U16())
		}
	}
	return r.Err()
}

// ReadLayout reads the dimensions and animation of a frame group and
// allocates group.Sprites without reading sprite ids. Shared with the OBD codec.
func ReadLayout(r *binio.Reader, group *thing.FrameGroup, patternZ, durations bool, defaultDuration uint32) error {
	group.Width = r.U8()
	group.Height = r.U8()
	if group.Width > 1 || group.Height > 1 {
		group.ExactSize = r.U8()
	} else {
		group.ExactSize = 32
	}
	group.Layers = r.U8()
	group.PatternX = r.U8()
	group.PatternY = r.U8()
	if patternZ {
		group.PatternZ = r.U8()
	} else {
		group.PatternZ = 1
	}
	group.Frames = r.U8()
	if r.Err() != nil {
		return r.Err()
	}
	if group.Frames > 1 {
		if durations {
			group.Mode = thing.AnimationMode(r.U8())
			group.LoopCount = r.I32()
			group.StartFrame = r.I8()
			group.Durations = make([]thing.FrameDuration, group.Frames)
			for i := range group.Durations {
				group.Durations[i].Min = r.U32()
				group.Durations[i].Max = r.U32()
			}
		} else {
			group.EnsureDurations(defaultDuration)
		}
	}
	total := group.TotalSprites()
	if total == 0 || total > maxSpritesPerGroup {
		return fmt.Errorf("invalid sprite count %d (%dx%d layers=%d patterns=%dx%dx%d frames=%d)",
			total, group.Width, group.Height, group.Layers, group.PatternX, group.PatternY, group.PatternZ, group.Frames)
	}
	group.Sprites = make([]uint32, total)
	return r.Err()
}

// Encode serializes a dat file.
func Encode(file *File, opts Options) ([]byte, error) {
	w := binio.NewWriter(1 << 20)
	w.U32(file.Signature)
	for _, category := range thing.Categories {
		maxID := file.MaxID(category)
		if maxID > 0xFFFF {
			return nil, fmt.Errorf("too many %ss: max id %d exceeds 65535", category, maxID)
		}
		w.U16(uint16(maxID))
	}
	for _, category := range thing.Categories {
		for i, object := range file.Things[category] {
			id := category.MinID() + uint32(i)
			if object == nil {
				return nil, fmt.Errorf("%s %d is missing", category, id)
			}
			if err := WriteThing(w, object, opts); err != nil {
				return nil, fmt.Errorf("%s %d: %w", category, id, err)
			}
		}
	}
	return w.Bytes(), nil
}

// WriteThing encodes one thing (flags + texture patterns).
func WriteThing(w *binio.Writer, object *thing.Thing, opts Options) error {
	if err := object.Validate(); err != nil {
		return err
	}
	Table(opts.Format).WriteProperties(w, &object.Props)
	withGroups := opts.Features.FrameGroups && object.Category == thing.CategoryOutfit
	groups := object.FrameGroups
	if !withGroups {
		groups = groups[:1]
	} else {
		w.U8(uint8(len(groups)))
	}
	for i, group := range groups {
		if withGroups {
			// ObjectBuilder marks a lone outfit group as "walking" (1).
			groupType := uint8(i)
			if len(groups) < 2 {
				groupType = 1
			}
			w.U8(groupType)
		}
		if err := WriteGroup(w, group, opts.hasPatternZ(), opts.Features.ImprovedAnimations, opts.Features.Extended); err != nil {
			return err
		}
	}
	return nil
}

// WriteGroup writes one frame group layout followed by its sprite ids.
func WriteGroup(w *binio.Writer, group *thing.FrameGroup, patternZ, durations, extended bool) error {
	if err := WriteLayout(w, group, patternZ, durations); err != nil {
		return err
	}
	for _, id := range group.Sprites {
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
func WriteLayout(w *binio.Writer, group *thing.FrameGroup, patternZ, durations bool) error {
	w.U8(group.Width)
	w.U8(group.Height)
	if group.Width > 1 || group.Height > 1 {
		w.U8(group.ExactSize)
	}
	w.U8(group.Layers)
	w.U8(group.PatternX)
	w.U8(group.PatternY)
	if patternZ {
		w.U8(group.PatternZ)
	} else if group.PatternZ > 1 {
		return fmt.Errorf("pattern Z %d is not supported by this format", group.PatternZ)
	}
	w.U8(group.Frames)
	if durations && group.IsAnimation() {
		w.U8(uint8(group.Mode))
		w.I32(group.LoopCount)
		w.I8(group.StartFrame)
		for _, duration := range group.Durations {
			w.U32(duration.Min)
			w.U32(duration.Max)
		}
	}
	return nil
}
