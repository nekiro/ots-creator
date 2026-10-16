package assets

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/binio"
)

// boolFlags are the flags without payload.
var boolFlags = []struct {
	num protowire.Number
	get func(*thing.Properties) *bool
}{
	{2, func(props *thing.Properties) *bool { return &props.GroundBorder }},
	{3, func(props *thing.Properties) *bool { return &props.OnBottom }},
	{4, func(props *thing.Properties) *bool { return &props.OnTop }},
	{5, func(props *thing.Properties) *bool { return &props.Container }},
	{6, func(props *thing.Properties) *bool { return &props.Stackable }},
	{7, func(props *thing.Properties) *bool { return &props.Usable }},
	{8, func(props *thing.Properties) *bool { return &props.ForceUse }},
	{9, func(props *thing.Properties) *bool { return &props.MultiUse }},
	{12, func(props *thing.Properties) *bool { return &props.Fluid }},
	{13, func(props *thing.Properties) *bool { return &props.Unpassable }},
	{14, func(props *thing.Properties) *bool { return &props.Unmoveable }},
	{15, func(props *thing.Properties) *bool { return &props.BlockMissile }},
	{16, func(props *thing.Properties) *bool { return &props.BlockPathfind }},
	{17, func(props *thing.Properties) *bool { return &props.NoMoveAnimation }},
	{18, func(props *thing.Properties) *bool { return &props.Pickupable }},
	{19, func(props *thing.Properties) *bool { return &props.FluidContainer }},
	{20, func(props *thing.Properties) *bool { return &props.Hangable }},
	{22, func(props *thing.Properties) *bool { return &props.Rotatable }},
	{24, func(props *thing.Properties) *bool { return &props.DontHide }},
	{25, func(props *thing.Properties) *bool { return &props.Translucent }},
	{28, func(props *thing.Properties) *bool { return &props.LyingObject }},
	{29, func(props *thing.Properties) *bool { return &props.AnimateAlways }},
	{32, func(props *thing.Properties) *bool { return &props.FullGround }},
	{33, func(props *thing.Properties) *bool { return &props.IgnoreLook }},
	{37, func(props *thing.Properties) *bool { return &props.Wrappable }},
	{38, func(props *thing.Properties) *bool { return &props.Unwrappable }},
	{39, func(props *thing.Properties) *bool { return &props.TopEffect }},
	{42, func(props *thing.Properties) *bool { return &props.Corpse }},
	{43, func(props *thing.Properties) *bool { return &props.PlayerCorpse }},
	{45, func(props *thing.Properties) *bool { return &props.Ammo }},
	{46, func(props *thing.Properties) *bool { return &props.ShowOffSocket }},
	{47, func(props *thing.Properties) *bool { return &props.Reportable }},
	{49, func(props *thing.Properties) *bool { return &props.ReverseAddonsEast }},
	{50, func(props *thing.Properties) *bool { return &props.ReverseAddonsWest }},
	{51, func(props *thing.Properties) *bool { return &props.ReverseAddonsSouth }},
	{52, func(props *thing.Properties) *bool { return &props.ReverseAddonsNorth }},
	{53, func(props *thing.Properties) *bool { return &props.Wearout }},
	{54, func(props *thing.Properties) *bool { return &props.ClockExpire }},
	{55, func(props *thing.Properties) *bool { return &props.Expire }},
	{56, func(props *thing.Properties) *bool { return &props.ExpireStop }},
	{57, func(props *thing.Properties) *bool { return &props.WrapKit }},
}

// valueFlags are the flags whose payload is one number (payloadValue): the
// property that marks the flag set and the property holding the number.
var valueFlags = []struct {
	num    protowire.Number
	fields func(*thing.Properties) (set *bool, value *uint16)
}{
	{flagBank, func(props *thing.Properties) (*bool, *uint16) { return &props.Ground, &props.GroundSpeed }},
	{flagWrite, func(props *thing.Properties) (*bool, *uint16) { return &props.Writable, &props.MaxTextLength }},
	{flagWriteOnce, func(props *thing.Properties) (*bool, *uint16) { return &props.WritableOnce, &props.MaxReadLength }},
	{flagHeight, func(props *thing.Properties) (*bool, *uint16) { return &props.HasElevation, &props.Elevation }},
	{flagAutomap, func(props *thing.Properties) (*bool, *uint16) { return &props.MiniMap, &props.MiniMapColor }},
	{flagLensHelp, func(props *thing.Properties) (*bool, *uint16) { return &props.LensHelp, &props.LensHelpValue }},
	{flagClothes, func(props *thing.Properties) (*bool, *uint16) { return &props.Cloth, &props.ClothSlot }},
	{flagDefaultAction, func(props *thing.Properties) (*bool, *uint16) { return &props.HasDefaultAction, &props.DefaultAction }},
	{flagChangedToExpire, func(props *thing.Properties) (*bool, *uint16) { return &props.ChangedToExpire, &props.FormerObjectID }},
	{flagCyclopedia, func(props *thing.Properties) (*bool, *uint16) { return &props.Cyclopedia, &props.CyclopediaType }},
	{flagUpgradeClassification, func(props *thing.Properties) (*bool, *uint16) {
		return &props.HasUpgradeClassification, &props.UpgradeClassification
	}},
}

// boolFlagIndex and valueFlagIndex find the table entry of a flag number.
var boolFlagIndex, valueFlagIndex = func() (map[protowire.Number]int, map[protowire.Number]int) {
	bools := map[protowire.Number]int{}
	for i, flag := range boolFlags {
		bools[flag.num] = i
	}
	values := map[protowire.Number]int{}
	for i, flag := range valueFlags {
		values[flag.num] = i
	}
	return bools, values
}()

// professionMask turns PLAYER_PROFESSION values (knight 1 ... monk 5) into
// the vocation bit mask of thing.Market (knight 1, paladin 2, sorcerer 4,
// druid 8, monk 16). Any, none and promoted give no bit.
func professionMask(values []uint64) uint16 {
	var mask uint16
	for _, value := range values {
		if value >= 1 && value <= 5 {
			mask |= 1 << (value - 1)
		}
	}
	return mask
}

// knownFlags lists every flag number written by encodeFlags.
var knownFlags = func() []protowire.Number {
	out := []protowire.Number{flagHook, flagLight, flagShift, flagMarket, flagNpcSaleData, flagHookSouth, flagHookEast}
	for _, flag := range valueFlags {
		out = append(out, flag.num)
	}
	for _, flag := range boolFlags {
		out = append(out, flag.num)
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

// Unsupported lists the json names of properties set in props that
// appearances cannot store.
func Unsupported(props *thing.Properties) []string {
	var out []string
	if props.HasCharges {
		out = append(out, "hasCharges")
	}
	if props.FloorChange {
		out = append(out, "floorChange")
	}
	if props.HasBones {
		out = append(out, "hasBones")
	}
	return out
}

// clampUint16 limits a decoded number to the uint16 of the model.
func clampUint16(value uint64) uint16 { return uint16(min(value, math.MaxUint16)) }

// clampInt16 reads a proto int32 and limits it to the int16 of the model.
func clampInt16(value uint64) int16 {
	return int16(max(min(int64(int32(value)), math.MaxInt16), math.MinInt16))
}

func decodeFlags(flags message, props *thing.Properties) {
	for _, flag := range boolFlags {
		*flag.get(props) = flags.bool(flag.num)
	}
	for _, flag := range valueFlags {
		if flags.has(flag.num) {
			set, value := flag.fields(props)
			*set, *value = true, clampUint16(flags.sub(flag.num).uint(payloadValue))
		}
	}
	switch flags.sub(flagHook).uint(payloadValue) {
	case hookDirectionSouth:
		props.HookSouth = true
	case hookDirectionEast:
		props.HookEast = true
	}
	props.HookSouth = props.HookSouth || flags.bool(flagHookSouth)
	props.HookEast = props.HookEast || flags.bool(flagHookEast)
	if flags.has(flagLight) {
		light := flags.sub(flagLight)
		props.HasLight, props.LightLevel, props.LightColor = true, clampUint16(light.uint(lightBrightness)), clampUint16(light.uint(lightColor))
	}
	if flags.has(flagShift) {
		shift := flags.sub(flagShift)
		props.HasOffset, props.OffsetX, props.OffsetY = true, clampInt16(shift.uint(shiftX)), clampInt16(shift.uint(shiftY))
	}
	if flags.has(flagMarket) {
		market := flags.sub(flagMarket)
		name, _ := market.get(marketName)
		props.IsMarket = true
		props.Market = thing.Market{
			Category:      clampUint16(market.uint(marketCategory)),
			TradeAs:       clampUint16(market.uint(marketTradeAs)),
			ShowAs:        clampUint16(market.uint(marketShowAs)),
			Name:          string(name.payload),
			RestrictLevel: clampUint16(market.uint(marketMinLevel)),

			RestrictProfession: professionMask(market.uints(marketVocation)),
		}
	}
}

// decodeNpcSales reads the repeated NPC sale entries of the flags.
func decodeNpcSales(flags message) []thing.NpcSale {
	var out []thing.NpcSale
	for _, sale := range flags.all(flagNpcSaleData) {
		name, _ := sale.get(npcName)
		location, _ := sale.get(npcLocation)
		questFlag, _ := sale.get(npcQuestFlag)
		out = append(out, thing.NpcSale{
			Name:              binio.DecodeLatin1(name.payload),
			Location:          binio.DecodeLatin1(location.payload),
			SalePrice:         uint32(sale.uint(npcSalePrice)),
			BuyPrice:          uint32(sale.uint(npcBuyPrice)),
			CurrencyObjectID:  uint32(sale.uint(npcCurrency)),
			CurrencyQuestFlag: binio.DecodeLatin1(questFlag.payload),
		})
	}
	return out
}

func encodeNpcSale(sale thing.NpcSale) []byte {
	out := putBytes(nil, npcName, binio.EncodeLatin1(sale.Name))
	out = putBytes(out, npcLocation, binio.EncodeLatin1(sale.Location))
	out = putUint(out, npcSalePrice, uint64(sale.SalePrice))
	out = putUint(out, npcBuyPrice, uint64(sale.BuyPrice))
	if sale.CurrencyObjectID != 0 {
		out = putUint(out, npcCurrency, uint64(sale.CurrencyObjectID))
	}
	if sale.CurrencyQuestFlag != "" {
		out = putBytes(out, npcQuestFlag, binio.EncodeLatin1(sale.CurrencyQuestFlag))
	}
	return out
}

// flagWriter builds a flags message on top of the original flags.
type flagWriter struct {
	out      []byte
	original message
}

// sub writes a flag message from fields, keeping the original fields whose
// numbers are not in known.
func (w *flagWriter) sub(num protowire.Number, known []protowire.Number, fields []byte) {
	w.out = putBytes(w.out, num, w.original.sub(num).appendOthers(fields, known...))
}

// value writes a flag whose payload is one number.
func (w *flagWriter) value(num protowire.Number, value uint64) {
	w.sub(num, []protowire.Number{payloadValue}, putUint(nil, payloadValue, value))
}

// encodeFlags writes the flags of props and the NPC sale entries, in field
// number order. Fields of original (the original flags) that the model does
// not know are kept, also inside the flags it does.
func encodeFlags(props *thing.Properties, npcSales []thing.NpcSale, original message) []byte {
	writer := &flagWriter{original: original}
	newHooks := original.has(flagHookSouth) || original.has(flagHookEast)
	for num := protowire.Number(1); num <= flagLast; num++ {
		if i, ok := boolFlagIndex[num]; ok {
			writer.out = putBool(writer.out, num, *boolFlags[i].get(props))
			continue
		}
		if i, ok := valueFlagIndex[num]; ok {
			if set, value := valueFlags[i].fields(props); *set {
				writer.value(num, uint64(*value))
			}
			continue
		}
		switch num {
		case flagHook:
			if !newHooks && props.HookSouth {
				writer.value(flagHook, hookDirectionSouth)
			} else if !newHooks && props.HookEast {
				writer.value(flagHook, hookDirectionEast)
			}
		case flagLight:
			if props.HasLight {
				fields := putUint(nil, lightBrightness, uint64(props.LightLevel))
				fields = putUint(fields, lightColor, uint64(props.LightColor))
				writer.sub(flagLight, []protowire.Number{lightBrightness, lightColor}, fields)
			}
		case flagShift:
			if props.HasOffset {
				fields := putInt(nil, shiftX, int32(props.OffsetX))
				fields = putInt(fields, shiftY, int32(props.OffsetY))
				writer.sub(flagShift, []protowire.Number{shiftX, shiftY}, fields)
			}
		case flagMarket:
			if props.IsMarket {
				writer.market(props.Market)
			}
		case flagNpcSaleData:
			for _, sale := range npcSales {
				writer.out = putBytes(writer.out, flagNpcSaleData, encodeNpcSale(sale))
			}
		}
	}
	// Both hooks, or a file that uses the newer hook flags.
	if newHooks {
		writer.out = putBool(writer.out, flagHookSouth, props.HookSouth)
		writer.out = putBool(writer.out, flagHookEast, props.HookEast)
	} else if props.HookSouth && props.HookEast {
		writer.out = putBool(writer.out, flagHookEast, true)
	}
	return original.appendOthers(writer.out, knownFlags...)
}

func (w *flagWriter) market(market thing.Market) {
	fields := putUint(nil, marketCategory, uint64(market.Category))
	fields = putUint(fields, marketTradeAs, uint64(market.TradeAs))
	fields = putUint(fields, marketShowAs, uint64(market.ShowAs))
	if market.RestrictLevel > 0 {
		fields = putUint(fields, marketMinLevel, uint64(market.RestrictLevel))
	}
	fields = putBytes(fields, marketName, []byte(market.Name))
	known := []protowire.Number{marketCategory, marketTradeAs, marketShowAs, marketMinLevel, marketName}
	// Unchanged vocations keep their original values (promoted, any).
	if market.RestrictProfession != professionMask(w.original.sub(flagMarket).uints(marketVocation)) {
		known = append(known, marketVocation)
		for profession := uint64(1); profession <= 5; profession++ {
			if market.RestrictProfession&(1<<(profession-1)) != 0 {
				fields = putUint(fields, marketVocation, profession)
			}
		}
	}
	w.sub(flagMarket, known, fields)
}
