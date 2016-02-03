package bg

import (
	"image"
	"image/color"
)

func pack4Bytes(bytes []uint8) uint32 {
	return uint32((uint32(bytes[3]) << 24) | (uint32(bytes[2]) << 16) | (uint32(bytes[1]) << 8) | uint32(bytes[0]))
}

func DecompressBlockDXT1Internal(block []uint8, img *image.RGBA, pt image.Point) {
	var temp, code uint32

	var color0, color1 uint32
	var r0, g0, b0, r1, g1, b1 uint32

	var i, j uint32

	color0 = ((uint32(block[1]) << 8) | uint32(block[0]))
	color1 = ((uint32(block[3]) << 8) | uint32(block[2]))

	temp = ((color0>>11)*255 + 16)
	r0 = ((temp/32 + temp) / 32)
	temp = (((color0&0x07E0)>>5)*255 + 32)
	g0 = ((temp/64 + temp) / 64)
	temp = ((color0&0x001F)*255 + 16)
	b0 = ((temp/32 + temp) / 32)

	temp = (color1>>11)*255 + 16
	r1 = ((temp/32 + temp) / 32)
	temp = ((color1&0x07E0)>>5)*255 + 32
	g1 = ((temp/64 + temp) / 64)
	temp = (color1&0x001F)*255 + 16
	b1 = ((temp/32 + temp) / 32)

	code = pack4Bytes(block[4:8])

	var c color.Color

	if color0 > color1 {
		for j = 0; j < 4; j++ {
			for i = 0; i < 4; i++ {
				var positionCode uint32

				alpha := uint8(255)

				positionCode = (code >> (2 * (4*j + i))) & 0x03

				switch positionCode {
				case 0:
					c = color.RGBA{uint8(r0), uint8(g0), uint8(b0), alpha}
				case 1:
					c = color.RGBA{uint8(r1), uint8(g1), uint8(b1), alpha}
				case 2:
					c = color.RGBA{uint8((2*r0 + r1) / 3), uint8((2*g0 + g1) / 3), uint8((2*b0 + b1) / 3), alpha}
				case 3:
					c = color.RGBA{uint8((r0 + 2*r1) / 3), uint8((g0 + 2*g1) / 3), uint8((b0 + 2*b1) / 3), alpha}
				}

				img.Set(int(i)+pt.X, int(j)+pt.Y, c)
			}
		}
	} else {
		for j = 0; j < 4; j++ {
			for i = 0; i < 4; i++ {
				var positionCode uint32

				alpha := uint8(255)

				positionCode = (code >> (2 * (4*j + i))) & 0x03

				switch positionCode {
				case 0:
					c = color.RGBA{uint8(r0), uint8(g0), uint8(b0), alpha}
				case 1:
					c = color.RGBA{uint8(r1), uint8(g1), uint8(b1), alpha}
				case 2:
					c = color.RGBA{uint8((r0 + r1) / 2), uint8((g0 + g1) / 2), uint8((b0 + b1) / 2), alpha}
				case 3:
					c = color.RGBA{0, 0, 0, alpha}
				}

				img.Set(int(i)+pt.X, int(j)+pt.Y, c)
			}
		}
	}
}
func DecompressBlockDXT5Internal(blockStorage []uint8, img *image.RGBA, pt image.Point) {
	var alpha0, alpha1 uint8
	var bits []uint8
	var alphaCode1 uint32
	var alphaCode2 uint16

	var color0, color1 uint32
	var r0, g0, b0, r1, g1, b1 uint8

	var temp, code uint32
	var c color.Color

	alpha0 = blockStorage[0]
	alpha1 = blockStorage[1]

	bits = blockStorage[2:]
	alphaCode1 = pack4Bytes(bits[2:6])
	alphaCode2 = uint16(bits[0]) | (uint16(bits[1]) << 8)

	color0 = ((uint32(blockStorage[9]) << 8) | uint32(blockStorage[8]))
	color1 = ((uint32(blockStorage[11]) << 8) | uint32(blockStorage[10]))

	temp = (color0>>11)*255 + 16
	r0 = (uint8)((temp/32 + temp) / 32)
	temp = ((color0&0x07E0)>>5)*255 + 32
	g0 = (uint8)((temp/64 + temp) / 64)
	temp = (color0&0x001F)*255 + 16
	b0 = (uint8)((temp/32 + temp) / 32)

	temp = (color1>>11)*255 + 16
	r1 = (uint8)((temp/32 + temp) / 32)
	temp = ((color1&0x07E0)>>5)*255 + 32
	g1 = (uint8)((temp/64 + temp) / 64)
	temp = (color1&0x001F)*255 + 16
	b1 = (uint8)((temp/32 + temp) / 32)

	code = pack4Bytes(blockStorage[12:])

	for j := 0; j < 4; j++ {
		for i := 0; i < 4; i++ {
			var finalAlpha uint8
			var alphaCode, alphaCodeIndex uint32
			var colorCode uint8

			alphaCodeIndex = uint32(3 * (4*j + i))
			if alphaCodeIndex <= 12 {
				alphaCode = uint32((alphaCode2 >> alphaCodeIndex) & 0x07)
			} else if alphaCodeIndex == 15 {
				alphaCode = uint32(alphaCode2>>15) | ((alphaCode1 << 1) & 0x06)
			} else /* alphaCodeIndex >= 18 && alphaCodeIndex <= 45 */ {
				alphaCode = (alphaCode1 >> (alphaCodeIndex - 16)) & 0x07
			}

			if alphaCode == 0 {
				finalAlpha = alpha0
			} else if alphaCode == 1 {
				finalAlpha = alpha1
			} else {
				if alpha0 > alpha1 {
					finalAlpha = (uint8)(((8-alphaCode)*uint32(alpha0) + (alphaCode-1)*uint32(alpha1)) / 7)
				} else {
					if alphaCode == 6 {
						finalAlpha = 0
					} else if alphaCode == 7 {
						finalAlpha = 255
					} else {
						finalAlpha = (uint8)(((6-alphaCode)*uint32(alpha0) + (alphaCode-1)*uint32(alpha1)) / 5)
					}
				}
			}

			colorCode = uint8((code >> uint32(2*(4*j+i))) & 0x03)

			switch colorCode {
			case 0:
				c = color.RGBA{uint8(r0), uint8(g0), uint8(b0), finalAlpha}
			case 1:
				c = color.RGBA{uint8(r1), uint8(g1), uint8(b1), finalAlpha}
			case 2:
				c = color.RGBA{uint8((2*r0 + r1) / 3), uint8((2*g0 + g1) / 3), uint8((2*b0 + b1) / 3), finalAlpha}
			case 3:
				c = color.RGBA{uint8((r0 + 2*r1) / 3), uint8((g0 + 2*g1) / 3), uint8((b0 + 2*b1) / 3), finalAlpha}
			}

			img.Set(int(i)+pt.X, int(j)+pt.Y, c)
		}
	}

}
