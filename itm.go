package bg

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type itmHeader struct {
	Signature, Version    [4]byte
	GenericName           uint32
	IdentifiedName        uint32
	UsedUpItemID          RESREF
	ItemFlags             uint32
	ItemType              uint16
	NotUsableBy           uint32
	AnimationType         uint16
	MinLevelRequired      uint16
	MinStrRequired        uint16
	MinStrBonusRequired   uint8
	NotUsableBy2a         uint8
	MinIntRequired        uint8
	NotUsableBy2b         uint8
	MinDexRequired        uint8
	NotUsableBy2c         uint8
	MinWisRequired        uint8
	NotUsableBy2d         uint8
	MinConRequired        uint8
	ProficiencyType       uint8
	MinChrRequired        uint16
	BaseValue             uint32
	MaxStackable          uint16
	ItemIcon              RESREF
	LoreValue             uint16
	GroundIcon            RESREF
	BaseWeight            uint32
	GenericDescription    uint32
	IdentifiedDescription uint32
	DescriptionPicture    RESREF
	Attributes            uint32
	AbilityOffset         uint32
	AbilityCount          uint16
	EffectsOffset         uint32
	EquipedStartingEffect uint16
	EquipedEffectCount    uint16
}

const (
	NUM_ATTACK_TYPES = 6
)

type itmAbility struct {
	Type                 uint16
	QuickSlotType        uint8
	LargeDamageDice      uint8
	QuickSlotIcon        RESREF
	ActionType           uint8
	ActionCount          uint8
	Range                uint16
	LauncherType         uint8
	LargeDamageDiceCount uint8
	SpeedFactor          uint8
	LargeDamageDiceBonus uint8
	Thac0Bonus           int16
	DamageDice           uint8
	School               uint8
	DamageDiceCount      uint8
	SecondaryType        uint8
	DamageDiceBonus      uint16
	DamageType           uint16
	EffectCount          uint16
	StartingEffect       uint16
	MaxUsageCount        uint16
	UsageFlags           uint16
	AbilityFlags         uint32
	MissileType          uint16
	AttackProbability    [NUM_ATTACK_TYPES]uint16
}

type ItmEffect struct {
	EffectID         uint16
	TargetType       uint8
	SpellLevel       uint8
	EffectAmount     int32
	Flags            uint32
	DurationType     uint16
	Duration         uint32
	ProbabilityUpper uint8
	ProbabilityLower uint8
	Res              RESREF
	NumDice          uint32
	DiceSize         uint32
	SavingThrow      uint32
	SaveMod          int32
	Special          uint32
}

type ITM struct {
	Header    itmHeader
	Abilities []itmAbility
	Effects   []ItmEffect
	Filename  string
}

func (itm *ITM) Write(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, itm.Header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, itm.Abilities)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, itm.Effects)
	if err != nil {
		return err
	}
	return nil
}

func (itm *ITM) Tp2Block(baseNum int) (string, []int) {
	stringIds := []int{
		int(itm.Header.GenericName),
		int(itm.Header.IdentifiedName),
		int(itm.Header.GenericDescription),
		int(itm.Header.IdentifiedDescription),
	}
	out := "COPY_EXISTING ~" + itm.Filename + "~ ~override~\n"
	if int(itm.Header.GenericName) >= 0 {
		out += fmt.Sprintf("\tSAY 0x0008 #%d\n", baseNum+int(itm.Header.GenericName))
	}
	if int(itm.Header.IdentifiedName) >= 0 {
		out += fmt.Sprintf("\tSAY 0x000C #%d\n", baseNum+int(itm.Header.IdentifiedName))
	}
	if int(itm.Header.GenericDescription) >= 0 {
		out += fmt.Sprintf("\tSAY 0x0050 #%d\n", baseNum+int(itm.Header.GenericDescription))
	}
	if int(itm.Header.IdentifiedDescription) >= 0 {
		out += fmt.Sprintf("\tSAY 0x0054 #%d\n", baseNum+int(itm.Header.IdentifiedDescription))
	}

	if int(itm.Header.GenericName) >= 0 {
		out += fmt.Sprintf("STRING_SET #%d = @%d\n", baseNum+int(itm.Header.GenericName), baseNum+int(itm.Header.GenericName))
	}
	if int(itm.Header.IdentifiedName) >= 0 {
		out += fmt.Sprintf("STRING_SET #%d = @%d\n", baseNum+int(itm.Header.IdentifiedName), baseNum+int(itm.Header.IdentifiedName))
	}
	if int(itm.Header.GenericDescription) >= 0 {
		out += fmt.Sprintf("STRING_SET #%d = @%d\n", baseNum+int(itm.Header.GenericDescription), baseNum+int(itm.Header.GenericDescription))
	}
	if int(itm.Header.IdentifiedDescription) >= 0 {
		out += fmt.Sprintf("STRING_SET #%d = @%d\n", baseNum+int(itm.Header.IdentifiedDescription), baseNum+int(itm.Header.IdentifiedDescription))
	}
	return out, stringIds

}

func (itm *ITM) Strings() map[string]int {
	names := map[string]int{}
	if int(itm.Header.GenericName) >= 0 {
		names["genericName"] = int(itm.Header.GenericName)
	}
	if int(itm.Header.IdentifiedName) >= 0 {
		names["identifiedName"] = int(itm.Header.IdentifiedName)
	}
	if int(itm.Header.GenericDescription) >= 0 {
		names["genericDescription"] = int(itm.Header.GenericDescription)
	}
	if int(itm.Header.IdentifiedDescription) >= 0 {
		names["identifiedDescription"] = int(itm.Header.IdentifiedDescription)
	}
	return names
}

func OpenITM(r io.ReadSeeker) (*ITM, error) {
	itm := &ITM{}

	err := binary.Read(r, binary.LittleEndian, &itm.Header)
	if err != nil {
		return nil, err
	}

	itm.Abilities = make([]itmAbility, itm.Header.AbilityCount)
	_, err = r.Seek(int64(itm.Header.AbilityOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &itm.Abilities)
	if err != nil {
		return nil, err
	}
	effectsCount := 0
	for _, ability := range itm.Abilities {
		effectsCount += int(ability.EffectCount)
	}
	effectsCount += int(itm.Header.EquipedEffectCount)
	itm.Effects = make([]ItmEffect, effectsCount)
	r.Seek(int64(itm.Header.EffectsOffset), os.SEEK_SET)
	binary.Read(r, binary.LittleEndian, &itm.Effects)

	return itm, nil
}
