package bg

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ajstarks/svgo"
	"image"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type JsonWed struct {
	Overlays []jsonWedOverlay
	Doors    []jsonWedDoor
	Walls    []jsonWedPolygon `json:"-"`
}

type jsonWedOverlay struct {
	Width   int
	Height  int
	Name    string
	Flags   int
	Tilemap []jsonWedTilemap `json:"-"`
	Stencil string `json:",omitempty"`
	Svg     string `json:",omitempty"`
}

type jsonWedTilemap struct {
	Id        int
	Count     int
	Alt       int
	Flags     int
	AnimSpeed int
	WFlags    int
}

type jsonWedDoor struct {
	Name           string
	State          int
	PolygonsOpen   []jsonWedPolygon `json:"-"`
	PolygonsClosed []jsonWedPolygon `json:"-"`
}

type jsonWedPolygon struct {
	Mode   int
	Height byte
	Verts  []image.Point
}

func (poly *jsonWedPolygon) SvgPath() string {
	out := ""
	for idx, v := range poly.Verts {
		if idx == 0 {
			out += fmt.Sprintf("M %d %d ", v.X, v.Y)
		} else {
			out += fmt.Sprintf("L %d %d ", v.X, v.Y)
		}
	}
	out += "z"
	return out
}

func (jw *JsonWed) ToJson() (string, error) {
	bytes, err := json.MarshalIndent(jw, "", "\t")
	if err != nil {
		return "", err
	}

	return string(bytes[0:]), nil
}

func (jw *JsonWed) Export(name string, dir string) error {
	wallSvgFile, err := os.Create(filepath.Join(dir, name) + ".svg")
	if err != nil {
		return err
	}
	defer wallSvgFile.Close()

	jsonFile, err := os.Create(filepath.Join(dir, name) + ".jwed")
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	jw.Overlays[0].Svg = name + ".svg"

	data, err := jw.ToJson()
	if err != nil {
		return err
	}
	jsonFile.WriteString(data)

	width := jw.Overlays[0].Width * 64
	height := jw.Overlays[0].Height * 64

	wallSvg := svg.New(wallSvgFile)
	wallSvg.Start(width, height)
	wallSvg.Gid("background")
	wallSvg.Image(0, 0, width, height, name+".png")
	wallSvg.Gend()
	wallSvg.Gid("walls")
	for _, wall := range jw.Walls {
		if len(wall.Verts) > 0 {
			wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, wall.SvgPath(), "fill:none;stroke:blue")))
			wallSvg.Desc(fmt.Sprintf("mode: %d", wall.Mode))
			wallSvg.Writer.Write([]byte("</path>"))
		}
	}
	wallSvg.Gend()
	for _, door := range jw.Doors {
		if len(door.PolygonsOpen) > 0 {
			wallSvg.Gid("door_open_" + door.Name)
			for _, poly := range door.PolygonsOpen {
				wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, poly.SvgPath(), "fill:auqa;stroke:aqua")))
				wallSvg.Desc(fmt.Sprintf("mode: %d", poly.Mode))
				wallSvg.Writer.Write([]byte("</path>"))
			}
			wallSvg.Gend()
		}

		if len(door.PolygonsClosed) > 0 {
			wallSvg.Gid("door_closed_" + door.Name)
			for _, poly := range door.PolygonsClosed {
				wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, poly.SvgPath(), "fill:none;stroke:red")))
				wallSvg.Desc(fmt.Sprintf("mode: %d", poly.Mode))
				wallSvg.Writer.Write([]byte("</path>"))
			}
			wallSvg.Gend()
		}
	}
	wallSvg.End()

	return nil
}

func (jw *JsonWed) ImportOverlays(wed *Wed) {
	jw.Overlays = make([]jsonWedOverlay, len(wed.Overlays))
	for idx, overlay := range wed.Overlays {
		jw.Overlays[idx].Width = int(overlay.Width)
		jw.Overlays[idx].Height = int(overlay.Height)
		jw.Overlays[idx].Name = overlay.Name.String()
		jw.Overlays[idx].Flags = int(overlay.LayerFlags)

		jw.Overlays[idx].Tilemap = make([]jsonWedTilemap, len(wed.Tilemaps[idx]))
		for tmIdx, tilemap := range wed.Tilemaps[idx] {
			jw.Overlays[idx].Tilemap[tmIdx].Id = int(tilemap.TileIndexLookupIndex)
			jw.Overlays[idx].Tilemap[tmIdx].Count = int(tilemap.TileIndexLookupCount)
			jw.Overlays[idx].Tilemap[tmIdx].Alt = int(tilemap.AlternateTileIndex)
			jw.Overlays[idx].Tilemap[tmIdx].Flags = int(tilemap.Flags)
			jw.Overlays[idx].Tilemap[tmIdx].AnimSpeed = int(tilemap.AnimSpeed)
			jw.Overlays[idx].Tilemap[tmIdx].WFlags = int(tilemap.WFlags)
		}
	}
}

func (jw *JsonWed) ImportDoors(wed *Wed) {
	jw.Doors = make([]jsonWedDoor, len(wed.Doors))

	for idx, door := range wed.Doors {
		jw.Doors[idx].ImportDoor(&door, wed)
	}
}

func (jdoor *jsonWedDoor) ImportDoor(door *wedDoor, wed *Wed) {
	jdoor.Name = door.Name.String()
	jdoor.State = int(door.State)

	jdoor.PolygonsOpen = make([]jsonWedPolygon, door.PolygonOpenCount)
	jdoor.PolygonsClosed = make([]jsonWedPolygon, door.PolygonClosedCount)

	polySize := binary.Size(&wedPolygon{})
	for idx := range jdoor.PolygonsOpen {
		polyIdx := int(door.PolygonOpenOffset-wed.Header2.PolygonOffset) / polySize
		jdoor.PolygonsOpen[idx].ImportPolygon(polyIdx, wed)
	}
	for idx := range jdoor.PolygonsClosed {
		polyIdx := int(door.PolygonClosedOffset-wed.Header2.PolygonOffset) / polySize
		jdoor.PolygonsClosed[idx].ImportPolygon(polyIdx, wed)
	}
}

func (poly *jsonWedPolygon) ImportPolygon(polyIdx int, wed *Wed) {
	poly.Mode = int(wed.Polygons[polyIdx].Mode)
	poly.Height = wed.Polygons[polyIdx].Height
	poly.Verts = make([]image.Point, wed.Polygons[polyIdx].VertexCount)
	for idx, vert := range wed.Vertices[int(wed.Polygons[polyIdx].VertexIndex) : int(wed.Polygons[polyIdx].VertexIndex)+len(poly.Verts)] {
		poly.Verts[idx].X = int(vert.X)
		poly.Verts[idx].Y = int(vert.Y)
	}
}

func (poly *jsonWedPolygon) BoundingBox() image.Rectangle {
	min := image.Point{math.MaxUint16, math.MaxUint16}
	max := image.Point{0, 0}

	for _, vert := range poly.Verts {
		if vert.X < min.X {
			min.X = int(vert.X)
		}
		if vert.Y < min.Y {
			min.Y = int(vert.Y)
		}
		if vert.X > max.X {
			max.X = int(vert.X)
		}
		if vert.Y > max.Y {
			max.Y = int(vert.Y)
		}
	}
	return image.Rectangle{min, max}.Canon()

}

func (poly *jsonWedPolygon) EffectedTiles(pt image.Point) []int {
	out := make([]int, 0)
	bb := poly.BoundingBox()
	bb.Min.X /= 64
	bb.Min.Y /= 64
	bb.Max.X /= 64
	bb.Max.Y /= 64
	pt.X /= 64
	pt.Y /= 64

	// All tiles in our bounding box should be considered effected
	for y := bb.Min.Y; y <= bb.Max.Y; y++ {
		for x := bb.Min.X; x <= bb.Max.X; x++ {
			out = append(out, y*pt.X+x)
		}
	}

	return out
}

func (poly *jsonWedPolygon) ToWedPoly() (wedPolygon, []wedVertex) {
	wedPoly := wedPolygon{}
	wedPoly.Mode = byte(poly.Mode)
	wedPoly.Height = byte(poly.Height)
	bb := poly.BoundingBox().Canon()
	wedPoly.MinX = int16(bb.Min.X)
	wedPoly.MinY = int16(bb.Min.Y)
	wedPoly.MaxX = int16(bb.Max.X)
	wedPoly.MaxY = int16(bb.Max.Y)

	verts := make([]wedVertex, len(poly.Verts))

	for idx, v := range poly.Verts {
		verts[idx].X = int16(v.X)
		verts[idx].Y = int16(v.Y)
	}

	return wedPoly, verts
}

func (door *jsonWedDoor) EffectedTiles(pt image.Point) []int {
	out := make(map[int]struct{})

	for _, poly := range door.PolygonsOpen {
		tiles := poly.EffectedTiles(pt)
		for _, val := range tiles {
			out[val] = struct{}{}
		}
	}
	for _, poly := range door.PolygonsClosed {
		tiles := poly.EffectedTiles(pt)
		for _, val := range tiles {
			out[val] = struct{}{}
		}
	}

	ints := make([]int, len(out))
	i := 0
	for key := range out {
		ints[i] = key
		i++
	}
	sort.Ints(ints)

	return ints
}

func (jw *JsonWed) ImportWalls(wed *Wed) {
	jw.Walls = make([]jsonWedPolygon, int(wed.Header2.PolygonCount))
	for idx := range jw.Walls {
		jw.Walls[idx].ImportPolygon(idx, wed)
	}
}

func (jw *JsonWed) GenerateWallPolys() ([]wedPolygon, []wedVertex, []uint16, []wedWallGroup) {
	wallPolys := make([]wedPolygon, 0)
	wallVerts := make([]wedVertex, 0)
	wallIndices := make([]uint16, 0)

	groups_x := int(math.Ceil(float64(jw.Overlays[0].Width) / 10.0))
	groups_y := int(math.Ceil(float64(jw.Overlays[0].Height) / 7.5))
	wallGroups := make([]wedWallGroup, groups_x*groups_y)

	polyIdx := 0
	vertIdx := 0

	for _, poly := range jw.Walls {
		wallPoly, verts := poly.ToWedPoly()
		if len(verts) > 0 {
			wallPoly.VertexIndex = uint32(vertIdx)
			wallPoly.VertexCount = uint32(len(verts))
			wallPolys = append(wallPolys, wallPoly)
			wallVerts = append(wallVerts, verts...)
			vertIdx += len(verts)
		}
	}

	for idx := range wallGroups {
		y := idx / groups_x
		x := idx - y*groups_x
		wg := &wallGroups[idx]
		bounds := image.Rect(x*640, y*480, x*640+640, y*480+480)
		wg.PolygonIndex = uint16(polyIdx)
		for wallIdx, poly := range wallPolys {
			bb := image.Rect(int(poly.MinX), int(poly.MinY), int(poly.MaxX), int(poly.MaxY))
			if bb.In(bounds) || bb.Overlaps(bounds) {
				wallIndices = append(wallIndices, uint16(wallIdx))
				wg.PolygonCount++
			}
		}
		polyIdx += int(wg.PolygonCount)
	}

	return wallPolys, wallVerts, wallIndices, wallGroups
}

func (jw *JsonWed) ToWed() (*Wed, error) {
	wed := Wed{}

	wed.Header = wedHeader{
		Signature:           [4]byte{'W', 'E', 'D', ' '},
		Version:             [4]byte{'V', '1', '.', '3'},
		OverlayCount:        uint32(len(jw.Overlays)),
		DoorCount:           uint32(len(jw.Doors)),
		OverlayOffset:       uint32(0),
		SecondHeaderOffset:  uint32(0),
		DoorOffset:          uint32(0),
		DoorTileCellsOffset: uint32(0),
	}

	wed.Overlays = make([]wedOverlay, len(jw.Overlays))
	wed.Tilemaps = make([][]wedTilemap, len(jw.Overlays))
	wed.TileIndices = make([]uint16, 0)
	for idx, overlay := range jw.Overlays {
		o := &wed.Overlays[idx]
		o.Width = uint16(overlay.Width)
		o.Height = uint16(overlay.Height)
		o.Name = NewResref(overlay.Name)
		o.NumUniqueTiles = 0
		o.LayerFlags = 0
		o.TileIndexLookupOffset = 0
		wed.Tilemaps[idx] = make([]wedTilemap, len(overlay.Tilemap))
		o.TilemapOffset = uint32(0)
		for i := 0; i < overlay.Width*overlay.Height; i++ {
			tm := &wed.Tilemaps[idx][i]
			tm.AlternateTileIndex = -1
			tm.TileIndexLookupIndex = uint16(i + 1)
			tm.TileIndexLookupCount = 1
			wed.TileIndices = append(wed.TileIndices, tm.TileIndexLookupIndex)
		}
	}

	wed.Polygons, wed.Vertices, wed.PolygonIndices, wed.WallGroups = jw.GenerateWallPolys()

	doorTileCellIndex := 0
	wed.Doors = make([]wedDoor, len(jw.Doors))
	wed.DoorTileCells = make([]uint16, 0)
	for idx, door := range jw.Doors {
		d := &wed.Doors[idx]
		d.Name = NewResref(door.Name)
		d.State = uint16(door.State)
		d.DoorTileCellIndex = uint16(doorTileCellIndex)
		effectedTiles := door.EffectedTiles(image.Point{int(wed.Overlays[0].Width * 64), int(wed.Overlays[0].Height * 64)})
		d.DoorTileCellCount = uint16(len(effectedTiles))
		doorTileCellIndex += len(effectedTiles)
		d.PolygonOpenCount = uint16(len(door.PolygonsOpen))
		d.PolygonClosedCount = uint16(len(door.PolygonsClosed))
		d.PolygonOpenOffset = uint32(0)
		d.PolygonClosedOffset = uint32(0)

		for _, poly := range door.PolygonsOpen {
			poly, verts := poly.ToWedPoly()
			wed.Polygons = append(wed.Polygons, poly)
			wed.Vertices = append(wed.Vertices, verts...)
		}
		for _, poly := range door.PolygonsClosed {
			poly, verts := poly.ToWedPoly()
			wed.Polygons = append(wed.Polygons, poly)
			wed.Vertices = append(wed.Vertices, verts...)
		}
		for _, tileId := range effectedTiles {
			wed.DoorTileCells = append(wed.DoorTileCells, uint16(tileId))
		}
	}

	wed.UpdateOffsets()

	return &wed, nil
}

func (jw *JsonWed) importSvg() error {
	doors := map[string]int{}
	for idx, door := range jw.Doors {
		doors[strings.ToLower(door.Name)] = idx
	}

	for _, overlay := range jw.Overlays {
		if overlay.Svg != "" {
			f, err := os.Open(overlay.Svg)
			if err != nil {
				return err
			}
			svg, err := OpenSVG(f)
			if err != nil {
				return err
			}
			for _, paths := range svg.Paths() {
				if strings.HasPrefix(paths.Name, "door_closed_") {
					doorName := strings.ToLower(strings.TrimPrefix(paths.Name, "door_closed_"))
					door := &jw.Doors[doors[doorName]]
					for _, poly := range paths.Polygons {
						door.PolygonsClosed = append(door.PolygonsClosed, poly.JsonPoly())
					}
				} else if strings.HasPrefix(paths.Name, "door_open_") {
					doorName := strings.ToLower(strings.TrimPrefix(paths.Name, "door_open_"))
					door := &jw.Doors[doors[doorName]]
					for _, poly := range paths.Polygons {
						door.PolygonsOpen = append(door.PolygonsOpen, poly.JsonPoly())
					}
				} else if strings.HasPrefix(paths.Name, "walls") {
					for _, poly := range paths.Polygons {
						jw.Walls = append(jw.Walls, poly.JsonPoly())
					}
				}
			}
		}
	}
	log.Printf("Door: %+v\n", jw.Doors[0])
	return nil
}

func (jw *JsonWed) importStencils() {
	for _, overlay := range jw.Overlays {

	}
	return nil
}

func OpenJWed(r io.ReadSeeker) (*JsonWed, error) {
	jwed := JsonWed{}
	jsonBlob, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(jsonBlob, &jwed); err != nil {
		return nil, err
	}

	if err = jwed.importSvg(); err != nil {
		return nil, err
	}

	if err = jwed.importStencils(); err != nil {
		return nil, err
	}

	return &jwed, nil
}

func CreateJWed(width int, height int, name string) (*JsonWed, error) {
	jwed := JsonWed{}
	jwed.Overlays = make([]jsonWedOverlay, 1)
	jwed.Overlays[0].Width = width
	jwed.Overlays[0].Height = height
	jwed.Overlays[0].Name = name

	return &jwed, nil

}

func test() {
	log.Printf("hihi")
}
