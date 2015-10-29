// +build darwin

package bg

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ajstarks/svgo"
)

type JsonWed struct {
	Overlays    []jsonWedOverlay
	Doors       []jsonWedDoor
	Walls       []jsonWedPolygon `json:"-"`
	TileIndices []int            `json:"-"`
}

type jsonWedOverlay struct {
	Width         int
	Height        int
	Name          string
	Flags         int
	Animations    []jsonWedAnimation `json:",omitempty"`
	Tilemap       []jsonWedTilemap   `json:"-"`
	Stencils      []string           `json:",omitempty"`
	Svg           string             `json:",omitempty"`
	BackgroundImg image.Image        `json:"-"`
	ClosedImage   image.Image        `json:"-"`
	StencilImages []image.Image      `json:"-"`
	Tis           *Tis               `json:"-"`
}

type jsonWedTilemap struct {
	Id        int
	Count     int
	Alt       int
	Flags     int
	AnimSpeed int
	WFlags    int
}

type jsonWedAnimation struct {
	X    int
	Y    int
	Name string
	img  image.Image
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

func (o *jsonWedOverlay) PixelWidth() int {
	return o.Width * 64
}
func (o *jsonWedOverlay) PixelHeight() int {
	return o.Height * 64
}

func (o *jsonWedOverlay) TileImage(x int, y int) *image.RGBA {
	return o.tileImage(x, y, o.BackgroundImg)
}

func (o *jsonWedOverlay) ClosedTileImage(x int, y int) *image.RGBA {
	return o.tileImage(x, y, o.ClosedImage)
}

func (o *jsonWedOverlay) tileImage(x int, y int, imgIn image.Image) *image.RGBA {
	rect := image.Rect(x*64, y*64, x*64+64, y*64+64)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	//trans := color.RGBA{0, 255, 0, 255}

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := imgIn.At(x, y)
			img.Set(x-rect.Min.X, y-rect.Min.Y, c)
		}
	}

	return img
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
		return fmt.Errorf("Unable to open svg: %v", err)
	}
	defer wallSvgFile.Close()

	jsonFile, err := os.Create(filepath.Join(dir, name) + ".jwed")
	if err != nil {
		return fmt.Errorf("unable to create jwed: %v", err)
	}
	defer jsonFile.Close()

	jw.Overlays[0].Svg = name + ".svg"

	width := jw.Overlays[0].PixelWidth()
	height := jw.Overlays[0].PixelHeight()

	wallSvg := svg.New(wallSvgFile)
	wallSvg.Start(width, height, "xmlns:inkscape=\"http://www.inkscape.org/namespaces/inkscape\"")
	wallSvg.Group("id=\"background\" inkscape:groupmode=\"layer\"")
	wallSvg.Image(0, 0, width, height, name+".png")
	wallSvg.Gend()
	for _, wall := range jw.Walls {
		if len(wall.Verts) > 0 {
			wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, wall.SvgPath(), "fill:none;stroke:blue")))
			wallSvg.Desc(fmt.Sprintf("mode: %d", wall.Mode))
			wallSvg.Writer.Write([]byte("</path>"))
		}
	}
	for _, door := range jw.Doors {
		if len(door.PolygonsOpen) > 0 {
			wallSvg.Group("id=\"door_open_" + door.Name + "\" inkscape:groupmode=\"layer\"")
			for _, poly := range door.PolygonsOpen {
				wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, poly.SvgPath(), "fill:auqa;stroke:aqua")))
				wallSvg.Desc(fmt.Sprintf("mode: %d", poly.Mode))
				wallSvg.Writer.Write([]byte("</path>"))
			}
			wallSvg.Gend()
		}

		if len(door.PolygonsClosed) > 0 {
			wallSvg.Group("id=\"door_closed_" + door.Name + "\" inkscape:groupmode=\"layer\"")
			for _, poly := range door.PolygonsClosed {
				wallSvg.Writer.Write([]byte(fmt.Sprintf(`<path d="%s" style="%s">`, poly.SvgPath(), "fill:none;stroke:red")))
				wallSvg.Desc(fmt.Sprintf("mode: %d", poly.Mode))
				wallSvg.Writer.Write([]byte("</path>"))
			}
			wallSvg.Gend()
		}
	}
	wallSvg.End()

	for _, ov := range jw.Overlays {
		fileName := fmt.Sprintf("%s.png", ov.Name)
		f, err := os.Create(filepath.Join(dir, fileName))
		if err != nil {
			return err
		}
		png.Encode(f, ov.BackgroundImg)
		f.Close()
	}

	for overlayIdx, _ := range jw.Overlays[1:] {
		overlay := &jw.Overlays[1+overlayIdx]
		if overlay.Width > 0 && overlay.Height > 0 {
			tileList := make([]jsonWedTilemap, 0)
			for _, tm := range jw.Overlays[0].Tilemap {
				if tm.Flags&(1<<uint(overlayIdx+1)) == (1 << uint(overlayIdx+1)) {
					tileList = append(tileList, tm)
				} else if tm.Flags != 0 {
					log.Printf("Idx: %d Flags: %d\n", overlayIdx+1, tm.Flags)
				}
			}

			img := image.NewRGBA(image.Rect(0, 0, jw.Overlays[0].PixelWidth(), jw.Overlays[0].PixelHeight()))
			tileBounds := image.Rect(0, 0, 64, 64)

			for _, tm := range tileList {
				tileId := 0
				if tm.Alt != -1 {
					tileId = tm.Alt
				} else {
					tileId = jw.TileIndices[tm.Id]
				}
				tileImg := jw.Overlays[0].Tis.SubImage(tileId)
				X, Y := 64*(jw.TileIndices[tm.Id]%jw.Overlays[0].Width), 64*(jw.TileIndices[tm.Id]/jw.Overlays[0].Width)

				filter := true
				if filter {
					for y := tileBounds.Min.Y; y < tileBounds.Max.Y; y++ {
						for x := tileBounds.Min.X; x < tileBounds.Max.X; x++ {
							r, g, b, _ := tileImg.At(x, y).RGBA()
							if r == 0 && g == 0 && b == 0 {
								tileImg.Set(x, y, color.RGBA{0, 0, 0, 0})
							}
							if r == 0 && g == 65535 && b == 0 {
								tapCount := 0
								cr, cg, cb := 0, 0, 0
								taps := []image.Point{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}}
								for _, tap := range taps {
									if tap.In(tileBounds) {
										tapCount++
										tr, tg, tb, _ := tileImg.At(tap.X, tap.Y).RGBA()
										cr += int(tr)
										cg += int(tg)
										cb += int(tb)
									}
								}
								divider := tapCount * 0xFF
								tileImg.Set(x, y, color.RGBA{uint8(cr / divider), uint8(cg / divider), uint8(cb / divider), 255})
							}
						}
					}
				}
				draw.Draw(img, image.Rect(X, Y, X+64, Y+64), tileImg, image.Pt(0, 0), draw.Src)
			}
			stencilName := fmt.Sprintf("%s_overlay_%d.png", name, overlayIdx+1)
			f, err := os.Create(filepath.Join(dir, stencilName))
			if err != nil {
				return err
			}
			png.Encode(f, img)

			f.Close()
			overlay.Stencils = append(overlay.Stencils, fmt.Sprintf("%s_overlay_%d", name, overlayIdx+1))

		}
	}

	if len(jw.Doors) > 0 {
		img := image.NewRGBA(image.Rect(0, 0, jw.Overlays[0].PixelWidth(), jw.Overlays[0].PixelHeight()))
		for _, door := range jw.Doors {
			tiles := door.EffectedTiles(image.Point{jw.Overlays[0].PixelWidth(), jw.Overlays[0].PixelHeight()}, &jw.Overlays[0])
			for _, tile := range tiles {
				X, Y := 64*(tile%jw.Overlays[0].Width), 64*(tile/jw.Overlays[0].Width)
				tileBounds := image.Rect(0, 0, 64, 64)
				tm := jw.Overlays[0].Tilemap[tile]
				tileId := 0
				if tm.Alt != -1 {
					tileId = tm.Alt
				} else {
					tileId = jw.TileIndices[tm.Id]
				}

				tileImg := jw.Overlays[0].Tis.SubImage(tileId)
				for y := tileBounds.Min.Y; y < tileBounds.Max.Y; y++ {
					for x := tileBounds.Min.X; x < tileBounds.Max.X; x++ {
						r, g, b, _ := tileImg.At(x, y).RGBA()
						if r == 0 && g == 0 && b == 0 {
							tileImg.Set(x, y, color.RGBA{0, 0, 0, 0})
						}
					}
				}

				draw.Draw(img, image.Rect(X, Y, X+64, Y+64), tileImg, image.Pt(0, 0), draw.Src)
			}
		}
		closedName := fmt.Sprintf("%sc.png", name)
		f, err := os.Create(filepath.Join(dir, closedName))
		if err != nil {
			return err
		}
		png.Encode(f, img)
		f.Close()
	}

	data, err := jw.ToJson()
	if err != nil {
		return err
	}
	jsonFile.WriteString(data)
	return nil
}

func (jw *JsonWed) ImportOverlays(wed *Wed) error {
	jw.Overlays = make([]jsonWedOverlay, 0)
	jw.TileIndices = make([]int, len(wed.TileIndices))
	for idx, overlay := range wed.Overlays {
		if overlay.Name.String() != "" {
			ov := jsonWedOverlay{}
			ov.Width = int(overlay.Width)
			ov.Height = int(overlay.Height)
			ov.Name = overlay.Name.String()
			ov.Flags = int(overlay.LayerFlags)

			ov.Tilemap = make([]jsonWedTilemap, len(wed.Tilemaps[idx]))
			for tmIdx, tilemap := range wed.Tilemaps[idx] {
				ov.Tilemap[tmIdx].Id = int(tilemap.TileIndexLookupIndex)
				ov.Tilemap[tmIdx].Count = int(tilemap.TileIndexLookupCount)
				ov.Tilemap[tmIdx].Alt = int(tilemap.AlternateTileIndex)
				ov.Tilemap[tmIdx].Flags = int(tilemap.Flags)
				ov.Tilemap[tmIdx].AnimSpeed = int(tilemap.AnimSpeed)
				ov.Tilemap[tmIdx].WFlags = int(tilemap.WFlags)
			}

			tisFile, err := os.Open(overlay.Name.String() + ".tis")
			if err != nil {
				return fmt.Errorf("unable to open overlay: %s %v", overlay.Name.String(), err)
			}
			defer tisFile.Close()

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get working directory: %v", err)
			}
			tis, err := OpenTis(tisFile, overlay.Name.String(), cwd)
			if err != nil {
				return fmt.Errorf("unable to open tis: %v", err)
			}
			ov.Tis = tis
			img := image.NewRGBA(image.Rect(0, 0, 64*ov.Width, 64*ov.Height))
			closedimg := image.NewRGBA(image.Rect(0, 0, 64*ov.Width, 64*ov.Height))
			for y := 0; y < int(ov.Height); y++ {
				for x := 0; x < int(ov.Width); x++ {
					tileNum := y*int(ov.Width) + x
					tileImg := tis.SubImage(tileNum)
					draw.Draw(img, image.Rect(x*64, y*64, x*64+64, y*64+64), tileImg, image.Pt(0, 0), draw.Src)
					if ov.Tilemap[tileNum].Alt != -1 {
						tileImg = tis.SubImage(ov.Tilemap[tileNum].Alt)
						draw.Draw(closedimg, image.Rect(x*64, y*64, x*64+64, y*64+64), tileImg, image.Pt(0, 0), draw.Src)
					}
				}
			}
			ov.BackgroundImg = img
			ov.ClosedImage = closedimg

			jw.Overlays = append(jw.Overlays, ov)
		}
	}
	for idx, ti := range wed.TileIndices {
		jw.TileIndices[idx] = int(ti)
	}
	return nil
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
	wedPoly.VertexCount = uint32(len(poly.Verts))

	return wedPoly, verts
}

func same_color(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()

	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func (o *jsonWedOverlay) TilesDiffer(tileId int) bool {
	y := tileId / o.Width
	x := tileId % o.Width

	bounds := image.Rect(x*64, y*64, x*64+64, y*64+64)

	for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
		for x := bounds.Min.X; x <= bounds.Max.X; x++ {
			if !same_color(o.ClosedImage.At(x, y), o.BackgroundImg.At(x, y)) {
				r1, g1, b1, a1 := o.ClosedImage.At(x, y).RGBA()
				r2, g2, b2, a2 := o.BackgroundImg.At(x, y).RGBA()
				log.Printf("C1[ %d, %d, %d, %d ], C2[ %d, %d, %d, %d]\n", r1, g1, b1, a1, r2, g2, b2, a2)
				return true
			}
		}
	}

	return false
}

func (door *jsonWedDoor) EffectedTiles(pt image.Point, overlay *jsonWedOverlay) []int {
	out := make(map[int]bool)

	for _, poly := range door.PolygonsOpen {
		tiles := poly.EffectedTiles(pt)
		for _, val := range tiles {
			if overlay.TilesDiffer(val) {
				out[val] = true
			}
		}
	}
	for _, poly := range door.PolygonsClosed {
		tiles := poly.EffectedTiles(pt)
		for _, val := range tiles {
			if overlay.TilesDiffer(val) {
				out[val] = true
			}
		}
	}
	log.Printf("Tiles: %+v\n", out)

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

func (o *jsonWedOverlay) GenerateTiles(x int, y int, closed bool) ([]*image.RGBA, *image.RGBA, int) {
	bounds := image.Rect(x*64, y*64, x*64+64, y*64+64)
	var images []*image.RGBA
	var stencilImg *image.RGBA
	flags := 0

	stencilId := -1
	alphaCount := 0
	// Dont try to check for stencils for closed images
	if !closed {
		for idx, stencil := range o.StencilImages {
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					_, _, _, a := stencil.At(x, y).RGBA()
					if a != 0 {
						stencilId = idx
						alphaCount++
					}
				}
			}
		}
	}

	if stencilId >= 0 && alphaCount != 4096 {
		images = make([]*image.RGBA, 1)
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		if closed {
			draw.Draw(img, image.Rect(0, 0, 64, 64), o.ClosedImage, bounds.Min, draw.Src)
		} else {
			draw.Draw(img, image.Rect(0, 0, 64, 64), o.BackgroundImg, bounds.Min, draw.Src)
		}
		images[0] = img
		stencilImg = image.NewRGBA(image.Rect(0, 0, 64, 64))
		sBounds := stencilImg.Bounds()
		for y := sBounds.Min.Y; y < sBounds.Max.Y; y++ {
			for x := sBounds.Min.X; x < sBounds.Max.X; x++ {
				c := o.StencilImages[stencilId].At(bounds.Min.X+x, bounds.Min.Y+y)
				_, _, _, a := c.RGBA()
				if a != 0 {
					stencilImg.Set(x, y, color.RGBA{0, 255, 0, 255})
				} else {
					stencilImg.Set(x, y, color.RGBA{0, 255, 0, 255})
					stencilImg.Set(x, y, images[0].At(x, y))
				}
			}
		}
		flags = (1 << (uint(stencilId) + 1))

	} else {
		images = make([]*image.RGBA, 1)
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		if closed {
			draw.Draw(img, image.Rect(0, 0, 64, 64), o.ClosedImage, bounds.Min, draw.Src)
		} else {
			draw.Draw(img, image.Rect(0, 0, 64, 64), o.BackgroundImg, bounds.Min, draw.Src)
		}
		images[0] = img
		stencilImg = nil

		// Fully transparent water section
		if stencilId >= 0 && alphaCount == 4096 {
			flags = (1 << (uint(stencilId) + 1))
		} else {
			flags = 0
		}
	}

	for _, anim := range o.Animations {
		if anim.X == x && anim.Y == y {
			bounds := anim.img.Bounds()
			frames := (bounds.Max.X - bounds.Min.X) / 64
			images = make([]*image.RGBA, 0)
			for i := 0; i < frames; i++ {
				img := image.NewRGBA(image.Rect(0, 0, 64, 64))
				draw.Draw(img, image.Rect(0, 0, 64, 64), anim.img, image.Pt(i*64, 0), draw.Src)
				images = append(images, img)
			}
		}
	}

	return images, stencilImg, flags
}

func (jw *JsonWed) ToWed() (*Wed, error) {
	wed := Wed{}

	wed.Header = wedHeader{
		Signature:           [4]byte{'W', 'E', 'D', ' '},
		Version:             [4]byte{'V', '1', '.', '3'},
		OverlayOffset:       uint32(0),
		SecondHeaderOffset:  uint32(0),
		DoorOffset:          uint32(0),
		DoorTileCellsOffset: uint32(0),
	}

	wed.Overlays = make([]wedOverlay, len(jw.Overlays))
	wed.Tilemaps = make([][]wedTilemap, len(jw.Overlays))
	wed.TileIndices = make([]uint16, 0)

	for idx, overlay := range jw.Overlays {
		stencilOffset := overlay.Width * overlay.Height
		stencils := make([]*image.RGBA, 0)
		o := &wed.Overlays[idx]
		o.Width = uint16(overlay.Width)
		o.Height = uint16(overlay.Height)
		o.Name = NewResref(overlay.Name)
		o.NumUniqueTiles = 0
		o.LayerFlags = uint16(overlay.Flags)
		o.TileIndexLookupOffset = 0
		wed.Tilemaps[idx] = make([]wedTilemap, len(overlay.Tilemap))
		o.TilemapOffset = uint32(0)
		jw.Overlays[idx].Tis = NewTis()

		for y := 0; y < overlay.Height; y++ {
			for x := 0; x < overlay.Width; x++ {
				tm := &wed.Tilemaps[idx][y*overlay.Width+x]

				tiles, stencilImg, flags := overlay.GenerateTiles(x, y, false)
				for _, tile := range tiles {
					tileId := jw.Overlays[idx].Tis.AddTile(tile)
					if idx == 0 {
						tm.TileIndexLookupIndex = uint16(len(wed.TileIndices))
					}
					wed.TileIndices = append(wed.TileIndices, uint16(tileId))
				}

				tm.AlternateTileIndex = int16(-1)
				tm.TileIndexLookupCount = uint16(len(tiles))
				tm.Flags = byte(flags)
				if stencilImg != nil {
					stencils = append(stencils, stencilImg)
					tm.AlternateTileIndex = int16(stencilOffset)
					stencilOffset++
				}
			}
		}
		for _, stencil := range stencils {
			jw.Overlays[idx].Tis.AddTile(stencil)
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
		effectedTiles := door.EffectedTiles(image.Point{jw.Overlays[0].PixelWidth(), jw.Overlays[0].PixelHeight()}, &jw.Overlays[0])
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
			y := tileId / jw.Overlays[0].Width
			x := tileId % jw.Overlays[0].Width
			tiles, stencil, _ := jw.Overlays[0].GenerateTiles(x, y, true)
			altTile := 0
			if stencil != nil {
				altTile = jw.Overlays[0].Tis.AddTile(stencil)
			} else {
				altTile = jw.Overlays[0].Tis.AddTile(tiles[0])
			}
			log.Printf("IDX: %d tileId: %d\n", idx, tileId)
			tm := &wed.Tilemaps[0][tileId]
			tm.AlternateTileIndex = int16(altTile)
			wed.DoorTileCells = append(wed.DoorTileCells, uint16(tileId))
		}
	}

	wed.UpdateOffsets()

	for _, overlay := range jw.Overlays {
		f, err := os.Create(fmt.Sprintf("%s.tis", overlay.Name))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		err = overlay.Tis.Write(f)
		if err != nil {
			return nil, err
		}
	}

	return &wed, nil
}

func (jw *JsonWed) importImages() error {
	for idx := range jw.Overlays {
		overlay := &jw.Overlays[idx]
		if overlay.Name != "" {
			f, err := os.Open(overlay.Name + ".png")
			if err != nil {
				log.Printf("Unable to open overlay png: " + overlay.Name + ".png")
				return err
			}

			img, err := png.Decode(f)
			if err != nil {
				log.Printf("unable to decode overlay png")
				return err
			}

			overlay.BackgroundImg = img

			f.Close()

		}
		for _, stencil := range overlay.Stencils {
			f, err := os.Open(stencil + ".png")
			if err != nil {
				log.Printf("Unable to open stencil png: " + stencil + ".png")
				return err
			}

			stencilImg, err := png.Decode(f)
			if err != nil {
				log.Printf("unable to decode stencil png")
				return err
			}

			overlay.StencilImages = append(overlay.StencilImages, stencilImg)

			f.Close()
		}
		for idx, _ := range overlay.Animations {
			animation := &overlay.Animations[idx]
			f, err := os.Open(animation.Name + ".png")
			if err != nil {
				return err
			}

			animationImg, err := png.Decode(f)
			if err != nil {
				return err
			}

			animation.img = animationImg

			f.Close()
		}
	}

	f, err := os.Open(jw.Overlays[0].Name + "c.png")
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Unable to open closed png:" + jw.Overlays[0].Name + "c.png")
		return err
	}

	closed, err := png.Decode(f)
	if err != nil {
		log.Printf("Unable to load decode closed png")
		return err
	}
	jw.Overlays[0].ClosedImage = closed

	return nil
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
			f.Close()
		}
	}
	return nil
}

func (jw *JsonWed) generateTilemaps() error {
	for idx, _ := range jw.Overlays {
		overlay := &jw.Overlays[idx]

		overlay.Tilemap = make([]jsonWedTilemap, overlay.Width*overlay.Height)
		for tmIdx, _ := range overlay.Tilemap {
			tm := &overlay.Tilemap[tmIdx]

			tm.Id = tmIdx
			tm.Count = 1
			tm.Alt = -1
		}
	}
	return nil
}

func OpenJWed(r io.ReadSeeker) (*JsonWed, error) {
	jwed := JsonWed{}
	jsonBlob, err := ioutil.ReadAll(r)
	if err != nil {
		log.Printf("unable to read json")
		return nil, err
	}
	if err = json.Unmarshal(jsonBlob, &jwed); err != nil {
		log.Printf("unable to unmarshal json")
		return nil, err
	}

	if err = jwed.importSvg(); err != nil {
		log.Printf("unable to import svg")
		return nil, err
	}

	if err = jwed.importImages(); err != nil {
		log.Printf("unable to import images")
		return nil, err
	}

	if err = jwed.generateTilemaps(); err != nil {
		log.Printf("unable to generate tilemaps")
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
