// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// countOps counts occurrences of a PDF operator in a content stream.
// An operator is a standalone token at the end of an operand sequence.
func countOps(stream []byte, op string) int {
	s := string(stream)
	count := 0
	// Simple: count lines ending with the operator or containing " op\n"
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " "+op) || line == op {
			count++
		}
	}
	return count
}

func containsOp(stream []byte, op string) bool {
	return countOps(stream, op) > 0
}

// --- Stage 1: Draw function rendering tests ---

func TestDrawWavyLine(t *testing.T) {
	s := content.NewStream()
	drawWavyLine(s, 10, 50, 100, 1.5)
	b := s.Bytes()

	if len(b) == 0 {
		t.Fatal("wavy line produced empty stream")
	}

	// Wavy line uses zigzag LineTo segments alternating up/down.
	// For a 100pt line with amplitude 1.5 (step=6), expect ~16 segments.
	lines := countOps(b, "l")
	if lines < 10 {
		t.Errorf("expected ≥10 line segments for 100pt wavy line, got %d", lines)
	}

	// Must start with moveto (m) and end with stroke (S).
	if !containsOp(b, "m") {
		t.Error("wavy line missing moveto operator")
	}
	if !containsOp(b, "S") {
		t.Error("wavy line missing stroke operator")
	}

	// Wider line should have more segments.
	s2 := content.NewStream()
	drawWavyLine(s2, 0, 0, 200, 1.5)
	lines2 := countOps(s2.Bytes(), "l")
	if lines2 <= lines {
		t.Errorf("200pt wavy line should have more segments than 100pt: %d vs %d", lines2, lines)
	}
}

func TestDrawBoxShadow(t *testing.T) {
	// Render a Div with box-shadow through the full pipeline.
	d := NewDiv().
		SetBackground(RGB(1, 1, 1)).
		SetPadding(10).
		SetBorder(SolidBorder(1, ColorBlack))
	d.boxShadows = []BoxShadow{{
		OffsetX: 4, OffsetY: 4, Blur: 8, Spread: 0,
		Color: RGB(0, 0, 0),
	}}
	d.Add(NewParagraph("Shadow test", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Box shadow uses save/restore (q/Q) scope.
	saves := countOps(b, "q")
	restores := countOps(b, "Q")
	if saves != restores {
		t.Errorf("unbalanced save/restore: %d q vs %d Q", saves, restores)
	}
	// Shadow involves a fill operation (f).
	if !containsOp(b, "f") {
		t.Error("box shadow should produce fill operator")
	}
}

func TestDrawTextShadow(t *testing.T) {
	// Paragraph with text shadow.
	p := NewStyledParagraph(
		NewRun("Shadow text", font.Helvetica, 12),
	)
	p.runs[0].TextShadow = &TextShadow{
		OffsetX: 2, OffsetY: 2, Blur: 0,
		Color: RGB(0.5, 0.5, 0.5),
	}

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(p)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Text shadow draws text twice: once for shadow, once for actual.
	// Each text draw uses BT/ET (begin/end text).
	textBlocks := countOps(b, "BT")
	if textBlocks < 2 {
		t.Errorf("expected ≥2 text blocks (shadow + actual), got %d", textBlocks)
	}
}

func TestDrawOutline(t *testing.T) {
	d := NewDiv().
		SetBackground(RGB(1, 1, 1)).
		SetPadding(10)
	d.outlineWidth = 2
	d.outlineStyle = "solid"
	d.outlineColor = RGB(1, 0, 0)
	d.outlineOffset = 4
	d.Add(NewParagraph("Outline test", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Outline draws a rectangle stroke (re + S).
	if !containsOp(b, "re") {
		t.Error("outline should produce rectangle operator")
	}
	if !containsOp(b, "S") {
		t.Error("outline should produce stroke operator")
	}
	// Outline uses RG (stroke color).
	if !containsOp(b, "RG") {
		t.Error("outline should set stroke color (RG)")
	}
}

func TestDrawColumnRules(t *testing.T) {
	cols := NewColumns(3)
	cols.SetGap(20)
	cols.SetColumnRule(ColumnRule{Width: 1, Color: RGB(0.5, 0.5, 0.5), Style: "solid"})
	cols.Add(0, NewParagraph("Col 1 text here", font.Helvetica, 10))
	cols.Add(1, NewParagraph("Col 2 text here", font.Helvetica, 10))
	cols.Add(2, NewParagraph("Col 3 text here", font.Helvetica, 10))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(cols)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// 3 columns → 2 column rules (vertical lines between columns).
	// Each rule is a moveto + lineto (m + l) pair.
	moveOps := countOps(b, "m")
	lineOps := countOps(b, "l")
	if moveOps < 2 {
		t.Errorf("expected ≥2 moveto operators for 2 column rules, got %d", moveOps)
	}
	if lineOps < 2 {
		t.Errorf("expected ≥2 lineto operators for 2 column rules, got %d", lineOps)
	}
}

// TestActualTextArabicWord verifies that an Arabic-only paragraph rendered
// through the full pipeline emits at least one /Span /ActualText marker and
// that the marker's UTF-16BE payload round-trips to the original Arabic
// input string. The check is per-word: every shaped Arabic word should
// carry its own ActualText sequence.
func TestActualTextArabicWord(t *testing.T) {
	// "سلام" — salam, "peace". A single Arabic word, four codepoints,
	// shaped into Presentation Forms-B by ShapeArabic.
	const input = "\u0633\u0644\u0627\u0645"
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph(input, font.Helvetica, 12))
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	stream := string(pages[0].Stream.Bytes())
	if !strings.Contains(stream, "/ActualText") {
		t.Fatalf("expected /ActualText marker in content stream:\n%s", stream)
	}
	// Decode the first ActualText payload back to UTF-8 and compare to
	// the original input. There should be exactly one for this word.
	got := extractFirstActualText(t, stream)
	if got != input {
		t.Errorf("ActualText round trip: got %q, want %q", got, input)
	}
	// Confirm marked-content brackets are balanced (one BDC per EMC).
	bdc := strings.Count(stream, " BDC")
	emc := strings.Count(stream, "EMC")
	if bdc != emc {
		t.Errorf("BDC/EMC unbalanced: %d BDC vs %d EMC", bdc, emc)
	}
}

// TestActualTextLatinUnchanged verifies that a Latin-only paragraph
// produces no /ActualText markers (no shaping happened).
func TestActualTextLatinUnchanged(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Hello world", font.Helvetica, 12))
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	stream := string(pages[0].Stream.Bytes())
	if strings.Contains(stream, "/ActualText") {
		t.Errorf("Latin-only paragraph should produce no /ActualText markers:\n%s", stream)
	}
}

// TestActualTextOptOut verifies that SetActualText(false) suppresses
// /ActualText emission even for shaped Arabic words.
func TestActualTextOptOut(t *testing.T) {
	const input = "\u0633\u0644\u0627\u0645"
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetActualText(false)
	r.Add(NewParagraph(input, font.Helvetica, 12))
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	stream := string(pages[0].Stream.Bytes())
	if strings.Contains(stream, "/ActualText") {
		t.Errorf("SetActualText(false) should suppress markers:\n%s", stream)
	}
}

// TestActualTextMixedArabicLatin verifies that in a paragraph mixing Arabic
// and Latin words, the Arabic words carry /ActualText markers and the Latin
// words are emitted plain. The number of markers should equal the number of
// Arabic words.
func TestActualTextMixedArabicLatin(t *testing.T) {
	// "Hello سلام world" — Latin, Arabic, Latin. Each is its own
	// whitespace-delimited token.
	const input = "Hello \u0633\u0644\u0627\u0645 world"
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph(input, font.Helvetica, 12))
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	stream := string(pages[0].Stream.Bytes())
	count := strings.Count(stream, "/ActualText")
	if count != 1 {
		t.Errorf("expected exactly 1 /ActualText marker for the Arabic word, got %d:\n%s", count, stream)
	}
	got := extractFirstActualText(t, stream)
	if got != "\u0633\u0644\u0627\u0645" {
		t.Errorf("ActualText payload: got %q, want %q", got, "\u0633\u0644\u0627\u0645")
	}
}

// extractFirstActualText decodes the first /Span /ActualText literal-string
// payload found in stream and returns its UTF-8 representation. It is the
// inverse of content.Stream.BeginMarkedContentActualText. The stream may
// contain other operators on either side of the marker.
func extractFirstActualText(t *testing.T, stream string) string {
	t.Helper()
	const marker = "/Span <</ActualText ("
	start := strings.Index(stream, marker)
	if start < 0 {
		t.Fatalf("no /ActualText marker found")
	}
	body := stream[start+len(marker):]
	// Walk forward, honoring escape sequences, until we hit the closing
	// ')' that terminates the literal string. Track parenthesis nesting
	// because PDF allows balanced unescaped parentheses inside strings.
	depth := 1
	end := -1
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' {
			// Skip the escape sequence: either \n/\r/\t/\\/(/) (one char)
			// or \ddd (up to three octal digits).
			if i+1 >= len(body) {
				break
			}
			next := body[i+1]
			if next >= '0' && next <= '7' {
				j := 1
				for j < 3 && i+1+j < len(body) && body[i+1+j] >= '0' && body[i+1+j] <= '7' {
					j++
				}
				i += j
				continue
			}
			i++
			continue
		}
		if c == '(' {
			depth++
			continue
		}
		if c == ')' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end < 0 {
		t.Fatalf("unterminated /ActualText literal")
	}
	literal := body[:end]
	// Undo PDF literal-string escapes to recover raw bytes.
	var raw []byte
	for i := 0; i < len(literal); i++ {
		c := literal[i]
		if c != '\\' {
			raw = append(raw, c)
			continue
		}
		if i+1 >= len(literal) {
			t.Fatalf("trailing backslash")
		}
		next := literal[i+1]
		switch next {
		case '\\', '(', ')':
			raw = append(raw, next)
			i++
		case 'n':
			raw = append(raw, '\n')
			i++
		case 'r':
			raw = append(raw, '\r')
			i++
		case 't':
			raw = append(raw, '\t')
			i++
		default:
			if next < '0' || next > '7' {
				t.Fatalf("unsupported escape %q", next)
			}
			var v byte
			j := 0
			for j < 3 && i+1+j < len(literal) && literal[i+1+j] >= '0' && literal[i+1+j] <= '7' {
				v = v*8 + (literal[i+1+j] - '0')
				j++
			}
			raw = append(raw, v)
			i += j
		}
	}
	if len(raw) < 2 || raw[0] != 0xFE || raw[1] != 0xFF {
		t.Fatalf("missing UTF-16BE BOM in payload: %x", raw)
	}
	raw = raw[2:]
	if len(raw)%2 != 0 {
		t.Fatalf("odd-length UTF-16BE payload: %x", raw)
	}
	var runes []rune
	for i := 0; i < len(raw); i += 2 {
		u := uint16(raw[i])<<8 | uint16(raw[i+1])
		if u >= 0xD800 && u <= 0xDBFF {
			if i+3 >= len(raw) {
				t.Fatalf("dangling high surrogate at %d", i)
			}
			lo := uint16(raw[i+2])<<8 | uint16(raw[i+3])
			r := 0x10000 + (uint32(u-0xD800) << 10) + uint32(lo-0xDC00)
			runes = append(runes, rune(r))
			i += 2
			continue
		}
		runes = append(runes, rune(u))
	}
	return string(runes)
}

func TestDrawSaveRestoreBalance(t *testing.T) {
	// Complex element: Div with background, border, shadow, outline.
	// Verify all q/Q are balanced.
	d := NewDiv().
		SetBackground(RGB(0.9, 0.9, 0.9)).
		SetBorder(SolidBorder(1, ColorBlack)).
		SetOpacity(0.8).
		SetPadding(10)
	d.outlineWidth = 1
	d.outlineStyle = "solid"
	d.outlineColor = ColorBlack
	d.boxShadows = []BoxShadow{{OffsetX: 2, OffsetY: 2, Blur: 4, Color: RGB(0, 0, 0)}}
	d.Add(NewParagraph("Complex", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	saves := countOps(b, "q")
	restores := countOps(b, "Q")
	if saves != restores {
		t.Errorf("UNBALANCED save/restore: %d q vs %d Q — this will corrupt the graphics state", saves, restores)
	}
}
