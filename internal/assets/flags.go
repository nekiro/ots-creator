package assets

import (
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/thing"
)

// AppearanceFlags field numbers the editor maps to thing.Properties.
const (
	fBank           = 1
	fWrite          = 10
	fWriteOnce      = 11
	fHook           = 21
	fLight          = 23
	fShift          = 26
	fHeight         = 27
	fAutomap        = 30
	fLenshelp       = 31
	fClothes        = 34
	fDefaultAction  = 35
	fMarket         = 36
	fNpcSaleData    = 40
	fChangedExpire  = 41
	fCyclopedia     = 44
	fUpgrade        = 48
	fLastFlag       = 57 // wrapkit, the highest flag the editor knows
	fHookSouth      = 70
	fHookEast       = 71
	hookSouth       = 1
	hookEast        = 2
	marketCategory  = 1
	marketTradeAs   = 2
	marketShowAs    = 3
	marketVocation  = 5 // repeated PLAYER_PROFESSION
	marketMinLevel  = 6
	marketName      = 7
	payloadFirstSub = 1
)

// boolFlags are the flags without payload.
var boolFlags = []struct {
	num protowire.Number
	get func(*thing.Properties) *bool
}{
	{2, func(p *thing.Properties) *bool { return &p.GroundBorder }},
	{3, func(p *thing.Properties) *bool { return &p.OnBottom }},
	{4, func(p *thing.Properties) *bool { return &p.OnTop }},
	{5, func(p *thing.Properties) *bool { return &p.Container }},
	{6, func(p *thing.Properties) *bool { return &p.Stackable }},
	{7, func(p *thing.Properties) *bool { return &p.Usable }},
	{8, func(p *thing.Properties) *bool { return &p.ForceUse }},
	{9, func(p *thing.Properties) *bool { return &p.MultiUse }},
	{12, func(p *thing.Properties) *bool { return &p.Fluid }},
	{13, func(p *thing.Properties) *bool { return &p.Unpassable }},
	{14, func(p *thing.Properties) *bool { return &p.Unmoveable }},
	{15, func(p *thing.Properties) *bool { return &p.BlockMissile }},
	{16, func(p *thing.Properties) *bool { return &p.BlockPathfind }},
	{17, func(p *thing.Properties) *bool { return &p.NoMoveAnimation }},
	{18, func(p *thing.Properties) *bool { return &p.Pickupable }},
	{19, func(p *thing.Properties) *bool { return &p.FluidContainer }},
	{20, func(p *thing.Properties) *bool { return &p.Hangable }},
	{22, func(p *thing.Properties) *bool { return &p.Rotatable }},
	{24, func(p *thing.Properties) *bool { return &p.DontHide }},
	{25, func(p *thing.Properties) *bool { return &p.Translucent }},
	{28, func(p *thing.Properties) *bool { return &p.LyingObject }},
	{29, func(p *thing.Properties) *bool { return &p.AnimateAlways }},
	{32, func(p *thing.Properties) *bool { return &p.FullGround }},
	{33, func(p *thing.Properties) *bool { return &p.IgnoreLook }},
	{37, func(p *thing.Properties) *bool { return &p.Wrappable }},
	{38, func(p *thing.Properties) *bool { return &p.Unwrappable }},
	{39, func(p *thing.Properties) *bool { return &p.TopEffect }},
	{42, func(p *thing.Properties) *bool { return &p.Corpse }},
	{43, func(p *thing.Properties) *bool { return &p.PlayerCorpse }},
	{45, func(p *thing.Properties) *bool { return &p.Ammo }},
	{46, func(p *thing.Properties) *bool { return &p.ShowOffSocket }},
	{47, func(p *thing.Properties) *bool { return &p.Reportable }},
	{49, func(p *thing.Properties) *bool { return &p.ReverseAddonsEast }},
	{50, func(p *thing.Properties) *bool { return &p.ReverseAddonsWest }},
	{51, func(p *thing.Properties) *bool { return &p.ReverseAddonsSouth }},
	{52, func(p *thing.Properties) *bool { return &p.ReverseAddonsNorth }},
	{53, func(p *thing.Properties) *bool { return &p.Wearout }},
	{54, func(p *thing.Properties) *bool { return &p.ClockExpire }},
	{55, func(p *thing.Properties) *bool { return &p.Expire }},
	{56, func(p *thing.Properties) *bool { return &p.ExpireStop }},
	{57, func(p *thing.Properties) *bool { return &p.WrapKit }},
}

// NPC sale entry fields (AppearanceFlagNPC).
const (
	npcName      = 1
	npcLocation  = 2
	npcSalePrice = 3
	npcBuyPrice  = 4
	npcCurrency  = 5
	npcQuestFlag = 6
)

// professionMask turns PLAYER_PROFESSION values (knight 1 ... monk 5) into
// the vocation bit mask of thing.Market (knight 1, paladin 2, sorcerer 4,
// druid 8, monk 16). Any, none and promoted give no bit.
func professionMask(values []uint64) uint16 {
	var mask uint16
	for _, v := range values {
		if v >= 1 && v <= 5 {
			mask |= 1 << (v - 1)
		}
	}
	return mask
}

// knownFlags lists every flag number written by encodeFlags.
var knownFlags = func() []protowire.Number {
	out := []protowire.Number{fBank, fWrite, fWriteOnce, fHook, fLight, fShift, fHeight, fAutomap, fLenshelp, fClothes, fDefaultAction, fMarket,
		fNpcSaleData, fChangedExpire, fCyclopedia, fUpgrade, fHookSouth, fHookEast}
	for _, f := range boolFlags {
		out = append(out, f.num)
	}
	return out
}()

// Supported lists the json names of the properties appearances can store.
func Supported() []string {
	return []string{
		"ground", "groundBorder", "onBottom", "onTop", "container", "stackable", "forceUse", "multiUse",
		"writable", "writableOnce", "fluidContainer", "fluid", "unpassable", "unmoveable", "blockMissile",
		"blockPathfind", "noMoveAnimation", "pickupable", "hangable", "hookSouth", "hookEast", "rotatable",
		"hasLight", "dontHide", "translucent", "hasOffset", "hasElevation", "lyingObject", "animateAlways",
		"miniMap", "lensHelp", "fullGround", "ignoreLook", "cloth", "isMarket", "hasDefaultAction",
		"wrappable", "unwrappable", "topEffect", "usable",
		"changedToExpire", "corpse", "playerCorpse", "cyclopedia", "ammo", "showOffSocket", "reportable",
		"hasUpgradeClassification", "reverseAddonsEast", "reverseAddonsWest", "reverseAddonsSouth",
		"reverseAddonsNorth", "wearout", "clockExpire", "expire", "expireStop", "wrapKit", "npcSales",
	}
}

// Unsupported lists the json names of properties set in p that appearances
// cannot store.
func Unsupported(p *thing.Properties) []string {
	var out []string
	if p.HasCharges {
		out = append(out, "hasCharges")
	}
	if p.FloorChange {
		out = append(out, "floorChange")
	}
	if p.HasBones {
		out = append(out, "hasBones")
	}
	return out
}

func u16(v uint64) uint16 { return uint16(min(v, 0xFFFF)) }

func i16(v uint64) int16 { return int16(max(min(int64(int32(v)), 32767), -32768)) }

func decodeFlags(m message, p *thing.Properties) {
	for _, f := range boolFlags {
		*f.get(p) = m.bool(f.num)
	}
	if m.has(fBank) {
		p.Ground, p.GroundSpeed = true, u16(m.sub(fBank).uint(1))
	}
	if m.has(fWrite) {
		p.Writable, p.MaxTextLength = true, u16(m.sub(fWrite).uint(1))
	}
	if m.has(fWriteOnce) {
		p.WritableOnce, p.MaxReadLength = true, u16(m.sub(fWriteOnce).uint(1))
	}
	switch m.sub(fHook).uint(1) {
	case hookSouth:
		p.HookSouth = true
	case hookEast:
		p.HookEast = true
	}
	p.HookSouth = p.HookSouth || m.bool(fHookSouth)
	p.HookEast = p.HookEast || m.bool(fHookEast)
	if m.has(fLight) {
		s := m.sub(fLight)
		p.HasLight, p.LightLevel, p.LightColor = true, u16(s.uint(1)), u16(s.uint(2))
	}
	if m.has(fShift) {
		s := m.sub(fShift)
		p.HasOffset, p.OffsetX, p.OffsetY = true, i16(s.uint(1)), i16(s.uint(2))
	}
	if m.has(fHeight) {
		p.HasElevation, p.Elevation = true, u16(m.sub(fHeight).uint(1))
	}
	if m.has(fAutomap) {
		p.MiniMap, p.MiniMapColor = true, u16(m.sub(fAutomap).uint(1))
	}
	if m.has(fLenshelp) {
		p.LensHelp, p.LensHelpValue = true, u16(m.sub(fLenshelp).uint(1))
	}
	if m.has(fClothes) {
		p.Cloth, p.ClothSlot = true, u16(m.sub(fClothes).uint(1))
	}
	if m.has(fDefaultAction) {
		p.HasDefaultAction, p.DefaultAction = true, u16(m.sub(fDefaultAction).uint(1))
	}
	if m.has(fMarket) {
		s := m.sub(fMarket)
		name, _ := s.get(marketName)
		p.IsMarket = true
		p.Market = thing.Market{
			Category:      u16(s.uint(marketCategory)),
			TradeAs:       u16(s.uint(marketTradeAs)),
			ShowAs:        u16(s.uint(marketShowAs)),
			Name:          string(name.b),
			RestrictLevel: u16(s.uint(marketMinLevel)),

			RestrictProfession: professionMask(s.uints(marketVocation)),
		}
	}
	if m.has(fChangedExpire) {
		p.ChangedToExpire, p.FormerObjectID = true, u16(m.sub(fChangedExpire).uint(1))
	}
	if m.has(fCyclopedia) {
		p.Cyclopedia, p.CyclopediaType = true, u16(m.sub(fCyclopedia).uint(1))
	}
	if m.has(fUpgrade) {
		p.HasUpgradeClassification, p.UpgradeClassification = true, u16(m.sub(fUpgrade).uint(1))
	}
}

// decodeNpcSales reads the repeated NPC sale entries of the flags.
func decodeNpcSales(m message) []thing.NpcSale {
	var out []thing.NpcSale
	for _, s := range m.all(fNpcSaleData) {
		name, _ := s.get(npcName)
		loc, _ := s.get(npcLocation)
		quest, _ := s.get(npcQuestFlag)
		out = append(out, thing.NpcSale{
			Name:              binio.DecodeLatin1(name.b),
			Location:          binio.DecodeLatin1(loc.b),
			SalePrice:         uint32(s.uint(npcSalePrice)),
			BuyPrice:          uint32(s.uint(npcBuyPrice)),
			CurrencyObjectID:  uint32(s.uint(npcCurrency)),
			CurrencyQuestFlag: binio.DecodeLatin1(quest.b),
		})
	}
	return out
}

func encodeNpcSale(n thing.NpcSale) []byte {
	b := putBytes(nil, npcName, binio.EncodeLatin1(n.Name))
	b = putBytes(b, npcLocation, binio.EncodeLatin1(n.Location))
	b = putUint(b, npcSalePrice, uint64(n.SalePrice))
	b = putUint(b, npcBuyPrice, uint64(n.BuyPrice))
	if n.CurrencyObjectID != 0 {
		b = putUint(b, npcCurrency, uint64(n.CurrencyObjectID))
	}
	if n.CurrencyQuestFlag != "" {
		b = putBytes(b, npcQuestFlag, binio.EncodeLatin1(n.CurrencyQuestFlag))
	}
	return b
}

// encodeFlags writes the flags of p and the NPC sale entries. Fields of om
// (the original flags) that the model does not know are kept, also inside
// the flags it does.
func encodeFlags(p *thing.Properties, npc []thing.NpcSale, om message) []byte {
	var b []byte
	sub := func(num protowire.Number, known []protowire.Number, fields []byte) {
		b = putBytes(b, num, om.sub(num).appendOthers(fields, known...))
	}
	one := func(num protowire.Number, v uint64) {
		sub(num, []protowire.Number{payloadFirstSub}, putUint(nil, payloadFirstSub, v))
	}
	bools := map[protowire.Number]bool{}
	for _, f := range boolFlags {
		bools[f.num] = *f.get(p)
	}
	newHooks := om.has(fHookSouth) || om.has(fHookEast)
	for num := protowire.Number(1); num <= fLastFlag; num++ {
		if v, ok := bools[num]; ok {
			b = putBool(b, num, v)
			continue
		}
		switch num {
		case fBank:
			if p.Ground {
				one(fBank, uint64(p.GroundSpeed))
			}
		case fWrite:
			if p.Writable {
				one(fWrite, uint64(p.MaxTextLength))
			}
		case fWriteOnce:
			if p.WritableOnce {
				one(fWriteOnce, uint64(p.MaxReadLength))
			}
		case fHook:
			if !newHooks && p.HookSouth {
				one(fHook, hookSouth)
			} else if !newHooks && p.HookEast {
				one(fHook, hookEast)
			}
		case fLight:
			if p.HasLight {
				sub(fLight, []protowire.Number{1, 2}, putUint(putUint(nil, 1, uint64(p.LightLevel)), 2, uint64(p.LightColor)))
			}
		case fShift:
			if p.HasOffset {
				sub(fShift, []protowire.Number{1, 2}, putInt(putInt(nil, 1, int32(p.OffsetX)), 2, int32(p.OffsetY)))
			}
		case fHeight:
			if p.HasElevation {
				one(fHeight, uint64(p.Elevation))
			}
		case fAutomap:
			if p.MiniMap {
				one(fAutomap, uint64(p.MiniMapColor))
			}
		case fLenshelp:
			if p.LensHelp {
				one(fLenshelp, uint64(p.LensHelpValue))
			}
		case fClothes:
			if p.Cloth {
				one(fClothes, uint64(p.ClothSlot))
			}
		case fDefaultAction:
			if p.HasDefaultAction {
				one(fDefaultAction, uint64(p.DefaultAction))
			}
		case fMarket:
			if p.IsMarket {
				m := p.Market
				f := putUint(nil, marketCategory, uint64(m.Category))
				f = putUint(f, marketTradeAs, uint64(m.TradeAs))
				f = putUint(f, marketShowAs, uint64(m.ShowAs))
				if m.RestrictLevel > 0 {
					f = putUint(f, marketMinLevel, uint64(m.RestrictLevel))
				}
				f = putBytes(f, marketName, []byte(m.Name))
				known := []protowire.Number{marketCategory, marketTradeAs, marketShowAs, marketMinLevel, marketName}
				// Unchanged vocations keep their original values (promoted, any).
				if m.RestrictProfession != professionMask(om.sub(fMarket).uints(marketVocation)) {
					known = append(known, marketVocation)
					for v := uint64(1); v <= 5; v++ {
						if m.RestrictProfession&(1<<(v-1)) != 0 {
							f = putUint(f, marketVocation, v)
						}
					}
				}
				sub(fMarket, known, f)
			}
		case fNpcSaleData:
			for _, n := range npc {
				b = putBytes(b, fNpcSaleData, encodeNpcSale(n))
			}
		case fChangedExpire:
			if p.ChangedToExpire {
				one(fChangedExpire, uint64(p.FormerObjectID))
			}
		case fCyclopedia:
			if p.Cyclopedia {
				one(fCyclopedia, uint64(p.CyclopediaType))
			}
		case fUpgrade:
			if p.HasUpgradeClassification {
				one(fUpgrade, uint64(p.UpgradeClassification))
			}
		}
	}
	// Both hooks, or a file that uses the newer hook flags.
	if newHooks {
		b = putBool(b, fHookSouth, p.HookSouth)
		b = putBool(b, fHookEast, p.HookEast)
	} else if p.HookSouth && p.HookEast {
		b = putBool(b, fHookEast, true)
	}
	return om.appendOthers(b, knownFlags...)
}
