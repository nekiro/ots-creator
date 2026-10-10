package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func version(t *testing.T, value uint16) client.Version {
	t.Helper()
	for _, v := range client.Versions() {
		if v.Value == value {
			return v
		}
	}
	t.Fatalf("no version %d", value)
	return client.Version{}
}

// setItem gives item id the sprites and pickupable flag, adding the item
// (and empty ones before it) when needed.
func setItem(t *testing.T, p *Project, id uint32, pickup bool, sprites ...uint32) {
	t.Helper()
	for p.Info().Counts.Items < id {
		if _, err := p.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	it, _ := p.Thing(thing.CategoryItem, id)
	it.Props.Pickupable = pickup
	g := it.FrameGroups[0]
	g.Width, g.Frames = uint8(len(sprites)), 1
	g.Sprites = sprites
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	a.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0)})
	// Same pixels under other ids in b.
	b.AddSprites([][]byte{solid(0, 255, 0), solid(255, 0, 0), solid(0, 0, 255)})
	setItem(t, a, 100, false, 1)
	setItem(t, b, 100, false, 2) // same red sprite: equal
	setItem(t, a, 101, true, 1)
	setItem(t, b, 101, false, 2) // flag differs
	setItem(t, a, 102, false, 2)
	setItem(t, b, 102, false, 3) // green vs blue
	setItem(t, a, 103, false, 1, 2)
	setItem(t, b, 103, false, 2) // size differs
	setItem(t, b, 104, false, 3) // only in b
	setItem(t, a, 105, false, 0)
	setItem(t, b, 105, false, 3) // empty in a
	setItem(t, b, 106, false, 0) // only in b, no sprites

	res, err := Diff(a, b, thing.CategoryItem)
	if err != nil {
		t.Fatal(err)
	}
	want := []DiffEntry{
		{ID: 101, Status: DiffChanged, Changes: []string{ChangeProps}},
		{ID: 102, Status: DiffChanged, Changes: []string{ChangeSprites}},
		{ID: 103, Status: DiffChanged, Changes: []string{ChangeSize}},
		{ID: 104, Status: DiffOnlyB},
		{ID: 105, Status: DiffOnlyB},
		{ID: 106, Status: DiffOnlyB, Empty: true},
	}
	if res.Same != 1 || !slices.EqualFunc(res.Entries, want, func(x, y DiffEntry) bool {
		return x.ID == y.ID && x.Status == y.Status && x.Empty == y.Empty && slices.Equal(x.Changes, y.Changes)
	}) {
		t.Fatalf("diff %+v", res)
	}

	back, _ := Diff(b, a, thing.CategoryItem)
	if back.Entries[len(back.Entries)-1].Status != DiffOnlyA {
		t.Fatalf("reverse diff %+v", back)
	}
}

func TestTransferReplaceAndExtend(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	a.AddSprites([][]byte{solid(255, 0, 0)})
	b.AddSprites([][]byte{solid(255, 0, 0), solid(0, 0, 255)})
	setItem(t, b, 100, true, 1)
	setItem(t, b, 103, false, 2, 1)

	res, err := Transfer(a, b, thing.CategoryItem, []uint32{100, 103}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.IDs, []uint32{100, 103}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	// Red sprite 1 is the same in both clients and is reused; blue is new.
	if a.Info().Counts.Items != 103 || a.Info().Counts.Sprites != 2 {
		t.Fatalf("counts %+v", a.Info().Counts)
	}
	it, _ := a.Thing(thing.CategoryItem, 103)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{2, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if d, _ := Diff(a, b, thing.CategoryItem); len(d.Entries) != 0 {
		t.Fatalf("still differs: %+v", d.Entries)
	}
	// One undo reverts the whole transfer.
	a.Undo()
	if a.Info().Counts.Items != 100 || a.Info().Counts.Sprites != 1 {
		t.Fatalf("after undo %+v", a.Info().Counts)
	}
}

func TestRestoreThings(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	a.AddSprites([][]byte{solid(255, 0, 0)})
	b.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, a, 100, false, 1)
	setItem(t, a, 101, false, 1)
	setItem(t, b, 100, true, 1)
	setItem(t, b, 101, true, 1)
	setItem(t, b, 102, true, 1)

	res, err := Transfer(a, b, thing.CategoryItem, []uint32{100, 101, 102}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Revert one copy only, then the copy past the end.
	if err := a.RestoreThings(thing.CategoryItem, map[uint32]*thing.Thing{100: res.Before[100]}); err != nil {
		t.Fatal(err)
	}
	if it, _ := a.Thing(thing.CategoryItem, 100); it.Props.Pickupable || it.FrameGroups[0].Sprites[0] != 1 {
		t.Fatalf("100 not restored: %+v", it)
	}
	if it, _ := a.Thing(thing.CategoryItem, 101); !it.Props.Pickupable {
		t.Fatal("101 was reverted too")
	}
	if err := a.RestoreThings(thing.CategoryItem, map[uint32]*thing.Thing{102: res.Before[102]}); err != nil {
		t.Fatal(err)
	}
	if a.Info().Counts.Items != 101 {
		t.Fatalf("items %d", a.Info().Counts.Items)
	}
	// Each revert is its own undo step.
	a.Undo()
	if a.Info().Counts.Items != 102 {
		t.Fatalf("after undo %d", a.Info().Counts.Items)
	}
}

func TestTransferAppend(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	b.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, b, 100, true, 1)
	setItem(t, b, 101, false, 1)

	res, err := Transfer(a, b, thing.CategoryItem, []uint32{100, 101}, true)
	if err != nil {
		t.Fatal(err)
	}
	// Both things share one sprite, which is added once.
	if !slices.Equal(res.IDs, []uint32{101, 102}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	it, _ := a.Thing(thing.CategoryItem, 101)
	if !it.Props.Pickupable || it.FrameGroups[0].Sprites[0] != 1 {
		t.Fatalf("copied %+v", it)
	}
	if _, err := Transfer(a, a, thing.CategoryItem, []uint32{100}, true); err == nil {
		t.Fatal("transfer into itself")
	}
	if _, err := Transfer(a, b, thing.CategoryItem, []uint32{500}, true); err == nil {
		t.Fatal("missing source thing")
	}
}

func TestTransferMergesFrameGroups(t *testing.T) {
	old := New(version(t, 860), client.Features{})
	if old.Info().Features.FrameGroups {
		t.Skip("8.60 has frame groups")
	}
	b := New(v1098(), client.Features{})
	o, _ := b.Thing(thing.CategoryOutfit, 1)
	o = splitGroups(o)
	if err := b.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Transfer(old, b, thing.CategoryOutfit, []uint32{1}, false); err != nil {
		t.Fatal(err)
	}
	got, _ := old.Thing(thing.CategoryOutfit, 1)
	if len(got.FrameGroups) != 1 || got.FrameGroups[0].Frames != 2 {
		t.Fatalf("groups %+v", got.FrameGroups)
	}
}

func TestDiffIgnoresFormatDifferences(t *testing.T) {
	old := New(version(t, 860), client.Features{})
	cur := New(v1098(), client.Features{})
	if old.Info().Features.FrameGroups || old.Info().Features.ImprovedAnimations {
		t.Skip("8.60 has frame groups or improved animations")
	}
	for _, p := range []*Project{old, cur} {
		p.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0), solid(0, 0, 255)})
	}
	// The same walking outfit: one group of 3 frames in 8.60, idle + walking
	// groups with their own durations in 10.98.
	o, _ := old.Thing(thing.CategoryOutfit, 1)
	g := o.FrameGroups[0]
	g.Resize(1, 1, 1, 1, 1, 1, 3, thing.CategoryOutfit.DefaultDuration())
	g.Sprites = []uint32{1, 2, 3}
	if err := old.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	n := splitGroups(o)
	n.FrameGroups[1].Durations = []thing.FrameDuration{{Min: 50, Max: 50}, {Min: 70, Max: 70}}
	if err := cur.UpdateThing(n); err != nil {
		t.Fatal(err)
	}
	if d, err := Diff(old, cur, thing.CategoryOutfit); err != nil || len(d.Entries) != 0 || d.Same != 1 {
		t.Fatalf("diff %+v %v", d, err)
	}

	// Both clients store durations: they count.
	other := New(v1098(), client.Features{})
	other.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0), solid(0, 0, 255)})
	n.FrameGroups[1].Durations[0].Max = 90
	if err := other.UpdateThing(n); err != nil {
		t.Fatal(err)
	}
	d, err := Diff(cur, other, thing.CategoryOutfit)
	if err != nil || len(d.Entries) != 1 || !slices.Equal(d.Entries[0].Changes, []string{ChangeAnimation}) {
		t.Fatalf("diff %+v %v", d, err)
	}
}

func TestDiffChunksKeepOrder(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	a.AddSprites([][]byte{solid(255, 0, 0)})
	b.AddSprites([][]byte{solid(255, 0, 0)})
	const n = 3*1024 + 17
	setItem(t, a, 100+n, false, 1)
	setItem(t, b, 100+n-5, true, 1)
	d, err := Diff(a, b, thing.CategoryItem)
	if err != nil {
		t.Fatal(err)
	}
	// 100+n-5 is empty in a, the last 5 ids exist only in a.
	if len(d.Entries) != 6 || d.Same != n-5 {
		t.Fatalf("%d entries, %d same", len(d.Entries), d.Same)
	}
	for i := 1; i < len(d.Entries); i++ {
		if d.Entries[i].ID <= d.Entries[i-1].ID {
			t.Fatalf("out of order: %+v", d.Entries)
		}
	}
	if d.Entries[0].Status != DiffOnlyB || d.Entries[5].Status != DiffOnlyA {
		t.Fatalf("entries %+v", d.Entries)
	}
}

func TestTransferTranscodesTransparency(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{Transparency: true})
	half := solid(10, 20, 30)
	for i := 3; i < len(half); i += 8 {
		half[i] = 0 // every other pixel transparent
	}
	b.AddSprites([][]byte{half})
	setItem(t, b, 100, false, 1)
	if _, err := Transfer(a, b, thing.CategoryItem, []uint32{100}, false); err != nil {
		t.Fatal(err)
	}
	if d, err := Diff(a, b, thing.CategoryItem); err != nil || len(d.Entries) != 0 {
		t.Fatalf("diff %+v %v", d, err)
	}
	got, _ := a.SpritePixels(1)
	want, _ := b.SpritePixels(1)
	if !slices.Equal(got, want) {
		t.Fatal("pixels changed")
	}
}

func TestTransferToAndFreeIDs(t *testing.T) {
	a := New(v1098(), client.Features{})
	b := New(v1098(), client.Features{})
	a.AddSprites([][]byte{solid(255, 0, 0)})
	b.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, a, 101, false, 1)
	setItem(t, a, 102, false, 0)
	setItem(t, a, 103, true, 0) // no pixels but a flag: not free
	setItem(t, a, 104, false, 1)
	setItem(t, b, 101, true, 1)
	setItem(t, b, 102, false, 1)

	free, err := a.FreeIDs(thing.CategoryItem, 101, 3)
	if err != nil || !slices.Equal(free, []uint32{102, 105, 106}) {
		t.Fatalf("free %v %v", free, err)
	}
	if taken := a.TakenIDs(thing.CategoryItem, []uint32{101, 102, 103, 200}); !slices.Equal(taken, []uint32{101, 103}) {
		t.Fatalf("taken %v", taken)
	}

	res, err := TransferTo(a, b, thing.CategoryItem, []uint32{101, 102}, []uint32{102, 106})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.IDs, []uint32{102, 106}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	if res.Before[102] == nil || res.Before[106] != nil {
		t.Fatalf("before %+v", res.Before)
	}
	it, _ := a.Thing(thing.CategoryItem, 102)
	if !it.Props.Pickupable || it.ID != 102 {
		t.Fatalf("copied %+v", it)
	}
	if a.Info().Counts.Items != 106 {
		t.Fatalf("items %d", a.Info().Counts.Items)
	}
	if _, err := TransferTo(a, b, thing.CategoryItem, []uint32{101, 102}, []uint32{110, 110}); err == nil {
		t.Fatal("duplicate target")
	}
	if _, err := TransferTo(a, b, thing.CategoryItem, []uint32{101}, []uint32{99}); err == nil {
		t.Fatal("target below the first id")
	}
}
