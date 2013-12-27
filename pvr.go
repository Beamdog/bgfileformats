package bg

func clamp(x, l, h uint32) uint32 {
	if x < l {
		return l
	} else if x > h {
		return h
	} else {
		return x
	}
}

func LIMIT_COORD(Val, Size uint32, AssumeImageTiles bool) uint32 {
	if AssumeImageTiles {
		return ((Val) & ((Size) - 1))
	} else {
		return clamp(Val, 0, Size-1)
	}
}

/*****************************************************************************
 * defines and consts
 *****************************************************************************/
const PT_INDEX uint32 = 2 // The Punch-through index

const BLK_Y_SIZE uint32 = 4 // always 4 for all 2D block types

const BLK_X_MAX uint32 = 8 // Max X dimension for blocks

const BLK_X_2BPP uint32 = 8 // dimensions for the two formats
const BLK_X_4BPP uint32 = 4

/*****************************************************************************
* constants
*****************************************************************************/

const PVRTEX_MIPMAP uint32 = (1 << 8)         // has mip map levels
const PVRTEX_TWIDDLE uint32 = (1 << 9)        // is twiddled
const PVRTEX_BUMPMAP uint32 = (1 << 10)       // has normals encoded for a bump map
const PVRTEX_TILING uint32 = (1 << 11)        // is bordered for tiled pvr
const PVRTEX_CUBEMAP uint32 = (1 << 12)       // is a cubemap/skybox
const PVRTEX_FALSEMIPCOL uint32 = (1 << 13)   // are there false coloured MIP levels
const PVRTEX_VOLUME uint32 = (1 << 14)        // is this a volume texture
const PVRTEX_ALPHA uint32 = (1 << 15)         // v2.1 is there transparency info in the texture
const PVRTEX_VERTICAL_FLIP uint32 = (1 << 16) // v2.1 is the texture vertically flipped

const PVRTEX_PIXELTYPE uint32 = 0xff        // pixel type is always in the last 16bits of the flags
const PVRTEX_IDENTIFIER uint32 = 0x21525650 // the pvr identifier is the characters 'P','V','R'

const PVRTEX_V1_HEADER_SIZE uint32 = 44 // old header size was 44 for identification purposes

const PVRTC2_MIN_TEXWIDTH uint32 = 16
const PVRTC2_MIN_TEXHEIGHT uint32 = 8
const PVRTC4_MIN_TEXWIDTH uint32 = 8
const PVRTC4_MIN_TEXHEIGHT uint32 = 8
const ETC_MIN_TEXWIDTH uint32 = 4
const ETC_MIN_TEXHEIGHT uint32 = 4
const DXT_MIN_TEXWIDTH uint32 = 4
const DXT_MIN_TEXHEIGHT uint32 = 4

/*!***********************************************************************
  @Function		util_number_is_power_2
  @Input		input A number
  @Returns		TRUE if the number is an integer power of two, else FALSE.
  @Description	Check that a number is an integer power of two, i.e.
				1, 2, 4, 8, ... etc.
				Returns FALSE for zero.
*************************************************************************/
func util_number_is_power_2(input uint32) bool {
	minus1 := uint32(0)

	if input == 0 {
		return false
	}

	minus1 = input - 1
	return ((input | minus1) == (input ^ minus1))
}

/*!***********************************************************************
 @Function		Unpack5554Colour
 @Input			pBlock
 @Input			ABColours
 @Description	Given a block, extract the colour information and convert
				to 5554 formats
*************************************************************************/
func Unpack5554Colour(pBlock []uint32, ABColours *[2][4]uint32) {
	var RawBits [2]uint32

	i := 0

	// Extract A and B
	RawBits[0] = pBlock[1] & (0xFFFE) // 15 bits (shifted up by one)
	RawBits[1] = pBlock[1] >> 16      // 16 bits

	// step through both colours
	for i = 0; i < 2; i++ {
		// If completely opaque
		if RawBits[i]&(1<<15) != 0 {
			// Extract R and G (both 5 bit)
			ABColours[i][0] = (RawBits[i] >> 10) & 0x1F
			ABColours[i][1] = (RawBits[i] >> 5) & 0x1F

			/*
				The precision of Blue depends on  A or B. If A then we need to
				replicate the top bit to get 5 bits in total
			*/
			ABColours[i][2] = RawBits[i] & 0x1F
			if i == 0 {
				ABColours[0][2] |= ABColours[0][2] >> 4
			}

			// set 4bit alpha fully on...
			ABColours[i][3] = 0xF
		} else {
			/*
				Extract R and G (both 4 bit).
				(Leave a space on the end for the replication of bits
			*/
			ABColours[i][0] = (RawBits[i] >> (8 - 1)) & 0x1E
			ABColours[i][1] = (RawBits[i] >> (4 - 1)) & 0x1E

			// replicate bits to truly expand to 5 bits
			ABColours[i][0] |= ABColours[i][0] >> 4
			ABColours[i][1] |= ABColours[i][1] >> 4

			// grab the 3(+padding) or 4 bits of blue and add an extra padding bit
			ABColours[i][2] = (RawBits[i] & 0xF) << 1

			/*
				expand from 3 to 5 bits if this is from colour A, or 4 to 5 bits if from
				colour B
			*/
			if i == 0 {
				ABColours[0][2] |= ABColours[0][2] >> 3
			} else {
				ABColours[0][2] |= ABColours[0][2] >> 4
			}

			// Set the alpha bits to be 3 + a zero on the end
			ABColours[i][3] = (RawBits[i] >> 11) & 0xE
		}
	}
}

/*!***********************************************************************
 @Function		UnpackModulations
 @Input			pBlock
 @Input			Do2bitMode
 @Input			ModulationVals
 @Input			ModulationModes
 @Input			StartX
 @Input			StartY
 @Description	Given the block and the texture type and it's relative
				position in the 2x2 group of blocks, extract the bit
				patterns for the fully defined pixels.
*************************************************************************/
func UnpackModulations(pBlock []uint32,
	Do2bitMode bool,
	ModulationVals *[8][16]uint32,
	ModulationModes *[8][16]uint32,
	StartX uint32,
	StartY uint32) {
	var BlockModMode uint32
	var ModulationBits uint32

	var x, y uint32

	BlockModMode = pBlock[1] & 1
	ModulationBits = pBlock[0]

	// if it's in an interpolated mode
	if Do2bitMode && BlockModMode != 0 {
		/*
			run through all the pixels in the block. Note we can now treat all the
			"stored" values as if they have 2bits (even when they didn't!)
		*/
		for y = 0; y < BLK_Y_SIZE; y++ {
			for x = 0; x < BLK_X_2BPP; x++ {
				ModulationModes[y+StartY][x+StartX] = BlockModMode

				// if this is a stored value...
				if ((x ^ y) & 1) == 0 {
					ModulationVals[y+StartY][x+StartX] = ModulationBits & 3
					ModulationBits >>= 2
				}
			}
		}
	} else if Do2bitMode {
		for y = 0; y < BLK_Y_SIZE; y++ {
			for x = 0; x < BLK_X_2BPP; x++ {
				ModulationModes[y+StartY][x+StartX] = BlockModMode

				// double the bits so 0=> 00, and 1=>11
				if ModulationBits&1 != 0 {
					ModulationVals[y+StartY][x+StartX] = 0x3
				} else {
					ModulationVals[y+StartY][x+StartX] = 0x0
				}
				ModulationBits >>= 1
			}
		}
	} else {
		for y = 0; y < BLK_Y_SIZE; y++ {
			for x = 0; x < BLK_X_4BPP; x++ {
				ModulationModes[y+StartY][x+StartX] = BlockModMode

				ModulationVals[y+StartY][x+StartX] = ModulationBits & 3
				ModulationBits >>= 2
			}
		}
	}

	// make sure nothing is left over
	//assert(ModulationBits==0);
}

/*!***********************************************************************
 @Function		InterpolateColours
 @Input			ColourP
 @Input			ColourQ
 @Input			ColourR
 @Input			ColourS
 @Input			Do2bitMode
 @Input			x
 @Input			y
 @Modified		Result
 @Description	This performs a HW bit accurate interpolation of either the
				A or B colours for a particular pixel.

				NOTE: It is assumed that the source colours are in ARGB 5554
				format - This means that some "preparation" of the values will
				be necessary.
*************************************************************************/
func InterpolateColours(ColourP [4]uint32,
	ColourQ [4]uint32,
	ColourR [4]uint32,
	ColourS [4]uint32,
	Do2bitMode bool,
	x uint32,
	y uint32,
	Result *[4]uint32) {
	var u, v, uscale, k, tmp1, tmp2 uint32
	var P, Q, R, S [4]uint32

	// Copy the colours
	for k = 0; k < 4; k++ {
		P[k] = ColourP[k]
		Q[k] = ColourQ[k]
		R[k] = ColourR[k]
		S[k] = ColourS[k]
	}

	// put the x and y values into the right range
	v = (y & 0x3) | ((^y & 0x2) << 1)

	if Do2bitMode {
		u = (x & 0x7) | ((^x & 0x4) << 1)
	} else {
		u = (x & 0x3) | ((^x & 0x2) << 1)
	}

	// get the u and v scale amounts
	v = v - BLK_Y_SIZE/2

	if Do2bitMode {
		u = u - BLK_X_2BPP/2
		uscale = 8
	} else {
		u = u - BLK_X_4BPP/2
		uscale = 4
	}

	for k = 0; k < 4; k++ {
		tmp1 = P[k]*uscale + u*(Q[k]-P[k])
		tmp2 = R[k]*uscale + u*(S[k]-R[k])

		tmp1 = tmp1*4 + v*(tmp2-tmp1)

		Result[k] = tmp1
	}

	// Lop off the appropriate number of bits to get us to 8 bit precision
	if Do2bitMode {
		// do RGB
		for k = 0; k < 3; k++ {
			Result[k] >>= 2
		}

		Result[3] >>= 1
	} else {
		// do RGB  (A is ok)
		for k = 0; k < 3; k++ {
			Result[k] >>= 1
		}
	}

	// sanity check
	//for k = 0; k < 4; k++ {
	//	assert(Result[k] < 256);
	//}

	/*
		Convert from 5554 to GB 5.3 => 8
	*/
	for k = 0; k < 3; k++ {
		Result[k] += Result[k] >> 5
	}

	Result[3] += Result[3] >> 4

	// 2nd sanity check
	//for k = 0; k < 4; k++ {
	//	assert(Result[k] < 256);
	//}

}

/*!***********************************************************************
 @Function		GetModulationValue
 @Input			x
 @Input			y
 @Input			Do2bitMode
 @Input			ModulationVals
 @Input			ModulationModes
 @Input			Mod
 @Input			DoPT
 @Description	Get the modulation value as a numerator of a fraction of 8ths
*************************************************************************/
func GetModulationValue(x uint32,
	y uint32,
	Do2bitMode bool,
	ModulationVals [8][16]uint32,
	ModulationModes [8][16]uint32,
	Mod *uint32,
	DoPT *uint32) {
	RepVals0 := [4]uint32{0, 3, 5, 8}
	RepVals1 := [4]uint32{0, 4, 4, 8}

	var ModVal uint32

	// Map X and Y into the local 2x2 block
	y = (y & 0x3) | ((^y & 0x2) << 1)

	if Do2bitMode {
		x = (x & 0x7) | ((^x & 0x4) << 1)
	} else {
		x = (x & 0x3) | ((^x & 0x2) << 1)
	}

	// assume no PT for now
	*DoPT = 0

	// extract the modulation value. If a simple encoding
	if ModulationModes[y][x] == 0 {
		ModVal = RepVals0[ModulationVals[y][x]]
	} else if Do2bitMode {
		// if this is a stored value
		if ((x ^ y) & 1) == 0 {
			ModVal = RepVals0[ModulationVals[y][x]]
		} else if ModulationModes[y][x] == 1 { // else average from the neighbours if H&V interpolation..
			ModVal = (RepVals0[ModulationVals[y-1][x]] +
				RepVals0[ModulationVals[y+1][x]] +
				RepVals0[ModulationVals[y][x-1]] +
				RepVals0[ModulationVals[y][x+1]] + 2) / 4
		} else if ModulationModes[y][x] == 2 { // else if H-Only
			ModVal = (RepVals0[ModulationVals[y][x-1]] +
				RepVals0[ModulationVals[y][x+1]] + 1) / 2
		} else { // else it's V-Only
			ModVal = (RepVals0[ModulationVals[y-1][x]] +
				RepVals0[ModulationVals[y+1][x]] + 1) / 2
		}
	} else {
		ModVal = RepVals1[ModulationVals[y][x]]

		if ModulationVals[y][x] == PT_INDEX {
			*DoPT = 1
		} else {
			*DoPT = 0
		}
	}

	*Mod = ModVal
}

/*!***********************************************************************
 @Function		TwiddleUV
 @Input			YSize	Y dimension of the texture in pixels
 @Input			XSize	X dimension of the texture in pixels
 @Input			YPos	Pixel Y position
 @Input			XPos	Pixel X position
 @Returns		The twiddled offset of the pixel
 @Description	Given the Block (or pixel) coordinates and the dimension of
				the texture in blocks (or pixels) this returns the twiddled
				offset of the block (or pixel) from the start of the map.

				NOTE the dimensions of the texture must be a power of 2
*************************************************************************/
var DisableTwiddlingRoutine int = 0

func TwiddleUV(YSize uint32, XSize uint32, YPos uint32, XPos uint32) uint32 {
	var Twiddled, MinDimension, MaxValue, SrcBitPos, DstBitPos, ShiftCount uint32

	//assert(YPos < YSize);
	//assert(XPos < XSize);

	//assert(POWER_OF_2(YSize));
	//assert(POWER_OF_2(XSize));

	if YSize < XSize {
		MinDimension = YSize
		MaxValue = XPos
	} else {
		MinDimension = XSize
		MaxValue = YPos
	}

	// Nasty hack to disable twiddling
	if DisableTwiddlingRoutine != 0 {
		return (YPos*XSize + XPos)
	}

	// Step through all the bits in the "minimum" dimension
	SrcBitPos = 1
	DstBitPos = 1
	Twiddled = 0
	ShiftCount = 0

	for SrcBitPos < MinDimension {
		if YPos&SrcBitPos != 0 {
			Twiddled |= DstBitPos
		}

		if XPos&SrcBitPos != 0 {
			Twiddled |= (DstBitPos << 1)
		}

		SrcBitPos <<= 1
		DstBitPos <<= 2
		ShiftCount += 1

	}

	// prepend any unused bits
	MaxValue >>= ShiftCount

	Twiddled |= (MaxValue << (2 * ShiftCount))

	return Twiddled
}

/*!***********************************************************************
 @Function		Decompress
 @Input			pCompressedData The PVRTC texture data to decompress
 @Input			Do2BitMode Signifies whether the data is PVRTC2 or PVRTC4
 @Input			XDim X dimension of the texture
 @Input			YDim Y dimension of the texture
 @Input			AssumeImageTiles Assume the texture data tiles
 @Modified		pResultImage The decompressed texture data
 @Description	Decompresses PVRTC to RGBA 8888
*************************************************************************/
func Decompress(pCompressedData []uint32,
	Do2bitMode bool,
	XDim uint32,
	YDim uint32,
	AssumeImageTiles bool,
	pResultImage []uint8) {
	var x, y, i, j, BlkX, BlkY, BlkXp1, BlkYp1, XBlockSize, BlkXDim, BlkYDim, StartX, StartY, Mod, DoPT uint32

	var ModulationVals, ModulationModes [8][16]uint32

	var uPosition uint32

	// local neighbourhood of blocks
	var pBlocks [2][2]uint32

	var pPrevious [2][2]uint32

	// Low precision colours extracted from the blocks
	type ColorData struct {
		Reps [2][4]uint32
	}
	var Colours5554 [2][2]ColorData

	// Interpolated A and B colours for the pixel
	var ASig, BSig, Result [4]uint32

	if Do2bitMode {
		XBlockSize = BLK_X_2BPP
	} else {
		XBlockSize = BLK_X_4BPP
	}

	// For MBX don't allow the sizes to get too small
	BlkXDim = uint32(max(2, int(XDim/XBlockSize)))
	BlkYDim = uint32(max(2, int(YDim/BLK_Y_SIZE)))

	/*
		Step through the pixels of the image decompressing each one in turn

		Note that this is a hideously inefficient way to do this!
	*/
	for y = 0; y < YDim; y++ {
		for x = 0; x < XDim; x++ {
			// map this pixel to the top left neighbourhood of blocks
			BlkX = (x - XBlockSize/2)
			BlkY = (y - BLK_Y_SIZE/2)

			BlkX = LIMIT_COORD(BlkX, XDim, AssumeImageTiles)
			BlkY = LIMIT_COORD(BlkY, YDim, AssumeImageTiles)

			BlkX /= XBlockSize
			BlkY /= BLK_Y_SIZE

			// compute the positions of the other 3 blocks
			BlkXp1 = LIMIT_COORD(BlkX+1, BlkXDim, AssumeImageTiles)
			BlkYp1 = LIMIT_COORD(BlkY+1, BlkYDim, AssumeImageTiles)

			// Map to block memory locations
			pBlocks[0][0] = pCompressedData[TwiddleUV(uint32(BlkYDim), uint32(BlkXDim), uint32(BlkY), uint32(BlkX))]
			pBlocks[0][1] = pCompressedData[TwiddleUV(uint32(BlkYDim), uint32(BlkXDim), uint32(BlkY), uint32(BlkXp1))]
			pBlocks[1][0] = pCompressedData[TwiddleUV(uint32(BlkYDim), uint32(BlkXDim), uint32(BlkYp1), uint32(BlkX))]
			pBlocks[1][1] = pCompressedData[TwiddleUV(uint32(BlkYDim), uint32(BlkXDim), uint32(BlkYp1), uint32(BlkXp1))]

			/*
				extract the colours and the modulation information IF the previous values
				have changed.
			*/
			if copy(pPrevious[:], pBlocks[:]) != 0 {
				StartY = 0
				for i = 0; i < 2; i++ {
					StartX = 0
					for j = 0; j < 2; j++ {
						Unpack5554Colour(pBlocks[i][j:j+1], &Colours5554[i][j].Reps)

						UnpackModulations(pBlocks[i][j:j+1],
							Do2bitMode,
							&ModulationVals,
							&ModulationModes,
							StartX, StartY)

						StartX += XBlockSize
					}

					StartY += BLK_Y_SIZE
				}

				// make a copy of the new pointers
				copy(pPrevious[:], pBlocks[:])
			}

			// decompress the pixel.  First compute the interpolated A and B signals
			InterpolateColours(Colours5554[0][0].Reps[0],
				Colours5554[0][1].Reps[0],
				Colours5554[1][0].Reps[0],
				Colours5554[1][1].Reps[0],
				Do2bitMode, x, y,
				&ASig)

			InterpolateColours(Colours5554[0][0].Reps[1],
				Colours5554[0][1].Reps[1],
				Colours5554[1][0].Reps[1],
				Colours5554[1][1].Reps[1],
				Do2bitMode, x, y,
				&BSig)

			GetModulationValue(x, y, Do2bitMode, ModulationVals, ModulationModes,
				&Mod, &DoPT)

			// compute the modulated colour
			for i = 0; i < 4; i++ {
				Result[i] = ASig[i]*8 + Mod*(BSig[i]-ASig[i])
				Result[i] >>= 3
			}

			if DoPT != 0 {
				Result[3] = 0
			}

			// Store the result in the output image
			uPosition = (x + y*XDim) << 2
			pResultImage[uPosition+0] = uint8(Result[0])
			pResultImage[uPosition+1] = uint8(Result[1])
			pResultImage[uPosition+2] = uint8(Result[2])
			pResultImage[uPosition+3] = uint8(Result[3])
		}
	}
}
