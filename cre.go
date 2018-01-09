package bg

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
)

type creHeader struct {
	Signature, Version           [4]byte
	Name                         uint32
	ApparentName                 uint32
	Flags                        uint32
	XPValue                      uint32
	XP                           uint32
	Gold                         uint32
	GeneralState                 uint32
	HitPoints                    uint16
	MaxHitPointsBase             uint16
	AnimationType                uint32
	Colors                       [7]uint8
	EffectVersion                uint8
	PortraitSmall                RESREF
	PortraitLarge                RESREF
	Reputation                   uint8
	HideInShadowsBase            uint8
	ArmorClass                   uint16
	ArmorClassBase               uint16
	ArmorClassCurshingAdjustment uint16
	ArmorClassMissileAdjustment  uint16
	ArmorClassPiercingAdjustment uint16
	ArmorClassSlashingAdjustment uint16
	ToHitArmoreClass0Base        int8
	NumberOfAttacksBase          uint8
	SaveVsDeathBase              uint8
	SaveVsWandsBase              uint8
	SaveVsPolyBase               uint8
	SaveVsBreathBase             uint8
	SaveVsSpellBase              uint8
	ResistFireBase               int8
	ResistColdBase               int8
	ResistElectricityBase        int8
	ResistAcidBase               int8
	ResistMagicBase              int8
	ResistMagicFireBase          int8
	ResistMagicColdBase          int8
	ResistSlashingBase           int8
	ResistCrushingBase           int8
	ResistPiercingBase           int8
	ResistMissileBase            int8
	DetectIllusionBase           uint8
	SetTrapsBase                 uint8
	LoreBase                     uint8
	LockPickingBase              uint8
	MoveSilentlyBase             uint8
	TrapsBase                    uint8
	PickPocketBase               uint8
	Fatigue                      uint8
	Intoxication                 uint8
	LuckBase                     int8
	Proficiencies                [20]int8
	UndeadLevel                  uint8
	TrackingBase                 uint8
	TrackingTarget               LONGSTRING
	Speech                       [100]uint32
	Level1                       uint8
	Level2                       uint8
	Level3                       uint8
	Sex                          uint8
	STRBase                      uint8
	STRExtraBase                 uint8
	INTBase                      uint8
	WISBase                      uint8
	DEXBase                      uint8
	CONBase                      uint8
	CHRBase                      uint8
	Morale                       uint8
	MoraleBreak                  uint8
	HatedRace                    uint8
	MoraleRecoveryTime           uint16
	MageSpecUpperWorld           uint16
	MageSpecialization           uint16

	ScriptOverride RESREF
	ScriptClass    RESREF
	ScriptRace     RESREF
	ScriptGeneral  RESREF
	ScriptDefault  RESREF
}

type creOffsets struct {
	EnemyAlly                   uint8
	General                     uint8
	Race                        uint8
	Class                       uint8
	Specifics                   uint8
	Gender                      uint8
	SpecialCase                 [5]uint8
	Alignment                   uint8
	Instance                    uint32
	Name                        LONGSTRING
	KnownSpellListOffset        uint32
	KnownSpellListCount         uint32
	MemorizationLevelListOffset uint32
	MemorizationLevelListCount  uint32
	MemorizationSpellListOffset uint32
	MemorizationSpellListCount  uint32
	EquipmentListOffset         uint32
	ItemListOffset              uint32
	ItemListCount               uint32
	EffectListOffset            uint32
	EffectListCount             uint32
	Dialog                      RESREF
}

type creKnownSpell struct {
	KnownSpellID RESREF
	SpellLevel   uint16
	MagicType    uint16
}

type creMemorizedSpellLevel struct {
	SpellLevel             uint16
	BaseCount              uint16
	Count                  uint16
	MagicType              uint16
	MemorizedStartingSpell uint32
	MemorizedCount         uint32
}

type creMemorizedSpell struct {
	SpellID   RESREF
	Flags     uint16
	Alignment [2]uint8
}

type creItem struct {
	ItemID       RESREF
	Wear         uint16
	UsageCount   [3]uint16
	DynamicFlags uint32
}

type creEquipment struct {
	HelmetItem            uint16
	ArmorItem             uint16
	ShieldItem            uint16
	GauntletsItem         uint16
	RingLeftItem          uint16
	RingRightItem         uint16
	AmuletItem            uint16
	BeltItem              uint16
	BootsItem             uint16
	WeaponItem            [4]uint16
	AmmoItem              [4]uint16
	CloakItem             uint16
	MiscItem              [20]uint16
	SelectedWeapon        uint16
	SelectedWeaponAbility uint16
}

type CRE struct {
	Header               creHeader
	Offsets              creOffsets
	KnownSpells          []creKnownSpell
	MemorizedSpellLevels []creMemorizedSpellLevel
	MemorizedSpells      []creMemorizedSpell
	Effects              []ItmEffect
	Effectsv2            []effEffect
	Items                []creItem
	Equipment            creEquipment
}

func (cre *CRE) Write(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, cre.Header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, cre.Offsets)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, cre.KnownSpells)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, cre.MemorizedSpellLevels)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, cre.MemorizedSpells)
	if err != nil {
		return err
	}
	if cre.Header.EffectVersion == 0 {
		err = binary.Write(w, binary.LittleEndian, cre.Effects)
		if err != nil {
			return err
		}
	} else if cre.Header.EffectVersion == 1 {
		err = binary.Write(w, binary.LittleEndian, cre.Effectsv2)
		if err != nil {
			return err
		}
	}
	err = binary.Write(w, binary.LittleEndian, cre.Items)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, cre.Equipment)
	if err != nil {
		return err
	}

	return nil
}

func OpenCre(r io.ReadSeeker) (*CRE, error) {
	cre := &CRE{}

	err := binary.Read(r, binary.LittleEndian, &cre.Header)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.LittleEndian, &cre.Offsets)
	if err != nil {
		return nil, err
	}

	cre.KnownSpells = make([]creKnownSpell, cre.Offsets.KnownSpellListCount)
	_, err = r.Seek(int64(cre.Offsets.KnownSpellListOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &cre.KnownSpells)
	if err != nil {
		return nil, err
	}
	cre.MemorizedSpellLevels = make([]creMemorizedSpellLevel, cre.Offsets.MemorizationLevelListCount)
	_, err = r.Seek(int64(cre.Offsets.MemorizationLevelListOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &cre.MemorizedSpellLevels)
	if err != nil {
		return nil, err
	}
	cre.MemorizedSpells = make([]creMemorizedSpell, cre.Offsets.MemorizationSpellListCount)
	_, err = r.Seek(int64(cre.Offsets.MemorizationSpellListOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &cre.MemorizedSpells)
	if err != nil {
		return nil, err
	}
	cre.Items = make([]creItem, cre.Offsets.ItemListCount)
	_, err = r.Seek(int64(cre.Offsets.ItemListOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &cre.Items)
	if err != nil {
		return nil, err
	}
	_, err = r.Seek(int64(cre.Offsets.EquipmentListOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &cre.Equipment)
	if err != nil {
		return nil, err
	}
	if cre.Header.EffectVersion == 0 {
		cre.Effects = make([]ItmEffect, cre.Offsets.EffectListCount)
		_, err = r.Seek(int64(cre.Offsets.EffectListOffset), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		err = binary.Read(r, binary.LittleEndian, &cre.Effects)
		if err != nil {
			return nil, err
		}
	} else if cre.Header.EffectVersion == 1 {
		cre.Effectsv2 = make([]effEffect, cre.Offsets.EffectListCount)
		_, err = r.Seek(int64(cre.Offsets.EffectListOffset), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		err = binary.Read(r, binary.LittleEndian, &cre.Effectsv2)
		if err != nil {
			return nil, err
		}

	}
	return cre, nil
}
func (cre *CRE) WriteJson(w io.Writer) error {
	bytes, err := json.MarshalIndent(cre, "", "\t")
	if err != nil {
		return err
	}

	_, err = w.Write(bytes)
	return err
}
