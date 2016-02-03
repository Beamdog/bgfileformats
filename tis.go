package bg

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type tisHeader struct {
	Signature, Version [4]byte
	TileCount          uint32
	TileLength         uint32
	HeaderSize         uint32
	TileSize           uint32
}

type Tis struct {
	Header   tisHeader
	image    *image.RGBA
	textures []tisPvrTexture
	tiles    []tisTile
	Name     string
	version  int
	imgTiles []*image.RGBA
}

type tisPVRTextureHeaderV3 struct {
	Version       uint32
	Flags         uint32
	PixelFormatlo uint32
	PixelFormathi uint32
	ColourSpace   uint32
	ChanelType    uint32
	Height        uint32
	Width         uint32
	Depth         uint32
	NumSurfaces   uint32
	Numfaces      uint32
	MIPMapCount   uint32
	MetaDataSize  uint32
}

type tisTile struct {
	Texture int32
	X       int32
	Y       int32
}

type tisPvrTexture struct {
	Header tisPVRTextureHeaderV3
	Image  *image.RGBA
	name   string
}

func (tis *Tis) SubImage(tileNum int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	tileBounds := image.Rect(0, 0, 64, 64)

	if tis.version == 1 {
		draw.Draw(img, tileBounds, tis.image, image.Pt(tileNum*64, 0), draw.Src)
	} else if tis.version == 2 {
		if tileNum < len(tis.tiles) && tileNum >= 0 {
			tile := tis.tiles[tileNum]
			log.Printf("tile: tilenum: %d %+v\n", tileNum, tile)
			draw.Draw(img, tileBounds, tis.textures[tile.Texture].Image, image.Pt(int(tile.X), int(tile.Y)), draw.Src)
		} else {
			log.Printf("Tilename: %d is greater then %d\n", tileNum, len(tis.tiles))
			panic("omg")
		}
	}

	return img
}

func (tis *Tis) writeV1(w io.Writer) error {
	h := tisHeader{
		Signature:  [4]byte{'T', 'I', 'S', ' '},
		Version:    [4]byte{'V', '1', ' ', ' '},
		TileCount:  uint32(len(tis.imgTiles)),
		TileLength: 5120,
		HeaderSize: uint32(binary.Size(tisHeader{})),
		TileSize:   64,
	}
	if err := binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}
	tileColors := [256]uint32{}
	tileData := [4096]uint8{}
	palette := make([]color.Color, 256)
	palette[0] = color.RGBA{0, 255, 0, 255}
	outImg := image.NewPaletted(image.Rect(0, 0, 64, 64), palette)
	for _, img := range tis.imgTiles {
		mcq := MedianCutQuantizer{255, nil}
		mcq.Quantize(outImg, image.Rect(0, 0, 64, 64), img, image.Point{})
		for idx, c := range outImg.Palette {
			r, g, b, _ := c.RGBA()
			tileColors[idx+1] = uint32((255 << 24) | ((r >> 8) << 16) | ((g >> 8) << 8) | (b >> 8))

		}
		tileColors[0] = uint32((255 << 24) | (255 << 8))
		for idx, i := range outImg.Pix {
			x := idx % img.Bounds().Dx()
			y := idx / img.Bounds().Dx()
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				tileData[idx] = 0
			} else if tileColors[i+1] == tileColors[0] {
				tileData[idx] = 0
			} else {
				tileData[idx] = i + 1
			}
		}
		binary.Write(w, binary.LittleEndian, tileColors)
		binary.Write(w, binary.LittleEndian, tileData)
	}

	return nil
}

func (tis *Tis) readV1(r io.ReadSeeker) error {
	if err := binary.Read(r, binary.LittleEndian, &tis.Header); err != nil {
		return err
	}
	tileColors := [256]uint32{}
	tileData := [4096]uint8{}
	tileRect := image.Rect(0, 0, 64, 64)
	tilePalette := make([]color.Color, 256)

	tis.image = image.NewRGBA(image.Rect(0, 0, int(tis.Header.TileCount)*64, 64))

	r.Seek(int64(tis.Header.HeaderSize), os.SEEK_SET)
	for idx := 0; idx < int(tis.Header.TileCount); idx++ {
		if err := binary.Read(r, binary.LittleEndian, &tileColors); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &tileData); err != nil {
			return err
		}
		for colorIdx, c := range tileColors {
			r, g, b := uint8((c>>16)&0xff), uint8((c>>8)&0xff), uint8((c>>0)&0xff)
			tilePalette[colorIdx] = color.RGBA{r, g, b, 0xff}
		}
		tile := image.NewPaletted(image.Rect(0, 0, 64, 64), tilePalette)
		tile.Pix = tileData[:]

		tileRect = image.Rect(idx*64, 0, (idx+1)*64, 64)

		draw.Draw(tis.image, tileRect, tile, image.Point{0, 0}, draw.Src)
	}
	return nil
}

func (tex *tisPvrTexture) decompressDXT1(block []uint8) {
	tex.Image = image.NewRGBA(image.Rect(0, 0, int(tex.Header.Width), int(tex.Header.Height)))
	//tex.Image = image.NewRGBA(image.Rect(0, 0, int(tex.Header.Width), 8))

	blockCountX := (tex.Header.Width + 3) / 4
	blockCountY := (tex.Header.Height + 3) / 4

	offset := 0

	for j := uint32(0); j < blockCountY; j++ {
		for i := uint32(0); i < blockCountX; i++ {
			DecompressBlockDXT1Internal(block[offset+int(i)*8:offset+int(i)*8+8], tex.Image, image.Pt(int(i*4), int(j*4)))
		}
		offset += int(blockCountX) * 8
	}
}

func (tex *tisPvrTexture) decompressDXT5(block []uint8) {
	tex.Image = image.NewRGBA(image.Rect(0, 0, int(tex.Header.Width), int(tex.Header.Height)))
	//tex.Image = image.NewRGBA(image.Rect(0, 0, int(tex.Header.Width), 8))

	blockCountX := (tex.Header.Width + 3) / 4
	blockCountY := (tex.Header.Height + 3) / 4

	offset := 0

	for j := uint32(0); j < blockCountY; j++ {
		for i := uint32(0); i < blockCountX; i++ {
			DecompressBlockDXT5Internal(block[offset+int(i)*16:offset+int(i)*16+16], tex.Image, image.Pt(int(i*4), int(j*4)))
		}
		offset += int(blockCountX) * 16
	}
}

func (tis *Tis) AddTile(img *image.RGBA) int {
	newImg := image.NewRGBA(image.Rect(0, 0, 64, 64))

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			//_,_,_,a := c.RGBA()
			newImg.Set(x, y, c)
		}
	}

	tis.imgTiles = append(tis.imgTiles, newImg)
	return len(tis.imgTiles) - 1
}

func NewPVRTexture(r io.ReadSeeker) (*tisPvrTexture, error) {
	tex := tisPvrTexture{}
	var destLen uint32

	binary.Read(r, binary.LittleEndian, &destLen)

	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	uncompressed, err := ioutil.ReadAll(zr)
	data := bytes.NewReader(uncompressed)
	data.Seek(0, os.SEEK_SET)
	binary.Read(data, binary.LittleEndian, &tex.Header)

	switch tex.Header.PixelFormatlo {
	case 0: // 2BPP RGB PVRTC
	case 1: // 2BPP RGBA PVRTC
	case 2: // 4BPP RGB PVRTC
	case 3: // 4BPP RGBA PVRTC
	case 7: // DXT1
		tex.decompressDXT1(uncompressed[binary.Size(tex.Header):])
	case 9: // DXT3
		return nil, fmt.Errorf("DXT3 decompression not supported")
	case 11: // DXT5
		tex.decompressDXT5(uncompressed[binary.Size(tex.Header):])
	}

	return &tex, nil
}

func (tis *Tis) readV2(r io.ReadSeeker, fileLen int64, root string) error {
	tileCount := fileLen / int64(binary.Size(&tisTile{}))
	tiles := make([]tisTile, tileCount)
	if err := binary.Read(r, binary.LittleEndian, &tiles); err != nil {
		return fmt.Errorf("Unable to read tiles: %v", err)
	}
	textures := make([]tisPvrTexture, 0)
	for i := 0; ; i++ {
		fname := fmt.Sprintf("%c%.6s%02d.pvrz", tis.Name[0], tis.Name[2:], i)
		f, err := os.Open(filepath.Join(root, fname))
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return fmt.Errorf("Error loading pvrz: %s %v", tis.Name[0], err)
		}

		log.Printf("Loading: %s\n", fname)
		tex, err := NewPVRTexture(f)
		f.Close()
		if err != nil {
			return err
		}
		textures = append(textures, *tex)
	}
	tis.textures = textures
	tis.tiles = tiles

	return nil
}

func (tis *Tis) Write(w io.Writer) error {
	if tis.version == 1 {
		tis.writeV1(w)
	} else if tis.version == 2 {
		return errors.New("Unable to write TISv2")
	}
	return nil
}

func NewTis() *Tis {
	tis := Tis{version: 1}
	return &tis
}

func OpenTis(r io.ReadSeeker, name string, root string) (*Tis, error) {
	tis := Tis{Name: name}

	var err error
	header := tisHeader{}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("Unable to read header: %v", err)
	}

	fileLen, err := r.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}
	r.Seek(0, os.SEEK_SET)

	if header.Signature != [4]byte{'T', 'I', 'S', ' '} {
		tis.version = 2
		err = tis.readV2(r, fileLen, root)
	} else {
		tis.version = 1
		err = tis.readV1(r)
	}
	if err != nil {
		return nil, fmt.Errorf("Unable to read tis data: %v", err)
	}

	return &tis, nil
}
