package app

import (
	"errors"
	"math"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

var errNothingSelected = errors.New("nothing is selected")

// selectionEdit is edit for selection changes: Count is the size of the
// selection afterwards.
func (ms *MapService) selectionEdit(id int, fn func(editor *mapedit.Editor)) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		fn(editor)
		return editor.SelectionCount(), nil
	})
}

// SelectRect selects a rectangle.
func (ms *MapService) SelectRect(id int, rect mapedit.Rect, floors string, add, remove, compensate bool) (MapChange, error) {
	return ms.selectionEdit(id, func(editor *mapedit.Editor) { editor.SelectRect(rect, floors, add, remove, compensate) })
}

// SelectAt selects the top thing of a tile.
func (ms *MapService) SelectAt(id int, pos otmap.Position, toggle, creatures, spawns bool) (MapChange, error) {
	return ms.selectionEdit(id, func(editor *mapedit.Editor) { editor.SelectAt(pos, toggle, creatures, spawns) })
}

// SelectItem selects one item of a tile.
func (ms *MapService) SelectItem(id int, pos otmap.Position, index int, add bool) (MapChange, error) {
	return ms.selectionEdit(id, func(editor *mapedit.Editor) { editor.SelectItem(pos, index, add) })
}

// SelectTiles selects whole tiles.
func (ms *MapService) SelectTiles(id int, positions []otmap.Position, add bool) (MapChange, error) {
	return ms.selectionEdit(id, func(editor *mapedit.Editor) { editor.SelectTiles(positions, add) })
}

// ClearSelection deselects everything.
func (ms *MapService) ClearSelection(id int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { editor.ClearSelection(); return nil })
}

// SelectAll selects every tile of a floor.
func (ms *MapService) SelectAll(id int, floor int) (MapChange, error) {
	z := uint8(floor)
	whole := mapedit.Rect{From: otmap.Position{Z: z}, To: otmap.Position{X: math.MaxUint16, Y: math.MaxUint16, Z: z}}
	return ms.selectionEdit(id, func(editor *mapedit.Editor) {
		editor.SelectRect(whole, mapedit.FloorsCurrent, false, false, false)
	})
}

// Copy copies the selection to the clipboard (shared by all maps) and
// returns the number of copied tiles.
func (ms *MapService) Copy(id int) (tiles int, err error) {
	err = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		clip := opened.editor.Copy()
		if clip == nil {
			return errNothingSelected
		}
		ms.clipboard = clip
		ms.clipboardMode = opened.editor.Items.Mode
		tiles = len(clip.Tiles)
		return nil
	})
	return tiles, err
}

// Cut copies the selection and deletes it.
func (ms *MapService) Cut(id int) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		clip := editor.Cut()
		if clip == nil {
			return 0, errNothingSelected
		}
		ms.clipboard = clip
		ms.clipboardMode = editor.Items.Mode
		return len(clip.Tiles), nil
	})
}

// Clipboard returns the copied content for the paste preview.
func (ms *MapService) Clipboard() *mapedit.Clip {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if opened := ms.maps[ms.activeID]; opened != nil && ms.clipboardMode != opened.editor.Items.Mode {
		return nil
	}
	return ms.clipboard
}

// Paste puts the clipboard with its top-left corner at pos.
func (ms *MapService) Paste(id int, pos otmap.Position) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error {
		if ms.clipboard != nil && ms.clipboardMode != editor.Items.Mode {
			return errors.New("the clipboard uses a different item ID mode; copy items from a map with the same mode")
		}
		return editor.Paste(ms.clipboard, pos)
	})
}

// DeleteSelection removes everything selected.
func (ms *MapService) DeleteSelection(id int) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) { return editor.DeleteSelection(""), nil })
}

// MoveSelection moves the selection.
func (ms *MapService) MoveSelection(id int, dx, dy, dz int) (MapChange, error) {
	return ms.editMap(id, func(editor *mapedit.Editor) error { return editor.MoveSelection(dx, dy, dz) })
}
