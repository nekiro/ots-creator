package materials

import (
	"slices"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

func idRange(node *xmltree.Node) (uint32, uint32) {
	if node.Has("id") {
		id := uint32(node.U16("id"))
		return id, id
	}
	fromID, toID := uint32(node.U16("fromid")), uint32(node.U16("toid"))
	if toID < fromID {
		toID = fromID
	}
	return fromID, toID
}

func borderNode(node *xmltree.Node) *Border {
	border := &Border{ID: uint32(node.Int("id")), Group: node.U16("group"), Edges: map[string]uint16{}}
	if strings.EqualFold(node.Str("type"), "optional") {
		border.Optional = true
	}
	for _, child := range node.Children {
		edge := strings.ToLower(child.Str("edge"))
		id := child.U16("item")
		if id != 0 && slices.Contains(EdgeNames, edge) {
			border.Edges[edge] = id
		}
	}
	return border
}

func (im *importer) look(node *xmltree.Node) uint16 {
	if node.Has("server_lookid") {
		return node.U16("server_lookid")
	}
	if node.Has("lookid") {
		id := node.U16("lookid")
		if im.toServer != nil {
			if serverID, ok := im.toServer(id); ok {
				return serverID
			}
		}
		return id
	}
	return 0
}

func itemSpec(node *xmltree.Node) ItemSpec {
	spec := ItemSpec{ID: node.U16("id")}
	for _, attr := range []string{"subtype", "count"} {
		if node.Has(attr) {
			spec.Subtype = node.U16(attr)
		}
	}
	for _, attr := range []string{"aid", "actionid"} {
		if node.Has(attr) {
			spec.ActionID = node.U16(attr)
		}
	}
	for _, attr := range []string{"uid", "uniqueid"} {
		if node.Has(attr) {
			spec.UniqueID = node.U16(attr)
		}
	}
	spec.Text = node.Str("text")
	return spec
}

func (im *importer) brush(node *xmltree.Node) {
	name := node.Str("name")
	if name == "" {
		im.warn("brush without a name")
		return
	}
	brushType := strings.ToLower(node.Str("type"))
	switch brushType {
	case "wall decoration":
		brushType = TypeDecoration
	case "ground", "wall", "doodad", "carpet", "table", "":
	default:
		im.warn("brush %q has unknown type %q", name, brushType)
		return
	}
	brush := im.materials.Brush(name)
	if brush == nil {
		brush = &Brush{Name: name}
		im.materials.Brushes = append(im.materials.Brushes, brush)
	}
	if brushType != "" {
		brush.Type = brushType
	}
	if look := im.look(node); look != 0 {
		brush.Look = look
	}
	if len(node.Children) == 0 {
		return
	}
	switch brush.Type {
	case TypeGround:
		im.ground(brush, node)
	case TypeWall, TypeDecoration:
		im.wall(brush, node)
	case TypeDoodad:
		im.doodad(brush, node)
	case TypeCarpet, TypeTable:
		im.parts(brush, node)
	}
}

func (im *importer) ground(brush *Brush, node *xmltree.Node) {
	if node.Has("z-order") {
		brush.ZOrder = node.Int("z-order")
	}
	brush.SoloOptional = node.Bool("solo_optional")
	if node.Has("randomize") && !node.Bool("randomize") {
		brush.NoRandomize = true
	}
	for _, child := range node.Children {
		switch child.Name {
		case "item":
			brush.Items = append(brush.Items, Chance{ID: child.U16("id"), Chance: child.Int("chance")})
		case "optional":
			rule := &BorderRule{Align: "outer"}
			if child.Has("ground_equivalent") {
				rule.Inline = inlineBorder(child)
			} else {
				rule.Border = uint32(child.Int("id"))
			}
			brush.Optional = rule
		case "border":
			rule := BorderRule{Align: "outer", To: "all"}
			if strings.EqualFold(child.Str("align"), "inner") {
				rule.Align = "inner"
			}
			if child.Has("to") {
				rule.To = child.Str("to")
			}
			rule.Super = child.Bool("super")
			if child.Has("id") {
				rule.Border = uint32(child.Int("id"))
			} else if child.Has("ground_equivalent") {
				rule.Inline = inlineBorder(child)
			}
			for _, specificNode := range child.Children {
				if specificNode.Name == "specific" {
					rule.Specific = append(rule.Specific, specific(specificNode))
				}
			}
			brush.Borders = append(brush.Borders, rule)
		case "friend":
			brush.Friends = append(brush.Friends, child.Str("name"))
			brush.HateFriends = false
		case "enemy":
			brush.Friends = append(brush.Friends, child.Str("name"))
			brush.HateFriends = true
		case "clear_borders":
			brush.Borders = nil
		case "clear_friends":
			brush.Friends = nil
			brush.HateFriends = false
		}
	}
}

func specific(node *xmltree.Node) SpecificCase {
	var result SpecificCase
	for _, part := range node.Children {
		for _, child := range part.Children {
			switch part.Name {
			case "conditions":
				switch child.Name {
				case "match_border":
					result.Conditions = append(result.Conditions, Condition{Kind: "border", Border: uint32(child.Int("id")), Edge: strings.ToLower(child.Str("edge"))})
				case "match_group":
					result.Conditions = append(result.Conditions, Condition{Kind: "group", Group: child.U16("group"), Edge: strings.ToLower(child.Str("edge"))})
				case "match_item":
					result.Conditions = append(result.Conditions, Condition{Kind: "item", Item: child.U16("id")})
				}
			case "actions":
				switch child.Name {
				case "replace_border":
					result.Actions = append(result.Actions, Action{Kind: "replaceBorder", Border: uint32(child.Int("id")), Edge: strings.ToLower(child.Str("edge")), With: child.U16("with")})
				case "replace_item":
					result.Actions = append(result.Actions, Action{Kind: "replaceItem", Item: child.U16("id"), With: child.U16("with")})
				case "delete_borders":
					result.Actions = append(result.Actions, Action{Kind: "deleteBorders"})
				}
			}
		}
	}
	return result
}

func (im *importer) wall(brush *Brush, node *xmltree.Node) {
	if brush.Walls == nil {
		brush.Walls = map[string]*WallPart{}
	}
	for _, child := range node.Children {
		switch child.Name {
		case "wall":
			align := child.Str("type")
			if !slices.Contains(WallAlignments, align) {
				align = wallAlias(align)
				if align == "" {
					im.warn("wall brush %q: unknown alignment %q", brush.Name, child.Str("type"))
					continue
				}
			}
			part := brush.Walls[align]
			if part == nil {
				part = &WallPart{}
				brush.Walls[align] = part
			}
			for _, itemNode := range child.Children {
				switch itemNode.Name {
				case "item":
					part.Items = append(part.Items, Chance{ID: itemNode.U16("id"), Chance: itemNode.Int("chance")})
				case "door":
					doorType := strings.ReplaceAll(strings.ToLower(itemNode.Str("type")), "_", " ")
					open := true
					if itemNode.Has("open") {
						open = itemNode.Bool("open")
					}
					door := Door{ID: itemNode.U16("id"), Open: open, Hate: itemNode.Bool("hate")}
					switch doorType {
					case "any door":
						for _, expanded := range []string{"archway", "normal", "locked", "quest", "magic"} {
							door.Type = expanded
							part.Doors = append(part.Doors, door)
						}
					case "any window":
						for _, expanded := range []string{"window", "hatch window"} {
							door.Type = expanded
							part.Doors = append(part.Doors, door)
						}
					case "any":
						for _, expanded := range DoorTypes {
							door.Type = expanded
							part.Doors = append(part.Doors, door)
						}
					default:
						if !slices.Contains(DoorTypes, doorType) {
							im.warn("wall brush %q: unknown door type %q", brush.Name, doorType)
							continue
						}
						door.Type = doorType
						part.Doors = append(part.Doors, door)
					}
				}
			}
		case "friend":
			brush.Friends = append(brush.Friends, child.Str("name"))
			if child.Bool("redirect") {
				brush.Redirect = child.Str("name")
			}
		case "alternate", "composite", "item":
			// Some wall brushes are placed like doodads.
			im.doodad(brush, node)
			return
		}
	}
}

func wallAlias(name string) string {
	switch strings.ToLower(name) {
	case "northwest diagonal":
		return "corner"
	case "south t":
		return "south T"
	case "east t":
		return "east T"
	case "west t":
		return "west T"
	case "north t":
		return "north T"
	}
	for _, alignment := range WallAlignments {
		if strings.EqualFold(alignment, name) {
			return alignment
		}
	}
	return ""
}

func (im *importer) doodad(brush *Brush, node *xmltree.Node) {
	options := brush.Doodad
	if options == nil {
		options = &DoodadOptions{}
		brush.Doodad = options
	}
	options.OnBlocking = node.Bool("on_blocking")
	options.OnDuplicate = node.Bool("on_duplicate")
	options.RedoBorders = node.Bool("redo_borders") || node.Bool("reborder")
	options.OneSize = node.Bool("one_size")
	options.Draggable = node.Bool("draggable")
	options.RemoveOptionalBorder = node.Bool("remove_optional_border")
	if thickness := node.Str("thickness"); thickness != "" {
		if amount, ceiling, ok := strings.Cut(thickness, "/"); ok {
			options.Thickness, _ = strconv.Atoi(strings.TrimSpace(amount))
			options.ThicknessCeiling, _ = strconv.Atoi(strings.TrimSpace(ceiling))
		}
	}
	var top Alternative
	for _, child := range node.Children {
		if child.Name == "alternate" {
			brush.Alternatives = append(brush.Alternatives, alternative(child))
		}
	}
	top = alternative(node)
	if len(top.Singles) > 0 || len(top.Composites) > 0 {
		if len(brush.Alternatives) == 0 {
			brush.Alternatives = append(brush.Alternatives, top)
		} else {
			last := &brush.Alternatives[len(brush.Alternatives)-1]
			last.Singles = append(last.Singles, top.Singles...)
			last.Composites = append(last.Composites, top.Composites...)
		}
	}
}

func alternative(node *xmltree.Node) Alternative {
	var result Alternative
	for _, child := range node.Children {
		switch child.Name {
		case "item":
			result.Singles = append(result.Singles, Single{Chance: child.Int("chance"), Item: itemSpec(child)})
		case "composite":
			composite := Composite{Chance: child.Int("chance")}
			for _, tileNode := range child.Children {
				if tileNode.Name != "tile" {
					continue
				}
				compositeTile := CompositeTile{X: tileNode.Int("x"), Y: tileNode.Int("y"), Z: tileNode.Int("z")}
				for _, itemNode := range tileNode.Children {
					if itemNode.Name == "item" {
						compositeTile.Items = append(compositeTile.Items, itemSpec(itemNode))
					}
				}
				if len(compositeTile.Items) > 0 {
					composite.Tiles = append(composite.Tiles, compositeTile)
				}
			}
			if len(composite.Tiles) > 0 {
				result.Composites = append(result.Composites, composite)
			}
		}
	}
	return result
}

// inlineBorder reads a border defined inside a ground brush: it has no id
// and is keyed by the ground item it stands for.
func inlineBorder(node *xmltree.Node) *Border {
	inline := borderNode(node)
	inline.ID = 0
	inline.GroundEquivalent = node.U16("ground_equivalent")
	return inline
}

func (im *importer) parts(brush *Brush, node *xmltree.Node) {
	if brush.Parts == nil {
		brush.Parts = map[string][]Chance{}
	}
	for _, child := range node.Children {
		if child.Name != "carpet" && child.Name != "table" {
			continue
		}
		align := strings.ToLower(child.Str("align"))
		if brush.Type == TypeCarpet {
			if align == "center" {
				align = "c"
			}
			if align != "c" && !slices.Contains(EdgeNames, align) {
				im.warn("carpet %q: unknown alignment %q", brush.Name, align)
				continue
			}
		} else if !slices.Contains(TableAlignments, align) {
			im.warn("table %q: unknown alignment %q", brush.Name, align)
			continue
		}
		if child.Has("id") {
			brush.Parts[align] = append(brush.Parts[align], Chance{ID: child.U16("id"), Chance: max(child.Int("chance"), 1)})
		}
		for _, itemNode := range child.Children {
			if itemNode.Name == "item" {
				brush.Parts[align] = append(brush.Parts[align], Chance{ID: itemNode.U16("id"), Chance: itemNode.Int("chance")})
			}
		}
	}
}
