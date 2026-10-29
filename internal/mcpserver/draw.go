package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	// maxGridSide limits the rows and columns of paint_grid.
	maxGridSide = 256
	// maxBatch limits the operations of draw_batch.
	maxBatch = 500
	// defaultDirection is the direction creatures face (south).
	defaultDirection = 2
	// maxCheckedPoints is the most points a draw checks for skipped ones.
	maxCheckedPoints = 500
)

// drawOrder is the order paint_grid draws brush kinds in: what others
// connect to or stand on first.
var drawOrder = []string{
	mapedit.KindEraser, mapedit.KindGround, mapedit.KindOptionalBorder, mapedit.KindCarpet, mapedit.KindWall,
	mapedit.KindDoor, mapedit.KindTable, mapedit.KindDoodad, mapedit.KindRaw, mapedit.KindHouse, mapedit.KindHouseExit,
	mapedit.KindFlag, mapedit.KindZone, mapedit.KindSpawn, mapedit.KindSpawnNPC, mapedit.KindCreature, mapedit.KindWaypoint,
}

// tileFlags names the tile flags of flag brushes and set_tile.
var tileFlags = []struct {
	name string
	bit  uint32
}{
	{"pz", otmap.FlagPZ}, {"noPvp", otmap.FlagNoPVP}, {"noLogout", otmap.FlagNoLogout}, {"pvpZone", otmap.FlagPVPZone}, {"refresh", otmap.FlagRefresh},
}

func tileFlag(name string) (uint32, error) {
	for _, flag := range tileFlags {
		if strings.EqualFold(flag.name, name) {
			return flag.bit, nil
		}
	}
	return 0, fmt.Errorf("unknown tile flag %q (pz, noPvp, noLogout, pvpZone, refresh)", name)
}

func tileFlagNames(flags uint32) []string {
	var names []string
	for _, flag := range tileFlags {
		if flags&flag.bit != 0 {
			names = append(names, flag.name)
		}
	}
	return names
}

// Brush is what a draw places, without where.
type Brush struct {
	Kind      string  `json:"kind" jsonschema:"ground, wall, door, doodad, carpet, table, raw (one item as is), eraser, creature, spawn, spawnNpc, house, houseExit, waypoint, flag, zone or optionalBorder"`
	Name      string  `json:"name,omitempty" jsonschema:"brush name for ground, wall, doodad, carpet and table (see list_brushes)"`
	Item      int     `json:"item,omitempty" jsonschema:"item id for raw"`
	Creature  string  `json:"creature,omitempty" jsonschema:"monster or NPC name for creature; creatures are only placed on walkable tiles, and one outside every spawn gets its own spawn of radius 1 (draw kind spawn first)"`
	SpawnTime int     `json:"spawnTime,omitempty" jsonschema:"creature respawn time in seconds"`
	Direction *int    `json:"direction,omitempty" jsonschema:"creature facing: 0 north, 1 east, 2 south (default), 3 west"`
	House     int     `json:"house,omitempty" jsonschema:"house id for house and houseExit (create houses with save_house)"`
	Waypoint  string  `json:"waypoint,omitempty" jsonschema:"waypoint name for waypoint"`
	Flag      string  `json:"flag,omitempty" jsonschema:"tile flag for flag: pz, noPvp, noLogout, pvpZone or refresh"`
	Zone      int     `json:"zone,omitempty" jsonschema:"zone id for zone (create zones with save_zone)"`
	DoorType  string  `json:"doorType,omitempty" jsonschema:"door: archway, normal, locked, quest, magic, window or hatch window; drawn on an existing wall"`
	Erase     bool    `json:"erase,omitempty" jsonschema:"remove what the brush places instead"`
	Alt       bool    `json:"alt,omitempty" jsonschema:"ground: only replace the ground under the first point; wall: change the wall shape; door: place it open; doodad: ignore blocking tiles and duplicates"`
	Variation int     `json:"variation,omitempty" jsonschema:"doodad: which of the brush's alternatives"`
	Thickness float64 `json:"thickness,omitempty" jsonschema:"doodad density 0-1 for sizes above 0 (0: the brush's own)"`
}

// Rect is a rectangle of one floor.
type Rect struct {
	From Pos `json:"from" jsonschema:"one corner"`
	To   Pos `json:"to" jsonschema:"the opposite corner (its z is ignored)"`
}

// Blob is an irregular round area, like an island or a lake.
type Blob struct {
	Center    Pos     `json:"center"`
	RadiusX   int     `json:"radiusX" jsonschema:"half width in tiles"`
	RadiusY   int     `json:"radiusY,omitempty" jsonschema:"half height in tiles (default radiusX)"`
	Roughness float64 `json:"roughness,omitempty" jsonschema:"how ragged the edge is, 0 (an ellipse) to 1 (default 0.35)"`
	Seed      int     `json:"seed,omitempty" jsonschema:"another seed gives another shape"`
}

// DrawOp is one brush applied to points or an area.
type DrawOp struct {
	Brush
	Points    []Pos  `json:"points,omitempty" jsonschema:"brush centers"`
	Rect      *Rect  `json:"rect,omitempty" jsonschema:"fill a rectangle instead of points; walls draw its outline"`
	Polygon   []Pos  `json:"polygon,omitempty" jsonschema:"fill the polygon with these corners (in order, on one floor) instead of points; its edges are included"`
	Blob      *Blob  `json:"blob,omitempty" jsonschema:"fill an irregular round area instead of points"`
	Size      int    `json:"size,omitempty" jsonschema:"brush radius around each point: 0 one tile, 1 3x3, 2 5x5...; walls draw the outline of the square"`
	Shape     string `json:"shape,omitempty" jsonschema:"square (default) or circle"`
	Fill      bool   `json:"fill,omitempty" jsonschema:"ground: flood fill the area of the same ground (or empty tiles) under the first point"`
	OnlyEmpty bool   `json:"onlyEmpty,omitempty" jsonschema:"only paint tiles without a ground (e.g. sea around existing land)"`
}

// request turns a brush into a draw request without positions.
func (brush Brush) request() (mapedit.DrawRequest, error) {
	if !slices.Contains(drawOrder, brush.Kind) {
		return mapedit.DrawRequest{}, fmt.Errorf("unknown brush kind %q", brush.Kind)
	}
	if brush.Item < 0 || brush.Item > 0xFFFF || brush.House < 0 || brush.Zone < 0 || brush.Zone > 0xFFFF {
		return mapedit.DrawRequest{}, errors.New("item, house or zone id out of range")
	}
	request := mapedit.DrawRequest{
		Kind: brush.Kind, Name: brush.Name, Item: uint16(brush.Item), House: uint32(brush.House), Waypoint: brush.Waypoint,
		Creature: brush.Creature, SpawnTime: int32(brush.SpawnTime), Direction: defaultDirection, Zone: uint16(brush.Zone),
		DoorType: brush.DoorType, Erase: brush.Erase, Alt: brush.Alt, Variation: brush.Variation, Thickness: brush.Thickness,
	}
	if brush.Direction != nil {
		request.Direction = uint8(*brush.Direction & 3)
	}
	if brush.Kind == mapedit.KindFlag {
		flag, err := tileFlag(brush.Flag)
		if err != nil {
			return request, err
		}
		request.Flag = flag
	}
	switch {
	case brush.Kind == mapedit.KindRaw && brush.Item == 0:
		return request, errors.New("raw needs item")
	case brush.Kind == mapedit.KindCreature && brush.Creature == "":
		return request, errors.New("creature needs creature")
	case brush.Kind == mapedit.KindDoor && brush.DoorType == "":
		return request, errors.New("door needs doorType")
	case slices.Contains([]string{mapedit.KindGround, mapedit.KindWall, mapedit.KindDoodad, mapedit.KindCarpet, mapedit.KindTable}, brush.Kind) && brush.Name == "" && !brush.Erase:
		return request, fmt.Errorf("%s needs name", brush.Kind)
	}
	return request, nil
}

// request turns an operation into a draw request.
func (op DrawOp) request() (mapedit.DrawRequest, error) {
	request, err := op.Brush.request()
	if err != nil {
		return request, err
	}
	if request.Points, err = positions(op.Points); err != nil {
		return request, err
	}
	if len(op.Polygon) > 0 {
		if request.Points, err = polygonTiles(op.Polygon); err != nil {
			return request, err
		}
	}
	if op.Blob != nil {
		if request.Points, err = op.Blob.tiles(); err != nil {
			return request, err
		}
	}
	if op.Rect != nil {
		from, err := op.Rect.From.position()
		if err != nil {
			return request, err
		}
		to, err := op.Rect.To.position()
		if err != nil {
			return request, err
		}
		to.Z = from.Z
		request.Rect = &mapedit.Rect{From: from, To: to}
	}
	if len(request.Points) == 0 && request.Rect == nil {
		return request, errors.New("give points, rect, polygon or blob")
	}
	request.Size, request.Shape, request.Fill = max(op.Size, 0), op.Shape, op.Fill
	if len(op.Polygon) > 0 || op.Blob != nil {
		request.Size, request.Fill = 0, false
	}
	return request, nil
}

// onlyEmpty limits a request to the tiles without a ground.
func (s *Server) onlyEmpty(id int, request mapedit.DrawRequest) (mapedit.DrawRequest, error) {
	tiles, err := s.maps.DrawTiles(id, request, true)
	request.Points, request.Rect, request.Size, request.Fill = tiles, nil, 0, false
	return request, err
}

// skipped lists the points of a creature or raw draw that did not get what
// it places (blocking or missing ground, unknown item...).
func (s *Server) skipped(id int, request mapedit.DrawRequest) []Pos {
	if request.Erase || request.Rect != nil || request.Size > 0 || len(request.Points) > maxCheckedPoints ||
		(request.Kind != mapedit.KindCreature && request.Kind != mapedit.KindRaw) {
		return nil
	}
	var missed []Pos
	for _, point := range request.Points {
		tile, err := s.maps.Tile(id, point)
		if err != nil || !placed(tile, request) {
			missed = append(missed, Pos{X: int(point.X), Y: int(point.Y), Z: int(point.Z)})
		}
	}
	return missed
}

// placed reports whether a tile has what a creature or raw draw places.
func placed(tile app.TileInfo, request mapedit.DrawRequest) bool {
	if request.Kind == mapedit.KindCreature {
		for _, creature := range tile.Creatures {
			if strings.EqualFold(creature.Name, request.Creature) {
				return true
			}
		}
		return false
	}
	if tile.Ground != nil && tile.Ground.ID == request.Item {
		return true
	}
	for _, item := range tile.Items {
		if item.ID == request.Item {
			return true
		}
	}
	return false
}

type drawArgs struct {
	MapArg
	DrawOp
}

type batchArgs struct {
	MapArg
	Operations []DrawOp `json:"operations" jsonschema:"draws applied in order"`
}

// LegendEntry is what one symbol of paint_grid places.
type LegendEntry struct {
	Symbol  string  `json:"symbol" jsonschema:"one character of the rows"`
	Brushes []Brush `json:"brushes" jsonschema:"what to place on its tiles, e.g. a ground and a wall"`
}

type gridArgs struct {
	MapArg
	Origin Pos           `json:"origin" jsonschema:"position of the first character of the first row (the north-west corner)"`
	Rows   []string      `json:"rows" jsonschema:"one string per row, north to south; character n of a row is the tile n east of the origin. Spaces place nothing"`
	Legend []LegendEntry `json:"legend" jsonschema:"what each character places"`
}

func (s *Server) addDrawTools(server *mcp.Server) {
	addTool(server, "draw", "Applies one brush at points or over a rectangle, as the editor's brushes do: grounds get borders, walls connect, doodads scatter. Erase removes what the brush places.",
		func(_ context.Context, args drawArgs) (any, error) {
			request, err := args.DrawOp.request()
			if err != nil {
				return nil, err
			}
			return s.drawAll(args.MapID, []mapedit.DrawRequest{request}, []bool{args.OnlyEmpty})
		})

	addTool(server, "draw_batch", "Applies several brushes in order as one undo step: build a whole area in one call.",
		func(_ context.Context, args batchArgs) (any, error) {
			if len(args.Operations) > maxBatch {
				return nil, fmt.Errorf("at most %d operations", maxBatch)
			}
			requests := make([]mapedit.DrawRequest, 0, len(args.Operations))
			onlyEmpty := make([]bool, 0, len(args.Operations))
			for index, op := range args.Operations {
				request, err := op.request()
				if err != nil {
					return nil, fmt.Errorf("operation %d: %w", index, err)
				}
				requests, onlyEmpty = append(requests, request), append(onlyEmpty, op.OnlyEmpty)
			}
			return s.drawAll(args.MapID, requests, onlyEmpty)
		})

	addTool(server, "paint_grid", `Paints an area from a text plan, one character per tile, as one undo step. Example: rows ["#####", "#...#", "#...#", "##D##"] with legend # = ground "grass" + wall "stone wall", . = ground "wooden floor", D = ground "grass" + wall "stone wall" + door "normal". Brushes are applied by kind: grounds, then carpets, walls, doors, tables, doodads, items, creatures.`,
		func(_ context.Context, args gridArgs) (any, error) {
			requests, err := gridRequests(args)
			if err != nil {
				return nil, err
			}
			return s.drawAll(args.MapID, requests, nil)
		})
}

// drawAll applies draw requests as one undo step and shows them in the
// editor once. onlyEmpty (nil: none) limits requests to tiles without a
// ground.
func (s *Server) drawAll(mapID int, requests []mapedit.DrawRequest, onlyEmpty []bool) (any, error) {
	id, err := s.mapID(mapID)
	if err != nil {
		return nil, err
	}
	stroke := s.nextStroke()
	changes := make([]app.MapChange, 0, len(requests))
	var skipped []Pos
	var failed error
	for index, request := range requests {
		request.Stroke = stroke
		if index < len(onlyEmpty) && onlyEmpty[index] {
			if request, err = s.onlyEmpty(id, request); err != nil {
				return nil, err
			}
		}
		change, err := s.maps.Draw(id, request)
		if err == nil {
			skipped = append(skipped, s.skipped(id, request)...)
		}
		if change.Info.ID != 0 {
			changes = append(changes, change)
		}
		if err != nil {
			failed = fmt.Errorf("draw %d (%s %s): %w", index, request.Kind, request.Name, err)
			break
		}
	}
	change := mergeChanges(changes)
	s.changed(id, change)
	if failed != nil {
		return nil, fmt.Errorf("%w; the draws before it were applied (undo reverts them all)", failed)
	}
	result := editResult(id, change)
	result.Skipped = skipped
	return result, nil
}

// gridRequests turns a text plan into draw requests: one per brush of
// each symbol, ordered by drawOrder.
func gridRequests(args gridArgs) ([]mapedit.DrawRequest, error) {
	origin, err := args.Origin.position()
	if err != nil {
		return nil, err
	}
	if len(args.Rows) > maxGridSide {
		return nil, fmt.Errorf("at most %d rows", maxGridSide)
	}
	type group struct {
		request mapedit.DrawRequest
		order   int
	}
	var groups []*group
	bySymbol := map[rune][]*group{}
	for entryIndex, entry := range args.Legend {
		symbol := []rune(entry.Symbol)
		if len(symbol) != 1 || symbol[0] == ' ' {
			return nil, fmt.Errorf("legend symbol %q must be one character other than space", entry.Symbol)
		}
		if bySymbol[symbol[0]] != nil {
			return nil, fmt.Errorf("symbol %q is in the legend twice", entry.Symbol)
		}
		if len(entry.Brushes) == 0 {
			return nil, fmt.Errorf("symbol %q has no brushes", entry.Symbol)
		}
		for brushIndex, brush := range entry.Brushes {
			request, err := brush.request()
			if err != nil {
				return nil, fmt.Errorf("symbol %q: %w", entry.Symbol, err)
			}
			order := slices.Index(drawOrder, brush.Kind)*maxGridSide*maxGridSide + entryIndex*maxGridSide + brushIndex
			added := &group{request: request, order: order}
			groups = append(groups, added)
			bySymbol[symbol[0]] = append(bySymbol[symbol[0]], added)
		}
	}
	unknown := map[rune]bool{}
	for row, line := range args.Rows {
		for column, symbol := range []rune(line) {
			if symbol == ' ' {
				continue
			}
			if column >= maxGridSide {
				return nil, fmt.Errorf("rows are at most %d characters", maxGridSide)
			}
			symbolGroups, ok := bySymbol[symbol]
			if !ok {
				unknown[symbol] = true
				continue
			}
			pos, ok := origin.Add(column, row, 0)
			if !ok {
				return nil, fmt.Errorf("row %d column %d is out of the map's range", row, column)
			}
			for _, symbolGroup := range symbolGroups {
				symbolGroup.request.Points = append(symbolGroup.request.Points, pos)
			}
		}
	}
	if len(unknown) > 0 {
		symbols := make([]string, 0, len(unknown))
		for symbol := range unknown {
			symbols = append(symbols, string(symbol))
		}
		slices.Sort(symbols)
		return nil, fmt.Errorf("symbols not in the legend: %s", strings.Join(symbols, " "))
	}
	slices.SortFunc(groups, func(first, second *group) int { return cmp.Compare(first.order, second.order) })
	requests := make([]mapedit.DrawRequest, 0, len(groups))
	for _, group := range groups {
		if len(group.request.Points) > 0 {
			requests = append(requests, group.request)
		}
	}
	return requests, nil
}
