package bg

import (
	"encoding/xml"
	"io"
	"text/scanner"
	"strings"
	"strconv"
	"log"
	"regexp"
	"image"
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
	Mode int
}

type svgPath struct {
	Name        string `xml:"id,attr"`
	D           string `xml:"d,attr"`
	Description string `xml:"desc"`

	Polygons []svgPolygon
}

type svgGroup struct {
	Name      string        `xml:"id,attr"`
	Groups    []svgGroup    `xml:"g"`
	Paths     []svgPath     `xml:"path"`
	Transform string        `xml:"transform,attr"`
	translate svgVert
}

type SvgFile struct {
	XMLName xml.Name   `xml:"svg"`
	Width   int        `xml:"width,attr"`
	Height  int        `xml:"height,attr"`
	Groups  []svgGroup `xml:"g"`
}

func scanTwoInts(s scanner.Scanner) (int, int) {
	s.Scan()
	X, _ := strconv.Atoi(s.TokenText())
	s.Scan()
	Y, _ := strconv.Atoi(s.TokenText())
	return X,Y
}
func scanOneInt(s scanner.Scanner) int {
	s.Scan()
	X, _ := strconv.Atoi(s.TokenText())
	return X
}

func (path *svgPath) mode() int {
	var settingRegexp = regexp.MustCompile(`([a-zA-Z]+)\:([ 0-9]+)`)
	if path.Description != "" {
		matches := settingRegexp.FindStringSubmatch(path.Description)
		if matches != nil {
			switch(matches[1]) {
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

func (path *svgPath) generatePath() {
	var s scanner.Scanner
	s.Init(strings.NewReader(path.D))
	s.Mode = scanner.ScanFloats | scanner.ScanChars


	var curPoly *svgPolygon
	cursor := svgVert{0, 0}
	tok := s.Scan()
	for tok != scanner.EOF {
		switch(s.TokenText()) {
		case "M":
			poly := svgPolygon{Mode: path.mode()}
			curPoly = &poly

			cursor.X, cursor.Y = scanTwoInts(s)

			curPoly.Points = append(curPoly.Points, cursor)
		case "m":
			poly := svgPolygon{Mode: path.mode()}
			curPoly = &poly

			X, Y := scanTwoInts(s)
			cursor.X += X
			cursor.Y += Y

			curPoly.Points = append(curPoly.Points, cursor)
		case "L":
			cursor.X, cursor.Y = scanTwoInts(s)

			curPoly.Points = append(curPoly.Points, cursor)
		case "l":
			X, Y := scanTwoInts(s)
			cursor.X += X
			cursor.Y += Y

			curPoly.Points = append(curPoly.Points, cursor)
		case "H":
			cursor.X = scanOneInt(s)
			curPoly.Points = append(curPoly.Points, cursor)
		case "h":
			cursor.X += scanOneInt(s)
			curPoly.Points = append(curPoly.Points, cursor)
		case "V":
			cursor.Y = scanOneInt(s)
			curPoly.Points = append(curPoly.Points, cursor)
		case "v":
			cursor.Y += scanOneInt(s)
			curPoly.Points = append(curPoly.Points, cursor)
		case "z":
			path.Polygons = append(path.Polygons, *curPoly)
			curPoly = nil
		case "Z":
			path.Polygons = append(path.Polygons, *curPoly)
			curPoly = nil
		}
		tok = s.Scan()
	}
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

func (group *svgGroup) GetPaths() []svgPath{
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
