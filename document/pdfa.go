// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/carlos7ags/folio/core"
)

// PdfALevel specifies the PDF/A conformance level.
type PdfALevel int

// New PdfALevel constants must be appended to this block. The numeric
// values are part of the C ABI surface (see export/folio.h FOLIO_PDFA_*).
const (
	// PdfA2B is PDF/A-2b (ISO 19005-2:2011, Level B).
	// Based on PDF 1.7. Allows transparency. Requires font embedding,
	// XMP metadata, and an output intent with ICC profile.
	PdfA2B PdfALevel = iota

	// PdfA2U is PDF/A-2u (Level U). Adds Unicode mapping requirement.
	PdfA2U

	// PdfA2A is PDF/A-2a (Level A). Adds structure tagging requirement.
	PdfA2A

	// PdfA3B is PDF/A-3b (ISO 19005-3:2012, Level B). Like A-2b but
	// allows associated file attachments (§6.4).
	PdfA3B

	// PdfA1B is PDF/A-1b (ISO 19005-1:2005, Level B).
	// Based on PDF 1.4. Forbids transparency. Requires font embedding,
	// XMP metadata, and an output intent with ICC profile.
	PdfA1B

	// PdfA1A is PDF/A-1a (Level A). Like 1b but adds structure tagging.
	PdfA1A

	// PdfA3A is PDF/A-3a (ISO 19005-3:2012, Level A). Adds structure
	// tagging on top of A-3b; inherits attachment support from part 3.
	PdfA3A

	// PdfA4 is PDF/A-4 (ISO 19005-4:2020). Based on PDF 2.0. Has no
	// conformance level letter; XMP carries pdfaid:rev = "2020"
	// (§6.5). Forbids embedded files unless the level is PdfA4F or
	// PdfA4E.
	PdfA4

	// PdfA4F is PDF/A-4f (ISO 19005-4:2020). Like PdfA4 but permits
	// embedded files. Successor to PDF/A-3 for invoice/data carriers.
	PdfA4F

	// PdfA4E is PDF/A-4e (ISO 19005-4:2020). Engineering profile;
	// permits 3D, RichMedia, and embedded files. Successor to PDF/E-1.
	PdfA4E
)

// PdfAConfig holds PDF/A conformance settings.
type PdfAConfig struct {

	// OutputCondition is the output condition identifier
	// (e.g. "sRGB IEC61966-2.1"). Defaults to "sRGB IEC61966-2.1".
	OutputCondition string

	// ICCProfile is the ICC color profile data for the output intent.
	// If nil, a minimal sRGB profile description is used.
	ICCProfile []byte

	// XMPSchemas is an optional list of additional schema entries to declare
	// inside the single pdfaExtension:schemas <rdf:Bag>. Each entry is injected
	// as an <rdf:li rdf:parseType="Resource"> block alongside the built-in
	// pdfaf schema entry. Use this to declare custom namespaces such as the
	// Factur-X fx: schema required by ZUGFeRD validators.
	// There must be exactly one pdfaExtension:schemas block in the XMP stream —
	// this field merges into that block rather than adding a second one.
	XMPSchemas []XMPSchema

	// XMPProperties is an optional list of rdf:Description blocks carrying
	// actual property values (e.g. fx:DocumentType, fx:Version). Each entry
	// is injected verbatim as a separate rdf:Description inside rdf:RDF,
	// after the pdfaExtension:schemas block. The Namespace and Prefix fields
	// are used to build the xmlns attribute; Properties holds the values.
	XMPProperties []XMPPropertyBlock
	Level         PdfALevel
}

// XMPSchema describes one schema entry to add to the pdfaExtension:schemas bag.
type XMPSchema struct {
	// Schema is the human-readable schema name.
	Schema string
	// NamespaceURI is the full namespace URI (e.g. "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#").
	NamespaceURI string
	// Prefix is the XML namespace prefix (e.g. "fx").
	Prefix string
	// Properties lists each property declared in this schema.
	Properties []XMPSchemaProperty
}

// XMPSchemaProperty declares one property within an XMPSchema.
type XMPSchemaProperty struct {
	Name        string // e.g. "DocumentFileName"
	ValueType   string // e.g. "Text"
	Category    string // "external" or "internal"
	Description string
}

// XMPPropertyBlock carries actual XMP property values under a given namespace.
type XMPPropertyBlock struct {
	// Namespace is the full namespace URI used in the xmlns attribute.
	Namespace string
	// Prefix is the XML namespace prefix used for the property elements.
	Prefix string
	// Properties is a list of (name, value) pairs to emit as child elements.
	Properties []XMPProperty
}

// XMPProperty is a single name/value pair within an XMPPropertyBlock.
type XMPProperty struct {
	Name  string
	Value string
}

// SetPdfA enables PDF/A conformance on the document.
// This enforces: font embedding, XMP metadata, output intent,
// and disables encryption. For level A, tagging is enabled automatically.
// Full validation (Title, fonts, transparency) happens at WriteTo time
// or via [Document.ValidatePdfA].
func (d *Document) SetPdfA(config PdfAConfig) {
	d.pdfA = &config
	// Level A (any part) requires tagged PDF (ISO 19005-1 §6.8,
	// ISO 19005-2 §6.7.3, ISO 19005-3 §6.7.3). PDF/A-4 has no Level A.
	if isLevelA(config.Level) {
		d.tagged = true
	}
	// PDF/A disallows encryption (ISO 19005-1 §6.1.3, etc.).
	d.encryption = nil
}

// ValidatePdfA checks PDF/A requirements against the current document state.
// This can be called before WriteTo to catch issues early (missing Title,
// non-embedded fonts, forbidden transparency). Returns nil if PDF/A is not
// enabled or all checks pass.
func (d *Document) ValidatePdfA() error {
	if d.pdfA == nil {
		return nil
	}
	return d.validatePdfA(d.pages)
}

// pdfALevelString returns the conformance level letter for the
// pdfaid:conformance XMP property. Returns "" when the level has no
// conformance letter (PDF/A-4 base profile, ISO 19005-4 §6.5).
func pdfALevelString(level PdfALevel) string {
	switch level {
	case PdfA1B, PdfA2B, PdfA3B:
		return "B"
	case PdfA2U:
		return "U"
	case PdfA1A, PdfA2A, PdfA3A:
		return "A"
	case PdfA4F:
		return "F"
	case PdfA4E:
		return "E"
	case PdfA4:
		return ""
	default:
		return "B"
	}
}

// pdfAPartNumber returns the PDF/A part number.
func pdfAPartNumber(level PdfALevel) int {
	switch level {
	case PdfA1B, PdfA1A:
		return 1
	case PdfA3B, PdfA3A:
		return 3
	case PdfA4, PdfA4F, PdfA4E:
		return 4
	default:
		return 2
	}
}

// isPdfA1 returns true if the level is a PDF/A-1 variant.
func isPdfA1(level PdfALevel) bool {
	return level == PdfA1B || level == PdfA1A
}

// isPdfA4 returns true if the level is a PDF/A-4 variant.
func isPdfA4(level PdfALevel) bool {
	return level == PdfA4 || level == PdfA4F || level == PdfA4E
}

// isLevelA returns true if the level is an accessibility-conformance
// (Level A) variant. PDF/A-4 has no Level A equivalent.
func isLevelA(level PdfALevel) bool {
	return level == PdfA1A || level == PdfA2A || level == PdfA3A
}

// allowsAttachments returns true if the level permits embedded files
// in the /EmbeddedFiles name tree (ISO 19005-3 §6.4 for part 3;
// ISO 19005-4 §6.8 for A-4f / A-4e).
func allowsAttachments(level PdfALevel) bool {
	switch level {
	case PdfA3B, PdfA3A, PdfA4F, PdfA4E:
		return true
	default:
		return false
	}
}

// pdfVersionForPdfA returns the PDF version string for the given level.
// PDF/A-1 is based on PDF 1.4 (ISO 19005-1 §6.1.2);
// PDF/A-2 and PDF/A-3 on PDF 1.7 (ISO 19005-2 §6.1.2);
// PDF/A-4 on PDF 2.0 (ISO 19005-4 §6.1).
func pdfVersionForPdfA(level PdfALevel) string {
	switch {
	case isPdfA1(level):
		return "1.4"
	case isPdfA4(level):
		return "2.0"
	default:
		return "1.7"
	}
}

// validatePdfA checks that the document meets PDF/A requirements.
// Returns an error describing the first violation found, or nil if valid.
func (d *Document) validatePdfA(allPages []*Page) error {
	if d.pdfA == nil {
		return nil
	}

	// PDF/A forbids encryption.
	if d.encryption != nil {
		return fmt.Errorf("document: pdfa: encryption is not allowed in PDF/A documents")
	}

	// All fonts on all pages must be embedded (no bare standard fonts).
	for i, page := range allPages {
		for _, fr := range page.fonts {
			if fr.standard != nil && fr.embedded == nil {
				return fmt.Errorf("document: pdfa: page %d uses non-embedded standard font %q; PDF/A requires all fonts to be embedded",
					i+1, fr.standard.Name())
			}
		}
	}

	// PDF/A-1 forbids transparency (ISO 19005-1 §6.4).
	if isPdfA1(d.pdfA.Level) {
		for i, page := range allPages {
			if len(page.extGStates) > 0 {
				return fmt.Errorf("document: pdfa: page %d uses transparency (ExtGState); PDF/A-1 forbids transparency", i+1)
			}
		}
	}

	// File attachments are only permitted in parts/levels that explicitly
	// allow them: PDF/A-3 (ISO 19005-3 §6.4) and PDF/A-4f / PDF/A-4e
	// (ISO 19005-4 §6.8). Plain PDF/A-4 forbids them.
	if len(d.attachments) > 0 && !allowsAttachments(d.pdfA.Level) {
		return fmt.Errorf("document: pdfa: file attachments are only permitted in PDF/A-3 (a/b) or PDF/A-4f / PDF/A-4e; current level does not allow them")
	}

	// Title is required.
	if d.Info.Title == "" {
		return fmt.Errorf("document: pdfa: document Title is required for PDF/A conformance")
	}

	// Level A (accessibility-conformance) requires a declared natural
	// language for all text (ISO 19005-2 §6.7.2 / 19005-3 §6.7.2).
	// Folio satisfies this via the catalog /Lang entry, populated from
	// Info.Language. Per-structure Lang is not yet exposed.
	if isLevelA(d.pdfA.Level) && d.Info.Language == "" {
		return fmt.Errorf("document: pdfa: Info.Language is required for Level A (accessibility-conformance) variants; set Info.Language to a BCP 47 / RFC 3066 tag (e.g. \"en-US\")")
	}

	return nil
}

// buildXMPMetadata generates the XMP metadata stream for PDF/A identification.
func buildXMPMetadata(info Info, level PdfALevel, xmpSchemas []XMPSchema, xmpProperties []XMPPropertyBlock, deterministic bool, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfIndirectReference {
	part := pdfAPartNumber(level)
	conf := pdfALevelString(level)

	// Prefer the caller-supplied creation date. In deterministic mode the
	// wall-clock fallback is suppressed so an unset date resolves to the zero
	// time (a fixed value) rather than time.Now, keeping output reproducible.
	var now time.Time
	if !info.CreationDate.IsZero() {
		now = info.CreationDate
	} else if !deterministic {
		now = time.Now()
	}
	dateStr := now.Format("2006-01-02T15:04:05-07:00")

	title := xmlEscape(info.Title)
	author := xmlEscape(info.Author)
	language := xmlEscape(info.Language)
	creator := xmlEscape(info.Creator)
	if creator == "" {
		creator = "Folio"
	}
	producer := xmlEscape(info.Producer)
	if producer == "" {
		producer = "Folio (github.com/carlos7ags/folio)"
	}

	var b strings.Builder
	b.WriteString(`<?xpacket begin="` + "\xef\xbb\xbf" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>`)
	b.WriteString("\n")
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/">`)
	b.WriteString("\n")
	b.WriteString(`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
	b.WriteString("\n")

	// Dublin Core (title, creator, language)
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:dc="http://purl.org/dc/elements/1.1/">`)
	b.WriteString("\n")
	if title != "" {
		b.WriteString(`<dc:title><rdf:Alt><rdf:li xml:lang="x-default">` + title + `</rdf:li></rdf:Alt></dc:title>`)
		b.WriteString("\n")
	}
	if author != "" {
		b.WriteString(`<dc:creator><rdf:Seq><rdf:li>` + author + `</rdf:li></rdf:Seq></dc:creator>`)
		b.WriteString("\n")
	}
	if language != "" {
		// dc:language mirrors the catalog /Lang entry. PDF/A Level A
		// (ISO 19005-2 §6.7.2) and PDF/UA-1 conformance check for it.
		b.WriteString(`<dc:language><rdf:Bag><rdf:li>` + language + `</rdf:li></rdf:Bag></dc:language>`)
		b.WriteString("\n")
	}
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// XMP Basic (creator tool, dates)
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:xmp="http://ns.adobe.com/xap/1.0/">`)
	b.WriteString("\n")
	b.WriteString(`<xmp:CreatorTool>` + creator + `</xmp:CreatorTool>`)
	b.WriteString("\n")
	b.WriteString(`<xmp:CreateDate>` + dateStr + `</xmp:CreateDate>`)
	b.WriteString("\n")
	b.WriteString(`<xmp:ModifyDate>` + dateStr + `</xmp:ModifyDate>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// PDF properties
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:pdf="http://ns.adobe.com/pdf/1.3/">`)
	b.WriteString("\n")
	b.WriteString(`<pdf:Producer>` + producer + `</pdf:Producer>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// PDF/A identification (ISO 19005-1 §6.7.11, 19005-2 §6.7.11,
	// 19005-3 §6.7.11, 19005-4 §6.5).
	//
	// Parts 1–3 carry pdfaid:part + pdfaid:conformance.
	// Part 4 carries pdfaid:part + pdfaid:rev = "2020"; conformance is
	// only present for the A-4f / A-4e profiles (values "F" / "E").
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">`)
	b.WriteString("\n")
	fmt.Fprintf(&b, `<pdfaid:part>%d</pdfaid:part>`, part)
	b.WriteString("\n")
	if isPdfA4(level) {
		b.WriteString(`<pdfaid:rev>2020</pdfaid:rev>`)
		b.WriteString("\n")
	}
	if conf != "" {
		b.WriteString(`<pdfaid:conformance>` + conf + `</pdfaid:conformance>`)
		b.WriteString("\n")
	}
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// Any level that allows associated files (PDF/A-3 a/b, PDF/A-4f,
	// PDF/A-4e) needs the pdfaf extension schema declared so that
	// validators recognise the AFRelationship key on filespecs.
	// The built-in pdfaf schema and any caller-supplied schemas
	// (e.g. Factur-X fx:) are merged into one <rdf:Bag> to keep a
	// single pdfaExtension:schemas block.
	emitAFSchema := allowsAttachments(level)
	if emitAFSchema || len(xmpSchemas) > 0 {
		b.WriteString(`<rdf:Description rdf:about=""`)
		b.WriteString(` xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"`)
		b.WriteString(` xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#"`)
		b.WriteString(` xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">`)
		b.WriteString("\n")
		b.WriteString(`<pdfaExtension:schemas><rdf:Bag>`)
		b.WriteString("\n")

		// Built-in pdfaf schema (AF associated files, ISO 19005-3 §6.4
		// and ISO 19005-4 §6.8).
		if emitAFSchema {
			b.WriteString(`<rdf:li rdf:parseType="Resource">`)
			b.WriteString(`<pdfaSchema:schema>PDF/A Associated File Attachment</pdfaSchema:schema>`)
			b.WriteString(`<pdfaSchema:namespaceURI>http://www.aiim.org/pdfa/ns/f#</pdfaSchema:namespaceURI>`)
			b.WriteString(`<pdfaSchema:prefix>pdfaf</pdfaSchema:prefix>`)
			b.WriteString(`<pdfaSchema:property><rdf:Seq><rdf:li rdf:parseType="Resource">`)
			b.WriteString(`<pdfaProperty:name>file</pdfaProperty:name>`)
			b.WriteString(`<pdfaProperty:valueType>URI</pdfaProperty:valueType>`)
			b.WriteString(`<pdfaProperty:category>external</pdfaProperty:category>`)
			b.WriteString(`<pdfaProperty:description>Associated file</pdfaProperty:description>`)
			b.WriteString(`</rdf:li></rdf:Seq></pdfaSchema:property>`)
			b.WriteString(`</rdf:li>`)
			b.WriteString("\n")
		}

		// Caller-supplied schema declarations (e.g. Factur-X fx: schema).
		for _, schema := range xmpSchemas {
			b.WriteString(`<rdf:li rdf:parseType="Resource">`)
			b.WriteString("\n")
			b.WriteString(`<pdfaSchema:schema>` + xmlEscape(schema.Schema) + `</pdfaSchema:schema>`)
			b.WriteString("\n")
			b.WriteString(`<pdfaSchema:namespaceURI>` + xmlEscape(schema.NamespaceURI) + `</pdfaSchema:namespaceURI>`)
			b.WriteString("\n")
			b.WriteString(`<pdfaSchema:prefix>` + xmlEscape(schema.Prefix) + `</pdfaSchema:prefix>`)
			b.WriteString("\n")
			if len(schema.Properties) > 0 {
				b.WriteString(`<pdfaSchema:property><rdf:Seq>`)
				b.WriteString("\n")
				for _, prop := range schema.Properties {
					b.WriteString(`<rdf:li rdf:parseType="Resource">`)
					b.WriteString("\n")
					b.WriteString(`<pdfaProperty:name>` + xmlEscape(prop.Name) + `</pdfaProperty:name>`)
					b.WriteString("\n")
					b.WriteString(`<pdfaProperty:valueType>` + xmlEscape(prop.ValueType) + `</pdfaProperty:valueType>`)
					b.WriteString("\n")
					b.WriteString(`<pdfaProperty:category>` + xmlEscape(prop.Category) + `</pdfaProperty:category>`)
					b.WriteString("\n")
					b.WriteString(`<pdfaProperty:description>` + xmlEscape(prop.Description) + `</pdfaProperty:description>`)
					b.WriteString("\n")
					b.WriteString(`</rdf:li>`)
					b.WriteString("\n")
				}
				b.WriteString(`</rdf:Seq></pdfaSchema:property>`)
				b.WriteString("\n")
			}
			b.WriteString(`</rdf:li>`)
			b.WriteString("\n")
		}

		b.WriteString(`</rdf:Bag></pdfaExtension:schemas>`)
		b.WriteString("\n")
		b.WriteString(`</rdf:Description>`)
		b.WriteString("\n")
	}

	// Caller-supplied property value blocks (e.g. fx:DocumentType, fx:Version).
	for _, block := range xmpProperties {
		b.WriteString(`<rdf:Description rdf:about=""`)
		b.WriteString(` xmlns:` + block.Prefix + `="` + xmlEscape(block.Namespace) + `">`)
		b.WriteString("\n")
		for _, prop := range block.Properties {
			b.WriteString(`<` + block.Prefix + `:` + prop.Name + `>` + xmlEscape(prop.Value) + `</` + block.Prefix + `:` + prop.Name + `>`)
			b.WriteString("\n")
		}
		b.WriteString(`</rdf:Description>`)
		b.WriteString("\n")
	}

	b.WriteString(`</rdf:RDF>`)
	b.WriteString("\n")
	b.WriteString(`</x:xmpmeta>`)
	b.WriteString("\n")
	b.WriteString(`<?xpacket end="w"?>`)

	xmpBytes := []byte(b.String())

	// XMP metadata stream: must NOT be compressed, must have /Type /Metadata /Subtype /XML.
	stream := core.NewPdfStream(xmpBytes)
	stream.Dict.Set("Type", core.NewPdfName("Metadata"))
	stream.Dict.Set("Subtype", core.NewPdfName("XML"))

	return addObject(stream)
}

// buildOutputIntent creates the PDF/A output intent dictionary with
// an embedded ICC color profile.
func buildOutputIntent(config *PdfAConfig, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfIndirectReference {
	condition := config.OutputCondition
	if condition == "" {
		condition = "sRGB IEC61966-2.1"
	}

	// ICC profile stream.
	profileData := config.ICCProfile
	if len(profileData) == 0 {
		profileData = srgbICCProfile()
	}

	profileStream := core.NewPdfStreamCompressed(profileData)
	profileStream.Dict.Set("N", core.NewPdfInteger(3)) // 3 components (RGB)
	profileRef := addObject(profileStream)

	// Output intent dictionary.
	intent := core.NewPdfDictionary()
	intent.Set("Type", core.NewPdfName("OutputIntent"))
	intent.Set("S", core.NewPdfName("GTS_PDFA1")) // required for PDF/A
	intent.Set("OutputConditionIdentifier", core.NewPdfLiteralString(condition))
	intent.Set("RegistryName", core.NewPdfLiteralString("http://www.color.org"))
	intent.Set("Info", core.NewPdfLiteralString(condition))
	intent.Set("DestOutputProfile", profileRef)

	return addObject(intent)
}

// srgbICCProfile returns a complete sRGB IEC61966-2.1 ICC v2 profile.
// The profile contains all 9 required tags for an RGB display profile:
// desc, cprt, wtpt, rXYZ, gXYZ, bXYZ, rTRC, gTRC, bTRC.
// The TRC uses a 1024-entry LUT that accurately represents the sRGB
// piecewise transfer function, passing strict PDF/A validators (veraPDF).
func srgbICCProfile() []byte {
	const (
		headerSize  = 128
		tagCount    = 9
		tagTableOff = headerSize
		tagTableSz  = 4 + tagCount*12 // count + entries
	)
	dataOff := tagTableOff + tagTableSz

	// Precompute tag data.
	descData := iccTextDescriptionTag("sRGB IEC61966-2.1")
	cprtData := iccTextTag("Public Domain")
	wtptData := iccXYZTag(0.9504559, 1.0000000, 1.0890577) // D50
	rXYZData := iccXYZTag(0.4360747, 0.2225045, 0.0139322)
	gXYZData := iccXYZTag(0.3850649, 0.7168786, 0.0971045)
	bXYZData := iccXYZTag(0.1430804, 0.0606169, 0.7141733)
	trcData := iccSRGBCurveTag()

	// Layout tags sequentially, 4-byte aligned.
	type tagLayout struct {
		sig  string
		data []byte
	}
	tags := []tagLayout{
		{"desc", descData},
		{"cprt", cprtData},
		{"wtpt", wtptData},
		{"rXYZ", rXYZData},
		{"gXYZ", gXYZData},
		{"bXYZ", bXYZData},
		{"rTRC", trcData},
	}

	// Compute offsets and total size.
	offsets := make([]int, len(tags))
	off := dataOff
	for i, t := range tags {
		offsets[i] = off
		off += len(t.data)
		// Pad to 4-byte boundary.
		if off%4 != 0 {
			off += 4 - off%4
		}
	}
	profileSize := off

	// gTRC and bTRC share rTRC data (same curve for all channels).
	trcOff := offsets[6]
	trcSz := len(trcData)

	profile := make([]byte, profileSize)

	// --- Header (128 bytes) ---
	binary.BigEndian.PutUint32(profile[0:4], uint32(profileSize))
	// Version 2.1.0.
	profile[8] = 2
	profile[9] = 0x10
	copy(profile[12:16], "mntr") // device class: monitor
	copy(profile[16:20], "RGB ") // color space
	copy(profile[20:24], "XYZ ") // PCS
	// Date: 2024-01-01 00:00:00.
	binary.BigEndian.PutUint16(profile[24:26], 2024)
	binary.BigEndian.PutUint16(profile[26:28], 1) // month
	binary.BigEndian.PutUint16(profile[28:30], 1) // day
	copy(profile[36:40], "acsp")                  // signature
	copy(profile[40:44], "APPL")                  // primary platform
	// Illuminant D50.
	iccPutS15Fixed16(profile[68:72], 0.9504559)
	iccPutS15Fixed16(profile[72:76], 1.0000000)
	iccPutS15Fixed16(profile[76:80], 1.0890577)

	// --- Tag table ---
	binary.BigEndian.PutUint32(profile[tagTableOff:], uint32(tagCount))
	type tagEntry struct {
		sig    string
		offset int
		size   int
	}
	entries := []tagEntry{
		{"desc", offsets[0], len(tags[0].data)},
		{"cprt", offsets[1], len(tags[1].data)},
		{"wtpt", offsets[2], len(tags[2].data)},
		{"rXYZ", offsets[3], len(tags[3].data)},
		{"gXYZ", offsets[4], len(tags[4].data)},
		{"bXYZ", offsets[5], len(tags[5].data)},
		{"rTRC", trcOff, trcSz},
		{"gTRC", trcOff, trcSz}, // shared with rTRC
		{"bTRC", trcOff, trcSz}, // shared with rTRC
	}
	for i, e := range entries {
		p := tagTableOff + 4 + i*12
		copy(profile[p:p+4], e.sig)
		binary.BigEndian.PutUint32(profile[p+4:p+8], uint32(e.offset))
		binary.BigEndian.PutUint32(profile[p+8:p+12], uint32(e.size))
	}

	// --- Tag data ---
	for i, t := range tags {
		copy(profile[offsets[i]:], t.data)
	}

	return profile
}

// iccPutS15Fixed16 writes a float64 as an ICC s15Fixed16Number (big-endian).
func iccPutS15Fixed16(b []byte, v float64) {
	fixed := int32(math.Round(v * 65536))
	binary.BigEndian.PutUint32(b, uint32(fixed))
}

// iccXYZTag returns an ICC 'XYZ ' tag for the given XYZ values.
func iccXYZTag(x, y, z float64) []byte {
	// 'XYZ ' signature (4) + reserved (4) + one XYZNumber (12) = 20 bytes.
	data := make([]byte, 20)
	copy(data[0:4], "XYZ ")
	iccPutS15Fixed16(data[8:12], x)
	iccPutS15Fixed16(data[12:16], y)
	iccPutS15Fixed16(data[16:20], z)
	return data
}

// iccTextDescriptionTag returns an ICC 'desc' (textDescriptionType) tag.
func iccTextDescriptionTag(s string) []byte {
	ascii := []byte(s)
	asciiLen := len(ascii) + 1 // includes null terminator
	// desc: sig(4) + reserved(4) + asciiCount(4) + ascii+null + unicodeCode(4) + unicodeCount(4) + scriptCode(2) + scriptCount(1) + scriptData(67)
	size := 4 + 4 + 4 + asciiLen + 4 + 4 + 2 + 1 + 67
	data := make([]byte, size)
	copy(data[0:4], "desc")
	binary.BigEndian.PutUint32(data[8:12], uint32(asciiLen))
	copy(data[12:12+len(ascii)], ascii)
	// Remaining fields (unicode, scriptcode) are zero = not present.
	return data
}

// iccTextTag returns an ICC 'text' (textType) tag.
func iccTextTag(s string) []byte {
	ascii := []byte(s)
	// text: sig(4) + reserved(4) + string including null.
	data := make([]byte, 4+4+len(ascii)+1)
	copy(data[0:4], "text")
	copy(data[8:], ascii)
	return data
}

// iccSRGBCurveTag returns an ICC 'curv' tag with a 1024-entry LUT
// that represents the sRGB transfer function (IEC 61966-2.1).
func iccSRGBCurveTag() []byte {
	const n = 1024
	// curv: sig(4) + reserved(4) + count(4) + entries(n*2).
	data := make([]byte, 4+4+4+n*2)
	copy(data[0:4], "curv")
	binary.BigEndian.PutUint32(data[8:12], n)

	for i := 0; i < n; i++ {
		// sRGB forward transfer: linear → encoded is the inverse,
		// but ICC TRC stores the forward (encoded → linear) direction.
		// For the ICC TRC, input is the encoded value, output is linear.
		// The LUT maps index i (0..1023) to linear value as uint16.
		t := float64(i) / float64(n-1) // encoded sRGB value [0,1]
		var linear float64
		if t <= 0.04045 {
			linear = t / 12.92
		} else {
			linear = math.Pow((t+0.055)/1.055, 2.4)
		}
		val := uint16(math.Round(linear * 65535))
		off := 12 + i*2
		binary.BigEndian.PutUint16(data[off:off+2], val)
	}
	return data
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
