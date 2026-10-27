package mapedit

import (
	"cmp"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// borderPieces returns the border pieces a tile gets from neighbors (bits
// of neighbors8) that hold the other brush. Two adjacent sides make a
// diagonal; other sides make straight pieces; a corner neighbor makes a
// corner piece when neither of its sides is set.
func borderPieces(mask int) []int {
	north, east, south, west := mask&bitN != 0, mask&bitE != 0, mask&bitS != 0, mask&bitW != 0
	sides := 0
	for _, side := range []bool{north, east, south, west} {
		if side {
			sides++
		}
	}
	var pieces []int
	switch {
	case sides == 2 && north && west:
		pieces = append(pieces, edgeDNW)
	case sides == 2 && north && east:
		pieces = append(pieces, edgeDNE)
	case sides == 2 && south && east:
		pieces = append(pieces, edgeDSE)
	case sides == 2 && south && west:
		pieces = append(pieces, edgeDSW)
	default:
		if north {
			pieces = append(pieces, edgeN)
		}
		if east {
			pieces = append(pieces, edgeE)
		}
		if south {
			pieces = append(pieces, edgeS)
		}
		if west {
			pieces = append(pieces, edgeW)
		}
	}
	if mask&bitNW != 0 && !north && !west {
		pieces = append(pieces, edgeCNW)
	}
	if mask&bitNE != 0 && !north && !east {
		pieces = append(pieces, edgeCNE)
	}
	if mask&bitSW != 0 && !south && !west {
		pieces = append(pieces, edgeCSW)
	}
	if mask&bitSE != 0 && !south && !east {
		pieces = append(pieces, edgeCSE)
	}
	return pieces
}

// diagonalSides returns the two side pieces a diagonal piece is made of
// when its border set lacks it (edgeNone for other pieces).
func diagonalSides(diagonal int) (first, second int) {
	switch diagonal {
	case edgeDNW:
		return edgeW, edgeN
	case edgeDNE:
		return edgeE, edgeN
	case edgeDSW:
		return edgeS, edgeW
	case edgeDSE:
		return edgeS, edgeE
	}
	return edgeNone, edgeNone
}

// findBorderBlock returns the first inner or outer border block of the
// brush that targets the brush id or all brushes.
func findBorderBlock(brush *GroundBrush, outer bool, target int) *borderBlock {
	for _, block := range brush.borders {
		if block.outer == outer && (block.to == target || block.to == toAll) {
			return block
		}
	}
	return nil
}

func findZilchBorderBlock(brush *GroundBrush, outer bool) *borderBlock {
	for _, block := range brush.borders {
		if block.outer == outer && block.to == toNone {
			return block
		}
	}
	return nil
}

// brushTo finds the border placed on a tile of brush first next to a tile
// of brush second (either can be nil: an empty tile).
func brushTo(first, second *GroundBrush) *borderBlock {
	switch {
	case first != nil && second != nil:
		if first.zOrder < second.zOrder && second.hasOuterBorder() {
			if first.hasInner {
				if block := findBorderBlock(first, false, second.id); block != nil {
					return block
				}
			}
			return findBorderBlock(second, true, first.id)
		} else if first.hasInner {
			return findBorderBlock(first, false, second.id)
		}
	case first != nil:
		if first.hasInnerZilch {
			return findZilchBorderBlock(first, false)
		}
	case second != nil:
		if second.hasOuterZilchBorder() {
			return findZilchBorderBlock(second, true)
		}
	}
	return nil
}

type borderCluster struct {
	align  int
	z      int
	border *autoBorder
}

type borderPlan struct {
	ids []uint16 // bottom to top
	// managed holds every piece of the border sets the plan used, so a new
	// plan also replaces other pieces of the same sets.
	managed      map[uint16]bool
	dropOptional bool
}

// borderNeighbor is the ground of a neighbor while borders are planned;
// done marks neighbors already put in a cluster.
type borderNeighbor struct {
	done  bool
	brush *GroundBrush
}

// claimNeighbors marks the neighbors from start on that are not done yet
// and whose brush matches as done, and returns their bits.
func claimNeighbors(neighbors *[8]borderNeighbor, start int, matches func(*GroundBrush) bool) int {
	mask := 0
	for j := start; j < len(neighbors); j++ {
		if !neighbors[j].done && matches(neighbors[j].brush) {
			neighbors[j].done = true
			mask |= 1 << j
		}
	}
	return mask
}

func mergeBorderCluster(clusters []borderCluster, border *autoBorder, mask, z int) []borderCluster {
	for i := range clusters {
		if clusters[i].border == border {
			clusters[i].align |= mask
			clusters[i].z = max(clusters[i].z, z)
			return clusters
		}
	}
	return append(clusters, borderCluster{mask, z, border})
}

// planBorders computes the borders of a tile of brush self at pos, reading
// the neighbors through ground. optional tells whether the tile carries an
// optional (mountain) border.
func (e *Editor) planBorders(pos otmap.Position, self *GroundBrush, optional bool, ground func(otmap.Position) *GroundBrush) borderPlan {
	plan := borderPlan{managed: map[uint16]bool{}}
	var neighbors [8]borderNeighbor
	for i, offset := range neighbors8 {
		if neighborPos, ok := pos.Add(offset[0], offset[1], 0); ok {
			neighbors[i].brush = ground(neighborPos)
		}
	}
	var clusters []borderCluster
	var specificBlocks []*borderBlock
	addSpecific := func(block *borderBlock) {
		if len(block.specific) > 0 && !slices.Contains(specificBlocks, block) {
			specificBlocks = append(specificBlocks, block)
		}
	}
	for i := range neighbors {
		if neighbors[i].done {
			continue
		}
		other := neighbors[i].brush
		isOther := func(brush *GroundBrush) bool { return brush != nil && brush.id == other.id }
		switch {
		case self != nil && other != nil:
			if other.id == self.id {
				continue
			}
			if !(other.hasOuterBorder() || self.hasInner) {
				break
			}
			onlyMountain := false
			if other.friendOf(self) || self.friendOf(other) {
				if other.optional == nil {
					continue
				}
				onlyMountain = true
			}
			mask := claimNeighbors(&neighbors, i, isOther)
			if mask == 0 {
				break
			}
			if other.optional != nil && optional {
				clusters = append(clusters, borderCluster{mask, optionalBorderZ, other.optional})
				if other.soloOptional {
					onlyMountain = true
				}
			}
			if onlyMountain {
				break
			}
			block := brushTo(self, other)
			if block == nil {
				break
			}
			clusters = mergeBorderCluster(clusters, block.border, mask, other.zOrder)
			addSpecific(block)
		case self != nil && other == nil:
			if !self.hasInnerZilch {
				break
			}
			mask := claimNeighbors(&neighbors, i, func(brush *GroundBrush) bool { return brush == nil })
			if mask == 0 {
				break
			}
			if block := brushTo(self, nil); block != nil {
				if block.border != nil {
					clusters = append(clusters, borderCluster{mask, innerZilchBorderZ, block.border})
				}
				addSpecific(block)
			}
			continue
		case self == nil && other != nil:
			if !other.hasOuterZilchBorder() {
				break
			}
			mask := claimNeighbors(&neighbors, i, isOther)
			if mask == 0 {
				break
			}
			if block := brushTo(nil, other); block != nil {
				if block.border != nil {
					clusters = append(clusters, borderCluster{mask, other.zOrder, block.border})
				}
				addSpecific(block)
			}
			if other.optional != nil && optional {
				clusters = append(clusters, borderCluster{mask, optionalBorderZ, other.optional})
			} else {
				plan.dropOptional = true
			}
		}
		neighbors[i].done = true
	}
	plan.stackClusters(clusters)
	e.applySpecificCases(&plan, specificBlocks)
	return plan
}

// stackClusters puts the border pieces of the clusters on the plan, the
// cluster with the lowest z order lowest.
func (plan *borderPlan) stackClusters(clusters []borderCluster) {
	slices.SortStableFunc(clusters, func(left, right borderCluster) int { return cmp.Compare(left.z, right.z) })
	// Each piece goes to the bottom, so the lowest cluster ends lowest. The
	// pieces are collected top first and reversed once at the end.
	var topFirst []uint16
	putBottom := func(id uint16) { topFirst = append(topFirst, id) }
	for i := len(clusters) - 1; i >= 0; i-- {
		cluster := clusters[i]
		if cluster.border == nil {
			continue
		}
		for _, id := range cluster.border.tiles {
			if id != 0 {
				plan.managed[id] = true
			}
		}
		for _, piece := range borderPieces(cluster.align) {
			if id := cluster.border.tiles[piece]; id != 0 {
				putBottom(id)
				continue
			}
			// A missing diagonal is made of its two sides.
			first, second := diagonalSides(piece)
			for _, side := range []int{first, second} {
				if side != edgeNone && cluster.border.tiles[side] != 0 {
					putBottom(cluster.border.tiles[side])
				}
			}
		}
	}
	slices.Reverse(topFirst)
	plan.ids = append(topFirst, plan.ids...)
}

func (e *Editor) applySpecificCases(plan *borderPlan, blocks []*borderBlock) {
	for _, block := range blocks {
		for _, specific := range block.specific {
			plan.ids = e.applySpecific(plan.ids, specific)
			for _, id := range specific.match {
				plan.managed[id] = true
			}
			for _, id := range []uint16{specific.replace, specific.with} {
				if id != 0 {
					plan.managed[id] = true
				}
			}
		}
	}
}

func (e *Editor) applySpecific(ids []uint16, specific specificCase) []uint16 {
	if len(specific.match) == 0 {
		return ids
	}
	matches := 0
	for _, id := range ids {
		if specific.matchGroup > 0 {
			if info := e.Brushes.info[id]; info != nil && info.borderGroup == specific.matchGroup && info.borderAlign == specific.groupAlign {
				matches++
				continue
			}
		}
		for _, matchID := range specific.match {
			if id == matchID {
				matches++
			}
		}
	}
	if matches != len(specific.match) {
		return ids
	}
	if specific.deleteAll {
		return slices.DeleteFunc(ids, func(id uint16) bool { return slices.Contains(specific.match, id) })
	}
	if i := slices.Index(ids, specific.replace); i >= 0 {
		ids[i] = specific.with
	}
	return ids
}

// fitBorders redoes the borders of the tile at pos after its neighborhood
// changed. Unlike RME it does not wipe the tile: it compares the plans
// from before and after the action and swaps only the pieces of the
// border sets they use. Borders it does not know stay, and a tile whose
// plan did not change is left alone.
func (e *Editor) fitBorders(tx *Tx, pos otmap.Position) { e.reborder(tx, pos, false) }

// fitToNeighbors fits the borders, walls, tables and carpets of the tile at
// pos to its neighbors (automagic after a change next to it).
func (e *Editor) fitToNeighbors(tx *Tx, pos otmap.Position) {
	e.fitBorders(tx, pos)
	e.fitWalls(tx, pos)
	e.fitTables(tx, pos)
	e.fitCarpets(tx, pos)
}

// reborder redoes the borders at pos. full replaces every material border
// on the tile; it is used by the borderize commands and when the ground
// of the tile itself changed brush.
func (e *Editor) reborder(tx *Tx, pos otmap.Position, full bool) {
	current := tx.Get(pos)
	self := e.groundBrushOf(current)
	newPlan := e.planBorders(pos, self, current != nil && e.hasOptionalBorder(current), func(neighborPos otmap.Position) *GroundBrush {
		return e.groundBrushOf(e.peekPos(neighborPos))
	})
	if current == nil && len(newPlan.ids) == 0 {
		return
	}
	prior := tx.Prior(pos)
	dropOptional := newPlan.dropOptional && current != nil && current.State&otmap.StateOptionalBorder != 0
	var drop func(uint16) bool
	if full || e.groundBrushOf(prior) != self {
		drop = func(id uint16) bool {
			info := e.Brushes.info[id]
			return info != nil && info.border
		}
	} else {
		oldPlan := e.planBorders(pos, self, prior != nil && e.hasOptionalBorder(prior), func(neighborPos otmap.Position) *GroundBrush {
			return e.groundBrushOf(tx.Prior(neighborPos))
		})
		if !dropOptional && slices.Equal(oldPlan.ids, newPlan.ids) && oldPlan.dropOptional == newPlan.dropOptional {
			return
		}
		drop = func(id uint16) bool {
			return oldPlan.managed[id] || newPlan.managed[id] || slices.Contains(oldPlan.ids, id)
		}
	}
	var currentItems []otmap.Item
	if current != nil {
		currentItems = current.Items
	}
	restacked := e.restack(currentItems, newPlan.ids, drop)
	sameID := func(restackedItem, existing otmap.Item) bool { return restackedItem.ID == existing.ID }
	if current != nil && !dropOptional && slices.EqualFunc(restacked, current.Items, sameID) {
		return
	}
	tile := tx.Edit(pos)
	if tile != current {
		// The restacked items share extras with current, which can be the
		// tile from before the action: the edited tile gets its own copies.
		for i := range restacked {
			restacked[i] = restacked[i].Clone()
		}
	}
	tile.Items = restacked
	if dropOptional {
		tile.State &^= otmap.StateOptionalBorder
	}
}

// restack returns items with the dropped border pieces at the bottom of
// the stack replaced by ids. Kept borders go right above the new ones.
// Pieces already there are reused, keeping their selection.
func (e *Editor) restack(items []otmap.Item, ids []uint16, drop func(uint16) bool) []otmap.Item {
	borderCount := 0
	for borderCount < len(items) && e.Brushes.isBorder(items[borderCount].ID) {
		borderCount++
	}
	var spare, kept []otmap.Item
	for _, item := range items[:borderCount] {
		if drop(item.ID) {
			spare = append(spare, item)
		} else {
			kept = append(kept, item)
		}
	}
	restacked := make([]otmap.Item, 0, len(ids)+len(items))
	for _, id := range ids {
		item := otmap.Item{ID: id}
		if i := slices.IndexFunc(spare, func(spareItem otmap.Item) bool { return spareItem.ID == id }); i >= 0 {
			item = spare[i]
			spare = slices.Delete(spare, i, i+1)
		}
		restacked = append(restacked, item)
	}
	restacked = append(restacked, kept...)
	return append(restacked, items[borderCount:]...)
}
