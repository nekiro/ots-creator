package mapedit

import (
	"cmp"
	"slices"
	"sort"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// span is a run of x positions in one row, from and to included.
type span struct{ from, to int }

// rowSpans maps a row (y) to its sorted, disjoint spans.
type rowSpans map[int][]span

func (rows rowSpans) contains(x, y int) bool {
	spans := rows[y]
	index := sort.Search(len(spans), func(i int) bool { return spans[i].to >= x })
	return index < len(spans) && spans[index].from <= x
}

// floorRows holds, per floor, the x positions of the walkable tiles of
// each row (y).
type floorRows [otmap.MaxFloor + 1]map[int][]int

// RemoveUnreachable removes tiles no player can see: tiles farther than
// the client view (9x7 tiles) from any walkable tile within two floors.
//
// The map is read twice: once to collect the walkable tiles, once to test
// every tile against the area they make visible, kept as row spans.
func (e *Editor) RemoveUnreachable() int {
	progress := e.progressOver(2 * e.Map.TileCount())
	var walkable floorRows
	for floor := range walkable {
		walkable[floor] = map[int][]int{}
	}
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		if !e.blocking(tile) {
			rows := walkable[pos.Z]
			rows[int(pos.Y)] = append(rows[int(pos.Y)], int(pos.X))
		}
		progress()
	})
	visible := visibleSpans(walkable)
	var unreachable []otmap.Position
	e.Map.Each(func(pos otmap.Position, _ *otmap.Tile) {
		if !visible[pos.Z].contains(int(pos.X), int(pos.Y)) {
			unreachable = append(unreachable, pos)
		}
		progress()
	})
	if len(unreachable) == 0 {
		return 0
	}
	tx := e.Begin("Remove unreachable tiles", 0)
	for _, pos := range unreachable {
		tile := tx.Edit(pos)
		*tile = otmap.Tile{}
	}
	return tx.Commit()
}

// visibleSpans returns, per floor, the spans of each row that a player on
// a walkable tile sees: within clientViewRangeX/Y tiles and
// clientViewFloorsRange floors of it.
func visibleSpans(walkable floorRows) [otmap.MaxFloor + 1]rowSpans {
	// Walkable tiles widened by the horizontal view range, per floor.
	var widened [otmap.MaxFloor + 1]rowSpans
	for floor, rows := range walkable {
		widened[floor] = rowSpans{}
		for y, xs := range rows {
			slices.Sort(xs)
			var spans []span
			for _, x := range xs {
				spans = appendSpan(spans, span{x - clientViewRangeX, x + clientViewRangeX})
			}
			widened[floor][y] = spans
		}
	}
	var visible [otmap.MaxFloor + 1]rowSpans
	for floor := range visible {
		rows := rowSpans{}
		for source := max(floor-clientViewFloorsRange, 0); source <= min(floor+clientViewFloorsRange, otmap.MaxFloor); source++ {
			for y, spans := range widened[source] {
				for dy := -clientViewRangeY; dy <= clientViewRangeY; dy++ {
					rows[y+dy] = append(rows[y+dy], spans...)
				}
			}
		}
		for y, spans := range rows {
			rows[y] = mergeSpans(spans)
		}
		visible[floor] = rows
	}
	return visible
}

// mergeSpans sorts spans and joins the ones that overlap or touch, in place.
func mergeSpans(spans []span) []span {
	slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.from, b.from) })
	merged := spans[:0]
	for _, next := range spans {
		merged = appendSpan(merged, next)
	}
	return merged
}

// appendSpan adds a span that starts at or after the start of the last
// one, joining them when they overlap or touch.
func appendSpan(spans []span, next span) []span {
	if last := len(spans) - 1; last >= 0 && next.from <= spans[last].to+1 {
		spans[last].to = max(spans[last].to, next.to)
		return spans
	}
	return append(spans, next)
}
