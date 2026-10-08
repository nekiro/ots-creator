package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ThingService edits things.
type ThingService struct {
	s *Session
}

// NewThingService returns the service.
func NewThingService(s *Session) *ThingService { return &ThingService{s: s} }

// Get returns one thing.
func (ts *ThingService) Get(c thing.Category, id uint32) (*thing.Thing, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	return p.Thing(c, id)
}

// Find returns the ids of things in a category that match the filter.
func (ts *ThingService) Find(c thing.Category, f project.Filter) ([]uint32, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	return p.FindThings(c, f)
}

// Names returns the names of the named things of a category, by id.
func (ts *ThingService) Names(c thing.Category) (map[uint32]string, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	return p.Names(c), nil
}

// Update replaces a thing.
func (ts *ThingService) Update(t *thing.Thing) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	if err := p.UpdateThing(t); err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}

// Add appends an empty thing and returns its id.
func (ts *ThingService) Add(c thing.Category) (uint32, error) {
	p, err := ts.s.Project()
	if err != nil {
		return 0, err
	}
	id, err := p.AddThing(c)
	if err == nil {
		ts.s.Changed()
	}
	return id, err
}

// Duplicate copies things to the end of their category.
func (ts *ThingService) Duplicate(c thing.Category, ids []uint32) ([]uint32, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	out, err := p.DuplicateThings(c, ids)
	if err == nil {
		ts.s.Changed()
	}
	return out, err
}

// Remove removes things (see project.RemoveThings).
func (ts *ThingService) Remove(c thing.Category, ids []uint32) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	if err := p.RemoveThings(c, ids); err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}

// ImportResult reports one imported file.
type ImportResult struct {
	Path     string         `json:"path"`
	Category thing.Category `json:"category"`
	ID       uint32         `json:"id"`
	Error    string         `json:"error"`
}

// ImportOBD imports OBD files. With replaceID set and one file, that thing
// is replaced instead of appending.
func (ts *ThingService) ImportOBD(paths []string, replaceID uint32) ([]ImportResult, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	if replaceID != 0 && len(paths) != 1 {
		return nil, errors.New("replace needs exactly one file")
	}
	out := make([]ImportResult, 0, len(paths))
	for _, path := range paths {
		res := ImportResult{Path: path}
		data, err := os.ReadFile(path)
		if err == nil {
			var d *obd.Data
			if d, err = obd.Decode(data); err == nil {
				res.Category = d.Thing.Category
				res.ID, err = p.ImportOBD(d, replaceID)
			}
		}
		if err != nil {
			res.Error = err.Error()
		}
		out = append(out, res)
	}
	ts.s.Changed()
	return out, nil
}

// ExportOBD writes one OBD file per thing into dir, named {category}_{id}.obd.
// Exports always use the newest OBD version; imports accept every version.
func (ts *ThingService) ExportOBD(c thing.Category, ids []uint32, dir string) ([]string, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	var written []string
	for _, id := range ids {
		d, err := p.ExportOBD(c, id, obd.Version3)
		if err != nil {
			return written, err
		}
		data, err := obd.Encode(d)
		if err != nil {
			return written, fmt.Errorf("%s %d: %w", c, id, err)
		}
		path := filepath.Join(dir, fmt.Sprintf("%s_%d.obd", c, id))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// ExportSheet writes the sprite sheet of one frame group, or of the whole
// thing with AllGroups. The format follows the file extension (png, bmp or
// jpg); formats without alpha get a magenta background.
func (ts *ThingService) ExportSheet(c thing.Category, id uint32, group int, path string, transparent bool) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	img, err := renderSheet(p, c, id, group, transparent)
	if err != nil {
		return err
	}
	data, err := imaging.Encode(img, imaging.FormatOf(path), imaging.Magenta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// renderSheet draws the sprite sheet of one frame group on magenta (like
// ObjectBuilder) or a transparent background.
// AllGroups selects every frame group of a thing in one sheet (idle frames
// then walking frames).
const AllGroups = -1

func renderSheet(p *project.Project, c thing.Category, id uint32, group int, transparent bool) (*image.NRGBA, error) {
	t, err := p.Thing(c, id)
	if err != nil {
		return nil, err
	}
	var g *thing.FrameGroup
	switch {
	case group == AllGroups:
		var ok bool
		if g, ok = t.Stacked(); !ok {
			return nil, errors.New("idle and walking groups have different layouts; export them one by one")
		}
	case group < 0 || group >= len(t.FrameGroups):
		return nil, fmt.Errorf("no frame group %d", group)
	default:
		g = t.FrameGroups[group]
	}
	bg := imaging.Magenta
	if transparent {
		bg = color.NRGBA{}
	}
	var firstErr error
	img := imaging.Sheet(g, p.SpriteSize(), bg, func(slot int) []byte {
		sid := g.Sprites[slot]
		if sid == 0 {
			return nil
		}
		px, err := p.SpritePixels(sid)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return px
	})
	return img, firstErr
}

// ImportSheet loads a sheet image into a thing. A sheet with the size of
// the given frame group replaces that group's sprites; any other sheet
// sets the layout from the image (see project.ImportSheet).
func (ts *ThingService) ImportSheet(c thing.Category, id uint32, group int, path string) error {
	img, err := imaging.Load(path)
	if err != nil {
		return err
	}
	return ts.importSheet(c, id, group, img)
}

// PasteSheet is ImportSheet for an image from the clipboard (PNG, BMP,
// GIF or JPEG bytes).
func (ts *ThingService) PasteSheet(c thing.Category, id uint32, group int, data []byte) error {
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("clipboard image: %w", err)
	}
	return ts.importSheet(c, id, group, img)
}

func (ts *ThingService) importSheet(c thing.Category, id uint32, group int, img *image.NRGBA) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	t, err := p.Thing(c, id)
	if err != nil {
		return err
	}
	if group < 0 || group >= len(t.FrameGroups) {
		return fmt.Errorf("no frame group %d", group)
	}
	if w, h := t.FrameGroups[group].SheetSize(p.SpriteSize()); w == img.Rect.Dx() && h == img.Rect.Dy() {
		pixels, err := imaging.SliceSheet(img, t.FrameGroups[group], p.SpriteSize())
		if err != nil {
			return err
		}
		err = p.SetGroupPixels(c, id, group, pixels)
	} else {
		err = p.ImportSheet(c, id, img)
	}
	if err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}

// Patch applies the same property changes to several things (bulk edit)
// and returns how many changed.
func (ts *ThingService) Patch(c thing.Category, ids []uint32, patch project.PropsPatch) (int, error) {
	p, err := ts.s.Project()
	if err != nil {
		return 0, err
	}
	n, err := p.PatchThings(c, ids, patch)
	if err == nil && n > 0 {
		ts.s.Changed()
	}
	return n, err
}

// UpdateMany replaces several things in one undo step.
func (ts *ThingService) UpdateMany(things []*thing.Thing) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	if err := p.UpdateThings(things); err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}

// SetDurations sets every frame duration of animated things in the given
// categories and returns how many things changed.
func (ts *ThingService) SetDurations(cats []thing.Category, minMs, maxMs uint32) (int, error) {
	p, err := ts.s.Project()
	if err != nil {
		return 0, err
	}
	n, err := p.SetDurations(cats, minMs, maxMs)
	if err == nil && n > 0 {
		ts.s.Changed()
	}
	return n, err
}

// ConvertFrameGroups splits outfits into idle and walking groups, or
// merges them back into one group.
func (ts *ThingService) ConvertFrameGroups(toGroups bool) (project.ConvertResult, error) {
	p, err := ts.s.Project()
	if err != nil {
		return project.ConvertResult{}, err
	}
	res := p.ConvertFrameGroups(toGroups)
	if res.Converted > 0 {
		ts.s.Changed()
	}
	return res, nil
}

// OBDFile is a decoded OBD file for previewing. Sprite ids of Thing are
// rewritten to 1..len(Sprites); Sprites[id-1] holds RGBA pixels.
type OBDFile struct {
	Path          string       `json:"path"`
	Version       int          `json:"version"`
	ClientVersion uint16       `json:"clientVersion"`
	SpriteSize    int          `json:"spriteSize"`
	Thing         *thing.Thing `json:"thing"`
	Sprites       [][]byte     `json:"sprites"`
}

// ReadOBD decodes an OBD file without importing it. No client needs to be
// open.
func (ts *ThingService) ReadOBD(path string) (*OBDFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := obd.Decode(data)
	if err != nil {
		return nil, err
	}
	out := &OBDFile{Path: path, Version: d.Version, ClientVersion: d.ClientVersion, SpriteSize: d.SpriteSize, Thing: d.Thing.Clone(), Sprites: [][]byte{}}
	for gi, g := range out.Thing.FrameGroups {
		for si := range g.Sprites {
			out.Sprites = append(out.Sprites, d.Sprites[gi][si].Pixels)
			g.Sprites[si] = uint32(len(out.Sprites))
		}
	}
	return out, nil
}
