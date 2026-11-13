package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
)

type suggestArgs struct {
	MapArg
	Pos  Pos    `json:"pos" jsonschema:"the tile to fill"`
	Kind string `json:"kind,omitempty" jsonschema:"brush being drawn with, to also get what goes with it: ground, wall, doodad, carpet, table or raw"`
	Name string `json:"name,omitempty" jsonschema:"brush name for kind"`
	Item int    `json:"item,omitempty" jsonschema:"item id for kind raw"`
}

type variantsArgs struct {
	MapArg
	Item int `json:"item" jsonschema:"item id"`
}

type swapItemArgs struct {
	MapArg
	ItemRefArg
	Item int `json:"item" jsonschema:"new item id, e.g. one of variants"`
}

type completeArgs struct {
	MapArg
	Pos   Pos  `json:"pos" jsonschema:"tile of the part already placed"`
	Item  int  `json:"item" jsonschema:"item id of that part"`
	Place bool `json:"place,omitempty" jsonschema:"also place the missing parts (one undo step)"`
}

// namedSuggestion is a suggestion with the item's name.
type namedSuggestion struct {
	app.Suggestion
	Name string `json:"name,omitempty"`
}

type namedItem struct {
	Item uint16 `json:"item"`
	Name string `json:"name,omitempty"`
}

type namedPart struct {
	Pos Pos `json:"pos"`
	namedItem
}

func itemID(id int) (uint16, error) {
	if id <= 0 || id > 0xFFFF {
		return 0, fmt.Errorf("item id %d is out of range", id)
	}
	return uint16(id), nil
}

func (s *Server) itemName(id uint16) string {
	item, err := s.maps.Item(id)
	if err != nil {
		return ""
	}
	return item.Name
}

func (s *Server) nameSuggestions(list []app.Suggestion) []namedSuggestion {
	out := make([]namedSuggestion, 0, len(list))
	for _, suggestion := range list {
		out = append(out, namedSuggestion{Suggestion: suggestion, Name: s.itemName(suggestion.Item)})
	}
	return out
}

func (s *Server) addSuggestTools(server *mcp.Server) {
	addTool(server, "suggest_items", "What usually goes on a tile, learned from the rest of the map: items that fit the place (here), items that go with the brush being drawn (with) and grounds that border the tile's ground (grounds), best first with the reason. Use it to decorate in the map's own style.",
		func(_ context.Context, args suggestArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			brush := app.SuggestBrush{Kind: args.Kind, Name: args.Name}
			if args.Kind == "raw" {
				if brush.Item, err = itemID(args.Item); err != nil {
					return nil, err
				}
			}
			suggestions, err := s.maps.Suggest(id, pos, brush)
			if err != nil {
				return nil, err
			}
			result := map[string]any{"here": s.nameSuggestions(suggestions.Here), "with": s.nameSuggestions(suggestions.With), "grounds": s.nameSuggestions(suggestions.Grounds)}
			if suggestions.Pending > 0 {
				result["pending"] = fmt.Sprintf("%d map blocks are still being counted: ask again for fuller results", suggestions.Pending)
			}
			return result, nil
		})

	addTool(server, "variants", "Items an item can be swapped for, the item first: its family (other directions, colors, states), then items the map uses in the same places, then items that look alike. Swap with swap_item.",
		func(_ context.Context, args variantsArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			item, err := itemID(args.Item)
			if err != nil {
				return nil, err
			}
			variants, err := s.maps.Variants(id, item)
			if err != nil {
				return nil, err
			}
			out := make([]namedItem, 0, len(variants))
			for _, variant := range variants {
				out = append(out, namedItem{Item: variant, Name: s.itemName(variant)})
			}
			return map[string]any{"variants": out}, nil
		})

	addTool(server, "swap_item", "Replaces an item on the map by another one in the same place of the stack, keeping its attributes.",
		func(_ context.Context, args swapItemArgs) (any, error) {
			item, err := itemID(args.Item)
			if err != nil {
				return nil, err
			}
			ref, err := args.ItemRefArg.ref()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SwapItem(id, ref, item, 0) })
		})

	addTool(server, "complete_object", "The parts missing around one part of an object made of several items (a big tree, a statue, a ship), as the map shows them elsewhere; with place they are put on the map.",
		func(_ context.Context, args completeArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			item, err := itemID(args.Item)
			if err != nil {
				return nil, err
			}
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			parts, err := s.maps.Completions(id, pos, item)
			if err != nil {
				return nil, err
			}
			named := make([]namedPart, 0, len(parts))
			for _, part := range parts {
				named = append(named, namedPart{Pos: Pos{X: int(part.Pos.X), Y: int(part.Pos.Y), Z: int(part.Pos.Z)}, namedItem: namedItem{Item: part.Item, Name: s.itemName(part.Item)}})
			}
			result := map[string]any{"missing": named}
			if !args.Place || len(parts) == 0 {
				return result, nil
			}
			change, err := s.maps.PlaceItems(id, parts, s.nextStroke())
			s.changed(id, change)
			if err != nil {
				return nil, err
			}
			result["placed"] = editResult(id, change)
			return result, nil
		})
}
