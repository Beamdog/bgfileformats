package bg

import (
	"encoding/xml"
	"image"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"
)

type svgVert struct {
	X, Y int
}

type svgPolyline struct {
	RawPoints string `xml:"points,attr"`
	Points    []svgVert
}

type svgPolygon struct {
	Points []svgVert
	Mode   int
}

type svgPath struct {
	Name        string `xml:"id,attr"`
	D           string `xml:"d,attr"`
	Description string `xml:"desc"`

	Polygons []svgPolygon
}

type svgGroup struct {
	Name      string     `xml:"id,attr"`
	Groups    []svgGroup `xml:"g"`
	Paths     []svgPath  `xml:"path"`
	Transform string     `xml:"transform,attr"`
	translate svgVert
}

type SvgFile struct {
	XMLName xml.Name   `xml:"svg"`
	Width   int        `xml:"width,attr"`
	Height  int        `xml:"height,attr"`
	Groups  []svgGroup `xml:"g"`
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
		wall.Verts[idx].X = pt.X
		wall.Verts[idx].Y = pt.Y
	}

	return wall
}


type svgPathScanner struct {
	Path string
	Polygons []svgPolygon
	CurrentPolygon *svgPolygon
	Cursor svgVert
	Mode int
	S scanner.Scanner
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

func (sps *svgPathScanner)scanWhitespace() {
	for r := sps.S.Peek(); (r == ' ' || r == ','); r = sps.S.Peek() {
		r = sps.S.Next()
	}
}

func (sps *svgPathScanner)scanOneInt() int {
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

		sps.Cursor.X, sps.Cursor.Y = sps.scanTwoInts()

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "m":
		poly := svgPolygon{Mode: sps.Mode}
		sps.CurrentPolygon = &poly

		X, Y := sps.scanTwoInts()
		sps.Cursor.X += X
		sps.Cursor.Y += Y

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "L":
		sps.Cursor.X, sps.Cursor.Y = sps.scanTwoInts()

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "l":
		X, Y := sps.scanTwoInts()
		sps.Cursor.X += X
		sps.Cursor.Y += Y

		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "H":
		sps.Cursor.X = sps.scanOneInt()
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "h":
		sps.Cursor.X += sps.scanOneInt()
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "V":
		sps.Cursor.Y = sps.scanOneInt()
		sps.CurrentPolygon.Points = append(sps.CurrentPolygon.Points, sps.Cursor)
	case "v":
		sps.Cursor.Y += sps.scanOneInt()
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

func (group *svgGroup) updateTransform() {
	var transformRegex = regexp.MustCompile(`([a-zA-Z]+)\((.*)\)`)
	if group.Transform != "" {
		matches := transformRegex.FindStringSubmatch(group.Transform)
		if matches == nil {
			log.Printf("Unknown transform: %s\n", group.Transform)
		} else if matches[1] == "translate" {
			coords := strings.Split(matches[2], ",")
			group.translate.X, _ = strconv.Atoi(coords[0])
			group.translate.Y, _ = strconv.Atoi(coords[1])
		} else {
			log.Printf("Unknown transform: [%s] in %s\n", matches[1], group.Transform)
		}
	}
}

func (group *svgGroup) updatePaths() {
	for idx := range group.Paths {
		group.Paths[idx].generatePath()
		group.Paths[idx].Name = group.Name
	}
}

func (group *svgGroup) process() {
	group.updateTransform()
	group.updatePaths()
	for idx := range group.Groups {
		g := &group.Groups[idx]
		g.process()
	}
}

func (group *svgGroup) GetPaths() []svgPath {
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

func (svg *SvgFile) process() {
	for idx := range svg.Groups {
		g := &svg.Groups[idx]
		g.process()
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
