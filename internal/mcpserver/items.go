package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// ItemAttrs are the attributes of an item on the map. Missing ones stay.
type ItemAttrs struct {
	ID          *int    `json:"id,omitempty" jsonschema:"change the item into another item id"`
	Subtype     *int    `json:"subtype,omitempty" jsonschema:"count of stackable items, charges or fluid type"`
	ActionID    *int    `json:"actionId,omitempty" jsonschema:"action id (0 removes)"`
	UniqueID    *int    `json:"uniqueId,omitempty" jsonschema:"unique id (0 removes)"`
	Text        *string `json:"text,omitempty" jsonschema:"text of signs, books and letters"`
	Description *string `json:"description,omitempty"`
	Dest        *Pos    `json:"dest,omitempty" jsonschema:"teleport destination"`
	ClearDest   bool    `json:"clearDest,omitempty" jsonschema:"remove the teleport destination"`
	DepotID     *int    `json:"depotId,omitempty" jsonschema:"depot (town) id of depot lockers"`
	DoorID      *int    `json:"doorId,omitempty" jsonschema:"house door id"`
}

func (attrs ItemAttrs) empty() bool { return attrs == ItemAttrs{} }

// uint16Arg converts an optional number argument.
func uint16Arg(name string, value *int) (*uint16, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 0 || *value > 0xFFFF {
		return nil, fmt.Errorf("%s is out of range (0-65535)", name)
	}
	converted := uint16(*value)
	return &converted, nil
}

func (attrs ItemAttrs) patch() (mapedit.ItemPatch, error) {
	patch := mapedit.ItemPatch{Text: attrs.Text, Description: attrs.Description, ClearDest: attrs.ClearDest}
	var err error
	for _, field := range []struct {
		name   string
		value  *int
		target **uint16
	}{
		{"id", attrs.ID, &patch.ID}, {"subtype", attrs.Subtype, &patch.Subtype}, {"actionId", attrs.ActionID, &patch.ActionID},
		{"uniqueId", attrs.UniqueID, &patch.UniqueID}, {"depotId", attrs.DepotID, &patch.DepotID},
	} {
		if *field.target, err = uint16Arg(field.name, field.value); err != nil {
			return patch, err
		}
	}
	if attrs.DoorID != nil {
		if *attrs.DoorID < 0 || *attrs.DoorID > 0xFF {
			return patch, errors.New("doorId is out of range (0-255)")
		}
		doorID := uint8(*attrs.DoorID)
		patch.DoorID = &doorID
	}
	if attrs.Dest != nil {
		dest, err := attrs.Dest.position()
		if err != nil {
			return patch, err
		}
		patch.Dest = &dest
	}
	return patch, nil
}

// ItemRefArg addresses an item on a tile.
type ItemRefArg struct {
	Pos   Pos   `json:"pos"`
	Index int   `json:"index" jsonschema:"index among the tile's items as get_tile lists them (bottom to top, ground not counted); -1 is the ground"`
	Path  []int `json:"path,omitempty" jsonschema:"indexes into container contents, outermost first"`
}

func (ref ItemRefArg) ref() (mapedit.ItemRef, error) {
	pos, err := ref.Pos.position()
	return mapedit.ItemRef{Pos: pos, Index: ref.Index, Path: ref.Path}, err
}

type addItemArgs struct {
	MapArg
	Pos  Pos `json:"pos"`
	Item int `json:"item" jsonschema:"item id"`
	ItemAttrs
}

type updateItemArgs struct {
	MapArg
	ItemRefArg
	ItemAttrs
}

type itemRefArgs struct {
	MapArg
	ItemRefArg
}

type moveItemArgs struct {
	MapArg
	Pos  Pos `json:"pos"`
	From int `json:"from" jsonschema:"item index"`
	To   int `json:"to" jsonschema:"new index"`
}

type containerArgs struct {
	MapArg
	ItemRefArg
	Item int `json:"item" jsonschema:"item id to put into the container"`
}

type setTileArgs struct {
	MapArg
	Pos     Pos      `json:"pos"`
	Flags   []string `json:"flags,omitempty" jsonschema:"the tile's flags, replacing the old ones: pz, noPvp, noLogout, pvpZone, refresh ([] clears)"`
	HouseID *int     `json:"houseId,omitempty" jsonschema:"house the tile belongs to (0: none)"`
	Zones   []int    `json:"zones,omitempty" jsonschema:"zone ids of the tile, replacing the old ones"`
}

type creatureArgs struct {
	MapArg
	Pos       Pos     `json:"pos"`
	Index     int     `json:"index" jsonschema:"creature index on the tile (get_tile)"`
	SpawnTime *int    `json:"spawnTime,omitempty" jsonschema:"respawn seconds"`
	Direction *int    `json:"direction,omitempty" jsonschema:"0 north, 1 east, 2 south, 3 west"`
	Weight    *int    `json:"weight,omitempty" jsonschema:"spawn weight (separate monster files)"`
	Name      *string `json:"name,omitempty" jsonschema:"another creature"`
	Remove    bool    `json:"remove,omitempty" jsonschema:"remove the creature"`
}

type spawnArgs struct {
	MapArg
	Pos    Pos  `json:"pos" jsonschema:"spawn center"`
	NPC    bool `json:"npc,omitempty" jsonschema:"the NPC spawn instead of the monster spawn"`
	Radius int  `json:"radius" jsonschema:"spawn radius in tiles; 0 removes the spawn"`
}

func (s *Server) addItemTools(server *mcp.Server) {
	addTool(server, "add_item", "Puts one item on a tile (it goes where the client stacks it) with optional attributes like actionId, uniqueId, text or a teleport destination.",
		func(_ context.Context, args addItemArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			if args.Item <= 0 || args.Item > 0xFFFF {
				return nil, errors.New("bad item id")
			}
			patch, err := args.ItemAttrs.patch()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) {
				change, err := s.maps.AddItem(id, pos, uint16(args.Item))
				if err != nil || args.ItemAttrs.empty() {
					return change, err
				}
				index, err := s.itemIndex(id, pos, uint16(args.Item))
				if err != nil {
					return change, err
				}
				updated, err := s.maps.UpdateItem(id, mapedit.ItemRef{Pos: pos, Index: index}, patch)
				return mergeChanges([]app.MapChange{change, updated}), err
			})
		})

	addTool(server, "update_item", "Changes attributes of an item on the map.",
		func(_ context.Context, args updateItemArgs) (any, error) {
			ref, err := args.ItemRefArg.ref()
			if err != nil {
				return nil, err
			}
			patch, err := args.ItemAttrs.patch()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.UpdateItem(id, ref, patch) })
		})

	addTool(server, "remove_item", "Removes one item (or the ground with index -1) from a tile.",
		func(_ context.Context, args itemRefArgs) (any, error) {
			ref, err := args.ItemRefArg.ref()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RemoveItem(id, ref) })
		})

	addTool(server, "move_item", "Moves an item to another place in its tile's stack.",
		func(_ context.Context, args moveItemArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.MoveItem(id, pos, args.From, args.To) })
		})

	addTool(server, "rotate_item", "Turns a rotatable item (chairs, beds...) into its next direction.",
		func(_ context.Context, args itemRefArgs) (any, error) {
			ref, err := args.ItemRefArg.ref()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RotateItem(id, ref) })
		})

	addTool(server, "switch_door", "Opens or closes a door.",
		func(_ context.Context, args itemRefArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SwitchDoor(id, pos, args.Index) })
		})

	addTool(server, "add_to_container", "Puts an item into a container on the map.",
		func(_ context.Context, args containerArgs) (any, error) {
			ref, err := args.ItemRefArg.ref()
			if err != nil {
				return nil, err
			}
			if args.Item <= 0 || args.Item > 0xFFFF {
				return nil, errors.New("bad item id")
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.AddToContainer(id, ref, uint16(args.Item)) })
		})

	addTool(server, "set_tile", "Sets the flags (protection zone...), house or zones of one tile. For many tiles draw with kind flag, house or zone.",
		func(_ context.Context, args setTileArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			var patch mapedit.TilePatch
			if args.Flags != nil {
				var flags uint32
				for _, name := range args.Flags {
					flag, err := tileFlag(name)
					if err != nil {
						return nil, err
					}
					flags |= flag
				}
				patch.Flags = &flags
			}
			if args.HouseID != nil {
				if *args.HouseID < 0 {
					return nil, errors.New("bad house id")
				}
				house := uint32(*args.HouseID)
				patch.HouseID = &house
			}
			if args.Zones != nil {
				zones, err := itemIDs(args.Zones)
				if err != nil {
					return nil, errors.New("bad zone id")
				}
				patch.Zones = &zones
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.UpdateTile(id, pos, patch) })
		})

	addTool(server, "update_creature", "Changes or removes a creature placed on a tile.",
		func(_ context.Context, args creatureArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			patch := mapedit.CreaturePatch{Name: args.Name, Remove: args.Remove}
			if args.SpawnTime != nil {
				spawnTime := int32(*args.SpawnTime)
				patch.SpawnTime = &spawnTime
			}
			if args.Direction != nil {
				direction := uint8(*args.Direction & 3)
				patch.Direction = &direction
			}
			if args.Weight != nil {
				weight := uint8(min(max(*args.Weight, 0), 0xFF))
				patch.Weight = &weight
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.UpdateCreature(id, pos, args.Index, patch) })
		})

	addTool(server, "set_spawn", "Creates a spawn area, changes its radius or removes it (radius 0). Creatures inside the radius belong to it.",
		func(_ context.Context, args spawnArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) {
				tile, err := s.maps.Tile(id, pos)
				if err != nil {
					return app.MapChange{}, err
				}
				exists := tile.Spawn != nil
				if args.NPC {
					exists = tile.SpawnNPC != nil
				}
				if exists || args.Radius <= 0 {
					return s.maps.UpdateSpawn(id, pos, args.NPC, int32(args.Radius))
				}
				kind := mapedit.KindSpawn
				if args.NPC {
					kind = mapedit.KindSpawnNPC
				}
				return s.maps.Draw(id, mapedit.DrawRequest{Kind: kind, Points: []otmap.Position{pos}, Size: args.Radius})
			})
		})
}

// itemIndex finds the index of the topmost item with an id on a tile (-1
// for the ground).
func (s *Server) itemIndex(id int, pos otmap.Position, item uint16) (int, error) {
	tile, err := s.maps.Tile(id, pos)
	if err != nil {
		return 0, err
	}
	for index := len(tile.Items) - 1; index >= 0; index-- {
		if tile.Items[index].ID == item {
			return index, nil
		}
	}
	if tile.Ground != nil && tile.Ground.ID == item {
		return -1, nil
	}
	return 0, errors.New("the added item was not found on the tile")
}
