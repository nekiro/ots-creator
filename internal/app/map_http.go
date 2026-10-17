package app

import (
	"image"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/maprender"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	// maxMinimapSide bounds the area of one minimap request.
	maxMinimapSide = 2048
	// markedItemDefaultColor is the minimap color (yellow) of marked items
	// without one.
	markedItemDefaultColor = 210
)

// mapRequest is a request for one open map, made under the read lock.
type mapRequest struct {
	opened *openMap
	data   *mapData
	query  url.Values
}

// serveMap handles the requests of the map renderer:
//
//	/res/map/{id}/chunks?k=key,key            tiles (maprender.EncodeChunks)
//	/res/map/{id}/geo?k=key,key&o=&h=&b=     quads (maprender.BuildGeometry)
//	/res/map/{id}/minimap?x=&y=&z=&w=&h=     one pixel per tile (PNG)
//
// Requests only read the map: they run side by side (read lock).
func (ms *MapService) serveMap(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	handler, ok := mapHandlers[parts[1]]
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "bad map id", http.StatusBadRequest)
		return
	}
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer unlock()
	w.Header().Set("Cache-Control", "no-store")
	handler(ms, w, mapRequest{opened: opened, data: data, query: r.URL.Query()})
}

// mapHandlers serves the requests of serveMap by their last path part.
var mapHandlers = map[string]func(*MapService, http.ResponseWriter, mapRequest){
	"chunks":  (*MapService).serveChunks,
	"geo":     (*MapService).serveGeometry,
	"minimap": (*MapService).serveMinimap,
}

// scene is what maprender draws the map from.
func (ms *MapService) scene(opened *openMap, data *mapData) *maprender.Scene {
	return &maprender.Scene{
		Map:          opened.editor.Map,
		Items:        opened.editor.Items,
		Things:       data.project,
		SpriteSize:   data.project.SpriteSize(),
		SpritePixels: data.project.SpritePixels,
		IsBlocking:   opened.editor.IsBlocking,
		Outfit: func(creature string) materials.Outfit {
			outfit, _ := data.outfit(creature)
			return outfit
		},
		MarkedItems: markedItemColors(opened.editor),
	}
}

// markedItemColors gives each marked item of the settings its minimap
// color (yellow when it has none).
func markedItemColors(editor *mapedit.Editor) map[uint16][3]uint8 {
	ids := editor.Settings.MarkedItems
	if ids == nil {
		ids = mapedit.DefaultMarkedItems(editor.Items.Mode)
	}
	colors := make(map[uint16][3]uint8, len(ids))
	for _, id := range ids {
		colorIndex := uint8(markedItemDefaultColor)
		if itemType := editor.Items.Get(id); itemType != nil && itemType.MinimapColor != 0 {
			colorIndex = uint8(itemType.MinimapColor)
		}
		red, green, blue := mapedit.MinimapRGB(colorIndex)
		colors[id] = [3]uint8{red, green, blue}
	}
	return colors
}

// chunkKeys parses the k parameter: chunk keys separated by commas,
// leaving out what is no key. ok is false for too many keys.
func chunkKeys(query url.Values) (keys []otmap.ChunkKey, ok bool) {
	list := strings.Split(query.Get("k"), ",")
	if len(list) > maprender.MaxChunksPerRequest {
		return nil, false
	}
	for _, text := range list {
		if key, err := strconv.ParseUint(text, 10, 32); err == nil {
			keys = append(keys, otmap.ChunkKey(key))
		}
	}
	return keys, true
}

// queryInt reads an int parameter, 0 when missing or invalid.
func queryInt(query url.Values, name string) int {
	value, _ := strconv.Atoi(query.Get(name))
	return value
}

func (ms *MapService) serveChunks(w http.ResponseWriter, request mapRequest) {
	keys, ok := chunkKeys(request.query)
	if !ok {
		http.Error(w, "too many chunks", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(maprender.EncodeChunks(ms.scene(request.opened, request.data), keys))
}

func (ms *MapService) serveGeometry(w http.ResponseWriter, request mapRequest) {
	keys, ok := chunkKeys(request.query)
	if !ok {
		http.Error(w, "too many chunks", http.StatusBadRequest)
		return
	}
	options := maprender.GeometryOptions{
		ViewFlags: uint32(queryInt(request.query, "o")),
		House:     uint32(queryInt(request.query, "h")),
		Block:     min(max(queryInt(request.query, "b"), 1), maprender.MaxBlock),
	}
	start := time.Now()
	data, rawSize, err := maprender.BuildGeometry(ms.scene(request.opened, request.data), keys, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Raw deflate: the page inflates it (DecompressionStream "deflate-raw").
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Build-Ms", strconv.FormatFloat(float64(time.Since(start).Microseconds())/1000, 'f', 2, 64))
	w.Header().Set("X-Raw-Bytes", strconv.Itoa(rawSize))
	w.Header().Set("X-Packed-Bytes", strconv.Itoa(len(data)))
	w.Write(data)
}

func (ms *MapService) serveMinimap(w http.ResponseWriter, request mapRequest) {
	query := request.query
	x, y, floor := queryInt(query, "x"), queryInt(query, "y"), queryInt(query, "z")
	width, height := queryInt(query, "w"), queryInt(query, "h")
	if width <= 0 || height <= 0 || width > maxMinimapSide || height > maxMinimapSide || floor < 0 || floor > otmap.MaxFloor {
		http.Error(w, "bad area", http.StatusBadRequest)
		return
	}
	data, err := imaging.EncodePNG(minimapImage(request.opened.editor, image.Rect(x, y, x+width, y+height), floor))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}
