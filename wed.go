package bg

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"os"
)

type wedHeader struct {
	Signature, Version  [4]byte
	OverlayCount        uint32
	DoorCount           uint32
	OverlayOffset       uint32
	SecondHeaderOffset  uint32
	DoorOffset          uint32
	DoorTileCellsOffset uint32
}

type wedOverlay struct {
	Width                 uint16
	Height                uint16
	Name                  RESREF
	NumUniqueTiles        uint16
	LayerFlags            uint16
	TilemapOffset         uint32
	TileIndexLookupOffset uint32
}

type wedHeader2 struct {
	PolygonCount             uint32
	PolygonOffset            uint32
	VertexOffset             uint32
	WallGroupOffset          uint32
	PolygonIndexLookupOffset uint32
}

type wedDoor struct {
	Name                RESREF
	State               uint16
	DoorTileCellIndex   uint16
	DoorTileCellCount   uint16
	PolygonOpenCount    uint16
	PolygonClosedCount  uint16
	PolygonOpenOffset   uint32
	PolygonClosedOffset uint32
}

type wedTilemap struct {
	TileIndexLookupIndex uint16
	TileIndexLookupCount uint16
	AlternateTileIndex   int16
	Flags                byte
	AnimSpeed            byte
	WFlags               uint16
}

type wedWallGroup struct {
	PolygonIndex uint16
	PolygonCount uint16
}

type wedPolygon struct {
	VertexIndex uint32
	VertexCount uint32
	Mode        byte
	Height      byte
	MinX        int16
	MaxX        int16
	MinY        int16
	MaxY        int16
}

type wedVertex struct {
	X int16
	Y int16
}

type Wed struct {
	Header         wedHeader
	Overlays       []wedOverlay
	Header2        wedHeader2
	Doors          []wedDoor
	Tilemaps       [][]wedTilemap
	DoorTileCells  []uint16
	TileIndices    []uint16
	WallGroups     []wedWallGroup
	Polygons       []wedPolygon
	PolygonIndices []uint16
	Vertices       []wedVertex
}

func (wed *Wed) ToJson() (string, error) {
	bytes, err := json.MarshalIndent(wed, "", "\t")
	if err != nil {
		return "", err
	}

	return string(bytes[0:]), nil
}

func (wed *Wed) ToJsonWed() (*JsonWed, error) {
	jw := JsonWed{}
	jw.ImportOverlays(wed)
	jw.ImportDoors(wed)
	jw.ImportWalls(wed)

	return &jw, nil
}

func (wed *Wed) UpdateOffsets() {

	wed.Header.OverlayCount = uint32(len(wed.Overlays))
	wed.Header.DoorCount = uint32(len(wed.Doors))
	wed.Header.OverlayOffset = uint32(binary.Size(wedHeader{}))
	wed.Header.SecondHeaderOffset = uint32(int(wed.Header.OverlayOffset) + len(wed.Overlays)*binary.Size(wedOverlay{}))
	wed.Header.DoorOffset = uint32(int(wed.Header.SecondHeaderOffset) + binary.Size(wedHeader2{}))
	tmSize := 0
	for _, tm := range wed.Tilemaps {
		tmSize += binary.Size(tm)
	}
	wed.Header.DoorTileCellsOffset = uint32(int(wed.Header.DoorOffset) + binary.Size(wed.Doors) + tmSize)

	log.Printf("Header: %+v\n", wed.Header)

	wed.Header2.PolygonCount = uint32(len(wed.Doors))
	wed.Header2.WallGroupOffset = uint32(int(wed.Header.DoorTileCellsOffset) + binary.Size(wed.DoorTileCells) + binary.Size(wed.TileIndices))
	wed.Header2.PolygonOffset = uint32(int(wed.Header2.WallGroupOffset) + binary.Size(wed.WallGroups))
	wed.Header2.PolygonIndexLookupOffset = uint32(int(wed.Header2.PolygonOffset) + binary.Size(wed.Polygons))
	wed.Header2.VertexOffset = uint32(int(wed.Header2.PolygonIndexLookupOffset) + binary.Size(wed.PolygonIndices))
	log.Printf("Header2: %+v\n", wed.Header2)

	// Update overlay offsets
	tmOffset := int(wed.Header.DoorOffset) + binary.Size(wed.Doors)
	tmIndexOffset := int(wed.Header.DoorTileCellsOffset) + binary.Size(wed.DoorTileCells)

	for idx, _ := range wed.Overlays {
		overlay := &wed.Overlays[idx]
		overlay.TilemapOffset = uint32(tmOffset)
		overlay.TileIndexLookupOffset = uint32(tmIndexOffset)
		tmIndexCount := 0
		for _, tm := range wed.Tilemaps[idx] {
			tmIndexCount += int(tm.TileIndexLookupCount)
		}

		tmOffset += binary.Size(wed.Tilemaps[idx])
		tmIndexOffset += tmIndexCount * 2
	}

	//Update door offsets
	doorPolyOffset := int(wed.Header2.PolygonOffset) + binary.Size(wedPolygon{})*len(wed.Doors)
	for idx, _ := range wed.Doors {
		door := &wed.Doors[idx]

		door.PolygonOpenOffset = uint32(doorPolyOffset)
		doorPolyOffset += binary.Size(wedPolygon{}) * int(door.PolygonOpenCount)
		door.PolygonClosedOffset = uint32(doorPolyOffset)
		doorPolyOffset += binary.Size(wedPolygon{}) * int(door.PolygonClosedCount)
	}

}

func (wed *Wed) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, wed.Header); err != nil {
		return err
	}
	offset := binary.Size(wed.Header)
	log.Printf("Overlays at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.Overlays); err != nil {
		return err
	}
	offset += binary.Size(wed.Overlays)
	log.Printf("Header2 at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.Header2); err != nil {
		return err
	}
	offset += binary.Size(wed.Header2)
	log.Printf("Doors at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.Doors); err != nil {
		return err
	}
	offset += binary.Size(wed.Doors)
	log.Printf("Tilemaps at: %d %X\n", offset, offset)
	for _, tm := range wed.Tilemaps {
		if err := binary.Write(w, binary.LittleEndian, tm); err != nil {
			return err
		}
		offset += binary.Size(tm)
	}
	log.Printf("DoorTileCells at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.DoorTileCells); err != nil {
		return err
	}
	offset += binary.Size(wed.DoorTileCells)
	log.Printf("TileIndices at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.TileIndices); err != nil {
		return err
	}
	offset += binary.Size(wed.TileIndices)
	log.Printf("Wall Groups at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.WallGroups); err != nil {
		return err
	}
	offset += binary.Size(wed.WallGroups)
	log.Printf("Polygons at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.Polygons); err != nil {
		return err
	}
	offset += binary.Size(wed.Polygons)
	log.Printf("PolygonIndices at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.PolygonIndices); err != nil {
		return err
	}
	offset += binary.Size(wed.PolygonIndices)
	log.Printf("Vertices at: %d %X\n", offset, offset)
	if err := binary.Write(w, binary.LittleEndian, wed.Vertices); err != nil {
		return err
	}
	offset += binary.Size(wed.Vertices)
	log.Printf("EOF at: %d %X\n", offset, offset)
	return nil
}

func OpenWed(r io.ReadSeeker) (*Wed, error) {
	wed := Wed{}

	var err error

	if err = binary.Read(r, binary.LittleEndian, &wed.Header); err != nil {
		return nil, err
	}

	wed.Overlays = make([]wedOverlay, wed.Header.OverlayCount)
	wed.Doors = make([]wedDoor, wed.Header.DoorCount)

	r.Seek(int64(wed.Header.SecondHeaderOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.Header2); err != nil {
		return nil, err
	}

	wed.Polygons = make([]wedPolygon, wed.Header2.PolygonCount)

	r.Seek(int64(wed.Header.OverlayOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.Overlays); err != nil {
		return nil, err
	}

	r.Seek(int64(wed.Header.DoorOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.Doors); err != nil {
		return nil, err
	}

	r.Seek(int64(wed.Header2.PolygonOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.Polygons); err != nil {
		return nil, err
	}

	doorTileCells := 0
	for _, door := range wed.Doors {
		polygons := make([]wedPolygon, door.PolygonOpenCount)
		r.Seek(int64(door.PolygonOpenOffset), os.SEEK_SET)
		if err = binary.Read(r, binary.LittleEndian, &polygons); err != nil {
			return nil, err
		}
		wed.Polygons = append(wed.Polygons, polygons...)

		polygons = make([]wedPolygon, door.PolygonClosedCount)
		r.Seek(int64(door.PolygonClosedOffset), os.SEEK_SET)
		if err = binary.Read(r, binary.LittleEndian, &polygons); err != nil {
			return nil, err
		}
		wed.Polygons = append(wed.Polygons, polygons...)

		doorTileCells += int(door.DoorTileCellCount)
	}

	wed.DoorTileCells = make([]uint16, doorTileCells)
	r.Seek(int64(wed.Header.DoorTileCellsOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.DoorTileCells); err != nil {
		return nil, err
	}

	log.Printf("Polygons: %+v\n", wed.Polygons)
	vertexCount := 0
	for _, polygon := range wed.Polygons {
		vertexCount += int(polygon.VertexCount)
	}
	wed.Vertices = make([]wedVertex, vertexCount)

	r.Seek(int64(wed.Header2.VertexOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.Vertices); err != nil {
		return nil, err
	}

	wallGroupCount := int(wed.Overlays[0].Width*wed.Overlays[0].Height) / 75
	wed.WallGroups = make([]wedWallGroup, wallGroupCount)

	r.Seek(int64(wed.Header2.WallGroupOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.WallGroups); err != nil {
		return nil, err
	}

	polygonIndexCount := 0
	for _, wallgroup := range wed.WallGroups {
		polygonIndexCount += int(wallgroup.PolygonCount)
	}
	wed.PolygonIndices = make([]uint16, polygonIndexCount)

	r.Seek(int64(wed.Header2.PolygonIndexLookupOffset), os.SEEK_SET)
	if err = binary.Read(r, binary.LittleEndian, &wed.PolygonIndices); err != nil {
		return nil, err
	}

	wed.Tilemaps = make([][]wedTilemap, len(wed.Overlays))
	for idx, overlay := range wed.Overlays {
		r.Seek(int64(overlay.TilemapOffset), os.SEEK_SET)
		wed.Tilemaps[idx] = make([]wedTilemap, int(overlay.Width*overlay.Height))
		if err = binary.Read(r, binary.LittleEndian, &wed.Tilemaps[idx]); err != nil {
			return nil, err
		}
		tileIndexLookupCount := 0
		for _, tilemap := range wed.Tilemaps[idx] {
			tileIndexLookupCount += int(tilemap.TileIndexLookupCount)
		}
		tileindices := make([]uint16, tileIndexLookupCount)
		r.Seek(int64(overlay.TileIndexLookupOffset), os.SEEK_SET)
		if err = binary.Read(r, binary.LittleEndian, &tileindices); err != nil {
			return nil, err
		}
		wed.TileIndices = append(wed.TileIndices, tileindices...)

	}

	return &wed, nil
}
