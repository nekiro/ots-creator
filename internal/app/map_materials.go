package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

// saveMaterials writes the brush file and reindexes brushes.
func (ms *MapService) saveMaterials(data *mapData) error {
	data.index()
	ms.setData(data)
	if data.materialsPath == "" {
		return errors.New("no materials file")
	}
	if err := os.MkdirAll(filepath.Dir(data.materialsPath), fsutil.DirMode); err != nil {
		return err
	}
	return data.materials.Save(data.materialsPath)
}

// editMaterials runs fn on the materials and saves them when it succeeds.
func (ms *MapService) editMaterials(fn func(materialSet *materials.Materials) error) error {
	return ms.withData(func(data *mapData) error {
		if err := fn(data.materials); err != nil {
			return err
		}
		return ms.saveMaterials(data)
	})
}

// MaterialsResult reports a materials change.
type MaterialsResult struct {
	Data     DataInfo `json:"data"`
	Warnings []string `json:"warnings"`
}

// ImportRME merges brushes from a Remere's Map Editor data folder.
func (ms *MapService) ImportRME(dir string) (result MaterialsResult, err error) {
	err = ms.withData(func(data *mapData) error {
		found, err := materials.FindRMEData(dir, int(data.project.Info().Version.Value))
		if err != nil {
			return err
		}
		imported, warnings, err := materials.ImportRME(found, data.items.FromClient)
		if err != nil {
			return err
		}
		data.materials.Merge(imported)
		if err := ms.saveMaterials(data); err != nil {
			return err
		}
		result = MaterialsResult{Data: data.info(), Warnings: warnings}
		return nil
	})
	return result, err
}

// ImportMaterials merges a materials JSON file.
func (ms *MapService) ImportMaterials(path string) (result MaterialsResult, err error) {
	err = ms.withData(func(data *mapData) error {
		imported, err := materials.Load(path)
		if err != nil {
			return err
		}
		data.materials.Merge(imported)
		if err := ms.saveMaterials(data); err != nil {
			return err
		}
		result = MaterialsResult{Data: data.info()}
		return nil
	})
	return result, err
}

// ExportMaterials writes the materials to a JSON file.
func (ms *MapService) ExportMaterials(path string) error {
	return ms.withData(func(data *mapData) error { return data.materials.Save(path) })
}

// Brush returns a brush definition for the brush editor.
func (ms *MapService) Brush(name string) (brush *materials.Brush, err error) {
	err = ms.withData(func(data *mapData) error {
		if brush = data.materials.Brush(name); brush == nil {
			return fmt.Errorf("no brush %q", name)
		}
		return nil
	})
	return brush, err
}

// Borders returns the border definitions.
func (ms *MapService) Borders() (borders []*materials.Border, err error) {
	err = ms.withData(func(data *mapData) error {
		borders = data.materials.Borders
		return nil
	})
	return borders, err
}

// SaveBorder creates (ID 0) or replaces a border and returns its id.
func (ms *MapService) SaveBorder(border materials.Border) (id uint32, err error) {
	err = ms.editMaterials(func(materialSet *materials.Materials) error {
		id = materialSet.SaveBorder(border)
		return nil
	})
	return id, err
}

// SaveBrush creates or replaces a brush; oldName is its current name (""
// for a new brush). tilesets, when not nil, are the collections the brush is
// in afterwards: missing ones are created, others no longer list it.
func (ms *MapService) SaveBrush(oldName string, brush materials.Brush, tilesets []string) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { return materialSet.SaveBrush(oldName, brush, tilesets) })
}

// RemoveBrush deletes a brush.
func (ms *MapService) RemoveBrush(name string) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { materialSet.RemoveBrush(name); return nil })
}

// SaveTileset creates or replaces a tileset (oldName "" creates).
func (ms *MapService) SaveTileset(oldName string, tileset materials.Tileset) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { return materialSet.SaveTileset(oldName, tileset) })
}

// RemoveTileset deletes a tileset (its brushes stay).
func (ms *MapService) RemoveTileset(name string) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { materialSet.RemoveTileset(name); return nil })
}

// AddToTileset adds items (raw palette) to a tileset, creating it.
func (ms *MapService) AddToTileset(name string, items []uint16) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { materialSet.AddRawItems(name, items); return nil })
}

// SaveCreature adds or replaces a creature type.
func (ms *MapService) SaveCreature(creature materials.CreatureType) error {
	return ms.editMaterials(func(materialSet *materials.Materials) error { return materialSet.SaveCreature(creature) })
}

// DoodadFromSelection creates a doodad brush from the selected items, as
// one composite relative to the selection's top-left tile.
func (ms *MapService) DoodadFromSelection(id int, name string, tilesets []string) error {
	return ms.withMap(id, func(m *openMap, data *mapData) error {
		clip := m.editor.Copy()
		if clip == nil {
			return errors.New("select the items of the doodad first")
		}
		brush, err := clip.DoodadBrush(name)
		if err != nil {
			return err
		}
		if err := data.materials.SaveBrush("", brush, tilesets); err != nil {
			return err
		}
		return ms.saveMaterials(data)
	})
}
