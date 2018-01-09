package bg

import (
	"encoding/binary"
	"io"
	"os"
)

type splHeader struct {
	Signature, Version  [4]byte
	GenericName         uint32
	IdentifiedName      uint32
	UsedUpItemID        [8]byte
	ItemFlags           uint32
	ItemType            uint16
	NotUsableBy         uint32
	AnimationType       [2]uint8
	MinLevelRequired    uint8
	School              uint8
	MinStrRequired      uint8
	SecondaryType       uint8
	MinStrBonusRequired uint8
	NotUsableBy2a       uint8
	MinIntRequired      uint8
	NotUsableBy2b       uint8
	MinDexRequired      uint8
	NotUsableBy2c       uint8
	MinWisRequired      uint8
	NotUsableBy2d       uint8
	MinConRequired      uint16
	MinChrRequired      uint16

	SpellLevel            uint32
	MaxStackable          uint16
	ItemIcon              [8]byte
	LoreValue             uint16
	GroundIcon            [8]byte
	BaseWeight            uint32
	GenericDescription    uint32
	IdentifiedDescription uint32
	DescriptionPicture    [8]byte
	Attributes            uint32
	AbilityOffset         uint32
	AbilityCount          uint16
	EffectsOffset         uint32
	CastingStartingEffect uint16
	CastingEffectCount    uint16
}

type splAbility struct {
	Type            uint16
	QuickSlotType   uint16
	QuickSlotIcon   [8]byte
	ActionType      uint8
	ActionCount     uint8
	Range           uint16
	MinCasterLevel  uint16
	SpeedFactor     uint16
	TimesPerDay     uint16
	DamageDice      uint16
	DamageDiceCount uint16
	DamageDiceBonus uint16
	DamgeType       uint16
	EffectCount     uint16
	StartingEffect  uint16
	MaxUsageCount   uint16
	UsageFlags      uint16
	MissileType     uint16
}

type SPL struct {
	Header    splHeader
	Abilities []splAbility
	Effects   []ItmEffect
	Filename  string
}

func (spl *SPL) Write(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, spl.Header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, spl.Abilities)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, spl.Effects)
	if err != nil {
		return err
	}
	return nil
}

func OpenSPL(r io.ReadSeeker) (*SPL, error) {
	spl := SPL{}

	err := binary.Read(r, binary.LittleEndian, &spl.Header)
	if err != nil {
		return nil, err
	}

	spl.Abilities = make([]splAbility, spl.Header.AbilityCount)
	_, err = r.Seek(int64(spl.Header.AbilityOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &spl.Abilities)
	if err != nil {
		return nil, err
	}
	effectsCount := 0
	for _, ability := range spl.Abilities {
		effectsCount += int(ability.EffectCount)
	}
	effectsCount += int(spl.Header.CastingEffectCount)
	spl.Effects = make([]ItmEffect, effectsCount)
	r.Seek(int64(spl.Header.EffectsOffset), os.SEEK_SET)
	binary.Read(r, binary.LittleEndian, &spl.Effects)

	return &spl, nil
}
