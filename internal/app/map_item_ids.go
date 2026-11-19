package app

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// ItemIDsPlan proposes how a map to open or create stores its item ids
// (see map_data.go) and tells why, for the user to confirm or change
// before the map loads.
type ItemIDsPlan struct {
	// Mode is the proposed item id mode: otitems.ModeOTB (server ids) or
	// otitems.ModeClient.
	Mode otitems.Mode `json:"mode"`
	// Reason tells why Mode is proposed.
	Reason string `json:"reason"`
	// ClientOTB is the items.otb attached to the client ("": none).
	ClientOTB string `json:"clientOtb"`
	// FoundOTB is another items.otb that fits the map: found next to it, or
	// the one an older version of the map editor used. "" when there is
	// none or it is the client's.
	FoundOTB string `json:"foundOtb"`
	// FileMode is the mode the map file states (OTMAP files do), "" when
	// the file does not say.
	FileMode otitems.Mode `json:"fileMode"`
	// Settled is set when the map file or the earlier choice for the map
	// decides the mode and its items.otb is at hand: the map opens without
	// asking.
	Settled bool `json:"settled"`
}

// ItemIDsChoice is how a map to open or create stores its item ids.
type ItemIDsChoice struct {
	// Mode is otitems.ModeOTB (server ids) or otitems.ModeClient; "" takes
	// the proposed one (PlanItemIDs).
	Mode otitems.Mode `json:"mode"`
	// OTBPath is the items.otb of server ids. Another file than the one of
	// the client is attached to the client (it replaces the old one, for
	// compiling too). "": the client's.
	OTBPath string `json:"otbPath"`
}

// PlanItemIDs proposes the item ids of the map at path ("": a new map).
func (ms *MapService) PlanItemIDs(path string) (ItemIDsPlan, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.planItemIDs(path)
}

func (ms *MapService) planItemIDs(path string) (ItemIDsPlan, error) {
	proj, err := ms.session.Project()
	if err != nil {
		return ItemIDsPlan{}, errNoClient
	}
	plan := ItemIDsPlan{ClientOTB: ms.session.ItemsOTB()}
	found, foundReason := "", ""
	if path != "" {
		found, foundReason = detectServerData(path).OTBPath, "an items.otb was found next to the map"
	}
	if config, _ := ms.store.Profile(clientKey(proj)); found == "" && fsutil.FileExists(config.LegacyOTBPath) {
		found, foundReason = config.LegacyOTBPath, "the map editor used this items.otb before"
	}
	if found != "" && !fsutil.SamePath(found, plan.ClientOTB) {
		plan.FoundOTB = found
	}
	if path != "" && otmap.IsOTMAP(path) {
		if clientIDs, ok := otmap.ItemIDsOf(path); ok {
			plan.FileMode = otitems.ModeOTB
			if clientIDs {
				plan.FileMode = otitems.ModeClient
			}
		}
	}
	remembered, hasRemembered := otitems.Mode(""), false
	if path != "" {
		remembered, hasRemembered = ms.store.MapItemIDs(path)
	}
	switch {
	case plan.FileMode != "":
		plan.Mode, plan.Reason = plan.FileMode, "the map file says it uses "+modeName(plan.FileMode)
		plan.Settled = plan.Mode == otitems.ModeClient || plan.ClientOTB != ""
	case hasRemembered:
		plan.Mode, plan.Reason = remembered, "chosen when the map was opened before"
		plan.Settled = plan.Mode == otitems.ModeClient || plan.ClientOTB != ""
	case plan.ClientOTB != "":
		plan.Mode, plan.Reason = otitems.ModeOTB, "the client has an items.otb attached"
	case plan.FoundOTB != "":
		plan.Mode, plan.Reason = otitems.ModeOTB, foundReason
	default:
		plan.Mode, plan.Reason = otitems.ModeClient, "no items.otb is attached to the client"
		if path != "" {
			plan.Reason += " or found next to the map"
		}
	}
	return plan, nil
}

func modeName(mode otitems.Mode) string {
	if mode == otitems.ModeClient {
		return "client ids"
	}
	return "server ids"
}

// chooseItemIDs applies the item ids chosen for the map at path ("": a new
// map): it attaches the chosen items.otb to the client, takes the server
// data found next to the map when the client has none, and returns the
// item data of the mode. The caller holds the write lock.
func (ms *MapService) chooseItemIDs(path string, choice ItemIDsChoice) (*mapData, error) {
	plan, err := ms.planItemIDs(path)
	if err != nil {
		return nil, err
	}
	if choice.Mode == "" {
		choice.Mode = plan.Mode
		if choice.OTBPath == "" && plan.Mode == otitems.ModeOTB && plan.ClientOTB == "" {
			choice.OTBPath = plan.FoundOTB
		}
	}
	switch choice.Mode {
	case otitems.ModeOTB:
		if choice.OTBPath != "" && !fsutil.SamePath(choice.OTBPath, plan.ClientOTB) {
			if err := ms.session.attachItemsOTB(choice.OTBPath); err != nil {
				return nil, err
			}
		}
		if ms.session.ItemsOTB() == "" {
			return nil, errNoItemsOTB
		}
	case otitems.ModeClient:
	default:
		return nil, fmt.Errorf("unknown item ids %q: use %q (server ids) or %q", choice.Mode, otitems.ModeOTB, otitems.ModeClient)
	}
	if path != "" {
		ms.adoptServerData(path)
	}
	return ms.dataOf(choice.Mode)
}

// adoptServerData stores the items.xml and server folder found next to the
// map at path when the client has none yet.
func (ms *MapService) adoptServerData(path string) {
	proj, err := ms.session.Project()
	if err != nil {
		return
	}
	client := clientKey(proj)
	config, _ := ms.store.Profile(client)
	if config.XMLPath != "" || config.ServerDir != "" {
		return
	}
	detected := detectServerData(path)
	if detected.XMLPath == "" && detected.ServerDir == "" {
		return
	}
	config.XMLPath, config.ServerDir = detected.XMLPath, detected.ServerDir
	if err := ms.store.SetProfile(client, config); err != nil {
		ms.session.Warn("settings: " + err.Error())
	}
	// The item data of the client read the old server data.
	clear(ms.data)
}
