package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type findPathArgs struct {
	MapArg
	From  Pos `json:"from"`
	To    Pos `json:"to"`
	Limit int `json:"limit,omitempty" jsonschema:"most tiles searched (default 250000, at most 2000000)"`
}

type validateArgs struct {
	MapArg
	Limit int `json:"limit,omitempty" jsonschema:"most problems listed (default 100); counts cover all"`
}

func (s *Server) addCheckTools(server *mcp.Server) {
	addTool(server, "find_path", "Can a player walk from one tile to another? The shortest way (diagonal steps included) through doors, stairs and holes, ladders, rope spots and teleports, as its turns and the points where it passes a door or changes floor (via). Without a way it gives the reached tile closest to the target: where to look for the wall or gap in the way. Floor change directions are guessed: down when a tile lies under it, up when none lies over it.",
		func(_ context.Context, args findPathArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			from, err := args.From.position()
			if err != nil {
				return nil, err
			}
			to, err := args.To.position()
			if err != nil {
				return nil, err
			}
			return s.maps.FindPath(id, from, to, args.Limit)
		})

	addTool(server, "validate_map", "Checks a map for mistakes that break it in game, without changing it: temples, house exits, waypoints and teleport destinations that cannot be stood on, houses without tiles or exit, tiles of missing houses or zones, duplicate unique ids, creatures on blocking tiles, spawns without creatures, unknown items and floor changes that lead nowhere. Run it before save_map.",
		func(_ context.Context, args validateArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			limit := args.Limit
			if limit <= 0 {
				limit = defaultListLimit
			}
			return s.maps.Validate(id, min(limit, maxListLimit))
		})
}
