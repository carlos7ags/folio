// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"slices"

	"github.com/carlos7ags/folio/font"
)

// LeadingNormal is the sentinel [Paragraph.SetLeading] value for CSS
// `line-height: normal`. Unlike a literal multiplier, it defers resolution
// to layout time, where it's derived from the actual font's vertical
// metrics (see [resolveLineHeight]) instead of a flat fontSize*1.2 — fonts
// that declare a larger line-gap get proportionally more leading, matching
// how browsers resolve `normal`.
const LeadingNormal = -1

// Paragraph is a block of text that word-wraps within the available width.
// It is composed of one or more TextRuns, each with its own font, size,
// and color. All runs flow together as a single word-wrapped unit.
type Paragraph struct {
	runs             []TextRun
	leading          float64 // line height multiplier (e.g. 1.2 means 120% of fontSize)
	noWrap           bool              // true for CSS white-space:nowrap — a word wider than maxWidth overflows instead of being character-broken
	align            Align
	alignSet         bool              // true if SetAlign was called explicitly
	direction        Direction         // base text direction for bidi layout
	spaceBefore      float64           // extra space before the paragraph (points)
	spaceAfter       float64           // extra space after the paragraph (points)
	background       *Color            // background fill color (nil = transparent)
	firstIndent      float64           // first-line indent (points, from CSS text-indent)
	orphans          int               // min lines at bottom of page before break (0 = disabled)
	widows           int               // min lines at top of page after break (0 = disabled)
	ellipsis         bool              // if true, truncate overflowing text with "..."
	wordBreak        string            // "normal" (default), "break-all", "keep-all"
	hyphens          string            // "none", "manual" (default), "auto" (automatic hyphenation)
	textAlignLast    Align             // alignment for the last line (0 = use default)
	textAlignLastSet bool              // true if textAlignLast was explicitly set
	stringSets       map[string]string // CSS string-set values to capture
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

// SetNoWrap sets CSS white-space:nowrap behavior. When true, a word wider
// than the available width overflows rather than being character-broken —
// matching how browsers treat an unbreakable nowrap run. Default is false.
func (p *Paragraph) SetNoWrap(noWrap bool) *Paragraph {
	p.noWrap = noWrap
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
	var dominantRun TextRun

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
			dominantRun = run
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
	// at character boundaries to fill lines maximally. white-space:nowrap
	// leaves an overlong word intact so it overflows instead of wrapping,
	// matching browser behavior.
	if p.wordBreak == "break-all" {
		measured = breakAllWords(measured, maxWidth)
	} else if !p.noWrap {
		measured = breakLongWords(measured, maxWidth)
	}

	lineHeight := resolveLineHeight(p.leading, maxFontSize, dominantRun)

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

// lineBreakMarker is a sentinel value in the word list that signals a
// forced line break from a \n character in the source text.
const lineBreakMarker = "\x00linebreak"
