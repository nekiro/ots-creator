package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
	"github.com/nekiro/ots-creator/internal/fsutil"
)

type ThingService struct {
	session *Session
}

func NewThingService(session *Session) *ThingService { return &ThingService{session: session} }

// edit runs fn on the open project; changed reports whether fn changed it
// (the session then tells the frontend).
func (ts *ThingService) edit(fn func(proj *project.Project) (changed bool, err error)) error {
	proj, err := ts.session.Project()
	if err != nil {
		return err
	}
	changed, err := fn(proj)
	if changed {
		ts.session.Changed()
	}
	return err
}

func (ts *ThingService) editAll(fn func(proj *project.Project) error) error {
	return ts.edit(func(proj *project.Project) (bool, error) {
		err := fn(proj)
		return err == nil, err
	})
}

func (ts *ThingService) Get(category thing.Category, id uint32) (*thing.Thing, error) {
	proj, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.Thing(category, id)
}

func (ts *ThingService) Find(category thing.Category, filter project.Filter) ([]uint32, error) {
	proj, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.FindThings(category, filter)
}

func (ts *ThingService) Names(category thing.Category) (map[uint32]string, error) {
	proj, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.Names(category), nil
}

func (ts *ThingService) Update(updated *thing.Thing) error {
	return ts.editAll(func(proj *project.Project) error { return proj.UpdateThing(updated) })
}

func (ts *ThingService) UpdateMany(things []*thing.Thing) error {
	return ts.editAll(func(proj *project.Project) error { return proj.UpdateThings(things) })
}

func (ts *ThingService) Add(category thing.Category) (id uint32, err error) {
	err = ts.editAll(func(proj *project.Project) (err error) {
		id, err = proj.AddThing(category)
		return err
	})
	return id, err
}

func (ts *ThingService) Duplicate(category thing.Category, ids []uint32) (copies []uint32, err error) {
	err = ts.editAll(func(proj *project.Project) (err error) {
		copies, err = proj.DuplicateThings(category, ids)
		return err
	})
	return copies, err
}

// Paste puts copies of things at the ids from at on (see
// project.PasteThings) and returns the ids written.
func (ts *ThingService) Paste(category thing.Category, at uint32, things []*thing.Thing) (ids []uint32, err error) {
	err = ts.editAll(func(proj *project.Project) (err error) {
		ids, err = proj.PasteThings(category, at, things)
		return err
	})
	return ids, err
}

func (ts *ThingService) Remove(category thing.Category, ids []uint32) error {
	return ts.editAll(func(proj *project.Project) error { return proj.RemoveThings(category, ids) })
}

// Move moves things to the ids from at on; see project.MoveThings. It
// returns the new ids of ids.
func (ts *ThingService) Move(category thing.Category, ids []uint32, at uint32, options project.MoveOptions) (moved []uint32, err error) {
	err = ts.editAll(func(proj *project.Project) (err error) {
		moved, err = proj.MoveThings(category, ids, at, options)
		return err
	})
	if err != nil {
		return nil, err
	}
	return moved, nil
}

func (ts *ThingService) Patch(category thing.Category, ids []uint32, patch project.PropsPatch) (changed int, err error) {
	err = ts.edit(func(proj *project.Project) (bool, error) {
		changed, err = proj.PatchThings(category, ids, patch)
		return err == nil && changed > 0, err
	})
	return changed, err
}

func (ts *ThingService) SetDurations(categories []thing.Category, minMs, maxMs uint32) (changed int, err error) {
	err = ts.edit(func(proj *project.Project) (bool, error) {
		changed, err = proj.SetDurations(categories, minMs, maxMs)
		return err == nil && changed > 0, err
	})
	return changed, err
}

func (ts *ThingService) ConvertFrameGroups(toGroups bool) (result project.ConvertResult, err error) {
	err = ts.edit(func(proj *project.Project) (bool, error) {
		result = proj.ConvertFrameGroups(toGroups)
		return result.Converted > 0, nil
	})
	return result, err
}

type ImportResult struct {
	Path     string         `json:"path"`
	Category thing.Category `json:"category"`
	ID       uint32         `json:"id"`
	Error    string         `json:"error"`
}

// ImportObjects imports object files (.otobj or .obd). With replaceID set
// and one file, that thing is replaced instead of appending.
func (ts *ThingService) ImportObjects(paths []string, replaceID uint32) ([]ImportResult, error) {
	if replaceID != 0 && len(paths) != 1 {
		if _, err := ts.session.Project(); err != nil {
			return nil, err
		}
		return nil, errors.New("replace needs exactly one file")
	}
	results := make([]ImportResult, 0, len(paths))
	err := ts.edit(func(proj *project.Project) (bool, error) {
		for _, path := range paths {
			result := ImportResult{Path: path}
			data, err := readObject(path)
			if err == nil {
				result.Category = data.Thing.Category
				result.ID, err = proj.ImportOBD(data, replaceID)
			}
			if err != nil {
				result.Error = err.Error()
			}
			results = append(results, result)
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ImportObjectsTo imports object files of a category, paths[i] to
// targets[i], replacing the things there (targets past the end extend the
// category). It is one undoable edit and returns the ids.
func (ts *ThingService) ImportObjectsTo(category thing.Category, paths []string, targets []uint32) (ids []uint32, err error) {
	err = ts.editAll(func(proj *project.Project) error {
		if len(paths) == 0 {
			return errors.New("nothing to import")
		}
		objects := make([]*obd.Data, len(paths))
		for i, path := range paths {
			if objects[i], err = readObject(path); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(path), err)
			}
		}
		ids, err = proj.ImportOBDsTo(category, objects, targets)
		return err
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (ts *ThingService) PlanTargets(category thing.Category, from uint32, count int, freeOnly bool) (TargetPlan, error) {
	proj, err := ts.session.Project()
	if err != nil {
		return TargetPlan{}, err
	}
	return planTargets(proj, category, from, count, freeOnly)
}

// ExportObjects writes one file per thing into dir, named
// {category}_{id}.otobj or .obd by format (ObjectOTOBJ or ObjectOBD).
// Imports accept every version of both formats.
func (ts *ThingService) ExportObjects(category thing.Category, ids []uint32, dir, format string) ([]string, error) {
	proj, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	var written []string
	for _, id := range ids {
		path, err := exportObject(proj, category, id, dir, format)
		if err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// CopyObjectFiles exports things into a temporary folder (emptied on each
// copy) and puts the files on the system clipboard, so they paste as files
// outside the program. text goes along as plain text. Windows only; it
// fails elsewhere.
func (ts *ThingService) CopyObjectFiles(category thing.Category, ids []uint32, format, text string) error {
	dir := filepath.Join(os.TempDir(), "ots-creator", "clipboard")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, fsutil.DirMode); err != nil {
		return err
	}
	paths, err := ts.ExportObjects(category, ids, dir, format)
	if err != nil {
		return err
	}
	return setClipboardFiles(paths, text)
}

// ImportClipboardObjects imports the object files (.otobj or .obd) copied
// to the system clipboard, like ImportObjects; none when it holds none.
// Windows only; it fails elsewhere.
func (ts *ThingService) ImportClipboardObjects() ([]ImportResult, error) {
	paths, err := clipboardFiles()
	if err != nil {
		return nil, err
	}
	paths = slices.DeleteFunc(paths, func(path string) bool { return !isObjectFile(path) })
	if len(paths) == 0 {
		return nil, nil
	}
	return ts.ImportObjects(paths, 0)
}

func (ts *ThingService) ReadObject(path string) (*ObjectFile, error) {
	data, err := readObject(path)
	if err != nil {
		return nil, err
	}
	return newObjectFile(path, data), nil
}
