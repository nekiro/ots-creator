package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
)

// mapToolNames are the tools of run_map_tool (see app.MapService.Tool).
var mapToolNames = []string{
	"borderizeSelection", "borderizeMap", "randomizeSelection", "randomizeMap", "removeCorpses", "removeInvalid",
	"removeUnreachable", "removeEmptySpawns", "clearInvalidHouses", "clearModified",
}

type selectAreaArgs struct {
	MapArg
	Rect   Rect   `json:"rect"`
	Floors string `json:"floors,omitempty" jsonschema:"current (only the rect's floor, default), lower (it and every floor below) or visible"`
	Mode   string `json:"mode,omitempty" jsonschema:"replace (default), add or remove"`
}

type selectTilesArgs struct {
	MapArg
	Positions []Pos `json:"positions"`
	Add       bool  `json:"add,omitempty" jsonschema:"add to the selection instead of replacing it"`
}

type pasteArgs struct {
	MapArg
	Pos Pos `json:"pos" jsonschema:"where the north-west corner of the copied area goes"`
}

type moveSelectionArgs struct {
	MapArg
	DX int `json:"dx,omitempty"`
	DY int `json:"dy,omitempty"`
	DZ int `json:"dz,omitempty" jsonschema:"floors down (negative: up)"`
}

type brushFromSelectionArgs struct {
	MapArg
	Name     string   `json:"name" jsonschema:"name of the new doodad brush"`
	Tilesets []string `json:"tilesets,omitempty" jsonschema:"tilesets to list it in"`
}

type mapToolArgs struct {
	MapArg
	Name        string `json:"name" jsonschema:"borderizeSelection, borderizeMap (redo all ground borders), randomizeSelection, randomizeMap (vary grounds), removeCorpses, removeInvalid (unknown items), removeUnreachable, removeEmptySpawns, clearInvalidHouses or clearModified"`
	InSelection bool   `json:"inSelection,omitempty" jsonschema:"item tools only touch the selection"`
}

type replaceArgs struct {
	MapArg
	Pairs []struct {
		From int `json:"from" jsonschema:"item id to replace"`
		To   int `json:"to" jsonschema:"new item id; 0 removes the items"`
	} `json:"pairs"`
	InSelection bool `json:"inSelection,omitempty"`
}

func (s *Server) addSelectionTools(server *mcp.Server) {
	addTool(server, "select_area", "Selects the tiles of a rectangle, for copy, cut, delete, move and borderizeSelection.",
		func(_ context.Context, args selectAreaArgs) (any, error) {
			from, err := args.Rect.From.position()
			if err != nil {
				return nil, err
			}
			to, err := args.Rect.To.position()
			if err != nil {
				return nil, err
			}
			to.Z = from.Z
			floors := args.Floors
			if floors == "" {
				floors = mapedit.FloorsCurrent
			}
			if !slices.Contains([]string{mapedit.FloorsCurrent, mapedit.FloorsLower, mapedit.FloorsVisible}, floors) {
				return nil, fmt.Errorf("unknown floors %q", floors)
			}
			add, remove := args.Mode == "add", args.Mode == "remove"
			if args.Mode != "" && args.Mode != "replace" && !add && !remove {
				return nil, fmt.Errorf("unknown mode %q", args.Mode)
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) {
				return s.maps.SelectRect(id, mapedit.Rect{From: from, To: to}, floors, add, remove, false)
			})
		})

	addTool(server, "select_tiles", "Selects whole tiles by position.",
		func(_ context.Context, args selectTilesArgs) (any, error) {
			list, err := positions(args.Positions)
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SelectTiles(id, list, args.Add) })
		})

	addTool(server, "clear_selection", "Deselects everything.",
		func(_ context.Context, args MapArg) (any, error) {
			return s.edit(args.MapID, s.maps.ClearSelection)
		})

	addTool(server, "copy_selection", "Copies the selected tiles to the clipboard (shared by all maps).",
		func(_ context.Context, args MapArg) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			tiles, err := s.maps.Copy(id)
			if err != nil {
				return nil, err
			}
			return map[string]any{"copiedTiles": tiles}, nil
		})

	addTool(server, "cut_selection", "Copies the selected tiles to the clipboard and deletes them.",
		func(_ context.Context, args MapArg) (any, error) {
			return s.edit(args.MapID, s.maps.Cut)
		})

	addTool(server, "paste", "Pastes the clipboard at a position (it may come from another map).",
		func(_ context.Context, args pasteArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.Paste(id, pos) })
		})

	addTool(server, "delete_selection", "Deletes the selected tiles, items and creatures.",
		func(_ context.Context, args MapArg) (any, error) {
			return s.edit(args.MapID, s.maps.DeleteSelection)
		})

	addTool(server, "move_selection", "Moves the selection by an offset, also to other floors.",
		func(_ context.Context, args moveSelectionArgs) (any, error) {
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.MoveSelection(id, args.DX, args.DY, args.DZ) })
		})

	addTool(server, "selection_to_brush", "Makes a doodad brush from the selected tiles, to place the same structure again with draw kind doodad.",
		func(_ context.Context, args brushFromSelectionArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			if args.Name == "" {
				return nil, errors.New("the brush needs a name")
			}
			if err := s.maps.DoodadFromSelection(id, args.Name, args.Tilesets); err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return map[string]any{"brush": args.Name}, nil
		})

	addTool(server, "run_map_tool", "Runs a cleanup tool on the whole map or the selection: redo borders, vary grounds, remove corpses, unknown items, unreachable tiles or empty spawns.",
		func(_ context.Context, args mapToolArgs) (any, error) {
			if !slices.Contains(mapToolNames, args.Name) {
				return nil, fmt.Errorf("unknown tool %q", args.Name)
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.Tool(id, args.Name, args.InSelection) })
		})

	addTool(server, "replace_items", "Replaces items by id on the whole map or the selection (to 0 removes them).",
		func(_ context.Context, args replaceArgs) (any, error) {
			from, to := make([]uint16, 0, len(args.Pairs)), make([]uint16, 0, len(args.Pairs))
			for _, pair := range args.Pairs {
				if pair.From <= 0 || pair.From > 0xFFFF || pair.To < 0 || pair.To > 0xFFFF {
					return nil, fmt.Errorf("bad item id pair %d -> %d", pair.From, pair.To)
				}
				from, to = append(from, uint16(pair.From)), append(to, uint16(pair.To))
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.Replace(id, from, to, args.InSelection) })
		})
}
