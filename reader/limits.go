// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"errors"
	"fmt"
	"io"
)

// ErrMemoryLimitExceeded is returned when decompressed data exceeds the configured limit.
var ErrMemoryLimitExceeded = errors.New("reader: decompressed stream exceeds memory limit")

// MemoryLimits configures memory safety bounds for the PDF reader.
// Zero values use sensible defaults. Negative values disable the limit.
type MemoryLimits struct {
	// MaxStreamSize is the maximum decompressed size of a single stream in bytes.
	// Prevents zip-bomb attacks where a small compressed payload expands to gigabytes.
	// Default: 256 MB. Set to -1 to disable.
	MaxStreamSize int64

	// MaxTotalAlloc is the maximum total decompressed bytes across all streams
	// in a single document. Default: 1 GB. Set to -1 to disable.
	MaxTotalAlloc int64

	// MaxXrefSize is the maximum decompressed size of an xref stream.
	// Xref streams are parsed before the resolver is available, so they have
	// a separate (smaller) limit. Default: 32 MB. Set to -1 to disable.
	MaxXrefSize int64

	// MaxObjectCount is the maximum number of objects allowed in the xref table.
	// Prevents excessive memory from a malicious xref claiming millions of objects.
	// Default: 1,000,000. Set to -1 to disable.
	MaxObjectCount int
}

// Default memory limits used when the caller does not override them.
const (
	defaultMaxStreamSize  = 256 << 20 // 256 MB
	defaultMaxTotalAlloc  = 1 << 30   // 1 GB
	defaultMaxXrefSize    = 32 << 20  // 32 MB
	defaultMaxObjectCount = 1_000_000
)

// effectiveMaxStreamSize returns the configured or default max stream size.
func (ml MemoryLimits) effectiveMaxStreamSize() int64 {
	if ml.MaxStreamSize < 0 {
		return -1 // disabled
	}
	if ml.MaxStreamSize == 0 {
		return defaultMaxStreamSize
	}
	return ml.MaxStreamSize
}

// effectiveMaxTotalAlloc returns the configured or default max total allocation.
func (ml MemoryLimits) effectiveMaxTotalAlloc() int64 {
	if ml.MaxTotalAlloc < 0 {
		return -1
	}
	if ml.MaxTotalAlloc == 0 {
		return defaultMaxTotalAlloc
	}
	return ml.MaxTotalAlloc
}

// effectiveMaxXrefSize returns the configured or default max xref stream size.
func (ml MemoryLimits) effectiveMaxXrefSize() int64 {
	if ml.MaxXrefSize < 0 {
		return -1
	}
	if ml.MaxXrefSize == 0 {
		return defaultMaxXrefSize
	}
	return ml.MaxXrefSize
}

// effectiveMaxObjectCount returns the configured or default max object count.
func (ml MemoryLimits) effectiveMaxObjectCount() int {
	if ml.MaxObjectCount < 0 {
		return -1
	}
	if ml.MaxObjectCount == 0 {
		return defaultMaxObjectCount
	}
	return ml.MaxObjectCount
}

// memoryTracker tracks cumulative decompressed bytes for a document.
type memoryTracker struct {
	limits    MemoryLimits
	totalUsed int64
}

// newMemoryTracker creates a memoryTracker with the given limits.
func newMemoryTracker(limits MemoryLimits) *memoryTracker {
	return &memoryTracker{limits: limits}
}

// checkStreamSize validates that a single stream decompression result
// is within bounds and updates the cumulative total.
func (mt *memoryTracker) checkStreamSize(size int64) error {
	maxStream := mt.limits.effectiveMaxStreamSize()
	if maxStream >= 0 && size > maxStream {
		return fmt.Errorf("%w: stream size %d exceeds limit %d", ErrMemoryLimitExceeded, size, maxStream)
	}

	mt.totalUsed += size
	maxTotal := mt.limits.effectiveMaxTotalAlloc()
	if maxTotal >= 0 && mt.totalUsed > maxTotal {
		return fmt.Errorf("%w: total allocation %d exceeds limit %d", ErrMemoryLimitExceeded, mt.totalUsed, maxTotal)
	}
	return nil
}

// limitedReadAll reads from r up to maxBytes. Returns ErrMemoryLimitExceeded
// if the data exceeds the limit. If maxBytes < 0, reads without limit.
func limitedReadAll(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes < 0 {
		return io.ReadAll(r)
	}
	// Read up to maxBytes+1 to detect overflow.
	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: exceeded %d bytes", ErrMemoryLimitExceeded, maxBytes)
	}
	return data, nil
}
