package mapedit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Ways a path changes place other than walking (PathPoint.Via).
const (
	ViaDoor     = "door"
	ViaDown     = "down"
	ViaUp       = "up"
	ViaLadder   = "ladder"
	ViaRopeSpot = "rope spot"
	ViaTeleport = "teleport"
)

const (
	// DefaultPathLimit and MaxPathLimit bound the tiles a path search
	// visits.
	DefaultPathLimit = 250_000
	MaxPathLimit     = 2_000_000
	// maxWaypoints limits the turns a path lists; past it only the points
	// where it passes a door or changes floor are kept.
	maxWaypoints = 200
)

// neighborOffsets are the eight walking directions.
var neighborOffsets = [8][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1}}

// PathPoint is a point of a path: a turn, or where it passes a door or
// reaches another place by Via.
type PathPoint struct {
	Pos otmap.Position `json:"pos"`
	Via string         `json:"via,omitempty"`
}

type PathResult struct {
	Found bool `json:"found"`
	// Steps is the number of moves of the path.
	Steps     int         `json:"steps,omitempty"`
	Waypoints []PathPoint `json:"waypoints,omitempty"`
	// Closest is the reached tile nearest to the target when there is no
	// path, Distance its distance (tiles, 10 per floor).
	Closest  *otmap.Position `json:"closest,omitempty"`
	Distance int             `json:"distance,omitempty"`
	Explored int             `json:"explored"`
	// Truncated: the search stopped at its limit.
	Truncated bool `json:"truncated,omitempty"`
}

type pathMove struct {
	to  otmap.Position
	via string
}

type pathNode struct {
	from otmap.Position
	via  string
}

// walkability of a tile for paths.
type walkability int

const (
	notWalkable walkability = iota
	walkable
	// throughDoor: walkable once its closed door opens.
	throughDoor
)

func (e *Editor) walkability(tile *otmap.Tile) walkability {
	if tile == nil || tile.Ground.ID == 0 {
		return notWalkable
	}
	door := false
	unpassable := func(id uint16) bool {
		itemType := e.itemType(id)
		if itemType == nil || !itemType.Flags.Has(otitems.FlagUnpassable) {
			return false
		}
		if e.isDoor(id, itemType) {
			door = true
			return false
		}
		return true
	}
	if unpassable(tile.Ground.ID) || slices.ContainsFunc(tile.Items, func(item otmap.Item) bool { return unpassable(item.ID) }) {
		return notWalkable
	}
	if door {
		return throughDoor
	}
	return walkable
}

func (e *Editor) isDoor(id uint16, itemType *otitems.Type) bool {
	if itemType != nil && itemType.IsDoor() {
		return true
	}
	info := e.Brushes.info[id]
	return info != nil && info.door
}

// isFloorChange reports whether a tile moves who steps on it to another
// floor.
func (e *Editor) isFloorChange(tile *otmap.Tile) bool {
	return tile != nil && e.hasFlag(tile, otitems.FlagFloorChange)
}

// standable reports whether a path can stop on the tile at pos: walkable
// and not moving on to another floor.
func (e *Editor) standable(pos otmap.Position) bool {
	tile := e.Map.Tile(pos)
	return e.walkability(tile) != notWalkable && !e.isFloorChange(tile)
}

// landing returns pos when a path can stop there, else the first
// standable neighbor of it.
func (e *Editor) landing(pos otmap.Position, self bool) (otmap.Position, bool) {
	if self && e.standable(pos) {
		return pos, true
	}
	for _, offset := range neighborOffsets {
		if next, ok := offsetPos(pos, offset[0], offset[1], 0); ok && e.standable(next) {
			return next, true
		}
	}
	return otmap.Position{}, false
}

func offsetPos(pos otmap.Position, dx, dy, dz int) (otmap.Position, bool) {
	x, y, z := int(pos.X)+dx, int(pos.Y)+dy, int(pos.Z)+dz
	if x < 0 || y < 0 || x > 0xFFFF || y > 0xFFFF || z < 0 || z > otmap.MaxFloor {
		return otmap.Position{}, false
	}
	return otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(z)}, true
}

// transitions returns where a tile takes who steps on or uses it: the
// floor below or above for floor change items (the direction is guessed:
// down when a tile lies under it, up when none lies over it), the floor
// above for ladders and rope spots, the destination of teleports.
func (e *Editor) transitions(pos otmap.Position, tile *otmap.Tile) []pathMove {
	var moves []pathMove
	for _, item := range tile.Items {
		if item.Ext == nil || item.Ext.Dest == nil {
			continue
		}
		if itemType := e.itemType(item.ID); itemType != nil && itemType.IsTeleport() {
			if to, ok := e.landing(*item.Ext.Dest, true); ok {
				moves = append(moves, pathMove{to, ViaTeleport})
			}
		}
	}
	if e.isFloorChange(tile) {
		if below, ok := offsetPos(pos, 0, 0, 1); ok && e.Map.Tile(below) != nil {
			if to, ok := e.landing(below, true); ok {
				moves = append(moves, pathMove{to, ViaDown})
			}
		}
		if above, ok := offsetPos(pos, 0, 0, -1); ok && e.walkability(e.Map.Tile(above)) == notWalkable {
			if to, ok := e.landing(above, false); ok {
				moves = append(moves, pathMove{to, ViaUp})
			}
		}
	}
	if via := e.climbItem(tile); via != "" {
		if above, ok := offsetPos(pos, 0, 0, -1); ok {
			south, _ := offsetPos(above, 0, 1, 0)
			if e.standable(south) {
				moves = append(moves, pathMove{south, via})
			} else if to, ok := e.landing(above, false); ok {
				moves = append(moves, pathMove{to, via})
			}
		}
	}
	return moves
}

// climbItem names a ladder or rope spot on a tile (used to go up).
func (e *Editor) climbItem(tile *otmap.Tile) string {
	check := func(id uint16) string {
		itemType := e.itemType(id)
		if itemType == nil {
			return ""
		}
		name := strings.ToLower(itemType.Name)
		switch {
		case strings.Contains(name, "ladder"):
			return ViaLadder
		case strings.Contains(name, "rope spot"):
			return ViaRopeSpot
		}
		return ""
	}
	if via := check(tile.Ground.ID); via != "" {
		return via
	}
	for _, item := range tile.Items {
		if via := check(item.ID); via != "" {
			return via
		}
	}
	return ""
}

func pathDistance(from, to otmap.Position) int {
	dx, dy, dz := absInt(int(from.X)-int(to.X)), absInt(int(from.Y)-int(to.Y)), absInt(int(from.Z)-int(to.Z))
	return max(dx, dy) + 10*dz
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// FindPath searches the shortest way a player walks from one tile to
// another (diagonal steps included), through doors, floor changes, ladders,
// rope spots and teleports. limit bounds the tiles visited.
func (e *Editor) FindPath(from, to otmap.Position, limit int) (PathResult, error) {
	if limit <= 0 {
		limit = DefaultPathLimit
	}
	limit = min(limit, MaxPathLimit)
	if e.walkability(e.Map.Tile(from)) == notWalkable {
		return PathResult{}, fmt.Errorf("the start %d,%d,%d is not walkable", from.X, from.Y, from.Z)
	}
	if e.walkability(e.Map.Tile(to)) == notWalkable {
		return PathResult{}, fmt.Errorf("the target %d,%d,%d is not walkable", to.X, to.Y, to.Z)
	}
	parents := map[otmap.Position]pathNode{from: {from: from}}
	queue := []otmap.Position{from}
	closest, closestDistance := from, pathDistance(from, to)
	var result PathResult
	for len(queue) > 0 {
		pos := queue[0]
		queue = queue[1:]
		result.Explored++
		if pos == to {
			result.Found = true
			result.Steps, result.Waypoints = pathOf(parents, from, to)
			return result, nil
		}
		if distance := pathDistance(pos, to); distance < closestDistance {
			closest, closestDistance = pos, distance
		}
		if len(parents) >= limit {
			result.Truncated = true
			continue
		}
		tile := e.Map.Tile(pos)
		moves := e.transitions(pos, tile)
		// Who steps on a floor change is moved on at once.
		if pos == from || !e.isFloorChange(tile) {
			for _, offset := range neighborOffsets {
				next, ok := offsetPos(pos, offset[0], offset[1], 0)
				if !ok {
					continue
				}
				switch e.walkability(e.Map.Tile(next)) {
				case walkable:
					moves = append(moves, pathMove{next, ""})
				case throughDoor:
					moves = append(moves, pathMove{next, ViaDoor})
				}
			}
		}
		for _, move := range moves {
			if _, seen := parents[move.to]; seen {
				continue
			}
			parents[move.to] = pathNode{from: pos, via: move.via}
			queue = append(queue, move.to)
		}
	}
	result.Closest, result.Distance = &closest, closestDistance
	return result, nil
}

// pathOf walks the parents back from to and returns the number of moves
// and the turns and special points of the path, start and end included.
func pathOf(parents map[otmap.Position]pathNode, from, to otmap.Position) (int, []PathPoint) {
	var reversed []PathPoint
	for pos := to; ; {
		node := parents[pos]
		reversed = append(reversed, PathPoint{Pos: pos, Via: node.via})
		if pos == from {
			break
		}
		pos = node.from
	}
	slices.Reverse(reversed)
	steps := len(reversed) - 1
	kept := []PathPoint{reversed[0]}
	special := []PathPoint{reversed[0]}
	for i := 1; i < len(reversed); i++ {
		point := reversed[i]
		last := i == len(reversed)-1
		if point.Via != "" || last {
			special = append(special, point)
		}
		if !last && point.Via == "" && reversed[i+1].Via == "" && direction(reversed[i-1].Pos, point.Pos) == direction(point.Pos, reversed[i+1].Pos) {
			continue
		}
		kept = append(kept, point)
	}
	if len(kept) > maxWaypoints {
		return steps, special
	}
	return steps, kept
}

func direction(from, to otmap.Position) [3]int {
	return [3]int{int(to.X) - int(from.X), int(to.Y) - int(from.Y), int(to.Z) - int(from.Z)}
}

// Problem is something wrong in a map that Validate found.
type Problem struct {
	Kind string          `json:"kind"`
	Pos  *otmap.Position `json:"pos,omitempty"`
	Text string          `json:"text"`
}

// Problem kinds.
const (
	ProblemTemple      = "temple"
	ProblemHouseExit   = "house exit"
	ProblemHouseTiles  = "house tiles"
	ProblemUnknownHome = "unknown house"
	ProblemWaypoint    = "waypoint"
	ProblemTeleport    = "teleport"
	ProblemUniqueID    = "unique id"
	ProblemCreature    = "creature"
	ProblemSpawn       = "empty spawn"
	ProblemUnknownItem = "unknown item"
	ProblemFloorChange = "floor change"
	ProblemZone        = "unknown zone"
)

// Validation is what Validate found: problems (at most the limit) and the
// count of each kind.
type Validation struct {
	Problems []Problem      `json:"problems"`
	Counts   map[string]int `json:"counts"`
	// More: more problems were found than listed.
	More bool `json:"more,omitempty"`
}

type validator struct {
	validation *Validation
	limit      int
}

func (v *validator) add(kind string, pos *otmap.Position, format string, args ...any) {
	v.validation.Counts[kind]++
	if len(v.validation.Problems) >= v.limit {
		v.validation.More = true
		return
	}
	if pos != nil {
		at := *pos
		pos = &at
	}
	v.validation.Problems = append(v.validation.Problems, Problem{Kind: kind, Pos: pos, Text: fmt.Sprintf(format, args...)})
}

// Validate looks for mistakes that break a map in game: temples, house
// exits, waypoints and teleport destinations that cannot be stood on,
// houses without tiles, tiles of missing houses or zones, duplicate unique
// ids, creatures on blocking tiles, spawns without creatures, unknown items
// and floor changes that lead nowhere. It does not change the map.
func (e *Editor) Validate(limit int) Validation {
	validation := Validation{Problems: []Problem{}, Counts: map[string]int{}}
	check := &validator{validation: &validation, limit: limit}
	walkableAt := func(pos otmap.Position) bool { return e.walkability(e.Map.Tile(pos)) != notWalkable }

	for _, id := range sortedKeys(e.Map.Towns) {
		town := e.Map.Towns[id]
		if !walkableAt(town.Temple) {
			check.add(ProblemTemple, &town.Temple, "the temple of town %d %q is not walkable", town.ID, town.Name)
		}
	}
	houseTiles := map[uint32]int{}
	zones := map[uint16]bool{}
	for _, zone := range e.Map.Zones {
		zones[zone.ID] = true
	}
	uniqueIDs := map[uint16]otmap.Position{}
	var spawns []otmap.Position
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		if tile.HouseID != 0 {
			if e.Map.Houses[tile.HouseID] == nil {
				check.add(ProblemUnknownHome, &pos, "the tile belongs to house %d, which does not exist", tile.HouseID)
			}
			houseTiles[tile.HouseID]++
		}
		if tile.Ground.ID != 0 && e.itemType(tile.Ground.ID) == nil {
			check.add(ProblemUnknownItem, &pos, "unknown ground %d", tile.Ground.ID)
		}
		for _, item := range tile.Items {
			itemType := e.itemType(item.ID)
			if itemType == nil {
				check.add(ProblemUnknownItem, &pos, "unknown item %d", item.ID)
				continue
			}
			e.checkItem(check, pos, item, itemType, uniqueIDs, walkableAt)
		}
		if tile.Ext != nil {
			for _, zone := range tile.Ext.Zones {
				if !zones[zone] {
					check.add(ProblemZone, &pos, "the tile is in zone %d, which does not exist", zone)
				}
			}
			if len(tile.Ext.Creatures) > 0 && e.walkability(tile) == notWalkable {
				check.add(ProblemCreature, &pos, "%s stands on a tile that is not walkable", tile.Ext.Creatures[0].Name)
			}
			if tile.Ext.Spawn != nil || tile.Ext.SpawnNPC != nil {
				spawns = append(spawns, pos)
			}
		}
		if e.isFloorChange(tile) && len(e.transitions(pos, tile)) == 0 {
			check.add(ProblemFloorChange, &pos, "the floor change leads to no walkable tile on the floor below or above")
		}
	})
	for _, id := range sortedKeys(e.Map.Houses) {
		house := e.Map.Houses[id]
		if houseTiles[id] == 0 {
			check.add(ProblemHouseTiles, nil, "house %d %q has no tiles", house.ID, house.Name)
		}
		exit := house.Exit
		switch {
		case exit == (otmap.Position{}):
			check.add(ProblemHouseExit, nil, "house %d %q has no exit", house.ID, house.Name)
		case !walkableAt(exit):
			check.add(ProblemHouseExit, &exit, "the exit of house %d %q is not walkable", house.ID, house.Name)
		case e.Map.Tile(exit).HouseID == id:
			check.add(ProblemHouseExit, &exit, "the exit of house %d %q is inside the house", house.ID, house.Name)
		}
	}
	for _, waypoint := range e.Map.Waypoints {
		if !walkableAt(waypoint.Pos) {
			check.add(ProblemWaypoint, &waypoint.Pos, "waypoint %q is not walkable", waypoint.Name)
		}
	}
	for _, pos := range spawns {
		e.checkSpawn(check, pos)
	}
	return validation
}

func (e *Editor) checkItem(check *validator, pos otmap.Position, item otmap.Item, itemType *otitems.Type, uniqueIDs map[uint16]otmap.Position, walkableAt func(otmap.Position) bool) {
	if item.Ext != nil && item.Ext.UniqueID != 0 {
		if first, ok := uniqueIDs[item.Ext.UniqueID]; ok {
			check.add(ProblemUniqueID, &pos, "unique id %d is used again (first at %d,%d,%d)", item.Ext.UniqueID, first.X, first.Y, first.Z)
		} else {
			uniqueIDs[item.Ext.UniqueID] = pos
		}
	}
	if !itemType.IsTeleport() {
		return
	}
	switch {
	case item.Ext == nil || item.Ext.Dest == nil || *item.Ext.Dest == (otmap.Position{}):
		check.add(ProblemTeleport, &pos, "teleport %d has no destination", item.ID)
	case !walkableAt(*item.Ext.Dest):
		dest := *item.Ext.Dest
		check.add(ProblemTeleport, &pos, "teleport %d leads to %d,%d,%d, which is not walkable", item.ID, dest.X, dest.Y, dest.Z)
	}
}

// checkSpawn reports a spawn with no creature within its radius.
func (e *Editor) checkSpawn(check *validator, pos otmap.Position) {
	tile := e.Map.Tile(pos)
	for _, spawn := range []struct {
		area *otmap.Spawn
		npc  bool
	}{{tile.Ext.Spawn, false}, {tile.Ext.SpawnNPC, true}} {
		if spawn.area == nil {
			continue
		}
		radius := int(max(spawn.area.Radius, 0))
		found := false
		for dy := -radius; dy <= radius && !found; dy++ {
			for dx := -radius; dx <= radius && !found; dx++ {
				at, ok := offsetPos(pos, dx, dy, 0)
				if !ok {
					continue
				}
				if other := e.Map.Tile(at); other != nil && other.Ext != nil {
					found = slices.ContainsFunc(other.Ext.Creatures, func(creature otmap.Creature) bool { return creature.NPC == spawn.npc })
				}
			}
		}
		if !found {
			kind := "monster"
			if spawn.npc {
				kind = "NPC"
			}
			check.add(ProblemSpawn, &pos, "the %s spawn (radius %d) holds no creature", kind, radius)
		}
	}
}

func sortedKeys[V any](values map[uint32]V) []uint32 {
	keys := make([]uint32, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
