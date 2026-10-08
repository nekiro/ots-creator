package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

func v1098() client.Version {
	v, _ := client.FindBySignatures(0x42A3, 0x57BBD603)
	return v
}

type recorded struct {
	events []string
}

func newEnv(t *testing.T) (*Session, *ProjectService, *ThingService, *SpriteService, *recorded) {
	t.Helper()
	rec := &recorded{}
	s := NewSession(func(name string, _ any) { rec.events = append(rec.events, name) })
	return s, NewProjectService(s), NewThingService(s), NewSpriteService(s), rec
}

func writePNG(t *testing.T, path string, w, h int, fill [4]byte) {
	t.Helper()
	m := newImage(w, h, fill)
	data, err := imaging.EncodePNG(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newImage(w, h int, fill [4]byte) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(m.Pix, bytes.Repeat(fill[:], w*h))
	return m
}

func TestNoProject(t *testing.T) {
	s, ps, ts, _, _ := newEnv(t)
	if _, err := ts.Get(thing.CategoryItem, 100); !errors.Is(err, ErrNoProject) {
		t.Fatalf("err = %v", err)
	}
	if ps.State().Open {
		t.Fatal("state open")
	}
	rr := httptest.NewRecorder()
	NewResources(s).ServeHTTP(rr, httptest.NewRequest("GET", "/res/sprite/1", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	s, ps, ts, ss, rec := newEnv(t)
	st := ps.New(v1098(), client.Features{})
	if !st.Open || st.Rev == 0 || len(rec.events) != 1 {
		t.Fatalf("state %+v events %v", st, rec.events)
	}

	img := filepath.Join(dir, "tiles.png")
	writePNG(t, img, 64, 32, [4]byte{200, 10, 10, 255})
	ids, err := ss.ImportImages([]string{img})
	if err != nil || len(ids) != 2 {
		t.Fatalf("import %v %v", ids, err)
	}

	it, _ := ts.Get(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = ids[1]
	if err := ts.Update(it); err != nil {
		t.Fatal(err)
	}

	// Resources: raw sprite, batch and thumbnail.
	h := NewResources(s).Middleware(http.NotFoundHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/sprite/2?r=1", nil))
	if rr.Code != 200 || rr.Body.Len() != 32*32*4 || rr.Body.Bytes()[0] != 200 || rr.Header().Get("X-Sprite-Size") != "32" {
		t.Fatalf("sprite %d len %d", rr.Code, rr.Body.Len())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/sprites?ids=1,0,2", nil))
	if rr.Body.Len() != 3*32*32*4 || rr.Body.Bytes()[32*32*4+3] != 0 {
		t.Fatalf("batch len %d", rr.Body.Len())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/spritepng/1", nil))
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("spritepng %d", rr.Code)
	}
	if len(ps.State().Info.Supported) < 40 {
		t.Fatalf("supported %v", ps.State().Info.Supported)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/thumb/item/100", nil))
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("thumb %d %s", rr.Code, rr.Body.String())
	}
	if pimg, err := png.Decode(bytes.NewReader(rr.Body.Bytes())); err != nil || pimg.Bounds().Dx() != 32 {
		t.Fatal("thumb png")
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/texture/item/100?l=5", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad texture pos code %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/sheet/item/100?g=0&bg=transparent", nil))
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("sheet %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/res/sheet/item/100?g=3", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad sheet group code %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/index.html", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatal("non-resource must go to next handler")
	}

	// OBD export + import.
	paths, err := ts.ExportOBD(thing.CategoryItem, []uint32{100}, dir)
	if err != nil || len(paths) != 1 {
		t.Fatalf("export %v %v", paths, err)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if d, err := obd.Decode(raw); err != nil || d.Version != obd.Version3 {
		t.Fatalf("export must write the newest OBD version: %v", err)
	}
	view, err := ts.ReadOBD(paths[0])
	if err != nil || view.Version != obd.Version3 || view.Thing.Category != thing.CategoryItem || len(view.Sprites) != len(view.Thing.SpriteIDs()) {
		t.Fatalf("read obd %+v %v", view, err)
	}
	if ids := view.Thing.SpriteIDs(); ids[0] != 1 || len(view.Sprites[0]) != 32*32*4 {
		t.Fatalf("read obd sprites %v", ids)
	}
	res, err := ts.ImportOBD(paths, 0)
	if err != nil || res[0].Error != "" || res[0].ID != 101 {
		t.Fatalf("import %+v %v", res, err)
	}

	// Sheet export + import.
	sheet := filepath.Join(dir, "sheet.png")
	if err := ts.ExportSheet(thing.CategoryItem, 100, 0, sheet, false); err != nil {
		t.Fatal(err)
	}
	if err := ts.ImportSheet(thing.CategoryItem, 101, 0, sheet); err != nil {
		t.Fatal(err)
	}

	// Compile, then reopen via Inspect.
	datPath, sprPath := filepath.Join(dir, "Tibia.dat"), filepath.Join(dir, "Tibia.spr")
	if err := ps.Compile(); err == nil {
		t.Fatal("compile without paths must fail")
	}
	if err := ps.CompileAs(CompileRequest{DatPath: datPath, SprPath: sprPath, Version: v1098(), Features: ps.State().Info.Features, WriteOTFI: true}); err != nil {
		t.Fatal(err)
	}
	cf, err := ps.Inspect(filepath.Join(dir, "Tibia.otfi"))
	if err != nil || !cf.HasOTFI || cf.Detected == nil || cf.Detected.Value != 1098 {
		t.Fatalf("inspect %+v %v", cf, err)
	}
	if byDir, err := ps.Inspect(dir); err != nil || byDir.DatPath != cf.DatPath || !byDir.HasOTFI {
		t.Fatalf("inspect dir %+v %v", byDir, err)
	}
	if _, err := ps.Inspect(t.TempDir()); err == nil {
		t.Fatal("inspect of an empty dir must fail")
	}
	st, err = ps.Open(OpenRequest{DatPath: cf.DatPath, SprPath: cf.SprPath, Features: cf.Features})
	if err != nil || st.Info.Counts.Items != 101 {
		t.Fatalf("open %+v %v", st.Info.Counts, err)
	}

	exported, err := ss.Export([]uint32{1}, dir, "bmp")
	if err != nil || len(exported) != 1 || filepath.Ext(exported[0]) != ".bmp" {
		t.Fatalf("sprite export %v %v", exported, err)
	}
	ps.Close()
	if ps.State().Open {
		t.Fatal("close")
	}
}

func TestUndoRedoService(t *testing.T) {
	_, ps, ts, _, _ := newEnv(t)
	ps.New(v1098(), client.Features{})
	ts.Add(thing.CategoryEffect)
	label, err := ps.Undo()
	if err != nil || label == "" {
		t.Fatalf("undo %q %v", label, err)
	}
	if ps.State().Info.Counts.Effects != 1 {
		t.Fatal("undo count")
	}
	if label, _ := ps.Redo(); label == "" || ps.State().Info.Counts.Effects != 2 {
		t.Fatal("redo")
	}
	if label, _ := ps.Redo(); label != "" {
		t.Fatal("nothing to redo")
	}
}
