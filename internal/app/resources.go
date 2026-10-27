package app

import (
	"errors"
	"image"
	"net/http"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

const ResourcePrefix = "/res/"

const maxBatch = 4096

// Resources serves binary data to the webview without JSON/base64:
//
//	/res/sprite/{id}             raw RGBA, size*size*4 bytes
//	/res/sprites?ids=1,2,3       raw RGBA of several sprites, concatenated
//	/res/spritepng/{id}          PNG of one sprite
//	/res/thumb/{cat}/{id}        PNG of the list thumbnail
//	/res/texture/{cat}/{id}?g=&l=&x=&y=&z=&f=   PNG of one texture
//	/res/sheet/{cat}/{id}?g=&bg=transparent      PNG of a frame group sprite sheet
//	/res/map/{map}/chunks?k=…                    map chunks (see MapService.serveMap)
//	/res/map/{map}/minimap?x=&y=&z=&w=&h=        PNG minimap of an area
//
// The same paths under /res/b/ serve the second client of a comparison.
//
// The response header X-Sprite-Size carries the sprite edge length. URLs
// should include ?r={rev} so cached responses are invalidated on change.
type Resources struct {
	session *Session
	Maps    *MapService
}

func NewResources(session *Session) *Resources { return &Resources{session: session} }

func (h *Resources) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, ResourcePrefix) {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Resources) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, ResourcePrefix), "/")
	if len(parts) == 1 && parts[0] == "perflog" && r.Method == http.MethodPost {
		appendPerfLog(w, r)
		return
	}
	if len(parts) > 0 && parts[0] == "map" && h.Maps != nil {
		h.Maps.serveMap(w, r, parts[1:])
		return
	}
	client := h.session.Project
	if len(parts) > 1 && parts[0] == "b" {
		client, parts = h.session.Other, parts[1:]
	}
	proj, err := client()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("X-Sprite-Size", strconv.Itoa(proj.SpriteSize()))
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	switch {
	case len(parts) == 2 && parts[0] == "sprite":
		h.sprite(w, proj, parts[1])
	case len(parts) == 2 && parts[0] == "spritepng":
		h.spritePNG(w, proj, parts[1])
	case len(parts) == 1 && parts[0] == "sprites":
		h.sprites(w, r, proj)
	case len(parts) == 3 && parts[0] == "thumb":
		h.texture(w, r, proj, parts[1], parts[2], true)
	case len(parts) == 3 && parts[0] == "texture":
		h.texture(w, r, proj, parts[1], parts[2], false)
	case len(parts) == 3 && parts[0] == "sheet":
		h.sheet(w, r, proj, parts[1], parts[2])
	default:
		http.NotFound(w, r)
	}
}

func parseID(text string) (uint32, error) {
	id, err := strconv.ParseUint(text, 10, 32)
	return uint32(id), err
}

func parseThing(w http.ResponseWriter, categoryText, idText string) (category thing.Category, id uint32, ok bool) {
	category, err := thing.ParseCategory(categoryText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return 0, 0, false
	}
	if id, err = parseID(idText); err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return 0, 0, false
	}
	return category, id, true
}

func spritePixels(w http.ResponseWriter, proj *project.Project, idText string) (pixels []byte, ok bool) {
	id, err := parseID(idText)
	if err != nil {
		http.Error(w, "bad sprite id", http.StatusBadRequest)
		return nil, false
	}
	pixels, err = proj.SpritePixels(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil, false
	}
	return pixels, true
}

func servePNG(w http.ResponseWriter, img image.Image) {
	data, err := imaging.EncodePNG(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

func (h *Resources) sprite(w http.ResponseWriter, proj *project.Project, idText string) {
	if pixels, ok := spritePixels(w, proj, idText); ok {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(pixels)
	}
}

func (h *Resources) spritePNG(w http.ResponseWriter, proj *project.Project, idText string) {
	if pixels, ok := spritePixels(w, proj, idText); ok {
		servePNG(w, spriteImage(pixels, proj.SpriteSize()))
	}
}

func (h *Resources) sprites(w http.ResponseWriter, r *http.Request, proj *project.Project) {
	idTexts := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(idTexts) > maxBatch {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	spriteBytes := proj.SpriteSize() * proj.SpriteSize() * 4
	out := make([]byte, 0, len(idTexts)*spriteBytes)
	empty := make([]byte, spriteBytes)
	for _, idText := range idTexts {
		id, err := parseID(idText)
		if err != nil {
			http.Error(w, "bad sprite id "+idText, http.StatusBadRequest)
			return
		}
		if id == 0 {
			out = append(out, empty...)
			continue
		}
		pixels, err := proj.SpritePixels(id)
		if err != nil {
			out = append(out, empty...) // missing sprites render empty
			continue
		}
		out = append(out, pixels...)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(out)
}

func (h *Resources) sheet(w http.ResponseWriter, r *http.Request, proj *project.Project, categoryText, idText string) {
	category, id, ok := parseThing(w, categoryText, idText)
	if !ok {
		return
	}
	group := queryInt(r.URL.Query(), "g")
	img, err := renderSheet(proj, category, id, group, r.URL.Query().Get("bg") == "transparent")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	servePNG(w, img)
}

func (h *Resources) texture(w http.ResponseWriter, r *http.Request, proj *project.Project, categoryText, idText string, thumb bool) {
	category, id, ok := parseThing(w, categoryText, idText)
	if !ok {
		return
	}
	var pos project.TexturePos
	if thumb {
		found, err := proj.Thing(category, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		pos = project.ThumbnailPos(found)
	} else if pos, ok = texturePos(w, r, proj, category, id); !ok {
		return
	}
	img, err := proj.Render(category, id, pos)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrNoProject) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	if thumb {
		// Lists fit the visible pixels into the slot, so a small creature in
		// a 64x64 texture is not drawn tiny in a corner.
		img = imaging.CropToContent(img)
	}
	servePNG(w, img)
}

// texturePos reads the texture of a /res/texture request. wrap=1 takes
// x, y, z as a map position: the pattern the client draws there (grounds
// and borders vary by tile).
func texturePos(w http.ResponseWriter, r *http.Request, proj *project.Project, category thing.Category, id uint32) (project.TexturePos, bool) {
	query := r.URL.Query()
	param := func(name string) int { return max(queryInt(query, name), 0) }
	pos := project.TexturePos{
		Group: thing.FrameGroupType(param("g")), Layer: param("l"),
		PatternX: param("x"), PatternY: param("y"), PatternZ: param("z"), Frame: param("f"),
	}
	if query.Get("wrap") != "1" {
		return pos, true
	}
	found, err := proj.Thing(category, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return pos, false
	}
	if group := found.Group(pos.Group); group != nil {
		pos.PatternX %= max(int(group.PatternX), 1)
		pos.PatternY %= max(int(group.PatternY), 1)
		pos.PatternZ %= max(int(group.PatternZ), 1)
	}
	return pos, true
}
