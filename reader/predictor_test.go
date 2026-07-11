// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// encodePNGRows applies PNG row filtering forward so tests can assert
// decodePNGPredictor recovers the original bytes. rows holds the raw
// (unfiltered) sample bytes, one row per entry, each rowBytes long.
// filterTypes gives the PNG filter (0-4) to apply to each row.
func encodePNGRows(rows [][]byte, filterTypes []int, bpp int) []byte {
	var out []byte
	prevRow := make([]byte, len(rows[0]))
	for i, row := range rows {
		ft := filterTypes[i]
		rowBytes := len(row)
		filtered := make([]byte, rowBytes)
		for j := 0; j < rowBytes; j++ {
			left := byte(0)
			if j >= bpp {
				left = row[j-bpp]
			}
			up := prevRow[j]
			upLeft := byte(0)
			if j >= bpp {
				upLeft = prevRow[j-bpp]
			}
			switch ft {
			case 0:
				filtered[j] = row[j]
			case 1:
				filtered[j] = row[j] - left
			case 2:
				filtered[j] = row[j] - up
			case 3:
				filtered[j] = row[j] - byte((int(left)+int(up))/2)
			case 4:
				filtered[j] = row[j] - paethPredictor(left, up, upLeft)
			}
		}
		out = append(out, byte(ft))
		out = append(out, filtered...)
		prevRow = row
	}
	return out
}

func TestDecodePNGPredictorRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		rows     [][]byte
		filters  []int
		bpp      int
		rowBytes int
	}{
		{
			name:     "colors1 bpc8 filter0",
			rows:     [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}, {12, 22, 32, 42}},
			filters:  []int{0, 0, 0},
			bpp:      1,
			rowBytes: 4,
		},
		{
			name:     "colors1 bpc8 filter1 sub",
			rows:     [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}, {12, 22, 32, 42}},
			filters:  []int{1, 1, 1},
			bpp:      1,
			rowBytes: 4,
		},
		{
			name:     "colors1 bpc8 filter2 up",
			rows:     [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}, {12, 22, 32, 42}},
			filters:  []int{2, 2, 2},
			bpp:      1,
			rowBytes: 4,
		},
		{
			name:     "colors1 bpc8 filter3 average",
			rows:     [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}, {12, 22, 32, 42}},
			filters:  []int{3, 3, 3},
			bpp:      1,
			rowBytes: 4,
		},
		{
			name:     "colors1 bpc8 filter4 paeth",
			rows:     [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}, {12, 22, 32, 42}},
			filters:  []int{4, 4, 4},
			bpp:      1,
			rowBytes: 4,
		},
		{
			// RGB, bpp=3: 4 pixels/row where components differ within a
			// pixel, so a bpp=1 decoder would produce different bytes.
			name: "colors3 bpc8 filter1 sub rgb",
			rows: [][]byte{
				{10, 100, 200, 20, 110, 210, 30, 120, 220, 40, 130, 230},
				{15, 90, 190, 25, 95, 195, 35, 105, 205, 45, 115, 215},
				{5, 250, 60, 15, 240, 70, 25, 230, 80, 35, 220, 90},
			},
			filters:  []int{1, 1, 1},
			bpp:      3,
			rowBytes: 12,
		},
		{
			name: "colors3 bpc8 filter2 up rgb",
			rows: [][]byte{
				{10, 100, 200, 20, 110, 210, 30, 120, 220, 40, 130, 230},
				{15, 90, 190, 25, 95, 195, 35, 105, 205, 45, 115, 215},
				{5, 250, 60, 15, 240, 70, 25, 230, 80, 35, 220, 90},
			},
			filters:  []int{2, 2, 2},
			bpp:      3,
			rowBytes: 12,
		},
		{
			name: "colors3 bpc8 filter3 average rgb",
			rows: [][]byte{
				{10, 100, 200, 20, 110, 210, 30, 120, 220, 40, 130, 230},
				{15, 90, 190, 25, 95, 195, 35, 105, 205, 45, 115, 215},
				{5, 250, 60, 15, 240, 70, 25, 230, 80, 35, 220, 90},
			},
			filters:  []int{3, 3, 3},
			bpp:      3,
			rowBytes: 12,
		},
		{
			name: "colors3 bpc8 filter4 paeth rgb",
			rows: [][]byte{
				{10, 100, 200, 20, 110, 210, 30, 120, 220, 40, 130, 230},
				{15, 90, 190, 25, 95, 195, 35, 105, 205, 45, 115, 215},
				{5, 250, 60, 15, 240, 70, 25, 230, 80, 35, 220, 90},
			},
			filters:  []int{4, 4, 4},
			bpp:      3,
			rowBytes: 12,
		},
		{
			// colors1, bpc16: bpp=2.
			name:     "colors1 bpc16 filter1 sub",
			rows:     [][]byte{{1, 2, 3, 4}, {5, 6, 7, 8}},
			filters:  []int{1, 1},
			bpp:      2,
			rowBytes: 4,
		},
		{
			name:     "colors1 bpc16 filter4 paeth",
			rows:     [][]byte{{1, 2, 3, 4}, {5, 6, 7, 8}},
			filters:  []int{4, 4},
			bpp:      2,
			rowBytes: 4,
		},
		{
			// colors4, bpc8: bpp=4 (RGBA-shaped).
			name:     "colors4 bpc8 filter2 up",
			rows:     [][]byte{{1, 2, 3, 4, 5, 6, 7, 8}, {9, 10, 11, 12, 13, 14, 15, 16}},
			filters:  []int{2, 2},
			bpp:      4,
			rowBytes: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want []byte
			for _, row := range tt.rows {
				want = append(want, row...)
			}

			encoded := encodePNGRows(tt.rows, tt.filters, tt.bpp)

			got, err := decodePNGPredictor(encoded, tt.rowBytes, tt.bpp)
			if err != nil {
				t.Fatalf("decodePNGPredictor: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("decodePNGPredictor() = %v, want %v", got, want)
			}
		})
	}
}

func TestApplyPredictorColorsBpc(t *testing.T) {
	rows := [][]byte{
		{10, 100, 200, 20, 110, 210, 30, 120, 220, 40, 130, 230},
		{15, 90, 190, 25, 95, 195, 35, 105, 205, 45, 115, 215},
	}
	filters := []int{1, 1}
	encoded := encodePNGRows(rows, filters, 3)

	dict := core.NewPdfDictionary()
	dict.Set("Predictor", core.NewPdfInteger(12))
	dict.Set("Columns", core.NewPdfInteger(4))
	dict.Set("Colors", core.NewPdfInteger(3))
	dict.Set("BitsPerComponent", core.NewPdfInteger(8))

	got, err := applyPredictor(encoded, dict)
	if err != nil {
		t.Fatalf("applyPredictor: %v", err)
	}

	want, err := decodePNGPredictor(encoded, 12, 3)
	if err != nil {
		t.Fatalf("decodePNGPredictor: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("applyPredictor() = %v, want %v", got, want)
	}
}

func TestApplyPredictorDefaults(t *testing.T) {
	// No Colors/BitsPerComponent — must behave as bpp=1, rowBytes=columns
	// (the xref-stream shape).
	rows := [][]byte{{10, 20, 30, 40}, {11, 21, 31, 41}}
	filters := []int{1, 1}
	encoded := encodePNGRows(rows, filters, 1)

	dict := core.NewPdfDictionary()
	dict.Set("Predictor", core.NewPdfInteger(12))
	dict.Set("Columns", core.NewPdfInteger(4))

	got, err := applyPredictor(encoded, dict)
	if err != nil {
		t.Fatalf("applyPredictor: %v", err)
	}

	want, err := decodePNGPredictor(encoded, 4, 1)
	if err != nil {
		t.Fatalf("decodePNGPredictor: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("applyPredictor() = %v, want %v", got, want)
	}
}

func TestApplyPredictorHostileValues(t *testing.T) {
	tests := []struct {
		name    string
		colors  int
		bpc     int
		columns int
	}{
		{"colors zero", 0, 8, 4},
		{"colors negative", -3, 8, 4},
		{"bpc invalid small", 1, 7, 4},
		{"bpc invalid huge", 1, 1000000, 4},
		{"columns huge", 1, 8, 1 << 30},
	}

	data := []byte{1, 10, 20, 30, 40, 1, 11, 21, 31, 41}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := core.NewPdfDictionary()
			dict.Set("Predictor", core.NewPdfInteger(12))
			dict.Set("Columns", core.NewPdfInteger(tt.columns))
			dict.Set("Colors", core.NewPdfInteger(tt.colors))
			dict.Set("BitsPerComponent", core.NewPdfInteger(tt.bpc))

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("applyPredictor panicked: %v", r)
				}
			}()
			if _, err := applyPredictor(data, dict); err != nil {
				t.Fatalf("applyPredictor: %v", err)
			}
		})
	}
}

func TestDecodePNGPredictorTruncatedFinalRow(t *testing.T) {
	// One full row (filter byte + 4 data bytes) plus a partial trailing row.
	data := []byte{1, 10, 20, 30, 40, 0, 1, 2}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("decodePNGPredictor panicked: %v", r)
		}
	}()
	if _, err := decodePNGPredictor(data, 4, 1); err != nil {
		t.Fatalf("decodePNGPredictor: %v", err)
	}
}
