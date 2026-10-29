package mcpserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
)

// Characters of terrain_grid besides the grounds.
const (
	terrainEmpty    = ' '
	terrainNoGround = '_'
	terrainBlocking = '#'
	terrainCreature = '@'
	terrainNPC      = '&'
	terrainSpawn    = '+'
)

// terrainReserved are the characters grounds never get.
const terrainReserved = " _#@&+"

// groundSymbols picks the characters left for grounds, in order of
// preference: letters of the name, then any letter or digit.
var groundSymbols = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789%=:;!?$^<>/\\|()[]{}")

type terrainArgs struct {
	areaArgs
	Overlay *bool `json:"overlay,omitempty" jsonschema:"mark blocking items (#), monsters (@), NPCs (&) and spawn centers (+) over the ground (default true)"`
}

func (s *Server) addTerrainTools(server *mcp.Server) {
	addTool(server, "terrain_grid", "A text map of an area, one character per tile: the ground (brush or ground item, see the legend), blocking items, creatures and spawns. Row and column labels give exact coordinates: use it to place things tile by tile, e.g. coastlines.",
		func(_ context.Context, args terrainArgs) (any, error) {
			id, err := s.mapID(args.MapID)
			if err != nil {
				return nil, err
			}
			area, err := args.area()
			if err != nil {
				return nil, err
			}
			cells, err := s.maps.AreaTerrain(id, area, args.Z)
			if err != nil {
				return nil, err
			}
			return textResult(terrainText(cells, args.X, args.Y, area.Dx(), args.Z, args.Overlay == nil || *args.Overlay)), nil
		})
}

// terrainText writes terrain cells as rows of characters with x and y
// labels and a legend.
func terrainText(cells []app.TerrainCell, originX, originY, width, floor int, overlay bool) string {
	symbols := map[string]rune{}
	used := map[rune]bool{}
	var legend []string
	symbolOf := func(ground string) rune {
		if symbol, ok := symbols[ground]; ok {
			return symbol
		}
		symbol := pickSymbol(ground, used)
		symbols[ground], used[symbol] = symbol, true
		legend = append(legend, fmt.Sprintf("%c=%s", symbol, ground))
		return symbol
	}
	height := len(cells) / width
	labelWidth := len(strconv.Itoa(originY + height - 1))
	var text strings.Builder
	fmt.Fprintf(&text, "Floor %d. Column n of a row is x %d+n; each row starts with its y.\n", floor, originX)
	// Two ruler lines: x at every multiple of 10, then the last digit.
	tens := []rune(strings.Repeat(" ", width+8))
	for column := range width {
		if x := originX + column; x%10 == 0 {
			copy(tens[column:], []rune(strconv.Itoa(x)))
		}
	}
	fmt.Fprintf(&text, "%*s %s\n", labelWidth, "", strings.TrimRight(string(tens), " "))
	digits := make([]byte, width)
	for column := range width {
		digits[column] = byte('0' + (originX+column)%10)
	}
	fmt.Fprintf(&text, "%*s %s\n", labelWidth, "", digits)
	row := make([]rune, width)
	for index, cell := range cells {
		column := index % width
		switch {
		case !cell.Exists:
			row[column] = terrainEmpty
		case overlay && cell.Creature && cell.NPC:
			row[column] = terrainNPC
		case overlay && cell.Creature:
			row[column] = terrainCreature
		case overlay && cell.Spawn:
			row[column] = terrainSpawn
		case overlay && cell.Blocking:
			row[column] = terrainBlocking
		case cell.Ground == "":
			row[column] = terrainNoGround
		default:
			row[column] = symbolOf(cell.Ground)
		}
		if column == width-1 {
			fmt.Fprintf(&text, "%*d %s\n", labelWidth, originY+index/width, string(row))
		}
	}
	text.WriteString("Legend: ' '=no tile, _=no ground")
	if overlay {
		text.WriteString(", #=blocking item, @=monster, &=NPC, +=spawn center")
	}
	for _, entry := range legend {
		text.WriteString(", " + entry)
	}
	text.WriteByte('\n')
	return text.String()
}

// pickSymbol gives a ground a free character: ~ for water, else a letter of
// its name, else any free one.
func pickSymbol(ground string, used map[rune]bool) rune {
	lower := strings.ToLower(ground)
	if (strings.Contains(lower, "sea") || strings.Contains(lower, "water")) && !used['~'] {
		return '~'
	}
	for _, letter := range lower {
		if unicode.IsLetter(letter) && letter < unicode.MaxASCII && !used[letter] && !strings.ContainsRune(terrainReserved, letter) {
			return letter
		}
	}
	for _, symbol := range groundSymbols {
		if !used[symbol] {
			return symbol
		}
	}
	return '?'
}
