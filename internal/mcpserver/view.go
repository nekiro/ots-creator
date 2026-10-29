package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Sizes of rendered images.
const (
	// defaultImageSide and maxImageSide bound the longer side of a map
	// image in pixels.
	defaultImageSide = 1024
	maxImageSide     = 2048
	// defaultAreaSide is the width and height of render_area without them.
	defaultAreaSide = 24
	// minGridTile is the smallest tile in pixels that still gets grid lines.
	minGridTile = 8
	// labelGap is the least space in pixels between two coordinate labels.
	labelGap = 6
	// describeLimit is the default and largest number of tiles describe_area
	// lists.
	describeLimit = 400
	maxDescribe   = 2000
	// maxAreaRead is how many tiles describe_area reads to filter them.
	maxAreaRead = 128 * 128
)

// Colors of the grid drawn over map images.
var (
	gridColor   = color.NRGBA{255, 255, 255, 40}
	labelColor  = color.NRGBA{255, 255, 255, 255}
	marginColor = color.NRGBA{24, 24, 24, 255}
)

// areaArgs select a rectangle of one floor.
type areaArgs struct {
	MapArg
	X      int `json:"x" jsonschema:"west edge"`
	Y      int `json:"y" jsonschema:"north edge"`
	Z      int `json:"z" jsonschema:"floor"`
	Width  int `json:"width,omitempty" jsonschema:"tiles east (default 24)"`
	Height int `json:"height,omitempty" jsonschema:"tiles south (default 24)"`
}

func (args areaArgs) area() (image.Rectangle, error) {
	width, height := args.Width, args.Height
	if width <= 0 {
		width = defaultAreaSide
	}
	if height <= 0 {
		height = defaultAreaSide
	}
	if args.X < 0 || args.Y < 0 || args.X+width > 0x10000 || args.Y+height > 0x10000 || args.Z < 0 || args.Z > otmap.MaxFloor {
		return image.Rectangle{}, errors.New("the area is out of the map's range")
	}
	return image.Rect(args.X, args.Y, args.X+width, args.Y+height), nil
}

type renderArgs struct {
	areaArgs
	Creatures *bool `json:"creatures,omitempty" jsonschema:"draw creatures (default true)"`
	Grid      *bool `json:"grid,omitempty" jsonschema:"draw tile grid lines and coordinates (default true)"`
	MaxPixels int   `json:"maxPixels,omitempty" jsonschema:"longest image side in pixels (default 1024, at most 2048); the tile size shrinks to fit"`
}

type minimapArgs struct {
	areaArgs
	MaxPixels int `json:"maxPixels,omitempty" jsonschema:"longest image side in pixels (default 1024)"`
}

type describeArgs struct {
	areaArgs
	Limit   int    `json:"limit,omitempty" jsonschema:"most tiles listed (default 400)"`
	Only    string `json:"only,omitempty" jsonschema:"only tiles with: items (anything besides ground and borders), creatures (creatures or spawns) or special (item attributes, houses, flags, zones, waypoints)"`
	Borders bool   `json:"borders,omitempty" jsonschema:"also list border items (left out by default)"`
}

// describeFilter selects what describe_area lists.
type describeFilter struct {
	only    string
	borders bool
}

// Values of describeArgs.Only.
const (
	onlyItems     = "items"
	onlyCreatures = "creatures"
	onlySpecial   = "special"
)

// keeps reports whether a tile is listed.
func (filter describeFilter) keeps(tile app.TileInfo) bool {
	switch filter.only {
	case onlyItems:
		return slices.ContainsFunc(tile.Items, func(item app.TileItem) bool { return !isBorder(item) })
	case onlyCreatures:
		return len(tile.Creatures) > 0 || tile.Spawn != nil || tile.SpawnNPC != nil
	case onlySpecial:
		return tile.HouseID != 0 || tile.Flags != 0 || len(tile.Zones) > 0 || len(tile.Waypoints) > 0 || tile.HouseExit != "" ||
			tile.Temple != "" || slices.ContainsFunc(tile.Items, func(item app.TileItem) bool { return item.Ext != nil })
	}
	return true
}

func isBorder(item app.TileItem) bool { return item.TopOrder == 1 }

type tileArgs struct {
	MapArg
	Pos Pos `json:"pos"`
}

type findArgs struct {
	MapArg
	IDs         []int  `json:"ids,omitempty" jsonschema:"item ids to find"`
	Name        string `json:"name,omitempty" jsonschema:"part of item names"`
	Kind        string `json:"kind,omitempty" jsonschema:"unique, action, container, writeable, teleport, door or duplicate (the same item twice on a tile)"`
	ActionID    int    `json:"actionId,omitempty" jsonschema:"only items with this action id"`
	UniqueID    int    `json:"uniqueId,omitempty" jsonschema:"only items with this unique id"`
	InSelection bool   `json:"inSelection,omitempty"`
	Containers  bool   `json:"containers,omitempty" jsonschema:"also search container contents"`
	Limit       int    `json:"limit,omitempty" jsonschema:"most results (default 100)"`
}

func (s *Server) addViewTools(server *mcp.Server) {
	addTool(server, "render_area", "An image of an area of one floor drawn with the client's sprites, as in the editor. With the grid, labels on the top and left edges give x and y. Use it to check what you drew.",
		func(_ context.Context, args renderArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			area, err := args.area()
			if err != nil {
				return nil, err
			}
			img, err := s.maps.RenderArea(id, area, args.Z, args.Creatures == nil || *args.Creatures)
			if err != nil {
				return nil, err
			}
			tileSize := fitTile(area, img.Bounds().Dx()/area.Dx(), imageSide(args.MaxPixels))
			out := scaled(img, area, tileSize)
			if args.Grid == nil || *args.Grid {
				out = withGrid(out, area, tileSize)
			}
			text := fmt.Sprintf("Floor %d, x %d-%d, y %d-%d, %d px per tile.", args.Z, area.Min.X, area.Max.X-1, area.Min.Y, area.Max.Y-1, tileSize)
			return imageResult(out, text)
		})

	addTool(server, "render_minimap", "A minimap image of an area (each tile one color, scaled up): an overview of big areas.",
		func(_ context.Context, args minimapArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			area, err := args.area()
			if err != nil {
				return nil, err
			}
			img, err := s.maps.MinimapArea(id, area, args.Z)
			if err != nil {
				return nil, err
			}
			tileSize := max(1, imageSide(args.MaxPixels)/max(area.Dx(), area.Dy()))
			out := image.NewNRGBA(image.Rect(0, 0, area.Dx()*tileSize, area.Dy()*tileSize))
			xdraw.NearestNeighbor.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Src, nil)
			text := fmt.Sprintf("Floor %d, x %d-%d, y %d-%d, %d px per tile.", args.Z, area.Min.X, area.Max.X-1, area.Min.Y, area.Max.Y-1, tileSize)
			return imageResult(out, text)
		})

	addTool(server, "describe_area", "Lists the tiles of an area as text: ground and items (ids, names in the legend) bottom to top, item attributes, houses, flags, creatures and spawns. Empty tiles are left out.",
		func(_ context.Context, args describeArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			area, err := args.area()
			if err != nil {
				return nil, err
			}
			if !slices.Contains([]string{"", onlyItems, onlyCreatures, onlySpecial}, args.Only) {
				return nil, fmt.Errorf("unknown only %q", args.Only)
			}
			limit := describeLimit
			if args.Limit > 0 {
				limit = min(args.Limit, maxDescribe)
			}
			filter := describeFilter{only: args.Only, borders: args.Borders}
			if filter.only != "" {
				// Filtered out tiles do not count: read the whole area.
				limit = maxAreaRead
			}
			from := otmap.Position{X: uint16(area.Min.X), Y: uint16(area.Min.Y), Z: uint8(args.Z)}
			to := otmap.Position{X: uint16(area.Max.X - 1), Y: uint16(area.Max.Y - 1), Z: uint8(args.Z)}
			tiles, more, err := s.maps.AreaTiles(id, from, to, limit)
			if err != nil {
				return nil, err
			}
			listed := min(describeLimit, maxDescribe)
			if args.Limit > 0 {
				listed = min(args.Limit, maxDescribe)
			}
			return textResult(describeTiles(tiles, area, args.Z, more, filter, listed)), nil
		})

	addTool(server, "get_tile", "Everything on one tile: ground, items with their index (for update_item and remove_item) and attributes, flags, house, zones, creatures and spawns.",
		func(_ context.Context, args tileArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			pos, err := args.Pos.position()
			if err != nil {
				return nil, err
			}
			return s.maps.Tile(id, pos)
		})

	addTool(server, "find_items", "Finds items on a map by id, name or kind (action and unique ids, containers, doors, teleports...).",
		func(_ context.Context, args findArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			ids, err := itemIDs(args.IDs)
			if err != nil {
				return nil, err
			}
			query := mapedit.FindQuery{IDs: ids, Name: args.Name, Kind: args.Kind, ActionID: uint16(args.ActionID), UniqueID: uint16(args.UniqueID), InSelection: args.InSelection, Containers: args.Containers, Limit: listLimit(args.Limit)}
			return s.maps.Find(id, query)
		})

	addTool(server, "get_statistics", "Counts of a map: tiles, items, walkable and blocking tiles, houses, towns, spawns, monsters, NPCs and more.",
		func(_ context.Context, args MapArg) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			return s.maps.Stats(id)
		})
}

// itemIDs checks item id arguments.
func itemIDs(list []int) ([]uint16, error) {
	out := make([]uint16, 0, len(list))
	for _, id := range list {
		if id <= 0 || id > 0xFFFF {
			return nil, fmt.Errorf("bad item id %d", id)
		}
		out = append(out, uint16(id))
	}
	return out, nil
}

func imageSide(maxPixels int) int {
	if maxPixels <= 0 {
		return defaultImageSide
	}
	return min(maxPixels, maxImageSide)
}

// fitTile returns the tile size in pixels that fits area into side pixels,
// at most the sprite size.
func fitTile(area image.Rectangle, spriteSize, side int) int {
	return max(1, min(spriteSize, side/max(area.Dx(), area.Dy())))
}

// scaled resizes a map image to tileSize pixels per tile.
func scaled(img *image.NRGBA, area image.Rectangle, tileSize int) *image.NRGBA {
	bounds := image.Rect(0, 0, area.Dx()*tileSize, area.Dy()*tileSize)
	if bounds == img.Bounds() {
		return img
	}
	out := image.NewNRGBA(bounds)
	xdraw.BiLinear.Scale(out, bounds, img, img.Bounds(), xdraw.Src, nil)
	return out
}

// withGrid draws tile lines over a map image and puts it in a margin with
// x labels on top and y labels on the left.
func withGrid(img *image.NRGBA, area image.Rectangle, tileSize int) *image.NRGBA {
	face := basicfont.Face7x13
	digits := len(strconv.Itoa(max(area.Max.X, area.Max.Y)))
	charWidth := face.Advance
	marginLeft, marginTop := digits*charWidth+4, face.Height+2
	out := image.NewNRGBA(image.Rect(0, 0, marginLeft+img.Bounds().Dx(), marginTop+img.Bounds().Dy()))
	xdraw.Draw(out, out.Bounds(), image.NewUniform(marginColor), image.Point{}, xdraw.Src)
	xdraw.Draw(out, img.Bounds().Add(image.Pt(marginLeft, marginTop)), img, image.Point{}, xdraw.Src)
	if tileSize >= minGridTile {
		line := image.NewUniform(gridColor)
		for column := 1; column < area.Dx(); column++ {
			x := marginLeft + column*tileSize
			xdraw.Draw(out, image.Rect(x, marginTop, x+1, out.Bounds().Dy()), line, image.Point{}, xdraw.Over)
		}
		for row := 1; row < area.Dy(); row++ {
			y := marginTop + row*tileSize
			xdraw.Draw(out, image.Rect(marginLeft, y, out.Bounds().Dx(), y+1), line, image.Point{}, xdraw.Over)
		}
	}
	drawer := &font.Drawer{Dst: out, Src: image.NewUniform(labelColor), Face: face}
	// Labels every step tiles, on multiples of step so they are easy to
	// read.
	stepX := labelStep(digits*charWidth+labelGap, tileSize)
	for x := area.Min.X; x < area.Max.X; x++ {
		if x%stepX == 0 {
			drawer.Dot = fixed.P(marginLeft+(x-area.Min.X)*tileSize+1, face.Ascent+1)
			drawer.DrawString(strconv.Itoa(x))
		}
	}
	stepY := labelStep(face.Height, tileSize)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		if y%stepY == 0 {
			drawer.Dot = fixed.P(2, marginTop+(y-area.Min.Y)*tileSize+face.Ascent)
			drawer.DrawString(strconv.Itoa(y))
		}
	}
	return out
}

// labelStep returns how many tiles apart labels of size pixels go: 1, 2, 5,
// 10, 20, 50...
func labelStep(size, tileSize int) int {
	for step := 1; ; step *= 10 {
		for _, factor := range [...]int{1, 2, 5} {
			if step*factor*tileSize >= size {
				return step * factor
			}
		}
	}
}

func imageResult(img *image.NRGBA, text string) (*mcp.CallToolResult, error) {
	data, err := imaging.EncodePNG(img)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: data, MIMEType: "image/png"}, &mcp.TextContent{Text: text}}}, nil
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// describeTiles writes the tiles a filter keeps (at most limit) as text
// lines with a legend of item names.
func describeTiles(tiles []app.TileInfo, area image.Rectangle, floor int, more bool, filter describeFilter, limit int) string {
	kept := slices.DeleteFunc(slices.Clone(tiles), func(tile app.TileInfo) bool { return !filter.keeps(tile) })
	if len(kept) > limit {
		kept, more = kept[:limit], true
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Floor %d, x %d-%d, y %d-%d: %d tiles", floor, area.Min.X, area.Max.X-1, area.Min.Y, area.Max.Y-1, len(kept))
	if more {
		text.WriteString(" listed, more not listed (smaller area or higher limit)")
	}
	text.WriteString(".\nLine: x,y: ground | items bottom to top [index] | tile")
	if !filter.borders {
		text.WriteString(" (borders left out)")
	}
	text.WriteByte('\n')
	names := map[uint16]string{}
	var order []uint16
	note := func(item app.TileItem) {
		if _, ok := names[item.ID]; !ok {
			names[item.ID] = item.Name
			order = append(order, item.ID)
		}
	}
	for _, tile := range kept {
		fmt.Fprintf(&text, "%d,%d: ", tile.Pos.X, tile.Pos.Y)
		if tile.Ground != nil {
			note(*tile.Ground)
			text.WriteString(itemText(*tile.Ground))
		} else {
			text.WriteString("-")
		}
		text.WriteString(" |")
		for index, item := range tile.Items {
			if !filter.borders && isBorder(item) {
				continue
			}
			note(item)
			fmt.Fprintf(&text, " [%d]%s", index, itemText(item))
		}
		if extras := tileExtras(tile); extras != "" {
			text.WriteString(" | " + extras)
		}
		text.WriteByte('\n')
	}
	if len(order) > 0 {
		text.WriteString("Legend:")
		for _, id := range order {
			fmt.Fprintf(&text, " %d=%s;", id, names[id])
		}
		text.WriteByte('\n')
	}
	return text.String()
}

// itemText is an item id with its attributes.
func itemText(item app.TileItem) string {
	text := strconv.Itoa(int(item.ID))
	if item.Subtype > 1 {
		text += "x" + strconv.Itoa(int(item.Subtype))
	}
	if ext := item.Ext; ext != nil {
		if ext.ActionID != 0 {
			text += " aid=" + strconv.Itoa(int(ext.ActionID))
		}
		if ext.UniqueID != 0 {
			text += " uid=" + strconv.Itoa(int(ext.UniqueID))
		}
		if ext.Dest != nil {
			text += fmt.Sprintf(" dest=%d,%d,%d", ext.Dest.X, ext.Dest.Y, ext.Dest.Z)
		}
		if ext.DoorID != 0 {
			text += " door=" + strconv.Itoa(int(ext.DoorID))
		}
		if ext.DepotID != 0 {
			text += " depot=" + strconv.Itoa(int(ext.DepotID))
		}
		if ext.Text != "" {
			text += " text=" + strconv.Quote(ext.Text)
		}
	}
	if len(item.Contents) > 0 {
		parts := make([]string, 0, len(item.Contents))
		for _, content := range item.Contents {
			parts = append(parts, itemText(content))
		}
		text += " {" + strings.Join(parts, ", ") + "}"
	}
	return text
}

// tileExtras lists what a tile has besides items.
func tileExtras(tile app.TileInfo) string {
	var parts []string
	if tile.HouseID != 0 {
		parts = append(parts, "house "+strconv.Itoa(int(tile.HouseID)))
	}
	parts = append(parts, tileFlagNames(tile.Flags)...)
	for _, zone := range tile.Zones {
		parts = append(parts, "zone "+strconv.Itoa(int(zone)))
	}
	for _, creature := range tile.Creatures {
		kind := "monster"
		if creature.NPC {
			kind = "npc"
		}
		parts = append(parts, kind+" "+creature.Name)
	}
	if tile.Spawn != nil {
		parts = append(parts, "spawn r"+strconv.Itoa(int(tile.Spawn.Radius)))
	}
	if tile.SpawnNPC != nil {
		parts = append(parts, "npc spawn r"+strconv.Itoa(int(tile.SpawnNPC.Radius)))
	}
	if tile.HouseExit != "" {
		parts = append(parts, "exit of "+tile.HouseExit)
	}
	if tile.Temple != "" {
		parts = append(parts, "temple of "+tile.Temple)
	}
	for _, waypoint := range tile.Waypoints {
		parts = append(parts, "waypoint "+waypoint)
	}
	return strings.Join(parts, ", ")
}
