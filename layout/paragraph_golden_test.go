// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// checkGolden compares got against the committed golden file, or writes it
// when UPDATE_GOLDEN is set. Regenerating goldens must never run in CI — a
// stale-golden failure there is the safety net working as intended.
func checkGolden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", "paragraph_golden", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s — run UPDATE_GOLDEN=1 go test to create: %v", path, err)
	}
	if string(want) != got {
		t.Errorf("golden mismatch for %s\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

// f4 formats a float with fixed 4-decimal precision so goldens are stable
// across architectures.
func f4(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}

// checkFinite fails the test if v is NaN/Inf — a dumped value like that is
// a product bug this corpus exists to catch, not something to pin silently.
func checkFinite(t *testing.T, label string, v float64) {
	t.Helper()
	if v != v || v > 1e300 || v < -1e300 {
		t.Fatalf("%s: non-finite geometry value: %v", label, v)
	}
}

// dumpLines renders the geometry of lines exactly as Layout produced them.
func dumpLines(t *testing.T, sb *strings.Builder, lines []Line) {
	t.Helper()
	for i, line := range lines {
		checkFinite(t, fmt.Sprintf("line %d width", i), line.Width)
		checkFinite(t, fmt.Sprintf("line %d height", i), line.Height)
		fmt.Fprintf(sb, "line %d: width=%s height=%s spaceW=%s isLast=%v align=%d spaceBefore=%s\n",
			i, f4(line.Width), f4(line.Height), f4(line.SpaceW), line.IsLast, line.Align, f4(line.SpaceBefore))
		for j, w := range line.Words {
			checkFinite(t, fmt.Sprintf("line %d word %d width", i, j), w.Width)
			if w.FontSize <= 0 {
				t.Fatalf("line %d word %d: non-positive font size %v", i, j, w.FontSize)
			}
			fmt.Fprintf(sb, "  word %d: %q w=%s spaceAfter=%s fontSize=%s break=%v embedded=%v\n",
				j, w.Text, f4(w.Width), f4(w.SpaceAfter), f4(w.FontSize), w.LineBreak, w.Embedded != nil)
		}
	}
}

// dumpParagraph lays out p at maxWidth and returns a deterministic
// geometry dump: every line and word, followed by the width/height
// summary trailer.
func dumpParagraph(t *testing.T, p *Paragraph, maxWidth float64) string {
	t.Helper()
	var sb strings.Builder
	lines := p.Layout(maxWidth)
	dumpLines(t, &sb, lines)
	fmt.Fprintf(&sb, "minWidth=%s maxWidth=%s measureHeight(W)=%s lines(W)=%d\n",
		f4(p.MinWidth()), f4(p.MaxWidth()), f4(p.MeasureHeight(maxWidth)), p.MeasureLines(maxWidth))
	return sb.String()
}

// TestParagraphGolden pins layout/paragraph.go's word-wrap and measurement
// geometry against a fixed corpus of standard-font paragraphs. This is the
// behavior-preservation net for refactors of paragraph.go: a
// golden diff here means observable line-breaking or measurement changed.
func TestParagraphGolden(t *testing.T) {
	t.Run("simple_wrap", func(t *testing.T) {
		p := NewParagraph("Hello World", font.Helvetica, 12)
		checkGolden(t, "simple_wrap", dumpParagraph(t, p, 40))
	})

	t.Run("wide_single_line", func(t *testing.T) {
		p := NewParagraph("Hello World", font.Helvetica, 12)
		checkGolden(t, "wide_single_line", dumpParagraph(t, p, 500))
	})

	t.Run("justify", func(t *testing.T) {
		p := NewParagraph("The quick brown fox jumps over the lazy dog. It runs fast.", font.Helvetica, 12)
		p.SetAlign(AlignJustify)
		checkGolden(t, "justify", dumpParagraph(t, p, 120))
	})

	t.Run("center_right", func(t *testing.T) {
		// Width forces a two-line wrap so per-line Width differs between
		// lines, not just the align tag.
		const text = "The quick brown fox jumps"
		var sb strings.Builder
		fmt.Fprintln(&sb, "== AlignCenter ==")
		center := NewParagraph(text, font.Helvetica, 12)
		center.SetAlign(AlignCenter)
		dumpLines(t, &sb, center.Layout(80))
		fmt.Fprintln(&sb, "== AlignRight ==")
		right := NewParagraph(text, font.Helvetica, 12)
		right.SetAlign(AlignRight)
		dumpLines(t, &sb, right.Layout(80))
		checkGolden(t, "center_right", sb.String())
	})

	t.Run("long_word_break", func(t *testing.T) {
		p := NewParagraph(strings.Repeat("x", 80), font.Helvetica, 12)
		checkGolden(t, "long_word_break", dumpParagraph(t, p, 100))
	})

	t.Run("cjk_break", func(t *testing.T) {
		// The only embeddable font fixture checked into the repo
		// (font/testdata/synthetic_cjk.ttf) covers exactly ten CJK
		// ideographs; see loadSyntheticCJKEmbedded's package comment in
		// marginbox_embedded_glyph_issue328_test.go. Build the corpus
		// string from that covered set so glyphs resolve to real GIDs
		// instead of .notdef.
		emb := loadSyntheticCJKEmbedded(t)
		p := NewParagraphEmbedded("中华人民共和国是一个", emb, 12)
		checkGolden(t, "cjk_break", dumpParagraph(t, p, 60))
	})

	t.Run("explicit_newlines", func(t *testing.T) {
		p := NewParagraph("Line one\nLine two\nLine three", font.Helvetica, 12)
		checkGolden(t, "explicit_newlines", dumpParagraph(t, p, 200))
	})

	t.Run("mixed_runs", func(t *testing.T) {
		p := NewStyledParagraph(
			TextRun{Text: "Big ", Font: font.TimesRoman, FontSize: 18},
			TextRun{Text: "small text", Font: font.Courier, FontSize: 10},
		)
		checkGolden(t, "mixed_runs", dumpParagraph(t, p, 100))
	})

	t.Run("leading", func(t *testing.T) {
		p := NewParagraph("Hello World", font.Helvetica, 12)
		p.SetLeading(2.0)
		checkGolden(t, "leading", dumpParagraph(t, p, 40))
	})

	t.Run("first_line_indent", func(t *testing.T) {
		p := NewParagraph("The quick brown fox jumps over the lazy dog", font.Helvetica, 12)
		p.SetFirstLineIndent(24)
		checkGolden(t, "first_line_indent", dumpParagraph(t, p, 100))
	})

	t.Run("split_after_line", func(t *testing.T) {
		p := NewParagraph("Hello World", font.Helvetica, 12)
		head, tail := p.SplitAfterLine(1, 40)
		var sb strings.Builder
		fmt.Fprintln(&sb, "== head ==")
		if head != nil {
			dumpLines(t, &sb, head.Layout(40))
		} else {
			fmt.Fprintln(&sb, "<nil>")
		}
		fmt.Fprintln(&sb, "== tail ==")
		if tail != nil {
			dumpLines(t, &sb, tail.Layout(40))
		} else {
			fmt.Fprintln(&sb, "<nil>")
		}
		checkGolden(t, "split_after_line", sb.String())
	})

	t.Run("ellipsis", func(t *testing.T) {
		p := NewParagraph("This text is far too long to fit on one line", font.Helvetica, 12)
		p.SetEllipsis(true)
		checkGolden(t, "ellipsis", dumpParagraph(t, p, 60))
	})
}
