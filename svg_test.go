package bg

import (
	"testing"
)

func TestSvgParseAbsolutePath(t *testing.T) {
	//absolute path - triangle
	path := svgPath{D: "M 100 100 L 300 100 L 200 300 z"}
	path.generatePath()

	if len(path.Polygons) != 1 {
		t.Errorf("Did not generate 1 polygon: %+v\n", path)
	}
	expectedPoints := []svgVert{{100, 100}, {300, 100}, {200, 300}}
	for idx, pt := range expectedPoints {
		if path.Polygons[0].Points[idx] != pt {
			t.Errorf("Point[%d] %v != %v", idx, path.Polygons[0].Points[idx], pt)
		}
	}
}

func testSvgParseAbsoluteRepeatCommands(t *testing.T) {
	//absolute path repeat commands - triangle
	path := svgPath{D: "M 100 100 L 300 100 200 300 z"}
	path.generatePath()

	if len(path.Polygons) != 1 {
		t.Errorf("Did not generate 1 polygon: %+v\n", path)
	}
	expectedPoints := []svgVert{{100, 100}, {300, 100}, {200, 300}}
	for idx, pt := range expectedPoints {
		if path.Polygons[0].Points[idx] != pt {
			t.Errorf("Point[%d] %v != %v", idx, path.Polygons[0].Points[idx], pt)
		}
	}
}

func TestSvgParseRelative(t *testing.T) {
	//relative path - triangle
	path := svgPath{D: "M 100 100 l 200 0 l -100 200 z"}
	path.generatePath()

	if len(path.Polygons) != 1 {
		t.Errorf("Did not generate 1 polygon: %+v\n", path)
	}
	expectedPoints := []svgVert{{100, 100}, {300, 100}, {200, 300}}
	for idx, pt := range expectedPoints {
		if path.Polygons[0].Points[idx] != pt {
			t.Errorf("Point[%d] %v != %v", idx, path.Polygons[0].Points[idx], pt)
		}
	}
}

func TestSvgParseCommas(t *testing.T) {
	//relative path comma seperators- triangle
	path := svgPath{D: "M 100,101 l 200,0 l -100,200 z"}
	path.generatePath()

	if len(path.Polygons) != 1 {
		t.Errorf("Did not generate 1 polygon: %+v\n", path)
	}
	expectedPoints := []svgVert{{100, 101}, {300, 101}, {200, 301}}
	for idx, pt := range expectedPoints {
		if path.Polygons[0].Points[idx] != pt {
			t.Errorf("Point[%d] %v != %v", idx, path.Polygons[0].Points[idx], pt)
		}
	}
}

func TestSvgParseFloats(t *testing.T) {
	//relative path comma seperators- triangle
	path := svgPath{D: "M 100.4,101.7 l 200.2,0 l -100.5,200.7 z"}
	path.generatePath()

	if len(path.Polygons) != 1 {
		t.Errorf("Did not generate 1 polygon: %+v\n", path)
	}
	expectedPoints := []svgVert{{100, 101}, {300, 101}, {200, 301}}
	for idx, pt := range expectedPoints {
		if path.Polygons[0].Points[idx] != pt {
			t.Errorf("Point[%d] %v != %v", idx, path.Polygons[0].Points[idx], pt)
		}
	}
}
