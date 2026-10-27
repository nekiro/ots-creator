package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func thingEnv(t *testing.T) (*Session, *ThingService, *SpriteService) {
	t.Helper()
	session, projects, things, sprites, _ := newEnv(t)
	projects.New(v1098(), client.Features{})
	for range 3 {
		if _, err := things.Add(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	return session, things, sprites
}

func TestThingServiceNoProject(t *testing.T) {
	_, _, things, _, _ := newEnv(t)
	if _, err := things.Add(thing.CategoryItem); !errors.Is(err, ErrNoProject) {
		t.Fatalf("Add err = %v", err)
	}
	if err := things.Remove(thing.CategoryItem, []uint32{100}); !errors.Is(err, ErrNoProject) {
		t.Fatalf("Remove err = %v", err)
	}
	if _, err := things.ImportObjects([]string{"a", "b"}, 100); !errors.Is(err, ErrNoProject) {
		t.Fatalf("ImportObjects err = %v", err)
	}
}

func TestThingServiceEdits(t *testing.T) {
	session, things, _ := thingEnv(t)
	item, err := things.Get(thing.CategoryItem, 100)
	if err != nil {
		t.Fatal(err)
	}
	item.Name = "sword"
	if err := things.UpdateMany([]*thing.Thing{item}); err != nil {
		t.Fatal(err)
	}
	if names, _ := things.Names(thing.CategoryItem); names[100] != "sword" {
		t.Fatalf("names = %v", names)
	}
	if found, _ := things.Find(thing.CategoryItem, project.Filter{Name: "SWO"}); !slices.Equal(found, []uint32{100}) {
		t.Fatalf("find = %v", found)
	}
	copies, err := things.Duplicate(thing.CategoryItem, []uint32{100})
	if err != nil || len(copies) != 1 || copies[0] <= 102 {
		t.Fatalf("duplicate = %v, %v", copies, err)
	}
	moved, err := things.Move(thing.CategoryItem, copies, 101, project.MoveOptions{})
	if err != nil || !slices.Equal(moved, []uint32{101}) {
		t.Fatalf("move = %v, %v", moved, err)
	}
	if _, err := things.Move(thing.CategoryItem, []uint32{999}, 100, project.MoveOptions{}); err == nil {
		t.Fatal("moved a missing thing")
	}
	changed, err := things.Patch(thing.CategoryItem, []uint32{100, 101}, project.PropsPatch{"pickupable": true})
	if err != nil || changed != 2 {
		t.Fatalf("patch = %d, %v", changed, err)
	}
	if unchanged, err := things.Patch(thing.CategoryItem, []uint32{100}, project.PropsPatch{}); err != nil || unchanged != 0 {
		t.Fatalf("empty patch = %d, %v", unchanged, err)
	}
	if _, err := things.SetDurations([]thing.Category{thing.CategoryItem}, 100, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := things.ConvertFrameGroups(true); err != nil {
		t.Fatal(err)
	}
	if err := things.Remove(thing.CategoryItem, copies); err != nil {
		t.Fatal(err)
	}
	if state := session.State(); !state.Info.Changed {
		t.Fatal("edits did not mark the client changed")
	}
}

func TestThingServicePixels(t *testing.T) {
	_, things, sprites := thingEnv(t)
	// One red pixel in the middle, so shifting keeps it in the texture.
	red := make([]byte, 32*32*4)
	center := (16*32 + 16) * 4
	red[center], red[center+3] = 255, 255
	if err := things.SetPixels(thing.CategoryItem, 100, 0, []int{0}, [][]byte{red}); err != nil {
		t.Fatal(err)
	}
	if err := things.ShiftPixels(thing.CategoryItem, 100, 0, 1, 0, false, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := things.SetPixels(thing.CategoryItem, 100, 5, []int{0}, [][]byte{red}); err == nil {
		t.Fatal("painted a missing frame group")
	}
	item, _ := things.Get(thing.CategoryItem, 100)
	spriteID := item.FrameGroups[0].Sprites[0]
	if users, err := sprites.Users(spriteID); err != nil || len(users) != 1 {
		t.Fatalf("users = %v, %v", users, err)
	}
	if found, err := sprites.Find(project.SpritesUnused); err != nil {
		t.Fatalf("find = %v, %v", found, err)
	}
	other, err := sprites.AddPixels([][]byte{red})
	if err != nil {
		t.Fatal(err)
	}
	if replaced, err := sprites.ReplaceRefs([]uint32{spriteID}, other[0]); err != nil || replaced != 1 {
		t.Fatalf("replace refs = %d, %v", replaced, err)
	}
	if result, err := sprites.Analyze(project.OptimizeOptions{Unused: true}); err != nil || result.Unused == 0 {
		t.Fatalf("analyze = %+v, %v", result, err)
	}
	path := filepath.Join(t.TempDir(), "sprite.png")
	writePNG(t, path, 32, 32, [4]byte{0, 0, 255, 255})
	if err := sprites.Replace(other[0], path); err != nil {
		t.Fatal(err)
	}
	if err := sprites.Remove([]uint32{spriteID}); err != nil {
		t.Fatal(err)
	}
}

func TestInspectClient(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := inspectClient(dir); err == nil {
		t.Fatal("inspected an empty folder")
	}
	write("Tibia.dat", []byte{0xA3, 0x42, 0, 0})
	if _, err := inspectClient(dir); err == nil {
		t.Fatal("inspected a client without spr")
	}
	write("Tibia1.spr", nil)
	if _, err := inspectClient(dir); err == nil || !strings.Contains(err.Error(), "splits its sprites") {
		t.Fatalf("split sprites err = %v", err)
	}
	write("Tibia.spr", []byte{0x03, 0xD6, 0xBB, 0x57})
	files, err := inspectClient(filepath.Join(dir, "Tibia.spr"))
	if err != nil {
		t.Fatal(err)
	}
	if files.Detected == nil || files.Detected.Value != 1098 || files.Features.SpriteSize != client.DefaultSpriteSize || files.HasOTFI {
		t.Fatalf("files = %+v", files)
	}
	write("short.dat", []byte{1})
	if _, err := readSignature(filepath.Join(dir, "short.dat")); err == nil {
		t.Fatal("read a signature from a 1 byte file")
	}
}
