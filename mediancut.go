// Copyright 2013 Andrew Bonventre. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bg

import (
	"container/heap"
	"image"
	"image/color"
	"image/draw"
	"sort"
)

const (
	numDimensions = 3
)

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

type point [numDimensions]int

type block struct {
	minCorner, maxCorner point
	points               []point
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

func newBlock(p []point) *block {
	return &block{
		minCorner: point{0x00, 0x00, 0x00},
		maxCorner: point{0xFF, 0xFF, 0xFF},
		points:    p,
	}
}

func (b *block) longestSideIndex() int {
	m := b.maxCorner[0] - b.minCorner[0]
	maxIndex := 0
	for i := 1; i < numDimensions; i++ {
		diff := b.maxCorner[i] - b.minCorner[i]
		if diff > m {
			m = diff
			maxIndex = i
		}
	}
	return maxIndex
}

func (b *block) longestSideLength() int {
	i := b.longestSideIndex()
	return b.maxCorner[i] - b.minCorner[i]
}

func (b *block) shrink() {
	for j := 0; j < numDimensions; j++ {
		b.minCorner[j] = b.points[0][j]
		b.maxCorner[j] = b.points[0][j]
	}
	for i := 1; i < len(b.points); i++ {
		for j := 0; j < numDimensions; j++ {
			b.minCorner[j] = min(b.minCorner[j], b.points[i][j])
			b.maxCorner[j] = max(b.maxCorner[j], b.points[i][j])
		}
	}
}

func (b *block) len() int {
	return len(b.points)
}

func (b *block) medianIndex() int {
	// This median cut implementation doesn't rely on blocks being cut exactly in half, in practice
	// we can improve the result by adjusting the split point to not fall within one color group so
	// that ultimately we do not split a color across multiple blocks.
	// This assumes the block is already sorted by longestSideIndex, although nothing bad will 
	// happen if it isn't.
	length := len(b.points)
	mid := length / 2
	li := b.longestSideIndex()
	midVal := b.points[mid][li]
	left := mid;
	right := mid + 1;

	for ; left > 0 && midVal == b.points[left-1][li]; left-- {}
	for ; right < length && midVal == b.points[right][li]; right++ {}

	if left > 0 && mid - left < right - mid {
		return left
	} else if right < length {
		return right
	} else {
		return mid
	}
}

type pointSorter struct {
	points []point
	by     func(p1, p2 *point) bool
}

func (p *pointSorter) Len() int {
	return len(p.points)
}

func (p *pointSorter) Swap(i, j int) {
	p.points[i], p.points[j] = p.points[j], p.points[i]
}

func (p *pointSorter) Less(i, j int) bool {
	return p.by(&p.points[i], &p.points[j])
}

// A priorityQueue implements heap.Interface and holds blocks.
type priorityQueue []*block

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	li := pq[i].longestSideLength()
	lj := pq[j].longestSideLength()
	// choosing most diverse block, using number of pixels as a tiebreaker
	return li > lj || (li == lj && pq[i].len() > pq[j].len())
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*block)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[:n-1]
	return item
}

// clip clips r against each image's bounds (after translating into
// the destination image's co-ordinate space) and shifts the point
// sp by the same amount as the change in r.Min.
func clip(dst draw.Image, r *image.Rectangle, src image.Image, sp *image.Point) {
	orig := r.Min
	*r = r.Intersect(dst.Bounds())
	*r = r.Intersect(src.Bounds().Add(orig.Sub(*sp)))
	dx := r.Min.X - orig.X
	dy := r.Min.Y - orig.Y
	if dx == 0 && dy == 0 {
		return
	}
	(*sp).X += dx
	(*sp).Y += dy
}

// MedianCutQuantizer constructs a palette with a maximum of
// NumColor colors by iteratively splitting clusters of color
// points mapped on a three-dimensional (RGB) Euclidian space.
// Once the number of clusters is within the specified bounds,
// the resulting color is computed by averaging those within
// each grouping.
type MedianCutQuantizer struct {
	NumColor int
	Points   []point
}

func (q *MedianCutQuantizer) AddPoint(r, g, b int) {
	q.Points = append(q.Points, point{r, g, b})
}
func (q *MedianCutQuantizer) MedianCut() color.Palette {
	return q.medianCut(q.Points)

}
func (q *MedianCutQuantizer) medianCut(points []point) color.Palette {
	if q.NumColor == 0 {
		return color.Palette{}
	}

	initialBlock := newBlock(points)
	initialBlock.shrink()
	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, initialBlock)

	for pq.Len() < q.NumColor {
		longestBlock := heap.Pop(pq).(*block)
		points := longestBlock.points
		// Can't assume anything about order of blocks in heap, have to Pop before we can 
		// determine if there is work left
		if len(points) < 2 || longestBlock.longestSideLength() == 0 {
			heap.Push(pq, longestBlock)
			break
		}

		li := longestBlock.longestSideIndex()
		// TODO: Instead of sorting the entire slice, finding the median using an
		// algorithm like introselect would give much better performance.
		sort.Sort(&pointSorter{
			points: points,
			by:     func(p1, p2 *point) bool { return p1[li] < p2[li] },
		})
		median := longestBlock.medianIndex()
		block1 := newBlock(points[:median])
		block2 := newBlock(points[median:])
		block1.shrink()
		block2.shrink()
		heap.Push(pq, block1)
		heap.Push(pq, block2)
	}

	palette := make(color.Palette, q.NumColor)
	var n int
	for n = 0; pq.Len() > 0; n++ {
		block := heap.Pop(pq).(*block)
		var sum [numDimensions]int
		for i := 0; i < len(block.points); i++ {
			for j := 0; j < numDimensions; j++ {
				sum[j] += block.points[i][j]
			}
		}
		palette[n] = color.RGBA64{
			R: uint16(sum[0] / len(block.points)),
			G: uint16(sum[1] / len(block.points)),
			B: uint16(sum[2] / len(block.points)),
			A: 0xFFFF,
		}
	}
	// Trim to only the colors present in the image, which
	// could be less than NumColor.
	return palette[:n]
}

func (q *MedianCutQuantizer) Quantize(dst *image.Paletted, r image.Rectangle, src image.Image, sp image.Point) {
	clip(dst, &r, src, &sp)
	if r.Empty() {
		return
	}

	points := make([]point, r.Dx()*r.Dy())
	colorSet := make(map[uint32]color.Color, q.NumColor)
	i := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := src.At(x, y)
			r, g, b, a := c.RGBA()
			// Exclude transparent (implicit or explicit) pixels from calculation, it is up to the
			// caller to handle them.
			if !(r == 0 && g == 0xffff && b == 0) && a != 0 {
				colorSet[(r>>8)<<16|(g>>8)<<8|b>>8] = c
				points[i][0] = int(r)
				points[i][1] = int(g)
				points[i][2] = int(b)
				i++
			}
		}
	}
	points = points[0:i] // shrink points slice as extra space was likely allocated for discarded transparent pixels

	if len(colorSet) <= q.NumColor {
		// No need to quantize since the total number of colors
		// fits within the palette.
		dst.Palette = make(color.Palette, len(colorSet))
		i := 0
		for _, c := range colorSet {
			dst.Palette[i] = c
			i++
		}
	} else {
		dst.Palette = q.medianCut(points)
	}

	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			// TODO: this should be done more efficiently.
			dst.Set(sp.X+x, sp.Y+y, src.At(r.Min.X+x, r.Min.Y+y))
		}
	}
}
