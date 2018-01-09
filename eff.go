package bg

type effHeader struct {
	Signature, Version [4]byte
}
type effEffect struct {
	Signature, Version [4]byte
	EffectID           uint32
	TargetType         uint32
	SpellLevel         uint32
	EffectAmount       int32
	DWFlags            uint32
	DurationType       uint32
	Duration           uint32
	ProbabilityUpper   uint16
	ProbabilityLower   uint16
	Res                [8]byte
	NumDice            uint32
	DiceSize           uint32
	SavingThrow        uint32
	SaveMod            int32
	Special            uint32

	School          uint32
	JeremyIsAnIdiot uint32
	MinLevel        uint32
	MaxLevel        uint32
	Flags           uint32

	EffectAmount2 int32
	EffectAmount3 int32
	EffectAmount4 int32
	EffectAmound5 int32

	Res2 [8]byte
	Res3 [8]byte

	SourceX        int32
	SourceY        int32
	TargetX        int32
	TargetY        int32
	SourceType     uint32
	SourceRes      [8]byte
	SourceFlags    uint32
	ProjectileType uint32
	SlotNum        int32
	ScriptName     [32]byte
	CasterLevel    uint32
	FirstCall      uint32
	SecondaryType  uint32
	Pad            [15]uint32
}
