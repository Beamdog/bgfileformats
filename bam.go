package bg

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/scanner"
)

func next_pow_two(v uint) uint {
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}

type BAM struct {
	Image           []image.Paletted
	ImageRgba       []image.RGBA
	Sequences       []BamSequence
	SequenceToImage []int16
	Width           int
	Height          int
	Frames          []BamFrame
	RleFrame        [][]uint8
	Palette         color.Palette
}
type BamSequence struct {
	Start int
	Count int
}

type BamFrame struct {
	Width, Height    uint16
	CenterX, CenterY int16
	FrameOffset      uint32
}

type BamCycle struct {
	FrameCount int16
	FrameIndex int16
}
type BamHeader struct {
	Signature, Version [4]byte
	Frames             uint16
	Cycles             uint8
	CompressedColor    uint8
	FrameOffset        uint32
	PaletteOffset      uint32
	FrameLutOffset     uint32
}

type BamHeaderV2 struct {
	Signature, Version                         [4]byte
	Frames, Sequences, Quads                   uint32
	FramesOffset, SequencesOffset, QuadsOffset uint32
}

type BamFrameV2 struct {
	Width, Height    uint16
	CenterX, CenterY int16
	QuadStart        int16
	QuadCount        int16
}

type BamMosaicQuad struct {
	Texture            int32
	X, Y, W, H, SX, SY int32
}

type BamCHeader struct {
	Signature, Version [4]byte
	UncompressedSize   uint32
}

type decoder struct {
	Header        BamHeader
	Frames        []BamFrame
	Cycles        []BamCycle
	Palette       []uint32
	FrameLUT      []int16
	image         []image.Paletted
	imageRgba     []image.RGBA
	colorMap      color.Palette
	width         int
	height        int
	sequences     []BamSequence
	customPalette bool
	replaceColor  map[color.Color]int
}

func uint32ToRGBA(c uint32) (r, g, b, a uint8) {
	b = uint8(c & 0x000000ff)
	g = uint8((c & 0x0000ff00) >> 8)
	r = uint8((c & 0x00ff0000) >> 16)
	a = uint8((c & 0xff000000) >> 24)

	return r, g, b, a
}

type bgpal struct {
	palette color.Palette
}

func (p bgpal) Convert(c color.Color) color.Color {
	return color.RGBA{0, 0, 0, 255}
}
func (p bgpal) Index(c color.Color) int {
	return 0
}

func (d *decoder) decode_bamd(r io.Reader) error {
	var s scanner.Scanner
	var err error
	var imgFrames []image.Image
	s.Init(r)
	s.Whitespace = 1<<'\t' | 1<<' '
	frameNames := map[string]int{}
	d.replaceColor = map[color.Color]int{}

	useImagePalette := false
	useSmallPalette := false
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
					return fmt.Errorf("Error converting %s to an int: %v", s.TokenText(), err)
				}
				center_x *= neg_x
				tok = s.Scan()
				if s.TokenText() == "-" {
					neg_y = -1
					s.Scan()
				}
				if center_y, err = strconv.Atoi(s.TokenText()); err != nil {
					return fmt.Errorf("Error converting %s to an int: %v", s.TokenText(), err)
				}
				center_y *= neg_y
			}
			imgFile, err := os.Open(filepath.Clean(path))
			if err != nil {
				return fmt.Errorf("Unable to open %s: %v", filepath.Clean(path), err)
			}
			img, _, err := image.Decode(imgFile)
			if err != nil {
				return fmt.Errorf("Unable to decode png %s: %v", filepath.Clean(path), err)
			}
			imgFile.Close()

			r := trimBounds(img)
			palImg, ok := img.(*image.Paletted)
			if !ok {
				imgTrim := image.NewRGBA(image.Rect(0, 0, r.Dx()+1, r.Dy()+1))
				draw.Draw(imgTrim, imgTrim.Bounds(), img, r.Min, draw.Over)
				imgFrames = append(imgFrames, imgTrim)
			} else {
				// Copy over the pixels, don't use draw.Draw it doesn't handle transarencies
				imgTrim := image.NewPaletted(image.Rect(0, 0, r.Dx()+1, r.Dy()+1), palImg.Palette)
				bounds := r
				for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
					for x := bounds.Min.X; x <= bounds.Max.X; x++ {
						c1 := palImg.At(x, y)
						imgTrim.Set(x-bounds.Min.X, y-bounds.Min.Y, c1)
					}
				}
				imgFrames = append(imgFrames, imgTrim)
			}
			center_x = center_x - r.Min.X
			center_y = center_y - r.Min.Y

			frame := BamFrame{uint16(r.Size().X + 1), uint16(r.Size().Y + 1), int16(center_x), int16(center_y), 0}

			frameNames[name] = len(d.Frames)
			d.Frames = append(d.Frames, frame)
		} else if strings.ToLower(s.TokenText()) == "sequence" {
			frames := make([]string, 0)
			sequences := make([]uint16, 0)
			for tok = s.Scan(); !(s.TokenText() == "\n" || s.TokenText() == "\r" || tok == scanner.EOF); tok = s.Scan() {
				frame := strings.TrimSpace(s.TokenText())
				frames = append(frames, frame)
				sequences = append(sequences, uint16(frameNames[frame]))
			}
			var sequence BamSequence

			if len(frames) > 0 {
				newlut := make([]int16, len(d.FrameLUT)+len(frames))
				sequence.Start = len(d.FrameLUT)
				sequence.Count = len(frames)
				copy(newlut, d.FrameLUT)
				for i, f := range frames {
					newlut[sequence.Start+i] = int16(frameNames[f])
				}
				d.FrameLUT = newlut
			} else {
				sequence.Start = -1
				sequence.Count = 0
			}

			d.sequences = append(d.sequences, sequence)
		} else if strings.ToLower(s.TokenText()) == "colormap" {
			tok = s.Scan()
			c, err := strconv.ParseUint(strings.TrimSpace(s.TokenText()), 0, 32)
			if err != nil {
				log.Printf("Invalid color: %s %+v", s.TokenText(), err)
			}
			r, g, b, a := uint32ToRGBA(uint32(c))

			tok = s.Scan()
			index, err := strconv.Atoi(s.TokenText())
			if err != nil {
				log.Printf("Invalid palette index: %d %+v", s.TokenText(), err)
			}
			colToReplace := color.RGBA{r, g, b, a}
			d.replaceColor[colToReplace] = index
			log.Printf("Trying to replace %+v with index %d\n", colToReplace, index)

		} else if strings.ToLower(s.TokenText()) == "usepalette" {
			useImagePalette = true
		} else if strings.ToLower(s.TokenText()) == "smallpalette" {
			useSmallPalette = true
		}

	}

	paletteImg, ok := imgFrames[0].(*image.Paletted)
	quantizeImage := false
	if ok && len(imgFrames) == 1 {
		log.Printf("Using existing palette")
		useImagePalette = true

	} else {
		quantizeImage = true
	}
	if useImagePalette {
		d.colorMap = make([]color.Color, len(paletteImg.Palette))
		copy(d.colorMap, paletteImg.Palette)
		// convert transparent palette entry to color key
		for idx, c := range d.colorMap {
			_, _, _, a := c.RGBA()
			if a == 0 && idx != 0 {
				// Swap idx with position 0
				// We need to replace the colors at entry 0 with our idx
				d.replaceColor[d.colorMap[0]] = idx
				d.replaceColor[d.colorMap[idx]] = 0
				d.colorMap[0], d.colorMap[idx] = d.colorMap[idx], d.colorMap[0]
				d.colorMap[0] = color.RGBA{0, 255, 0, 255}
				break
			}
		}
	} else if quantizeImage && !d.customPalette {
		log.Printf("Generating palette")
		maxHeight := 0
		width := 0
		for _, i := range imgFrames {
			if i.Bounds().Dy() > maxHeight {
				maxHeight = i.Bounds().Dy()
			}
			width += i.Bounds().Dx()
		}
		contactSheet := image.NewRGBA(image.Rect(0, 0, width, maxHeight))
		x := 0
		for _, i := range imgFrames {
			r := image.Rect(x, 0, x+i.Bounds().Dx(), i.Bounds().Dy())
			draw.Draw(contactSheet, r, i, image.Pt(0, 0), draw.Over)
			x += i.Bounds().Dx()
		}

		palette := make([]color.Color, 256)
		var numPalette int
		if useSmallPalette {
			numPalette = 1
			palette[0] = color.RGBA{0, 255, 0, 255}
		} else {
			numPalette = 4
			palette[0] = color.RGBA{0, 255, 0, 255}
			palette[1] = color.RGBA{128, 128, 128, 255}
			palette[2] = color.RGBA{255, 128, 0, 255}
			palette[3] = color.RGBA{255, 128, 0, 255}
		}
		paletteImg = image.NewPaletted(image.Rect(0, 0, width, maxHeight), palette)

		mcq := MedianCutQuantizer{255 - numPalette, nil}
		mcq.Quantize(paletteImg, image.Rect(0, 0, width, maxHeight), contactSheet, image.Pt(0, 0))

		/*
			fCs, _ := os.Create("contactsheet_out.png")
			png.Encode(fCs, contactSheet)
			fCs.Close()
			fPal, _ := os.Create("palette_out.png")
			png.Encode(fPal, paletteImg)
			fPal.Close()
		*/
		log.Printf("palette size: %d", len(paletteImg.Palette))
		paletteImg.Palette[0] = color.RGBA{0, 255, 0, 255}
		if !useSmallPalette {
			paletteImg.Palette[1] = color.RGBA{128, 128, 128, 255}
			if len(paletteImg.Palette) > 2 {
				paletteImg.Palette[2] = color.RGBA{255, 128, 0, 255}
			}
			if len(paletteImg.Palette) > 3 {
				paletteImg.Palette[3] = color.RGBA{255, 128, 0, 255}
			}
		}
		d.colorMap = paletteImg.Palette
	}

	for _, i := range imgFrames {
		img := image.NewPaletted(i.Bounds(), d.colorMap)
		b := i.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				col := i.At(x, y)
				replace, ok := d.replaceColor[col]
				_, _, _, a := col.RGBA()
				if a == 0 {
					img.Set(x, y, d.colorMap[0])
				} else if ok {
					img.Set(x, y, d.colorMap[replace])
				} else {
					img.Set(x, y, col)
				}
			}
		}

		d.image = append(d.image, *img)
	}

	return nil
}

func (d *decoder) readv1(r io.ReadSeeker, data []byte) error {
	d.Frames = make([]BamFrame, uint64(d.Header.Frames))
	r.Seek(int64(d.Header.FrameOffset), 0)
	err := binary.Read(r, binary.LittleEndian, &d.Frames)
	if err != nil {
		return fmt.Errorf("Unable to read frames from bam")
	}
	for _, frame := range d.Frames {
		if d.width < int(frame.Width)+int(frame.CenterX)*2 {
			d.width = int(frame.Width) + int(frame.CenterX)*2
		}
		if d.height < int(frame.Height)+int(frame.CenterY)*2 {
			d.height = int(frame.Height) + int(frame.CenterY)*2
		}
	}

	d.Cycles = make([]BamCycle, uint64(d.Header.Cycles))
	binary.Read(r, binary.LittleEndian, &d.Cycles)

	d.Palette = make([]uint32, 256)
	r.Seek(int64(d.Header.PaletteOffset), 0)
	binary.Read(r, binary.LittleEndian, &d.Palette)
	d.colorMap = make([]color.Color, 256)
	for idx, c := range d.Palette {
		r, g, b := uint8((c>>16)&0xff), uint8((c>>8)&0xff), uint8((c>>0)&0xff)
		d.colorMap[idx] = color.RGBA{r, g, b, 0xff}
	}
	d.colorMap[d.Header.CompressedColor] = color.RGBA{0, 0xff, 0, 0xff}

	var totalFrameLut = 0
	d.sequences = make([]BamSequence, len(d.Cycles))
	for idx, cycle := range d.Cycles {
		d.sequences[idx] = BamSequence{int(cycle.FrameIndex), int(cycle.FrameCount)}
		if int(cycle.FrameCount)+int(cycle.FrameIndex) > totalFrameLut {
			totalFrameLut = int(cycle.FrameCount) + int(cycle.FrameIndex)
		}
	}
	d.FrameLUT = make([]int16, totalFrameLut)
	r.Seek(int64(d.Header.FrameLutOffset), 0)
	binary.Read(r, binary.LittleEndian, &d.FrameLUT)

	for _, frame := range d.Frames {
		if frame.Width == 0 || frame.Height == 0 {
			img := image.NewPaletted(image.Rect(0, 0, 1, 1), d.colorMap)
			d.image = append(d.image, *img)
			continue
		}
		img := image.NewPaletted(image.Rect(0, 0, int(frame.Width), int(frame.Height)), d.colorMap)
		// uncompressed
		if frame.FrameOffset&0x80000000 != 0 {
			r.Seek(int64(frame.FrameOffset&0x7FFFFFFF), 0)

			binary.Read(r, binary.LittleEndian, &img.Pix)
		} else {
			dataOffset := 0
			compressed := false
			for _, entry := range data[frame.FrameOffset:] {
				if compressed {
					for i := 0; i <= int(entry); i++ {
						if dataOffset >= int(frame.Width*frame.Height) {
							break
						}
						img.Pix[dataOffset] = d.Header.CompressedColor
						dataOffset += 1
					}
					compressed = false
				} else {
					if entry == d.Header.CompressedColor {
						compressed = true
					} else {
						img.Pix[dataOffset] = entry
						dataOffset += 1
					}
				}
				if dataOffset >= int(frame.Width*frame.Height) {
					break
				}
			}
		}
		d.image = append(d.image, *img)
	}
	return nil
}
func (d *decoder) readv2(r io.ReadSeeker, data []byte, key *KEY) error {
	if key == nil {
		return fmt.Errorf("V2 bams not supported if no key specified")
	}
	var header BamHeaderV2
	r.Seek(0, os.SEEK_SET)
	err := binary.Read(r, binary.LittleEndian, &header)
	if err != nil {
		return fmt.Errorf("Unable to read frames from bam")
	}

	v2Frames := make([]BamFrameV2, uint64(header.Frames))
	r.Seek(int64(header.FramesOffset), os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &v2Frames)
	if err != nil {
		return fmt.Errorf("Unable to read frames from bam")
	}
	for _, frame := range v2Frames {
		if d.width < int(frame.Width)+int(frame.CenterX)*2 {
			d.width = int(frame.Width) + int(frame.CenterX)*2
		}
		if d.height < int(frame.Height)+int(frame.CenterY)*2 {
			d.height = int(frame.Height) + int(frame.CenterY)*2
		}
	}

	d.Cycles = make([]BamCycle, uint64(header.Sequences))
	r.Seek(int64(header.SequencesOffset), os.SEEK_SET)
	binary.Read(r, binary.LittleEndian, &d.Cycles)

	quads := make([]BamMosaicQuad, uint64(header.Quads))
	r.Seek(int64(header.QuadsOffset), os.SEEK_SET)
	binary.Read(r, binary.LittleEndian, &quads)

	for _, frame := range v2Frames {
		source := image.NewRGBA(image.Rect(0, 0, d.width, d.height))
		for i := frame.QuadStart; i < frame.QuadStart+frame.QuadCount; i++ {
			quad := quads[i]
			filename := fmt.Sprintf("mos%04d.pvrz", quad.Texture)
			pvrzData, err := key.OpenFile(filename)
			if err != nil {
				return fmt.Errorf("Error loading pvrz: %s %v", filename, err)
			}
			buf := bytes.NewReader(pvrzData)
			tex, err := NewPVRTexture(buf)
			if err != nil {
				return fmt.Errorf("Error parsing pvrz data: %v", err)
			}
			rect := image.Rect(int(quad.SX), int(quad.SY), int(quad.SX+quad.W), int(quad.SY+quad.H))
			pt := image.Pt(int(quad.X), int(quad.Y))
			draw.Draw(source, rect, tex.Image, pt, draw.Over)
		}
		d.imageRgba = append(d.imageRgba, *source)
	}

	return nil
}

func (d *decoder) decode(r io.Reader, configOnly bool, key *KEY) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("Unable to read bam: %v", err)
	}
	bamFile := bytes.NewReader(data)
	binary.Read(bamFile, binary.LittleEndian, &d.Header)

	if string(d.Header.Signature[0:]) == "BAMC" {
		bamFile.Seek(0x0C, os.SEEK_SET)
		r, err := zlib.NewReader(bamFile)
		if err != nil {
			return fmt.Errorf("Unable to zlib decompress BAMC file: %v", err)
		}
		uncompressed, err := ioutil.ReadAll(r)
		bamFile = bytes.NewReader(uncompressed)
		binary.Read(bamFile, binary.LittleEndian, &d.Header)
		data = uncompressed
	}

	if string(d.Header.Signature[0:]) != "BAM " {
		return fmt.Errorf("First 4 bytes not BAM ")
	}

	//
	if string(d.Header.Version[0:]) == "V2  " {
		return d.readv2(bamFile, data, key)
	} else if string(d.Header.Version[0:]) == "V1  " {
		return d.readv1(bamFile, data)
	}

	return nil
}

func OpenBAM(r io.ReadSeeker, key *KEY) (*BAM, error) {
	var d decoder
	if err := d.decode(r, false, key); err != nil {
		return nil, err
	}
	bam := &BAM{
		Image:           d.image,
		ImageRgba:       d.imageRgba,
		Sequences:       d.sequences,
		SequenceToImage: d.FrameLUT,
		Width:           d.width,
		Height:          d.height,
		Frames:          d.Frames,
	}
	return bam, nil
}

func OpenBAMD(r io.ReadSeeker, palettePath string) (*BAM, error) {
	var d decoder

	if _, err := os.Stat(palettePath); err == nil {
		log.Printf("Using palette at: %s\n", palettePath)
		paletteFile, err := os.Open(palettePath)
		if err != nil {
			return nil, fmt.Errorf("Unable to open palette %s: %v", palettePath, err)
		}
		defer paletteFile.Close()

		palette_template_img, err := png.Decode(paletteFile)
		if err != nil {
			return nil, fmt.Errorf("Unable to decode png %s: %v", palettePath, err)
		}
		d.colorMap = palette_template_img.ColorModel().(color.Palette)
		d.customPalette = true
	}
	if err := d.decode_bamd(r); err != nil {
		return nil, err
	}
	bam := &BAM{
		Image:           d.image,
		Sequences:       d.sequences,
		SequenceToImage: d.FrameLUT,
		Width:           d.width,
		Height:          d.height,
		Frames:          d.Frames,
		Palette:         d.colorMap,
	}
	bam.RebuildSequencesAndDropFrames()

	return bam, nil
}

func ColorInPalette(color color.Color, palette color.Palette) bool {
	for i := 0; i < len(palette); i++ {
		if palette[i] == color {
			return true
		}
	}
	return false
}

func MakeBamFromGif(animation *gif.GIF, sequences []image.Point) (*BAM, error) {
	maxHeight, maxWidth := 0, 0

	for _, img := range animation.Image {
		if img.Bounds().Dy() > maxHeight {
			maxHeight = img.Bounds().Dy()
		}
		if img.Bounds().Dx() > maxWidth {
			maxWidth = img.Bounds().Dx()
		}
	}

	bam := &BAM{Width: maxWidth, Height: maxHeight}
	bam.Frames = make([]BamFrame, len(animation.Image))
	bam.Sequences = make([]BamSequence, len(sequences))
	bam.Image = make([]image.Paletted, len(animation.Image))

	bam.Palette = make([]color.Color, 256)
	currentPaletteEntry := 1
	bam.Palette[0] = color.RGBA{0, 255, 0, 255}

	for idx, img := range animation.Image {
		bam.Frames[idx].Width = uint16(img.Bounds().Dx())
		bam.Frames[idx].Height = uint16(img.Bounds().Dy())
		bam.Frames[idx].CenterX = int16(img.Bounds().Min.X)
		bam.Frames[idx].CenterY = int16(img.Bounds().Min.Y)
		for _, color := range img.Palette {
			if !ColorInPalette(color, bam.Palette) {
				_, _, _, a := color.RGBA()
				if currentPaletteEntry < 256 {
					if a == 0xffff {
						bam.Palette[currentPaletteEntry] = color
						currentPaletteEntry++
					}
				} else {
					log.Printf("WARNING: palette overflow, unable to place %v %d", color, currentPaletteEntry)
				}
			}
		}
	}
	for i := currentPaletteEntry; i < len(bam.Palette); i++ {
		bam.Palette[i] = color.RGBA{0, 0, 0, 0}
	}
	for idx, img := range animation.Image {
		b := img.Bounds()
		rect := image.Rectangle{b.Min, b.Max}
		bam.Image[idx] = *image.NewPaletted(rect, bam.Palette)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				col := img.At(x, y)
				_, _, _, a := col.RGBA()
				// convert transparent gif pixels to our transparent pixel
				if a == 0 {
					col = bam.Palette[0]
				}
				bam.Image[idx].Set(x, y, bam.Palette.Convert(col))
			}
		}

	}

	for idx, seq := range sequences {
		bam.Sequences[idx].Start = seq.X
		bam.Sequences[idx].Count = seq.Y
		startFrame := seq.X
		for i := seq.X; i < seq.X+seq.Y; i++ {
			bam.SequenceToImage = append(bam.SequenceToImage, int16(startFrame))
			startFrame++
		}
	}

	return bam, nil
}

func (bam *BAM) ExpandAndCenterImages() {
	maxW, maxH := 0, 0
	for idx, img := range bam.Image {
		f := bam.Frames[idx]
		dx := img.Bounds().Dx()
		dy := img.Bounds().Dy()

		if f.CenterX < 0 {
			dx += int(f.CenterX) * -2
		} else {
			dx += int(f.CenterX) * 2
		}
		if f.CenterY < 0 {
			dy += int(f.CenterY) * -2
		} else {
			dy += int(f.CenterY) * 2
		}

		if dx > maxW {
			maxW = dx
		}
		if dy > maxH {
			maxH = dy
		}
	}
	log.Printf("MaxX: %d MaxY: %d\n", maxW, maxH)
	for idx, img := range bam.Image {
		f := bam.Frames[idx]
		if f.CenterX != 0 || f.CenterY != 0 {
			offsetX := maxW/2 - int(f.CenterX)
			offsetY := maxH/2 - int(f.CenterY)

			i := image.NewPaletted(image.Rect(0, 0, maxW, maxH), img.Palette)
			draw.Draw(i, image.Rect(offsetX, offsetY, offsetX+int(f.Width), offsetY+int(f.Height)), &img, image.ZP, draw.Over)
			bam.Image[idx] = *i

		}
	}
}

func (bam *BAM) MakeGif(outputPath string, name string) error {
	bam.ExpandAndCenterImages()
	for idx, seq := range bam.Sequences {
		if seq.Start >= 0 && seq.Count > 0 {
			pathname := filepath.Join(outputPath, fmt.Sprintf("%s_%03d.gif", name, idx))
			g := gif.GIF{}
			g.Image = make([]*image.Paletted, seq.Count)
			g.Delay = make([]int, seq.Count)
			g.LoopCount = 0

			for iIdx := seq.Start; iIdx < seq.Start+seq.Count; iIdx++ {
				imgIdx := int(bam.SequenceToImage[iIdx])

				g.Image[iIdx-seq.Start] = &bam.Image[imgIdx]
				g.Delay[iIdx-seq.Start] = 10
			}
			outFile, err := os.Create(pathname)
			if err != nil {
				return fmt.Errorf("Unable to create file %s: %v", pathname, err)
			}
			gif.EncodeAll(outFile, &g)

			outFile.Close()
		}
	}
	return nil
}

type bamcHeader struct {
	Signature, Version [4]byte
	Length             uint32
}

func (bam *BAM) MakeBam(wRaw io.Writer) error {
	var w io.Writer
	var b *bytes.Buffer
	bamc := true
	bamrle := true
	if bamc {
		// keep a reference to our zlib writer around so we can close it/flush it
		b = bytes.NewBuffer([]byte{})
		w = b
	} else {
		w = wRaw
	}
	header := BamHeader{}
	cycleEntries := make([]BamCycle, len(bam.Sequences))

	header.Signature = [4]byte{'B', 'A', 'M', ' '}
	header.Version = [4]byte{'V', '1', ' ', ' '}
	header.Frames = uint16(len(bam.Frames))
	header.Cycles = uint8(len(bam.Sequences))
	header.FrameOffset = uint32(binary.Size(header))
	header.PaletteOffset = uint32(int(header.FrameOffset)+binary.Size(bam.Frames[0])*len(bam.Frames)) + uint32(binary.Size(cycleEntries))
	header.FrameLutOffset = header.PaletteOffset + 4*256
	header.CompressedColor = 0

	err := binary.Write(w, binary.LittleEndian, header)
	if err != nil {
		return fmt.Errorf("Unable to write header: %v", err)
	}

	if bamrle {
		bam.RleFrame = make([][]uint8, len(bam.Image))
		for idx, img := range bam.Image {
			bam.RleFrame[idx] = rleBam(img.Pix, header.CompressedColor)
		}
	}

	frameDataStart := header.FrameLutOffset + uint32(binary.Size(bam.SequenceToImage))
	lastOffset := 0
	for idx, frame := range bam.Frames {
		bam.Frames[idx].FrameOffset = frameDataStart + uint32(lastOffset)
		if !bamrle {
			bam.Frames[idx].FrameOffset |= 0x80000000
			lastOffset += int(frame.Width) * int(frame.Height)

		} else {
			lastOffset += len(bam.RleFrame[idx])
		}
	}
	binary.Write(w, binary.LittleEndian, bam.Frames)

	for idx, seq := range bam.Sequences {
		cycleEntries[idx].FrameCount = int16(seq.Count)
		cycleEntries[idx].FrameIndex = int16(seq.Start)
	}
	binary.Write(w, binary.LittleEndian, cycleEntries)

	palette := make([]uint32, 256)
	palette[0] = (0<<24 | 0<<16 | 255<<8 | 0)
	for i := 1; i < len(bam.Palette); i++ {
		r, g, b, a := bam.Palette[i].RGBA()
		r = r >> 8
		g = g >> 8
		b = b >> 8
		a = 0xff

		palette[i] = (uint32(a) << 24) | (uint32(r) << 16) | (uint32(g) << 8) | uint32(b)
	}
	binary.Write(w, binary.LittleEndian, palette)

	binary.Write(w, binary.LittleEndian, bam.SequenceToImage)
	for idx, img := range bam.Image {
		if bamrle {
			w.Write(bam.RleFrame[idx])
		} else {
			w.Write(img.Pix)
		}
	}

	if bamc {
		h := bamcHeader{Signature: [4]byte{'B', 'A', 'M', 'C'}, Version: [4]byte{'V', '1', ' ', ' '}}
		h.Length = uint32(b.Len())
		binary.Write(wRaw, binary.LittleEndian, h)

		var data bytes.Buffer
		zw := zlib.NewWriter(&data)
		zw.Write(b.Bytes())
		zw.Close()

		_, err := wRaw.Write(data.Bytes())
		if err != nil {
			return fmt.Errorf("Unable to write compressed data to BAMC: %v", err)
		}
	}

	return nil
}

func rleBam(pix []uint8, rleKey uint8) []byte {
	out := make([]byte, 0)
	inTransparent := false
	for idx := range pix {
		if pix[idx] == rleKey {
			if inTransparent {
				out[len(out)-1]++
				if out[len(out)-1] == 255 {
					inTransparent = false
				}
			} else {
				out = append(out, pix[idx])
				out = append(out, 0)
				inTransparent = true
			}
		} else {
			inTransparent = false
			out = append(out, pix[idx])
		}
	}
	return out
}

func (bam *BAM) MakeBamd(output string, name string, mirror bool, offset_x int, offset_y int) {
	usedFrames := map[int]bool{}

	for _, seq := range bam.Sequences {
		for v := seq.Start; v < seq.Start+seq.Count; v++ {
			frame := bam.SequenceToImage[v]
			usedFrames[int(frame)] = true
		}
	}

	frames := make([]int, len(usedFrames))
	i := 0
	for k, _ := range usedFrames {
		frames[i] = k
		i++
	}
	sort.Ints(frames)

	for _, idx := range frames {
		frame := bam.Frames[idx]
		pathname := strings.Replace(path.Join(output, fmt.Sprintf("%s_%03d.png", name, idx)), "\\", "/", -1)
		if mirror {
			fmt.Printf("frame f%05d \"%s\" %d %d\n", idx, pathname, int(frame.Width)-int(frame.CenterX)+offset_x, int(frame.CenterY)+offset_y)
		} else {
			fmt.Printf("frame f%05d \"%s\" %d %d\n", idx, pathname, int(frame.CenterX)+offset_x, int(frame.CenterY)+offset_y)
		}

		i := image.NewPaletted(image.Rect(0, 0, int(frame.Width), int(frame.Height)), bam.Image[0].Palette)
		draw.Draw(i, image.Rect(0, 0, int(frame.Width), int(frame.Height)), &bam.Image[idx], image.ZP, draw.Over)
		if mirror {
			bounds := i.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X/2; x++ {
					c1 := i.At(x, y)
					c2 := i.At(bounds.Max.X-x, y)
					i.Set(x, y, c2)
					i.Set(bounds.Max.X-x, y, c1)
				}
			}
		}
		f, err := os.Create(pathname)
		if err != nil {
			log.Fatalf("Unable to create %s: %v", pathname, err)
		}

		//var pi image.PalettedImage
		//pi = i
		png.Encode(f, i)
		f.Close()
	}
	fmt.Printf("\n\n")
	for idx, seq := range bam.Sequences {
		fmt.Printf("sequence ")
		for v := seq.Start; v < seq.Start+seq.Count; v++ {
			frame := bam.SequenceToImage[v]
			if frame >= 0 {
				fmt.Printf("f%05d ", bam.SequenceToImage[v])
			}
		}
		fmt.Printf(" // SEQ %d\n", idx)
	}
}

func colorsEqual(a, b color.Color) bool {
	r1, b1, g1, a1 := a.RGBA()
	r2, b2, g2, a2 := b.RGBA()
	return r1 == r2 && b1 == b2 && g1 == g2 && a1 == a2
}

func trimBounds(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	firstX := bounds.Max.X
	firstY := bounds.Max.Y
	lastX := bounds.Min.X
	lastY := bounds.Min.Y
	transColor := color.RGBA{0, 255, 0, 255}
	transCount := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			if !colorsEqual(c, transColor) {
				if y < firstY {
					firstY = y
				}
				if y > lastY {
					lastY = y
				}
				if x < firstX {
					firstX = x
				}
				if x > lastX {
					lastX = x
				}
			} else {
				transCount++
			}
		}
	}

	// Image is entirely transparent, so just return a 1x1 rect
	if transCount == bounds.Dx()*bounds.Dy() {
		firstX = 0
		firstY = 0
		lastX = 1
		lastY = 1
	}

	return image.Rect(firstX, firstY, lastX, lastY)
}

func (bam *BAM) RebuildSequencesAndDropFrames() {
	foundFrames := map[int]bool{}

	for _, seq := range bam.Sequences {
		for v := seq.Start; v < seq.Start+seq.Count; v++ {
			foundFrames[int(bam.SequenceToImage[v])] = true
		}
	}

	newFrames := make([]BamFrame, 0)
	newImages := make([]image.Paletted, 0)
	framesRemoved := 1
	for idx, _ := range bam.Frames {
		found, ok := foundFrames[idx]
		if found && ok {
			newFrames = append(newFrames, bam.Frames[idx])
			newImages = append(newImages, bam.Image[idx])
		} else {
			log.Printf("dropping frame: %d\n", idx)
			for i, val := range bam.SequenceToImage {
				if int(val) > idx-framesRemoved {
					//log.Printf("S2I[%d] is %d changing to %d Idx is: %d\n", i, val, int(val) - 1, idx)
					bam.SequenceToImage[i] = int16(int(val) - 1)
				}
			}
			framesRemoved++
		}
	}
	bam.Image = newImages
	bam.Frames = newFrames
}

func (bam *BAM) MakeSpriteSheet(imgWriter io.Writer, jsonWriter io.Writer) {
	size := image.Point{0, 0}
	maxY := 0

	jsonData := fmt.Sprintf("{\"frames\": [\n")

	numFramesX := int(math.Sqrt(float64(len(bam.Frames))))
	seqSize := image.Point{0, 0}
	for idx, f := range bam.Frames {
		seqSize.X += int(f.Width)
		if int(f.Height) > seqSize.Y {
			seqSize.Y = int(f.Height)
		}
		if int(f.Height) > maxY {
			maxY = int(f.Height)
		}
		if (idx+1)%numFramesX == 0 {
			size.Y += seqSize.Y
			if seqSize.X > size.X {
				size.X = seqSize.X
			}
			seqSize = image.Point{0, 0}
		}
	}
	size.Y += maxY

	size.X = int(next_pow_two(uint(size.X)))
	size.Y = int(next_pow_two(uint(size.Y)))
	i := image.NewPaletted(image.Rect(0, 0, size.X, size.Y), bam.Image[0].Palette)
	maxY = 0
	y := 1
	x := 1
	lastFrame := len(bam.Frames) - 1
	for idx, frame := range bam.Frames {
		img := &bam.Image[idx]
		drawRect := image.Rect(
			x,
			y,
			x+int(frame.Width),
			y+int(frame.Height),
		)

		draw.Draw(i, drawRect, img, image.Point{0, 0}, draw.Src)

		jsonData += fmt.Sprintf("\t{\"filename\": \"frame_%d\", \"frame\": {\"x\":%d,\"y\":%d,\"w\":%d,\"h\":%d},\"rotated\": false,\"trimmed\":true,\"spriteSourceSize\": {\"x\":%d,\"y\":%d,\"w\":%d,\"h\":%d}, \"sourceSize\": {\"w\":%d,\"h\":%d}}", idx, drawRect.Min.X, drawRect.Min.Y, drawRect.Dx(), drawRect.Dy(), frame.CenterX*-1, frame.CenterY*-1, int16(frame.Width)+frame.CenterX, int16(frame.Height)+frame.CenterY, frame.Width, frame.Height)
		x += int(frame.Width) + 2

		if int(frame.Height) > maxY {
			maxY = int(frame.Height)
		}
		if (idx+1)%numFramesX == 0 {
			y += maxY
			//maxY = 0
			x = 1
		}
		if idx != lastFrame {
			jsonData += fmt.Sprintf(",\n")
		} else {
			jsonData += fmt.Sprintf("\n")
		}
	}
	i.Palette[0] = color.RGBA{0, 0, 0, 0}
	jsonData += fmt.Sprintf("]}\n")
	jsonWriter.Write([]byte(jsonData))
	png.Encode(imgWriter, i)
}
