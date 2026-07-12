// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// This file pins the exact chunking behavior of breakLongWords and
// hyphenateWord as it exists today, before their candidate-scan loops
// are rewritten to use cumulative prefix widths instead of re-measuring
// each candidate from scratch. The expectations below were captured by
// running the corpus against the pre-optimization code; they must not
// be edited by the optimization that follows — any divergence means the
// break/hyphenation choice changed.

type chunkExpectation struct {
	text  string
	width uint64 // math.Float64bits(Word.Width)
}

func TestBreakLongWordsCharacterization(t *testing.T) {
	cjkFace, err := font.LoadFont("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("LoadFont: %v", err)
	}
	cjkFont := font.NewEmbeddedFont(cjkFace)

	tests := []struct {
		name     string
		text     string
		fontSize float64
		maxWidth float64
		std      *font.Standard
		embedded *font.EmbeddedFont
		want     []chunkExpectation
	}{
		{
			name:     "kern-bearing ascii",
			text:     strings.Repeat("abcdefghij", 40),
			fontSize: 12,
			maxWidth: 100,
			std:      font.Helvetica,
			want: []chunkExpectation{
				{text: "abcdefghijabcdefg", width: 0x40585916872b020c},
				{text: "hijabcdefghijabcde", width: 0x4058d89374bc6a7f},
				{text: "fghijabcdefghijabc", width: 0x405803126e978d50},
				{text: "defghijabcdefghija", width: 0x40582e147ae147ae},
				{text: "bcdefghijabcdefgh", width: 0x40585916872b020c},
				{text: "ijabcdefghijabcdef", width: 0x405803126e978d50},
				{text: "ghijabcdefghijabcd", width: 0x4058d89374bc6a7f},
				{text: "efghijabcdefghijab", width: 0x40582e147ae147ae},
				{text: "cdefghijabcdefghij", width: 0x405803126e978d50},
				{text: "abcdefghijabcdefg", width: 0x40585916872b020c},
				{text: "hijabcdefghijabcde", width: 0x4058d89374bc6a7f},
				{text: "fghijabcdefghijabc", width: 0x405803126e978d50},
				{text: "defghijabcdefghija", width: 0x40582e147ae147ae},
				{text: "bcdefghijabcdefgh", width: 0x40585916872b020c},
				{text: "ijabcdefghijabcdef", width: 0x405803126e978d50},
				{text: "ghijabcdefghijabcd", width: 0x4058d89374bc6a7f},
				{text: "efghijabcdefghijab", width: 0x40582e147ae147ae},
				{text: "cdefghijabcdefghij", width: 0x405803126e978d50},
				{text: "abcdefghijabcdefg", width: 0x40585916872b020c},
				{text: "hijabcdefghijabcde", width: 0x4058d89374bc6a7f},
				{text: "fghijabcdefghijabc", width: 0x405803126e978d50},
				{text: "defghijabcdefghija", width: 0x40582e147ae147ae},
				{text: "bcdefghij", width: 0x404803126e978d50},
			},
		},
		{
			name:     "non-monotonic kern pairs",
			text:     strings.Repeat("AVAWAToTo", 50),
			fontSize: 12,
			maxWidth: 80,
			std:      font.Helvetica,
			want: []chunkExpectation{
				{text: "AVAWAToToAV", width: 0x4053cccccccccccc},
				{text: "AWAToToAVA", width: 0x4052024dd2f1a9fc},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToToAVA", width: 0x4053c51eb851eb85},
				{text: "WAToToAVAW", width: 0x4052d70a3d70a3d7},
				{text: "AToToAVAWAT", width: 0x40537b645a1cac08},
				{text: "oToAVAWAToT", width: 0x4053824dd2f1a9fc},
				{text: "oAVAWAToToA", width: 0x4053ad4fdf3b645a},
				{text: "VAWAToTo", width: 0x404c6f1a9fbe76c8},
			},
		},
		{
			name:     "combining marks",
			text:     strings.Repeat("éx̂", 100),
			fontSize: 12,
			maxWidth: 60,
			std:      font.Helvetica,
			want: []chunkExpectation{
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂éx̂éx̂éx̂é", width: 0x404b3d70a3d70a3e},
				{text: "x̂éx̂éx̂éx̂éx̂", width: 0x404ae76c8b439581},
				{text: "éx̂", width: 0x40289fbe76c8b43a},
			},
		},
		{
			name:     "embedded kern-bearing ascii",
			text:     strings.Repeat("abcdefghij", 40),
			fontSize: 12,
			maxWidth: 100,
			embedded: cjkFont,
			want: []chunkExpectation{
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
				{text: "abcdefgh", width: 0x4058000000000000},
				{text: "ijabcdef", width: 0x4058000000000000},
				{text: "ghijabcd", width: 0x4058000000000000},
				{text: "efghijab", width: 0x4058000000000000},
				{text: "cdefghij", width: 0x4058000000000000},
			},
		},
		{
			name:     "embedded combining marks",
			text:     strings.Repeat("éx̂", 100),
			fontSize: 12,
			maxWidth: 60,
			embedded: cjkFont,
			want: []chunkExpectation{
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
				{text: "éx̂éx̂é", width: 0x404e000000000000},
				{text: "x̂éx̂éx̂", width: 0x404e000000000000},
			},
		},
		{
			name:     "embedded CJK token",
			text:     strings.Repeat("中华人民共和国是一个历史悠久的文明古国", 20),
			fontSize: 12,
			maxWidth: 80,
			embedded: cjkFont,
			want: []chunkExpectation{
				{text: "中华人民共和", width: 0x4052000000000000},
				{text: "国是一个历史", width: 0x4052000000000000},
				{text: "悠久的文明古", width: 0x4052000000000000},
				{text: "国中华人民共", width: 0x4052000000000000},
				{text: "和国是一个历", width: 0x4052000000000000},
				{text: "史悠久的文明", width: 0x4052000000000000},
				{text: "古国中华人民", width: 0x4052000000000000},
				{text: "共和国是一个", width: 0x4052000000000000},
				{text: "历史悠久的文", width: 0x4052000000000000},
				{text: "明古国中华人", width: 0x4052000000000000},
				{text: "民共和国是一", width: 0x4052000000000000},
				{text: "个历史悠久的", width: 0x4052000000000000},
				{text: "文明古国中华", width: 0x4052000000000000},
				{text: "人民共和国是", width: 0x4052000000000000},
				{text: "一个历史悠久", width: 0x4052000000000000},
				{text: "的文明古国中", width: 0x4052000000000000},
				{text: "华人民共和国", width: 0x4052000000000000},
				{text: "是一个历史悠", width: 0x4052000000000000},
				{text: "久的文明古国", width: 0x4052000000000000},
				{text: "中华人民共和", width: 0x4052000000000000},
				{text: "国是一个历史", width: 0x4052000000000000},
				{text: "悠久的文明古", width: 0x4052000000000000},
				{text: "国中华人民共", width: 0x4052000000000000},
				{text: "和国是一个历", width: 0x4052000000000000},
				{text: "史悠久的文明", width: 0x4052000000000000},
				{text: "古国中华人民", width: 0x4052000000000000},
				{text: "共和国是一个", width: 0x4052000000000000},
				{text: "历史悠久的文", width: 0x4052000000000000},
				{text: "明古国中华人", width: 0x4052000000000000},
				{text: "民共和国是一", width: 0x4052000000000000},
				{text: "个历史悠久的", width: 0x4052000000000000},
				{text: "文明古国中华", width: 0x4052000000000000},
				{text: "人民共和国是", width: 0x4052000000000000},
				{text: "一个历史悠久", width: 0x4052000000000000},
				{text: "的文明古国中", width: 0x4052000000000000},
				{text: "华人民共和国", width: 0x4052000000000000},
				{text: "是一个历史悠", width: 0x4052000000000000},
				{text: "久的文明古国", width: 0x4052000000000000},
				{text: "中华人民共和", width: 0x4052000000000000},
				{text: "国是一个历史", width: 0x4052000000000000},
				{text: "悠久的文明古", width: 0x4052000000000000},
				{text: "国中华人民共", width: 0x4052000000000000},
				{text: "和国是一个历", width: 0x4052000000000000},
				{text: "史悠久的文明", width: 0x4052000000000000},
				{text: "古国中华人民", width: 0x4052000000000000},
				{text: "共和国是一个", width: 0x4052000000000000},
				{text: "历史悠久的文", width: 0x4052000000000000},
				{text: "明古国中华人", width: 0x4052000000000000},
				{text: "民共和国是一", width: 0x4052000000000000},
				{text: "个历史悠久的", width: 0x4052000000000000},
				{text: "文明古国中华", width: 0x4052000000000000},
				{text: "人民共和国是", width: 0x4052000000000000},
				{text: "一个历史悠久", width: 0x4052000000000000},
				{text: "的文明古国中", width: 0x4052000000000000},
				{text: "华人民共和国", width: 0x4052000000000000},
				{text: "是一个历史悠", width: 0x4052000000000000},
				{text: "久的文明古国", width: 0x4052000000000000},
				{text: "中华人民共和", width: 0x4052000000000000},
				{text: "国是一个历史", width: 0x4052000000000000},
				{text: "悠久的文明古", width: 0x4052000000000000},
				{text: "国中华人民共", width: 0x4052000000000000},
				{text: "和国是一个历", width: 0x4052000000000000},
				{text: "史悠久的文明", width: 0x4052000000000000},
				{text: "古国", width: 0x4038000000000000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var measurer font.TextMeasurer
			w := Word{
				Text:     tt.text,
				FontSize: tt.fontSize,
				Font:     tt.std,
				Embedded: tt.embedded,
			}
			if tt.embedded != nil {
				measurer = tt.embedded
			} else {
				measurer = tt.std
			}
			w.Width = measurer.MeasureString(tt.text, tt.fontSize)

			chunks := breakLongWords([]Word{w}, tt.maxWidth)

			if tt.want == nil {
				for _, c := range chunks {
					t.Logf("{text: %q, width: 0x%x /* %v */},", c.Text, math.Float64bits(c.Width), c.Width)
				}
				t.Fatal("no expectations set — paste the logged literals above")
			}

			if len(chunks) != len(tt.want) {
				t.Fatalf("got %d chunks, want %d", len(chunks), len(tt.want))
			}
			for i, c := range chunks {
				want := tt.want[i]
				if c.Text != want.text {
					t.Errorf("chunk %d: Text = %q, want %q", i, c.Text, want.text)
				}
				if got := math.Float64bits(c.Width); got != want.width {
					t.Errorf("chunk %d: Width bits = 0x%x (%v), want 0x%x (%v)",
						i, got, c.Width, want.width, math.Float64frombits(want.width))
				}
			}
		})
	}
}

type hyphenExpectation struct {
	partText  string
	partWidth uint64
	restText  string
	restWidth uint64
	ok        bool
}

func TestHyphenateWordCharacterization(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		fontSize  float64
		available float64
		want      hyphenExpectation
	}{
		{
			name:      "pattern-based split",
			text:      "hyphenation",
			fontSize:  12,
			available: 200,
			want: hyphenExpectation{
				partText:  "hyphen-",
				partWidth: 0x40457f7ced916873,
				restText:  "ation",
				restWidth: 0x403a04189374bc6b,
				ok:        true,
			},
		},
		{
			name:      "fallback char-boundary split",
			text:      "abc123def456ghi789jkl012mno345",
			fontSize:  12,
			available: 80,
			want: hyphenExpectation{
				partText:  "abc123def45-",
				partWidth: 0x40525851eb851eb8,
				restText:  "6ghi789jkl012mno345",
				restWidth: 0x405d595810624dd2,
				ok:        true,
			},
		},
		{
			name:      "no split fits",
			text:      "hyphenation",
			fontSize:  12,
			available: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Word{
				Text:     tt.text,
				FontSize: tt.fontSize,
				Font:     font.Helvetica,
			}
			part, rest, ok := hyphenateWord(w, tt.available)

			if !tt.want.ok && ok {
				t.Logf("part: {text: %q, width: 0x%x /* %v */}", part.Text, math.Float64bits(part.Width), part.Width)
				t.Logf("rest: {text: %q, width: 0x%x /* %v */}", rest.Text, math.Float64bits(rest.Width), rest.Width)
				t.Logf("ok: %v", ok)
				t.Fatal("no expectations set — paste the logged literals above")
			}

			if ok != tt.want.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.want.ok)
			}
			if !ok {
				return
			}
			if part.Text != tt.want.partText {
				t.Errorf("part.Text = %q, want %q", part.Text, tt.want.partText)
			}
			if got := math.Float64bits(part.Width); got != tt.want.partWidth {
				t.Errorf("part.Width bits = 0x%x (%v), want 0x%x (%v)",
					got, part.Width, tt.want.partWidth, math.Float64frombits(tt.want.partWidth))
			}
			if rest.Text != tt.want.restText {
				t.Errorf("rest.Text = %q, want %q", rest.Text, tt.want.restText)
			}
			if got := math.Float64bits(rest.Width); got != tt.want.restWidth {
				t.Errorf("rest.Width bits = 0x%x (%v), want 0x%x (%v)",
					got, rest.Width, tt.want.restWidth, math.Float64frombits(tt.want.restWidth))
			}
		})
	}
}
