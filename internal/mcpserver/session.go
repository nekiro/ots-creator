package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// MapArg selects the map of a tool.
type MapArg struct {
	MapID int `json:"mapId,omitempty" jsonschema:"open map id; 0 or none: the map shown in the editor"`
}

// mapSummary is an open map in get_status.
type mapSummary struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Active  bool   `json:"active,omitempty"`
	Changed bool   `json:"unsavedChanges"`
	Tiles   int    `json:"tiles"`
	Width   uint16 `json:"width"`
	Height  uint16 `json:"height"`
	// Bounds is the used area: min x, min y, max x, max y.
	Bounds []int `json:"bounds,omitempty"`
}

func summarize(info app.MapInfo, active int) mapSummary {
	summary := mapSummary{ID: info.ID, Name: info.Name, Path: info.Path, Active: info.ID == active, Changed: info.Changed, Tiles: info.Tiles, Width: info.Props.Width, Height: info.Props.Height}
	if info.HasBounds {
		summary.Bounds = info.Bounds[:]
	}
	return summary
}

type statusResult struct {
	Client    string          `json:"client"`
	Version   string          `json:"version,omitempty"`
	Data      *app.DataInfo   `json:"itemData,omitempty"`
	Maps      []mapSummary    `json:"maps"`
	Recent    []app.RecentMap `json:"recentMaps,omitempty"`
	DoorTypes []string        `json:"doorTypes"`
}

type openClientArgs struct {
	Path string `json:"path" jsonschema:"client folder, or its Tibia.dat, Tibia.spr or .otfi file; a Tibia 12+ assets folder works too"`
	// Discard allows replacing a client with unsaved object changes.
	Discard bool `json:"discardChanges,omitempty" jsonschema:"replace the open client even when its objects have unsaved changes"`
}

type serverDataArgs struct {
	DetectFrom    string `json:"detectFrom,omitempty" jsonschema:"server data folder or map path to look for items.otb and items.xml in; found files fill the paths left empty"`
	ItemsMode     string `json:"itemsMode,omitempty" jsonschema:"auto (items.otb when found), otb or client (map ids are client ids)"`
	OTBPath       string `json:"otbPath,omitempty" jsonschema:"items.otb path"`
	XMLPath       string `json:"xmlPath,omitempty" jsonschema:"items.xml path"`
	MaterialsPath string `json:"materialsPath,omitempty" jsonschema:"brush file (JSON); empty: the default one of the client version"`
	ServerDir     string `json:"serverDir,omitempty" jsonschema:"server data folder scanned for monsters and NPCs"`
}

type newMapArgs struct {
	Width       int    `json:"width,omitempty" jsonschema:"map width in tiles (default 2048)"`
	Height      int    `json:"height,omitempty" jsonschema:"map height in tiles (default 2048)"`
	Description string `json:"description,omitempty"`
	Fill        string `json:"fill,omitempty" jsonschema:"ground brush painted over the whole ground floor (only for maps up to 4 million tiles)"`
	// SeparateSpawns stores monsters and NPCs in two files.
	SeparateSpawns bool `json:"separateSpawns,omitempty" jsonschema:"keep monsters and NPCs in separate files (servers on client ids)"`
}

type pathArgs struct {
	Path string `json:"path" jsonschema:"file path"`
}

type saveMapArgs struct {
	MapArg
	Path string `json:"path,omitempty" jsonschema:"where to save (.otbm); empty: the map's own file. Houses, spawns and zones are written next to it"`
}

type closeMapArgs struct {
	MapArg
	Discard bool `json:"discardChanges,omitempty" jsonschema:"close even with unsaved changes (they are lost)"`
}

type propertiesArgs struct {
	MapArg
	Description *string `json:"description,omitempty"`
	Width       *int    `json:"width,omitempty"`
	Height      *int    `json:"height,omitempty"`
	HouseFile   *string `json:"houseFile,omitempty" jsonschema:"house file name written next to the map"`
	SpawnFile   *string `json:"spawnFile,omitempty"`
	NpcFile     *string `json:"npcFile,omitempty"`
	ZoneFile    *string `json:"zoneFile,omitempty"`
	// SeparateSpawns: see newMapArgs.
	SeparateSpawns *bool `json:"separateSpawns,omitempty"`
}

type importArgs struct {
	MapArg
	Path    string `json:"path" jsonschema:"OTBM file to merge into the map"`
	DX      int    `json:"dx,omitempty" jsonschema:"x offset of the imported tiles"`
	DY      int    `json:"dy,omitempty"`
	DZ      int    `json:"dz,omitempty"`
	Houses  string `json:"houses,omitempty" jsonschema:"merge (keep house ids, existing win), renumber or skip"`
	Replace bool   `json:"replace,omitempty" jsonschema:"replace existing tiles instead of merging items"`
	Towns   bool   `json:"towns,omitempty" jsonschema:"also add missing towns and waypoints"`
}

type stepsArgs struct {
	MapArg
	Steps int `json:"steps,omitempty" jsonschema:"how many steps (default 1)"`
}

type showArgs struct {
	MapArg
	Pos Pos `json:"pos"`
}

func (s *Server) addSessionTools(server *mcp.Server) {
	addTool(server, "get_status", "The open client, its item and brush data, the open maps (with ids) and the recent maps. Call it first.",
		func(context.Context, struct{}) (any, error) {
			status := s.maps.Status()
			active := s.maps.ActiveMap()
			result := statusResult{Client: "none: use open_client", Maps: []mapSummary{}, Recent: status.Recent, DoorTypes: doorTypes()}
			if state := s.projects.State(); state.Open {
				result.Client, result.Version = state.Info.DatPath, state.Info.Version.Name
			}
			if status.Data.Ready {
				result.Data = &status.Data
			}
			for _, info := range status.Maps {
				result.Maps = append(result.Maps, summarize(info, active))
			}
			return result, nil
		})

	addTool(server, "open_client", "Opens a client (Tibia.dat/spr or a Tibia 12+ assets folder). The map editor draws with its sprites, so a client must be open before maps.",
		func(_ context.Context, args openClientArgs) (any, error) {
			if state := s.projects.State(); state.Open && state.Info.Changed && !args.Discard {
				return nil, errors.New("the open client has unsaved object changes: save them in the app or pass discardChanges")
			}
			files, err := s.projects.Inspect(args.Path)
			if err != nil {
				return nil, err
			}
			if files.Detected == nil {
				return nil, errors.New("the client version was not recognized from the file signatures; open it in the app")
			}
			state, err := s.projects.Open(app.OpenRequest{Format: files.Format, DatPath: files.DatPath, SprPath: files.SprPath, Version: files.Detected, Features: files.Features})
			if err != nil {
				return nil, err
			}
			result := map[string]any{"client": state.Info.DatPath, "version": state.Info.Version.Name}
			if state.OTBError != "" {
				result["itemsOtbError"] = state.OTBError
			}
			return result, nil
		})

	addTool(server, "set_server_data", "Sets where the map editor takes item data (items.otb, items.xml), brushes and creatures from, and reloads them. Missing fields keep their current values.",
		func(_ context.Context, args serverDataArgs) (any, error) {
			config := s.maps.Status().Data.Config
			if args.DetectFrom != "" {
				detected := s.maps.DetectData(args.DetectFrom)
				config.OTBPath, config.XMLPath = detected.OTBPath, detected.XMLPath
				if detected.ServerDir != "" {
					config.ServerDir = detected.ServerDir
				}
			}
			setIfGiven(&config.ItemsMode, args.ItemsMode)
			setIfGiven(&config.OTBPath, args.OTBPath)
			setIfGiven(&config.XMLPath, args.XMLPath)
			setIfGiven(&config.MaterialsPath, args.MaterialsPath)
			setIfGiven(&config.ServerDir, args.ServerDir)
			info, err := s.maps.SetData(config)
			if err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventData})
			return info, nil
		})

	addTool(server, "new_map", "Creates an empty map and shows it in the editor. Returns its id.",
		func(_ context.Context, args newMapArgs) (any, error) {
			options := app.NewMapOptions{Width: 2048, Height: 2048, Description: args.Description, Fill: args.Fill, SpawnFormat: otmap.SpawnsSingle}
			if args.Width > 0 {
				options.Width = uint16(min(args.Width, 0xFFFF))
			}
			if args.Height > 0 {
				options.Height = uint16(min(args.Height, 0xFFFF))
			}
			if args.SeparateSpawns {
				options.SpawnFormat = otmap.SpawnsSeparate
			}
			info, err := s.maps.NewMap(options)
			if err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventOpened, ID: info.ID})
			return summarize(info, info.ID), nil
		})

	addTool(server, "open_map", "Opens an OTBM map (with its houses, spawns and zones) and shows it in the editor. Returns its id and load warnings.",
		func(_ context.Context, args pathArgs) (any, error) {
			opened, err := s.maps.OpenMap(args.Path)
			if err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventOpened, ID: opened.Info.ID})
			return map[string]any{"map": summarize(opened.Info, opened.Info.ID), "warnings": opened.Warnings}, nil
		})

	addTool(server, "save_map", "Saves a map as OTBM with its house, spawn and zone files.",
		func(_ context.Context, args saveMapArgs) (any, error) {
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.SaveMap(id, args.Path) })
		})

	addTool(server, "close_map", "Closes a map without saving.",
		func(_ context.Context, args closeMapArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			info, err := s.maps.Info(id)
			if err != nil {
				return nil, err
			}
			if info.Changed && !args.Discard {
				return nil, errors.New("the map has unsaved changes: save_map first or pass discardChanges")
			}
			s.maps.CloseMap(id)
			s.notify(EventMap, Event{Kind: EventClosed, ID: id})
			return map[string]any{"closed": id}, nil
		})

	addTool(server, "get_map_info", "A map's properties, used area, towns, houses, waypoints, zones and undo state.",
		func(_ context.Context, args MapArg) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			return s.maps.Info(id)
		})

	addTool(server, "set_map_properties", "Changes the map header: description, size and the names of the house, spawn, NPC and zone files. Missing fields stay.",
		func(_ context.Context, args propertiesArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			info, err := s.maps.Info(id)
			if err != nil {
				return nil, err
			}
			props := info.Props
			setIf(&props.Description, args.Description)
			setIf(&props.HouseFile, args.HouseFile)
			setIf(&props.SpawnFile, args.SpawnFile)
			setIf(&props.NpcFile, args.NpcFile)
			setIf(&props.ZoneFile, args.ZoneFile)
			if args.Width != nil {
				props.Width = uint16(min(max(*args.Width, 0), 0xFFFF))
			}
			if args.Height != nil {
				props.Height = uint16(min(max(*args.Height, 0), 0xFFFF))
			}
			if args.SeparateSpawns != nil {
				props.SpawnFormat = otmap.SpawnsSingle
				if *args.SeparateSpawns {
					props.SpawnFormat = otmap.SpawnsSeparate
				}
			}
			return s.edit(id, func(id int) (app.MapChange, error) { return s.maps.SetProperties(id, props) })
		})

	addTool(server, "import_map", "Merges another OTBM map file into a map at an offset.",
		func(_ context.Context, args importArgs) (any, error) {
			options := mapedit.ImportOptions{DX: args.DX, DY: args.DY, DZ: args.DZ, Houses: args.Houses, Replace: args.Replace, Towns: args.Towns}
			return s.edit(args.MapID, func(id int) (app.MapChange, error) { return s.maps.ImportMap(id, args.Path, options) })
		})

	addTool(server, "undo", "Reverts the last edits of a map.",
		func(_ context.Context, args stepsArgs) (any, error) {
			return s.repeat(args, s.maps.Undo)
		})

	addTool(server, "redo", "Applies undone edits of a map again.",
		func(_ context.Context, args stepsArgs) (any, error) {
			return s.repeat(args, s.maps.Redo)
		})

	addTool(server, "get_history", "The undo and redo steps of a map (label and tile count).",
		func(_ context.Context, args MapArg) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			return s.maps.History(id)
		})

	addTool(server, "show_in_editor", "Centers the editor's view on a position, so the user sees what you work on.",
		func(_ context.Context, args showArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			s.notify(EventMap, Event{Kind: EventShow, ID: id, Pos: &pos})
			return map[string]any{"shown": pos}, nil
		})
}

// repeat runs undo or redo a number of times as one tool call.
func (s *Server) repeat(args stepsArgs, step func(id int) (app.MapChange, error)) (any, error) {
	id, err := s.mapID(args.MapID)
	if err != nil {
		return nil, err
	}
	var changes []app.MapChange
	for range max(args.Steps, 1) {
		change, err := step(id)
		if err != nil {
			break
		}
		changes = append(changes, change)
	}
	change := mergeChanges(changes)
	s.changed(id, change)
	result := editResult(id, change)
	result.Count = len(changes)
	return result, nil
}

// setIf sets target to an optional argument that was given.
func setIf[T any](target *T, value *T) {
	if value != nil {
		*target = *value
	}
}

// setIfGiven sets target to a text argument that is not empty.
func setIfGiven(target *string, value string) {
	if value != "" {
		*target = value
	}
}
