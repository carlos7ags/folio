// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestByteWidth(t *testing.T) {
	cases := []struct {
		v    int
		want int
	}{
		{-1, 1},
		{0, 1},
		{1, 1},
		{255, 1},
		{256, 2},
		{65535, 2},
		{65536, 3},
		{1<<24 - 1, 3},
		{1 << 24, 4},
		{1<<32 - 1, 4},
		{1 << 32, 5},
	}
	for _, c := range cases {
		if got := byteWidth(c.v); got != c.want {
			t.Errorf("byteWidth(%d) = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestXRefStreamWidths(t *testing.T) {
	cases := []struct {
		name                                    string
		maxOffset, maxGen, maxObjStmNum, maxIdx int
		want                                    [3]int
	}{
		{
			name: "empty document",
			want: [3]int{1, 1, 1},
		},
		{
			name:      "small file no objstm",
			maxOffset: 4096,
			want:      [3]int{1, 2, 1},
		},
		{
			name:      "65 KiB file",
			maxOffset: 65535,
			want:      [3]int{1, 2, 1},
		},
		{
			name:      "65 KiB + 1 file forces 3-byte field 2",
			maxOffset: 65536,
			want:      [3]int{1, 3, 1},
		},
		{
			name:      "16 MiB - 1",
			maxOffset: 1<<24 - 1,
			want:      [3]int{1, 3, 1},
		},
		{
			name:      "16 MiB",
			maxOffset: 1 << 24,
			want:      [3]int{1, 4, 1},
		},
		{
			name:         "objstm number larger than offset bumps field 2",
			maxOffset:    100,
			maxObjStmNum: 70000,
			want:         [3]int{1, 3, 1},
		},
		{
			name:      "index inside objstm bumps field 3",
			maxOffset: 100,
			maxIdx:    300,
			want:      [3]int{1, 1, 2},
		},
		{
			name:      "generation bumps field 3 when no index",
			maxOffset: 100,
			maxGen:    65535,
			want:      [3]int{1, 1, 2},
		},
		{
			name:      "index dominates generation when both present",
			maxOffset: 100,
			maxGen:    7,
			maxIdx:    1000,
			want:      [3]int{1, 1, 2},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := XRefStreamWidths(c.maxOffset, c.maxGen, c.maxObjStmNum, c.maxIdx)
			if got != c.want {
				t.Errorf("XRefStreamWidths(%d,%d,%d,%d) = %v, want %v",
					c.maxOffset, c.maxGen, c.maxObjStmNum, c.maxIdx, got, c.want)
			}
		})
	}
}
