package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

type townArgs struct {
	MapArg
	ID     int    `json:"id,omitempty" jsonschema:"town id to change; 0 creates a town"`
	Name   string `json:"name"`
	Temple Pos    `json:"temple" jsonschema:"temple position, where players of the town respawn"`
}

type houseArgs struct {
	MapArg
	ID        int     `json:"id,omitempty" jsonschema:"house id to change; 0 creates a house. Missing fields keep their values"`
	Name      *string `json:"name,omitempty"`
	TownID    *int    `json:"townId,omitempty"`
	Rent      *int    `json:"rent,omitempty"`
	Guildhall *bool   `json:"guildhall,omitempty"`
	Exit      *Pos    `json:"exit,omitempty" jsonschema:"entrance tile outside the house"`
	Beds      *int    `json:"beds,omitempty"`
	ClientID  *int    `json:"clientId,omitempty"`
}

type idArgs struct {
	MapArg
	ID int `json:"id"`
}

type waypointArgs struct {
	MapArg
	OldName string `json:"oldName,omitempty" jsonschema:"name of the waypoint to change; empty creates one"`
	Name    string `json:"name"`
	Pos     Pos    `json:"pos"`
}

type zoneArgs struct {
	MapArg
	ID   int    `json:"id,omitempty" jsonschema:"zone id to rename; 0 creates a zone"`
	Name string `json:"name"`
}

func (s *Server) addWorldTools(server *mcp.Server) {
	addTool(server, "save_town", "Creates or changes a town (name and temple). count in the result is the town id.",
		func(_ context.Context, args townArgs) (any, error) {
			temple, err := args.Temple.position()
			if err != nil {
				return nil, err
			}
			if args.ID < 0 {
				return nil, errors.New("bad town id")
			}
			town := otmap.Town{ID: uint32(args.ID), Name: args.Name, Temple: temple}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SaveTown(id, town) })
		})

	addTool(server, "remove_town", "Deletes a town.",
		func(_ context.Context, args idArgs) (any, error) {
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RemoveTown(id, uint32(max(args.ID, 0))) })
		})

	addTool(server, "save_house", "Creates or changes a house: name, town, rent, entrance... count in the result is the house id. Its tiles are drawn with kind house, its entrance with kind houseExit.",
		func(_ context.Context, args houseArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			house, err := s.house(id, args)
			if err != nil {
				return nil, err
			}
			return s.edit(id, func(id int) (app.MapChange, error) { return s.maps.SaveHouse(id, house) })
		})

	addTool(server, "remove_house", "Deletes a house; its tiles stay without house.",
		func(_ context.Context, args idArgs) (any, error) {
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RemoveHouse(id, uint32(max(args.ID, 0))) })
		})

	addTool(server, "get_house_tiles", "The tiles of a house.",
		func(_ context.Context, args idArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			return s.maps.HouseTiles(id, uint32(max(args.ID, 0)))
		})

	addTool(server, "save_waypoint", "Creates, moves or renames a waypoint (a named position used by scripts and NPCs).",
		func(_ context.Context, args waypointArgs) (any, error) {
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			waypoint := otmap.Waypoint{Name: args.Name, Pos: pos}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SaveWaypoint(id, args.OldName, waypoint) })
		})

	addTool(server, "remove_waypoint", "Deletes a waypoint.",
		func(_ context.Context, args nameMapArgs) (any, error) {
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RemoveWaypoint(id, args.Name) })
		})

	addTool(server, "save_zone", "Creates or renames a zone (a named group of tiles for scripts). count in the result is the zone id; draw its tiles with kind zone.",
		func(_ context.Context, args zoneArgs) (any, error) {
			if args.ID < 0 || args.ID > 0xFFFF {
				return nil, errors.New("bad zone id")
			}
			zone := otmap.Zone{ID: uint16(args.ID), Name: args.Name}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SaveZone(id, zone) })
		})

	addTool(server, "remove_zone", "Deletes a zone.",
		func(_ context.Context, args idArgs) (any, error) {
			if args.ID < 0 || args.ID > 0xFFFF {
				return nil, errors.New("bad zone id")
			}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.RemoveZone(id, uint16(args.ID)) })
		})
}

type nameMapArgs struct {
	MapArg
	Name string `json:"name"`
}

// house builds the house a save_house call stores: the existing one with
// the given fields changed.
func (s *Server) house(id int, args houseArgs) (otmap.House, error) {
	house := otmap.House{}
	if args.ID < 0 {
		return house, errors.New("bad house id")
	}
	if args.ID != 0 {
		info, err := s.maps.Info(id)
		if err != nil {
			return house, err
		}
		found := false
		for _, existing := range info.Houses {
			if existing.ID == uint32(args.ID) {
				house, found = existing.House, true
			}
		}
		if !found {
			return house, fmt.Errorf("there is no house %d (id 0 creates one)", args.ID)
		}
	} else if args.Name == nil {
		return house, errors.New("a new house needs a name")
	}
	setIf(&house.Name, args.Name)
	setIf(&house.Guildhall, args.Guildhall)
	if args.TownID != nil {
		house.TownID = uint32(max(*args.TownID, 0))
	}
	if args.Rent != nil {
		house.Rent = int32(*args.Rent)
	}
	if args.Beds != nil {
		house.Beds = int32(*args.Beds)
	}
	if args.ClientID != nil {
		house.ClientID = uint32(max(*args.ClientID, 0))
	}
	if args.Exit != nil {
		exit, err := args.Exit.position()
		if err != nil {
			return house, err
		}
		house.Exit = exit
	}
	return house, nil
}
