package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

type bordersArgs struct {
	IDs  []uint32 `json:"ids,omitempty" jsonschema:"border ids, as brushes refer to them"`
	Item int      `json:"item,omitempty" jsonschema:"or: every border that uses this item id"`
}

// borderInput is a materials.Border whose id may be left out.
type borderInput struct {
	ID               uint32            `json:"id,omitempty" jsonschema:"border to replace; leave out to create one"`
	Group            uint16            `json:"group,omitempty"`
	Edges            map[string]uint16 `json:"edges" jsonschema:"n, e, s, w (sides), cnw, cne, csw, cse (outer corners) and dnw, dne, dse, dsw (inner corners) to item ids"`
	GroundEquivalent uint16            `json:"groundEquivalent,omitempty"`
	Optional         bool              `json:"optional,omitempty" jsonschema:"only placed with the optional border (draw kind optionalBorder)"`
}

type saveBorderArgs struct {
	Border borderInput `json:"border"`
}

type saveBrushArgs struct {
	Brush    materials.Brush `json:"brush" jsonschema:"the whole brush, as get_brush returns it; type is ground, wall, decoration, doodad, carpet or table"`
	OldName  string          `json:"oldName,omitempty" jsonschema:"current name of the brush to replace or rename; leave out to create one"`
	Tilesets []string        `json:"tilesets,omitempty" jsonschema:"tilesets that list the brush afterwards (missing ones are created); leave out to keep them as they are"`
}

type tilesetItemsArgs struct {
	Name  string `json:"name" jsonschema:"tileset, created when missing"`
	Items []int  `json:"items" jsonschema:"item ids to list in it as raw items"`
}

type saveCreatureArgs struct {
	Creature materials.CreatureType `json:"creature" jsonschema:"name, npc and the outfit it shows in the editor (lookType of the client's outfits, colors 0-132, addons 0-3, or lookItem)"`
}

var brushTypes = []string{materials.TypeGround, materials.TypeWall, materials.TypeDecoration, materials.TypeDoodad, materials.TypeCarpet, materials.TypeTable}

// unknownItems returns the ids of a list the item data does not know.
func (s *Server) unknownItems(ids []uint16) []uint16 {
	var unknown []uint16
	for _, id := range ids {
		if id == 0 || slices.Contains(unknown, id) {
			continue
		}
		if _, err := s.maps.Item(id); err != nil {
			unknown = append(unknown, id)
		}
	}
	return unknown
}

// brushItems returns the item ids a brush places.
func brushItems(brush materials.Brush) []uint16 {
	ids := []uint16{brush.Look}
	for _, item := range brush.Items {
		ids = append(ids, item.ID)
	}
	for _, part := range brush.Walls {
		if part == nil {
			continue
		}
		for _, item := range part.Items {
			ids = append(ids, item.ID)
		}
		for _, door := range part.Doors {
			ids = append(ids, door.ID)
		}
	}
	for _, items := range brush.Parts {
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	}
	for _, alternative := range brush.Alternatives {
		for _, single := range alternative.Singles {
			ids = append(ids, single.Item.ID)
		}
		for _, composite := range alternative.Composites {
			for _, tile := range composite.Tiles {
				for _, item := range tile.Items {
					ids = append(ids, item.ID)
				}
			}
		}
	}
	return ids
}

func checkBrush(brush materials.Brush) error {
	if !slices.Contains(brushTypes, brush.Type) {
		return fmt.Errorf("brush type %q: use one of %s", brush.Type, strings.Join(brushTypes, ", "))
	}
	for alignment := range brush.Walls {
		if !slices.Contains(materials.WallAlignments, alignment) {
			return fmt.Errorf("wall part %q: use one of %s", alignment, strings.Join(materials.WallAlignments, ", "))
		}
	}
	for _, part := range brush.Walls {
		if part == nil {
			continue
		}
		for _, door := range part.Doors {
			if !slices.Contains(materials.DoorTypes, door.Type) {
				return fmt.Errorf("door type %q: use one of %s", door.Type, strings.Join(materials.DoorTypes, ", "))
			}
		}
	}
	for _, rule := range brush.Borders {
		if rule.Align != "outer" && rule.Align != "inner" {
			return fmt.Errorf("border align %q: use outer or inner", rule.Align)
		}
	}
	return nil
}

func checkEdges(edges map[string]uint16) error {
	for edge := range edges {
		if !slices.Contains(materials.EdgeNames, edge) {
			return fmt.Errorf("edge %q: use one of %s", edge, strings.Join(materials.EdgeNames, ", "))
		}
	}
	return nil
}

// withWarnings adds a warning about unknown items to a result.
func withWarnings(result map[string]any, unknown []uint16) map[string]any {
	if len(unknown) > 0 {
		result["warnings"] = []string{fmt.Sprintf("the item data does not know items %v", unknown)}
	}
	return result
}

func (s *Server) addMaterialTools(server *mcp.Server) {
	addTool(server, "get_borders", "Borders by id (as brushes refer to them in borders), or every border that uses an item: the item of each edge and corner.",
		func(_ context.Context, args bordersArgs) (any, error) {
			if len(args.IDs) == 0 && args.Item <= 0 {
				return nil, errors.New("give border ids or an item")
			}
			borders, err := s.maps.Borders()
			if err != nil {
				return nil, err
			}
			found := []*materials.Border{}
			for _, border := range borders {
				uses := false
				for _, item := range border.Edges {
					uses = uses || int(item) == args.Item
				}
				if slices.Contains(args.IDs, border.ID) || (args.Item > 0 && uses) {
					found = append(found, border)
				}
			}
			return map[string]any{"borders": found}, nil
		})

	addTool(server, "save_border", "Creates or replaces a border (the items placed along the edges between grounds); ground brushes use it by id in their borders. Written to the brush file at once: undo does not revert it. count in the result is the border id.",
		func(_ context.Context, args saveBorderArgs) (any, error) {
			if err := checkEdges(args.Border.Edges); err != nil {
				return nil, err
			}
			if len(args.Border.Edges) == 0 {
				return nil, errors.New("the border needs edges")
			}
			ids := make([]uint16, 0, len(args.Border.Edges))
			for _, item := range args.Border.Edges {
				ids = append(ids, item)
			}
			id, err := s.maps.SaveBorder(materials.Border(args.Border))
			if err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return withWarnings(map[string]any{"count": id}, s.unknownItems(ids)), nil
		})

	addTool(server, "save_brush", "Creates, replaces or renames a brush: take one from get_brush as a model and change its items, borders (by border id, see get_borders and save_border), wall pieces or doodad tiles. Written to the brush file at once: undo does not revert it.",
		func(_ context.Context, args saveBrushArgs) (any, error) {
			if err := checkBrush(args.Brush); err != nil {
				return nil, err
			}
			borders, err := s.maps.Borders()
			if err != nil {
				return nil, err
			}
			for _, rule := range args.Brush.Borders {
				if rule.Border == 0 && rule.Inline == nil {
					return nil, errors.New("every border rule needs a border id or an inline border")
				}
				if rule.Border != 0 && !slices.ContainsFunc(borders, func(border *materials.Border) bool { return border.ID == rule.Border }) {
					return nil, fmt.Errorf("no border %d: create it with save_border", rule.Border)
				}
			}
			if err := s.maps.SaveBrush(args.OldName, args.Brush, args.Tilesets); err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			saved, err := s.maps.Brush(args.Brush.Name)
			if err != nil {
				return nil, err
			}
			return withWarnings(map[string]any{"brush": saved}, s.unknownItems(brushItems(args.Brush))), nil
		})

	addTool(server, "remove_brush", "Deletes a brush and takes it out of its tilesets. Written to the brush file at once: undo does not revert it.",
		func(_ context.Context, args nameArgs) (any, error) {
			if _, err := s.maps.Brush(args.Name); err != nil {
				return nil, err
			}
			if err := s.maps.RemoveBrush(args.Name); err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return map[string]any{"removed": args.Name}, nil
		})

	addTool(server, "add_to_tileset", "Lists items in a tileset (created when missing), so they show in the editor's library; brushes are put in tilesets by save_brush.",
		func(_ context.Context, args tilesetItemsArgs) (any, error) {
			if strings.TrimSpace(args.Name) == "" || len(args.Items) == 0 {
				return nil, errors.New("give a tileset name and items")
			}
			ids := make([]uint16, 0, len(args.Items))
			for _, item := range args.Items {
				if item <= 0 || item > 0xFFFF {
					return nil, fmt.Errorf("item id %d is out of range", item)
				}
				ids = append(ids, uint16(item))
			}
			if unknown := s.unknownItems(ids); len(unknown) > 0 {
				return nil, fmt.Errorf("the item data does not know items %v", unknown)
			}
			if err := s.maps.AddToTileset(args.Name, ids); err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return map[string]any{"tileset": args.Name, "count": len(ids)}, nil
		})

	addTool(server, "save_creature", "Creates or changes a monster or NPC the editor can place, with the outfit it shows. Written to the brush file at once.",
		func(_ context.Context, args saveCreatureArgs) (any, error) {
			if err := s.maps.SaveCreature(args.Creature); err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return map[string]any{"creature": args.Creature.Name}, nil
		})
}
