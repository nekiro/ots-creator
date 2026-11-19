package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func TestMapsUseTheItemsOTBOfTheClient(t *testing.T) {
	ms, session := mapTestService(t)
	if plan, err := ms.PlanItemIDs(""); err != nil || plan.Mode != otitems.ModeClient || plan.ClientOTB != "" {
		t.Fatalf("plan without items.otb = %+v, %v", plan, err)
	}
	otb := writeTestOTB(t)
	if err := session.attachItemsOTB(otb); err != nil {
		t.Fatal(err)
	}
	plan, err := ms.PlanItemIDs("")
	if err != nil || plan.Mode != otitems.ModeOTB || plan.ClientOTB != otb || !strings.Contains(plan.Reason, "attached") {
		t.Fatalf("plan with the client's items.otb = %+v, %v", plan, err)
	}
	info, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64})
	if err != nil || info.ItemMode != otitems.ModeOTB {
		t.Fatalf("new map = %+v, %v", info, err)
	}
	view, err := ms.ViewData(info.ID, true)
	if err != nil || view.Data.OTBPath != otb || view.Data.Items != 3 {
		t.Fatalf("item data = %+v, %v", view.Data, err)
	}
}

func TestClientIDsAreKeptWhenTheClientHasAnItemsOTB(t *testing.T) {
	ms, session := mapTestService(t)
	if err := session.attachItemsOTB(writeTestOTB(t)); err != nil {
		t.Fatal(err)
	}
	client, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64, ItemIDs: ItemIDsChoice{Mode: otitems.ModeClient}})
	if err != nil || client.ItemMode != otitems.ModeClient {
		t.Fatalf("client id map = %+v, %v", client, err)
	}
	server, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64})
	if err != nil || server.ItemMode != otitems.ModeOTB {
		t.Fatalf("server id map = %+v, %v", server, err)
	}
	// Each map keeps the data of its own ids.
	for id, mode := range map[int]otitems.Mode{client.ID: otitems.ModeClient, server.ID: otitems.ModeOTB} {
		if view, err := ms.ViewData(id, true); err != nil || view.Data.Mode != mode {
			t.Fatalf("map %d data = %+v, %v", id, view.Data, err)
		}
	}
	path := filepath.Join(t.TempDir(), "client.otbm")
	if _, err := ms.SaveMap(client.ID, path); err != nil {
		t.Fatal(err)
	}
	ms.CloseMap(client.ID)
	plan, err := ms.PlanItemIDs(path)
	if err != nil || plan.Mode != otitems.ModeClient || !plan.Settled {
		t.Fatalf("plan of the saved client id map = %+v, %v", plan, err)
	}
	opened, err := ms.OpenMap(path, ItemIDsChoice{})
	if err != nil || opened.Info.ItemMode != otitems.ModeClient {
		t.Fatalf("reopened = %+v, %v", opened.Info, err)
	}
}

func TestChoosingAnItemsOTBAttachesItToTheClient(t *testing.T) {
	ms, session := mapTestService(t)
	otb := writeTestOTB(t)
	info, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64, ItemIDs: ItemIDsChoice{Mode: otitems.ModeOTB, OTBPath: otb}})
	if err != nil || info.ItemMode != otitems.ModeOTB {
		t.Fatalf("new map = %+v, %v", info, err)
	}
	if !fsutil.SamePath(session.ItemsOTB(), otb) {
		t.Fatalf("client items.otb = %q", session.ItemsOTB())
	}
}

func TestServerIDsWithoutItemsOTBFail(t *testing.T) {
	ms, session := mapTestService(t)
	if _, err := ms.NewMap(NewMapOptions{ItemIDs: ItemIDsChoice{Mode: otitems.ModeOTB}}); !errors.Is(err, errNoItemsOTB) {
		t.Fatalf("server ids without items.otb = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing.otb")
	if _, err := ms.NewMap(NewMapOptions{ItemIDs: ItemIDsChoice{Mode: otitems.ModeOTB, OTBPath: missing}}); err == nil {
		t.Fatal("opened on a missing items.otb")
	}
	if session.ItemsOTB() != "" || len(ms.maps) != 0 {
		t.Fatal("a missing items.otb changed the client or opened a map")
	}
	if _, err := ms.NewMap(NewMapOptions{ItemIDs: ItemIDsChoice{Mode: "auto"}}); err == nil {
		t.Fatal("accepted unknown item ids")
	}
}

func TestPlanFollowsTheMapFileAndTheServerFolder(t *testing.T) {
	ms, session := mapTestService(t)
	server := t.TempDir()
	items := filepath.Join(server, "data", "items")
	world := filepath.Join(server, "data", "world")
	for _, dir := range []string{items, world} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	otb, err := os.ReadFile(writeTestOTB(t))
	if err != nil {
		t.Fatal(err)
	}
	found := filepath.Join(items, "items.otb")
	if err := os.WriteFile(found, otb, 0o644); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(world, "map.otbm")
	plan, err := ms.PlanItemIDs(mapPath)
	if err != nil || plan.Mode != otitems.ModeOTB || plan.FoundOTB != found || plan.ClientOTB != "" || plan.Settled {
		t.Fatalf("plan next to an items.otb = %+v, %v", plan, err)
	}
	// An OTMAP file says which ids it stores.
	info, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64, ItemIDs: ItemIDsChoice{Mode: otitems.ModeClient}})
	if err != nil {
		t.Fatal(err)
	}
	textPath := filepath.Join(world, "client.otmap")
	if _, err := ms.SaveMap(info.ID, textPath); err != nil {
		t.Fatal(err)
	}
	if clientIDs, ok := otmap.ItemIDsOf(textPath); !ok || !clientIDs {
		t.Fatal("the OTMAP file does not say client ids")
	}
	if err := session.attachItemsOTB(found); err != nil {
		t.Fatal(err)
	}
	plan, err = ms.PlanItemIDs(textPath)
	if err != nil || plan.Mode != otitems.ModeClient || plan.FileMode != otitems.ModeClient || plan.FoundOTB != "" {
		t.Fatalf("plan of a client id OTMAP = %+v, %v", plan, err)
	}
}

func TestRememberedServerIDsAskAgainWithoutItemsOTB(t *testing.T) {
	ms, session := mapTestService(t)
	if err := session.attachItemsOTB(writeTestOTB(t)); err != nil {
		t.Fatal(err)
	}
	info, err := ms.NewMap(NewMapOptions{Width: 64, Height: 64})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "server.otbm")
	if _, err := ms.SaveMap(info.ID, path); err != nil {
		t.Fatal(err)
	}
	if plan, _ := ms.PlanItemIDs(path); plan.Mode != otitems.ModeOTB || !plan.Settled {
		t.Fatalf("plan with the items.otb = %+v", plan)
	}
	if err := session.attachItemsOTB(""); err != nil {
		t.Fatal(err)
	}
	if plan, _ := ms.PlanItemIDs(path); plan.Mode != otitems.ModeOTB || plan.Settled {
		t.Fatalf("plan without the items.otb = %+v", plan)
	}
}
