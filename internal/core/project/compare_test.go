package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func version(t *testing.T, value uint16) client.Version {
	t.Helper()
	for _, version := range client.Versions() {
		if version.Value == value {
			return version
		}
	}
	t.Fatalf("no version %d", value)
	return client.Version{}
}

// setItem gives item id the sprites and pickupable flag, adding the item
// (and empty ones before it) when needed.
func setItem(t *testing.T, project *Project, id uint32, pickup bool, sprites ...uint32) {
	t.Helper()
	for project.Info().Counts.Items < id {
		if _, err := project.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	it, _ := project.Thing(thing.CategoryItem, id)
	it.Props.Pickupable = pickup
	group := it.FrameGroups[0]
	group.Width, group.Frames = uint8(len(sprites)), 1
	group.Sprites = sprites
	if err := project.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientA.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0)})
	// Same pixels under other ids in b.
	clientB.AddSprites([][]byte{solid(0, 255, 0), solid(255, 0, 0), solid(0, 0, 255)})
	setItem(t, clientA, 100, false, 1)
	setItem(t, clientB, 100, false, 2) // same red sprite: equal
	setItem(t, clientA, 101, true, 1)
	setItem(t, clientB, 101, false, 2) // flag differs
	setItem(t, clientA, 102, false, 2)
	setItem(t, clientB, 102, false, 3) // green vs blue
	setItem(t, clientA, 103, false, 1, 2)
	setItem(t, clientB, 103, false, 2) // size differs
	setItem(t, clientB, 104, false, 3) // only in b
	setItem(t, clientA, 105, false, 0)
	setItem(t, clientB, 105, false, 3) // empty in a
	setItem(t, clientB, 106, false, 0) // only in b, no sprites

	result, err := Diff(clientA, clientB, thing.CategoryItem)
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
	if result.Same != 1 || !slices.EqualFunc(result.Entries, want, func(x, y DiffEntry) bool {
		return x.ID == y.ID && x.Status == y.Status && x.Empty == y.Empty && slices.Equal(x.Changes, y.Changes)
	}) {
		t.Fatalf("diff %+v", result)
	}

	back, _ := Diff(clientB, clientA, thing.CategoryItem)
	if back.Entries[len(back.Entries)-1].Status != DiffOnlyA {
		t.Fatalf("reverse diff %+v", back)
	}
}

func TestTransferReplaceAndExtend(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientA.AddSprites([][]byte{solid(255, 0, 0)})
	clientB.AddSprites([][]byte{solid(255, 0, 0), solid(0, 0, 255)})
	setItem(t, clientB, 100, true, 1)
	setItem(t, clientB, 103, false, 2, 1)

	res, err := Transfer(clientA, clientB, thing.CategoryItem, []uint32{100, 103}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.IDs, []uint32{100, 103}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	// Red sprite 1 is the same in both clients and is reused; blue is new.
	if clientA.Info().Counts.Items != 103 || clientA.Info().Counts.Sprites != 2 {
		t.Fatalf("counts %+v", clientA.Info().Counts)
	}
	it, _ := clientA.Thing(thing.CategoryItem, 103)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{2, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if d, _ := Diff(clientA, clientB, thing.CategoryItem); len(d.Entries) != 0 {
		t.Fatalf("still differs: %+v", d.Entries)
	}
	// One undo reverts the whole transfer.
	clientA.Undo()
	if clientA.Info().Counts.Items != 100 || clientA.Info().Counts.Sprites != 1 {
		t.Fatalf("after undo %+v", clientA.Info().Counts)
	}
}

func TestRestoreThings(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientA.AddSprites([][]byte{solid(255, 0, 0)})
	clientB.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, clientA, 100, false, 1)
	setItem(t, clientA, 101, false, 1)
	setItem(t, clientB, 100, true, 1)
	setItem(t, clientB, 101, true, 1)
	setItem(t, clientB, 102, true, 1)

	res, err := Transfer(clientA, clientB, thing.CategoryItem, []uint32{100, 101, 102}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Revert one copy only, then the copy past the end.
	if err := clientA.RestoreThings(thing.CategoryItem, map[uint32]*thing.Thing{100: res.Before[100]}); err != nil {
		t.Fatal(err)
	}
	if it, _ := clientA.Thing(thing.CategoryItem, 100); it.Props.Pickupable || it.FrameGroups[0].Sprites[0] != 1 {
		t.Fatalf("100 not restored: %+v", it)
	}
	if it, _ := clientA.Thing(thing.CategoryItem, 101); !it.Props.Pickupable {
		t.Fatal("101 was reverted too")
	}
	if err := clientA.RestoreThings(thing.CategoryItem, map[uint32]*thing.Thing{102: res.Before[102]}); err != nil {
		t.Fatal(err)
	}
	if clientA.Info().Counts.Items != 101 {
		t.Fatalf("items %d", clientA.Info().Counts.Items)
	}
	// Each revert is its own undo step.
	clientA.Undo()
	if clientA.Info().Counts.Items != 102 {
		t.Fatalf("after undo %d", clientA.Info().Counts.Items)
	}
}

func TestTransferAppend(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientB.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, clientB, 100, true, 1)
	setItem(t, clientB, 101, false, 1)

	res, err := Transfer(clientA, clientB, thing.CategoryItem, []uint32{100, 101}, true)
	if err != nil {
		t.Fatal(err)
	}
	// Both things share one sprite, which is added once.
	if !slices.Equal(res.IDs, []uint32{101, 102}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	it, _ := clientA.Thing(thing.CategoryItem, 101)
	if !it.Props.Pickupable || it.FrameGroups[0].Sprites[0] != 1 {
		t.Fatalf("copied %+v", it)
	}
	if _, err := Transfer(clientA, clientA, thing.CategoryItem, []uint32{100}, true); err == nil {
		t.Fatal("transfer into itself")
	}
	if _, err := Transfer(clientA, clientB, thing.CategoryItem, []uint32{500}, true); err == nil {
		t.Fatal("missing source thing")
	}
}

func TestTransferMergesFrameGroups(t *testing.T) {
	old := New(version(t, 860), client.Features{})
	if old.Info().Features.FrameGroups {
		t.Skip("8.60 has frame groups")
	}
	clientB := New(v1098(), client.Features{})
	o, _ := clientB.Thing(thing.CategoryOutfit, 1)
	o = splitGroups(o)
	if err := clientB.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Transfer(old, clientB, thing.CategoryOutfit, []uint32{1}, false); err != nil {
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
	for _, project := range []*Project{old, cur} {
		project.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0), solid(0, 0, 255)})
	}
	// The same walking outfit: one group of 3 frames in 8.60, idle + walking
	// groups with their own durations in 10.98.
	o, _ := old.Thing(thing.CategoryOutfit, 1)
	group := o.FrameGroups[0]
	group.Resize(1, 1, 1, 1, 1, 1, 3, thing.CategoryOutfit.DefaultDuration())
	group.Sprites = []uint32{1, 2, 3}
	if err := old.UpdateThing(o); err != nil {
		t.Fatal(err)
	}
	duplicate := splitGroups(o)
	duplicate.FrameGroups[1].Durations = []thing.FrameDuration{{Min: 50, Max: 50}, {Min: 70, Max: 70}}
	if err := cur.UpdateThing(duplicate); err != nil {
		t.Fatal(err)
	}
	if d, err := Diff(old, cur, thing.CategoryOutfit); err != nil || len(d.Entries) != 0 || d.Same != 1 {
		t.Fatalf("diff %+v %v", d, err)
	}

	// Both clients store durations: they count.
	other := New(v1098(), client.Features{})
	other.AddSprites([][]byte{solid(255, 0, 0), solid(0, 255, 0), solid(0, 0, 255)})
	duplicate.FrameGroups[1].Durations[0].Max = 90
	if err := other.UpdateThing(duplicate); err != nil {
		t.Fatal(err)
	}
	d, err := Diff(cur, other, thing.CategoryOutfit)
	if err != nil || len(d.Entries) != 1 || !slices.Equal(d.Entries[0].Changes, []string{ChangeAnimation}) {
		t.Fatalf("diff %+v %v", d, err)
	}
}

func TestDiffChunksKeepOrder(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientA.AddSprites([][]byte{solid(255, 0, 0)})
	clientB.AddSprites([][]byte{solid(255, 0, 0)})
	const n = 3*1024 + 17
	setItem(t, clientA, 100+n, false, 1)
	setItem(t, clientB, 100+n-5, true, 1)
	d, err := Diff(clientA, clientB, thing.CategoryItem)
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
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{Transparency: true})
	half := solid(10, 20, 30)
	for i := 3; i < len(half); i += 8 {
		half[i] = 0 // every other pixel transparent
	}
	clientB.AddSprites([][]byte{half})
	setItem(t, clientB, 100, false, 1)
	if _, err := Transfer(clientA, clientB, thing.CategoryItem, []uint32{100}, false); err != nil {
		t.Fatal(err)
	}
	if d, err := Diff(clientA, clientB, thing.CategoryItem); err != nil || len(d.Entries) != 0 {
		t.Fatalf("diff %+v %v", d, err)
	}
	got, _ := clientA.SpritePixels(1)
	want, _ := clientB.SpritePixels(1)
	if !slices.Equal(got, want) {
		t.Fatal("pixels changed")
	}
}

func TestTransferToAndFreeIDs(t *testing.T) {
	clientA := New(v1098(), client.Features{})
	clientB := New(v1098(), client.Features{})
	clientA.AddSprites([][]byte{solid(255, 0, 0)})
	clientB.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, clientA, 101, false, 1)
	setItem(t, clientA, 102, false, 0)
	setItem(t, clientA, 103, true, 0) // no pixels but a flag: not free
	setItem(t, clientA, 104, false, 1)
	setItem(t, clientB, 101, true, 1)
	setItem(t, clientB, 102, false, 1)

	free, err := clientA.FreeIDs(thing.CategoryItem, 101, 3)
	if err != nil || !slices.Equal(free, []uint32{102, 105, 106}) {
		t.Fatalf("free %v %v", free, err)
	}
	if taken := clientA.TakenIDs(thing.CategoryItem, []uint32{101, 102, 103, 200}); !slices.Equal(taken, []uint32{101, 103}) {
		t.Fatalf("taken %v", taken)
	}

	res, err := TransferTo(clientA, clientB, thing.CategoryItem, []uint32{101, 102}, []uint32{102, 106})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.IDs, []uint32{102, 106}) || res.Sprites != 1 {
		t.Fatalf("result %+v", res)
	}
	if res.Before[102] == nil || res.Before[106] != nil {
		t.Fatalf("before %+v", res.Before)
	}
	it, _ := clientA.Thing(thing.CategoryItem, 102)
	if !it.Props.Pickupable || it.ID != 102 {
		t.Fatalf("copied %+v", it)
	}
	if clientA.Info().Counts.Items != 106 {
		t.Fatalf("items %d", clientA.Info().Counts.Items)
	}
	if _, err := TransferTo(clientA, clientB, thing.CategoryItem, []uint32{101, 102}, []uint32{110, 110}); err == nil {
		t.Fatal("duplicate target")
	}
	if _, err := TransferTo(clientA, clientB, thing.CategoryItem, []uint32{101}, []uint32{99}); err == nil {
		t.Fatal("target below the first id")
	}
}
