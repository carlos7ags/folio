// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/carlos7ags/folio/font"
)

// Paragraph is a block of text that word-wraps within the available width.
// It is composed of one or more TextRuns, each with its own font, size,
// and color. All runs flow together as a single word-wrapped unit.
type Paragraph struct {
	background       *Color            // background fill color (nil = transparent)
	stringSets       map[string]string // CSS string-set values to capture
	wordBreak        string            // "normal" (default), "break-all", "keep-all"
	hyphens          string            // "none", "manual" (default), "auto" (automatic hyphenation)
	runs             []TextRun
	leading          float64 // line height multiplier (e.g. 1.2 means 120% of fontSize)
	spaceBefore      float64 // extra space before the paragraph (points)
	spaceAfter       float64 // extra space after the paragraph (points)
	align            Align
	textAlignLast    Align     // alignment for the last line (0 = use default)
	direction        Direction // base text direction for bidi layout
	firstIndent      float64   // first-line indent (points, from CSS text-indent)
	orphans          int       // min lines at bottom of page before break (0 = disabled)
	widows           int       // min lines at top of page after break (0 = disabled)
	alignSet         bool      // true if SetAlign was called explicitly
	ellipsis         bool      // if true, truncate overflowing text with "..."
	textAlignLastSet bool      // true if textAlignLast was explicitly set
}

// NewParagraph creates a paragraph with a single run using a standard PDF font.
// Panics if f is nil or fontSize is not positive.
func NewParagraph(text string, f *font.Standard, fontSize float64) *Paragraph {
	if f == nil {
		panic("layout.NewParagraph: nil font")
	}
	if fontSize <= 0 {
		panic("layout.NewParagraph: fontSize must be positive")
	}
	text = normalizeText(text)
	return &Paragraph{
		runs:    []TextRun{{Text: text, Font: f, FontSize: fontSize}},
		leading: 1.2,
		align:   AlignLeft,
	}
}

// NewParagraphEmbedded creates a paragraph with a single run using an embedded font.
// Panics if ef is nil or fontSize is not positive.
func NewParagraphEmbedded(text string, ef *font.EmbeddedFont, fontSize float64) *Paragraph {
	if ef == nil {
		panic("layout.NewParagraphEmbedded: nil font")
	}
	if fontSize <= 0 {
		panic("layout.NewParagraphEmbedded: fontSize must be positive")
	}
	text = normalizeText(text)
	return &Paragraph{
		runs:    []TextRun{{Text: text, Embedded: ef, FontSize: fontSize}},
		leading: 1.2,
		align:   AlignLeft,
	}
}

// NewStyledParagraph creates a paragraph from multiple styled runs.
// Runs are concatenated and word-wrapped as a single flowing text.
// Panics if any text run has both Font and Embedded nil.
// Runs with InlineElement or IsLineBreak set are exempt from the font
// requirement.
//
// The runs argument is not mutated. NFC normalization is performed on
// a fresh copy so callers retain ownership of their input slice.
func NewStyledParagraph(runs ...TextRun) *Paragraph {
	for i, r := range runs {
		if r.InlineElement != nil || r.IsLineBreak {
			continue
		}
		if r.Font == nil && r.Embedded == nil {
			panic(fmt.Sprintf("layout.NewStyledParagraph: run %d has nil Font and nil Embedded", i))
		}
	}
	return &Paragraph{
		runs:    normalizeRuns(runs),
		leading: 1.2,
		align:   AlignLeft,
	}
}

// AddRun appends a styled run to the paragraph.
// Panics if the run has both Font and Embedded nil (unless InlineElement
// or IsLineBreak is set).
func (p *Paragraph) AddRun(r TextRun) *Paragraph {
	if r.InlineElement == nil && !r.IsLineBreak && r.Font == nil && r.Embedded == nil {
		panic("layout.Paragraph.AddRun: run has nil Font and nil Embedded")
	}
	r.Text = normalizeText(r.Text)
	p.runs = append(p.runs, r)
	return p
}

// SetLeading sets the line height multiplier (default 1.2).
func (p *Paragraph) SetLeading(l float64) *Paragraph {
	p.leading = l
	return p
}

// Runs returns a copy of the paragraph's text runs for inspection.
// Mutating the returned slice does not affect the paragraph.
func (p *Paragraph) Runs() []TextRun {
	out := make([]TextRun, len(p.runs))
	copy(out, p.runs)
	return out
}

// SetAlign sets the horizontal text alignment. When this method is called
// explicitly, the alignment is treated as an author override and will not
// be changed by the automatic RTL default (which would otherwise flip the
// alignment to AlignRight for right-to-left paragraphs).
func (p *Paragraph) SetAlign(a Align) *Paragraph {
	p.align = a
	p.alignSet = true
	return p
}

// Align reports the paragraph's text alignment. Returns the value
// most recently set via SetAlign, or the zero-value AlignLeft if
// SetAlign has never been called.
func (p *Paragraph) Align() Align { return p.align }

// SetDirection sets the base text direction for bidi layout. The default
// is DirectionAuto, which auto-detects from the first strong directional
// character in the paragraph text. When direction resolves to RTL and no
// explicit SetAlign call has been made, the paragraph defaults to
// right-aligned.
func (p *Paragraph) SetDirection(d Direction) *Paragraph {
	p.direction = d
	return p
}

// Direction returns the paragraph's base text direction setting.
func (p *Paragraph) Direction() Direction {
	return p.direction
}

// SetSpaceBefore sets extra vertical space before the paragraph (in points).
func (p *Paragraph) SetSpaceBefore(pts float64) *Paragraph {
	p.spaceBefore = pts
	return p
}

// SetSpaceAfter sets extra vertical space after the paragraph (in points).
func (p *Paragraph) SetSpaceAfter(pts float64) *Paragraph {
	p.spaceAfter = pts
	return p
}

// GetSpaceBefore returns the extra vertical space before the paragraph.
func (p *Paragraph) GetSpaceBefore() float64 { return p.spaceBefore }

// GetSpaceAfter returns the extra vertical space after the paragraph.
func (p *Paragraph) GetSpaceAfter() float64 { return p.spaceAfter }

// SetBackground sets a background fill color for the paragraph.
func (p *Paragraph) SetBackground(c Color) *Paragraph {
	p.background = &c
	return p
}

// Background returns a copy of the paragraph's background fill color, or nil
// if the paragraph has no block-level background. A copy is returned so callers
// cannot mutate the paragraph's internal fill through the pointer. Provided for
// testing and for callers that own the box fill (e.g. a wrapping Div) and need
// to detect and clear a redundant paragraph-level background.
func (p *Paragraph) Background() *Color {
	if p.background == nil {
		return nil
	}
	c := *p.background
	return &c
}

// ClearBackground removes the paragraph's block-level background fill.
// Per-run/word BackgroundColor (inline <span> highlights) is unaffected.
func (p *Paragraph) ClearBackground() *Paragraph {
	p.background = nil
	return p
}

// ClearMatchingRunBackgrounds removes the per-run/word BackgroundColor highlight
// from every TextRun whose background equals bg. It returns true if at least one
// run background was cleared.
//
// This exists for the case where an inline element (e.g. a bare <span>) that
// carries a block-level background is blockified into its own rounded box. The
// span's CSS background becomes a TextRun.BackgroundColor highlight, which the
// renderer paints as a plain (square-cornered) rectangle behind the text. When a
// wrapping Div now owns and draws the rounded box fill, that run highlight is a
// redundant square overdraw on top of the rounded corners (issue #329). Clearing
// it leaves a single rounded fill.
func (p *Paragraph) ClearMatchingRunBackgrounds(bg Color) bool {
	cleared := false
	for i := range p.runs {
		if rc := p.runs[i].BackgroundColor; rc != nil && *rc == bg {
			p.runs[i].BackgroundColor = nil
			cleared = true
		}
	}
	return cleared
}

// HasRunBackground reports whether any TextRun carries a per-run/word
// BackgroundColor highlight equal to bg.
func (p *Paragraph) HasRunBackground(bg Color) bool {
	for i := range p.runs {
		if rc := p.runs[i].BackgroundColor; rc != nil && *rc == bg {
			return true
		}
	}
	return false
}

// SetFirstLineIndent sets the indentation for the first line (in points).
// This corresponds to the CSS text-indent property.
func (p *Paragraph) SetFirstLineIndent(pts float64) *Paragraph {
	p.firstIndent = pts
	return p
}

// SetOrphans sets the minimum number of lines that must remain at the
// bottom of a page before a page break. If fewer lines would remain,
// the entire paragraph is pushed to the next page (via KeepWithNext).
// Default is 0 (disabled). Typical value: 2.
func (p *Paragraph) SetOrphans(n int) *Paragraph {
	p.orphans = n
	return p
}

// SetWordBreak sets the word-break behavior. "break-all" allows breaking
// within any word at character boundaries. "keep-all" prevents CJK characters
// from breaking individually (breaks only at spaces). Default is "normal"
// (CJK breaks at character boundaries, Latin breaks at spaces only).
func (p *Paragraph) SetWordBreak(wb string) *Paragraph {
	p.wordBreak = wb
	return p
}

// SetHyphens sets the hyphenation mode. "auto" enables automatic hyphenation
// at syllable boundaries. "none" disables all hyphenation. "manual" (default)
// only breaks at soft hyphens (&shy;).
func (p *Paragraph) SetHyphens(h string) *Paragraph {
	p.hyphens = h
	return p
}

// SetTextAlignLast sets the alignment for the last line of the paragraph.
// This is used to override the normal alignment (e.g. justify) for just
// the final line.
func (p *Paragraph) SetTextAlignLast(a Align) *Paragraph {
	p.textAlignLast = a
	p.textAlignLastSet = true
	return p
}

// SetStringSet attaches a CSS string-set value to this paragraph.
func (p *Paragraph) SetStringSet(name, value string) *Paragraph {
	if p.stringSets == nil {
		p.stringSets = make(map[string]string)
	}
	p.stringSets[name] = value
	return p
}

// SetEllipsis enables or disables text truncation with "..." when text
// overflows the available width. Typically used with overflow:hidden and
// a fixed width container.
func (p *Paragraph) SetEllipsis(v bool) *Paragraph {
	p.ellipsis = v
	return p
}

// SetWidows sets the minimum number of lines that must appear at the
// top of a page after a page break. If fewer lines would appear,
// additional lines are pulled from the previous page. Implemented
// by setting KeepWithNext on trailing lines.
// Default is 0 (disabled). Typical value: 2.
func (p *Paragraph) SetWidows(n int) *Paragraph {
	p.widows = n
	return p
}

// Layout implements Element. It splits the paragraph text into wrapped lines
// that fit within maxWidth. Words from different runs carry their own styling.
func (p *Paragraph) Layout(maxWidth float64) []Line {
	// Flatten all runs into a single word list, each word carrying
	// the styling of the run it came from.
	var measured []Word
	var maxFontSize float64

	nextLineBreakFromBr := false // tracks <br> line breaks across runs
	for i, run := range p.runs {
		if run.InlineElement != nil {
			glueAdjacentRuns(measured, p.runs, i)
			measured = append(measured, measureInlineElement(run, maxWidth, measured, p.runs, i))
			continue
		}
		if run.IsLineBreak {
			// <br> marker: the next word on the next run gets LineBreak=true.
			nextLineBreakFromBr = true
			continue
		}

		// Zero the previous word's SpaceAfter when this run abuts it
		// with no whitespace (e.g. "C" + "<sub>8</sub>").
		glueAdjacentRuns(measured, p.runs, i)

		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		words := splitWords(run.Text)

		nextLineBreak := nextLineBreakFromBr
		nextLineBreakFromBr = false
		for _, w := range words {
			if w == lineBreakMarker {
				if nextLineBreak {
					// Consecutive line breaks (\n\n): insert a blank word
					// to produce an empty line in the output.
					measured = append(measured, Word{
						Font:      run.Font,
						Embedded:  run.Embedded,
						FontSize:  run.FontSize,
						LineBreak: true,
					})
				}
				nextLineBreak = true
				continue
			}
			// Build the word from the unshaped text first so that
			// splitMixedBidiWord can split on the original codepoints.
			// Each piece is then shaped independently and its pre-shape
			// text is captured into OriginalText so the renderer can
			// emit ISO 32000-2 §14.9.4 /Span /ActualText markers.
			word := Word{
				Text:            w,
				Font:            run.Font,
				Embedded:        run.Embedded,
				FontSize:        run.FontSize,
				Color:           run.Color,
				Decoration:      run.Decoration,
				LineBreak:       nextLineBreak,
				DecorationColor: run.DecorationColor,
				DecorationStyle: run.DecorationStyle,
				SpaceAfter:      spaceW,
				LetterSpacing:   run.LetterSpacing,
				WordSpacing:     run.WordSpacing,
				BaselineShift:   run.BaselineShift,
				LinkURI:         run.LinkURI,
				TextShadow:      run.TextShadow,
				BackgroundColor: run.BackgroundColor,
			}
			// Split words with mixed bidi levels into sub-words so
			// each piece can be independently reordered by the bidi
			// algorithm. E.g. "מחיר:₪42" splits at the transition
			// between Hebrew and digit characters.
			if subs := splitMixedBidiWord(word); subs != nil {
				for si, sub := range subs {
					shapeAndMeasureWord(&sub, run, measurer)
					if si == 0 {
						sub.LineBreak = nextLineBreak
					}
					measured = append(measured, sub)
				}
			} else {
				shapeAndMeasureWord(&word, run, measurer)
				measured = append(measured, word)
			}
			nextLineBreak = false
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
		}
	}

	if len(measured) == 0 {
		// Empty paragraph: still emit spacing if spaceBefore/spaceAfter is set.
		if p.spaceBefore > 0 || p.spaceAfter > 0 {
			return []Line{{
				Height:      0,
				SpaceBefore: p.spaceBefore,
				SpaceAfterV: p.spaceAfter,
				IsLast:      true,
			}}
		}
		return nil
	}

	// CJK break: split words containing CJK characters at character
	// boundaries so the wrap algorithm can break between them. Skipped
	// for keep-all (CJK words stay intact, break only at spaces).
	if p.wordBreak != "keep-all" {
		measured = breakCJKWords(measured)
	}

	// Break words that don't fit. With word-break:break-all, break ALL words
	// at character boundaries to fill lines maximally.
	if p.wordBreak == "break-all" {
		measured = breakAllWords(measured, maxWidth)
	} else {
		measured = breakLongWords(measured, maxWidth)
	}

	lineHeight := maxFontSize * p.leading

	// Greedy word-wrap.
	// Space between words uses the preceding word's SpaceAfter.
	var lines []Line
	lineStart := 0
	lineWidth := measured[0].Width
	effectiveMax := maxWidth
	if p.firstIndent != 0 {
		effectiveMax = maxWidth - p.firstIndent
	}

	for i := 1; i < len(measured); i++ {
		// Forced line break from \n.
		if measured[i].LineBreak {
			lines = append(lines, Line{
				Words: slices.Clone(measured[lineStart:i]),
				Width: lineWidth, Height: lineHeight, SpaceW: measured[lineStart].SpaceAfter,
			})
			lineStart = i
			lineWidth = measured[i].Width
			effectiveMax = maxWidth
			continue
		}
		spaceW := measured[i-1].SpaceAfter
		candidate := lineWidth + spaceW + measured[i].Width
		if candidate > effectiveMax && lineStart < i {
			// Try hyphenation: if enabled, attempt to break the next word
			// and fit part of it on this line with a hyphen.
			if p.hyphens == "auto" {
				remaining := effectiveMax - lineWidth - spaceW
				if part, rest, ok := hyphenateWord(measured[i], remaining); ok {
					// Fit the first part on this line.
					lineWords := make([]Word, i-lineStart+1)
					copy(lineWords, measured[lineStart:i])
					lineWords[len(lineWords)-1] = part
					lw := lineWidth + spaceW + part.Width
					lines = append(lines, buildLine(lineWords, lw, lineHeight, p.align, false))
					measured[i] = rest
					lineStart = i
					lineWidth = rest.Width
					effectiveMax = maxWidth
					continue
				}
			}
			lines = append(lines, buildLine(measured[lineStart:i], lineWidth, lineHeight, p.align, false))
			lineStart = i
			lineWidth = measured[i].Width
			effectiveMax = maxWidth // subsequent lines use full width
		} else {
			lineWidth = candidate
		}
	}
	// Last line.
	lines = append(lines, buildLine(measured[lineStart:], lineWidth, lineHeight, p.align, true))

	// Bidi reordering: resolve the paragraph direction and reorder each
	// line's words into visual order. Same step as in PlanLayout.
	resolvedDirL := DirectionLTR
	for i, line := range lines {
		reordered, dir := resolveLineBidi(line.Words, p.direction)
		lines[i].Words = reordered
		if i == 0 {
			resolvedDirL = dir
		}
	}
	// Apply RTL default alignment. When direction is explicitly set (from
	// CSS direction:rtl or HTML dir="rtl"), always use it for alignment
	// even if the bidi algorithm resolved LTR from the text content.
	// This matches CSS behavior where direction:rtl right-aligns
	// regardless of the script in the text.
	effectiveDir := resolvedDirL
	if p.direction != DirectionAuto {
		effectiveDir = p.direction
	}
	if effectiveDir == DirectionRTL && !p.alignSet {
		for j := range lines {
			lines[j].Align = AlignRight
		}
	}

	// Apply ellipsis truncation: if enabled, keep only the first line
	// and replace trailing text with "..." if it overflows.
	if p.ellipsis && len(lines) > 1 {
		lines = lines[:1]
		lines[0].IsLast = true
		// Truncate words to fit within maxWidth and append ellipsis.
		lines[0] = truncateWithEllipsis(lines[0], maxWidth)
	}

	// Apply text-align-last: override alignment on the last line.
	if p.textAlignLastSet && len(lines) > 0 {
		lines[len(lines)-1].Align = p.textAlignLast
	}

	// Apply paragraph-level properties to the first/last lines.
	if len(lines) > 0 {
		if p.spaceBefore > 0 {
			lines[0].SpaceBefore = p.spaceBefore
		}
		if p.spaceAfter > 0 {
			lines[len(lines)-1].SpaceAfterV = p.spaceAfter
		}
		if p.background != nil {
			for i := range lines {
				lines[i].Background = p.background
			}
		}

		// Orphans: if the paragraph has more lines than the orphan
		// threshold, mark the first N lines with KeepWithNext so the
		// renderer won't break after fewer than N lines at the bottom.
		if p.orphans > 0 && len(lines) > p.orphans {
			for i := range min(p.orphans, len(lines)-1) {
				lines[i].KeepWithNext = true
			}
		}

		// Widows: if the paragraph has more lines than the widow
		// threshold, mark lines near the end so the renderer pulls
		// enough lines to the next page. We set KeepWithNext on
		// lines starting from (total - widows) to ensure at least
		// `widows` lines land on the next page after any break.
		if p.widows > 0 && len(lines) > p.widows {
			start := max(len(lines)-p.widows-1, 0)
			for i := start; i < len(lines)-1; i++ {
				lines[i].KeepWithNext = true
			}
		}
	}

	return lines
}

// runMeasurer returns the text measurer for a run.
func runMeasurer(run TextRun) font.TextMeasurer {
	if run.Embedded != nil {
		return run.Embedded
	}
	if run.Font != nil {
		return run.Font
	}
	// Fallback: use Helvetica if no font is set (defensive).
	return font.Helvetica
}

// shapeAndMeasureWord runs the appropriate per-script shaper over a
// single Word produced by paragraph layout, measures its post-shaping
// width, and captures the pre-shaping text into OriginalText for
// ActualText recovery. This function centralises the per-script
// dispatch so that the two nearly-identical word-building loops in
// Layout() can share a single code path.
//
// Arabic words are routed through ShapeArabic, which substitutes
// Presentation Forms-B codepoints in place and leaves Width to be
// measured via MeasureString. Indic words (Devanagari, Bengali,
// Gujarati, Gurmukhi, Kannada, Malayalam, Oriya, Tamil, Telugu) are
// routed through ShapeIndicWithEmbedded, which returns a glyph ID
// stream; the stream is stored on w.GIDs and measured via the
// EmbeddedFont.MeasureGIDs fast path. All other scripts pass
// through unchanged.
func shapeAndMeasureWord(w *Word, run TextRun, measurer font.TextMeasurer) {
	origText := w.Text

	// Indic path: try the shaper first. It only fires for
	// EmbeddedFont runs that expose GSUB; otherwise we fall through
	// to the rune-based path so fonts without Indic tables still
	// render (if imperfectly) via the ordinary MeasureString.
	if w.Embedded != nil {
		if sc := indicScriptOfWord(origText); sc != ScriptCommon {
			if gids, ok := ShapeIndicWithEmbedded(origText, w.Embedded, sc); ok {
				w.GIDs = gids
				w.Width = w.Embedded.MeasureGIDs(gids, run.FontSize)
				if run.LetterSpacing != 0 && len(gids) > 1 {
					w.Width += run.LetterSpacing * float64(len(gids)-1)
				}
				// Preserve the original codepoints for ActualText
				// recovery and for copy/paste; Text stays equal to
				// OriginalText so the bidi and wrapping passes still
				// see real characters.
				w.OriginalText = origText
				return
			}
		}
	}

	// Arabic / general rune path.
	w.Text = ShapeArabic(w.Text)
	// CSS counter(page) / counter(pages) placeholders are resolved at
	// content-stream emission once pagination is final. Measure them
	// here using a fixed-digit reservation so line breaks stay valid
	// after substitution. Width of the actual digits is bounded by the
	// reserved width for typical document lengths.
	measureText := w.Text
	if hasCounterPlaceholder(measureText) {
		measureText = expandCountersForMeasure(measureText)
	}
	w.Width = measurer.MeasureString(measureText, run.FontSize)
	if run.LetterSpacing != 0 && len([]rune(measureText)) > 1 {
		w.Width += run.LetterSpacing * float64(len([]rune(measureText))-1)
	}
	if w.Text != origText {
		w.OriginalText = origText
	}
}

// computeBaseline returns the distance from the top of the line box to the
// text baseline using CSS half-leading (CSS 2.1 §10.8.1):
//
//	leading = lineH - (ascent + descent)
//	half-leading = leading / 2
//	baseline from top = half-leading + ascent = (lineH + ascent - descent) / 2
//
// When a line has mixed font sizes, the largest baseline wins so that all
// text on the line shares a common baseline position.
func computeBaseline(words []Word, lineH float64) float64 {
	baseline := 0.0
	for _, w := range words {
		if w.InlineBlock != nil {
			continue
		}
		var ascent, descent float64
		if w.Font != nil {
			ascent = w.Font.Ascent(w.FontSize)
			descent = w.Font.Descent(w.FontSize)
		} else if w.Embedded != nil {
			face := w.Embedded.Face()
			upem := float64(face.UnitsPerEm())
			ascent = float64(face.Ascent()) / upem * w.FontSize
			descent = -float64(face.Descent()) / upem * w.FontSize
		}
		if ascent > 0 {
			wb := (lineH + ascent - descent) / 2
			if wb > baseline {
				baseline = wb
			}
		}
	}
	if baseline == 0 {
		baseline = lineH * 0.8
	}
	return baseline
}

// runsAdjacent returns true when run at index i directly abuts the previous
// run with no whitespace between them. This happens with inline elements like
// <sub>/<sup> where "C<sub>8</sub>" produces runs ["C", "8"] with no space.
// When true, the last word of the previous run should have SpaceAfter = 0
// so the words render flush against each other.
func runsAdjacent(runs []TextRun, i int) bool {
	if i <= 0 || len(runs) <= i {
		return false
	}
	cur := runs[i]
	prev := runs[i-1]
	// Skip inline element runs — they have their own spacing logic.
	if cur.InlineElement != nil || prev.InlineElement != nil {
		return false
	}
	if cur.Text == "" || prev.Text == "" {
		return false
	}
	// If previous run ends without whitespace and current starts without
	// whitespace, the runs are adjacent (no inter-word space).
	lastChar := prev.Text[len(prev.Text)-1]
	firstChar := cur.Text[0]
	return !isASCIISpace(lastChar) && !isASCIISpace(firstChar)
}

// isASCIISpace checks for ASCII whitespace. HTML parsers normalize most
// whitespace to ASCII, so this covers the practical cases. Non-ASCII
// whitespace (e.g. \u00A0 non-breaking space) is not treated as a
// separator, which matches browser behavior (NBSP doesn't break words).
func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// glueAdjacentRuns zeroes SpaceAfter on the last measured word when the
// current run directly abuts the previous run with no whitespace.
func glueAdjacentRuns(measured []Word, runs []TextRun, runIdx int) {
	if runsAdjacent(runs, runIdx) && len(measured) > 0 {
		measured[len(measured)-1].SpaceAfter = 0
	}
}

// measureInlineElement measures an inline element run and returns a Word
// representing it as an inline-block in the paragraph's word stream.
// Both Layout() and measureWords() use this to keep measurement logic
// in one place.
func measureInlineElement(run TextRun, maxWidth float64, measured []Word, runs []TextRun, runIdx int) Word {
	plan := run.InlineElement.PlanLayout(LayoutArea{
		Width: maxWidth, Height: 1e6,
	})
	var iw, ih float64
	if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
		iw = plan.Blocks[0].Width
		ih = plan.Consumed
	}
	return Word{
		InlineBlock:  run.InlineElement,
		InlineWidth:  iw,
		InlineHeight: ih,
		Width:        iw,
		SpaceAfter:   inlineSpaceAfter(measured, runs, runIdx),
	}
}

// inlineSpaceAfter computes the space-after width for an inline element word.
// It uses surrounding text metrics when available: first from already-measured
// words (preceding context), then by scanning forward in the runs for the
// next text run (following context). When no text context exists at all
// (e.g. a paragraph of only inline elements), it returns 0 so the elements
// sit flush against each other.
func inlineSpaceAfter(measured []Word, runs []TextRun, currentIdx int) float64 {
	// Preceding word has known metrics — inherit its spacing.
	if len(measured) > 0 {
		return measured[len(measured)-1].SpaceAfter
	}
	// Look ahead for the next text run to derive font-based spacing.
	for j := currentIdx + 1; j < len(runs); j++ {
		r := runs[j]
		if r.InlineElement != nil {
			continue
		}
		return runMeasurer(r).MeasureString(" ", r.FontSize) + r.WordSpacing
	}
	// No text context at all — flush spacing.
	return 0
}

// buildLine creates a Line from a slice of words.
func buildLine(words []Word, width, height float64, align Align, isLast bool) Line {
	// SpaceW: use the first word's SpaceAfter as default (for single-font compatibility).
	spaceW := 0.0
	if len(words) > 0 {
		spaceW = words[0].SpaceAfter
	}
	return Line{
		Words:  slices.Clone(words),
		Width:  width,
		Height: height,
		SpaceW: spaceW,
		Align:  align,
		IsLast: isLast,
	}
}

// breakCJKWords splits words containing CJK characters at break opportunities
// so the word-wrap algorithm can break between CJK characters. Each CJK
// sub-token gets SpaceAfter=0 (no inter-word spacing in CJK text), except
// the last sub-token which inherits the original word's SpaceAfter. Kinsoku
// rules are respected: opening punctuation groups with the following character,
// closing punctuation groups with the preceding character.
func breakCJKWords(words []Word) []Word {
	var result []Word
	for _, w := range words {
		tokens := splitCJKToken(w.Text)
		if len(tokens) <= 1 {
			result = append(result, w)
			continue
		}
		// Determine the measure function for re-measuring sub-tokens.
		var measure func(string) float64
		if w.Embedded != nil {
			emb := w.Embedded
			measure = func(s string) float64 { return emb.MeasureString(s, w.FontSize) }
		} else if w.Font != nil {
			f := w.Font
			measure = func(s string) float64 { return f.MeasureString(s, w.FontSize) }
		} else {
			result = append(result, w)
			continue
		}

		for j, tok := range tokens {
			sub := w
			sub.Text = tok
			sub.Width = measure(tok)
			if sub.LetterSpacing != 0 && len([]rune(tok)) > 1 {
				sub.Width += sub.LetterSpacing * float64(len([]rune(tok))-1)
			}
			if j < len(tokens)-1 {
				sub.SpaceAfter = 0 // no inter-word space within CJK run
			}
			// Only the first sub-token inherits LineBreak from the original word.
			if j > 0 {
				sub.LineBreak = false
			}
			result = append(result, sub)
		}
	}
	return result
}

// breakLongWords splits any word exceeding maxWidth into character-level
// chunks so that the word-wrap algorithm can handle them. Words that fit
// are unchanged.
//
// For shaped words (where OriginalText carries the pre-shape Unicode and
// either Text was Arabic-shaped to Presentation Forms-B or GIDs carries
// the Indic shaper output), the function breaks on OriginalText
// codepoint boundaries and re-runs the appropriate shaper for each
// chunk. This preserves Word.OriginalText (used by the renderer's
// /ActualText marked-content wrapper, ISO 32000-2 §14.9.4) and
// Word.GIDs (needed by the Identity-H emit path for Indic). A prior
// implementation dropped both fields on subsequent chunks, breaking
// copy/paste recovery and Devanagari rendering at chunk boundaries
// (#255).
//
// Re-shaping per chunk produces context-independent glyphs at the chunk
// boundaries — Arabic cursive joins won't carry across a break, and
// Indic ligatures that would span a break do not form. That's the
// expected outcome for a character-broken word: the layout engine has
// already decided the word must split at this character, and shaping
// honours that decision.
func breakLongWords(words []Word, maxWidth float64) []Word {
	var result []Word
	for _, w := range words {
		if w.Width <= maxWidth {
			result = append(result, w)
			continue
		}

		// Source text for codepoint-boundary breaks: pre-shape
		// OriginalText when the word was shaped, else the post-shape
		// Text. The shaped flag drives both the rune-iteration source
		// and the per-chunk shaping path below.
		shaped := w.OriginalText != ""
		sourceText := w.Text
		if shaped {
			sourceText = w.OriginalText
		}
		runes := []rune(sourceText)

		shapeChunk := shapedChunkBuilder(&w, shaped)
		// Probe the first single-rune chunk to detect a missing
		// measurer (no Font and no Embedded). Without one we cannot
		// width-test candidates, so pass the source word through
		// verbatim — matches the original break-by-characters
		// fallback semantics.
		if _, _, _, ok := shapeChunk(string(runes[0:1])); !ok {
			result = append(result, w)
			continue
		}

		start := 0
		for start < len(runes) {
			end := start + 1
			for end < len(runes) {
				_, _, width, _ := shapeChunk(string(runes[start : end+1]))
				if width > maxWidth {
					break
				}
				end++
			}
			chunkSource := string(runes[start:end])
			chunkText, chunkGIDs, chunkWidth, _ := shapeChunk(chunkSource)
			chunk := Word{
				Text:          chunkText,
				Width:         chunkWidth,
				Font:          w.Font,
				Embedded:      w.Embedded,
				FontSize:      w.FontSize,
				Color:         w.Color,
				Decoration:    w.Decoration,
				SpaceAfter:    0, // no inter-word space within a broken word
				LetterSpacing: w.LetterSpacing,
				WordSpacing:   w.WordSpacing,
				GIDs:          chunkGIDs,
			}
			if shaped {
				// Per-chunk pre-shape slice. Without this the
				// renderer's /ActualText wrapper would emit the
				// shaped codepoints, breaking copy/paste recovery.
				chunk.OriginalText = chunkSource
			}
			result = append(result, chunk)
			start = end
		}
	}
	return result
}

// shapedChunkBuilder returns a function that produces (post-shape text,
// GIDs, width, ok) for an arbitrary slice of a word's source text. It
// captures w by pointer so that per-chunk results inherit the word's
// font / Embedded / size without allocating closures over each field.
// The shaped flag selects the path: unshaped words route through
// MeasureString on the source text; shaped words re-run the Indic
// shaper when w.GIDs was non-nil, otherwise the Arabic shaper.
func shapedChunkBuilder(w *Word, shaped bool) func(chunkSrc string) (text string, gids []uint16, width float64, ok bool) {
	return func(chunkSrc string) (text string, gids []uint16, width float64, ok bool) {
		if !shaped {
			if w.Embedded != nil {
				return chunkSrc, nil, w.Embedded.MeasureString(chunkSrc, w.FontSize), true
			}
			if w.Font != nil {
				return chunkSrc, nil, w.Font.MeasureString(chunkSrc, w.FontSize), true
			}
			return "", nil, 0, false
		}
		// Indic path — try only if the original word carried GIDs
		// (i.e. the initial shaper succeeded). The script of the
		// chunk could differ from the script of the whole word at
		// boundaries between scripts, but breakLongWords operates on
		// a single word from the shaper's segmentation, so the chunk
		// shares the parent word's script.
		if w.GIDs != nil && w.Embedded != nil {
			if sc := indicScriptOfWord(chunkSrc); sc != ScriptCommon {
				if g, ok := ShapeIndicWithEmbedded(chunkSrc, w.Embedded, sc); ok {
					return chunkSrc, g, w.Embedded.MeasureGIDs(g, w.FontSize), true
				}
			}
		}
		// Arabic / general rune fallback. ShapeArabic is a no-op for
		// non-Arabic input so this also handles a defensive fallback
		// path where the Indic shaper failed.
		shapedText := ShapeArabic(chunkSrc)
		if w.Embedded != nil {
			return shapedText, nil, w.Embedded.MeasureString(shapedText, w.FontSize), true
		}
		if w.Font != nil {
			return shapedText, nil, w.Font.MeasureString(shapedText, w.FontSize), true
		}
		return "", nil, 0, false
	}
}

// hyphenateWord attempts to split a word to fit within `available` points
// using the Liang-Knuth hyphenation algorithm for linguistically correct
// syllable breaks. Returns the first part (with trailing hyphen) and the
// remainder. Returns ok=false if no valid split point is found.
func hyphenateWord(w Word, available float64) (part, rest Word, ok bool) {
	runes := []rune(w.Text)
	if len(runes) < 4 {
		return Word{}, Word{}, false
	}

	var measure func(string) float64
	if w.Embedded != nil {
		measure = func(s string) float64 { return w.Embedded.MeasureString(s, w.FontSize) }
	} else if w.Font != nil {
		measure = func(s string) float64 { return w.Font.MeasureString(s, w.FontSize) }
	} else {
		return Word{}, Word{}, false
	}

	hyphenW := measure("-")

	// Get linguistically valid break points from the hyphenator.
	// Only attempt pattern-based hyphenation for pure-alpha words;
	// fall back to character-boundary splitting for others.
	var breakPoints []int
	if isAlphaWord(w.Text) {
		breakPoints = DefaultHyphenator().Hyphenate(w.Text)
	}

	// Find the latest valid break point that fits.
	bestSplit := -1
	if len(breakPoints) > 0 {
		for i := len(breakPoints) - 1; i >= 0; i-- {
			bp := breakPoints[i]
			prefix := string(runes[:bp])
			pw := measure(prefix) + hyphenW
			if pw <= available {
				bestSplit = bp
				break
			}
		}
	}

	// Fallback: if no pattern-based break fits, try character boundaries
	// (at least 2 chars from each end) for very long words.
	if bestSplit < 0 {
		for i := len(runes) - 2; i >= 2; i-- {
			prefix := string(runes[:i])
			pw := measure(prefix) + hyphenW
			if pw <= available {
				bestSplit = i
				break
			}
		}
	}

	if bestSplit < 0 {
		return Word{}, Word{}, false
	}

	prefixText := string(runes[:bestSplit]) + "-"
	suffixText := string(runes[bestSplit:])

	part = w
	part.Text = prefixText
	part.Width = measure(prefixText)
	part.SpaceAfter = 0

	rest = w
	rest.Text = suffixText
	rest.Width = measure(suffixText)

	return part, rest, true
}

// breakAllWords breaks every word into individual characters, allowing
// the word-wrap algorithm to break within any word (word-break: break-all).
func breakAllWords(words []Word, maxWidth float64) []Word {
	var result []Word
	for _, w := range words {
		runes := []rune(w.Text)
		if len(runes) <= 1 {
			result = append(result, w)
			continue
		}
		measurer := w.Font
		emb := w.Embedded
		var measure func(string) float64
		if emb != nil {
			measure = func(s string) float64 { return emb.MeasureString(s, w.FontSize) }
		} else if measurer != nil {
			measure = func(s string) float64 { return measurer.MeasureString(s, w.FontSize) }
		} else {
			result = append(result, w)
			continue
		}
		// Split into individual characters as separate "words".
		for j, r := range runes {
			ch := string(r)
			cw := Word{
				Text:          ch,
				Width:         measure(ch),
				Font:          w.Font,
				Embedded:      w.Embedded,
				FontSize:      w.FontSize,
				Color:         w.Color,
				Decoration:    w.Decoration,
				LetterSpacing: w.LetterSpacing,
				WordSpacing:   w.WordSpacing,
				SpaceAfter:    0, // no space between characters of same word
			}
			if j == len(runes)-1 {
				cw.SpaceAfter = w.SpaceAfter // preserve inter-word space on last char
			}
			result = append(result, cw)
		}
	}
	return result
}

// MinWidth implements Measurable. Returns the width of the longest word
// (the narrowest the paragraph can be without clipping).
func (p *Paragraph) MinWidth() float64 {
	maxWordW := 0.0
	for _, run := range p.runs {
		measurer := runMeasurer(run)
		for _, w := range splitWords(run.Text) {
			ww := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				ww += run.LetterSpacing * float64(len([]rune(w))-1)
			}
			if ww > maxWordW {
				maxWordW = ww
			}
		}
	}
	return maxWordW
}

// MaxWidth implements Measurable. Returns the width of all text on a single
// line (the natural width without wrapping).
func (p *Paragraph) MaxWidth() float64 {
	total := 0.0
	for _, run := range p.runs {
		// Inline elements (e.g. a nested inline-block) participate in the
		// same line as adjacent text. Treat them like a non-breaking word
		// that contributes its own max-content width plus the same
		// inter-run space the text branch adds below, so a fit-content
		// parent measures wide enough to contain the nested chip.
		if run.InlineElement != nil {
			total += inlineElementMaxWidth(run.InlineElement)
			// Add a space after the inline element (mirrors the per-run
			// trailing space added for text runs).
			total += inlineRunSpace(p.runs, run)
			continue
		}
		measurer := runMeasurer(run)
		words := splitWords(run.Text)
		spaceW := measurer.MeasureString(" ", run.FontSize)
		for i, w := range words {
			ww := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				ww += run.LetterSpacing * float64(len([]rune(w))-1)
			}
			total += ww
			if i < len(words)-1 {
				total += spaceW
			}
		}
		// Add a space between runs.
		if len(words) > 0 {
			total += spaceW
		}
	}
	return total
}

// inlineElementMaxWidth reports the max-content width of an inline element
// run. It prefers the element's own Measurable.MaxWidth(); otherwise it
// falls back to measuring via PlanLayout at a large width and reading the
// first block's width (mirroring how measureInlineElement extracts width).
func inlineElementMaxWidth(el Element) float64 {
	if m, ok := el.(Measurable); ok {
		return m.MaxWidth()
	}
	plan := el.PlanLayout(LayoutArea{Width: 1e6, Height: 1e6})
	if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
		return plan.Blocks[0].Width
	}
	return 0
}

// inlineRunSpace returns the inter-run trailing space to add after an inline
// element run, consistent with the per-run space the text branch of MaxWidth
// adds. It derives the space width from a measurer; inline element runs carry
// no font, so a fallback FontSize is used when none is present.
func inlineRunSpace(runs []TextRun, run TextRun) float64 {
	// An inline-element run typically has no font; derive the space width
	// from the run's own font metrics when available, otherwise fall back
	// to a neighboring text run's measurer.
	if run.Font != nil || run.Embedded != nil {
		return runMeasurer(run).MeasureString(" ", runFontSize(run))
	}
	for _, r := range runs {
		if r.InlineElement == nil && (r.Font != nil || r.Embedded != nil) {
			return runMeasurer(r).MeasureString(" ", r.FontSize)
		}
	}
	return 0
}

// runFontSize returns a non-zero font size for measuring, defaulting to a
// reasonable value when the run carries none.
func runFontSize(run TextRun) float64 {
	if run.FontSize > 0 {
		return run.FontSize
	}
	return 12
}

// MeasureLines reports how many wrapped lines this paragraph produces at
// maxWidth. Equivalent to len(p.Layout(maxWidth)). Useful for clamp
// decisions ("if MeasureLines > 8 then split") without inspecting words.
//
// Safe to call concurrently with other read methods (Layout, MeasureHeight)
// as long as no goroutine is mutating the paragraph through setters.
func (p *Paragraph) MeasureLines(maxWidth float64) int {
	return len(p.Layout(maxWidth))
}

// MeasureHeight reports the rendered height (in points) of this paragraph
// at maxWidth — the sum of each line's Height. Does NOT include
// SpaceBefore or SpaceAfter; the caller adds those if relevant.
//
// Safe to call concurrently with other read methods (Layout, MeasureLines)
// as long as no goroutine is mutating the paragraph through setters.
func (p *Paragraph) MeasureHeight(maxWidth float64) float64 {
	total := 0.0
	for _, line := range p.Layout(maxWidth) {
		total += line.Height
	}
	return total
}

// SplitAfterLine wraps the paragraph at maxWidth and splits it after
// the first n rendered lines. Returns:
//   - head: a paragraph containing the first min(n, total) lines.
//   - tail: a paragraph containing the remaining lines, or nil if no
//     overflow (n >= total).
//
// If n <= 0, head is nil and tail is the entire paragraph.
//
// Both returned paragraphs are clones — the receiver is unchanged.
// Styling (fonts, colors, links, decorations, inline elements, shaped
// scripts) is preserved across the split via the same word-grouping
// rules used for page-break splits.
//
// Spacing ownership: SpaceBefore stays with head only (the tail picks
// up where the head left off mid-paragraph; spaceBefore would double).
// SpaceAfter stays with tail only (symmetric reason). FirstLineIndent
// is dropped from both halves — re-laying the head at a different
// width re-applies indent to its first line, and the tail by definition
// is not the start of a paragraph.
//
// Safe to call concurrently with other read methods on the receiver.
//
// Re-laying the returned head/tail at a different maxWidth than the
// split is supported but may change line counts (the words are real
// — not pre-wrapped). Callers that want stable line counts should
// re-lay at the same width they passed to SplitAfterLine.
func (p *Paragraph) SplitAfterLine(n int, maxWidth float64) (head, tail *Paragraph) {
	lines := p.Layout(maxWidth)
	if n <= 0 {
		// Tail is the entire paragraph — preserves both spacings,
		// since cloneWithWords already propagates spaceAfter and we
		// restore spaceBefore explicitly here.
		t := p.cloneWithWords(flattenLineWords(lines))
		t.spaceBefore = p.spaceBefore
		return nil, t
	}
	if n >= len(lines) {
		// Head is the entire paragraph — preserve both spacings.
		h := p.cloneWithWords(flattenLineWords(lines))
		h.spaceBefore = p.spaceBefore
		return h, nil
	}

	headWords := flattenLineWords(lines[:n])
	tailWords := flattenLineWords(lines[n:])

	head = p.cloneWithWords(headWords)
	tail = p.cloneWithWords(tailWords)

	// cloneWithWords does not propagate spaceBefore (it's used for
	// page-split tails where spaceBefore is unwanted). For SplitAfterLine
	// the head IS the same paragraph just with fewer lines, so propagate
	// it explicitly. SpaceAfter belongs to tail; SpaceBefore belongs to
	// head — never both.
	head.spaceBefore = p.spaceBefore
	head.spaceAfter = 0
	tail.spaceBefore = 0
	return head, tail
}

// flattenLineWords concatenates words across lines, marking the first
// word of every line after the first with LineBreak=true so cloneWithWords
// reconstructs explicit \n separators. This preserves the wrap exactly
// as Layout produced it, even after a round-trip through cloneWithWords.
//
// Word fields (including pointer fields like *Color and *TextShadow)
// are copied by value here; the cloned paragraph shares those pointers
// with the source. Standard Go semantics — caller mutation of the
// shared color/shadow would affect both halves.
func flattenLineWords(lines []Line) []Word {
	total := 0
	for _, line := range lines {
		total += len(line.Words)
	}
	words := make([]Word, 0, total)
	for i, line := range lines {
		for j, w := range line.Words {
			if i > 0 && j == 0 {
				w.LineBreak = true
			}
			words = append(words, w)
		}
	}
	return words
}

// splitWords splits text into words, preserving \n as a lineBreakMarker
// sentinel that forces a line break during word-wrapping. CJK character
// splitting is NOT done here; it happens in breakCJKWords after word
// measurement, so that SpaceAfter values are correctly set to 0 for
// characters within a CJK run (CJK text has no inter-word spacing).
func splitWords(text string) []string {
	// Normalize \r\n and bare \r to \n, then split on newlines.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	var result []string
	for i, line := range lines {
		if i > 0 {
			result = append(result, lineBreakMarker)
		}
		result = append(result, strings.Fields(line)...)
	}
	return result
}

// lineBreakMarker is a sentinel value in the word list that signals a
// forced line break from a \n character in the source text.
const lineBreakMarker = "\x00linebreak"

// PlanLayout implements Element. It computes word-wrapped lines that fit
// within the available area. If the paragraph doesn't fit entirely, it
// returns LayoutPartial with the remaining words as the Overflow element.
func (p *Paragraph) PlanLayout(area LayoutArea) LayoutPlan {
	measured, maxFontSize := p.measureWords(area.Width)

	if len(measured) == 0 {
		consumed := p.spaceBefore + p.spaceAfter
		return LayoutPlan{Status: LayoutFull, Consumed: consumed}
	}

	lineHeight := maxFontSize * p.leading
	wordLines := p.wrapWords(measured, area.Width)

	// Bidi reordering: resolve the paragraph's base direction and
	// reorder each line's words into visual order for correct
	// left-to-right rendering of RTL and mixed-direction text.
	resolvedDir := DirectionLTR
	for i, wl := range wordLines {
		reordered, dir := resolveLineBidi(wl, p.direction)
		wordLines[i] = reordered
		if i == 0 {
			resolvedDir = dir
		}
	}
	// When direction is explicitly set (CSS direction:rtl or HTML
	// dir="rtl"), use it for alignment even if bidi resolved LTR.
	if p.direction != DirectionAuto {
		resolvedDir = p.direction
	}

	// Compute heights and split at available height.
	type lineInfo struct {
		words       []Word
		width       float64
		isLast      bool
		spaceBefore float64
		spaceAfter  float64
	}

	infos := make([]lineInfo, len(wordLines))
	for i, wl := range wordLines {
		w := 0.0
		for _, word := range wl {
			w += word.Width
			if i > 0 || len(wl) > 1 {
				w += word.SpaceAfter
			}
		}
		// Recompute width properly.
		w = lineWidth(wl)
		infos[i] = lineInfo{
			words:  wl,
			width:  w,
			isLast: i == len(wordLines)-1,
		}
	}
	if len(infos) > 0 {
		infos[0].spaceBefore = p.spaceBefore
		infos[len(infos)-1].spaceAfter = p.spaceAfter
	}

	// Compute per-line height: max of text line height and any inline-block heights.
	lineHeights := make([]float64, len(infos))
	for i, info := range infos {
		lh := lineHeight
		for _, w := range info.words {
			if w.InlineBlock != nil && w.InlineHeight > lh {
				lh = w.InlineHeight
			}
		}
		lineHeights[i] = lh
	}

	// Determine how many lines fit.
	// area.Height <= 0 means no space left — nothing fits.
	if area.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	totalH := 0.0
	splitIdx := len(infos)
	for i, info := range infos {
		h := lineHeights[i]
		if i == 0 {
			h += info.spaceBefore
		}
		if i == len(infos)-1 {
			h += info.spaceAfter
		}
		if totalH+h > area.Height && i > 0 {
			splitIdx = i
			break
		}
		totalH += h
	}

	// Build placed blocks for fitted lines.
	blocks := make([]PlacedBlock, 0, splitIdx)
	curY := 0.0

	for i := range splitIdx {
		info := infos[i]
		if i == 0 {
			curY += info.spaceBefore
		}

		// bgX is the line box's left edge (first-line indent applied,
		// no alignment offset). The text rendering origin adds the
		// alignment offset on top of bgX. Separating these keeps the
		// paragraph background pinned to the content-box width
		// instead of riding along with center/right-aligned text —
		// otherwise the background leaks past the line's right edge
		// by the alignment offset (CSS Backgrounds & Borders L3 §2.1).
		bgX := 0.0
		lineMaxW := area.Width
		if i == 0 && p.firstIndent != 0 {
			bgX += p.firstIndent
			lineMaxW -= p.firstIndent
		}
		effectiveAlign := p.align
		// RTL paragraphs default to right-aligned unless the caller
		// explicitly called SetAlign. This matches CSS behavior where
		// `direction: rtl` flips the default text-align to "right".
		if resolvedDir == DirectionRTL && !p.alignSet {
			effectiveAlign = AlignRight
		}
		if p.textAlignLastSet && (info.isLast || i == splitIdx-1) {
			effectiveAlign = p.textAlignLast
		}
		alignOffset := 0.0
		switch effectiveAlign {
		case AlignCenter:
			alignOffset = (lineMaxW - info.width) / 2
		case AlignRight:
			alignOffset = lineMaxW - info.width
		}
		x := bgX + alignOffset

		// Capture for closure.
		capturedWords := slices.Clone(info.words)
		capturedIsLast := info.isLast || i == splitIdx-1
		capturedWidth := lineMaxW
		capturedAlign := effectiveAlign
		capturedBg := p.background
		capturedLineH := lineHeights[i]
		capturedAlignOffset := alignOffset

		// Build child blocks for inline-block words. Positions are
		// line-relative (starting at 0); the parent PlacedBlock's X
		// already carries the alignment offset, and the renderer adds
		// parent X to child X when drawing (render_plans.go).
		var inlineChildren []PlacedBlock
		inlineX := 0.0
		for _, w := range info.words {
			if w.InlineBlock != nil {
				ibPlan := w.InlineBlock.PlanLayout(LayoutArea{
					Width: w.InlineWidth, Height: w.InlineHeight,
				})
				for _, ib := range ibPlan.Blocks {
					ib.X += inlineX
					ib.Y += capturedLineH - w.InlineHeight
					inlineChildren = append(inlineChildren, ib)
				}
			}
			inlineX += w.Width + w.SpaceAfter
		}

		block := PlacedBlock{
			X:      x,
			Y:      curY,
			Width:  info.width,
			Height: capturedLineH,
			Tag:    "P",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				if capturedBg != nil {
					drawBackground(ctx, *capturedBg, absX-capturedAlignOffset, absTopY, capturedWidth, capturedLineH)
				}
				baseline := computeBaseline(capturedWords, capturedLineH)
				drawTextLine(ctx, capturedWords, absX, absTopY-baseline, capturedWidth, capturedAlign, capturedIsLast)
			},
			Children: inlineChildren,
		}
		// Compute precise link annotations for every distinct link URI
		// in this line. Each linked span gets its own annotation rect.
		block.Links = linkSpans(info.words)
		// Attach string-set values on the first block.
		if i == 0 && len(p.stringSets) > 0 {
			block.StringSets = p.stringSets
		}
		blocks = append(blocks, block)
		curY += capturedLineH
		if i == splitIdx-1 {
			curY += info.spaceAfter
		}
	}

	if splitIdx >= len(infos) {
		return LayoutPlan{
			Status:   LayoutFull,
			Consumed: curY,
			Blocks:   blocks,
		}
	}

	// Build overflow paragraph from remaining words.
	var overflowWords []Word
	for i := splitIdx; i < len(infos); i++ {
		overflowWords = append(overflowWords, infos[i].words...)
	}
	overflow := p.cloneWithWords(overflowWords)
	overflow.spaceBefore = 0 // no spaceBefore on continuation

	return LayoutPlan{
		Status:   LayoutPartial,
		Consumed: curY,
		Blocks:   blocks,
		Overflow: overflow,
	}
}

// measureWords flattens all runs into measured words.
//
// When a run's text starts with punctuation and no leading whitespace
// (e.g. ". Then" after a bold run), the leading punctuation characters
// are appended to the last word of the previous run. This produces
// "here." as one word instead of "here" + "." as two, matching standard
// typographic behavior at style boundaries.
func (p *Paragraph) measureWords(maxWidth float64) ([]Word, float64) {
	var measured []Word
	var maxFontSize float64

	nextLineBreakFromBr := false
	for i, run := range p.runs {
		if run.InlineElement != nil {
			glueAdjacentRuns(measured, p.runs, i)
			measured = append(measured, measureInlineElement(run, maxWidth, measured, p.runs, i))
			continue
		}
		if run.IsLineBreak {
			nextLineBreakFromBr = true
			continue
		}

		// Zero the previous word's SpaceAfter when this run abuts it
		// with no whitespace (e.g. "C" + "<sub>8</sub>").
		glueAdjacentRuns(measured, p.runs, i)

		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		text := run.Text

		// If the run starts with punctuation (no leading space) and we
		// already have words, append the punctuation to the previous word.
		// The punctuation renders in the previous word's font, which is
		// visually correct — "here." should look like one word.
		// Skip merging when the previous word has different styling (font size
		// or baseline shift), as the punctuation should use the current run's
		// styling, not the previous word's. This prevents "th" (superscript)
		// from absorbing "!" (normal) in "7<sup>th</sup>! works".
		if len(measured) > 0 && len(text) > 0 && !isSpace(rune(text[0])) {
			prev := &measured[len(measured)-1]
			sameStyle := prev.Font == run.Font && prev.Embedded == run.Embedded &&
				prev.FontSize == run.FontSize && prev.BaselineShift == run.BaselineShift
			if sameStyle {
				punct, rest := splitLeadingPunct(text)
				if punct != "" {
					prev.Text += punct
					prevMeasurer := wordMeasurer(*prev)
					prevMeasureText := prev.Text
					if hasCounterPlaceholder(prevMeasureText) {
						prevMeasureText = expandCountersForMeasure(prevMeasureText)
					}
					prev.Width = prevMeasurer.MeasureString(prevMeasureText, prev.FontSize)
					if prev.LetterSpacing != 0 {
						prev.Width += prev.LetterSpacing * float64(len([]rune(prevMeasureText))-1)
					}
					text = rest
				}
			}
		}

		words := splitWords(text)
		nextLineBreak := nextLineBreakFromBr
		nextLineBreakFromBr = false
		for _, w := range words {
			if w == lineBreakMarker {
				if nextLineBreak {
					measured = append(measured, Word{
						Font:      run.Font,
						Embedded:  run.Embedded,
						FontSize:  run.FontSize,
						LineBreak: true,
					})
				}
				nextLineBreak = true
				continue
			}
			// Build the word from the unshaped text first so the bidi
			// split sees the original codepoints; each piece is then
			// shaped and its pre-shape text recorded for /ActualText.
			word := Word{
				Text:            w,
				Font:            run.Font,
				Embedded:        run.Embedded,
				FontSize:        run.FontSize,
				Color:           run.Color,
				Decoration:      run.Decoration,
				DecorationColor: run.DecorationColor,
				DecorationStyle: run.DecorationStyle,
				SpaceAfter:      spaceW,
				LetterSpacing:   run.LetterSpacing,
				WordSpacing:     run.WordSpacing,
				BaselineShift:   run.BaselineShift,
				LinkURI:         run.LinkURI,
				TextShadow:      run.TextShadow,
				BackgroundColor: run.BackgroundColor,
				LineBreak:       nextLineBreak,
			}
			if subs := splitMixedBidiWord(word); subs != nil {
				for si, sub := range subs {
					shapeAndMeasureWord(&sub, run, measurer)
					if si == 0 {
						sub.LineBreak = nextLineBreak
					}
					measured = append(measured, sub)
				}
			} else {
				shapeAndMeasureWord(&word, run, measurer)
				measured = append(measured, word)
			}
			nextLineBreak = false
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
		}
	}

	if p.wordBreak != "keep-all" {
		measured = breakCJKWords(measured)
	}
	// Mirror the dispatch in measureWords (above): word-break:break-all
	// requires every word be split at character boundaries, not just the
	// over-long ones. Calling only breakLongWords here let break-all
	// silently degrade to default break behavior whenever re-measurement
	// was performed by this code path.
	if p.wordBreak == "break-all" {
		measured = breakAllWords(measured, maxWidth)
	} else {
		measured = breakLongWords(measured, maxWidth)
	}
	return measured, maxFontSize
}

// splitLeadingPunct splits a string into a leading punctuation prefix
// and the remainder. Returns ("", s) if s does not start with punctuation.
func splitLeadingPunct(s string) (punct, rest string) {
	i := 0
	for _, r := range s {
		if isPunctuation(r) {
			i += len(string(r))
		} else {
			break
		}
	}
	if i == 0 {
		return "", s
	}
	return s[:i], s[i:]
}

// isPunctuation reports whether r is a punctuation character that should
// attach to the preceding word rather than stand alone.
func isPunctuation(r rune) bool {
	switch r {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}', '"', '\'',
		'\u2019', '\u201D': // right single/double quotes
		return true
	}
	return false
}

// isSpace reports whether r is a whitespace character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// wordMeasurer returns a TextMeasurer for the given word's font.
func wordMeasurer(w Word) font.TextMeasurer {
	if w.Embedded != nil {
		return w.Embedded
	}
	return w.Font
}

// wrapWords performs greedy word-wrapping, returning groups of words per line.
func (p *Paragraph) wrapWords(words []Word, maxWidth float64) [][]Word {
	if len(words) == 0 {
		return nil
	}
	var lines [][]Word
	lineStart := 0
	// First line has reduced width for text-indent.
	effectiveWidth := maxWidth
	if p.firstIndent != 0 {
		effectiveWidth = maxWidth - p.firstIndent
	}
	lw := words[0].Width

	for i := 1; i < len(words); i++ {
		// Forced line break from \n in source text.
		if words[i].LineBreak {
			lines = append(lines, slices.Clone(words[lineStart:i]))
			lineStart = i
			lw = words[i].Width
			effectiveWidth = maxWidth
			continue
		}
		spaceW := words[i-1].SpaceAfter
		candidate := lw + spaceW + words[i].Width
		if candidate > effectiveWidth && lineStart < i {
			lines = append(lines, slices.Clone(words[lineStart:i]))
			lineStart = i
			lw = words[i].Width
			effectiveWidth = maxWidth
		} else {
			lw = candidate
		}
	}
	lines = append(lines, slices.Clone(words[lineStart:]))
	return lines
}

// linkSpans computes a LinkArea for every contiguous run of words that
// share the same non-empty LinkURI. Each span's X and W are relative to
// the line's starting x position. This supports multiple distinct links
// on the same line (e.g. "Visit GitHub or GitLab").
func linkSpans(words []Word) []LinkArea {
	var spans []LinkArea
	cx := 0.0
	i := 0
	for i < len(words) {
		uri := words[i].LinkURI
		if uri == "" {
			if i < len(words)-1 {
				cx += words[i].Width + words[i].SpaceAfter
			}
			i++
			continue
		}
		// Start of a linked span.
		startX := cx
		endX := cx + words[i].Width
		j := i + 1
		for j < len(words) && words[j].LinkURI == uri {
			// Extend through the space before this word.
			endX = cx
			for k := i; k < j; k++ {
				endX += words[k].Width + words[k].SpaceAfter
			}
			endX += words[j].Width
			j++
		}
		spans = append(spans, LinkArea{
			URI: uri,
			X:   startX,
			W:   endX - startX,
		})
		// Advance cx past all words in this span.
		for i < j {
			if i < len(words)-1 {
				cx += words[i].Width + words[i].SpaceAfter
			}
			i++
		}
	}
	return spans
}

// lineWidth computes the content width of a word slice.
func lineWidth(words []Word) float64 {
	if len(words) == 0 {
		return 0
	}
	w := 0.0
	for i, word := range words {
		w += word.Width
		if i < len(words)-1 {
			w += word.SpaceAfter
		}
	}
	return w
}

// truncateWithEllipsis truncates a line's words so the total width plus "..."
// fits within maxWidth. The ellipsis is appended to the last visible word's text.
func truncateWithEllipsis(line Line, maxWidth float64) Line {
	if len(line.Words) == 0 {
		return line
	}

	// Measure "..." using the last word's font metrics.
	lastWord := line.Words[len(line.Words)-1]
	var ellipsisW float64
	if lastWord.Embedded != nil {
		ellipsisW = lastWord.Embedded.MeasureString("...", lastWord.FontSize)
	} else if lastWord.Font != nil {
		ellipsisW = lastWord.Font.MeasureString("...", lastWord.FontSize)
	} else {
		// Approximate: 3 dots at ~0.3em each.
		ellipsisW = lastWord.FontSize * 0.9
	}

	// Find how many words fit with room for "...".
	w := 0.0
	cutIdx := len(line.Words)
	for i, word := range line.Words {
		nextW := w + word.Width
		if i > 0 {
			nextW += line.Words[i-1].SpaceAfter
		}
		if nextW+ellipsisW > maxWidth && i > 0 {
			cutIdx = i
			break
		}
		w = nextW
	}

	if cutIdx >= len(line.Words) {
		// Everything fits — just append ellipsis to the last word.
		words := slices.Clone(line.Words)
		last := &words[len(words)-1]
		last.Text += "..."
		last.Width += ellipsisW
		line.Words = words
		line.Width = lineWidth(words)
		return line
	}

	words := slices.Clone(line.Words[:cutIdx])
	if len(words) > 0 {
		last := &words[len(words)-1]
		last.Text += "..."
		last.Width += ellipsisW
	}
	line.Words = words
	line.Width = lineWidth(words)
	return line
}

// wordToRun converts a Word back to a TextRun, preserving all styling fields.
//
// For shaped scripts (Arabic, Indic), Word.Text holds post-shape codepoints
// (Presentation Forms-B for Arabic, post-reorder logical text for Indic)
// while Word.OriginalText holds the pre-shape input. The reconstructed
// TextRun must use the pre-shape text so that re-laying the cloned
// paragraph re-runs the shaper from scratch and re-populates OriginalText
// for /ActualText marked-content recovery (ISO 32000-2 §14.9.4). Without
// this, accessibility and copy/paste fidelity silently regress on any
// paragraph that crosses a page-split boundary.
//
// Word.InlineBlock holds the inline element (image, SVG, etc.) and round-
// trips to TextRun.InlineElement. Word.InlineWidth and Word.InlineHeight
// are cached measurements that re-compute during Layout; they do not need
// to round-trip. Word.GIDs are shaper output; TextRun has no GIDs field,
// so the shaper regenerates them on re-layout from the pre-shape input
// in TextRun.Text.
//
// Note: hyphenated words produced by hyphenateWord do not enter this
// path — hyphenation runs only inside Layout(), not in the wrapWords
// path used by PlanLayout. If hyphenation is ever wired into PlanLayout,
// the trailing "-" word and its rest pair will need a join-without-space
// fix in cloneWithWords (see issue tracking breakLongWords field loss).
func wordToRun(w Word) TextRun {
	text := w.Text
	if w.OriginalText != "" {
		text = w.OriginalText
	}
	return TextRun{
		Text:            text,
		Font:            w.Font,
		Embedded:        w.Embedded,
		FontSize:        w.FontSize,
		Color:           w.Color,
		Decoration:      w.Decoration,
		DecorationColor: w.DecorationColor,
		DecorationStyle: w.DecorationStyle,
		BaselineShift:   w.BaselineShift,
		LetterSpacing:   w.LetterSpacing,
		WordSpacing:     w.WordSpacing,
		LinkURI:         w.LinkURI,
		TextShadow:      w.TextShadow,
		BackgroundColor: w.BackgroundColor,
		InlineElement:   w.InlineBlock,
	}
}

// cloneWithWords creates a new Paragraph with the same style but different words.
// Used to create overflow paragraphs during splitting.
func (p *Paragraph) cloneWithWords(words []Word) *Paragraph {
	// Reconstruct runs from words. Group consecutive words with identical
	// styling into a single run. All Word-level styling fields must be
	// compared and preserved to avoid losing baseline shift, letter spacing,
	// links, highlights, etc. on page-split paragraphs.
	var runs []TextRun
	if len(words) > 0 {
		cur := wordToRun(words[0])
		// If the first word is a blank line (from \n\n), start text with
		// "\n" so splitWords produces a lineBreakMarker for the empty line.
		if words[0].Text == "" && words[0].LineBreak {
			cur.Text = "\n"
		}
		// Track the SpaceAfter of the most recently appended word so we
		// can decide whether to insert a separator before the next word
		// in the same run. The discriminator is SpaceAfter==0 vs >0,
		// not script — glueAdjacentRuns zeros SpaceAfter on Latin words
		// at run boundaries, breakCJKWords zeros it between CJK chars,
		// and breakLongWords does the same for character-broken words.
		// Re-measurement on the cloned paragraph regenerates the actual
		// space width for the destination font/size, so we only need
		// to faithfully encode "was there a space here" as a bool.
		prevSpaceAfter := words[0].SpaceAfter
		for _, w := range words[1:] {
			// Blank words (from consecutive \n\n) represent empty lines.
			// Serialize as "\n" so splitWords regenerates lineBreakMarkers.
			// Blank words have no visible text so style doesn't matter.
			//
			// Exclude inline elements: they also have Text="" but they
			// are NOT blank-line markers. SplitAtLine's flattenLineWords
			// sets LineBreak=true on the first word of every line after
			// the first, which can land on an inline element — without
			// this guard the inline would be silently dropped.
			if w.Text == "" && w.LineBreak && w.InlineBlock == nil {
				cur.Text += "\n"
				// A blank line resets the join state — whatever
				// follows starts a fresh visual line and should not
				// inherit the pre-blank-line word's SpaceAfter.
				prevSpaceAfter = 0
				continue
			}
			// Inline elements never merge with text or with another inline
			// element — each one needs its own TextRun so the renderer can
			// place the embedded element at the right point in the run list.
			isInline := w.InlineBlock != nil
			curInline := cur.InlineElement != nil
			sameRun := !isInline && !curInline &&
				w.Font == cur.Font && w.Embedded == cur.Embedded &&
				w.FontSize == cur.FontSize && w.Color == cur.Color &&
				w.Decoration == cur.Decoration && w.BaselineShift == cur.BaselineShift &&
				w.LetterSpacing == cur.LetterSpacing && w.WordSpacing == cur.WordSpacing &&
				w.LinkURI == cur.LinkURI && w.BackgroundColor == cur.BackgroundColor
			// A word with LineBreak=true had a forced \n before it.
			if w.LineBreak {
				if sameRun {
					cur.Text += "\n" + w.Text
				} else {
					// Style changes at line break: put \n at end of
					// current run, flush it, start new run for w.
					cur.Text += "\n"
					runs = append(runs, cur)
					cur = wordToRun(w)
				}
				prevSpaceAfter = w.SpaceAfter
				continue
			}
			if sameRun {
				if prevSpaceAfter == 0 {
					cur.Text += w.Text
				} else {
					cur.Text += " " + w.Text
				}
			} else {
				runs = append(runs, cur)
				cur = wordToRun(w)
			}
			prevSpaceAfter = w.SpaceAfter
		}
		runs = append(runs, cur)
	}

	return &Paragraph{
		runs:             runs,
		leading:          p.leading,
		align:            p.align,
		alignSet:         p.alignSet,
		direction:        p.direction,
		spaceAfter:       p.spaceAfter,
		background:       p.background,
		wordBreak:        p.wordBreak,
		hyphens:          p.hyphens,
		textAlignLast:    p.textAlignLast,
		textAlignLastSet: p.textAlignLastSet,
		ellipsis:         p.ellipsis,
		// firstIndent is NOT propagated — it only applies to the first line
		orphans: p.orphans,
		widows:  p.widows,
	}
}
