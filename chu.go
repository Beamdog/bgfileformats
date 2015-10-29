package bg

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"text/template"

	"github.com/ajstarks/svgo"
)

type chuHeader struct {
	Signature, Version [4]byte
	PanelCount         uint32
	ControlOffset      uint32
	PanelOffset        uint32
}

type chuPanel struct {
	ID     uint32
	X      uint16
	Y      uint16
	Width  uint16
	Height uint16
	Type   uint16
	Mosaic RESREF

	Buttons      []chuControlButton
	Sliders      []chuControlSlider
	Edits        []chuControlEdit
	TextDisplays []chuControlTextDisplay
	Labels       []chuControlLabel
	ScrollBars   []chuControlScrollBar
}

type chuPanelHeader struct {
	PanelID      uint32
	X            uint16
	Y            uint16
	Width        uint16
	Height       uint16
	Type         uint16
	ControlCount uint16
	Mosaic       RESREF
	FirstControl uint16
	Flags        uint16
}

type chuControlTable struct {
	ControlOffset uint32
	ControlSize   uint32
}

type chuControlBase struct {
	ControlID uint32
	X         uint16
	Y         uint16
	Width     uint16
	Height    uint16
	Type      uint16
}

type chuControlButton struct {
	ControlID     uint32
	X             uint16
	Y             uint16
	Width         uint16
	Height        uint16
	Type          uint16
	Bam           RESREF
	Sequence      uint8
	Flags         uint8
	FrameUp       uint8
	PtXLower      uint8
	FrameDown     uint8
	PtXHigher     uint8
	FrameFocus    uint8
	PtYLower      uint8
	FrameDisabled uint8
	PtYHigher     uint8
}

type chuControlSlider struct {
	ControlID          uint32
	X                  uint16
	Y                  uint16
	Width              uint16
	Height             uint16
	Type               uint16
	Slider             RESREF
	SliderThumb        RESREF
	SequenceThumb      uint16
	FrameThumbNormal   uint16
	FrameThumbSelected uint16
	XThumbOffset       uint16
	YThumbOffset       uint16
	TickSize           uint16
	Ticks              uint16
	ActiveBarTop       uint16
	ActiveBarBottom    uint16
	ActiveBarLeft      uint16
	ActiveBarRight     uint16
}
type chuControlEdit struct {
	ControlID         uint32
	X                 uint16
	Y                 uint16
	Width             uint16
	Height            uint16
	Type              uint16
	Edit              RESREF
	EditClientFocus   RESREF
	EditClientNoFocus RESREF
	EditCaret         RESREF
	SequenceEditCaret uint16
	FrameEditCaret    uint16
	XEditClientOffset uint16
	YEditClientOffset uint16
	XEditCaretOffset  uint16
	YEditCaretOffset  uint16
	TextFont          RESREF
	SequenceText      uint16
	DefaultString     [32]byte
	MaxChars          uint16
	CaseFormat        uint16
	TypeFormat        uint16
}
type chuControlTextDisplay struct {
	ControlID           uint32
	X                   uint16
	Y                   uint16
	Width               uint16
	Height              uint16
	Type                uint16
	TextFont            RESREF
	NameFont            RESREF
	TextForegroundColor uint32
	DefaultNameColor    uint32
	BackgroundColor     uint32
	ScrollBarID         uint32
}

type chuControlLabel struct {
	ControlID           uint32
	X                   uint16
	Y                   uint16
	Width               uint16
	Height              uint16
	Type                uint16
	StringRef           uint32
	TextFont            RESREF
	TextForegroundColor uint32
	BackgroundColor     uint32
	LabelType           uint16
}

type chuControlScrollBar struct {
	ControlID       uint32
	X               uint16
	Y               uint16
	Width           uint16
	Height          uint16
	Type            uint16
	Bam             RESREF
	Sequence        uint16
	TopFrameUp      uint16
	TopFrameDown    uint16
	BottomFrameUp   uint16
	BottomFrameDown uint16
	BarFrame        uint16
	ThumbFrame      uint16
	ControlWindowID uint32
}

type chuControl struct {
	ControlID uint32
	Data      []byte
}

const (
	BUTTON_FLAGS_LEFTALIGN      = 0x01
	BUTTON_FLAGS_RIGHTALIGN     = 0x02
	BUTTON_FLAGS_TOPALIGN       = 0x04
	BUTTON_FLAGS_BOTTOMALIGN    = 0x08
	BUTTON_FLAGS_USEREFPT       = 0x10
	BUTTON_FLAGS_LOWERCASELABEL = 0x20
	BUTTON_FLAGS_NOWORDWRAP     = 0x40
)

type CHU struct {
	Name     string
	Header   chuHeader
	panels   []chuPanelHeader
	Panels   []chuPanel
	Controls []chuControl
}

func (chu *CHU) OffsetStrings() {

}

func (chu *CHU) Write(w io.Writer) error {
	return nil
}

func (c *chuControlButton) PrintResources() {
	log.Printf("Bam: %s\n", c.Bam.String())
}
func (c *chuControlSlider) PrintResources() {
	log.Printf("Slider Bam: %s\n", c.Slider.String())
	log.Printf("Slider Thumb Bam: %s\n", c.SliderThumb.String())
}
func (c *chuControlEdit) PrintResources() {
	log.Printf("Edit: %s\n", c.Edit.String())
}
func (c *chuControlTextDisplay) PrintResources() {
	log.Printf("Text Font: %s\n", c.TextFont.String())
	log.Printf("Name Font: %s\n", c.NameFont.String())
}
func (c *chuControlLabel) PrintResources() {
	log.Printf("Text Font: %s\n", c.TextFont.String())
}
func (c *chuControlScrollBar) PrintResources() {
	log.Printf("Bam: %s\n", c.Bam.String())
}

func (p *chuPanel) PrintResources() {
	log.Printf("Mosaic: %s\n", p.Mosaic.String())
	for _, c := range p.Buttons {
		c.PrintResources()
	}
	for _, c := range p.Sliders {
		c.PrintResources()
	}
	for _, c := range p.Edits {
		c.PrintResources()
	}
	for _, c := range p.TextDisplays {
		c.PrintResources()
	}
	for _, c := range p.Labels {
		c.PrintResources()
	}
	for _, c := range p.ScrollBars {
		c.PrintResources()
	}

}

func (chu *CHU) PrintResources() {
	for _, p := range chu.Panels {
		p.PrintResources()
	}
}

func (l *chuControlLabel) MenuAlign() string {
	return "center center"
}

func (t *chuControlTextDisplay) HasScrollbar() bool {
	return t.ScrollBarID != 0xFFFFFFFF
}

func (c *chuControlButton) loadBamInfo() ([][]string, error) {
	bamIn, err := os.Open(c.Bam.String() + ".bamd")
	if err != nil {
		return nil, fmt.Errorf("Error loading bamd: %s %s", c.Bam.String()+".bamd", err)
	}
	defer bamIn.Close()

	var s scanner.Scanner
	s.Init(bamIn)
	s.Whitespace = 1<<'\t' | 1<<' '

	frames := map[string]string{}
	sequences := make([][]string, 0)

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if strings.ToLower(s.TokenText()) == "frame" {
			center_x, center_y := 0, 0
			tok = s.Scan()
			name := strings.TrimSpace(s.TokenText())
			tok = s.Scan()
			path := strings.Trim(s.TokenText(), "\"")
			tok = s.Scan()
			if s.TokenText() != "\n" {
				neg_x := 1
				neg_y := 1
				if s.TokenText() == "-" {
					neg_x = -1
					s.Scan()
				}
				if center_x, err = strconv.Atoi(s.TokenText()); err != nil {
					return nil, fmt.Errorf("Error converting %s to an int: %v", s.TokenText(), err)
				}
				center_x *= neg_x
				tok = s.Scan()
				if s.TokenText() == "-" {
					neg_y = -1
					s.Scan()
				}
				if center_y, err = strconv.Atoi(s.TokenText()); err != nil {
					return nil, fmt.Errorf("Error converting %s to an int: %v", s.TokenText(), err)
				}
				center_y *= neg_y
			}

			frames[name] = path
		} else if strings.ToLower(s.TokenText()) == "sequence" {
			seqframes := make([]string, 0)
			for tok = s.Scan(); !(s.TokenText() == "\n" || s.TokenText() == "\r" || tok == scanner.EOF); tok = s.Scan() {
				frame := strings.TrimSpace(s.TokenText())
				seqframes = append(seqframes, frames[frame])
			}

			sequences = append(sequences, seqframes)
		}
	}
	return sequences, nil
}

var bamLut map[string][][]string

func (c *chuControlButton) BamdPath() string {
	var lut [][]string
	var ok bool
	var err error
	lut, ok = bamLut[c.Bam.String()]
	if !ok {
		lut, err = c.loadBamInfo()
		if err != nil {
			log.Printf("err: %v\n", err)
			return ""
		}
		bamLut[c.Bam.String()] = lut
	}
	if len(lut) > int(c.Sequence) && len(lut[c.Sequence]) > int(c.FrameUp) {
		return lut[c.Sequence][c.FrameUp]
	}
	log.Printf("missing: %s %d %d %d", c.Bam.String(), c.Sequence, len(lut), c.FrameUp)
	return ""
}

func (chu *CHU) WriteSvg(w io.Writer) error {
	bamLut = map[string][][]string{}
	chuSvg := svg.New(w)
	chuSvg.Start(1024, 768, "xmlns:inkscape=\"http://www.inkscape.org/namespaces/inkscape\"")

	for _, p := range chu.Panels {
		chuSvg.Group(fmt.Sprintf("id=\"p_%d\" inkscape:label=\"panel_%d\" inkscape:groupmode=\"layer\"", p.ID, p.ID))
		if p.Mosaic.String() != "" {
			chuSvg.Image(int(p.X), int(p.Y), int(p.Width), int(p.Height), p.Mosaic.String()+".png")
		}
		chuSvg.Gid(fmt.Sprintf("pannel_%d_buttons", p.ID))
		for _, b := range p.Buttons {
			if b.BamdPath() != "" {
				chuSvg.Image(int(p.X+b.X), int(p.Y+b.Y), int(b.Width), int(b.Height), b.BamdPath())
			} else {
				chuSvg.Rect(int(p.X+b.X), int(p.Y+b.Y), int(b.Width), int(b.Height), "fill=\"red\" fill-opacity=\"0.5\"")
				chuSvg.Text(int(p.X+b.X), int(p.Y+b.Y)+10, b.Bam.String())
			}
		}
		chuSvg.Gend()
		chuSvg.Gend()
	}

	chuSvg.End()

	return nil

}

var g_records [][]string

func generatePanelContext(name string, panelId uint32) string {
	pid := strconv.Itoa(int(panelId))
	for _, r := range g_records {
		if name == r[0] && r[2] == pid {
			return r[1]
		}
	}
	return fmt.Sprintf("%d", panelId)
}

func generateControlContext(name string, panelId, controlId uint32) string {
	pid := strconv.Itoa(int(panelId))
	cid := strconv.Itoa(int(controlId))
	for _, r := range g_records {
		if name == r[0] && r[2] == pid && r[4] == cid {
			return r[3]
		}
	}
	return fmt.Sprintf("Control: %d", controlId)
}

func (chu *CHU) WriteMenu(w io.Writer, name string) error {
	csvFile, err := os.Open("menu.csv")
	if err != nil {
		return fmt.Errorf("Unable to open menu.csv: %+v", err)
	}
	r := csv.NewReader(csvFile)
	g_records, err = r.ReadAll()
	if err != nil {
		return fmt.Errorf("Unable to parse csv: %+v", err)
	}

	funcMap := template.FuncMap{
		"context": generateControlContext,
		"panel":   generatePanelContext,
	}
	tmpl, err := template.New("chu.tmpl").Funcs(funcMap).ParseFiles("chu.tmpl")
	if err != nil {
		return fmt.Errorf("Unable to compile menu template: %+v", err)
	}
	chu.Name = name
	return tmpl.Execute(w, chu)
}

func DecodeChu(r io.ReadSeeker) (*CHU, error) {
	chu := &CHU{}

	err := binary.Read(r, binary.LittleEndian, &chu.Header)
	if err != nil {
		return nil, err
	}

	chu.panels = make([]chuPanelHeader, chu.Header.PanelCount)
	_, err = r.Seek(int64(chu.Header.PanelOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &chu.panels)
	if err != nil {
		return nil, err
	}
	controlCount := 0
	for _, panel := range chu.panels {
		controlCount += int(panel.ControlCount)
	}
	controlTable := make([]chuControlTable, controlCount)
	_, err = r.Seek(int64(chu.Header.ControlOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &controlTable)

	chu.Controls = make([]chuControl, controlCount)

	for _, panel := range chu.panels {
		p := chuPanel{ID: panel.PanelID, X: panel.X, Y: panel.Y, Width: panel.Width, Height: panel.Height, Type: panel.Type, Mosaic: panel.Mosaic}
		for idx, control := range controlTable[panel.FirstControl : panel.FirstControl+panel.ControlCount] {
			_, err = r.Seek(int64(control.ControlOffset), os.SEEK_SET)
			if err != nil {
				return nil, err
			}
			controlBase := chuControlBase{}
			err = binary.Read(r, binary.LittleEndian, &controlBase)
			if err != nil {
				return nil, err
			}
			chu.Controls[idx].ControlID = controlBase.ControlID
			// Re-seek back to the start of the control once we know waht kind we have
			_, err = r.Seek(int64(control.ControlOffset), os.SEEK_SET)
			if err != nil {
				return nil, err
			}

			switch controlBase.Type {
			case 0: // BUTTON
				c := chuControlButton{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.Buttons = append(p.Buttons, c)
			case 2: // SLIDER
				c := chuControlSlider{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.Sliders = append(p.Sliders, c)

			case 3: // EDIT
				c := chuControlEdit{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.Edits = append(p.Edits, c)

			case 5: // TEXT
				c := chuControlTextDisplay{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.TextDisplays = append(p.TextDisplays, c)

			case 6: // LABEL
				c := chuControlLabel{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.Labels = append(p.Labels, c)

			case 7: // SCROLLBAR
				c := chuControlScrollBar{}
				err = binary.Read(r, binary.LittleEndian, &c)
				if err != nil {
					return nil, err
				}
				p.ScrollBars = append(p.ScrollBars, c)

			default: // OTHER LABEL

			}
		}
		chu.Panels = append(chu.Panels, p)
	}

	return chu, nil
}
