// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"github.com/carlos7ags/folio/core"
)

// FontEntry holds the decoded character mapping and glyph widths
// for a single PDF font used during content stream parsing.
type FontEntry struct {
	cmap     *CMap     // from /ToUnicode (preferred)
	encoding *Encoding // from /Encoding (fallback for simple fonts)
	isType0  bool      // composite font (2-byte codes by default)

	// Glyph widths in 1/1000 of text space unit.
	firstChar int         // /FirstChar for simple fonts
	widths    []int       // /Widths array (indexed by charCode - firstChar)
	cidWidths map[int]int // CID → width for Type0 fonts
	defaultW  int         // /DW default width for CIDFonts (default 1000)
}

// Decode converts raw character code bytes to Unicode text.
func (fe *FontEntry) Decode(raw []byte) string {
	if fe == nil {
		return string(raw)
	}
	if fe.cmap != nil {
		return fe.cmap.Decode(raw)
	}
	if fe.encoding != nil {
		return fe.encoding.Decode(raw)
	}
	return string(raw)
}

// CharWidth returns the width of a character code in 1/1000 of text space.
// Returns 0 if width data is not available (caller should use estimation).
func (fe *FontEntry) CharWidth(charCode int) int {
	if fe == nil {
		return 0
	}

	// CIDFont widths (Type0).
	if fe.cidWidths != nil {
		if w, ok := fe.cidWidths[charCode]; ok {
			return w
		}
		if fe.defaultW > 0 {
			return fe.defaultW
		}
		return 1000
	}

	// Simple font widths.
	if fe.widths != nil {
		idx := charCode - fe.firstChar
		if idx >= 0 && idx < len(fe.widths) {
			return fe.widths[idx]
		}
	}

	return 0
}

// TextWidth computes the width of raw character code bytes in 1/1000 units.
// For simple fonts, each byte is a character code. For CIDFonts, pairs of
// bytes form character codes.
func (fe *FontEntry) TextWidth(raw []byte) int {
	if fe == nil {
		return 0
	}

	total := 0
	if fe.isType0 {
		// CIDFont: 2-byte character codes.
		for i := 0; i+1 < len(raw); i += 2 {
			code := int(raw[i])<<8 | int(raw[i+1])
			total += fe.CharWidth(code)
		}
	} else {
		// Simple font: 1-byte character codes.
		for _, b := range raw {
			total += fe.CharWidth(int(b))
		}
	}
	return total
}

// FontCache maps font resource names (e.g. "F1") to their FontEntry.
type FontCache map[string]*FontEntry

// BuildFontCache constructs a FontCache from a page's Resources dictionary.
// The resolver is used to dereference indirect objects (font dicts, streams).
func BuildFontCache(resources *core.PdfDictionary, res *resolver) FontCache {
	if resources == nil {
		return nil
	}

	fontObj := resources.Get("Font")
	if fontObj == nil {
		return nil
	}
	fontObj = resolveWith(res, fontObj)
	fontDict, ok := fontObj.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	cache := make(FontCache)
	for _, entry := range fontDict.Entries {
		name := entry.Key.Value
		fontVal := resolveWith(res, entry.Value)
		fd, ok := fontVal.(*core.PdfDictionary)
		if !ok {
			continue
		}
		fe := parseFontEntry(fd, res)
		if fe != nil {
			cache[name] = fe
		}
	}
	return cache
}

// parseFontEntry extracts encoding and width information from a font dictionary.
func parseFontEntry(fd *core.PdfDictionary, res *resolver) *FontEntry {
	fe := &FontEntry{defaultW: 1000}

	// Check subtype for Type0 (composite) fonts.
	if st, ok := fd.Get("Subtype").(*core.PdfName); ok {
		fe.isType0 = st.Value == "Type0"
	}

	// Extract glyph widths.
	parseFontWidths(fd, fe, res)

	// 1. ToUnicode CMap — highest priority.
	if tuObj := fd.Get("ToUnicode"); tuObj != nil {
		tuObj = resolveWith(res, tuObj)
		if stream, ok := tuObj.(*core.PdfStream); ok && len(stream.Data) > 0 {
			fe.cmap = ParseCMap(stream.Data)
			// For Type0 fonts with Identity-H encoding and a ToUnicode CMap,
			// ensure the CMap uses 2-byte codes.
			if fe.isType0 && fe.cmap.CodeBytes() == 0 {
				fe.cmap.codeSpaceRanges = append(fe.cmap.codeSpaceRanges, codeSpaceRange{
					low: 0, high: 0xFFFF, bytes: 2,
				})
			}
			return fe
		}
	}

	// 2. /Encoding — for simple fonts.
	if encObj := fd.Get("Encoding"); encObj != nil {
		encObj = resolveWith(res, encObj)
		switch enc := encObj.(type) {
		case *core.PdfName:
			switch enc.Value {
			case "WinAnsiEncoding":
				fe.encoding = WinAnsiEncoding
			case "MacRomanEncoding":
				fe.encoding = MacRomanEncoding
			case "StandardEncoding":
				fe.encoding = StandardEncoding
			}
		case *core.PdfDictionary:
			fe.encoding = parseEncodingDict(enc, res)
		}
		if fe.encoding != nil {
			return fe
		}
	}

	// 3. Type0 with Identity-H but no ToUnicode — can't decode, return nil.
	if fe.isType0 {
		return fe // Decode will fall back to raw bytes.
	}

	return nil // No useful encoding found.
}

// parseEncodingDict handles /Encoding dictionaries with /BaseEncoding and /Differences.
func parseEncodingDict(d *core.PdfDictionary, res *resolver) *Encoding {
	// Start with base encoding.
	var base *Encoding
	if bn, ok := d.Get("BaseEncoding").(*core.PdfName); ok {
		switch bn.Value {
		case "WinAnsiEncoding":
			base = WinAnsiEncoding
		case "MacRomanEncoding":
			base = MacRomanEncoding
		case "StandardEncoding":
			base = StandardEncoding
		}
	}
	if base == nil {
		base = StandardEncoding
	}

	// Clone base encoding so we can modify it with Differences.
	enc := &Encoding{}
	*enc = *base

	// Apply /Differences array.
	diffsObj := d.Get("Differences")
	if diffsObj == nil {
		return enc
	}
	diffsObj = resolveWith(res, diffsObj)
	arr, ok := diffsObj.(*core.PdfArray)
	if !ok {
		return enc
	}

	code := 0
	for _, elem := range arr.Elements {
		switch v := elem.(type) {
		case *core.PdfNumber:
			code = int(v.IntValue())
		case *core.PdfName:
			if code >= 0 && code < 256 {
				if r := GlyphToRune(v.Value); r != 0 {
					enc.table[code] = r
				}
			}
			code++
		}
	}
	return enc
}

// parseFontWidths extracts glyph width data from a font dictionary.
func parseFontWidths(fd *core.PdfDictionary, fe *FontEntry, res *resolver) {
	if fe.isType0 {
		dfObj := fd.Get("DescendantFonts")
		if dfObj == nil {
			return
		}
		dfObj = resolveWith(res, dfObj)
		dfArr, ok := dfObj.(*core.PdfArray)
		if !ok || dfArr.Len() == 0 {
			return
		}
		cidFontObj := resolveWith(res, dfArr.Elements[0])
		cidFont, ok := cidFontObj.(*core.PdfDictionary)
		if !ok {
			return
		}

		if dw := cidFont.Get("DW"); dw != nil {
			if num, ok := dw.(*core.PdfNumber); ok {
				fe.defaultW = num.IntValue()
			}
		}

		wObj := cidFont.Get("W")
		if wObj == nil {
			return
		}
		wObj = resolveWith(res, wObj)
		wArr, ok := wObj.(*core.PdfArray)
		if !ok {
			return
		}
		fe.cidWidths = parseCIDWidths(wArr)
		return
	}

	// Simple font: /FirstChar, /LastChar, /Widths.
	fcObj := fd.Get("FirstChar")
	if fcObj == nil {
		return
	}
	fcNum, ok := fcObj.(*core.PdfNumber)
	if !ok {
		return
	}
	fe.firstChar = fcNum.IntValue()

	wObj := fd.Get("Widths")
	if wObj == nil {
		return
	}
	wObj = resolveWith(res, wObj)
	wArr, ok := wObj.(*core.PdfArray)
	if !ok {
		return
	}

	fe.widths = make([]int, wArr.Len())
	for i, elem := range wArr.Elements {
		if num, ok := elem.(*core.PdfNumber); ok {
			fe.widths[i] = num.IntValue()
		}
	}
}

// parseCIDWidths parses a CIDFont /W array into a CID → width map.
func parseCIDWidths(arr *core.PdfArray) map[int]int {
	widths := make(map[int]int)
	elems := arr.Elements
	i := 0

	for i < len(elems) {
		cidNum, ok := elems[i].(*core.PdfNumber)
		if !ok {
			i++
			continue
		}
		startCID := cidNum.IntValue()
		i++
		if i >= len(elems) {
			break
		}

		switch next := elems[i].(type) {
		case *core.PdfArray:
			for j, wElem := range next.Elements {
				if wNum, ok := wElem.(*core.PdfNumber); ok {
					widths[startCID+j] = wNum.IntValue()
				}
			}
			i++
		case *core.PdfNumber:
			endCID := next.IntValue()
			i++
			if i < len(elems) {
				if wNum, ok := elems[i].(*core.PdfNumber); ok {
					w := wNum.IntValue()
					for cid := startCID; cid <= endCID; cid++ {
						widths[cid] = w
					}
				}
				i++
			}
		default:
			i++
		}
	}

	return widths
}

// resolveWith resolves an indirect reference using the resolver, or returns the object as-is.
func resolveWith(res *resolver, obj core.PdfObject) core.PdfObject {
	if res == nil {
		return obj
	}
	resolved, err := res.ResolveDeep(obj)
	if err != nil {
		return obj
	}
	return resolved
}
