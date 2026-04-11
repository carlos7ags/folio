// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// GPOSFeature identifies an OpenType GPOS feature tag.
type GPOSFeature string

const (
	// GPOSKern is the Pair Positioning feature. It is the GPOS equivalent
	// of the legacy TrueType kern table and the preferred source of pair
	// kerning in modern fonts. ISO 14496-22 §6.3 LookupType 2.
	GPOSKern GPOSFeature = "kern"

	// GPOSMark is the Mark-to-Base Positioning feature. It positions a
	// combining mark glyph (e.g. an Arabic harakah or Hebrew niqqud) on
	// an explicit anchor of a base glyph. ISO 14496-22 §6.3 LookupType 4.
	GPOSMark GPOSFeature = "mark"

	// GPOSMkmk is the Mark-to-Mark Positioning feature. Recognized for
	// future use; LookupType 6 is not implemented in this iteration.
	GPOSMkmk GPOSFeature = "mkmk"
)

// PairAdjustment holds the XAdvance delta to apply to the first glyph of
// a kerning pair, expressed in font design units (FUnits). Only the
// horizontal advance is captured: the initial GPOS iteration ignores
// XPlacement, YPlacement, YAdvance, and any Device tables.
type PairAdjustment struct {
	XAdvance int16
}

// Anchor is an (x, y) point in font design units. Anchor Format 2
// (indexed anchor point for hinting) and Format 3 (with Device tables)
// are parsed but their extra fields are discarded.
type Anchor struct {
	X, Y int16
}

// MarkRecord is the entry of a MarkArray: the mark class the mark
// belongs to plus its attachment anchor.
type MarkRecord struct {
	Class  uint16
	Anchor Anchor
}

// BaseRecord is the entry of a BaseArray: one anchor per mark class for
// a given base glyph. The slice is indexed by mark class; a class with
// no declared anchor yields a zero Anchor.
type BaseRecord struct {
	Anchors []Anchor
}

// GPOSAdjustments holds parsed GPOS positioning data grouped by feature
// tag. A nil map or nil outer struct means "feature absent".
type GPOSAdjustments struct {
	// Pairs holds LookupType 2 adjustments keyed by (left, right) glyph
	// ID. Only the horizontal XAdvance is stored.
	Pairs map[GPOSFeature]map[[2]uint16]PairAdjustment

	// Marks holds LookupType 4 mark records keyed by mark glyph ID.
	Marks map[GPOSFeature]map[uint16]MarkRecord

	// Bases holds LookupType 4 base records keyed by base glyph ID.
	Bases map[GPOSFeature]map[uint16]BaseRecord
}

// ParseGPOS reads the GPOS table from raw font bytes and extracts
// LookupType 2 (Pair Positioning) and LookupType 4 (Mark-to-Base
// Positioning) data for the "kern" and "mark" features. Extension
// lookups (LookupType 9) are unwrapped for types 2 and 4.
//
// Script selection follows the same preference order as GSUB:
// "arab" and "latn" when present, "DFLT" as a fallback.
//
// Returns nil if the font has no GPOS table or no matching data.
//
// Reference: ISO 14496-22 §6.3, OpenType GPOS table.
func ParseGPOS(data []byte) *GPOSAdjustments {
	gpos := findTable(data, "GPOS")
	if len(gpos) < 10 {
		return nil
	}

	scriptListOff := int(be16(gpos, 4))
	featureListOff := int(be16(gpos, 6))
	lookupListOff := int(be16(gpos, 8))
	if scriptListOff >= len(gpos) || featureListOff >= len(gpos) || lookupListOff >= len(gpos) {
		return nil
	}

	featureIndices := scriptFeatureIndices(gpos, scriptListOff)
	if len(featureIndices) == 0 {
		return nil
	}

	targetTags := map[string]GPOSFeature{
		"kern": GPOSKern,
		"mark": GPOSMark,
		"mkmk": GPOSMkmk,
	}
	featureToLookups := matchGPOSFeatures(gpos, featureListOff, featureIndices, targetTags)
	if len(featureToLookups) == 0 {
		return nil
	}

	result := &GPOSAdjustments{
		Pairs: make(map[GPOSFeature]map[[2]uint16]PairAdjustment),
		Marks: make(map[GPOSFeature]map[uint16]MarkRecord),
		Bases: make(map[GPOSFeature]map[uint16]BaseRecord),
	}
	for feat, lookupIndices := range featureToLookups {
		pairs := make(map[[2]uint16]PairAdjustment)
		marks := make(map[uint16]MarkRecord)
		bases := make(map[uint16]BaseRecord)
		parseGPOSLookups(gpos, lookupListOff, lookupIndices, pairs, marks, bases)
		if len(pairs) > 0 {
			result.Pairs[feat] = pairs
		}
		if len(marks) > 0 {
			result.Marks[feat] = marks
		}
		if len(bases) > 0 {
			result.Bases[feat] = bases
		}
	}
	if len(result.Pairs) == 0 && len(result.Marks) == 0 && len(result.Bases) == 0 {
		return nil
	}
	return result
}

// PairAdjust returns the horizontal XAdvance adjustment in FUnits that
// the GPOS "kern" feature assigns to the pair (left, right), or 0 if the
// pair is absent. This is the GPOS-table analogue of the legacy kern
// table lookup.
func (g *GPOSAdjustments) PairAdjust(left, right uint16) int16 {
	if g == nil {
		return 0
	}
	pairs, ok := g.Pairs[GPOSKern]
	if !ok {
		return 0
	}
	return pairs[[2]uint16{left, right}].XAdvance
}

// MarkOffset returns the (dx, dy) offset in FUnits needed to move a mark
// glyph's origin so its anchor coincides with the base glyph's anchor
// for the same mark class. Returns ok=false when either glyph is absent
// from the feature's mark/base arrays, or when the base has no anchor
// declared for the mark's class.
//
// The formula is the direct anchor subtraction from ISO 14496-22 §6.3
// LookupType 4: dx = baseAnchor.X - markAnchor.X,
// dy = baseAnchor.Y - markAnchor.Y.
func (g *GPOSAdjustments) MarkOffset(baseGID, markGID uint16, feature GPOSFeature) (dx, dy int16, ok bool) {
	if g == nil {
		return 0, 0, false
	}
	marks, mok := g.Marks[feature]
	if !mok {
		return 0, 0, false
	}
	mark, hasMark := marks[markGID]
	if !hasMark {
		return 0, 0, false
	}
	bases, bok := g.Bases[feature]
	if !bok {
		return 0, 0, false
	}
	base, hasBase := bases[baseGID]
	if !hasBase {
		return 0, 0, false
	}
	if int(mark.Class) >= len(base.Anchors) {
		return 0, 0, false
	}
	b := base.Anchors[mark.Class]
	return b.X - mark.Anchor.X, b.Y - mark.Anchor.Y, true
}

// matchGPOSFeatures is the GPOS counterpart of matchFeatures: it walks
// the FeatureList, selects only features whose tag is in targetTags and
// whose index is in allowed, and returns a map from GPOSFeature to the
// lookup indices it references.
func matchGPOSFeatures(gpos []byte, off int, allowed []int, targetTags map[string]GPOSFeature) map[GPOSFeature][]int {
	if off+2 > len(gpos) {
		return nil
	}
	count := int(be16(gpos, off))
	if off+2+count*6 > len(gpos) {
		return nil
	}
	allowSet := make(map[int]bool, len(allowed))
	for _, idx := range allowed {
		allowSet[idx] = true
	}
	result := make(map[GPOSFeature][]int)
	for i := 0; i < count; i++ {
		if !allowSet[i] {
			continue
		}
		rec := gpos[off+2+i*6:]
		feat, ok := targetTags[string(rec[:4])]
		if !ok {
			continue
		}
		featureOff := off + int(be16(rec, 4))
		if featureOff+4 > len(gpos) {
			continue
		}
		lookupCount := int(be16(gpos, featureOff+2))
		if featureOff+4+lookupCount*2 > len(gpos) {
			continue
		}
		lookups := make([]int, lookupCount)
		for j := 0; j < lookupCount; j++ {
			lookups[j] = int(be16(gpos, featureOff+4+j*2))
		}
		result[feat] = append(result[feat], lookups...)
	}
	return result
}

// parseGPOSLookups walks each referenced lookup and dispatches its
// subtables to the appropriate LookupType parser. LookupType 9
// (Extension Positioning) is unwrapped inline for types 2 and 4.
func parseGPOSLookups(gpos []byte, listOff int, indices []int,
	pairs map[[2]uint16]PairAdjustment,
	marks map[uint16]MarkRecord,
	bases map[uint16]BaseRecord,
) {
	if listOff+2 > len(gpos) {
		return
	}
	count := int(be16(gpos, listOff))
	if listOff+2+count*2 > len(gpos) {
		return
	}
	for _, idx := range indices {
		if idx >= count {
			continue
		}
		lookupOff := listOff + int(be16(gpos, listOff+2+idx*2))
		parseGPOSLookup(gpos, lookupOff, pairs, marks, bases)
	}
}

// parseGPOSLookup reads a single Lookup table and calls the subtable
// parser matching its lookup type. Extension subtables (type 9) are
// followed to their target subtable in the same GPOS blob.
func parseGPOSLookup(gpos []byte, lookupOff int,
	pairs map[[2]uint16]PairAdjustment,
	marks map[uint16]MarkRecord,
	bases map[uint16]BaseRecord,
) {
	if lookupOff+6 > len(gpos) {
		return
	}
	lookupType := be16(gpos, lookupOff)
	subCount := int(be16(gpos, lookupOff+4))
	if lookupOff+6+subCount*2 > len(gpos) {
		return
	}
	for si := 0; si < subCount; si++ {
		subOff := lookupOff + int(be16(gpos, lookupOff+6+si*2))
		t := lookupType
		off := subOff
		if t == 9 {
			// ExtensionPosFormat1:
			//   posFormat         uint16 (==1)
			//   extensionLookupType uint16
			//   extensionOffset    Offset32 (from start of this subtable)
			if subOff+8 > len(gpos) {
				continue
			}
			t = be16(gpos, subOff+2)
			off = subOff + int(be32(gpos, subOff+4))
			if off >= len(gpos) {
				continue
			}
		}
		switch t {
		case 2:
			parsePairPos(gpos, off, pairs)
		case 4:
			parseMarkBasePos(gpos, off, marks, bases)
		}
	}
}

// parsePairPos dispatches on the PairPos format byte. Format 1 is
// explicit per-pair records; Format 2 is class-based pair tables.
func parsePairPos(gpos []byte, off int, out map[[2]uint16]PairAdjustment) {
	if off+2 > len(gpos) {
		return
	}
	format := be16(gpos, off)
	switch format {
	case 1:
		parsePairPosFormat1(gpos, off, out)
	case 2:
		parsePairPosFormat2(gpos, off, out)
	}
}

// parsePairPosFormat1 reads an explicit-pair PairPos subtable.
//
// Layout:
//
//	posFormat       uint16 (==1)
//	coverageOffset  Offset16
//	valueFormat1    uint16
//	valueFormat2    uint16
//	pairSetCount    uint16
//	pairSetOffsets[pairSetCount] Offset16 (from subtable start)
//
// Each PairSet:
//
//	pairValueCount  uint16
//	pairValueRecords[pairValueCount] {
//	    secondGlyph uint16
//	    valueRecord1 (valueFormat1 shape)
//	    valueRecord2 (valueFormat2 shape)
//	}
//
// Only the XAdvance component of valueRecord1 is extracted: that is the
// horizontal kerning delta applied to the first glyph's advance.
func parsePairPosFormat1(gpos []byte, off int, out map[[2]uint16]PairAdjustment) {
	if off+10 > len(gpos) {
		return
	}
	coverageOff := off + int(be16(gpos, off+2))
	valueFormat1 := be16(gpos, off+4)
	valueFormat2 := be16(gpos, off+6)
	pairSetCount := int(be16(gpos, off+8))
	if off+10+pairSetCount*2 > len(gpos) {
		return
	}
	covered := parseCoverage(gpos, coverageOff)
	if covered == nil {
		return
	}

	size1 := valueRecordSize(valueFormat1)
	size2 := valueRecordSize(valueFormat2)
	recSize := 2 + size1 + size2

	for i, firstGID := range covered {
		if i >= pairSetCount {
			break
		}
		setOff := off + int(be16(gpos, off+10+i*2))
		if setOff+2 > len(gpos) {
			continue
		}
		pairValueCount := int(be16(gpos, setOff))
		if setOff+2+pairValueCount*recSize > len(gpos) {
			continue
		}
		for j := 0; j < pairValueCount; j++ {
			base := setOff + 2 + j*recSize
			secondGID := be16(gpos, base)
			xAdv := valueRecordXAdvance(gpos, base+2, valueFormat1)
			if xAdv == 0 {
				continue
			}
			out[[2]uint16{firstGID, secondGID}] = PairAdjustment{XAdvance: xAdv}
		}
	}
}

// parsePairPosFormat2 reads a class-based PairPos subtable.
//
// Layout:
//
//	posFormat       uint16 (==2)
//	coverageOffset  Offset16
//	valueFormat1    uint16
//	valueFormat2    uint16
//	classDef1Offset Offset16
//	classDef2Offset Offset16
//	class1Count     uint16
//	class2Count     uint16
//	class1Records[class1Count] of class2Records[class2Count] of
//	    { valueRecord1, valueRecord2 }
//
// The coverage table bounds which glyphs are eligible as the first of a
// pair; class 0 in ClassDef1 represents "covered but not otherwise
// classified". Second glyphs are classified by classDef2 over all glyphs
// in the font; class 0 there represents the implicit catch-all.
func parsePairPosFormat2(gpos []byte, off int, out map[[2]uint16]PairAdjustment) {
	if off+16 > len(gpos) {
		return
	}
	coverageOff := off + int(be16(gpos, off+2))
	valueFormat1 := be16(gpos, off+4)
	valueFormat2 := be16(gpos, off+6)
	classDef1Off := off + int(be16(gpos, off+8))
	classDef2Off := off + int(be16(gpos, off+10))
	class1Count := int(be16(gpos, off+12))
	class2Count := int(be16(gpos, off+14))

	covered := parseCoverage(gpos, coverageOff)
	if covered == nil {
		return
	}
	class1 := parseClassDef(gpos, classDef1Off)
	class2 := parseClassDef(gpos, classDef2Off)

	size1 := valueRecordSize(valueFormat1)
	size2 := valueRecordSize(valueFormat2)
	class2RecSize := size1 + size2
	class1RecSize := class2Count * class2RecSize
	total := class1Count * class1RecSize
	if off+16+total > len(gpos) {
		return
	}

	// Bucket second-glyph classes for efficient iteration.
	class2ToGIDs := make(map[uint16][]uint16)
	for gid, cls := range class2 {
		if int(cls) >= class2Count {
			continue
		}
		class2ToGIDs[cls] = append(class2ToGIDs[cls], gid)
	}

	for _, leftGID := range covered {
		c1 := class1[leftGID]
		if int(c1) >= class1Count {
			continue
		}
		rowOff := off + 16 + int(c1)*class1RecSize
		for c2 := 0; c2 < class2Count; c2++ {
			recOff := rowOff + c2*class2RecSize
			xAdv := valueRecordXAdvance(gpos, recOff, valueFormat1)
			if xAdv == 0 {
				continue
			}
			rights, ok := class2ToGIDs[uint16(c2)]
			if !ok {
				continue
			}
			for _, rightGID := range rights {
				out[[2]uint16{leftGID, rightGID}] = PairAdjustment{XAdvance: xAdv}
			}
		}
	}
}

// valueRecordSize returns the byte size of a ValueRecord whose fields
// are selected by the given ValueFormat bitmask. Each set bit contributes
// one int16 (two bytes) field; the field order is fixed per
// ISO 14496-22 §6.3: XPlacement, YPlacement, XAdvance, YAdvance,
// XPlaDevice, YPlaDevice, XAdvDevice, YAdvDevice.
func valueRecordSize(vf uint16) int {
	n := 0
	for b := uint16(1); b != 0 && b <= 0x80; b <<= 1 {
		if vf&b != 0 {
			n++
		}
	}
	return n * 2
}

// valueRecordXAdvance reads the XAdvance int16 out of a ValueRecord
// located at off, given its ValueFormat bitmask. Returns 0 if the bit is
// clear or the field lies beyond the data end.
//
// Field order per ISO 14496-22 §6.3 Table "ValueRecord":
//
//	bit 0 XPlacement, bit 1 YPlacement, bit 2 XAdvance, bit 3 YAdvance,
//	bit 4 XPlaDevice, bit 5 YPlaDevice, bit 6 XAdvDevice, bit 7 YAdvDevice.
func valueRecordXAdvance(data []byte, off int, vf uint16) int16 {
	if vf&0x0004 == 0 {
		return 0
	}
	// Count the number of int16 fields preceding XAdvance.
	pos := 0
	if vf&0x0001 != 0 {
		pos++ // XPlacement
	}
	if vf&0x0002 != 0 {
		pos++ // YPlacement
	}
	fieldOff := off + pos*2
	if fieldOff+2 > len(data) {
		return 0
	}
	return int16(be16(data, fieldOff))
}

// parseMarkBasePos reads a MarkBasePosFormat1 subtable.
//
// Layout:
//
//	posFormat        uint16 (==1)
//	markCoverageOff  Offset16
//	baseCoverageOff  Offset16
//	markClassCount   uint16
//	markArrayOff     Offset16
//	baseArrayOff     Offset16
//
// The MarkArray and BaseArray are parsed through parseMarkArray and
// parseBaseArray. Marks and bases are keyed by glyph ID in the returned
// maps; look-up with MarkOffset combines them via mark class.
func parseMarkBasePos(gpos []byte, off int,
	marks map[uint16]MarkRecord,
	bases map[uint16]BaseRecord,
) {
	if off+12 > len(gpos) {
		return
	}
	format := be16(gpos, off)
	if format != 1 {
		return
	}
	markCoverageOff := off + int(be16(gpos, off+2))
	baseCoverageOff := off + int(be16(gpos, off+4))
	markClassCount := int(be16(gpos, off+6))
	markArrayOff := off + int(be16(gpos, off+8))
	baseArrayOff := off + int(be16(gpos, off+10))

	markCov := parseCoverage(gpos, markCoverageOff)
	baseCov := parseCoverage(gpos, baseCoverageOff)
	if markCov == nil || baseCov == nil {
		return
	}

	parseMarkArray(gpos, markArrayOff, markCov, marks)
	parseBaseArray(gpos, baseArrayOff, baseCov, markClassCount, bases)
}

// parseMarkArray reads a MarkArray and populates marks.
//
// Layout:
//
//	markCount         uint16
//	markRecords[markCount] {
//	    markClass       uint16
//	    markAnchorOff   Offset16 (from MarkArray start)
//	}
//
// Each index in markRecords corresponds to the coverage entry at the
// same coverage index, so markCov is indexed positionally.
func parseMarkArray(gpos []byte, off int, markCov []uint16, out map[uint16]MarkRecord) {
	if off+2 > len(gpos) {
		return
	}
	count := int(be16(gpos, off))
	if off+2+count*4 > len(gpos) {
		return
	}
	for i := 0; i < count && i < len(markCov); i++ {
		rec := off + 2 + i*4
		class := be16(gpos, rec)
		anchorOff := off + int(be16(gpos, rec+2))
		anchor, ok := parseAnchor(gpos, anchorOff)
		if !ok {
			continue
		}
		out[markCov[i]] = MarkRecord{Class: class, Anchor: anchor}
	}
}

// parseBaseArray reads a BaseArray and populates bases.
//
// Layout:
//
//	baseCount     uint16
//	baseRecords[baseCount] {
//	    baseAnchorOffsets[markClassCount] Offset16 (from BaseArray start)
//	}
//
// A zero anchor offset means "no anchor for this class"; such slots are
// left as the zero Anchor in the BaseRecord's Anchors slice.
func parseBaseArray(gpos []byte, off int, baseCov []uint16, markClassCount int, out map[uint16]BaseRecord) {
	if off+2 > len(gpos) {
		return
	}
	count := int(be16(gpos, off))
	rowSize := markClassCount * 2
	if off+2+count*rowSize > len(gpos) {
		return
	}
	for i := 0; i < count && i < len(baseCov); i++ {
		rowOff := off + 2 + i*rowSize
		anchors := make([]Anchor, markClassCount)
		for c := 0; c < markClassCount; c++ {
			aOffRaw := int(be16(gpos, rowOff+c*2))
			if aOffRaw == 0 {
				continue
			}
			anchor, ok := parseAnchor(gpos, off+aOffRaw)
			if !ok {
				continue
			}
			anchors[c] = anchor
		}
		out[baseCov[i]] = BaseRecord{Anchors: anchors}
	}
}

// parseAnchor reads an Anchor table at off. All three formats share the
// same first six bytes (format, x, y); formats 2 and 3 carry trailing
// fields (an anchor-point index, or Device table offsets) that this
// iteration deliberately ignores.
func parseAnchor(data []byte, off int) (Anchor, bool) {
	if off+6 > len(data) {
		return Anchor{}, false
	}
	format := be16(data, off)
	if format < 1 || format > 3 {
		return Anchor{}, false
	}
	x := int16(be16(data, off+2))
	y := int16(be16(data, off+4))
	return Anchor{X: x, Y: y}, true
}
