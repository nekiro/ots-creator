package mapedit

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
)

// Rect is a rectangle of tiles on one floor (corners in any order).
type Rect struct {
	From otmap.Position `json:"from"`
	To   otmap.Position `json:"to"`
}

func (r Rect) bounds() (x0, y0, x1, y1 int) {
	x0, x1 = int(min(r.From.X, r.To.X)), int(max(r.From.X, r.To.X))
	y0, y1 = int(min(r.From.Y, r.To.Y)), int(max(r.From.Y, r.To.Y))
	return
}

// DrawRequest applies a brush.
type DrawRequest struct {
	Kind string `json:"kind"`
	// Name is the brush name (ground, wall, doodad, carpet, table).
	Name string `json:"name"`
	// Item is the item of a raw brush.
	Item uint16 `json:"item"`
	// House is the house of house and house exit brushes.
	House uint32 `json:"house"`
	// Waypoint is the waypoint name.
	Waypoint string `json:"waypoint"`
	// Creature and NPC select the creature type.
	Creature  string `json:"creature"`
	NPC       bool   `json:"npc"`
	SpawnTime int32  `json:"spawnTime"`
	Direction uint8  `json:"direction"`
	// Flag is the tile flag of flag brushes, Zone the zone id.
	Flag uint32 `json:"flag"`
	Zone uint16 `json:"zone"`
	// DoorType is the door type of door brushes.
	DoorType string `json:"doorType"`

	// Points are the brush centers (a stroke may send several).
	Points []otmap.Position `json:"points"`
	Size   int              `json:"size"`
	// Shape is "square" or "circle".
	Shape string `json:"shape"`
	// Erase removes what the brush places (Ctrl).
	Erase bool `json:"erase"`
	// Alt: ground brushes only replace the ground they started on, walls
	// change shape on click, doors are placed open, doodads ignore
	// blocking tiles and duplicates.
	Alt bool `json:"alt"`
	// Fill flood fills the ground area under Points[0].
	Fill bool `json:"fill"`
	// Rect paints a rectangle (Shift+drag).
	Rect      *Rect   `json:"rect"`
	Variation int     `json:"variation"`
	Thickness float64 `json:"thickness"`
	// Stroke merges requests of one mouse stroke into one undo step.
	Stroke int64 `json:"stroke"`
}

type posSet struct {
	list []otmap.Position
	seen map[otmap.Position]bool
}

func newPosSet() *posSet { return &posSet{seen: map[otmap.Position]bool{}} }

func (s *posSet) add(pos otmap.Position) {
	if !s.seen[pos] {
		s.seen[pos] = true
		s.list = append(s.list, pos)
	}
}

func (s *posSet) addXY(x, y, z int) {
	if inRange(x, y) {
		s.add(posAt(x, y, z))
	}
}

// ring returns the tiles around a set (including the set).
func (s *posSet) ring() *posSet {
	around := newPosSet()
	for _, center := range s.list {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				around.addXY(int(center.X)+dx, int(center.Y)+dy, int(center.Z))
			}
		}
	}
	return around
}

// BrushArea returns the tiles a brush of a size and shape covers around a
// center.
func BrushArea(center otmap.Position, size int, shape string) []otmap.Position {
	tiles := newPosSet()
	addArea(tiles, center, size, shape)
	return tiles.list
}

func addArea(tiles *posSet, center otmap.Position, size int, shape string) {
	size = max(size, 0)
	for dy := -size; dy <= size; dy++ {
		for dx := -size; dx <= size; dx++ {
			if shape == shapeCircle && math.Sqrt(float64(dx*dx+dy*dy)) >= float64(size)+circleEdgeSlack {
				continue
			}
			tiles.addXY(int(center.X)+dx, int(center.Y)+dy, int(center.Z))
		}
	}
}

func addOutline(tiles *posSet, x0, y0, x1, y1, z int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x == x0 || x == x1 || y == y0 || y == y1 {
				tiles.addXY(x, y, z)
			}
		}
	}
}

// addRect adds every tile of a rectangle.
func addRect(tiles *posSet, x0, y0, x1, y1, z int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			tiles.addXY(x, y, z)
		}
	}
}

// Draw applies a brush and returns the number of changed tiles.
func (e *Editor) Draw(req DrawRequest) (int, error) {
	if len(req.Points) == 0 && req.Rect == nil {
		return 0, nil
	}
	if req.Kind == "" {
		return 0, fmt.Errorf("%w: no brush kind", ErrNoBrush)
	}
	label := strings.ToUpper(req.Kind[:1]) + req.Kind[1:]
	if req.Erase {
		label = "Erase " + req.Kind
	}
	tx := e.Begin(label, req.Stroke)
	var err error
	switch req.Kind {
	case KindHouseExit, KindWaypoint:
		err = e.drawMarker(tx, req)
	case KindSpawn, KindSpawnNPC:
		e.drawSpawn(tx, req)
	case KindCreature:
		err = e.drawCreature(tx, req)
	case KindDoor:
		e.drawDoorRequest(tx, req)
	case KindDoodad:
		err = e.drawDoodadRequest(tx, req)
	default:
		err = e.drawArea(tx, req)
	}
	changed := tx.Commit()
	return changed, err
}

// area returns the tiles a request paints.
func (e *Editor) area(req DrawRequest) *posSet {
	tiles := newPosSet()
	if req.Rect != nil {
		x0, y0, x1, y1 := req.Rect.bounds()
		if (x1-x0+1)*(y1-y0+1) > maxDrawTiles {
			return tiles
		}
		z := int(req.Rect.From.Z)
		if req.Kind == KindWall {
			addOutline(tiles, x0, y0, x1, y1, z)
			return tiles
		}
		addRect(tiles, x0, y0, x1, y1, z)
		return tiles
	}
	if req.Fill && req.Kind == KindGround && len(req.Points) > 0 {
		return e.floodFill(req.Points[0], req.Name)
	}
	for _, center := range req.Points {
		if req.Kind == KindWall && req.Size > 0 {
			addOutline(tiles, int(center.X)-req.Size, int(center.Y)-req.Size, int(center.X)+req.Size, int(center.Y)+req.Size, int(center.Z))
			continue
		}
		addArea(tiles, center, req.Size, req.Shape)
		if len(tiles.list) > maxDrawTiles {
			break
		}
	}
	return tiles
}

// floodFill returns the connected area of the ground brush under start
// (or of empty tiles), limited to a window around it.
func (e *Editor) floodFill(start otmap.Position, brush string) *posSet {
	filled := newPosSet()
	startTile := e.Map.Tile(start)
	oldBrush := e.groundBrushOf(startTile)
	targetBrush := e.Brushes.Ground(brush)
	if targetBrush == nil || (oldBrush != nil && oldBrush.id == targetBrush.id) {
		return filled
	}
	if startTile != nil && startTile.Ground.ID != 0 && oldBrush == nil {
		return filled // a ground without brush: nothing to match
	}
	matches := func(tile *otmap.Tile) bool {
		if oldBrush == nil {
			return tile == nil || tile.Ground.ID == 0
		}
		ground := e.groundBrushOf(tile)
		return ground != nil && ground.id == oldBrush.id
	}
	queue := []otmap.Position{start}
	filled.add(start)
	for len(queue) > 0 && len(filled.list) < maxDrawTiles {
		current := queue[0]
		queue = queue[1:]
		for _, step := range orthogonalSteps {
			nextX, nextY := int(current.X)+step[0], int(current.Y)+step[1]
			if !inRange(nextX, nextY) || abs(nextX-int(start.X)) > floodFillRadius || abs(nextY-int(start.Y)) > floodFillRadius {
				continue
			}
			next := posAt(nextX, nextY, int(current.Z))
			if filled.seen[next] || !matches(e.Map.Tile(next)) {
				continue
			}
			filled.add(next)
			queue = append(queue, next)
		}
	}
	return filled
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// strokeReplace remembers which ground an Alt stroke replaces.
type strokeReplace struct {
	stroke int64
	brush  *GroundBrush
	empty  bool
}

func (e *Editor) drawArea(tx *Tx, req DrawRequest) error {
	area := e.area(req)
	if len(area.list) == 0 {
		return nil
	}
	switch req.Kind {
	case KindRaw:
		return e.drawRaw(tx, req, area)
	case KindGround:
		return e.drawGround(tx, req, area)
	case KindEraser:
		e.drawEraser(tx, req, area)
	case KindWall:
		return e.drawWalls(tx, req, area)
	case KindCarpet, KindTable:
		return e.drawCarpetTable(tx, req, area)
	case KindHouse:
		return e.drawHouse(tx, req, area)
	case KindFlag:
		e.drawFlag(tx, req, area)
	case KindZone:
		e.drawZone(tx, req, area)
	case KindOptionalBorder:
		e.drawOptionalBorder(tx, req, area)
	default:
		return fmt.Errorf("%w: %s", ErrNoBrush, req.Kind)
	}
	return nil
}

// drawFlag sets or clears a tile flag on the grounds of an area.
func (e *Editor) drawFlag(tx *Tx, req DrawRequest, area *posSet) {
	for _, pos := range area.list {
		current := tx.Get(pos)
		if current == nil || current.Ground.ID == 0 {
			continue
		}
		hasFlag := current.Flags&req.Flag == req.Flag
		if hasFlag == !req.Erase {
			continue
		}
		tile := tx.Edit(pos)
		if req.Erase {
			tile.Flags &^= req.Flag
		} else {
			tile.Flags |= req.Flag
		}
	}
}

// drawZone adds or removes a zone on the grounds of an area.
func (e *Editor) drawZone(tx *Tx, req DrawRequest, area *posSet) {
	for _, pos := range area.list {
		current := tx.Get(pos)
		if current == nil || current.Ground.ID == 0 {
			continue
		}
		hasZone := slices.Contains(current.Zones(), req.Zone)
		if hasZone == !req.Erase {
			continue
		}
		tile := tx.Edit(pos)
		ext := tile.EnsureExt()
		if req.Erase {
			ext.Zones = slices.DeleteFunc(ext.Zones, func(zoneID uint16) bool { return zoneID == req.Zone })
			tile.Compact()
		} else {
			ext.Zones = append(ext.Zones, req.Zone)
		}
	}
}

// drawOptionalBorder marks or unmarks tiles for an optional border and
// borderizes them.
func (e *Editor) drawOptionalBorder(tx *Tx, req DrawRequest, area *posSet) {
	for _, pos := range area.list {
		current := tx.Get(pos)
		if req.Erase {
			if current == nil || !e.hasOptionalBorder(current) {
				continue
			}
			// The pieces count as a mark too (the mark is not saved with
			// the map), so they go with it.
			tile := tx.Edit(pos)
			tile.State &^= otmap.StateOptionalBorder
			tile.Items = slices.DeleteFunc(slices.Clone(tile.Items), func(item otmap.Item) bool {
				info := e.Brushes.info[item.ID]
				return info != nil && info.optionalBorder
			})
		} else {
			if ground := e.groundBrushOf(current); ground != nil && ground.optional != nil {
				continue // no gravel on a mountain
			}
			tx.Edit(pos).State |= otmap.StateOptionalBorder
		}
		e.borderize(tx, pos)
	}
}

func (e *Editor) drawRaw(tx *Tx, req DrawRequest, area *posSet) error {
	itemType := e.itemType(req.Item)
	if itemType == nil || req.Item == 0 {
		return fmt.Errorf("%w: item %d", ErrNoBrush, req.Item)
	}
	isRawItem := func(item otmap.Item) bool { return item.ID == req.Item }
	for _, pos := range area.list {
		if req.Erase {
			current := tx.Get(pos)
			if current == nil {
				continue
			}
			if current.Ground.ID != req.Item && !slices.ContainsFunc(current.Items, isRawItem) {
				continue
			}
			tile := tx.Edit(pos)
			if tile.Ground.ID == req.Item {
				tile.Ground = otmap.Item{}
			}
			tile.Items = slices.DeleteFunc(tile.Items, isRawItem)
			continue
		}
		tile := tx.Edit(pos)
		if e.Settings.RawReplaceBottom && itemType.TopOrder == bottomTopOrder {
			tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool { return e.topOrder(item.ID) == bottomTopOrder })
		}
		e.addItem(tile, e.newItem(req.Item))
	}
	return nil
}

func (e *Editor) drawGround(tx *Tx, req DrawRequest, area *posSet) error {
	brush := e.Brushes.Ground(req.Name)
	if brush == nil {
		return fmt.Errorf("%w: ground %q", ErrNoBrush, req.Name)
	}
	if req.Alt && !req.Erase && e.replace.stroke != req.Stroke && len(req.Points) > 0 {
		startTile := e.Map.Tile(req.Points[0])
		e.replace = strokeReplace{stroke: req.Stroke, brush: e.groundBrushOf(startTile), empty: e.groundBrushOf(startTile) == nil}
	}
	for _, pos := range area.list {
		current := tx.Get(pos)
		if req.Erase {
			if current == nil || e.groundBrushOf(current) != brush {
				continue
			}
			tile := tx.Edit(pos)
			tile.Ground = otmap.Item{}
			continue
		}
		if req.Alt {
			currentBrush := e.groundBrushOf(current)
			if e.replace.empty {
				if currentBrush != nil {
					continue
				}
			} else if currentBrush != e.replace.brush {
				continue
			}
		}
		if len(brush.items.ids) == 0 {
			continue
		}
		tile := tx.Edit(pos)
		if e.Settings.Automagic {
			e.cleanBorders(tile)
		}
		tile.Ground = e.newItem(brush.items.pick(e.rng))
	}
	if e.Settings.Automagic {
		for _, pos := range area.ring().list {
			e.borderize(tx, pos)
		}
	}
	return nil
}

func (e *Editor) drawEraser(tx *Tx, req DrawRequest, area *posSet) {
	for _, pos := range area.list {
		current := tx.Get(pos)
		if current == nil || (current.Ground.ID == 0 && len(current.Items) == 0) {
			continue
		}
		tile := tx.Edit(pos)
		leaveUnique := e.Settings.EraserLeaveUnique
		tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
			if leaveUnique && (item.Complex() || (!req.Erase && e.Brushes.isBorder(item.ID))) {
				return false
			}
			return true
		})
		if req.Erase && tile.Ground.ID != 0 && !(leaveUnique && tile.Ground.Complex()) {
			tile.Ground = otmap.Item{}
		}
	}
	if e.Settings.Automagic {
		for _, pos := range area.ring().list {
			e.borderize(tx, pos)
			e.wallize(tx, pos)
			e.tableize(tx, pos)
			e.carpetize(tx, pos)
		}
	}
}

func (e *Editor) drawWalls(tx *Tx, req DrawRequest, area *posSet) error {
	wall := e.Brushes.Wall(req.Name)
	if wall == nil {
		return fmt.Errorf("%w: wall %q", ErrNoBrush, req.Name)
	}
	if req.Alt && req.Size == 0 && req.Rect == nil && !req.Erase {
		// Alt-click changes the shape of the wall under the cursor.
		for _, pos := range area.list {
			if tx.Get(pos) != nil {
				e.cycleWall(tx.Edit(pos), wall)
			}
		}
		return nil
	}
	for _, pos := range area.list {
		current := tx.Get(pos)
		if req.Erase {
			if current == nil || !e.hasWall(current) {
				continue
			}
			e.cleanWalls(tx.Edit(pos), wall)
			continue
		}
		e.drawWall(tx.Edit(pos), wall)
	}
	if e.Settings.Automagic && !wall.decoration {
		for _, pos := range area.ring().list {
			e.wallize(tx, pos)
		}
	}
	return nil
}

func (e *Editor) drawCarpetTable(tx *Tx, req DrawRequest, area *posSet) error {
	isCarpet := req.Kind == KindCarpet
	carpetBrush, tableBrush := e.Brushes.Carpet(req.Name), e.Brushes.Table(req.Name)
	if (isCarpet && carpetBrush == nil) || (!isCarpet && tableBrush == nil) {
		return fmt.Errorf("%w: %s %q", ErrNoBrush, req.Kind, req.Name)
	}
	for _, pos := range area.list {
		current := tx.Get(pos)
		if req.Erase {
			if current == nil || (isCarpet && !e.hasCarpet(current)) || (!isCarpet && !e.hasTable(current)) {
				continue
			}
			tile := tx.Edit(pos)
			if isCarpet {
				e.removeCarpets(tile)
			} else {
				e.removeTables(tile)
			}
			continue
		}
		tile := tx.Edit(pos)
		if isCarpet {
			e.drawCarpet(tile, carpetBrush)
		} else {
			e.drawTable(tile, tableBrush)
		}
	}
	for _, pos := range area.ring().list {
		if isCarpet {
			e.carpetize(tx, pos)
		} else {
			e.tableize(tx, pos)
		}
	}
	return nil
}

func (e *Editor) drawDoorRequest(tx *Tx, req DrawRequest) {
	doorType := slices.Index(materials.DoorTypes, req.DoorType) + 1
	if doorType <= 0 {
		doorType = doorTypeNormal
	}
	for _, target := range req.Points {
		if req.Erase {
			e.removeDoor(tx, target)
		} else {
			e.drawDoor(tx, target, doorType, req.Alt)
		}
	}
	if e.Settings.Automagic {
		points := newPosSet()
		for _, target := range req.Points {
			points.add(target)
		}
		for _, pos := range points.ring().list {
			e.wallize(tx, pos)
		}
	}
}

func (e *Editor) drawDoodadRequest(tx *Tx, req DrawRequest) error {
	doodad := e.Brushes.Doodad(req.Name)
	if doodad == nil {
		return fmt.Errorf("%w: doodad %q", ErrNoBrush, req.Name)
	}
	if req.Erase {
		for _, pos := range e.area(req).list {
			if current := tx.Get(pos); current != nil {
				itemsBefore := len(current.Items)
				groundBefore := current.Ground.ID
				probe := current.Clone()
				e.eraseDoodad(probe, doodad)
				if len(probe.Items) != itemsBefore || probe.Ground.ID != groundBefore {
					e.eraseDoodad(tx.Edit(pos), doodad)
				}
			}
		}
		return nil
	}
	generated := e.doodad
	if generated == nil || !strings.EqualFold(generated.Brush, req.Name) || generated.Size != req.Size || generated.Shape != req.Shape || generated.Variation != req.Variation {
		e.GenerateDoodad(req.Name, req.Variation, req.Size, req.Shape, req.Thickness)
	}
	for _, center := range req.Points {
		e.placeDoodad(tx, center, req.Alt, req.Thickness)
	}
	return nil
}

func (e *Editor) drawHouse(tx *Tx, req DrawRequest, area *posSet) error {
	if req.House == 0 || (!req.Erase && e.Map.Houses[req.House] == nil) {
		return fmt.Errorf("%w: house %d", ErrNoBrush, req.House)
	}
	for _, pos := range area.list {
		current := tx.Get(pos)
		if current == nil || current.Ground.ID == 0 {
			continue
		}
		if req.Erase {
			if current.HouseID == 0 || current.HouseID != req.House {
				continue
			}
			eraseHouseTile(tx.Edit(pos))
			continue
		}
		if current.HouseID == req.House {
			continue
		}
		e.addHouseTile(tx, tx.Edit(pos), req.House)
	}
	return nil
}

// eraseHouseTile takes a tile out of its house and clears its door ids.
func eraseHouseTile(tile *otmap.Tile) {
	tile.HouseID = 0
	tile.Flags &^= otmap.FlagPZ
	for i := range tile.Items {
		if tile.Items[i].Ext != nil && tile.Items[i].Ext.DoorID != 0 {
			tile.Items[i].Ext.DoorID = 0
			tile.Items[i].Compact()
		}
	}
}

// addHouseTile puts a tile into a house, removing loose items and giving
// its doors ids when the settings say so.
func (e *Editor) addHouseTile(tx *Tx, tile *otmap.Tile, houseID uint32) {
	tile.HouseID = houseID
	tile.Flags |= otmap.FlagPZ
	if e.Settings.HouseRemoveItems {
		tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool {
			itemType := e.itemType(item.ID)
			return itemType != nil && itemType.Flags.Has(otitems.FlagMoveable) && !item.Complex()
		})
	}
	if e.Settings.AutoDoorID {
		for i := range tile.Items {
			if itemType := e.itemType(tile.Items[i].ID); itemType != nil && itemType.IsDoor() {
				e.assignDoorID(tx, tile, i)
			}
		}
	}
}

// assignDoorID gives a door on a house tile the lowest free door id of the
// house.
func (e *Editor) assignDoorID(tx *Tx, tile *otmap.Tile, i int) {
	if tile.HouseID == 0 {
		return
	}
	if tile.Items[i].Ext != nil && tile.Items[i].Ext.DoorID != 0 {
		return
	}
	used := map[uint8]bool{}
	for _, chunk := range e.Map.Chunks() {
		chunk.Each(func(_ otmap.Position, other *otmap.Tile) {
			if other.HouseID != tile.HouseID {
				return
			}
			for _, item := range other.Items {
				if item.Ext != nil && item.Ext.DoorID != 0 {
					used[item.Ext.DoorID] = true
				}
			}
		})
	}
	for id := 1; id <= math.MaxUint8; id++ {
		if !used[uint8(id)] {
			tile.Items[i].Ensure().DoorID = uint8(id)
			return
		}
	}
}

func (e *Editor) drawMarker(tx *Tx, req DrawRequest) error {
	target := req.Points[len(req.Points)-1]
	switch req.Kind {
	case KindHouseExit:
		return e.drawHouseExit(tx, req, target)
	case KindWaypoint:
		return e.drawWaypoint(tx, req, target)
	}
	return nil
}

// drawHouseExit moves the exit of a house to target, or clears it.
func (e *Editor) drawHouseExit(tx *Tx, req DrawRequest, target otmap.Position) error {
	house := e.Map.Houses[req.House]
	if house == nil {
		return fmt.Errorf("%w: house %d", ErrNoBrush, req.House)
	}
	if req.Erase {
		if house.Exit != target {
			return nil
		}
		tx.Meta()
		house.Exit = otmap.Position{}
		return nil
	}
	current := tx.Get(target)
	if current == nil || current.Ground.ID == 0 || current.HouseID != 0 || e.blocking(current) {
		return errors.New("a house exit must be on a walkable tile outside the house")
	}
	tx.Meta()
	house.Exit = target
	return nil
}

// drawWaypoint moves or adds a waypoint at target, or removes the waypoint
// there.
func (e *Editor) drawWaypoint(tx *Tx, req DrawRequest, target otmap.Position) error {
	if req.Erase {
		for i, waypoint := range e.Map.Waypoints {
			if waypoint.Pos == target {
				tx.Meta()
				e.Map.Waypoints = slices.Delete(e.Map.Waypoints, i, i+1)
				return nil
			}
		}
		return nil
	}
	if req.Waypoint == "" {
		return errors.New("choose a waypoint")
	}
	tx.Meta()
	if waypoint := e.Map.Waypoint(req.Waypoint); waypoint != nil {
		waypoint.Pos = target
	} else {
		e.Map.Waypoints = append(e.Map.Waypoints, &otmap.Waypoint{Name: req.Waypoint, Pos: target})
	}
	return nil
}

func (e *Editor) drawSpawn(tx *Tx, req DrawRequest) {
	isNPC := req.Kind == KindSpawnNPC
	for _, center := range req.Points {
		current := tx.Get(center)
		if req.Erase {
			if current == nil || (isNPC && current.SpawnNPC() == nil) || (!isNPC && current.Spawn() == nil) {
				continue
			}
			tile := tx.Edit(center)
			if isNPC {
				tile.Ext.SpawnNPC = nil
			} else {
				tile.Ext.Spawn = nil
			}
			tile.Compact()
			continue
		}
		tile := tx.Edit(center)
		spawn := &otmap.Spawn{Radius: int32(max(req.Size, 1))}
		if isNPC {
			tile.EnsureExt().SpawnNPC = spawn
		} else {
			tile.EnsureExt().Spawn = spawn
		}
	}
}

// covered reports whether a spawn area covers target.
func (e *Editor) covered(target otmap.Position, npc bool) bool {
	x, y := int(target.X), int(target.Y)
	found := false
	e.Map.EachIn(x-maxSpawnRadius, y-maxSpawnRadius, x+maxSpawnRadius, y+maxSpawnRadius, int(target.Z), int(target.Z), func(pos otmap.Position, tile *otmap.Tile) {
		if found {
			return
		}
		spawn := tile.Spawn()
		if npc && e.Map.SpawnFormat == otmap.SpawnsSeparate {
			spawn = tile.SpawnNPC()
		} else if spawn == nil {
			spawn = tile.SpawnNPC()
		}
		if spawn != nil && abs(int(pos.X)-x) <= int(spawn.Radius) && abs(int(pos.Y)-y) <= int(spawn.Radius) {
			found = true
		}
	})
	return found
}

func (e *Editor) drawCreature(tx *Tx, req DrawRequest) error {
	if !req.Erase && req.Creature == "" {
		return errors.New("choose a creature")
	}
	for _, target := range req.Points {
		current := tx.Get(target)
		if req.Erase {
			if current == nil || len(current.Creatures()) == 0 {
				continue
			}
			tile := tx.Edit(target)
			tile.Ext.Creatures = nil
			tile.Compact()
			continue
		}
		if current == nil || current.Ground.ID == 0 || (e.blocking(current) && !req.Alt) {
			continue
		}
		tile := tx.Edit(target)
		spawnTime := req.SpawnTime
		if spawnTime <= 0 {
			spawnTime = e.Settings.DefaultSpawnTime
		}
		creature := otmap.Creature{Name: req.Creature, NPC: req.NPC, SpawnTime: spawnTime, Direction: req.Direction}
		tile.EnsureExt().Creatures = []otmap.Creature{creature}
		if e.Settings.AutoCreateSpawn && !e.covered(target, req.NPC) {
			if req.NPC && e.Map.SpawnFormat == otmap.SpawnsSeparate {
				tile.Ext.SpawnNPC = &otmap.Spawn{Radius: 1}
			} else {
				tile.Ext.Spawn = &otmap.Spawn{Radius: 1}
			}
		}
	}
	return nil
}
