// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"unicode/utf8"

	"github.com/carlos7ags/folio/font"
)

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
	if run.LetterSpacing != 0 {
		if n := utf8.RuneCountInString(measureText); n > 1 {
			w.Width += run.LetterSpacing * float64(n-1)
		}
	}
	if w.Text != origText {
		w.OriginalText = origText
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
			if sub.LetterSpacing != 0 {
				if n := utf8.RuneCountInString(tok); n > 1 {
					sub.Width += sub.LetterSpacing * float64(n-1)
				}
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
	if maxWidth < 0 {
		maxWidth = 0
	}
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
		if len(runes) == 0 {
			// Zero-width/empty word (e.g. a blank LineBreak marker) can
			// reach this path when maxWidth is negative, making every
			// word "overflow". Nothing to split; pass it through.
			result = append(result, w)
			continue
		}

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
		byteStart := 0
		for start < len(runes) {
			var chunkSource, chunkText string
			var chunkGIDs []uint16
			var chunkWidth float64
			var runeLen int
			if !shaped {
				if w.LetterSpacing == 0 {
					// Unshaped words measure with MeasureString, which is prefix-
					// summable (grapheme boundaries are prefix-stable) and carries no
					// kerning across a chunk boundary. One walk from the chunk start
					// finds the first-overflow break point and its width bit-
					// identically, and stops there — so a token that splits into a few
					// large chunks costs O(token) total rather than re-measuring every
					// candidate prefix. The chunk is a zero-copy slice of the source.
					byteLen, rl, cw := wordPrefixFit(&w, sourceText[byteStart:], maxWidth)
					runeLen = rl
					chunkSource = sourceText[byteStart : byteStart+byteLen]
					chunkText = chunkSource
					chunkWidth = cw
					byteStart += byteLen
				} else {
					// Letter-spacing adds (n-1) gaps to an n-rune chunk (matching
					// paragraph_measure.go's intrinsic-width formula), so the fit
					// decision can't reuse the plain-prefix-sum fast path: walk the
					// cumulative rune-boundary widths and stop at the first prefix
					// whose spaced width would overflow. Always take at least one
					// rune so the loop makes progress.
					// widths[n] is the (unspaced) width of the first n runes
					// (widths[0] == 0 for zero runes) — see MeasureStringPrefixes.
					widths := wordPrefixWidths(&w, sourceText[byteStart:])
					rl := 1
					cw := 0.0
					maxN := len(widths) - 1
					if maxN >= 1 {
						cw = widths[1]
						for n := 1; n <= maxN; n++ {
							spaced := widths[n] + w.LetterSpacing*float64(n-1)
							if spaced > maxWidth && n > 1 {
								break
							}
							rl = n
							cw = spaced
						}
					}
					runeLen = rl
					chunkSource = string(runes[start : start+runeLen])
					chunkText = chunkSource
					chunkWidth = cw
					byteStart += len(chunkSource)
				}
			} else {
				// Shaped words re-shape per candidate: Arabic/Indic widths are
				// context-sensitive, not prefix-summable, so no cumulative
				// shortcut applies.
				end := start + 1
				for end < len(runes) {
					_, _, width, _ := shapeChunk(string(runes[start : end+1]))
					candidateRunes := end + 1 - start
					spaced := width + w.LetterSpacing*float64(candidateRunes-1)
					if spaced > maxWidth {
						break
					}
					end++
				}
				runeLen = end - start
				chunkSource = string(runes[start:end])
				chunkText, chunkGIDs, chunkWidth, _ = shapeChunk(chunkSource)
				if w.LetterSpacing != 0 && runeLen > 1 {
					chunkWidth += w.LetterSpacing * float64(runeLen-1)
				}
			}
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
			start += runeLen
		}
	}
	return result
}

// wordPrefixFit finds the first-overflow break for the unshaped chunk starting
// at s, using the word's measurer. The measurer probe in breakLongWords
// guarantees a Font or Embedded is set before this is reached.
func wordPrefixFit(w *Word, s string, maxWidth float64) (byteLen, runeLen int, width float64) {
	if w.Embedded != nil {
		return w.Embedded.PrefixFit(s, w.FontSize, maxWidth)
	}
	return w.Font.PrefixFit(s, w.FontSize, maxWidth)
}

// wordPrefixWidths returns cumulative rune-boundary widths of s for the
// word's measurer, or nil when the word has no measurer.
func wordPrefixWidths(w *Word, s string) []float64 {
	if w.Embedded != nil {
		return w.Embedded.MeasureStringPrefixes(s, w.FontSize)
	}
	if w.Font != nil {
		return w.Font.MeasureStringPrefixes(s, w.FontSize)
	}
	return nil
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

	// Cumulative rune-boundary widths of the whole word, computed once.
	// Bit-identical to measure(string(runes[:i])) for every i (grapheme
	// boundaries are prefix-stable), so both candidate loops below become
	// O(1) array lookups instead of an O(prefix) re-measure per candidate.
	prefixes := wordPrefixWidths(&w, w.Text)

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
			pw := prefixes[bp] + hyphenW
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
			pw := prefixes[i] + hyphenW
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
