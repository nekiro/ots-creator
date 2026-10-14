package app

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ResourcePrefix is the URL prefix served by Resources.
const ResourcePrefix = "/res/"

// maxBatch limits sprites per /res/sprites request.
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
	s *Session
	// Maps serves /res/map/ (map chunks and minimaps).
	Maps *MapService
}

// NewResources returns the handler.
func NewResources(s *Session) *Resources { return &Resources{s: s} }

// Middleware routes /res/ requests to the handler and everything else to next.
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
	project := h.s.Project
	if len(parts) > 1 && parts[0] == "b" {
		project, parts = h.s.Other, parts[1:]
	}
	p, err := project()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("X-Sprite-Size", strconv.Itoa(p.SpriteSize()))
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	switch {
	case len(parts) == 2 && parts[0] == "sprite":
		h.sprite(w, p, parts[1])
	case len(parts) == 2 && parts[0] == "spritepng":
		h.spritePNG(w, p, parts[1])
	case len(parts) == 1 && parts[0] == "sprites":
		h.sprites(w, r, p)
	case len(parts) == 3 && parts[0] == "thumb":
		h.texture(w, r, p, parts[1], parts[2], true)
	case len(parts) == 3 && parts[0] == "texture":
		h.texture(w, r, p, parts[1], parts[2], false)
	case len(parts) == 3 && parts[0] == "sheet":
		h.sheet(w, r, p, parts[1], parts[2])
	default:
		http.NotFound(w, r)
	}
}

func parseID(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	return uint32(n), err
}

func (h *Resources) sprite(w http.ResponseWriter, p *project.Project, idStr string) {
	id, err := parseID(idStr)
	if err != nil {
		http.Error(w, "bad sprite id", http.StatusBadRequest)
		return
	}
	px, err := p.SpritePixels(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(px)
}

func (h *Resources) spritePNG(w http.ResponseWriter, p *project.Project, idStr string) {
	id, err := parseID(idStr)
	if err != nil {
		http.Error(w, "bad sprite id", http.StatusBadRequest)
		return
	}
	px, err := p.SpritePixels(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	data, err := imaging.EncodePNG(imagingNRGBA(px, p.SpriteSize()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

func (h *Resources) sprites(w http.ResponseWriter, r *http.Request, p *project.Project) {
	raw := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(raw) > maxBatch {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	size := p.SpriteSize() * p.SpriteSize() * 4
	out := make([]byte, 0, len(raw)*size)
	empty := make([]byte, size)
	for _, s := range raw {
		id, err := parseID(s)
		if err != nil {
			http.Error(w, "bad sprite id "+s, http.StatusBadRequest)
			return
		}
		if id == 0 {
			out = append(out, empty...)
			continue
		}
		px, err := p.SpritePixels(id)
		if err != nil {
			out = append(out, empty...) // missing sprites render empty
			continue
		}
		out = append(out, px...)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(out)
}

func (h *Resources) sheet(w http.ResponseWriter, r *http.Request, p *project.Project, catStr, idStr string) {
	c, err := thing.ParseCategory(catStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := parseID(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	group, _ := strconv.Atoi(r.URL.Query().Get("g"))
	img, err := renderSheet(p, c, id, group, r.URL.Query().Get("bg") == "transparent")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := imaging.EncodePNG(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

func (h *Resources) texture(w http.ResponseWriter, r *http.Request, p *project.Project, catStr, idStr string, thumb bool) {
	c, err := thing.ParseCategory(catStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := parseID(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var pos project.TexturePos
	if thumb {
		t, err := p.Thing(c, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		pos = project.ThumbnailPos(t)
	} else {
		q := r.URL.Query()
		num := func(k string) int { n, _ := strconv.Atoi(q.Get(k)); return max(n, 0) }
		pos = project.TexturePos{Group: thing.FrameGroupType(num("g")), Layer: num("l"), PatternX: num("x"), PatternY: num("y"), PatternZ: num("z"), Frame: num("f")}
		// wrap=1 takes x, y, z as a map position: the pattern the client
		// draws there (grounds and borders vary by tile).
		if q.Get("wrap") == "1" {
			t, err := p.Thing(c, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if g := t.Group(pos.Group); g != nil {
				pos.PatternX %= max(int(g.PatternX), 1)
				pos.PatternY %= max(int(g.PatternY), 1)
				pos.PatternZ %= max(int(g.PatternZ), 1)
			}
		}
	}
	img, err := p.Render(c, id, pos)
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
	data, err := imaging.EncodePNG(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

// appendPerfLog appends the request body (one line of timings from the map
// renderer) to map-perf.log in the user's config directory, where it can be
// read without devtools.
func appendPerfLog(w http.ResponseWriter, r *http.Request) {
	line, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dir = filepath.Join(dir, "OTS Creator")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "map-perf.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), strings.TrimSpace(string(line)))
	w.WriteHeader(http.StatusNoContent)
}
