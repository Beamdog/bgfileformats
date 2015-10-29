// +build darwin

package bg

import (
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"

	//polyclip "github.com/akavel/polyclip-go"
	"github.com/paulsmith/gogeos/geos"
)

type svgVert struct {
	X, Y float64
}

type svgPolyline struct {
	RawPoints string `xml:"points,attr"`
	Points    []svgVert
}

type svgPolygon struct {
	Points     []svgVert
	Mode       int
	PointsData string `xml:"points,attr"`
}

type svgPath struct {
	Name        string `xml:"id,attr"`
	D           string `xml:"d,attr"`
	Description string `xml:"desc"`

	Polygons []svgPolygon
}

type SvgGroup struct {
	Name      string       `xml:"id,attr"`
	Groups    []SvgGroup   `xml:"g"`
	Paths     []svgPath    `xml:"path"`
	Polygons  []svgPolygon `xml:"polygon"`
	Transform string       `xml:"transform,attr"`
	translate svgVert
}

type SvgFile struct {
	XMLName xml.Name   `xml:"svg"`
	Width   int        `xml:"width,attr"`
	Height  int        `xml:"height,attr"`
	Groups  []SvgGroup `xml:"g"`
}

func (path *svgPath) mode() int {
	var settingRegexp = regexp.MustCompile(`([a-zA-Z]+)\:([ 0-9]+)`)
	if path.Description != "" {
		matches := settingRegexp.FindStringSubmatch(path.Description)
		if matches != nil {
			switch matches[1] {
			case "mode":
				val, _ := strconv.Atoi(strings.TrimSpace(matches[2]))
				return val
			}
		}
	}
	return 0
}

func (poly *svgPolygon) JsonPoly() jsonWedPolygon {
	wall := jsonWedPolygon{Mode: poly.Mode}
	wall.Verts = make([]image.Point, len(poly.Points))
	for idx, pt := range poly.Points {
		wall.Verts[idx].X = int(pt.X)
		wall.Verts[idx].Y = int(pt.Y)
	}

	return wall
}

func (poly *svgPolygon) generatePoints() {
	vals := strings.FieldsFunc(poly.PointsData, func(r rune) bool {
		return (r == ' ' || r == '\t' || r == ',')
	})
	poly.Points = make([]svgVert, len(vals)/2)
	for idx, _ := range poly.Points {
		x, _ := strconv.ParseFloat(vals[idx*2], 32)
		y, _ := strconv.ParseFloat(vals[idx*2+1], 32)
		poly.Points[idx].X = x
		poly.Points[idx].Y = y
	}
}

type svgPathScanner struct {
	Path           string
	Polygons       []svgPolygon
	CurrentPolygon *svgPolygon
	Cursor         svgVert
	Mode           int
	S              scanner.Scanner
}

func NewPathScanner(path string, mode int) svgPathScanner {
	log.Printf("Scanner path: %s\n", path)
	sps := svgPathScanner{Path: path, Mode: mode}
	return sps
}

func (sps *svgPathScanner) scanTwoInts() (int, int) {
	X := sps.scanOneInt()
	sps.scanWhitespace()
	Y := sps.scanOneInt()
	log.Printf("X: %d Y: %d\n", X, Y)
	return X, Y
}

func (sps *svgPathScanner) scanWhitespace() {
	for r := sps.S.Peek(); r == ' ' || r == ','; r = sps.S.Peek() {
		r = sps.S.Next()
	}
}

func (sps *svgPathScanner) scanOneInt() int {
	r := sps.S.Scan()

	sign := 1
	if r == '-' {
		sps.S.Scan()
		sign = -1
	}
	X, _ := strconv.ParseFloat(sps.S.TokenText(), 32)
	return int(X) * sign
}

func (sps svgPathScanner) GeneratePolygons() ([]svgPolygon, error) {
	sps.S.Init(strings.NewReader(sps.Path))
	sps.S.Mode = scanner.ScanFloats | scanner.ScanChars

	tok := sps.S.Scan()
	lastTokenText := ""
	for tok != scanner.EOF {
		tokenText := sps.S.TokenText()
		log.Printf("TT: %s LTT:%s\n", tokenText, lastTokenText)
		if !sps.handleToken(tokenText) {
			log.Printf("Retry\n")
			sps.handleToken(lastTokenText)
		}

		lastTokenText = tokenText
		tok = sps.S.Scan()
	}
	return sps.Polygons, nil
}

func (sps *svgPathScanner) handleToken(cmd string) bool {
	log.Printf("Cmd: %s\n", cmd)
	switch cmd {
	case "M":
		poly := svgPolygon{Mode: sps.Mode}
		sps.CurrentPolygon = &poly

		x, y := sps.scanTwoInts()
		sps.Cursor.X, sps.Cursor.Y = float64(x), float64(y)

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "m":
		poly := svgPolygon{Mode: sps.Mode}
		sps.CurrentPolygon = &poly

		X, Y := sps.scanTwoInts()
		sps.Cursor.X += float64(X)
		sps.Cursor.Y += float64(Y)

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "L":
		X, Y := sps.scanTwoInts()
		sps.Cursor.X, sps.Cursor.Y = float64(X), float64(Y)

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "l":
		X, Y := sps.scanTwoInts()
		sps.Cursor.X += float64(X)
		sps.Cursor.Y += float64(Y)

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "H":
		sps.Cursor.X = float64(sps.scanOneInt())
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "h":
		sps.Cursor.X += float64(sps.scanOneInt())
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "V":
		sps.Cursor.Y = float64(sps.scanOneInt())
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "v":
		sps.Cursor.Y += float64(sps.scanOneInt())
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "z":
		sps.Polygons = append(sps.Polygons, *sps.CurrentPolygon)
		sps.CurrentPolygon = nil
	case "Z":
		sps.Polygons = append(sps.Polygons, *sps.CurrentPolygon)
		sps.CurrentPolygon = nil
	default:
		return false
	}
	return true
}

func (path *svgPath) generatePath() {
	polys, err := NewPathScanner(path.D, path.mode()).GeneratePolygons()
	if err != nil {
		log.Printf("Error generating polygons: %v", err)
	}
	log.Printf("Polys: %+v\n", polys)
	path.Polygons = polys
}

func (group *SvgGroup) updateTransform() {
	var transformRegex = regexp.MustCompile(`([a-zA-Z]+)\((.*)\)`)
	if group.Transform != "" {
		matches := transformRegex.FindStringSubmatch(group.Transform)
		if matches == nil {
			log.Printf("Unknown transform: %s\n", group.Transform)
		} else if matches[1] == "translate" {
			coords := strings.Split(matches[2], ",")
			group.translate.X, _ = strconv.ParseFloat(coords[0], 64)
			group.translate.Y, _ = strconv.ParseFloat(coords[1], 64)
		} else {
			log.Printf("Unknown transform: [%s] in %s\n", matches[1], group.Transform)
		}
	}
}

func (group *SvgGroup) updatePaths() {
	for idx := range group.Paths {
		group.Paths[idx].generatePath()
		group.Paths[idx].Name = group.Name
	}
}

func (group *SvgGroup) updatePolygons() {
	for idx := range group.Polygons {
		group.Polygons[idx].generatePoints()
	}
}

func (group *SvgGroup) process() {
	group.updateTransform()
	group.updatePaths()
	group.updatePolygons()
	for idx := range group.Groups {
		g := &group.Groups[idx]
		g.process()
	}
}

func (group *SvgGroup) GetPaths() []svgPath {
	paths := make([]svgPath, 0)
	if strings.HasPrefix(group.Name, "door_open_") || strings.HasPrefix(group.Name, "door_closed_") || strings.HasPrefix(group.Name, "walls") {
		for _, p := range group.Paths {
			paths = append(paths, p)
		}
	}
	for _, g := range group.Groups {
		paths = append(paths, g.GetPaths()...)
	}

	for idx, _ := range paths {
		p := &paths[idx]
		for pIdx, _ := range p.Polygons {
			poly := &p.Polygons[pIdx]
			for vIdx, _ := range poly.Points {
				v := &poly.Points[vIdx]
				v.X += group.translate.X
				v.Y += group.translate.Y
			}
		}
	}

	return paths
}

func geosPolygonToPolygon(poly *geos.Geometry) svgPolygon {
	shell, err := poly.Shell()
	if err != nil {
		log.Fatal(fmt.Errorf("Shell creation error: %+v", err))
	}
	mergedPoly := svgPolygon{}
	coords, err := shell.Coords()
	if err != nil {
		log.Fatal(fmt.Errorf("Coords error: %+v", err))
	}
	for _, pt := range coords {
		mergedPoly.Points = append(mergedPoly.Points, svgVert{pt.X, pt.Y})
	}
	return mergedPoly
}

func (group *SvgGroup) MergePolygons() {
	if len(group.Polygons) > 0 {
		var poly *geos.Geometry

		for _, p := range group.Polygons {
			if len(p.Points) > 2 {
				verts := make([]geos.Coord, 0)
				for _, v := range p.Points {
					verts = append(verts, geos.NewCoord(v.X, v.Y))
				}
				verts = append(verts, geos.NewCoord(p.Points[0].X, p.Points[0].Y))
				if poly == nil {
					newPoly, err := geos.NewPolygon(verts)
					if err != nil {
						log.Fatal(fmt.Errorf("New poly creation error: %+v", err))
					}
					poly = newPoly
				} else {
					uPoly, _ := geos.NewPolygon(verts)
					uPolyType, _ := uPoly.Type()
					if uPolyType == geos.POLYGON {
						union, err := poly.Union(uPoly)
						if err != nil {
							log.Printf("Skipping poly: Poly union error: %+v %+v", err, uPoly)
						} else {
							poly = union
						}
					} else {
						log.Printf("Not a poly: %d %+v", uPolyType, uPoly)
					}
				}
			}
		}

		polyType, err := poly.Type()
		if err != nil {
			log.Fatal(fmt.Errorf("Poly type error: %+v", err))
		}
		if polyType == geos.POLYGON {
			group.Polygons = make([]svgPolygon, 1)
			group.Polygons[0] = geosPolygonToPolygon(poly)
		} else if polyType == geos.MULTIPOLYGON || polyType == geos.GEOMETRYCOLLECTION {
			geomCount, err := poly.NGeometry()
			if err != nil {
				log.Fatal(fmt.Errorf("Error getting geometry count", err))
			}
			log.Printf("GC: %d PLEN: %d\n", geomCount, len(group.Polygons))

			group.Polygons = make([]svgPolygon, 0)
			for i := 0; i < geomCount; i++ {
				geom, err := poly.Geometry(i)
				if err != nil {
					log.Fatal(fmt.Errorf("Error getting geometry: %d %+v", i, err))
				}
				geomType, _ := geom.Type()
				if geomType == geos.POLYGON {
					group.Polygons = append(group.Polygons, geosPolygonToPolygon(geom))
				}
			}
		} else {
			log.Printf("DUMP: %+v", poly)
			log.Printf("Polytype: %d\n", polyType)

		}
	}
}

/*
func (group *SvgGroup) MergePolygons() {
	if len(group.Polygons) > 0 {
		poly := polyclip.Polygon{}
		for _, cont := range group.Polygons {
			c := polyclip.Contour{}
			for _, v := range cont.Points {
				c.Add(polyclip.Point{v.X * 100, v.Y * 100})
			}
			poly.Add(c)
		}

		union := poly
		unionPoints := 0
		for cidx, cont := range group.Polygons {
			c := polyclip.Contour{}
			for _, v := range cont.Points {
				c.Add(polyclip.Point{v.X * 100, v.Y * 100})
			}
			u2 := union.Construct(polyclip.UNION, polyclip.Polygon{c})
			bb := c.BoundingBox()


			newPoints := 0
			for _, c := range u2 {
				newPoints += len(c)
			}
			if newPoints >= unionPoints {
				union = u2
				unionPoints = newPoints
			} else {
			log.Printf("Contour: %d %+v %d Area: %0.3f %d %d", cidx, c.BoundingBox(), len(c), (bb.Max.X-bb.Min.X) * (bb.Max.Y - bb.Min.Y), newPoints, unionPoints)
			log.Printf("Clockwise: %v", polyClockwise(c[0].X, c[0].Y, c[1].X, c[1].Y, c[len(c)-1].X, c[len(c)-1].Y))
			log.Printf("Union: %+v", union)
			log.Printf("Union2: %+v", u2)
			}

		}

		group.Polygons = make([]svgPolygon, len(union))
		for _, p := range union {
			mergedPoly := svgPolygon{}
			for _, pt := range p {
				mergedPoly.Points = append(mergedPoly.Points, svgVert{pt.X, pt.Y})
			}
			group.Polygons = append(group.Polygons, mergedPoly)
		}
	}
}
*/

func (group *SvgGroup) MergePolygonsRecursive() {
	group.MergePolygons()
	for idx := range group.Groups {
		group.Groups[idx].MergePolygonsRecursive()
	}
}

func (svg *SvgFile) process() {
	for idx := range svg.Groups {
		g := &svg.Groups[idx]
		g.process()
	}
}

func (svg *SvgFile) MergePolygonsByGroup() {
	for idx := range svg.Groups {
		svg.Groups[idx].MergePolygonsRecursive()
	}
}

func (svg *SvgFile) Paths() []svgPath {
	paths := make([]svgPath, 0)
	for _, g := range svg.Groups {
		paths = append(paths, g.GetPaths()...)
	}
	return paths
}

func OpenSVG(r io.Reader) (*SvgFile, error) {
	svg := &SvgFile{}
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&svg); err != nil {
		return nil, err
	}
	svg.process()

	return svg, nil

}
