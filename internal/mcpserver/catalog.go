package mcpserver

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

// Default and largest list sizes of catalog tools.
const (
	defaultListLimit = 100
	maxListLimit     = 1000
)

func listLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	return min(limit, maxListLimit)
}

func doorTypes() []string { return slices.Clone(materials.DoorTypes) }

// brushEntry is a brush in list results.
type brushEntry struct {
	Name string `json:"name"`
	// Kind is the draw kind (wall decorations are drawn as walls).
	Kind       string `json:"kind"`
	Decoration bool   `json:"wallDecoration,omitempty"`
	Look       uint16 `json:"lookItem,omitempty"`
	// NoBorders marks grounds that place no borders around them.
	NoBorders bool `json:"noBorders,omitempty"`
}

func newBrushEntry(brush app.PaletteBrush) brushEntry {
	entry := brushEntry{Name: brush.Name, Kind: brush.Kind, Look: brush.Look}
	if brush.Kind == materials.TypeDecoration {
		entry.Kind, entry.Decoration = materials.TypeWall, true
	}
	return entry
}

func brushEntries(brushes []app.PaletteBrush) []brushEntry {
	out := make([]brushEntry, 0, len(brushes))
	for _, brush := range brushes {
		out = append(out, newBrushEntry(brush))
	}
	return out
}

type listBrushesArgs struct {
	Query   string `json:"query,omitempty" jsonschema:"part of the brush name (case-insensitive)"`
	Kind    string `json:"kind,omitempty" jsonschema:"only brushes of this draw kind: ground, wall, doodad, carpet or table"`
	Tileset string `json:"tileset,omitempty" jsonschema:"only brushes of this tileset"`
	Limit   int    `json:"limit,omitempty" jsonschema:"most results (default 100)"`
}

type nameArgs struct {
	Name string `json:"name"`
}

type tilesetArgs struct {
	Name  string `json:"name"`
	Limit int    `json:"limit,omitempty" jsonschema:"most items listed (default 100)"`
}

type searchItemsArgs struct {
	Query string   `json:"query,omitempty" jsonschema:"part of the item name, or an item id"`
	Flags []string `json:"flags,omitempty" jsonschema:"only items with all these flags: ground, unpassable, blockMissile, blockPath, pickupable, moveable, stackable, fluidContainer, splash, hangable, hookSouth, hookEast, rotatable, readable, writable, container, floorChange, elevation, fullGround, charges, corpse, light"`
	Limit int      `json:"limit,omitempty" jsonschema:"most results (default 100)"`
}

type itemArgs struct {
	ID int `json:"id" jsonschema:"item id (as stored in the map)"`
}

type creaturesArgs struct {
	Query string `json:"query,omitempty" jsonschema:"part of the name"`
	NPC   *bool  `json:"npc,omitempty" jsonschema:"true: only NPCs, false: only monsters"`
	Limit int    `json:"limit,omitempty" jsonschema:"most results (default 100)"`
}

func (s *Server) addCatalogTools(server *mcp.Server) {
	addTool(server, "list_brushes", "Finds brushes to draw with: grounds (with automatic borders unless noBorders), walls, wall decorations, doodads (trees, stones, furniture groups), carpets and tables. Pass name and kind to draw.",
		func(_ context.Context, args listBrushesArgs) (any, error) {
			palette, err := s.maps.Palette()
			if err != nil {
				return nil, err
			}
			brushes := palette.Brushes
			if args.Tileset != "" {
				tileset := findTileset(palette, args.Tileset)
				if tileset == nil {
					return nil, errors.New("unknown tileset " + args.Tileset)
				}
				brushes = slices.Concat(tileset.Terrain, tileset.Doodads)
			}
			query := strings.ToLower(args.Query)
			limit := listLimit(args.Limit)
			found := []brushEntry{}
			more := false
			for _, brush := range brushes {
				entry := newBrushEntry(brush)
				if (args.Kind != "" && entry.Kind != args.Kind) || !strings.Contains(strings.ToLower(brush.Name), query) {
					continue
				}
				if len(found) == limit {
					more = true
					break
				}
				if entry.Kind == materials.TypeGround {
					hasBorders, err := s.maps.GroundHasBorders(entry.Name)
					entry.NoBorders = err == nil && !hasBorders
				}
				found = append(found, entry)
			}
			return map[string]any{"brushes": found, "more": more}, nil
		})

	addTool(server, "get_brush", "The definition of one brush: its items with their chances, borders, wall pieces and doors, or the tile groups of a doodad.",
		func(_ context.Context, args nameArgs) (any, error) {
			return s.maps.Brush(args.Name)
		})

	addTool(server, "list_tilesets", "The tilesets (groups of brushes and items, like the editor's library) with how much they hold.",
		func(context.Context, struct{}) (any, error) {
			palette, err := s.maps.Palette()
			if err != nil {
				return nil, err
			}
			type tilesetEntry struct {
				Name    string `json:"name"`
				Terrain int    `json:"terrainBrushes"`
				Doodads int    `json:"doodadBrushes"`
				Items   int    `json:"items"`
				Auto    bool   `json:"generated,omitempty"`
			}
			out := []tilesetEntry{}
			for _, tileset := range palette.Tilesets {
				out = append(out, tilesetEntry{Name: tileset.Name, Terrain: len(tileset.Terrain), Doodads: len(tileset.Doodads), Items: len(tileset.Items) + len(tileset.Raw), Auto: tileset.Auto})
			}
			return out, nil
		})

	addTool(server, "get_tileset", "The brushes and items of one tileset, items with their names.",
		func(_ context.Context, args tilesetArgs) (any, error) {
			palette, err := s.maps.Palette()
			if err != nil {
				return nil, err
			}
			tileset := findTileset(palette, args.Name)
			if tileset == nil {
				return nil, errors.New("unknown tileset " + args.Name)
			}
			ids := slices.Concat(tileset.Items, tileset.Raw)
			limit := listLimit(args.Limit)
			items := []app.ItemSummary{}
			for _, id := range ids[:min(len(ids), limit)] {
				if item, err := s.maps.Item(id); err == nil {
					items = append(items, item)
				}
			}
			return map[string]any{
				"name": tileset.Name, "terrain": brushEntries(tileset.Terrain), "doodads": brushEntries(tileset.Doodads),
				"items": items, "moreItems": len(ids) > limit,
			}, nil
		})

	addTool(server, "search_items", "Finds item types by name, id or flags, with the brush that places each one. Items are placed with draw kind raw or add_item.",
		func(_ context.Context, args searchItemsArgs) (any, error) {
			return s.maps.SearchItems(args.Query, args.Flags, listLimit(args.Limit))
		})

	addTool(server, "get_item", "One item type: name, client id, kind, flags and brush.",
		func(_ context.Context, args itemArgs) (any, error) {
			if args.ID <= 0 || args.ID > 0xFFFF {
				return nil, errors.New("bad item id")
			}
			return s.maps.Item(uint16(args.ID))
		})

	addTool(server, "list_creatures", "Monsters and NPCs that can be placed (from the brush file and the server data folder).",
		func(_ context.Context, args creaturesArgs) (any, error) {
			creatures, err := s.maps.Creatures()
			if err != nil {
				return nil, err
			}
			query := strings.ToLower(args.Query)
			limit := listLimit(args.Limit)
			type creatureEntry struct {
				Name string `json:"name"`
				NPC  bool   `json:"npc,omitempty"`
			}
			found := []creatureEntry{}
			more := false
			for _, creature := range creatures {
				if (args.NPC != nil && creature.NPC != *args.NPC) || !strings.Contains(strings.ToLower(creature.Name), query) {
					continue
				}
				if len(found) == limit {
					more = true
					break
				}
				found = append(found, creatureEntry{Name: creature.Name, NPC: creature.NPC})
			}
			return map[string]any{"creatures": found, "more": more}, nil
		})
}

// findTileset returns a tileset by name (case-insensitive).
func findTileset(palette app.Palette, name string) *app.PaletteTileset {
	for i := range palette.Tilesets {
		if strings.EqualFold(palette.Tilesets[i].Name, name) {
			return &palette.Tilesets[i]
		}
	}
	return nil
}
