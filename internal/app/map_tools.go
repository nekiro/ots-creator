package app

import (
	"errors"
	"fmt"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
)

// mapTool is a whole map or selection tool; it returns the number of
// affected tiles or items.
type mapTool struct {
	run func(editor *mapedit.Editor, inSelection bool) int
	// redrawAll is set for tools that change how every tile is drawn.
	redrawAll bool
}

var mapTools = map[string]mapTool{
	"borderizeSelection": {run: func(editor *mapedit.Editor, _ bool) int { return editor.BorderizeSelection() }},
	"borderizeMap":       {run: func(editor *mapedit.Editor, _ bool) int { return editor.BorderizeMap() }},
	"randomizeSelection": {run: func(editor *mapedit.Editor, _ bool) int { return editor.RandomizeSelection() }},
	"randomizeMap":       {run: func(editor *mapedit.Editor, _ bool) int { return editor.RandomizeMap() }},
	"removeCorpses":      {run: (*mapedit.Editor).RemoveCorpses},
	"removeInvalid":      {run: (*mapedit.Editor).RemoveInvalid},
	"removeUnreachable":  {run: func(editor *mapedit.Editor, _ bool) int { return editor.RemoveUnreachable() }},
	"removeEmptySpawns":  {run: func(editor *mapedit.Editor, _ bool) int { return editor.RemoveEmptySpawns() }},
	"clearInvalidHouses": {run: func(editor *mapedit.Editor, _ bool) int { return editor.ClearInvalidHouses() }},
	"clearModified": {
		run:       func(editor *mapedit.Editor, _ bool) int { editor.ClearModified(); return 0 },
		redrawAll: true,
	},
}

// Tool runs a map tool: "borderizeSelection", "borderizeMap",
// "randomizeSelection", "randomizeMap", "removeCorpses", "removeInvalid",
// "removeUnreachable", "removeEmptySpawns", "clearInvalidHouses",
// "clearModified". inSelection limits item tools to the selection.
func (ms *MapService) Tool(id int, name string, inSelection bool) (MapChange, error) {
	tool, ok := mapTools[name]
	if !ok {
		return MapChange{}, fmt.Errorf("unknown tool %q", name)
	}
	change, err := ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		return tool.run(editor, inSelection), nil
	})
	change.All = change.All || tool.redrawAll
	return change, err
}

// FindResult lists found items; More is set when the limit was reached.
type FindResult struct {
	Results []mapedit.FindResult `json:"results"`
	More    bool                 `json:"more"`
}

// Find searches items on the map.
func (ms *MapService) Find(id int, query mapedit.FindQuery) (found FindResult, err error) {
	err = ms.withMap(id, func(m *openMap, _ *mapData) error {
		found.Results, found.More = m.editor.Find(query)
		return nil
	})
	return found, err
}

// Replace replaces items (to 0 removes them).
func (ms *MapService) Replace(id int, from, to []uint16, inSelection bool) (MapChange, error) {
	if len(from) != len(to) {
		return MapChange{}, errors.New("replace needs pairs of ids")
	}
	replacements := map[uint16]uint16{}
	for i := range from {
		replacements[from[i]] = to[i]
	}
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		return editor.ReplaceItems(replacements, inSelection), nil
	})
}

// Stats returns map statistics.
func (ms *MapService) Stats(id int) (stats mapedit.Statistics, err error) {
	err = ms.withMap(id, func(m *openMap, _ *mapData) error {
		stats = m.editor.Stats()
		return nil
	})
	return stats, err
}
