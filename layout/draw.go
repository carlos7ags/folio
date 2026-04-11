// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"unicode/utf8"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/unicode/grapheme"
)

// setFillColor emits the correct fill color operator based on color space.
func setFillColor(stream *content.Stream, c Color) {
	if c.Space == ColorSpaceCMYK {
		stream.SetFillColorCMYK(c.C, c.M, c.Y, c.K)
	} else {
		stream.SetFillColorRGB(c.R, c.G, c.B)
	}
}

// setStrokeColor emits the correct stroke color operator based on color space.
func setStrokeColor(stream *content.Stream, c Color) {
	if c.Space == ColorSpaceCMYK {
		stream.SetStrokeColorCMYK(c.C, c.M, c.Y, c.K)
	} else {
		stream.SetStrokeColorRGB(c.R, c.G, c.B)
	}
}

// drawTextLine emits PDF operators for a line of text words at the given
// baseline position (x, y). This is the core text drawing function used
// by Paragraph, Heading, TabbedLine, and List Draw closures.
func drawTextLine(ctx DrawContext, words []Word, x, baselineY, maxWidth float64, align Align, isLast bool) {
	if len(words) == 0 {
		return
	}

	// For justified text, compute extra space between words. When the line
	// contains Arabic words, prefer kashida (tatweel) elongation over
	// inter-word stretching: insert U+0640 into Arabic words to consume
	// the slack above the natural inter-word gap. Whatever slack remains
	// after kashida insertion (no Arabic on the line, no legal sites,
	// or partial fill) falls through to whitespace stretching.
	extraSpace := 0.0
	if align == AlignJustify && !isLast && len(words) > 1 {
		totalWordWidth := 0.0
		for _, w := range words {
			totalWordWidth += w.Width
		}
		// Slack = space available beyond the natural inter-word gaps.
		naturalGaps := 0.0
		for i := 0; i < len(words)-1; i++ {
			naturalGaps += words[i].SpaceAfter
		}
		kashidaSlack := maxWidth - totalWordWidth - naturalGaps
		if kashidaSlack > 0 {
			consumed := applyKashidaJustification(words, kashidaSlack)
			totalWordWidth += consumed
		}
		gaps := float64(len(words) - 1)
		extraSpace = (maxWidth - totalWordWidth) / gaps
	}

	// First pass: draw highlight backgrounds behind words that have BackgroundColor.
	// This must happen before text rendering so the background is behind the text.
	{
		bgX := x
		for i, word := range words {
			if word.InlineBlock != nil {
				bgX += word.InlineWidth + word.SpaceAfter
				continue
			}

			if word.BackgroundColor != nil {
				// Compute the highlight rectangle covering the word using
				// actual font metrics (ascent/descent from PDF spec Appendix D).
				var ascent, descent float64
				if word.Font != nil {
					ascent = word.Font.Ascent(word.FontSize)
					descent = word.Font.Descent(word.FontSize)
				} else if word.Embedded != nil {
					face := word.Embedded.Face()
					upem := float64(face.UnitsPerEm())
					ascent = float64(face.Ascent()) / upem * word.FontSize
					d := face.Descent() // negative
					if d < 0 {
						d = -d
					}
					descent = float64(d) / upem * word.FontSize
				} else {
					ascent = word.FontSize * 0.75
					descent = word.FontSize * 0.25
				}
				rectH := ascent + descent
				rectY := baselineY - descent // bottom of rect in PDF coordinates

				// Extend through trailing space when the next word has the
				// same background color (produces continuous highlight like browsers).
				highlightW := word.Width
				if i < len(words)-1 && words[i+1].BackgroundColor != nil &&
					*words[i+1].BackgroundColor == *word.BackgroundColor {
					if align == AlignJustify && !isLast {
						highlightW += extraSpace
					} else {
						highlightW += word.SpaceAfter
					}
				}

				ctx.Stream.SaveState()
				setFillColor(ctx.Stream, *word.BackgroundColor)
				ctx.Stream.Rectangle(bgX, rectY, highlightW, rectH)
				ctx.Stream.Fill()
				ctx.Stream.RestoreState()
			}

			var advance float64
			if i < len(words)-1 {
				if align == AlignJustify && !isLast {
					advance = word.Width + extraSpace
				} else {
					advance = word.Width + word.SpaceAfter
				}
			}
			bgX += advance
		}
	}

	curColor := Color{R: -1, G: -1, B: -1}
	curX := x
	for i, word := range words {
		// Inline-block words: skip text rendering (rendered as child PlacedBlocks).
		if word.InlineBlock != nil {
			if i < len(words)-1 {
				curX += word.InlineWidth + word.SpaceAfter
			}
			continue
		}

		if word.Color != curColor {
			setFillColor(ctx.Stream, word.Color)
			curColor = word.Color
		}

		wordY := baselineY + word.BaselineShift

		// Draw text-shadow before the main text (shadow renders behind).
		if word.TextShadow != nil {
			drawTextShadow(ctx, word, curX, wordY)
			// Restore fill color after shadow drew with its own color.
			setFillColor(ctx.Stream, word.Color)
			curColor = word.Color
		}

		// When a shaper substituted glyph-form codepoints (currently
		// Arabic Presentation Forms-B), wrap the text-showing operators
		// in an ISO 32000-2 §14.9.4 /Span /ActualText marked-content
		// sequence so that copy/paste and accessibility tools recover
		// the original Unicode codepoints. The opt-out lives on the
		// renderer (default on) and travels through DrawContext.
		emitActualText := word.OriginalText != "" && ctx.ActualText
		if emitActualText {
			ctx.Stream.BeginMarkedContentActualText(word.OriginalText)
		}

		ctx.Stream.BeginText()
		resName := registerFont(ctx.Page, word)
		ctx.Stream.SetFont(resName, word.FontSize)
		if word.LetterSpacing != 0 {
			ctx.Stream.SetCharSpacing(word.LetterSpacing)
		}
		ctx.Stream.MoveText(curX, wordY)

		if word.Embedded != nil {
			drawWordEmbedded(ctx.Stream, word)
		} else {
			drawWordStandard(ctx.Stream, word)
		}

		if word.LetterSpacing != 0 {
			ctx.Stream.SetCharSpacing(0)
		}
		ctx.Stream.EndText()

		if emitActualText {
			ctx.Stream.EndMarkedContent()
		}

		// Compute the advance to the next word (used for spacing and underline extension).
		var advance float64
		if i < len(words)-1 {
			if align == AlignJustify && !isLast {
				advance = word.Width + extraSpace
			} else {
				advance = word.Width + word.SpaceAfter
			}
		}

		if word.Decoration != DecorationNone {
			decoWord := word
			// Extend the decoration through the trailing space when the next
			// word carries the same decoration and belongs to the same visual
			// phrase (same LinkURI and decoration color). This produces a
			// continuous underline for multi-word links while keeping a gap
			// between adjacent links with different URIs.
			if i < len(words)-1 {
				next := words[i+1]
				sameDecoration := next.Decoration&word.Decoration != 0
				sameLink := word.LinkURI == next.LinkURI
				sameColor := word.Color == next.Color
				if word.DecorationColor != nil && next.DecorationColor != nil {
					sameColor = *word.DecorationColor == *next.DecorationColor
				}
				if sameDecoration && sameLink && sameColor {
					decoWord.Width = advance
				}
			}
			drawDecorations(ctx.Stream, decoWord, curX, baselineY)
		}

		curX += advance
	}
}

// drawWordStandard emits a standard-font word with optional TJ kerning.
// Text is encoded to WinAnsiEncoding for standard PDF fonts.
func drawWordStandard(stream *content.Stream, word Word) {
	if word.Font == nil {
		return
	}
	runes := []rune(word.Text)
	if len(runes) < 2 {
		stream.ShowText(font.WinAnsiEncode(word.Text))
		return
	}

	var elements []content.TextArrayElement
	start := 0
	hasKerning := false

	for i := 1; i < len(runes); i++ {
		kern := word.Font.Kern(runes[i-1], runes[i])
		if kern != 0 {
			hasKerning = true
			elements = append(elements, content.TextArrayElement{Text: font.WinAnsiEncode(string(runes[start:i]))})
			elements = append(elements, content.TextArrayElement{Adjustment: -kern, IsAdjustment: true})
			start = i
		}
	}

	if !hasKerning {
		stream.ShowText(font.WinAnsiEncode(word.Text))
		return
	}

	if start < len(runes) {
		elements = append(elements, content.TextArrayElement{Text: font.WinAnsiEncode(string(runes[start:]))})
	}
	stream.ShowTextArray(elements)
}

// drawWordEmbedded emits an embedded-font word with optional TJ kerning.
// When the word contains any Extend / ZWJ combining marks and the font's
// GPOS table provides mark-to-base anchors, the emission path switches to
// cluster-at-a-time rendering so each mark can be Td-positioned on its
// base glyph's anchor (ISO 14496-22 §6.3 LookupType 4). Eligible words
// are Arabic with harakat, Hebrew with niqqud, and similar scripts.
// Latin and un-marked words take the original fast path.
func drawWordEmbedded(stream *content.Stream, word Word) {
	if word.Embedded == nil {
		return
	}
	// Shaper-produced glyph stream (currently Devanagari): emit the
	// GID stream directly through the Identity-H encoder. The TJ
	// kern-pair walk is skipped because complex-script shapers handle
	// positioning via GPOS and not via the kern table. This branch
	// must run before the mark-attachment check below because
	// Devanagari combining marks are classified as Extend in UAX #29,
	// so a shaped Devanagari word would otherwise re-enter the
	// mark path and be shaped a second time on the original text.
	if len(word.GIDs) > 0 {
		stream.ShowTextHex(word.Embedded.EncodeGIDs(word.GIDs, word.OriginalText))
		return
	}
	if markPositioningEligible(word) {
		drawWordEmbeddedWithMarks(stream, word)
		return
	}
	runes := []rune(word.Text)
	if len(runes) < 2 {
		stream.ShowTextHex(word.Embedded.EncodeString(word.Text))
		return
	}

	var elements []content.TextArrayElement
	start := 0
	hasKerning := false

	for i := 1; i < len(runes); i++ {
		kern := word.Embedded.Kern(runes[i-1], runes[i])
		if kern != 0 {
			hasKerning = true
			elements = append(elements, content.TextArrayElement{HexData: word.Embedded.EncodeString(string(runes[start:i]))})
			elements = append(elements, content.TextArrayElement{Adjustment: -kern, IsAdjustment: true})
			start = i
		}
	}

	if !hasKerning {
		stream.ShowTextHex(word.Embedded.EncodeString(word.Text))
		return
	}

	if start < len(runes) {
		elements = append(elements, content.TextArrayElement{HexData: word.Embedded.EncodeString(string(runes[start:]))})
	}
	stream.ShowTextArrayHex(elements)
}

// markPositioningEligible reports whether drawWordEmbedded should switch
// to the cluster-by-cluster mark-attachment path. Eligibility requires
// an embedded font whose Face exposes GPOS mark-to-base data, a word
// containing at least one Extend or ZWJ codepoint, and no letter
// spacing override (Tc complicates the text-matrix arithmetic).
func markPositioningEligible(word Word) bool {
	if word.Embedded == nil || word.LetterSpacing != 0 {
		return false
	}
	provider, ok := word.Embedded.Face().(font.GPOSProvider)
	if !ok {
		return false
	}
	gpos := provider.GPOS()
	if gpos == nil || len(gpos.Marks[font.GPOSMark]) == 0 || len(gpos.Bases[font.GPOSMark]) == 0 {
		return false
	}
	for _, r := range word.Text {
		p := grapheme.PropertyOf(r)
		if p == grapheme.PropExtend || p == grapheme.PropZWJ {
			return true
		}
	}
	return false
}

// drawWordEmbeddedWithMarks walks the word cluster-by-cluster (UAX #29
// §3.1.1 extended grapheme clusters) and emits each cluster's base
// glyph followed by GPOS-anchored marks. The text matrix ends at the
// same position as the fast path (one natural advance per cluster,
// plus inter-cluster kerning), so MeasureString and the post-emit
// matrix agree. Callers must be inside a BT...ET block with the text
// matrix already positioned at the word origin via MoveText.
//
// For each cluster the base rune contributes its full advance. Any
// SpacingMark runes (Indic vowel signs) are appended into the cluster's
// Tj hex string — they take their own advance, matching MeasureString.
// Extend and ZWJ runes look up a MarkOffset(base, mark, GPOSMark) on
// the font; on success the mark is drawn at the base anchor via a Td
// pair that bookends its Tj so the text matrix returns to the cluster's
// natural advance position. When MarkOffset returns ok=false the mark
// is emitted at the current text origin (zero advance from SpacingMark
// absence), which matches the no-GPOS fallback behaviour.
func drawWordEmbeddedWithMarks(stream *content.Stream, word Word) {
	ef := word.Embedded
	face := ef.Face()
	upem := face.UnitsPerEm()
	if upem == 0 {
		stream.ShowTextHex(ef.EncodeString(word.Text))
		return
	}
	provider, _ := face.(font.GPOSProvider)
	gpos := provider.GPOS()
	fontSize := word.FontSize
	scale := fontSize / float64(upem)

	text := word.Text
	breaks := grapheme.Breaks(text)
	var prevTail uint16
	havePrev := false

	for ci := 0; ci+1 < len(breaks); ci++ {
		cluster := text[breaks[ci]:breaks[ci+1]]
		if cluster == "" {
			continue
		}
		baseRune, baseSize := utf8.DecodeRuneInString(cluster)
		baseGID := face.GlyphIndex(baseRune)

		// Kerning between the tail of the previous cluster and this
		// cluster's base. Emit as a Td shift so TJ and mark-Td handling
		// do not have to be interleaved in a single TJ array.
		if havePrev {
			kernUnits := face.Kern(prevTail, baseGID)
			if kernUnits != 0 {
				stream.MoveText(float64(kernUnits)*scale, 0)
			}
		}

		// Collect the base plus any SpacingMarks into a single hex
		// string. SpacingMarks take their own advance and do not
		// participate in GPOS mark-to-base anchoring here.
		baseAndSpacing := cluster[:baseSize]
		clusterAdvance := float64(face.GlyphAdvance(baseGID)) * scale
		tail := baseGID
		type markRune struct {
			r    rune
			size int
		}
		var extendMarks []markRune
		for off := baseSize; off < len(cluster); {
			r, size := utf8.DecodeRuneInString(cluster[off:])
			p := grapheme.PropertyOf(r)
			switch p {
			case grapheme.PropExtend, grapheme.PropZWJ:
				extendMarks = append(extendMarks, markRune{r: r, size: size})
			case grapheme.PropSpacingMark:
				// Append to the Tj hex string so its advance is
				// consumed by the Tj operator itself, matching
				// MeasureString's cluster width contribution.
				baseAndSpacing += cluster[off : off+size]
				spGID := face.GlyphIndex(r)
				clusterAdvance += float64(face.GlyphAdvance(spGID)) * scale
				tail = spGID
			}
			off += size
		}

		stream.ShowTextHex(ef.EncodeString(baseAndSpacing))

		// Emit each Extend/ZWJ mark bracketed by Td moves that shift
		// the text matrix from the cluster-end position back to the
		// base origin plus the GPOS offset, and then back to the
		// cluster-end position for the next mark or next cluster.
		//
		// After Tj of the base (plus SpacingMarks), the text matrix is
		// at clusterEnd = (clusterAdvance, 0) relative to the cluster
		// start. A mark's origin must sit at (markDx, markDy) relative
		// to the base's origin (the cluster start), so the first Td
		// moves by (markDx - clusterAdvance, markDy) and the closing
		// Td moves by (clusterAdvance - markDx, -markDy). Marks have
		// zero advance, so Tj of the mark does not shift the matrix.
		for _, m := range extendMarks {
			markGID := face.GlyphIndex(m.r)
			dx, dy, ok := gpos.MarkOffset(baseGID, markGID, font.GPOSMark)
			if ok {
				dxPts := float64(dx) * scale
				dyPts := float64(dy) * scale
				stream.MoveText(dxPts-clusterAdvance, dyPts)
				stream.ShowTextHex(ef.EncodeString(string(m.r)))
				stream.MoveText(clusterAdvance-dxPts, -dyPts)
			} else {
				stream.ShowTextHex(ef.EncodeString(string(m.r)))
			}
		}

		prevTail = tail
		havePrev = true
	}
}

// drawDecorations draws underline and/or strikethrough for a word.
// Supports DecorationColor (separate from text color) and DecorationStyle
// ("solid", "dashed", "dotted", "double", "wavy").
func drawDecorations(stream *content.Stream, word Word, x, baselineY float64) {
	stream.SaveState()

	// Use decoration color if set, otherwise fall back to text color.
	decoColor := word.Color
	if word.DecorationColor != nil {
		decoColor = *word.DecorationColor
	}
	setStrokeColor(stream, decoColor)

	lw := max(word.FontSize*0.05, 0.5)
	stream.SetLineWidth(lw)

	// Apply dash pattern based on decoration style.
	switch word.DecorationStyle {
	case "dashed":
		stream.SetDashPattern([]float64{lw * 3, lw * 2}, 0)
	case "dotted":
		stream.SetDashPattern([]float64{lw, lw * 2}, 0)
	}

	if word.Decoration&DecorationUnderline != 0 {
		// Underline position: slightly below baseline (~descent * 0.75).
		var uy float64
		if word.Font != nil {
			uy = baselineY - word.Font.Descent(word.FontSize)*0.75
		} else {
			uy = baselineY - word.FontSize*0.15
		}
		switch word.DecorationStyle {
		case "double":
			// Draw two lines separated by the line width.
			stream.MoveTo(x, uy)
			stream.LineTo(x+word.Width, uy)
			stream.Stroke()
			stream.MoveTo(x, uy-lw*2)
			stream.LineTo(x+word.Width, uy-lw*2)
			stream.Stroke()
		case "wavy":
			drawWavyLine(stream, x, uy, word.Width, lw)
		default:
			stream.MoveTo(x, uy)
			stream.LineTo(x+word.Width, uy)
			stream.Stroke()
		}
	}
	if word.Decoration&DecorationStrikethrough != 0 {
		// Strikethrough position: roughly at x-height (~ascent * 0.4).
		var sy float64
		if word.Font != nil {
			sy = baselineY + word.Font.Ascent(word.FontSize)*0.4
		} else {
			sy = baselineY + word.FontSize*0.3
		}
		switch word.DecorationStyle {
		case "double":
			stream.MoveTo(x, sy)
			stream.LineTo(x+word.Width, sy)
			stream.Stroke()
			stream.MoveTo(x, sy+lw*2)
			stream.LineTo(x+word.Width, sy+lw*2)
			stream.Stroke()
		case "wavy":
			drawWavyLine(stream, x, sy, word.Width, lw)
		default:
			stream.MoveTo(x, sy)
			stream.LineTo(x+word.Width, sy)
			stream.Stroke()
		}
	}

	// Reset dash pattern if needed.
	if word.DecorationStyle == "dashed" || word.DecorationStyle == "dotted" {
		stream.SetDashPattern(nil, 0)
	}

	stream.RestoreState()
}

// drawWavyLine approximates a wavy line using small zigzag segments.
func drawWavyLine(stream *content.Stream, x, y, width, amplitude float64) {
	if amplitude < 0.5 {
		amplitude = 0.5
	}
	step := amplitude * 4
	curX := x
	up := true
	stream.MoveTo(curX, y)
	for curX < x+width {
		nextX := curX + step
		if nextX > x+width {
			nextX = x + width
		}
		if up {
			stream.LineTo(nextX, y+amplitude)
		} else {
			stream.LineTo(nextX, y-amplitude)
		}
		up = !up
		curX = nextX
	}
	stream.Stroke()
}

// drawBackground draws a filled rectangle behind a line.
func drawBackground(ctx DrawContext, bg Color, x, topY, width, height float64) {
	ctx.Stream.SaveState()
	setFillColor(ctx.Stream, bg)
	ctx.Stream.Rectangle(x, topY-height, width, height)
	ctx.Stream.Fill()
	ctx.Stream.RestoreState()
}

// registerFont ensures a font is registered on the page, returns the resource name.
func registerFont(page *PageResult, word Word) string {
	if word.Font != nil {
		for _, f := range page.Fonts {
			if f.Standard != nil && f.Standard.Name() == word.Font.Name() {
				return f.Name
			}
		}
		name := fmt.Sprintf("F%d", len(page.Fonts)+1)
		page.Fonts = append(page.Fonts, FontEntry{Name: name, Standard: word.Font})
		return name
	}
	if word.Embedded != nil {
		for _, f := range page.Fonts {
			if f.Embedded == word.Embedded {
				return f.Name
			}
		}
		name := fmt.Sprintf("F%d", len(page.Fonts)+1)
		page.Fonts = append(page.Fonts, FontEntry{Name: name, Embedded: word.Embedded})
		return name
	}
	return "F1"
}

// registerFontStandard ensures a standard font is registered on the page.
func registerFontStandard(page *PageResult, f *font.Standard) string {
	for _, fe := range page.Fonts {
		if fe.Standard != nil && fe.Standard.Name() == f.Name() {
			return fe.Name
		}
	}
	name := fmt.Sprintf("F%d", len(page.Fonts)+1)
	page.Fonts = append(page.Fonts, FontEntry{Name: name, Standard: f})
	return name
}

// registerImage ensures an image is registered on the page, returns the resource name.
func registerImage(page *PageResult, img *folioimage.Image) string {
	for _, ie := range page.Images {
		if ie.Image == img {
			return ie.Name
		}
	}
	name := fmt.Sprintf("Im%d", len(page.Images)+1)
	page.Images = append(page.Images, ImageEntry{Name: name, Image: img})
	return name
}

// registerOpacity ensures an ExtGState with the given opacity is registered,
// returns the resource name (e.g. "GS1").
func registerOpacity(page *PageResult, opacity float64) string {
	for _, gs := range page.ExtGStates {
		if gs.Opacity == opacity {
			return gs.Name
		}
	}
	name := fmt.Sprintf("GS%d", len(page.ExtGStates)+1)
	page.ExtGStates = append(page.ExtGStates, ExtGStateEntry{Name: name, Opacity: opacity})
	return name
}

// drawTextShadow draws a text shadow behind a word by re-drawing the same
// text at an offset with the shadow color. For blur > 0, a semi-transparent
// duplicate is drawn at a slightly larger offset to approximate the blur.
func drawTextShadow(ctx DrawContext, word Word, x, y float64) {
	shadow := word.TextShadow
	if shadow == nil {
		return
	}
	ctx.Stream.SaveState()

	// Shadow offset: CSS offsetY positive = down, PDF y-axis positive = up.
	sx := x + shadow.OffsetX
	sy := y - shadow.OffsetY

	// For blur, use reduced opacity to simulate the effect.
	if shadow.Blur > 0 {
		gsName := registerOpacity(ctx.Page, 0.5)
		ctx.Stream.SetExtGState(gsName)
	}

	setFillColor(ctx.Stream, shadow.Color)
	ctx.Stream.BeginText()
	resName := registerFont(ctx.Page, word)
	ctx.Stream.SetFont(resName, word.FontSize)
	if word.LetterSpacing != 0 {
		ctx.Stream.SetCharSpacing(word.LetterSpacing)
	}
	ctx.Stream.MoveText(sx, sy)
	if word.Embedded != nil {
		drawWordEmbedded(ctx.Stream, word)
	} else {
		drawWordStandard(ctx.Stream, word)
	}
	if word.LetterSpacing != 0 {
		ctx.Stream.SetCharSpacing(0)
	}
	ctx.Stream.EndText()
	ctx.Stream.RestoreState()
}

// drawBoxShadow draws a box-shadow approximation behind an element.
// It draws a filled rectangle offset by the shadow's OffsetX/OffsetY,
// expanded by Spread. For blur, an additional slightly larger, more
// transparent rectangle is drawn underneath.
func drawBoxShadow(ctx DrawContext, shadow *BoxShadow, x, y, w, h float64) {
	if shadow == nil {
		return
	}
	// Shadow position: in PDF, y increases upward, CSS offsetY positive = down.
	sx := x + shadow.OffsetX - shadow.Spread
	sy := y - shadow.OffsetY - shadow.Spread
	sw := w + 2*shadow.Spread
	sh := h + 2*shadow.Spread

	ctx.Stream.SaveState()

	// If blur > 0, draw a larger, more transparent rect first to approximate blur.
	if shadow.Blur > 0 {
		blurExpand := shadow.Blur
		// Use 50% opacity for the blur layer.
		blurColor := Color{R: shadow.Color.R, G: shadow.Color.G, B: shadow.Color.B, Space: shadow.Color.Space, C: shadow.Color.C, M: shadow.Color.M, Y: shadow.Color.Y, K: shadow.Color.K}
		gsName := registerOpacity(ctx.Page, 0.3)
		ctx.Stream.SaveState()
		ctx.Stream.SetExtGState(gsName)
		setFillColor(ctx.Stream, blurColor)
		ctx.Stream.Rectangle(sx-blurExpand, sy-blurExpand, sw+2*blurExpand, sh+2*blurExpand)
		ctx.Stream.Fill()
		ctx.Stream.RestoreState()
	}

	// Draw main shadow rectangle.
	setFillColor(ctx.Stream, shadow.Color)
	ctx.Stream.Rectangle(sx, sy, sw, sh)
	ctx.Stream.Fill()

	ctx.Stream.RestoreState()
}

// drawOutline draws an outline stroke around an element, outside the border edge.
func drawOutline(ctx DrawContext, width float64, style string, color Color, offset, x, y, w, h float64) {
	if width <= 0 {
		return
	}
	ctx.Stream.SaveState()
	setStrokeColor(ctx.Stream, color)
	ctx.Stream.SetLineWidth(width)

	// Apply dash pattern based on style.
	switch style {
	case "dashed":
		ctx.Stream.SetDashPattern([]float64{width * 3, width * 2}, 0)
	case "dotted":
		ctx.Stream.SetDashPattern([]float64{width, width * 2}, 0)
	}

	// Outline is drawn outside the element box, offset by outlineOffset + half line width.
	expand := offset + width/2
	ox := x - expand
	oy := y - expand
	ow := w + 2*expand
	oh := h + 2*expand
	ctx.Stream.Rectangle(ox, oy, ow, oh)
	ctx.Stream.Stroke()

	// Reset dash pattern if needed.
	if style == "dashed" || style == "dotted" {
		ctx.Stream.SetDashPattern(nil, 0)
	}

	ctx.Stream.RestoreState()
}

// drawRoundedBorders draws borders with per-corner rounded corners.
// Falls back to straight borders when border styles differ per side.
func drawRoundedBorders(stream *content.Stream, borders CellBorders, x, y, w, h float64, r [4]float64) {
	// If all borders are the same, draw a single rounded rect stroke.
	if borders.Top.Width > 0 && borders.Top == borders.Right &&
		borders.Top == borders.Bottom && borders.Top == borders.Left {
		stream.SaveState()
		setStrokeColor(stream, borders.Top.Color)
		stream.SetLineWidth(borders.Top.Width)
		stream.RoundedRectPerCorner(x, y, w, h, r[0], r[1], r[2], r[3])
		stream.Stroke()
		stream.RestoreState()
		return
	}
	// Mixed borders: draw each side individually (straight segments).
	drawCellBorders(stream, borders, x, y, w, h)
}

// drawBackgroundImage draws a background image into a container area.
// (x, y) is bottom-left corner, w and h are the container dimensions.
func drawBackgroundImage(ctx DrawContext, bg *BackgroundImage, x, y, w, h, radius float64) {
	if bg == nil || bg.Image == nil {
		return
	}

	imgW := float64(bg.Image.Width())
	imgH := float64(bg.Image.Height())
	if imgW <= 0 || imgH <= 0 {
		return
	}
	ar := imgW / imgH

	// Determine rendered size.
	drawW := imgW
	drawH := imgH

	switch bg.Size {
	case "cover":
		// Scale to cover entire container.
		scaleW := w / imgW
		scaleH := h / imgH
		scale := scaleW
		if scaleH > scale {
			scale = scaleH
		}
		drawW = imgW * scale
		drawH = imgH * scale
	case "contain":
		// Scale to fit entirely within container.
		scaleW := w / imgW
		scaleH := h / imgH
		scale := scaleW
		if scaleH < scale {
			scale = scaleH
		}
		drawW = imgW * scale
		drawH = imgH * scale
	default:
		if bg.SizeW > 0 && bg.SizeH > 0 {
			drawW = bg.SizeW
			drawH = bg.SizeH
		} else if bg.SizeW > 0 {
			drawW = bg.SizeW
			drawH = drawW / ar
		} else if bg.SizeH > 0 {
			drawH = bg.SizeH
			drawW = drawH * ar
		} else {
			// "auto" — use natural size, but clamp to container.
			if drawW > w {
				drawW = w
				drawH = drawW / ar
			}
			if drawH > h {
				drawH = h
				drawW = drawH * ar
			}
		}
	}

	// Register the image.
	resName := registerImage(ctx.Page, bg.Image)

	ctx.Stream.SaveState()

	// Clip to container bounds.
	if radius > 0 {
		ctx.Stream.RoundedRect(x, y, w, h, radius)
	} else {
		ctx.Stream.Rectangle(x, y, w, h)
	}
	ctx.Stream.ClipNonZero()
	ctx.Stream.EndPath()

	repeat := bg.Repeat
	if repeat == "" {
		repeat = "repeat"
	}

	// Compute initial position based on background-position.
	posX := bg.Position[0] // 0-1
	posY := bg.Position[1] // 0-1

	// Origin position: the offset of the image's top-left within the container.
	// PDF y-axis: y is bottom-left; image placed from bottom-left.
	startX := x + posX*(w-drawW)
	startY := y + (1-posY)*(h-drawH) // posY=0 → top → y + (h - drawH)

	switch repeat {
	case "no-repeat":
		ctx.Stream.SaveState()
		ctx.Stream.ConcatMatrix(drawW, 0, 0, drawH, startX, startY)
		ctx.Stream.Do(resName)
		ctx.Stream.RestoreState()

	case "repeat-x":
		tileX := startX
		// Extend left.
		for tileX > x {
			tileX -= drawW
		}
		for tileX < x+w {
			ctx.Stream.SaveState()
			ctx.Stream.ConcatMatrix(drawW, 0, 0, drawH, tileX, startY)
			ctx.Stream.Do(resName)
			ctx.Stream.RestoreState()
			tileX += drawW
		}

	case "repeat-y":
		tileY := startY
		for tileY+drawH < y+h {
			tileY += drawH
		}
		for tileY > y-drawH {
			ctx.Stream.SaveState()
			ctx.Stream.ConcatMatrix(drawW, 0, 0, drawH, startX, tileY)
			ctx.Stream.Do(resName)
			ctx.Stream.RestoreState()
			tileY -= drawH
		}

	default: // "repeat"
		tileX := startX
		for tileX > x {
			tileX -= drawW
		}
		tileY := startY
		for tileY+drawH < y+h {
			tileY += drawH
		}
		for ty := tileY; ty > y-drawH; ty -= drawH {
			for tx := tileX; tx < x+w; tx += drawW {
				ctx.Stream.SaveState()
				ctx.Stream.ConcatMatrix(drawW, 0, 0, drawH, tx, ty)
				ctx.Stream.Do(resName)
				ctx.Stream.RestoreState()
			}
		}
	}

	ctx.Stream.RestoreState()
}
