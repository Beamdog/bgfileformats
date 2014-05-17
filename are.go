package bg

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
)

type areaHeader struct {
	Signature, Version   [4]byte `json:"-"`
	AreaWed              RESREF
	LastSaved            uint32
	AreaFlags            uint32
	AreaNorth            RESREF
	AreaNorthFlags       uint32
	AreaEast             RESREF
	AreaEastFlags        uint32
	AreaSouth            RESREF
	AreaSouthFlags       uint32
	AreaWest             RESREF
	AreaWestFlags        uint32
	Areatype             uint16
	Rainprobability      uint16
	SnowProability       uint16
	FogProbability       uint16
	LightningProbability uint16
	WindSpeed            uint16
}

type areaFileOffsets struct {
	ActorsOffset            uint32
	ActorsCount             uint16
	RegionCount             uint16
	RegionOffset            uint32
	SpawnPointOffset        uint32
	SpawnPointCount         uint32
	EntranceOffset          uint32
	EntranceCount           uint32
	ContainerOffset         uint32
	ContainerCount          uint16
	ItemCount               uint16
	ItemOffset              uint32
	VertexOffset            uint32
	VertexCount             uint16
	AmbientCount            uint16
	AmbientOffset           uint32
	VariableOffset          uint32
	VariableCount           uint16
	TiledObjectFlagCount    uint16
	TiledObjectFlagOffset   uint32
	Script                  RESREF
	ExploredSize            uint32
	ExploredOffset          uint32
	DoorsCount              uint32
	DoorsOffset             uint32
	AnimationCount          uint32
	AnimationOffset         uint32
	TiledObjectCount        uint32
	TiledObjectOffset       uint32
	SongEntriesOffset       uint32
	RestInterruptionsOffset uint32
	AutomapOffset           uint32
	AutomapCount            uint32
	ProjectileTrapsOffset   uint32
	ProjectileTrapsCount    uint32
	RestMovieDay            RESREF
	RestMovieNight          RESREF
	Unknown                 [56]byte `json:"-"`
}

type areaActor struct {
	Name                LONGSTRING
	CurrentX            uint16
	CurrentY            uint16
	DestX               uint16
	DestY               uint16
	Flags               uint32
	Type                uint16
	FirstResSlot        byte
	AlignByte           byte `json:"-"`
	AnimationType       uint32
	Facing              uint16
	AlignWord           uint16 `json:"-"`
	ExpirationTime      uint32
	HuntingRange        uint16
	FollowRange         uint16
	TimeOfDayVisible    uint32
	NumberTimesTalkedTo uint32
	Dialog              RESREF
	OverrideScript      RESREF
	GeneralScript       RESREF
	ClassScript         RESREF
	RaceScript          RESREF
	DefaultScript       RESREF
	SpecificScript      RESREF
	CreatureData        RESREF
	CreatureOffset      uint32     `json:"-"`
	CreatureSize        uint32     `json:"-"`
	Unused              [32]uint32 `json:"-"`
}

type areaRegion struct {
	Name                    LONGSTRING
	Type                    uint16
	BoundingLeft            uint16
	BoundingTop             uint16
	BoundingRight           uint16
	BoundingBottom          uint16
	VertexCount             uint16
	VertexOffset            uint32
	TriggerValue            uint32
	CursorType              uint32
	Destination             RESREF
	EntranceName            LONGSTRING
	Flags                   uint32
	InformationText         uint32
	TrapDetectionDifficulty uint16
	TrapDisarmingDifficulty uint16
	TrapActivated           uint16
	TrapDetected            uint16
	TrapOriginX             uint16
	TrapOriginY             uint16
	KeyItem                 RESREF
	RegionScript            RESREF
	TransitionWalkToX       uint16
	TransitionWalkToY       uint16
	Unused                  [15]uint32 `json:"-"`
}

type areaSpawnPoint struct {
	Name                LONGSTRING
	CoordX              uint16
	CoordY              uint16
	RandomCreatures     [10]RESREF
	RandomCreatureCount uint16
	Difficulty          uint16
	SpawnRate           uint16
	Flags               uint16
	LifeSpan            uint32
	HuntingRange        uint32
	FollowRange         uint32
	MaxTypeNum          uint32
	Activated           uint16
	TimeOfDay           uint32
	ProbabilityDay      uint16
	ProbabilityNight    uint16
	Unused              [14]uint32 `json:"-"`
}

type areaEntrance struct {
	Name        LONGSTRING
	CoordX      uint16
	CoordY      uint16
	Orientation uint16
	Unused      [66]byte `json:"-"`
}

type areaContainer struct {
	Name                    LONGSTRING
	CoordX                  uint16
	CoordY                  uint16
	Type                    uint16
	LockDifficulty          uint16
	Flags                   uint32
	TrapDetectionDifficulty uint16
	TrapRemovalDifficulty   uint16
	ContainerTrapped        uint16
	TrapDetected            uint16
	TrapLaunchX             uint16
	TrapLaunchY             uint16
	BoundingTopLeft         uint16
	BoundingTopRight        uint16
	BoundingBottomRight     uint16
	BoundingBottomLeft      uint16
	ItemOffset              uint32
	ItemCount               uint32
	TrapScript              RESREF
	VertexOffset            uint32
	VertexCount             uint16
	TriggerRange            uint16
	OwnedBy                 LONGSTRING
	KeyType                 RESREF
	BreakDifficulty         uint32
	NotPickableString       uint32
	Unused                  [14]uint32 `json:"-"`
}

type areaItem struct {
	Resource   RESREF
	Expiration uint16
	UsageCount [3]uint16
	Flags      uint32
}

type areaVertex struct {
	Coordinate uint16
}

type areaAmbient struct {
	Name            LONGSTRING
	CoordinateX     uint16
	CoordinateY     uint16
	Range           uint16
	Alignment1      uint16 `json:"-"`
	PitchVariance   uint32
	VolumeVariance  uint16
	Volume          uint16
	Sounds          [10]RESREF
	SoundCount      uint16
	Alignment2      uint16 `json:"-"`
	Period          uint32
	PeriodVariance  uint32
	TimeOfDayActive uint32
	Flags           uint32
	Unused          [16]uint32 `json:"-"`
}

type areaVariable struct {
	Name       LONGSTRING
	Type       uint16
	ResRefType uint16
	DWValue    uint32
	IntValue   int32
	FloatValue float64
	ScriptName LONGSTRING
}

type areaDoor struct {
	Name                    LONGSTRING
	DoorID                  RESREF
	Flags                   uint32
	OpenDoorVertexOffset    uint32 `json:"-"`
	OpenDoorVertexCount     uint16 `json:"-"`
	ClosedDoorVertexCount   uint16 `json:"-"`
	CloseDoorVertexOffset   uint32 `json:"-"`
	OpenBoundingLeft        uint16
	OpenBoundingTop         uint16
	OpenBoundingRight       uint16
	OpenBoundingBottom      uint16
	ClosedBoundingLeft      uint16
	ClosedBoundingTop       uint16
	ClosedBoundingRight     uint16
	ClosedBoundingBottom    uint16
	OpenBlockVertexOffset   uint32 `json:"-"`
	OpenBlockVertexCount    uint16 `json:"-"`
	ClosedBlockVertexCount  uint16 `json:"-"`
	ClosedBlockVertexOffset uint32 `json:"-"`
	HitPoints               uint16
	ArmorClass              uint16
	OpenSound               RESREF
	ClosedSound             RESREF
	CursorType              uint32
	TrapDetectionDifficulty uint16
	TrapRemovalDifficulty   uint16
	DoorIsTrapped           uint16
	TrapDetected            uint16
	TrapLaunchTargetX       uint16
	TrapLaunchTargetY       uint16
	KeyItem                 RESREF
	DoorScript              RESREF
	DetectionDifficulty     uint32
	LockDifficulty          uint32
	WalkToX1                uint16
	WalkToY1                uint16
	WalkToX2                uint16
	WalkToY2                uint16
	NotPickableString       uint32
	TriggerName             LONGSTRING
	Unused                  [3]uint32 `json:"-"`
}

type areaAnimation struct {
	Name             LONGSTRING
	CoordX           uint16
	CoordY           uint16
	TimeOfDayVisible uint32
	Animation        RESREF
	BamSequence      uint16
	BamFrame         uint16
	Flags            uint32
	Height           int16
	Translucency     uint16
	StartFrameRane   uint16
	Probability      byte
	Period           byte
	Palette          RESREF
	Unused           uint32 `json:"-"`
}

type areaMapNote struct {
	CoordX uint16
	CoordY uint16
	Note   uint32
	Flags  uint32
	Id     uint32
	Unused [9]uint32
}

type areaTiledObject struct {
	Name                       LONGSTRING
	TileID                     RESREF
	Flags                      uint32
	PrimarySearchSquareStart   uint32
	PrimarySearchSquareCount   uint16
	SecondarySearchSquareCount uint16
	SecondarySearcHSquareStart uint32
	Unused                     [12]uint32 `json:"-"`
}

type areaProjectileTrap struct {
	Projectile        RESREF
	EffectBlockOffset uint32
	EffectBlockSize   uint16
	MissileId         uint16
	DelayCount        uint16
	RepetitionCount   uint16
	CoordX            uint16
	CoordY            uint16
	CoordZ            uint16
	TargetType        byte
	PortraitNum       byte
}

type areaSong struct {
	DaySong              uint32
	NightSong            uint32
	WinSong              uint32
	BattleSong           uint32
	LoseSong             uint32
	AltMusic0            uint32
	AltMusic1            uint32
	AltMusic2            uint32
	AltMusic3            uint32
	AltMusic4            uint32
	DayAmbient           RESREF
	DayAmbientExtended   RESREF
	DayAmbientVolume     uint32
	NightAmbient         RESREF
	NightAmbientExtended RESREF
	NightAmbientVolume   uint32
	Unused               [16]uint32 `json:"-"`
}

type areaRestEncounter struct {
	Name                 LONGSTRING
	RandomCreatureString [10]uint32
	RandomCreature       [10]RESREF
	RandomCreatureNum    uint16
	Difficulty           uint16
	LifeSpan             uint32
	HuntingRange         uint16
	FollowRange          uint16
	MaxTypeNum           uint16
	Activated            uint16
	ProbabilityDay       uint16
	ProbabilityNight     uint16
	Unused               [14]uint32 `json:"-"`
}

type Area struct {
	Header           areaHeader
	Offsets          areaFileOffsets `json:"-"`
	Actors           []areaActor
	Regions          []areaRegion
	SpawnPoints      []areaSpawnPoint
	Entrances        []areaEntrance
	Containers       []areaContainer
	Items            []areaItem
	Vertices         []areaVertex
	Ambients         []areaAmbient
	Variables        []areaVariable
	ExploredBitmask  []byte
	Doors            []areaDoor
	Animations       []areaAnimation
	MapNotes         []areaMapNote
	TiledObjects     []areaTiledObject
	Traps            []areaProjectileTrap
	Song             areaSong
	RestInterruption areaRestEncounter
}

func OpenArea(r io.ReadSeeker) (*Area, error) {
	area := Area{}

	err := binary.Read(r, binary.LittleEndian, &area.Header)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &area.Offsets)
	if err != nil {
		return nil, err
	}
	area.Actors = make([]areaActor, area.Offsets.ActorsCount)
	r.Seek(int64(area.Offsets.ActorsOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Actors)
	if err != nil {
		return nil, err
	}
	area.Regions = make([]areaRegion, area.Offsets.RegionCount)
	r.Seek(int64(area.Offsets.RegionOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Regions)
	if err != nil {
		return nil, err
	}
	area.SpawnPoints = make([]areaSpawnPoint, area.Offsets.SpawnPointCount)
	r.Seek(int64(area.Offsets.SpawnPointOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.SpawnPoints)
	if err != nil {
		return nil, err
	}
	area.Entrances = make([]areaEntrance, area.Offsets.EntranceCount)
	r.Seek(int64(area.Offsets.EntranceOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Entrances)
	if err != nil {
		return nil, err
	}
	area.Containers = make([]areaContainer, area.Offsets.ContainerCount)
	r.Seek(int64(area.Offsets.ContainerOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Containers)
	if err != nil {
		return nil, err
	}
	area.Items = make([]areaItem, area.Offsets.ItemCount)
	r.Seek(int64(area.Offsets.ItemOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Items)
	if err != nil {
		return nil, err
	}
	area.Vertices = make([]areaVertex, area.Offsets.VertexCount)
	r.Seek(int64(area.Offsets.VertexOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Vertices)
	if err != nil {
		return nil, err
	}
	area.Ambients = make([]areaAmbient, area.Offsets.AmbientCount)
	r.Seek(int64(area.Offsets.AmbientOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Ambients)
	if err != nil {
		return nil, err
	}
	area.Variables = make([]areaVariable, area.Offsets.VariableCount)
	r.Seek(int64(area.Offsets.VariableOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Variables)
	if err != nil {
		return nil, err
	}
	area.ExploredBitmask = make([]byte, area.Offsets.ExploredSize)
	r.Seek(int64(area.Offsets.VariableOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.ExploredBitmask)
	if err != nil {
		return nil, err
	}
	area.Doors = make([]areaDoor, area.Offsets.DoorsCount)
	r.Seek(int64(area.Offsets.DoorsOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Doors)
	if err != nil {
		return nil, err
	}
	area.Animations = make([]areaAnimation, area.Offsets.AnimationCount)
	r.Seek(int64(area.Offsets.AnimationOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Animations)
	if err != nil {
		return nil, err
	}
	area.MapNotes = make([]areaMapNote, area.Offsets.AutomapCount)
	r.Seek(int64(area.Offsets.AutomapOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.MapNotes)
	if err != nil {
		return nil, err
	}
	area.TiledObjects = make([]areaTiledObject, area.Offsets.TiledObjectCount)
	r.Seek(int64(area.Offsets.TiledObjectOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.TiledObjects)
	if err != nil {
		return nil, err
	}
	area.Traps = make([]areaProjectileTrap, area.Offsets.ProjectileTrapsCount)
	r.Seek(int64(area.Offsets.ProjectileTrapsOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Traps)
	if err != nil {
		return nil, err
	}
	r.Seek(int64(area.Offsets.SongEntriesOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.Song)
	if err != nil {
		return nil, err
	}
	r.Seek(int64(area.Offsets.RestInterruptionsOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &area.RestInterruption)
	if err != nil {
		return nil, err
	}

	return &area, nil
}

func (are *Area) WriteJson(w io.Writer) error {
	bytes, err := json.MarshalIndent(are, "", "\t")
	if err != nil {
		return err
	}

	_, err = w.Write(bytes)
	return err
}
