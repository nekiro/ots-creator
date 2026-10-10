package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/otobj"
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

// Move moves things to the ids from at on; see project.MoveThings. It
// returns the new ids of ids.
func (ts *ThingService) Move(c thing.Category, ids []uint32, at uint32, o project.MoveOptions) ([]uint32, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	out, err := p.MoveThings(c, ids, at, o)
	if err != nil {
		return nil, err
	}
	ts.s.Changed()
	return out, nil
}

// ImportResult reports one imported file.
type ImportResult struct {
	Path     string         `json:"path"`
	Category thing.Category `json:"category"`
	ID       uint32         `json:"id"`
	Error    string         `json:"error"`
}

// ImportObjects imports object files (.otobj or .obd). With replaceID set
// and one file, that thing is replaced instead of appending.
func (ts *ThingService) ImportObjects(paths []string, replaceID uint32) ([]ImportResult, error) {
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
		d, err := readObject(path)
		if err == nil {
			res.Category = d.Thing.Category
			res.ID, err = p.ImportOBD(d, replaceID)
		}
		if err != nil {
			res.Error = err.Error()
		}
		out = append(out, res)
	}
	ts.s.Changed()
	return out, nil
}

// ExportObjects writes one file per thing into dir, named
// {category}_{id}.otobj or .obd by format (ObjectOTOBJ or ObjectOBD).
// Imports accept every version of both formats.
func (ts *ThingService) ExportObjects(c thing.Category, ids []uint32, dir, format string) ([]string, error) {
	p, err := ts.s.Project()
	if err != nil {
		return nil, err
	}
	var written []string
	for _, id := range ids {
		path, err := exportObject(p, c, id, dir, format)
		if err != nil {
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

// ObjectFile is a decoded object file (.otobj or .obd) for previewing.
// Sprite ids of Thing are rewritten to 1..len(Sprites); Sprites[id-1]
// holds RGBA pixels.
type ObjectFile struct {
	Path string `json:"path"`
	// Format is ObjectOTOBJ or ObjectOBD; Version the version of that format.
	Format        string       `json:"format"`
	Version       int          `json:"version"`
	ClientVersion uint16       `json:"clientVersion"`
	SpriteSize    int          `json:"spriteSize"`
	Thing         *thing.Thing `json:"thing"`
	Sprites       [][]byte     `json:"sprites"`
}

// ReadObject decodes an object file without importing it. No client needs
// to be open.
func (ts *ThingService) ReadObject(path string) (*ObjectFile, error) {
	d, err := readObject(path)
	if err != nil {
		return nil, err
	}
	return newObjectFile(path, d), nil
}

// newObjectFile prepares decoded object data for the frontend.
func newObjectFile(path string, d *obd.Data) *ObjectFile {
	format := ObjectOBD
	if strings.EqualFold(filepath.Ext(path), otobj.Ext) {
		format = ObjectOTOBJ
	}
	out := &ObjectFile{Path: path, Format: format, Version: d.Version, ClientVersion: d.ClientVersion, SpriteSize: d.SpriteSize, Thing: d.Thing.Clone(), Sprites: [][]byte{}}
	for gi, g := range out.Thing.FrameGroups {
		for si := range g.Sprites {
			out.Sprites = append(out.Sprites, d.Sprites[gi][si].Pixels)
			g.Sprites[si] = uint32(len(out.Sprites))
		}
	}
	return out
}

// SetPixels replaces the pixels of sprite slots of one frame group after
// painting (see project.SetSlotPixels).
func (ts *ThingService) SetPixels(c thing.Category, id uint32, group int, slots []int, pixels [][]byte) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	if err := p.SetSlotPixels(c, id, group, slots, pixels, "Paint"); err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}

// ShiftPixels moves the pixels of every texture of a frame group, or with
// one set only the texture at frame and pattern x, y, z (all its layers).
func (ts *ThingService) ShiftPixels(c thing.Category, id uint32, group, dx, dy int, one bool, frame, x, y, z int) error {
	p, err := ts.s.Project()
	if err != nil {
		return err
	}
	pos := project.TexturePos{Frame: frame, PatternX: x, PatternY: y, PatternZ: z}
	if err := p.ShiftPixels(c, id, group, dx, dy, !one, pos); err != nil {
		return err
	}
	ts.s.Changed()
	return nil
}
