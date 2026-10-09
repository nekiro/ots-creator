package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestCompareService(t *testing.T) {
	dir := t.TempDir()
	s, ps, ts, ss, rec := newEnv(t)
	cs := NewCompareService(s)
	if _, err := cs.Diff(thing.CategoryItem); !errors.Is(err, ErrNoProject) {
		t.Fatalf("err = %v", err)
	}

	// B: a saved client with one red item.
	ps.New(v1098(), client.Features{})
	img := filepath.Join(dir, "red.png")
	writePNG(t, img, 32, 32, [4]byte{200, 10, 10, 255})
	if _, err := ss.ImportImages([]string{img}); err != nil {
		t.Fatal(err)
	}
	it, _ := ts.Get(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = 1
	if err := ts.Update(it); err != nil {
		t.Fatal(err)
	}
	datPath, sprPath := filepath.Join(dir, "Tibia.dat"), filepath.Join(dir, "Tibia.spr")
	if err := ps.CompileAs(CompileRequest{DatPath: datPath, SprPath: sprPath, Version: v1098(), Features: ps.State().Info.Features}); err != nil {
		t.Fatal(err)
	}
	v := v1098()
	st, err := cs.Open(OpenRequest{DatPath: datPath, SprPath: sprPath, Version: &v})
	if err != nil || !st.Open {
		t.Fatalf("open %+v %v", st, err)
	}
	if rec.events[len(rec.events)-1] != EventOtherChanged {
		t.Fatalf("events %v", rec.events)
	}

	// A: a new empty client.
	ps.New(v1098(), client.Features{})
	d, err := cs.Diff(thing.CategoryItem)
	if err != nil || len(d.Entries) != 1 || d.Entries[0].Changes[0] != project.ChangeSprites {
		t.Fatalf("diff %+v %v", d, err)
	}

	rr := httptest.NewRecorder()
	NewResources(s).ServeHTTP(rr, httptest.NewRequest("GET", "/res/b/sprite/1", nil))
	if rr.Code != http.StatusOK || rr.Body.Len() != 32*32*4 || rr.Body.Bytes()[0] != 200 {
		t.Fatalf("b sprite: %d %d", rr.Code, rr.Body.Len())
	}

	// B -> A appends, A -> B replaces.
	res, err := cs.Transfer(false, thing.CategoryItem, []uint32{100}, true)
	if err != nil || !slices.Equal(res.IDs, []uint32{101}) {
		t.Fatalf("transfer %+v %v", res, err)
	}
	if _, err := cs.Transfer(true, thing.CategoryItem, []uint32{101}, false); err != nil {
		t.Fatal(err)
	}
	if !cs.State().Info.Changed || cs.State().Info.Counts.Items != 101 {
		t.Fatalf("b %+v", cs.State().Info)
	}
	if !BlockClose(NewWindowService(s, func() {}))() {
		t.Fatal("unsaved second client must block closing")
	}
	if err := cs.Compile(); err != nil || cs.State().Info.Changed {
		t.Fatalf("compile b: %v", err)
	}
	if label, _ := cs.Undo(); label != "" {
		t.Fatalf("history kept after compile: %q", label)
	}
	cs.Close()
	if cs.State().Open {
		t.Fatal("still open")
	}
}
