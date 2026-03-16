// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/font"
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

	// CIDFont widths (Type0). Check cidWidths explicitly (nil map is safe
	// for reads in Go, but being explicit about the intent).
	if fe.cidWidths != nil {
		if w, ok := fe.cidWidths[charCode]; ok {
			return w
		}
		if fe.defaultW > 0 {
			return fe.defaultW
		}
		return 1000
	}

	// Simple font widths. Explicit nil guard: while len(nil) == 0 in Go
	// (making the idx check safe), the guard documents that widths may
	// be absent for fonts that don't declare /Widths.
	if fe.widths != nil {
		idx := charCode - fe.firstChar
		if idx >= 0 && idx < len(fe.widths) {
			return fe.widths[idx]
		}
	}

	return 0
}

// SpaceWidth returns the width of the space character in text space units (1/1000).
// Returns 0 if the space glyph width is not available.
func (fe *FontEntry) SpaceWidth() int {
	return fe.CharWidth(32)
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
	return BuildFontCacheWithShared(resources, res, nil)
}

// BuildFontCacheWithShared constructs a FontCache like BuildFontCache, but
// reuses parsed FontEntry values from a shared cross-page cache keyed by
// indirect reference object number. This avoids re-parsing the same font
// dictionary on every page of a multi-page document.
func BuildFontCacheWithShared(resources *core.PdfDictionary, res *resolver, shared map[int]*FontEntry) FontCache {
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

		// Check if the font value is an indirect reference so we can
		// look it up in the shared cache by object number.
		var objNum int
		var hasObjNum bool
		if ref, ok := entry.Value.(*core.PdfIndirectReference); ok {
			objNum = ref.ObjectNumber
			hasObjNum = true
		}

		// Try the shared cache first.
		if hasObjNum && shared != nil {
			if fe, found := shared[objNum]; found {
				cache[name] = fe
				continue
			}
		}

		fontVal := resolveWith(res, entry.Value)
		fd, ok := fontVal.(*core.PdfDictionary)
		if !ok {
			continue
		}
		fe := parseFontEntry(fd, res)
		if fe != nil {
			cache[name] = fe
			// Store in shared cache for reuse by other pages.
			if hasObjNum && shared != nil {
				shared[objNum] = fe
			}
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

	// 3. Type0 without ToUnicode — try extracting cmap from embedded font program.
	if fe.isType0 {
		if cmap := extractEmbeddedFontCMap(fd, res); cmap != nil {
			fe.cmap = cmap
		}
		return fe
	}

	// 4. Standard font recognition — if no encoding was declared in the PDF,
	// standard Type1 fonts (Helvetica, Times, Courier, etc.) use
	// WinAnsiEncoding by default and have well-known glyph widths.
	// PDF viewers are required to know these metrics (ISO 32000 §9.6.2.2),
	// so PDFs typically omit /Widths for standard fonts.
	if baseName := getBaseFont(fd); baseName != "" {
		if byteWidths := font.StandardFontByteWidths(baseName); byteWidths != nil {
			fe.encoding = WinAnsiEncoding
			fe.firstChar = 0
			fe.widths = byteWidths
			return fe
		}
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

// getBaseFont extracts the /BaseFont name from a font dictionary.
func getBaseFont(fd *core.PdfDictionary) string {
	if bf, ok := fd.Get("BaseFont").(*core.PdfName); ok {
		return bf.Value
	}
	return ""
}

// extractEmbeddedFontCMap attempts to extract a GID→Unicode mapping from an
// embedded font program in a Type0 (composite) font dictionary. It navigates
// the font hierarchy: Type0 → DescendantFonts → CIDFont → FontDescriptor →
// FontFile2/FontFile3 to find the embedded TrueType font, then uses its cmap
// table to build a reverse mapping from glyph IDs to Unicode code points.
func extractEmbeddedFontCMap(fd *core.PdfDictionary, res *resolver) *CMap {
	// 1. Get /DescendantFonts → first element → CIDFont dict.
	dfObj := fd.Get("DescendantFonts")
	if dfObj == nil {
		return nil
	}
	dfObj = resolveWith(res, dfObj)
	dfArr, ok := dfObj.(*core.PdfArray)
	if !ok || dfArr.Len() == 0 {
		return nil
	}
	cidFontObj := resolveWith(res, dfArr.Elements[0])
	cidFont, ok := cidFontObj.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	// 2. Check /CIDToGIDMap. If it's a stream (not /Identity), parse it
	//    to apply CID→GID remapping before the GID→Unicode lookup.
	var cidToGID []uint16
	if cidToGIDObj := cidFont.Get("CIDToGIDMap"); cidToGIDObj != nil {
		cidToGIDObj = resolveWith(res, cidToGIDObj)
		switch v := cidToGIDObj.(type) {
		case *core.PdfName:
			// /Identity — CID = GID, no remapping needed.
		case *core.PdfStream:
			// Stream of 2-byte big-endian GID entries, one per CID.
			data := v.Data
			if len(data) >= 2 {
				cidToGID = make([]uint16, len(data)/2)
				for i := 0; i+1 < len(data); i += 2 {
					cidToGID[i/2] = uint16(data[i])<<8 | uint16(data[i+1])
				}
			}
		}
	}

	// 3. Get /FontDescriptor.
	fdescObj := cidFont.Get("FontDescriptor")
	if fdescObj == nil {
		return nil
	}
	fdescObj = resolveWith(res, fdescObj)
	fdesc, ok := fdescObj.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	// 4. Try /FontFile2 (TrueType), then /FontFile3 (CFF/OpenType).
	var fontStream *core.PdfStream
	for _, key := range []string{"FontFile2", "FontFile3"} {
		ffObj := fdesc.Get(key)
		if ffObj == nil {
			continue
		}
		ffObj = resolveWith(res, ffObj)
		if s, ok := ffObj.(*core.PdfStream); ok && len(s.Data) > 0 {
			fontStream = s
			break
		}
	}
	if fontStream == nil {
		return nil
	}

	// 5. Build GID→Unicode map from the embedded font's cmap table.
	gidMap := font.BuildGIDToUnicode(fontStream.Data)
	if gidMap == nil {
		return nil
	}

	// 6. If we have a CIDToGIDMap stream, we need to remap:
	//    character code (=CID) → GID (via cidToGID) → Unicode (via gidMap).
	if cidToGID != nil {
		return buildCMapFromCIDToGID(cidToGID, gidMap)
	}

	// 7. Identity mapping: character code = CID = GID.
	return buildCMapFromGIDMap(gidMap)
}

// buildCMapFromGIDMap creates a CMap that maps 2-byte character codes
// (treated as glyph IDs under Identity-H encoding) to Unicode using a
// GID→rune reverse map extracted from an embedded font's cmap table.
func buildCMapFromGIDMap(gidToUnicode map[uint16]rune) *CMap {
	if len(gidToUnicode) == 0 {
		return nil
	}
	cm := &CMap{
		codeSpaceRanges: []codeSpaceRange{
			{low: 0, high: 0xFFFF, bytes: 2},
		},
		bfChars: make(map[uint32]string, len(gidToUnicode)),
	}
	for gid, r := range gidToUnicode {
		cm.bfChars[uint32(gid)] = string(r)
	}
	return cm
}

// buildCMapFromCIDToGID creates a CMap that maps 2-byte character codes (CIDs)
// to Unicode by first applying a CID→GID mapping, then a GID→rune mapping.
func buildCMapFromCIDToGID(cidToGID []uint16, gidToUnicode map[uint16]rune) *CMap {
	cm := &CMap{
		codeSpaceRanges: []codeSpaceRange{
			{low: 0, high: 0xFFFF, bytes: 2},
		},
		bfChars: make(map[uint32]string),
	}
	for cid, gid := range cidToGID {
		if gid == 0 {
			continue
		}
		if r, ok := gidToUnicode[gid]; ok {
			cm.bfChars[uint32(cid)] = string(r)
		}
	}
	if len(cm.bfChars) == 0 {
		return nil
	}
	return cm
}

// resolveWith resolves an indirect reference using the resolver, or returns
// the object as-is on failure. This is intentional tolerance: PDFs in the
// wild frequently contain broken or circular references in font dictionaries,
// encoding arrays, and width tables. Returning the original (unresolved)
// object lets callers degrade gracefully — they will typically see a type
// mismatch on the next type-assertion and skip the entry, which is the
// correct behavior for a reader that prioritises opening documents over
// strict validation. The error is deliberately not propagated because every
// call site already handles the "wrong type" case with an ok-check.
func resolveWith(res *resolver, obj core.PdfObject) core.PdfObject {
	if res == nil {
		return obj
	}
	resolved, err := res.ResolveDeep(obj)
	if err != nil {
		// Resolution failed (broken ref, circular ref, etc.).
		// Return the original object — callers handle unexpected types gracefully.
		return obj
	}
	return resolved
}
