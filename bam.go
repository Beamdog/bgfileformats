package bg

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"image/gif"
	"bytes"
	"compress/zlib"
	"text/scanner"
	"strings"
	"strconv"
	"path/filepath"
	"path"
)

type BAM struct {
    Image       []image.Paletted
    Sequences   []BamSequence
    SequenceToImage []int16
    Width       int
    Height      int
    Frames      []BamFrame
	Palette     color.Palette
}
type BamSequence struct {
    Start       int
    Count       int
}

type BamFrame struct {
    Width, Height    uint16
    CenterX, CenterY int16
    FrameOffset      uint32
}

type BamCycle struct {
    FrameCount int16
    FrameIndex uint16
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

type BamCHeader struct {
    Signature, Version [4]byte
    UncompressedSize uint32
}

type decoder struct {
    Header              BamHeader
    Frames              []BamFrame
    Cycles              []BamCycle
    Palette             []uint32
    FrameLUT            []int16
    FrameData           [][]byte
    image               []image.Paletted
    colorMap            color.Palette
    width               int
    height              int
    sequences           []BamSequence
}

func (d *decoder) decode_bamd(r io.Reader) error {
	var s scanner.Scanner
	s.Init(r)
	s.Whitespace = 1<<'\t' | 1 << ' '
	frameNames := map[string] int {}
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if strings.ToLower(s.TokenText()) == "frame" {
			center_x, center_y := 0,0
			tok = s.Scan()
			name := strings.TrimSpace(s.TokenText())
			tok = s.Scan()
			path := strings.Trim(s.TokenText(), "\"")
			tok = s.Scan()
			if s.TokenText() != "\n" {
				center_x, _ = strconv.Atoi(s.TokenText())
				tok = s.Scan()
				center_y, _ = strconv.Atoi(s.TokenText())
			}
			imgFile, err := os.Open(filepath.Clean(path))
			if err != nil {
				return err
			}
			img, err := png.Decode(imgFile)
			if err != nil {
				return err
			}
			imgFile.Close()

			paletted_img := image.NewPaletted(img.Bounds(), img.ColorModel().(color.Palette))
			paletted_img.Palette[0] = color.RGBA{0,255,0,255}
			bounds := img.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					_, _, _, a := img.At(x,y).RGBA()
					if a == 0 {
						paletted_img.Set(x,y, color.RGBA{0,255,0,255})
					} else {
						paletted_img.Set(x,y, img.At(x,y))
					}
				}
			}

			frame := BamFrame{uint16(img.Bounds().Size().X), uint16(img.Bounds().Size().Y), int16(center_x), int16(center_y), 0}

			frameNames[name] = len(d.Frames)
			d.Frames = append(d.Frames, frame)
			d.image = append(d.image, *paletted_img)
		} else if strings.ToLower(s.TokenText()) == "sequence" {
			frames := make([]string, 0)
			sequences := make([]uint16, 0)
			for tok = s.Scan(); s.TokenText() != "\n"; tok = s.Scan() {
				frame := strings.TrimSpace(s.TokenText())
				frames = append(frames, frame)
				sequences = append(sequences, uint16(frameNames[frame]))
			}
			var sequence BamSequence

			if len(frames) > 0 {
				newlut := make([]int16, len(d.FrameLUT) + len(frames))
				sequence.Start = len(d.FrameLUT)
				sequence.Count = len(frames)
				copy(newlut, d.FrameLUT)
				for i, f:= range frames {
					newlut[sequence.Start + i] = int16(frameNames[f])
				}
				d.FrameLUT = newlut
			} else {
				sequence.Start = -1
				sequence.Count = 0
			}

			d.sequences = append(d.sequences, sequence)
		}

	}
	d.colorMap = d.image[0].Palette


	return nil
}

func (d *decoder) decode(r io.Reader, configOnly bool) error {
    data, err := ioutil.ReadAll(r)
    if err != nil {
        log.Fatal("err", err)
    }
    bamFile := bytes.NewReader(data)
    binary.Read(bamFile, binary.LittleEndian, &d.Header)

    if string(d.Header.Signature[0:]) == "BAMC" {
        bamFile.Seek(0x0C, os.SEEK_SET)
        r, err := zlib.NewReader(bamFile)
        if err != nil {
            log.Fatal("err", err)
        }
        uncompressed, err := ioutil.ReadAll(r)
        bamFile = bytes.NewReader(uncompressed)
        binary.Read(bamFile, binary.LittleEndian, &d.Header)
        data = uncompressed
    }

    if string(d.Header.Signature[0:]) != "BAM " {
        log.Fatal("Not a bam")
    }

    d.Frames = make([]BamFrame, uint64(d.Header.Frames))
    d.FrameData = make([][]byte, uint64(d.Header.Frames))
    bamFile.Seek(int64(d.Header.FrameOffset), 0)
    err = binary.Read(bamFile, binary.LittleEndian, &d.Frames)
    if err != nil {
        log.Fatal("Err: ", err)
    }
    for _, frame := range d.Frames {
        if d.width < int(frame.Width) + int(frame.CenterX) * 2 {
            d.width = int(frame.Width) + int(frame.CenterX) * 2
        }
        if d.height < int(frame.Height) + int(frame.CenterY) * 2 {
            d.height = int(frame.Height) + int(frame.CenterY) * 2
        }
    }

    d.Cycles = make([]BamCycle, uint64(d.Header.Cycles))
    binary.Read(bamFile, binary.LittleEndian, &d.Cycles)

    d.Palette = make([]uint32, 256)
    bamFile.Seek(int64(d.Header.PaletteOffset), 0)
    binary.Read(bamFile, binary.LittleEndian, &d.Palette)
    d.colorMap = make([]color.Color, 256)
    for idx, c := range d.Palette {
        r, g, b := uint8((c >> 16) & 0xff), uint8((c >> 8 ) & 0xff), uint8((c >> 0) & 0xff)
        d.colorMap[idx] = color.RGBA{r, g, b, 0xff}
    }
    d.colorMap[d.Header.CompressedColor] = color.RGBA{0,0xff,0,0xff}

    var totalFrameLut = 0
    d.sequences = make([]BamSequence, len(d.Cycles))
    for idx, cycle := range d.Cycles {
        d.sequences[idx] = BamSequence{int(cycle.FrameIndex), int(cycle.FrameCount)}
        if int(cycle.FrameCount)+int(cycle.FrameIndex) > totalFrameLut {
            totalFrameLut = int(cycle.FrameCount) + int(cycle.FrameIndex)
        }
    }
    d.FrameLUT = make([]int16, totalFrameLut)
    bamFile.Seek(int64(d.Header.FrameLutOffset), 0)
    binary.Read(bamFile, binary.LittleEndian, &d.FrameLUT)

    for _, frame := range d.Frames {
        img := image.NewPaletted(image.Rect(0,0,int(frame.Width),int(frame.Height)), d.colorMap)
        // uncompressed
        if frame.FrameOffset&0x80000000 != 0 {
            bamFile.Seek(int64(frame.FrameOffset&0x7FFFFFFF), 0)

            binary.Read(bamFile, binary.LittleEndian, &img.Pix)
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

func OpenBAM(r io.ReadSeeker) (*BAM, error) {
    var d decoder
    if err := d.decode(r, false); err != nil {
        return nil, err
    }
    bam := &BAM{
        Image: d.image,
        Sequences: d.sequences,
        SequenceToImage: d.FrameLUT,
        Width: d.width,
        Height: d.height,
        Frames: d.Frames,
    }
    return bam, nil
}

func OpenBAMD(r io.ReadSeeker, palettePath string) (*BAM, error) {
	var d decoder
	if err := d.decode_bamd(r); err != nil {
		return nil, err
	}
	bam := &BAM{
        Image: d.image,
        Sequences: d.sequences,
        SequenceToImage: d.FrameLUT,
        Width: d.width,
        Height: d.height,
        Frames: d.Frames,
		Palette: d.colorMap,
    }
	if _, err := os.Stat(palettePath); err == nil {
		log.Printf("Using palette at: %s\n", palettePath)
		paletteFile, err := os.Open(palettePath)
		if err != nil {
			return nil, err
		}
		defer paletteFile.Close()

		palette_template_img, err := png.Decode(paletteFile)
		if err != nil {
			return nil, err
		}
		colors := make([]color.Color, 256)
		color := 0
		for i:= 0; i < palette_template_img.Bounds().Dx() * palette_template_img.Bounds().Dy() && color < len(colors); i++ {
			y := i / palette_template_img.Bounds().Dx();
			x := i % palette_template_img.Bounds().Dx();

			colors[color] = palette_template_img.At(x,y)
			color++
		}
		bam.Palette = colors
	}

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
	maxHeight, maxWidth := 0,0

	for _,img := range animation.Image {
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
	bam.Palette[0] = color.RGBA{0,255,0,255}

	for idx,img := range animation.Image {
		bam.Frames[idx].Width = uint16(img.Bounds().Dx())
		bam.Frames[idx].Height = uint16(img.Bounds().Dy())
		bam.Frames[idx].CenterX = int16(img.Bounds().Min.X)
		bam.Frames[idx].CenterY = int16(img.Bounds().Min.Y)
		for _, color := range img.Palette {
			if !ColorInPalette(color, bam.Palette) {
				_,_,_,a := color.RGBA()
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
		bam.Palette[i] = color.RGBA{0,0,0,0}
	}
	for idx, img := range animation.Image {
		b := img.Bounds()
		rect := image.Rectangle{b.Min, b.Max}
		bam.Image[idx] = *image.NewPaletted(rect, bam.Palette)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				col := img.At(x,y)
				_,_,_,a := col.RGBA()
				// convert transparent gif pixels to our transparent pixel
				if a == 0 {
					col = bam.Palette[0]
				}
				bam.Image[idx].Set(x,y, bam.Palette.Convert(col))
			}
		}

	}

	for idx,seq := range sequences {
		bam.Sequences[idx].Start = seq.X
		bam.Sequences[idx].Count = seq.Y
		startFrame := seq.X
		for i := seq.X; i < seq.X + seq.Y; i++ {
			bam.SequenceToImage = append(bam.SequenceToImage, int16(startFrame))
			startFrame++
		}
	}

	return bam, nil
}

func (bam *BAM) MakeGif(outputPath string, name string) error {
	for idx, seq := range bam.Sequences {
		pathname := filepath.Join(outputPath, fmt.Sprintf("%s_%03d.gif", name, idx))
		g := gif.GIF{}
		g.Image = make([]*image.Paletted, seq.Count)
		g.Delay = make([]int, seq.Count)
		g.LoopCount = 0
		for iIdx, _ := range bam.Image[seq.Start : seq.Start + seq.Count] {
			g.Image[iIdx] = &bam.Image[iIdx]
			g.Delay[iIdx] = 10
		}
		outFile, err := os.Create(pathname)
		if err != nil {
			return err
		}
		gif.EncodeAll(outFile, &g)

		outFile.Close()
	}
	return nil
}



func (bam *BAM) MakeBam(w io.Writer) error {
	header := BamHeader{}
	cycleEntries := make([]BamCycle, len(bam.Sequences))

	header.Signature = [4]byte{'B','A','M', ' '}
	header.Version = [4]byte{'V', '1', ' ', ' '}
	header.Frames = uint16(len(bam.Frames))
    header.Cycles = uint8(len(bam.Sequences))
	header.FrameOffset = uint32(binary.Size(header))
	header.PaletteOffset = uint32(int(header.FrameOffset) + binary.Size(bam.Frames[0]) * len(bam.Frames)) + uint32(binary.Size(cycleEntries))
	header.FrameLutOffset = header.PaletteOffset + 4 * 256
	header.CompressedColor = 0

	err := binary.Write(w, binary.LittleEndian, header)
	if err != nil {
		return err
	}


	frameDataStart := header.FrameLutOffset + uint32(binary.Size(bam.SequenceToImage))
	lastOffset := 0
	for idx, frame := range bam.Frames {
		bam.Frames[idx].FrameOffset = frameDataStart + uint32(lastOffset) | 0x80000000
		lastOffset += int(frame.Width) * int(frame.Height)
	}
	binary.Write(w, binary.LittleEndian, bam.Frames)

	for idx, seq := range bam.Sequences {
        cycleEntries[idx].FrameCount = int16(seq.Count)
		cycleEntries[idx].FrameIndex = uint16(seq.Start)
	}
	binary.Write(w, binary.LittleEndian, cycleEntries)

	palette := make([]uint32, 256)
	palette[0] = (0<<24 | 0<<16 | 255 << 8 | 0)
	for i := 1; i < len(bam.Palette); i++ {
		r,g,b,a := bam.Palette[i].RGBA()
		r = r >> 8;
		g = g >> 8;
		b = b >> 8;
		a = a >> 8;

		palette[i] = (uint32(a)<<24) | (uint32(r)<<16) | (uint32(g)<<8) | uint32(b)
	}
	binary.Write(w, binary.LittleEndian, palette)

	binary.Write(w, binary.LittleEndian, bam.SequenceToImage)
	for _, img := range bam.Image {
		binary.Write(w, binary.LittleEndian, img.Pix)

	}

	return nil
}


func (bam *BAM) MakeBamd(output string, name string, mirror bool, offset_x int, offset_y int) {
	for idx, frame := range bam.Frames {
		pathname := strings.Replace(path.Join(output,fmt.Sprintf("%s_%03d.png", name, idx)), "\\", "/", -1)
		if ( mirror ) {
			fmt.Printf("frame f%05d \"%s\" %d %d\n", idx, pathname, int(frame.Width) - int(frame.CenterX) + offset_x, int(frame.CenterY) + offset_y)
		} else {
			fmt.Printf("frame f%05d \"%s\" %d %d\n", idx, pathname, int(frame.CenterX) + offset_x, int(frame.CenterY) + offset_y)
		}

		i := image.NewPaletted(image.Rect(0, 0, int(frame.Width), int(frame.Height)), bam.Image[0].Palette)
		draw.Draw(i, image.Rect(0, 0, int(frame.Width), int(frame.Height)), &bam.Image[idx], image.ZP, draw.Over)
		if ( mirror ) {
			bounds := i.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X/2; x++ {
					c1 := i.At(x, y)
					c2 := i.At(bounds.Max.X - x, y)
					i.Set(x, y, c2)
					i.Set(bounds.Max.X - x, y, c1)
				}
			}
		}
		f, err := os.Create(pathname)
		if err != nil {
			log.Fatal(err)
		}

		//var pi image.PalettedImage
		//pi = i
		png.Encode(f, i)
		f.Close()
	}
	fmt.Printf("\n\n")
	for _, seq := range bam.Sequences {
		fmt.Printf("sequence ")
		for v := seq.Start; v < seq.Start + seq.Count; v++ {
			frame := bam.SequenceToImage[v]
			if frame >= 0 {
				fmt.Printf("f%05d ", bam.SequenceToImage[v])
			}
		}
		fmt.Printf("\n")
	}
}
