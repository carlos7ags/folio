// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
)

// GSUBFeature identifies an OpenType GSUB feature tag.
type GSUBFeature string

const (
	GSUBInit GSUBFeature = "init" // initial form
	GSUBMedi GSUBFeature = "medi" // medial form
	GSUBFina GSUBFeature = "fina" // final form
	GSUBIsol GSUBFeature = "isol" // isolated form
	GSUBLiga GSUBFeature = "liga" // standard ligatures
	GSUBRlig GSUBFeature = "rlig" // required ligatures
	GSUBClig GSUBFeature = "clig" // contextual ligatures
	GSUBCalt GSUBFeature = "calt" // contextual alternates
)

// LigatureSubst describes a single ligature substitution: a sequence of
// component glyph IDs (after the first) that, together with the first
// component used as the lookup key, are replaced by LigatureGID.
type LigatureSubst struct {
	Components  []uint16 // component GIDs after the first (may be empty)
	LigatureGID uint16
}

// ChainSubstAction is one SubstLookupRecord entry inside a chaining
// contextual substitution rule: at a given position within the matched
// input sequence, invoke another GSUB lookup.
type ChainSubstAction struct {
	SequenceIndex   uint16 // zero-based position in the input sequence
	LookupListIndex uint16 // index into the GSUB LookupList
}

// ChainContextSubst is the unified runtime representation of an OpenType
// LookupType 6 rule. All three subtable formats (simple/class/coverage)
// are decompressed at parse time into this shape: each element of the
// Backtrack, Input, and Lookahead slices is a set of acceptable GIDs at
// that position. The Backtrack slice is stored in reverse order so that
// Backtrack[0] is the glyph immediately preceding Input[0]. Input[0] is
// the trigger glyph: the outer apply loop uses it as the dispatch key.
//
// Reference: ISO 14496-22 §6.2 LookupType 6.
type ChainContextSubst struct {
	Backtrack [][]uint16
	Input     [][]uint16
	Lookahead [][]uint16
	Actions   []ChainSubstAction
}

// GSUBSubstitutions holds parsed GSUB lookups grouped by feature tag.
//
// Single holds LookupType 1 substitutions: a per-feature map from source
// glyph ID to replacement glyph ID.
//
// Ligature holds LookupType 4 substitutions: a per-feature map keyed by
// the first component glyph ID to a slice of candidate ligatures sharing
// that prefix. Slices are ordered so that longest matches appear first,
// which matches the OpenType greedy matching rule.
//
// ChainContext holds LookupType 6 chaining contextual substitutions: a
// per-feature map keyed by the trigger glyph (Input[0]) to the set of
// rules that may fire when that glyph is seen.
type GSUBSubstitutions struct {
	Single       map[GSUBFeature]map[uint16]uint16
	Ligature     map[GSUBFeature]map[uint16][]LigatureSubst
	ChainContext map[GSUBFeature]map[uint16][]ChainContextSubst

	// lookupTable is indexed by GSUB LookupList slot and carries a
	// per-lookup record so that ChainContext actions can dispatch to any
	// lookup slot referenced by a SubstLookupRecord, regardless of its
	// lookup type. Nil entries mean "lookup slot has no subtable shape
	// we understand as a dispatch target". See lookupRecord for the
	// per-type payload.
	lookupTable []*lookupRecord
}

// lookupRecord is the per-slot dispatch payload used by ChainContext
// action recursion. Type carries the GSUB LookupType after any LookupType
// 7 (Extension) unwrap; exactly one of Single/Lig/Chain is populated to
// match Type.
type lookupRecord struct {
	Type   uint16
	Single map[uint16]uint16
	Lig    map[uint16][]LigatureSubst
	Chain  map[uint16][]ChainContextSubst
}

// maxChainDepth bounds recursive ChainContext action dispatch. OpenType
// does not specify a limit; in practice fonts use shallow chains and
// widely deployed shapers cap recursion at 64 to prevent stack overflow
// from pathological or adversarial rule sets. We use the same bound.
const maxChainDepth = 64

// ParseGSUB reads the GSUB table from raw TrueType/OpenType font bytes
// and extracts Single (LookupType 1) and Ligature (LookupType 4)
// substitutions for the Arabic positional features, the standard Latin
// ligature features, and required/contextual ligatures.
//
// Script selection: "arab", "latn", and "DFLT" (in that preference order
// for the default LangSys). Extension lookups (LookupType 7) are
// unwrapped transparently.
//
// Returns nil if the font has no GSUB table or no matching features.
//
// Reference: ISO 14496-22 §6.2, OpenType GSUB table.
func ParseGSUB(data []byte) *GSUBSubstitutions {
	gsub := findTable(data, "GSUB")
	if gsub == nil {
		return nil
	}
	if len(gsub) < 10 {
		return nil
	}

	scriptListOff := int(be16(gsub, 4))
	featureListOff := int(be16(gsub, 6))
	lookupListOff := int(be16(gsub, 8))

	if scriptListOff >= len(gsub) || featureListOff >= len(gsub) || lookupListOff >= len(gsub) {
		return nil
	}

	featureIndices := scriptFeatureIndices(gsub, scriptListOff)
	if len(featureIndices) == 0 {
		return nil
	}

	targetTags := map[string]GSUBFeature{
		"init": GSUBInit,
		"medi": GSUBMedi,
		"fina": GSUBFina,
		"isol": GSUBIsol,
		"liga": GSUBLiga,
		"rlig": GSUBRlig,
		"clig": GSUBClig,
		"calt": GSUBCalt,
	}
	featureToLookups := matchFeatures(gsub, featureListOff, featureIndices, targetTags)
	if len(featureToLookups) == 0 {
		return nil
	}

	// Pre-scan the full LookupList once to build the internal per-slot
	// dispatch table used by ChainContext action recursion. This table
	// is sparse: a slot is left nil unless it carries a subtable shape
	// we can dispatch into (LookupType 1, 4, or 6; LookupType 7 is
	// unwrapped).
	lookupTable := buildLookupDispatchTable(gsub, lookupListOff)

	result := &GSUBSubstitutions{
		Single:       make(map[GSUBFeature]map[uint16]uint16),
		Ligature:     make(map[GSUBFeature]map[uint16][]LigatureSubst),
		ChainContext: make(map[GSUBFeature]map[uint16][]ChainContextSubst),
		lookupTable:  lookupTable,
	}
	for feat, lookupIndices := range featureToLookups {
		single := make(map[uint16]uint16)
		lig := make(map[uint16][]LigatureSubst)
		chain := make(map[uint16][]ChainContextSubst)
		parseLookups(gsub, lookupListOff, lookupIndices, single, lig, chain)
		if len(single) > 0 {
			result.Single[feat] = single
		}
		if len(lig) > 0 {
			// Order each bucket so longest component sequences come first
			// so ApplyLigature's greedy left-to-right scan produces the
			// longest match per ISO 14496-22 §6.2.
			for k := range lig {
				sortLigsByLenDesc(lig[k])
			}
			result.Ligature[feat] = lig
		}
		if len(chain) > 0 {
			result.ChainContext[feat] = chain
		}
	}
	if len(result.Single) == 0 && len(result.Ligature) == 0 && len(result.ChainContext) == 0 {
		return nil
	}
	return result
}

// ApplyLigature scans gids left-to-right and replaces the longest matching
// ligature sequence with the ligature glyph. Greedy longest-match per
// ISO 14496-22 §6.2. Returns a new slice; the input is not modified.
func (g *GSUBSubstitutions) ApplyLigature(gids []uint16, feature GSUBFeature) []uint16 {
	if g == nil || len(g.Ligature) == 0 || len(gids) == 0 {
		return gids
	}
	table, ok := g.Ligature[feature]
	if !ok || len(table) == 0 {
		return gids
	}
	out := make([]uint16, len(gids))
	copy(out, gids)
	i := 0
	for i < len(out) {
		next, consumed := applyLigatureAt(out, i, table)
		if consumed > 0 {
			out = next
			i++
			continue
		}
		i++
	}
	return out
}

// applyLigatureAt tests the ligature table at position pos and, on a
// longest greedy match, splices the matched components out of gids and
// inserts the ligature GID in place. The returned slice may alias gids
// on no match, or be shorter than gids when a multi-component ligature
// fires. consumed is the number of input positions that were collapsed
// (0 on no match, 1+len(components) on a match).
func applyLigatureAt(gids []uint16, pos int, table map[uint16][]LigatureSubst) ([]uint16, int) {
	if pos < 0 || pos >= len(gids) {
		return gids, 0
	}
	candidates := table[gids[pos]]
	for _, cand := range candidates {
		need := len(cand.Components)
		if pos+1+need > len(gids) {
			continue
		}
		matched := true
		for j := 0; j < need; j++ {
			if gids[pos+1+j] != cand.Components[j] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		// Splice: replace [pos : pos+1+need] with a single GID.
		gids[pos] = cand.LigatureGID
		if need > 0 {
			gids = append(gids[:pos+1], gids[pos+1+need:]...)
		}
		return gids, 1 + need
	}
	return gids, 0
}

// sortLigsByLenDesc sorts ligatures so that longer component sequences
// come first. Insertion sort is used to keep the implementation tiny and
// because ligature buckets are typically small (single digits).
func sortLigsByLenDesc(s []LigatureSubst) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j].Components) > len(s[j-1].Components); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// findTable locates a TrueType/OpenType table by its 4-byte tag in the
// raw font bytes and returns the table's data slice. Returns nil if not found.
func findTable(data []byte, tag string) []byte {
	if len(data) < 12 {
		return nil
	}
	// Handle TrueType Collections (TTC): use the first font.
	if len(data) >= 12 && string(data[:4]) == "ttcf" {
		if len(data) < 16 {
			return nil
		}
		numFonts := int(be32(data, 8))
		if numFonts < 1 || len(data) < 12+4 {
			return nil
		}
		offset := int(be32(data, 12))
		if offset >= len(data) {
			return nil
		}
		data = data[offset:]
	}
	numTables := int(be16(data, 4))
	if len(data) < 12+numTables*16 {
		return nil
	}
	tagBytes := []byte(tag)
	for i := 0; i < numTables; i++ {
		entry := data[12+i*16:]
		if entry[0] == tagBytes[0] && entry[1] == tagBytes[1] &&
			entry[2] == tagBytes[2] && entry[3] == tagBytes[3] {
			offset := int(be32(entry, 8))
			length := int(be32(entry, 12))
			if offset+length > len(data) {
				return nil
			}
			return data[offset : offset+length]
		}
	}
	return nil
}

// scriptFeatureIndices finds the feature indices referenced by the "arab"
// script, then "latn", then "DFLT" fallback in the GSUB ScriptList.
func scriptFeatureIndices(gsub []byte, off int) []int {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*6 > len(gsub) {
		return nil
	}

	// Collect LangSys offsets from all preferred scripts so a font that
	// only lists "latn" still contributes its Latin ligature features,
	// while "arab" contributes Arabic positional lookups. Duplicates are
	// folded in matchFeatures via the allowed set.
	var langSysOffs []int
	var dfltOff int
	dfltFound := false
	for i := 0; i < count; i++ {
		rec := gsub[off+2+i*6:]
		tag := string(rec[:4])
		scriptOff := off + int(be16(rec, 4))
		switch tag {
		case "arab", "latn":
			langSysOffs = append(langSysOffs, scriptOff)
		case "DFLT":
			dfltOff = scriptOff
			dfltFound = true
		}
	}
	if len(langSysOffs) == 0 && dfltFound {
		langSysOffs = append(langSysOffs, dfltOff)
	}
	if len(langSysOffs) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	var indices []int
	for _, langSysOff := range langSysOffs {
		if langSysOff+2 > len(gsub) {
			continue
		}
		defOff := int(be16(gsub, langSysOff))
		if defOff == 0 {
			continue
		}
		langSys := langSysOff + defOff
		if langSys+6 > len(gsub) {
			continue
		}
		featureCount := int(be16(gsub, langSys+4))
		if langSys+6+featureCount*2 > len(gsub) {
			continue
		}
		for i := 0; i < featureCount; i++ {
			idx := int(be16(gsub, langSys+6+i*2))
			if !seen[idx] {
				seen[idx] = true
				indices = append(indices, idx)
			}
		}
	}
	return indices
}

// matchFeatures scans the FeatureList for features matching targetTags
// whose indices appear in allowed. Returns a map from GSUBFeature to
// the lookup indices referenced by that feature.
func matchFeatures(gsub []byte, off int, allowed []int, targetTags map[string]GSUBFeature) map[GSUBFeature][]int {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*6 > len(gsub) {
		return nil
	}
	allowSet := make(map[int]bool, len(allowed))
	for _, idx := range allowed {
		allowSet[idx] = true
	}
	result := make(map[GSUBFeature][]int)
	for i := 0; i < count; i++ {
		if !allowSet[i] {
			continue
		}
		rec := gsub[off+2+i*6:]
		feat, ok := targetTags[string(rec[:4])]
		if !ok {
			continue
		}
		featureOff := off + int(be16(rec, 4))
		if featureOff+4 > len(gsub) {
			continue
		}
		lookupCount := int(be16(gsub, featureOff+2))
		if featureOff+4+lookupCount*2 > len(gsub) {
			continue
		}
		lookups := make([]int, lookupCount)
		for j := 0; j < lookupCount; j++ {
			lookups[j] = int(be16(gsub, featureOff+4+j*2))
		}
		result[feat] = append(result[feat], lookups...)
	}
	return result
}

// parseLookups walks each referenced lookup and dispatches its subtables
// to the appropriate LookupType parser. Extension lookups (type 7) are
// unwrapped; nested extensions are not expected by the spec and are
// ignored if encountered.
func parseLookups(gsub []byte, listOff int, indices []int, single map[uint16]uint16, lig map[uint16][]LigatureSubst, chain map[uint16][]ChainContextSubst) {
	if listOff+2 > len(gsub) {
		return
	}
	count := int(be16(gsub, listOff))
	for _, idx := range indices {
		if idx >= count {
			continue
		}
		lookupOff := listOff + int(be16(gsub, listOff+2+idx*2))
		parseLookup(gsub, lookupOff, single, lig, chain)
	}
}

// parseLookup reads a single Lookup table, following each subtable offset
// and calling the appropriate subtable parser for supported lookup types.
func parseLookup(gsub []byte, lookupOff int, single map[uint16]uint16, lig map[uint16][]LigatureSubst, chain map[uint16][]ChainContextSubst) {
	if lookupOff+6 > len(gsub) {
		return
	}
	lookupType := be16(gsub, lookupOff)
	subCount := int(be16(gsub, lookupOff+4))
	if lookupOff+6+subCount*2 > len(gsub) {
		return
	}
	for si := 0; si < subCount; si++ {
		subOff := lookupOff + int(be16(gsub, lookupOff+6+si*2))
		switch lookupType {
		case 1:
			parseSingleSubst(gsub, subOff, single)
		case 4:
			parseLigatureSubst(gsub, subOff, lig)
		case 6:
			parseChainContextSubst(gsub, subOff, chain)
		case 7:
			// Extension table: format(2), extensionLookupType(2),
			// extensionOffset(4, relative to the extension subtable start).
			if subOff+8 > len(gsub) {
				continue
			}
			extType := be16(gsub, subOff+2)
			extOff := subOff + int(be32(gsub, subOff+4))
			if extOff >= len(gsub) {
				continue
			}
			switch extType {
			case 1:
				parseSingleSubst(gsub, extOff, single)
			case 4:
				parseLigatureSubst(gsub, extOff, lig)
			case 6:
				parseChainContextSubst(gsub, extOff, chain)
			}
		}
	}
}

// buildLookupDispatchTable walks every lookup in the GSUB LookupList
// once and builds a per-slot dispatch record. For each supported lookup
// type it populates the matching map on a lookupRecord: Single for
// LookupType 1, Lig for LookupType 4, Chain for LookupType 6. LookupType
// 7 (Extension) is unwrapped and the extension's effective type drives
// the target map. The returned slice is indexed by lookupListIndex.
// Slots carrying no understood subtable shape receive a nil entry.
func buildLookupDispatchTable(gsub []byte, listOff int) []*lookupRecord {
	if listOff+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, listOff))
	if listOff+2+count*2 > len(gsub) {
		return nil
	}
	out := make([]*lookupRecord, count)
	for i := 0; i < count; i++ {
		lookupOff := listOff + int(be16(gsub, listOff+2+i*2))
		if lookupOff+6 > len(gsub) {
			continue
		}
		lookupType := be16(gsub, lookupOff)
		subCount := int(be16(gsub, lookupOff+4))
		if lookupOff+6+subCount*2 > len(gsub) {
			continue
		}
		rec := &lookupRecord{}
		for si := 0; si < subCount; si++ {
			subOff := lookupOff + int(be16(gsub, lookupOff+6+si*2))
			effType := lookupType
			effOff := subOff
			if lookupType == 7 {
				if subOff+8 > len(gsub) {
					continue
				}
				effType = be16(gsub, subOff+2)
				effOff = subOff + int(be32(gsub, subOff+4))
				if effOff >= len(gsub) {
					continue
				}
			}
			switch effType {
			case 1:
				if rec.Single == nil {
					rec.Single = make(map[uint16]uint16)
				}
				parseSingleSubst(gsub, effOff, rec.Single)
			case 4:
				if rec.Lig == nil {
					rec.Lig = make(map[uint16][]LigatureSubst)
				}
				parseLigatureSubst(gsub, effOff, rec.Lig)
			case 6:
				if rec.Chain == nil {
					rec.Chain = make(map[uint16][]ChainContextSubst)
				}
				parseChainContextSubst(gsub, effOff, rec.Chain)
			}
		}
		// Pick a dispatch type. We only support one per slot; mixing
		// types in a single lookup is not allowed by the spec anyway
		// (every subtable in a lookup must share the LookupType).
		switch {
		case len(rec.Single) > 0:
			rec.Type = 1
			out[i] = rec
		case len(rec.Lig) > 0:
			// Sort each bucket longest-first to match ApplyLigature's
			// greedy-longest scan.
			for k := range rec.Lig {
				sortLigsByLenDesc(rec.Lig[k])
			}
			rec.Type = 4
			out[i] = rec
		case len(rec.Chain) > 0:
			rec.Type = 6
			out[i] = rec
		}
	}
	return out
}

// parseSingleSubst reads a SingleSubstitution subtable (format 1 or 2)
// and adds entries to the substitution map.
func parseSingleSubst(gsub []byte, off int, subs map[uint16]uint16) {
	if off+6 > len(gsub) {
		return
	}
	format := be16(gsub, off)
	coverageOff := off + int(be16(gsub, off+2))

	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}

	switch format {
	case 1:
		delta := int16(be16(gsub, off+4))
		for _, gid := range covered {
			subs[gid] = uint16(int16(gid) + delta)
		}
	case 2:
		substCount := int(be16(gsub, off+4))
		if off+6+substCount*2 > len(gsub) {
			return
		}
		for i, gid := range covered {
			if i >= substCount {
				break
			}
			subs[gid] = be16(gsub, off+6+i*2)
		}
	}
}

// parseLigatureSubst reads a LigatureSubstFormat1 subtable and appends
// every ligature into lig keyed by its first component.
//
// Subtable layout (ISO 14496-22 §6.2 LookupType 4):
//
//	format           uint16  (always 1)
//	coverageOffset   Offset16
//	ligatureSetCount uint16
//	ligatureSetOffsets[ligatureSetCount] Offset16
//
// Each LigatureSet:
//
//	ligatureCount      uint16
//	ligatureOffsets[]  Offset16 (relative to LigatureSet)
//
// Each Ligature:
//
//	ligatureGlyph      uint16
//	componentCount     uint16
//	componentGlyphIDs[componentCount-1] uint16
func parseLigatureSubst(gsub []byte, off int, lig map[uint16][]LigatureSubst) {
	if off+6 > len(gsub) {
		return
	}
	format := be16(gsub, off)
	if format != 1 {
		return
	}
	coverageOff := off + int(be16(gsub, off+2))
	ligSetCount := int(be16(gsub, off+4))
	if off+6+ligSetCount*2 > len(gsub) {
		return
	}
	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}
	for i, firstGID := range covered {
		if i >= ligSetCount {
			break
		}
		setOff := off + int(be16(gsub, off+6+i*2))
		if setOff+2 > len(gsub) {
			continue
		}
		ligCount := int(be16(gsub, setOff))
		if setOff+2+ligCount*2 > len(gsub) {
			continue
		}
		for j := 0; j < ligCount; j++ {
			ligOff := setOff + int(be16(gsub, setOff+2+j*2))
			if ligOff+4 > len(gsub) {
				continue
			}
			ligGlyph := be16(gsub, ligOff)
			compCount := int(be16(gsub, ligOff+2))
			if compCount == 0 {
				continue
			}
			rest := compCount - 1
			if ligOff+4+rest*2 > len(gsub) {
				continue
			}
			var comps []uint16
			if rest > 0 {
				comps = make([]uint16, rest)
				for k := 0; k < rest; k++ {
					comps[k] = be16(gsub, ligOff+4+k*2)
				}
			}
			lig[firstGID] = append(lig[firstGID], LigatureSubst{
				Components:  comps,
				LigatureGID: ligGlyph,
			})
		}
	}
}

// parseCoverage reads a Coverage table and returns the list of covered
// glyph IDs in coverage index order.
func parseCoverage(gsub []byte, off int) []uint16 {
	if off+4 > len(gsub) {
		return nil
	}
	format := be16(gsub, off)
	switch format {
	case 1:
		count := int(be16(gsub, off+2))
		if off+4+count*2 > len(gsub) {
			return nil
		}
		result := make([]uint16, count)
		for i := 0; i < count; i++ {
			result[i] = be16(gsub, off+4+i*2)
		}
		return result
	case 2:
		// Format 2: RangeRecord[] where each record gives
		// startGlyphID, endGlyphID, startCoverageIndex. The coverage
		// index order is the one implied by startCoverageIndex, so
		// ranges must be placed at their declared index to preserve the
		// correspondence with Format 1 used by callers that index the
		// returned slice positionally (e.g. LigatureSubstFormat1).
		rangeCount := int(be16(gsub, off+2))
		if off+4+rangeCount*6 > len(gsub) {
			return nil
		}
		// First pass: compute total length from the highest end index.
		total := 0
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(gsub, rec)
			endGID := be16(gsub, rec+2)
			startCov := int(be16(gsub, rec+4))
			end := startCov + int(endGID-startGID) + 1
			if end > total {
				total = end
			}
		}
		result := make([]uint16, total)
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(gsub, rec)
			endGID := be16(gsub, rec+2)
			startCov := int(be16(gsub, rec+4))
			for gid := startGID; gid <= endGID; gid++ {
				idx := startCov + int(gid-startGID)
				if idx < len(result) {
					result[idx] = gid
				}
			}
		}
		return result
	}
	return nil
}

// parseClassDef reads an OpenType ClassDef table (format 1 or 2) and
// returns a map from glyph ID to class number. Glyphs not present in
// the table map to class 0 per ISO 14496-22 §3. Returns nil on malformed
// input. Callers should treat a missing entry the same as class 0.
func parseClassDef(data []byte, off int) map[uint16]uint16 {
	if off+2 > len(data) {
		return nil
	}
	format := be16(data, off)
	out := make(map[uint16]uint16)
	switch format {
	case 1:
		if off+6 > len(data) {
			return nil
		}
		startGID := be16(data, off+2)
		count := int(be16(data, off+4))
		if off+6+count*2 > len(data) {
			return nil
		}
		for i := 0; i < count; i++ {
			cls := be16(data, off+6+i*2)
			if cls != 0 {
				out[startGID+uint16(i)] = cls
			}
		}
	case 2:
		if off+4 > len(data) {
			return nil
		}
		rangeCount := int(be16(data, off+2))
		if off+4+rangeCount*6 > len(data) {
			return nil
		}
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(data, rec)
			endGID := be16(data, rec+2)
			cls := be16(data, rec+4)
			if cls == 0 {
				continue
			}
			for gid := int(startGID); gid <= int(endGID); gid++ {
				out[uint16(gid)] = cls
			}
		}
	default:
		return nil
	}
	return out
}

// classMembers inverts a class map: for the given class number it
// returns every GID mapped to that class. For class 0 it returns nil,
// since class 0 is "everything not otherwise listed" and enumerating it
// would require the full glyph space; ChainContext parsing treats
// class-0 positions specially (see parseChainContextFormat2).
func classMembers(classMap map[uint16]uint16, class uint16) []uint16 {
	if class == 0 || len(classMap) == 0 {
		return nil
	}
	var out []uint16
	for gid, cls := range classMap {
		if cls == class {
			out = append(out, gid)
		}
	}
	return out
}

// parseChainContextSubst dispatches a ChainContextSubst subtable by
// format. All three formats compile into the uniform ChainContextSubst
// runtime representation keyed by the trigger GID (Input[0]).
func parseChainContextSubst(gsub []byte, off int, chain map[uint16][]ChainContextSubst) {
	if off+2 > len(gsub) {
		return
	}
	format := be16(gsub, off)
	switch format {
	case 1:
		parseChainContextFormat1(gsub, off, chain)
	case 2:
		parseChainContextFormat2(gsub, off, chain)
	case 3:
		parseChainContextFormat3(gsub, off, chain)
	}
}

// parseChainContextFormat1 handles ChainContextSubstFormat1:
//
//	format(2) = 1
//	coverageOff(2)
//	chainSubRuleSetCount(2)
//	chainSubRuleSetOffsets[chainSubRuleSetCount] (2 each, relative to subtable)
//
// Each ChainSubRuleSet is parallel to Coverage: set[i] applies to
// coverage[i]. Each set holds ChainSubRules with explicit GID sequences.
func parseChainContextFormat1(gsub []byte, off int, chain map[uint16][]ChainContextSubst) {
	if off+6 > len(gsub) {
		return
	}
	coverageOff := off + int(be16(gsub, off+2))
	setCount := int(be16(gsub, off+4))
	if off+6+setCount*2 > len(gsub) {
		return
	}
	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}
	for i, firstGID := range covered {
		if i >= setCount {
			break
		}
		setOff := off + int(be16(gsub, off+6+i*2))
		if setOff+2 > len(gsub) {
			continue
		}
		ruleCount := int(be16(gsub, setOff))
		if setOff+2+ruleCount*2 > len(gsub) {
			continue
		}
		for r := 0; r < ruleCount; r++ {
			ruleOff := setOff + int(be16(gsub, setOff+2+r*2))
			rule := parseChainSubRule(gsub, ruleOff, firstGID)
			if rule != nil {
				chain[firstGID] = append(chain[firstGID], *rule)
			}
		}
	}
}

// parseChainSubRule reads a ChainSubRule:
//
//	backtrackGlyphCount(2) + backtrackSequence[backtrackGlyphCount](2 each)
//	inputGlyphCount(2)     + inputSequence[inputGlyphCount-1](2 each)
//	lookaheadGlyphCount(2) + lookaheadSequence[lookaheadGlyphCount](2 each)
//	substCount(2)          + substLookupRecords[substCount](4 each)
//
// The input sequence in the on-disk representation omits the first glyph
// (it came from Coverage); here we prepend firstGID so that Input[0]
// is the trigger and ApplyChainContext can uniformly use it as the key.
// Backtrack is stored reversed per the ChainContextSubst convention.
func parseChainSubRule(gsub []byte, off int, firstGID uint16) *ChainContextSubst {
	if off+2 > len(gsub) {
		return nil
	}
	p := off
	backCount := int(be16(gsub, p))
	p += 2
	if p+backCount*2 > len(gsub) {
		return nil
	}
	backtrack := make([][]uint16, backCount)
	for i := 0; i < backCount; i++ {
		// Spec order is nearest-to-farthest from the input start, which
		// already matches our "reversed" convention (Backtrack[0] is the
		// glyph immediately preceding Input[0]).
		backtrack[i] = []uint16{be16(gsub, p+i*2)}
	}
	p += backCount * 2
	if p+2 > len(gsub) {
		return nil
	}
	inputCount := int(be16(gsub, p))
	p += 2
	if inputCount < 1 {
		return nil
	}
	restInput := inputCount - 1
	if p+restInput*2 > len(gsub) {
		return nil
	}
	input := make([][]uint16, inputCount)
	input[0] = []uint16{firstGID}
	for i := 0; i < restInput; i++ {
		input[i+1] = []uint16{be16(gsub, p+i*2)}
	}
	p += restInput * 2
	if p+2 > len(gsub) {
		return nil
	}
	lookCount := int(be16(gsub, p))
	p += 2
	if p+lookCount*2 > len(gsub) {
		return nil
	}
	lookahead := make([][]uint16, lookCount)
	for i := 0; i < lookCount; i++ {
		lookahead[i] = []uint16{be16(gsub, p+i*2)}
	}
	p += lookCount * 2
	actions := parseSubstLookupRecords(gsub, p)
	return &ChainContextSubst{
		Backtrack: backtrack,
		Input:     input,
		Lookahead: lookahead,
		Actions:   actions,
	}
}

// parseChainContextFormat2 handles ChainContextSubstFormat2:
//
//	format(2) = 2
//	coverageOff(2)
//	backtrackClassDefOff(2)
//	inputClassDefOff(2)
//	lookaheadClassDefOff(2)
//	chainSubClassSetCount(2)
//	chainSubClassSetOffsets[chainSubClassSetCount](2 each)
//
// Each chainSubClassSet corresponds to one input class. Class 0 in the
// input class definition is "any glyph not explicitly assigned", and
// the spec allows a class-set offset of 0 to mean "no rules for this
// class". Inside each set, ChainSubClassRule carries class numbers
// instead of explicit GIDs for its back/input/lookahead sequences.
//
// At parse time we invert each referenced class back to the list of
// actual GIDs in that class so the runtime ChainContextSubst can
// uniformly test candidates. Backtrack and lookahead class-0 entries
// become empty GID sets, which ApplyChainContext treats as "matches
// any GID not listed in any non-zero class at that position"; since we
// don't carry the full glyph space, we emit the special empty set and
// the matcher interprets an empty set as a wildcard.
func parseChainContextFormat2(gsub []byte, off int, chain map[uint16][]ChainContextSubst) {
	if off+12 > len(gsub) {
		return
	}
	coverageOff := off + int(be16(gsub, off+2))
	backClassOff := off + int(be16(gsub, off+4))
	inputClassOff := off + int(be16(gsub, off+6))
	lookClassOff := off + int(be16(gsub, off+8))
	setCount := int(be16(gsub, off+10))
	if off+12+setCount*2 > len(gsub) {
		return
	}
	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}

	// Missing ClassDef offsets (value 0 in the spec) mean "no classes",
	// which parseClassDef will read as a malformed offset. We detect that
	// case via the raw offset value before turning it into an absolute
	// position.
	var backClass, inputClass, lookClass map[uint16]uint16
	if be16(gsub, off+4) != 0 {
		backClass = parseClassDef(gsub, backClassOff)
	}
	if be16(gsub, off+6) != 0 {
		inputClass = parseClassDef(gsub, inputClassOff)
	}
	if be16(gsub, off+8) != 0 {
		lookClass = parseClassDef(gsub, lookClassOff)
	}
	if inputClass == nil {
		inputClass = map[uint16]uint16{}
	}

	// Every covered GID shares the whole set of class-based rules, but
	// rules only fire when the input classes align. We expand the rules
	// per trigger GID by keying on the input's first-class members.
	for cls := 0; cls < setCount; cls++ {
		setRel := int(be16(gsub, off+12+cls*2))
		if setRel == 0 {
			continue
		}
		setOff := off + setRel
		if setOff+2 > len(gsub) {
			continue
		}
		ruleCount := int(be16(gsub, setOff))
		if setOff+2+ruleCount*2 > len(gsub) {
			continue
		}
		// Members of this input class that are ALSO in Coverage: those
		// are the GIDs that can actually trigger the rule. Coverage
		// bounds the set since Format 2 only applies at coverage hits.
		triggerGIDs := intersectCoverageWithClass(covered, inputClass, uint16(cls))
		if len(triggerGIDs) == 0 {
			continue
		}
		for r := 0; r < ruleCount; r++ {
			ruleOff := setOff + int(be16(gsub, setOff+2+r*2))
			rule := parseChainSubClassRule(gsub, ruleOff, backClass, inputClass, lookClass, uint16(cls), covered)
			if rule == nil {
				continue
			}
			for _, trig := range triggerGIDs {
				// Each trigger GID gets its own rule whose Input[0] is
				// fixed to that specific trigger — this keeps the outer
				// ApplyChainContext dispatch simple even though the
				// underlying rule was class-based.
				copyRule := cloneChainRuleWithTrigger(rule, trig)
				chain[trig] = append(chain[trig], copyRule)
			}
		}
	}
}

// intersectCoverageWithClass returns covered GIDs whose class in
// classMap is exactly cls. When cls is 0 the result is the covered
// GIDs that are NOT in any non-zero class.
func intersectCoverageWithClass(covered []uint16, classMap map[uint16]uint16, cls uint16) []uint16 {
	var out []uint16
	for _, gid := range covered {
		gidCls := classMap[gid]
		if gidCls == cls {
			out = append(out, gid)
		}
	}
	return out
}

// cloneChainRuleWithTrigger returns a shallow copy of rule whose
// Input[0] set is replaced by the single-element {trigger} set.
func cloneChainRuleWithTrigger(rule *ChainContextSubst, trigger uint16) ChainContextSubst {
	input := make([][]uint16, len(rule.Input))
	copy(input, rule.Input)
	input[0] = []uint16{trigger}
	return ChainContextSubst{
		Backtrack: rule.Backtrack,
		Input:     input,
		Lookahead: rule.Lookahead,
		Actions:   rule.Actions,
	}
}

// parseChainSubClassRule reads a ChainSubClassRule:
//
//	backtrackGlyphCount(2) + backtrackSequence[...](2 class numbers each)
//	inputGlyphCount(2)     + inputSequence[inputGlyphCount-1](2 each)
//	lookaheadGlyphCount(2) + lookaheadSequence[...](2 each)
//	substCount(2)          + substLookupRecords[...](4 each)
//
// Class numbers are resolved against the supplied class maps; an empty
// GID set is emitted for class 0 entries, which the matcher treats as
// a wildcard (any glyph at that position).
func parseChainSubClassRule(gsub []byte, off int, backClass, inputClass, lookClass map[uint16]uint16, firstInputClass uint16, covered []uint16) *ChainContextSubst {
	if off+2 > len(gsub) {
		return nil
	}
	p := off
	backCount := int(be16(gsub, p))
	p += 2
	if p+backCount*2 > len(gsub) {
		return nil
	}
	backtrack := make([][]uint16, backCount)
	for i := 0; i < backCount; i++ {
		backtrack[i] = classMembers(backClass, be16(gsub, p+i*2))
	}
	p += backCount * 2
	if p+2 > len(gsub) {
		return nil
	}
	inputCount := int(be16(gsub, p))
	p += 2
	if inputCount < 1 {
		return nil
	}
	restInput := inputCount - 1
	if p+restInput*2 > len(gsub) {
		return nil
	}
	input := make([][]uint16, inputCount)
	// Input[0] is the trigger; cloneChainRuleWithTrigger will replace it
	// per-GID. Temporarily stash the class-0 members restricted to
	// coverage here so we have a well-formed placeholder.
	input[0] = classMembers(inputClass, firstInputClass)
	if input[0] == nil {
		// Input class 0 under Coverage: use the covered GIDs directly.
		input[0] = append([]uint16(nil), covered...)
	}
	for i := 0; i < restInput; i++ {
		input[i+1] = classMembers(inputClass, be16(gsub, p+i*2))
	}
	p += restInput * 2
	if p+2 > len(gsub) {
		return nil
	}
	lookCount := int(be16(gsub, p))
	p += 2
	if p+lookCount*2 > len(gsub) {
		return nil
	}
	lookahead := make([][]uint16, lookCount)
	for i := 0; i < lookCount; i++ {
		lookahead[i] = classMembers(lookClass, be16(gsub, p+i*2))
	}
	p += lookCount * 2
	actions := parseSubstLookupRecords(gsub, p)
	return &ChainContextSubst{
		Backtrack: backtrack,
		Input:     input,
		Lookahead: lookahead,
		Actions:   actions,
	}
}

// parseChainContextFormat3 handles ChainContextSubstFormat3:
//
//	format(2) = 3
//	backtrackGlyphCount(2) + backtrackCoverageOffsets[...](2 each)
//	inputGlyphCount(2)     + inputCoverageOffsets[...](2 each)
//	lookaheadGlyphCount(2) + lookaheadCoverageOffsets[...](2 each)
//	substCount(2)          + substLookupRecords[...](4 each)
//
// Each position references a Coverage table directly. inputCoverage[0]
// is the trigger; we expand it per member GID into individual runtime
// rules keyed by that trigger, matching the format-2 shape.
func parseChainContextFormat3(gsub []byte, off int, chain map[uint16][]ChainContextSubst) {
	if off+4 > len(gsub) {
		return
	}
	p := off + 2
	backCount := int(be16(gsub, p))
	p += 2
	if p+backCount*2 > len(gsub) {
		return
	}
	backtrack := make([][]uint16, backCount)
	for i := 0; i < backCount; i++ {
		covOff := off + int(be16(gsub, p+i*2))
		backtrack[i] = parseCoverage(gsub, covOff)
	}
	p += backCount * 2
	if p+2 > len(gsub) {
		return
	}
	inputCount := int(be16(gsub, p))
	p += 2
	if inputCount < 1 || p+inputCount*2 > len(gsub) {
		return
	}
	input := make([][]uint16, inputCount)
	for i := 0; i < inputCount; i++ {
		covOff := off + int(be16(gsub, p+i*2))
		input[i] = parseCoverage(gsub, covOff)
	}
	p += inputCount * 2
	if p+2 > len(gsub) {
		return
	}
	lookCount := int(be16(gsub, p))
	p += 2
	if p+lookCount*2 > len(gsub) {
		return
	}
	lookahead := make([][]uint16, lookCount)
	for i := 0; i < lookCount; i++ {
		covOff := off + int(be16(gsub, p+i*2))
		lookahead[i] = parseCoverage(gsub, covOff)
	}
	p += lookCount * 2
	actions := parseSubstLookupRecords(gsub, p)
	// Expand per trigger GID.
	triggers := input[0]
	for _, trig := range triggers {
		rule := ChainContextSubst{
			Backtrack: backtrack,
			Input:     append([][]uint16{{trig}}, input[1:]...),
			Lookahead: lookahead,
			Actions:   actions,
		}
		chain[trig] = append(chain[trig], rule)
	}
}

// parseSubstLookupRecords reads substCount (uint16) followed by substCount
// SubstLookupRecord entries of 4 bytes each: sequenceIndex, lookupIndex.
func parseSubstLookupRecords(gsub []byte, off int) []ChainSubstAction {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*4 > len(gsub) {
		return nil
	}
	out := make([]ChainSubstAction, count)
	for i := 0; i < count; i++ {
		out[i] = ChainSubstAction{
			SequenceIndex:   be16(gsub, off+2+i*4),
			LookupListIndex: be16(gsub, off+2+i*4+2),
		}
	}
	return out
}

// ApplyChainContext walks gids left-to-right and applies chaining
// contextual substitution rules for the given feature. At each position
// the trigger GID is used as a map key into the feature's rule table;
// each candidate rule is tested for backtrack, input, and lookahead
// matches and, on success, its SubstLookupRecord actions are dispatched.
//
// Action dispatch supports LookupType 1 (Single), LookupType 4 (Ligature),
// and recursive LookupType 6 (ChainContext) targets. LookupType 7
// (Extension) wrappers are transparently unwrapped at parse time.
// Recursion depth is bounded by maxChainDepth to defend against
// pathological self-referential rule sets.
//
// Reference: ISO 14496-22 §6.2 LookupType 6.
func (g *GSUBSubstitutions) ApplyChainContext(gids []uint16, feature GSUBFeature) []uint16 {
	if g == nil || len(g.ChainContext) == 0 || len(gids) == 0 {
		return gids
	}
	table, ok := g.ChainContext[feature]
	if !ok || len(table) == 0 {
		return gids
	}
	out := make([]uint16, len(gids))
	copy(out, gids)
	i := 0
	for i < len(out) {
		out = g.applyChainContextAt(out, i, table, 0)
		i++
	}
	return out
}

// applyChainContextAt tests every rule bucketed under the trigger glyph
// at position i and, on the first matching rule, dispatches each of
// its SubstLookupRecord actions via applyLookup. Returns the gids slice
// (possibly reallocated by a ligature splice). depth counts how many
// nested ChainContext applies we are inside; applyLookup enforces the
// maxChainDepth ceiling.
func (g *GSUBSubstitutions) applyChainContextAt(gids []uint16, i int, table map[uint16][]ChainContextSubst, depth int) []uint16 {
	if i < 0 || i >= len(gids) {
		return gids
	}
	rules := table[gids[i]]
	if len(rules) == 0 {
		return gids
	}
	for ri := range rules {
		rule := &rules[ri]
		if !chainRuleMatches(gids, i, rule) {
			continue
		}
		for _, act := range rule.Actions {
			pos := i + int(act.SequenceIndex)
			if pos < 0 || pos >= len(gids) {
				continue
			}
			gids = g.applyLookup(gids, act.LookupListIndex, pos, depth)
		}
		// OpenType does not specify a post-match advance for the outer
		// loop; the existing behavior advances by 1 regardless of the
		// input sequence length, which matches both the spec's silence
		// and the historic Folio behavior. We keep that to avoid
		// changing the dispatch shape for Single-only rules.
		return gids
	}
	return gids
}

// applyLookup dispatches a ChainContext action into the target lookup
// at the given gid position. It mutates gids according to the target
// lookup's type (Single: point substitution; Ligature: greedy splice;
// ChainContext: recursive match with depth+1). Returns the possibly
// reallocated gids slice. Recursion is capped at maxChainDepth.
func (g *GSUBSubstitutions) applyLookup(gids []uint16, lookupIdx uint16, position, depth int) []uint16 {
	if depth >= maxChainDepth {
		return gids
	}
	if int(lookupIdx) >= len(g.lookupTable) {
		return gids
	}
	rec := g.lookupTable[lookupIdx]
	if rec == nil {
		return gids
	}
	if position < 0 || position >= len(gids) {
		return gids
	}
	switch rec.Type {
	case 1:
		if repl, found := rec.Single[gids[position]]; found {
			gids[position] = repl
		}
		return gids
	case 4:
		next, _ := applyLigatureAt(gids, position, rec.Lig)
		return next
	case 6:
		return g.applyChainContextAt(gids, position, rec.Chain, depth+1)
	}
	return gids
}

// chainRuleMatches tests a single ChainContextSubst rule against gids
// anchored at position i (where gids[i] is the trigger / Input[0]).
// Returns true when the full backtrack, input, and lookahead all match.
// An empty GID set at any position is treated as a wildcard — this is
// how Format 2 class-0 positions are encoded.
func chainRuleMatches(gids []uint16, i int, rule *ChainContextSubst) bool {
	if i < 0 || i >= len(gids) {
		return false
	}
	if len(rule.Input) == 0 {
		return false
	}
	// Input: starting at i, each position must match.
	for k := 0; k < len(rule.Input); k++ {
		if i+k >= len(gids) {
			return false
		}
		if !gidSetMatches(rule.Input[k], gids[i+k]) {
			return false
		}
	}
	// Backtrack: Backtrack[0] is the glyph immediately before Input[0].
	for k := 0; k < len(rule.Backtrack); k++ {
		pos := i - 1 - k
		if pos < 0 {
			return false
		}
		if !gidSetMatches(rule.Backtrack[k], gids[pos]) {
			return false
		}
	}
	// Lookahead starts right after the input sequence.
	lookStart := i + len(rule.Input)
	for k := 0; k < len(rule.Lookahead); k++ {
		pos := lookStart + k
		if pos >= len(gids) {
			return false
		}
		if !gidSetMatches(rule.Lookahead[k], gids[pos]) {
			return false
		}
	}
	return true
}

// gidSetMatches returns true if gid is in set, OR if set is empty (a
// wildcard, used by Format 2 class-0 positions where the full glyph
// space can't be materialized at parse time).
func gidSetMatches(set []uint16, gid uint16) bool {
	if len(set) == 0 {
		return true
	}
	for _, s := range set {
		if s == gid {
			return true
		}
	}
	return false
}

// be16 reads a big-endian uint16 from data at the given offset.
func be16(data []byte, off int) uint16 {
	return binary.BigEndian.Uint16(data[off:])
}

// be32 reads a big-endian uint32 from data at the given offset.
func be32(data []byte, off int) uint32 {
	return binary.BigEndian.Uint32(data[off:])
}
