package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ThingService edits things. Sheets and pixels are in thing_sheets.go.
type ThingService struct {
	session *Session
}

// NewThingService returns the service.
func NewThingService(session *Session) *ThingService { return &ThingService{session: session} }

// edit runs fn on the open project; changed reports whether fn changed it
// (the session then tells the frontend).
func (ts *ThingService) edit(fn func(p *project.Project) (changed bool, err error)) error {
	p, err := ts.session.Project()
	if err != nil {
		return err
	}
	changed, err := fn(p)
	if changed {
		ts.session.Changed()
	}
	return err
}

// editAll is edit for changes that always change the project when they
// succeed.
func (ts *ThingService) editAll(fn func(p *project.Project) error) error {
	return ts.edit(func(p *project.Project) (bool, error) {
		err := fn(p)
		return err == nil, err
	})
}

// Get returns one thing.
func (ts *ThingService) Get(category thing.Category, id uint32) (*thing.Thing, error) {
	p, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return p.Thing(category, id)
}

// Find returns the ids of things in a category that match the filter.
func (ts *ThingService) Find(category thing.Category, filter project.Filter) ([]uint32, error) {
	p, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return p.FindThings(category, filter)
}

// Names returns the names of the named things of a category, by id.
func (ts *ThingService) Names(category thing.Category) (map[uint32]string, error) {
	p, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	return p.Names(category), nil
}

// Update replaces a thing.
func (ts *ThingService) Update(updated *thing.Thing) error {
	return ts.editAll(func(p *project.Project) error { return p.UpdateThing(updated) })
}

// UpdateMany replaces several things in one undo step.
func (ts *ThingService) UpdateMany(things []*thing.Thing) error {
	return ts.editAll(func(p *project.Project) error { return p.UpdateThings(things) })
}

// Add appends an empty thing and returns its id.
func (ts *ThingService) Add(category thing.Category) (id uint32, err error) {
	err = ts.editAll(func(p *project.Project) (err error) {
		id, err = p.AddThing(category)
		return err
	})
	return id, err
}

// Duplicate copies things to the end of their category.
func (ts *ThingService) Duplicate(category thing.Category, ids []uint32) (copies []uint32, err error) {
	err = ts.editAll(func(p *project.Project) (err error) {
		copies, err = p.DuplicateThings(category, ids)
		return err
	})
	return copies, err
}

// Remove removes things (see project.RemoveThings).
func (ts *ThingService) Remove(category thing.Category, ids []uint32) error {
	return ts.editAll(func(p *project.Project) error { return p.RemoveThings(category, ids) })
}

// Move moves things to the ids from at on; see project.MoveThings. It
// returns the new ids of ids.
func (ts *ThingService) Move(category thing.Category, ids []uint32, at uint32, options project.MoveOptions) (moved []uint32, err error) {
	err = ts.editAll(func(p *project.Project) (err error) {
		moved, err = p.MoveThings(category, ids, at, options)
		return err
	})
	if err != nil {
		return nil, err
	}
	return moved, nil
}

// Patch applies the same property changes to several things (bulk edit)
// and returns how many changed.
func (ts *ThingService) Patch(category thing.Category, ids []uint32, patch project.PropsPatch) (changed int, err error) {
	err = ts.edit(func(p *project.Project) (bool, error) {
		changed, err = p.PatchThings(category, ids, patch)
		return err == nil && changed > 0, err
	})
	return changed, err
}

// SetDurations sets every frame duration of animated things in the given
// categories and returns how many things changed.
func (ts *ThingService) SetDurations(categories []thing.Category, minMs, maxMs uint32) (changed int, err error) {
	err = ts.edit(func(p *project.Project) (bool, error) {
		changed, err = p.SetDurations(categories, minMs, maxMs)
		return err == nil && changed > 0, err
	})
	return changed, err
}

// ConvertFrameGroups splits outfits into idle and walking groups, or
// merges them back into one group.
func (ts *ThingService) ConvertFrameGroups(toGroups bool) (result project.ConvertResult, err error) {
	err = ts.edit(func(p *project.Project) (bool, error) {
		result = p.ConvertFrameGroups(toGroups)
		return result.Converted > 0, nil
	})
	return result, err
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
	if replaceID != 0 && len(paths) != 1 {
		if _, err := ts.session.Project(); err != nil {
			return nil, err
		}
		return nil, errors.New("replace needs exactly one file")
	}
	results := make([]ImportResult, 0, len(paths))
	err := ts.edit(func(p *project.Project) (bool, error) {
		for _, path := range paths {
			result := ImportResult{Path: path}
			data, err := readObject(path)
			if err == nil {
				result.Category = data.Thing.Category
				result.ID, err = p.ImportOBD(data, replaceID)
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
	err = ts.editAll(func(p *project.Project) error {
		if len(paths) == 0 {
			return errors.New("nothing to import")
		}
		objects := make([]*obd.Data, len(paths))
		for i, path := range paths {
			if objects[i], err = readObject(path); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(path), err)
			}
		}
		ids, err = p.ImportOBDsTo(category, objects, targets)
		return err
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// PlanTargets returns count target ids of the open client from id from on:
// free ids only (see project.FreeIDs), or consecutive ids.
func (ts *ThingService) PlanTargets(category thing.Category, from uint32, count int, freeOnly bool) (TargetPlan, error) {
	p, err := ts.session.Project()
	if err != nil {
		return TargetPlan{}, err
	}
	return planTargets(p, category, from, count, freeOnly)
}

// ExportObjects writes one file per thing into dir, named
// {category}_{id}.otobj or .obd by format (ObjectOTOBJ or ObjectOBD).
// Imports accept every version of both formats.
func (ts *ThingService) ExportObjects(category thing.Category, ids []uint32, dir, format string) ([]string, error) {
	p, err := ts.session.Project()
	if err != nil {
		return nil, err
	}
	var written []string
	for _, id := range ids {
		path, err := exportObject(p, category, id, dir, format)
		if err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// ReadObject decodes an object file without importing it. No client needs
// to be open.
func (ts *ThingService) ReadObject(path string) (*ObjectFile, error) {
	data, err := readObject(path)
	if err != nil {
		return nil, err
	}
	return newObjectFile(path, data), nil
}
