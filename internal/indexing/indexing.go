package indexing

import (
	"fmt"
)

// ChunkProjection describes how a chunk maps onto a selection window.
type ChunkProjection struct {
	ChunkCoords []int64
	ChunkOffset []int
	OutOffset   []int
	Shape       []int
}

// ComputeChunkCoords returns the chunk coordinates overlapping the selection.
func ComputeChunkCoords(arrayShape []int64, chunkShape []int, selOffset, selShape []int64) ([][]int64, error) {
	ndim := len(arrayShape)
	if selOffset == nil {
		selOffset = make([]int64, ndim)
	}
	if selShape == nil {
		selShape = append([]int64(nil), arrayShape...)
	}
	start := make([]int64, ndim)
	end := make([]int64, ndim)
	numChunks := int64(1)
	for i := 0; i < ndim; i++ {
		staIdx := selOffset[i] / int64(chunkShape[i])
		endIdx := (selOffset[i] + selShape[i] - 1) / int64(chunkShape[i])
		numChunks *= (endIdx - staIdx + 1)
		start[i] = staIdx
		end[i] = endIdx
	}
	if numChunks > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("indexing: number of chunks exceeds int max")
	}
	out := make([][]int64, numChunks)
	current := append([]int64(nil), start...)
	for i := range out {
		out[i] = append([]int64(nil), current...)
		dimIdx := ndim - 1
		for dimIdx >= 0 {
			if current[dimIdx] >= end[dimIdx] {
				current[dimIdx] = start[dimIdx]
				dimIdx--
			} else {
				current[dimIdx]++
				dimIdx = -1
			}
		}
	}
	return out, nil
}

// ComputeProjection maps a chunk onto a selection.
func ComputeProjection(chunkCoords []int64, arrayShape []int64, chunkShape []int, selOffset, selShape []int64) (ChunkProjection, error) {
	ndim := len(chunkCoords)
	if selOffset == nil {
		selOffset = make([]int64, ndim)
	}
	if selShape == nil {
		selShape = append([]int64(nil), arrayShape...)
	}
	chunkOffset := make([]int, ndim)
	outOffset := make([]int, ndim)
	shape := make([]int, ndim)
	for dimIdx := 0; dimIdx < ndim; dimIdx++ {
		dimOffset := int64(chunkShape[dimIdx]) * chunkCoords[dimIdx]
		dimLimit := arrayShape[dimIdx]
		if next := (chunkCoords[dimIdx] + 1) * int64(chunkShape[dimIdx]); next < dimLimit {
			dimLimit = next
		}
		if selOffset[dimIdx] < dimOffset {
			chunkOffset[dimIdx] = 0
			outOffset[dimIdx] = int(dimOffset - selOffset[dimIdx])
		} else {
			chunkOffset[dimIdx] = int(selOffset[dimIdx] - dimOffset)
			outOffset[dimIdx] = 0
		}
		if selOffset[dimIdx]+selShape[dimIdx] > dimLimit {
			shape[dimIdx] = chunkShape[dimIdx] - chunkOffset[dimIdx]
		} else {
			shape[dimIdx] = int(selOffset[dimIdx] + selShape[dimIdx] - dimOffset - int64(chunkOffset[dimIdx]))
		}
	}
	return ChunkProjection{
		ChunkCoords: append([]int64(nil), chunkCoords...),
		ChunkOffset: chunkOffset,
		OutOffset:   outOffset,
		Shape:       shape,
	}, nil
}

// IsFullChunk reports whether the selection covers an entire chunk starting at 0.
func IsFullChunk(selOffset, selShape, chunkShape []int) bool {
	for i := range selOffset {
		if selOffset[i] != 0 || selShape[i] != chunkShape[i] {
			return false
		}
	}
	return true
}

// IsSingleFullChunk reports whether the selection is exactly one aligned full chunk.
func IsSingleFullChunk(selOffset, selShape []int64, chunkShape []int) bool {
	for i := range selOffset {
		if selOffset[i]%int64(chunkShape[i]) != 0 || selShape[i] != int64(chunkShape[i]) {
			return false
		}
	}
	return true
}

// ComputeSingleChunkCoords returns chunk coordinates for an aligned selection origin.
func ComputeSingleChunkCoords(selOffset []int64, chunkShape []int) []int64 {
	out := make([]int64, len(selOffset))
	for i := range selOffset {
		out[i] = selOffset[i] / int64(chunkShape[i])
	}
	return out
}

// IntsToInt64 converts []int to []int64.
func IntsToInt64(in []int) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}

// Int64ToInt converts []int64 to []int.
func Int64ToInt(in []int64) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

// Product returns the product of the values.
func Product(in []int) int {
	n := 1
	for _, v := range in {
		n *= v
	}
	return n
}
