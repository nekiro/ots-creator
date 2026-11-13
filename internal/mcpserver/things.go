package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"reflect"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// CategoryArg selects the kind of client object of a tool.
type CategoryArg struct {
	Category string `json:"category,omitempty" jsonschema:"item (default), outfit, effect or missile"`
}

func (arg CategoryArg) parse() (thing.Category, error) {
	if arg.Category == "" {
		return thing.CategoryItem, nil
	}
	return thing.ParseCategory(arg.Category)
}

type thingArgs struct {
	CategoryArg
	ID uint32 `json:"id" jsonschema:"client id of the object"`
}

type findThingsArgs struct {
	CategoryArg
	Name    string `json:"name,omitempty" jsonschema:"part of the name stored in the client (asset clients only)"`
	Flag    string `json:"flag,omitempty" jsonschema:"property that must be set, as named by get_thing (pickupable, stackable, hasLight...)"`
	Content string `json:"content,omitempty" jsonschema:"used (default: objects with sprites), empty or all"`
	Limit   int    `json:"limit,omitempty" jsonschema:"most ids returned (default 200)"`
}

type freeIDsArgs struct {
	CategoryArg
	From  uint32 `json:"from,omitempty" jsonschema:"first id to consider"`
	Count int    `json:"count,omitempty" jsonschema:"how many free ids (default 1, at most 100)"`
}

type renderThingsArgs struct {
	CategoryArg
	IDs       []uint32 `json:"ids" jsonschema:"objects to draw, one row each (at most 64)"`
	Scale     int      `json:"scale,omitempty" jsonschema:"pixel zoom 1-8 (default 4)"`
	FirstOnly bool     `json:"firstFrameOnly,omitempty" jsonschema:"draw only the first animation frame instead of all"`
}

type createThingArgs struct {
	CategoryArg
	ID          uint32  `json:"id" jsonschema:"client id to write; must be free unless replace is set (see free_ids)"`
	Sheet       string  `json:"sheet" jsonschema:"PNG sprite sheet: animation frames stacked top to bottom, patterns side by side; a 32x64 image is one 32x32 sprite with two frames"`
	Template    uint32  `json:"template,omitempty" jsonschema:"object whose properties, animation mode and frame durations are copied"`
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	DurationMs  uint32  `json:"durationMs,omitempty" jsonschema:"duration of every animation frame; default: the template's, else the client default"`
	Props       propMap `json:"props,omitempty" jsonschema:"properties to set over the template, as named by get_thing"`
	Replace     bool    `json:"replace,omitempty" jsonschema:"overwrite an object that already uses the id"`
}

type updateThingArgs struct {
	CategoryArg
	IDs         []uint32     `json:"ids,omitempty" jsonschema:"objects to change; or select them with where"`
	Where       *thingFilter `json:"where,omitempty" jsonschema:"change every object that matches instead of ids"`
	Props       propMap      `json:"props,omitempty" jsonschema:"properties to change, as named by get_thing"`
	Name        *string      `json:"name,omitempty"`
	Description *string      `json:"description,omitempty"`
	DurationMs  uint32       `json:"durationMs,omitempty" jsonschema:"duration of every animation frame"`
}

// thingFilter selects objects as find_things does.
type thingFilter struct {
	Name string `json:"name,omitempty" jsonschema:"part of the name stored in the client (asset clients only)"`
	Flag string `json:"flag,omitempty" jsonschema:"property that must be set, as named by get_thing"`
}

type idsArgs struct {
	CategoryArg
	IDs []uint32 `json:"ids"`
}

type duplicateThingsArgs struct {
	CategoryArg
	IDs     []uint32 `json:"ids" jsonschema:"objects to copy"`
	At      uint32   `json:"at,omitempty" jsonschema:"first id of the copies, in order; must be free unless replace is set (default: after the last object)"`
	Replace bool     `json:"replace,omitempty" jsonschema:"overwrite objects at the target ids"`
}

type moveThingsArgs struct {
	CategoryArg
	IDs        []uint32 `json:"ids" jsonschema:"objects to move, in id order"`
	At         uint32   `json:"at" jsonschema:"first target id"`
	Insert     bool     `json:"insert,omitempty" jsonschema:"put them before at and shift the objects between; otherwise they swap places with the objects at the targets"`
	UpdateRefs *bool    `json:"updateRefs,omitempty" jsonschema:"change item ids stored in items (market trade as and show as...) to the new ids (default true)"`
}

type importSheetArgs struct {
	CategoryArg
	ID    uint32 `json:"id"`
	Sheet string `json:"sheet" jsonschema:"PNG sprite sheet, see create_thing and export_sheet"`
	Group int    `json:"group,omitempty" jsonschema:"frame group to replace: 0 (default; idle of outfits) or 1 (walking of outfits)"`
}

type exportSheetArgs struct {
	CategoryArg
	ID         uint32 `json:"id"`
	Path       string `json:"path" jsonschema:"PNG file to write"`
	Group      int    `json:"group,omitempty" jsonschema:"frame group: 0 (default; idle of outfits), 1 (walking of outfits) or -1 (all groups stacked, when their layouts match)"`
	Background string `json:"background,omitempty" jsonschema:"transparent (default) or magenta (as Object Builder sheets)"`
}

type propMap map[string]any

// thingView is an object as get_thing shows it.
type thingView struct {
	ID          uint32         `json:"id"`
	Category    string         `json:"category"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Props       map[string]any `json:"props"`
	FrameGroups []groupView    `json:"frameGroups"`
	// ServerID is the id the attached items.otb gives an item once
	// save_client runs; the map uses server ids.
	ServerID   uint16 `json:"serverId,omitempty"`
	ServerNote string `json:"serverIdNote,omitempty"`
}

type groupView struct {
	Width      uint8                 `json:"width"`
	Height     uint8                 `json:"height"`
	Layers     uint8                 `json:"layers"`
	PatternX   uint8                 `json:"patternX"`
	PatternY   uint8                 `json:"patternY"`
	PatternZ   uint8                 `json:"patternZ"`
	Frames     uint8                 `json:"frames"`
	Mode       string                `json:"animationMode,omitempty"`
	LoopCount  int32                 `json:"loopCount,omitempty"`
	StartFrame int8                  `json:"startFrame,omitempty"`
	Durations  []thing.FrameDuration `json:"durations,omitempty"`
	Sprites    []uint32              `json:"sprites"`
}

type idName struct {
	ID   uint32 `json:"id"`
	Name string `json:"name,omitempty"`
}

func viewThing(object *thing.Thing) (thingView, error) {
	props, err := setProps(object.Props)
	if err != nil {
		return thingView{}, err
	}
	view := thingView{ID: object.ID, Category: object.Category.String(), Name: object.Name, Description: object.Description, Props: props}
	for _, group := range object.FrameGroups {
		groupInfo := groupView{Width: group.Width, Height: group.Height, Layers: group.Layers, PatternX: group.PatternX, PatternY: group.PatternY, PatternZ: group.PatternZ, Frames: group.Frames, Sprites: group.Sprites}
		if group.IsAnimation() {
			groupInfo.Mode, groupInfo.LoopCount, groupInfo.StartFrame, groupInfo.Durations = "async", group.LoopCount, group.StartFrame, group.Durations
			if group.Mode == thing.AnimationSync {
				groupInfo.Mode = "sync"
			}
		}
		view.FrameGroups = append(view.FrameGroups, groupInfo)
	}
	return view, nil
}

// setProps returns the properties that differ from their zero value, by
// their JSON names.
func setProps(props thing.Properties) (map[string]any, error) {
	data, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	all := map[string]any{}
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	kept := map[string]any{}
	for key, value := range all {
		if !isZeroJSON(value) {
			kept[key] = value
		}
	}
	return kept, nil
}

func isZeroJSON(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, inner := range typed {
			if !isZeroJSON(inner) {
				return false
			}
		}
		return true
	case []any:
		for _, inner := range typed {
			if !isZeroJSON(inner) {
				return false
			}
		}
		return true
	case nil:
		return true
	}
	return reflect.ValueOf(value).IsZero()
}

// applyDuration sets every frame of every animated group to durationMs.
func applyDuration(object *thing.Thing, durationMs uint32) {
	for _, group := range object.FrameGroups {
		if !group.IsAnimation() {
			continue
		}
		group.Durations = nil
		group.EnsureDurations(durationMs)
	}
}

// refreshItems reloads the item data of the map editor after the client
// objects changed, so names and sprites of the palette follow.
func (s *Server) refreshItems() {
	if !s.maps.Status().Data.Ready {
		return
	}
	if _, err := s.maps.Reload(); err == nil {
		s.notify(EventMap, Event{Kind: EventData})
	}
}

func (s *Server) thingResult(category thing.Category, id uint32) (any, error) {
	object, err := s.things.Get(category, id)
	if err != nil {
		return nil, err
	}
	view, err := viewThing(object)
	if err != nil || category != thing.CategoryItem {
		return view, err
	}
	serverIDs, err := s.projects.ItemServerIDs([]uint32{id})
	switch {
	case err != nil:
		view.ServerNote = err.Error()
	case serverIDs[id] != 0:
		view.ServerID = serverIDs[id]
	default:
		view.ServerNote = "the attached items.otb gives this item no server id: save_client only adds server ids for client ids past the last one it maps, so the map cannot place it"
	}
	return view, nil
}

// sheetLayout explains where the textures of a frame group are in its
// sprite sheet.
func sheetLayout(group *thing.FrameGroup, spriteSize int) string {
	width, height := group.SheetSize(spriteSize)
	return fmt.Sprintf("%dx%d px: textures of %dx%d px (width x height tiles of %d px), %d per row (patternZ*patternX*layers); texture n = ((((frame*patternZ+z)*patternY+y)*patternX+x)*layers+layer) goes left to right, then top to bottom",
		width, height, int(group.Width)*spriteSize, int(group.Height)*spriteSize, spriteSize, int(group.PatternZ)*int(group.PatternX)*int(group.Layers))
}

func (s *Server) addThingTools(server *mcp.Server) {
	addTool(server, "get_thing", "One object of the open client: name, the properties that are set, and per frame group the layout (size, layers, patterns, frames), animation timing and sprite ids.",
		func(_ context.Context, args thingArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			return s.thingResult(category, args.ID)
		})

	addTool(server, "find_things", "Searches the objects of the open client by stored name and/or a set property. Returns ids with names.",
		func(_ context.Context, args findThingsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			content := args.Content
			if content == "" {
				content = project.ContentUsed
			}
			ids, err := s.things.Find(category, project.Filter{Content: content, Flag: args.Flag, Name: args.Name})
			if err != nil {
				return nil, err
			}
			limit := args.Limit
			if limit <= 0 {
				limit = 200
			}
			names, err := s.things.Names(category)
			if err != nil {
				return nil, err
			}
			result := map[string]any{"total": len(ids)}
			found := make([]idName, 0, min(len(ids), limit))
			for _, id := range ids[:min(len(ids), limit)] {
				found = append(found, idName{ID: id, Name: names[id]})
			}
			result["things"] = found
			return result, nil
		})

	addTool(server, "free_ids", "Ids of the open client that hold no object (past the end, or blank objects), for create_thing. The server's items.xml may still use some of them: check it too.",
		func(_ context.Context, args freeIDsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			count := args.Count
			if count <= 0 {
				count = 1
			}
			plan, err := s.things.PlanTargets(category, args.From, min(count, 100), true)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ids": plan.IDs}, nil
		})

	addTool(server, "render_things", "An image of client objects at a zoom, one row per object and one column per animation frame, on a grey background. Use it to look at existing sprites and compare new ones with them.",
		func(_ context.Context, args renderThingsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if len(args.IDs) == 0 || len(args.IDs) > 64 {
				return nil, errors.New("give 1 to 64 ids")
			}
			scale := args.Scale
			if scale <= 0 {
				scale = 4
			}
			scale = min(scale, 8)
			rows := make([][]*image.NRGBA, 0, len(args.IDs))
			var legend strings.Builder
			cellWidth, cellHeight, columns := 0, 0, 1
			for row, id := range args.IDs {
				object, err := s.things.Get(category, id)
				if err != nil {
					return nil, err
				}
				frames := 1
				if group := object.Group(thing.FrameGroupDefault); group != nil && !args.FirstOnly {
					frames = max(int(group.Frames), 1)
				}
				var images []*image.NRGBA
				for frame := range frames {
					img, err := s.things.Render(category, id, frame)
					if err != nil {
						return nil, err
					}
					cellWidth, cellHeight = max(cellWidth, img.Rect.Dx()), max(cellHeight, img.Rect.Dy())
					images = append(images, img)
				}
				columns = max(columns, len(images))
				rows = append(rows, images)
				fmt.Fprintf(&legend, "row %d: %s %d %q, %d frame(s)\n", row+1, category, id, object.Name, frames)
			}
			return imageResult(thingSheet(rows, cellWidth, cellHeight, columns, scale), legend.String())
		})

	addTool(server, "create_thing", "Creates a client object at an id from a PNG sprite sheet, with the properties and animation timing of a template object. Shows in the editor right away; save_client writes it to disk.",
		func(_ context.Context, args createThingArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if args.ID < category.MinID() {
				return nil, fmt.Errorf("%s ids start at %d", category, category.MinID())
			}
			plan, err := s.things.PlanTargets(category, args.ID, 1, false)
			if err != nil {
				return nil, err
			}
			if len(plan.Taken) > 0 && !args.Replace {
				return nil, fmt.Errorf("%s %d is already used: pick a free id (free_ids) or pass replace", category, args.ID)
			}
			object := thing.New(args.ID, category)
			if args.Template != 0 {
				if object, err = s.things.Get(category, args.Template); err != nil {
					return nil, err
				}
				object.ID, object.NpcSales, object.Extra = args.ID, nil, nil
			}
			object.Name, object.Description = args.Name, args.Description
			if object.Props.IsMarket {
				object.Props.Market.Name = args.Name
				object.Props.Market.TradeAs = uint16(args.ID)
				object.Props.Market.ShowAs = uint16(args.ID)
			}
			if _, err := s.things.Paste(category, args.ID, []*thing.Thing{object}); err != nil {
				return nil, err
			}
			if err := s.things.ImportSheet(category, args.ID, 0, args.Sheet); err != nil {
				return nil, err
			}
			if len(args.Props) > 0 {
				if _, err := s.things.Patch(category, []uint32{args.ID}, project.PropsPatch(args.Props)); err != nil {
					return nil, err
				}
			}
			if args.DurationMs > 0 {
				created, err := s.things.Get(category, args.ID)
				if err != nil {
					return nil, err
				}
				applyDuration(created, args.DurationMs)
				if err := s.things.Update(created); err != nil {
					return nil, err
				}
			}
			s.refreshItems()
			return s.thingResult(category, args.ID)
		})

	addTool(server, "update_thing", "Changes properties, name, description or frame durations of client objects, given by ids or by a where filter (e.g. every pickupable item).",
		func(_ context.Context, args updateThingArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if args.Where != nil {
				if len(args.IDs) > 0 {
					return nil, errors.New("give ids or where, not both")
				}
				if args.Where.Name == "" && args.Where.Flag == "" {
					return nil, errors.New("where needs a name or a flag")
				}
				if args.IDs, err = s.things.Find(category, project.Filter{Content: project.ContentUsed, Flag: args.Where.Flag, Name: args.Where.Name}); err != nil {
					return nil, err
				}
				if len(args.IDs) == 0 {
					return map[string]any{"changed": 0}, nil
				}
			}
			if len(args.IDs) == 0 {
				return nil, errors.New("give at least one id")
			}
			changed := 0
			if len(args.Props) > 0 {
				if changed, err = s.things.Patch(category, args.IDs, project.PropsPatch(args.Props)); err != nil {
					return nil, err
				}
			}
			if args.Name != nil || args.Description != nil || args.DurationMs > 0 {
				objects := make([]*thing.Thing, 0, len(args.IDs))
				for _, id := range args.IDs {
					object, err := s.things.Get(category, id)
					if err != nil {
						return nil, err
					}
					if args.Name != nil {
						object.Name = *args.Name
					}
					if args.Description != nil {
						object.Description = *args.Description
					}
					if args.DurationMs > 0 {
						applyDuration(object, args.DurationMs)
					}
					objects = append(objects, object)
				}
				if err := s.things.UpdateMany(objects); err != nil {
					return nil, err
				}
				changed = max(changed, len(objects))
			}
			s.refreshItems()
			return map[string]any{"changed": changed}, nil
		})

	addTool(server, "duplicate_things", "Copies client objects (properties and sprites) to new ids: a quick base for a variant, whose sprites import_sheet then replaces.",
		func(_ context.Context, args duplicateThingsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if len(args.IDs) == 0 {
				return nil, errors.New("give at least one id")
			}
			var copies []uint32
			if args.At == 0 {
				if copies, err = s.things.Duplicate(category, args.IDs); err != nil {
					return nil, err
				}
			} else {
				if args.At < category.MinID() {
					return nil, fmt.Errorf("%s ids start at %d", category, category.MinID())
				}
				plan, err := s.things.PlanTargets(category, args.At, len(args.IDs), false)
				if err != nil {
					return nil, err
				}
				if len(plan.Taken) > 0 && !args.Replace {
					return nil, fmt.Errorf("%s ids %v are already used: pick free ids (free_ids) or pass replace", category, plan.Taken)
				}
				objects := make([]*thing.Thing, 0, len(args.IDs))
				for _, id := range args.IDs {
					object, err := s.things.Get(category, id)
					if err != nil {
						return nil, err
					}
					objects = append(objects, object)
				}
				if copies, err = s.things.Paste(category, args.At, objects); err != nil {
					return nil, err
				}
			}
			s.refreshItems()
			return map[string]any{"ids": copies}, nil
		})

	addTool(server, "remove_things", "Removes client objects. The last object of a category is deleted, others become blank so the ids after them stay.",
		func(_ context.Context, args idsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if len(args.IDs) == 0 {
				return nil, errors.New("give at least one id")
			}
			if err := s.things.Remove(category, args.IDs); err != nil {
				return nil, err
			}
			s.refreshItems()
			return map[string]any{"removed": len(args.IDs)}, nil
		})

	addTool(server, "move_things", "Moves client objects to other ids without losing any: the objects at the targets swap places with them, or with insert shift to close the gap. Returns the new ids. Items placed on maps keep their server ids; the attached items.otb follows on save_client.",
		func(_ context.Context, args moveThingsArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if len(args.IDs) == 0 {
				return nil, errors.New("give at least one id")
			}
			updateRefs := args.UpdateRefs == nil || *args.UpdateRefs
			moved, err := s.things.Move(category, args.IDs, args.At, project.MoveOptions{Insert: args.Insert, UpdateRefs: updateRefs})
			if err != nil {
				return nil, err
			}
			s.refreshItems()
			return map[string]any{"ids": moved}, nil
		})

	addTool(server, "import_sheet", "Replaces the sprites of a client object with a PNG sprite sheet (layout as in create_thing).",
		func(_ context.Context, args importSheetArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if err := s.things.ImportSheet(category, args.ID, args.Group, args.Sheet); err != nil {
				return nil, err
			}
			s.refreshItems()
			return s.thingResult(category, args.ID)
		})

	addTool(server, "export_sheet", "Writes the sprites of a client object to a PNG sprite sheet, the same layout import_sheet reads: edit it and import it back, or use it as a base for create_thing.",
		func(_ context.Context, args exportSheetArgs) (any, error) {
			category, err := args.parse()
			if err != nil {
				return nil, err
			}
			if args.Path == "" {
				return nil, errors.New("give the path of the PNG to write")
			}
			if args.Background != "" && args.Background != "transparent" && args.Background != "magenta" {
				return nil, fmt.Errorf("background %q: use transparent or magenta", args.Background)
			}
			if err := s.things.ExportSheet(category, args.ID, args.Group, args.Path, args.Background != "magenta"); err != nil {
				return nil, err
			}
			object, err := s.things.Get(category, args.ID)
			if err != nil {
				return nil, err
			}
			spriteSize, err := s.things.SpriteSize()
			if err != nil {
				return nil, err
			}
			group, ok := object.Stacked()
			if args.Group >= 0 && args.Group < len(object.FrameGroups) {
				group, ok = object.FrameGroups[args.Group], true
			}
			result := map[string]any{"path": args.Path}
			if ok {
				result["layout"] = sheetLayout(group, spriteSize)
				result["frames"], result["patternX"], result["patternY"], result["patternZ"], result["layers"] = group.Frames, group.PatternX, group.PatternY, group.PatternZ, group.Layers
			}
			return result, nil
		})

	addTool(server, "save_client", "Writes the client objects and sprites to the client files (the folder or files it was opened from).",
		func(context.Context, struct{}) (any, error) {
			stats, err := s.projects.Compile()
			if err != nil {
				return nil, err
			}
			s.refreshItems()
			return stats, nil
		})

	addTool(server, "client_undo", "Undoes the last change of the client objects (not of the map: that is undo).",
		func(context.Context, struct{}) (any, error) {
			label, err := s.projects.Undo()
			if err != nil {
				return nil, err
			}
			s.refreshItems()
			return map[string]any{"undone": label}, nil
		})

	addTool(server, "client_redo", "Redoes the last undone change of the client objects.",
		func(context.Context, struct{}) (any, error) {
			label, err := s.projects.Redo()
			if err != nil {
				return nil, err
			}
			s.refreshItems()
			return map[string]any{"redone": label}, nil
		})
}

// thingSheet lays rows of textures out on a grey grid, scaled up.
func thingSheet(rows [][]*image.NRGBA, cellWidth, cellHeight, columns, scale int) *image.NRGBA {
	const gap = 4
	cellWidth, cellHeight = cellWidth*scale, cellHeight*scale
	sheet := image.NewNRGBA(image.Rect(0, 0, gap+columns*(cellWidth+gap), gap+len(rows)*(cellHeight+gap)))
	draw.Draw(sheet, sheet.Rect, image.NewUniform(color.NRGBA{72, 72, 72, 255}), image.Point{}, draw.Src)
	for row, images := range rows {
		for column, img := range images {
			// Bottom-right aligned in the cell, as the client anchors textures.
			left := gap + column*(cellWidth+gap) + cellWidth - img.Rect.Dx()*scale
			top := gap + row*(cellHeight+gap) + cellHeight - img.Rect.Dy()*scale
			scaled := scaleNearest(img, scale)
			draw.Draw(sheet, scaled.Rect.Add(image.Pt(left, top)), scaled, image.Point{}, draw.Over)
		}
	}
	return sheet
}

func scaleNearest(img *image.NRGBA, scale int) *image.NRGBA {
	if scale == 1 {
		return img
	}
	width, height := img.Rect.Dx(), img.Rect.Dy()
	scaled := image.NewNRGBA(image.Rect(0, 0, width*scale, height*scale))
	for y := range height * scale {
		for x := range width * scale {
			source := img.PixOffset(img.Rect.Min.X+x/scale, img.Rect.Min.Y+y/scale)
			target := scaled.PixOffset(x, y)
			copy(scaled.Pix[target:target+4], img.Pix[source:source+4])
		}
	}
	return scaled
}
