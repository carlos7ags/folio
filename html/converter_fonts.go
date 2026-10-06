// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// maxFontBytes caps a single @font-face read. Larger than the image/CSS
// cap because pan-CJK TTC collections (Noto CJK, STHeiti, etc.) routinely
// run to tens or low hundreds of MB, yet the read must still be bounded.
const maxFontBytes = 256 << 20

// loadFontFaces loads @font-face fonts into the converter's embeddedFonts map.
// data: URIs are decoded inline; every other src — http(s) URL, BaseFS-
// relative path, and absolute filesystem path — flows through the unified
// [resolveLocalAsset] contract. The font is resolved relative to the
// stylesheet it was declared in (its origin), so url("../fonts/x.ttf")
// inside styles/site.css resolves to fonts/x.ttf at the BaseFS root.
// Inline <style> blocks resolve from the BaseFS root.
//
// Failures (missing data, invalid font bytes, fetch errors) are reported
// through Options.Logger at warn level and skipped — they never abort the
// conversion.
func (c *converter) loadFontFaces(faces []fontFaceRule) {
	// Document-level lang drives TTC face selection. Empty lang is
	// the no-op default — font.ParseFontForLanguage with "" picks
	// face 0, matching the legacy font.ParseFont behaviour for
	// back-compat. Per-element lang overrides (<p lang="ja">) are
	// not yet honoured (#280 Phase 2): a single face is loaded per
	// @font-face rule at converter setup time, so an element-level
	// override would need either eager multi-face loading or a
	// shape-time face selector — both deferred.
	lang := c.metadata.Language
	for _, ff := range faces {
		src := ff.src

		var face font.Face
		var data []byte
		var err error

		if strings.HasPrefix(src, "data:") {
			face, err = decodeFontDataURI(src)
		} else {
			data, err = c.resolveLocalAsset(ff.origin, src, maxFontBytes)
			if err == nil {
				face, err = font.ParseFontForLanguage(data, lang)
			}
		}

		if err != nil {
			c.reportAssetError("@font-face", err,
				"family", ff.family, "src", src, "origin", ff.origin)
			continue
		}
		ef := font.NewEmbeddedFont(face)
		// Key shape: family|<numeric weight>|style. The numeric weight
		// matches what computedStyle.FontWeight stores after
		// parseFontWeight, which lets resolveFontPair walk the available
		// weights for nearest-match selection.
		key := ff.family + "|" + strconv.Itoa(ff.weight) + "|" + ff.style
		c.embeddedFonts[key] = ef
	}
}

// decodeFontDataURI decodes a base64-encoded font from a data: URI.
// Supports data:font/truetype;base64,..., data:font/opentype;base64,...,
// data:application/x-font-ttf;base64,..., and similar media types.
func decodeFontDataURI(uri string) (font.Face, error) {
	rest := strings.TrimPrefix(uri, "data:")
	commaIdx := strings.IndexByte(rest, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("html: invalid data URI: no comma")
	}
	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	if !strings.Contains(meta, ";base64") {
		return nil, fmt.Errorf("html: font data URI must be base64-encoded")
	}

	data, err := base64Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("html: font data URI base64: %w", err)
	}

	return font.ParseTTF(data)
}

// getFallbackFont returns a Unicode-capable embedded font for text that
// can't be encoded in WinAnsiEncoding. The font is loaded lazily on first
// use. Returns nil if no suitable font is found.
//
// Lookup order: Options.FallbackFontPath via BaseFS, then via OS, then a
// hardcoded list of common system font locations.
func (c *converter) getFallbackFont() *font.EmbeddedFont {
	if c.fallbackFontLoaded {
		return c.fallbackFont
	}
	c.fallbackFontLoaded = true

	// Document lang reaches this lazy load because getFallbackFont
	// is only invoked from chooseFont during walkChildren, which
	// runs after findHTMLLang in ConvertFull/Convert. The same TTC
	// face-selection rules that govern @font-face apply: a doc with
	// lang="zh-CN" pulling NotoSansCJK-Regular.ttc as the system
	// fallback picks the SC face instead of JP face-0.
	lang := c.metadata.Language

	if c.opts.FallbackFontPath != "" {
		if face, err := c.loadFallbackFont(c.opts.FallbackFontPath, lang); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		} else {
			c.reportAssetError("FallbackFontPath", err,
				"path", c.opts.FallbackFontPath)
		}
	}

	// Search common system font locations for a Unicode-capable font.
	// CJK-specific fonts are listed first since they provide the widest
	// coverage for East Asian scripts while also covering Latin.
	candidates := []string{
		// macOS — CJK fonts
		"/Library/Fonts/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/Hiragino Sans GB.ttc",
		// macOS — general Unicode
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
		// Linux — CJK fonts
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		// Linux — general Unicode
		"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		// Windows — CJK fonts
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\msgothic.ttc`,
		`C:\Windows\Fonts\malgun.ttf`,
		`C:\Windows\Fonts\simsun.ttc`,
		// Windows — general Unicode
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\segoeui.ttf`,
	}
	for _, path := range candidates {
		if face, err := font.LoadFontForLanguage(path, lang); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		}
	}

	return nil
}

// loadFallbackFont resolves the FallbackFontPath option with two
// programmatic-only carve-outs from the standard [resolveLocalAsset]
// contract that document-content resolvers do not get:
//
//  1. An absolute path always bypasses BaseFS to the OS, regardless of
//     whether BaseFS is set. FallbackFontPath is configured by the
//     embedding application, almost always at a system font location;
//     callers typically build BaseFS for their asset directory rather
//     than the system font tree. Document-supplied absolute paths
//     (`<img src="/abs">`, `@font-face url('/abs')`) follow the
//     centralized rule because the asset reference comes from
//     untrusted content.
//
//  2. A relative path that misses in BaseFS retries against the OS so
//     a typo in the option does not silently produce no-fallback text.
//     The retry is logged at debug level so the BaseFS attempt remains
//     observable when investigating which path the loader took.
func (c *converter) loadFallbackFont(p, lang string) (font.Face, error) {
	if filepath.IsAbs(p) {
		return font.LoadFontForLanguage(p, lang)
	}
	if c.opts.BaseFS != nil {
		data, baseErr := c.resolveLocalAsset("", p, maxFontBytes)
		if baseErr == nil {
			return font.ParseFontForLanguage(data, lang)
		}
		c.logger.Debug("folio/html: FallbackFontPath not in BaseFS, trying OS",
			"path", p, "error", baseErr)
	}
	return font.LoadFontForLanguage(p, lang)
}

// resolveFontForText returns the best font for the given text. If the text
// can be encoded in WinAnsiEncoding, returns the standard font. Otherwise,
// tries the embedded fonts from @font-face, then the system fallback font.
func (c *converter) resolveFontForText(style computedStyle, text string) (*font.Standard, *font.EmbeddedFont) {
	stdFont, embFont := c.resolveFontPair(style)

	// If already using an embedded font (from @font-face), it handles Unicode.
	if embFont != nil {
		return nil, embFont
	}

	// Standard font — check if text fits in WinAnsiEncoding.
	if font.CanEncodeWinAnsi(text) {
		return stdFont, nil
	}

	// Text has non-WinAnsi characters — try fallback.
	if fb := c.getFallbackFont(); fb != nil {
		return nil, fb
	}

	// No fallback available — use standard font (chars will become ?).
	return stdFont, nil
}

// applyUnicodeBidi wraps text in Unicode bidi control characters based on
// the computed CSS direction and unicode-bidi properties. This implements
// CSS Unicode Bidirectional Algorithm integration:
//
//   - bidi-override + rtl: wraps in RLO...PDF (U+202E...U+202C) to force
//     all characters to RTL visual order regardless of their bidi class.
//   - bidi-override + ltr: wraps in LRO...PDF (U+202D...U+202C).
//   - embed + rtl: wraps in RLE...PDF (U+202B...U+202C).
//   - embed + ltr: wraps in LRE...PDF (U+202A...U+202C).
//   - isolate + rtl: wraps in RLI...PDI (U+2067...U+2069).
//   - isolate + ltr: wraps in LRI...PDI (U+2066...U+2069).
//
// These characters are consumed by the bidi algorithm in x/text/unicode/bidi
// during resolveLineBidi, producing the correct embedding levels.
func applyUnicodeBidi(text string, style computedStyle) string {
	if style.UnicodeBidi == "" || style.UnicodeBidi == "normal" {
		return text
	}
	switch style.UnicodeBidi {
	case "bidi-override":
		if style.Direction == layout.DirectionRTL {
			return "\u202E" + text + "\u202C" // RLO + text + PDF
		}
		if style.Direction == layout.DirectionLTR {
			return "\u202D" + text + "\u202C" // LRO + text + PDF
		}
	case "embed":
		if style.Direction == layout.DirectionRTL {
			return "\u202B" + text + "\u202C" // RLE + text + PDF
		}
		if style.Direction == layout.DirectionLTR {
			return "\u202A" + text + "\u202C" // LRE + text + PDF
		}
	case "isolate":
		if style.Direction == layout.DirectionRTL {
			return "\u2067" + text + "\u2069" // RLI + text + PDI
		}
		if style.Direction == layout.DirectionLTR {
			return "\u2066" + text + "\u2069" // LRI + text + PDI
		}
	case "isolate-override":
		if style.Direction == layout.DirectionRTL {
			return "\u2067\u202E" + text + "\u202C\u2069"
		}
		if style.Direction == layout.DirectionLTR {
			return "\u2067\u202D" + text + "\u202C\u2069"
		}
	}
	return text
}

// splitTextByFont splits a text string into one or more TextRuns at script
// boundaries where the font needs to change. Characters encodable in
// WinAnsiEncoding use the standard font; characters that need a fallback
// (Hebrew, Arabic, CJK, etc.) use the embedded fallback font. This enables
// mixed-script text like "Hello שלום" to render correctly when the standard
// font lacks Hebrew glyphs but the fallback font covers both scripts.
//
// When the style already specifies an embedded font (via @font-face), or
// when no fallback font is available, the text is returned as a single run
// (no splitting needed — same behavior as before this function existed).
func (c *converter) splitTextByFont(text string, style computedStyle) []layout.TextRun {
	// Apply unicode-bidi overrides by wrapping text in Unicode bidi
	// control characters. This forces the base embedding level per
	// CSS Unicode Bidirectional Algorithm §2.2.
	text = applyUnicodeBidi(text, style)

	stdFont, embFont := c.resolveFontPair(style)

	// If already using an embedded font, it handles all Unicode — no split.
	if embFont != nil {
		return []layout.TextRun{c.makeTextRun(text, nil, embFont, style)}
	}

	// If all text fits in WinAnsi, use standard font — no split.
	if font.CanEncodeWinAnsi(text) {
		return []layout.TextRun{c.makeTextRun(text, stdFont, nil, style)}
	}

	// Get the fallback font. If unavailable, return single run with std font.
	fb := c.getFallbackFont()
	if fb == nil {
		return []layout.TextRun{c.makeTextRun(text, stdFont, nil, style)}
	}

	// Split at boundaries between WinAnsi-encodable and non-WinAnsi characters.
	// Consecutive characters that share the same "needs fallback" status are
	// grouped into a single run to minimize run count.
	runes := []rune(text)
	var runs []layout.TextRun
	start := 0
	startNeedsFallback := !font.CanEncodeWinAnsiRune(runes[0])

	for i := 1; i <= len(runes); i++ {
		needsFallback := false
		if i < len(runes) {
			needsFallback = !font.CanEncodeWinAnsiRune(runes[i])
		}
		// Emit a run at boundaries or at end of string.
		if i == len(runes) || needsFallback != startNeedsFallback {
			seg := string(runes[start:i])
			if startNeedsFallback {
				runs = append(runs, c.makeTextRun(seg, nil, fb, style))
			} else {
				runs = append(runs, c.makeTextRun(seg, stdFont, nil, style))
			}
			start = i
			startNeedsFallback = needsFallback
		}
	}

	return runs
}

// makeTextRun creates a TextRun with all styling fields from the computed style.
func (c *converter) makeTextRun(text string, std *font.Standard, emb *font.EmbeddedFont, style computedStyle) layout.TextRun {
	return layout.TextRun{
		Text:            text,
		Font:            std,
		Embedded:        emb,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
		TextShadow:      textShadowFromStyle(style),
		BackgroundColor: style.BackgroundColor,
		NoWrap:          style.WhiteSpace == "nowrap",
	}
}
