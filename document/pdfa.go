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

const (
	// PdfA2B is PDF/A-2b (ISO 19005-2:2011, Level B).
	// Based on PDF 1.7. Allows transparency. Requires font embedding,
	// XMP metadata, and an output intent with ICC profile.
	PdfA2B PdfALevel = iota

	// PdfA2U is PDF/A-2u (Level U). Adds Unicode mapping requirement.
	PdfA2U

	// PdfA2A is PDF/A-2a (Level A). Adds structure tagging requirement.
	PdfA2A

	// PdfA3B is PDF/A-3b. Like A-2b but allows file attachments.
	PdfA3B

	// PdfA1B is PDF/A-1b (ISO 19005-1:2005, Level B).
	// Based on PDF 1.4. Forbids transparency. Requires font embedding,
	// XMP metadata, and an output intent with ICC profile.
	PdfA1B

	// PdfA1A is PDF/A-1a (Level A). Like 1b but adds structure tagging.
	PdfA1A
)

// PdfAConfig holds PDF/A conformance settings.
type PdfAConfig struct {
	Level PdfALevel

	// ICCProfile is the ICC color profile data for the output intent.
	// If nil, a minimal sRGB profile description is used.
	ICCProfile []byte

	// OutputCondition is the output condition identifier
	// (e.g. "sRGB IEC61966-2.1"). Defaults to "sRGB IEC61966-2.1".
	OutputCondition string

	// XMPExtension is an optional block of raw RDF XML that is injected
	// verbatim into the XMP metadata stream, immediately before the closing
	// </rdf:RDF> tag. Use this to add application-specific XMP namespaces
	// such as the Factur-X/ZUGFeRD fx: description block required by
	// ZUGFeRD validators. The string must be a valid rdf:Description element.
	XMPExtension string
}

// SetPdfA enables PDF/A conformance on the document.
// This enforces: font embedding, XMP metadata, output intent,
// and disables encryption. For level A, tagging is enabled automatically.
func (d *Document) SetPdfA(config PdfAConfig) {
	d.pdfA = &config
	// Level A (any part) requires tagged PDF.
	if config.Level == PdfA2A || config.Level == PdfA1A {
		d.tagged = true
	}
	// PDF/A disallows encryption.
	d.encryption = nil
}

// pdfALevelString returns the conformance level letter.
func pdfALevelString(level PdfALevel) string {
	switch level {
	case PdfA1B, PdfA2B, PdfA3B:
		return "B"
	case PdfA2U:
		return "U"
	case PdfA1A, PdfA2A:
		return "A"
	default:
		return "B"
	}
}

// pdfAPartNumber returns the PDF/A part number.
func pdfAPartNumber(level PdfALevel) int {
	switch level {
	case PdfA1B, PdfA1A:
		return 1
	case PdfA3B:
		return 3
	default:
		return 2
	}
}

// isPdfA1 returns true if the level is a PDF/A-1 variant.
func isPdfA1(level PdfALevel) bool {
	return level == PdfA1B || level == PdfA1A
}

// pdfVersionForPdfA returns the PDF version string for the given level.
// PDF/A-1 is based on PDF 1.4; PDF/A-2 and later on PDF 1.7.
func pdfVersionForPdfA(level PdfALevel) string {
	if isPdfA1(level) {
		return "1.4"
	}
	return "1.7"
}

// validatePdfA checks that the document meets PDF/A requirements.
// Returns an error describing the first violation found, or nil if valid.
func (d *Document) validatePdfA(allPages []*Page) error {
	if d.pdfA == nil {
		return nil
	}

	// PDF/A forbids encryption.
	if d.encryption != nil {
		return fmt.Errorf("pdfa: encryption is not allowed in PDF/A documents")
	}

	// All fonts on all pages must be embedded (no bare standard fonts).
	for i, page := range allPages {
		for _, fr := range page.fonts {
			if fr.standard != nil && fr.embedded == nil {
				return fmt.Errorf("pdfa: page %d uses non-embedded standard font %q; PDF/A requires all fonts to be embedded",
					i, fr.standard.Name())
			}
		}
	}

	// PDF/A-1 forbids transparency (ISO 19005-1 §6.4).
	if isPdfA1(d.pdfA.Level) {
		for i, page := range allPages {
			if len(page.extGStates) > 0 {
				return fmt.Errorf("pdfa: page %d uses transparency (ExtGState); PDF/A-1 forbids transparency", i)
			}
		}
	}

	// File attachments are only permitted in PDF/A-3B (ISO 19005-3 §6.4).
	if len(d.attachments) > 0 && d.pdfA.Level != PdfA3B {
		return fmt.Errorf("pdfa: file attachments are only permitted in PDF/A-3B; current level does not allow them")
	}

	// Title is required.
	if d.Info.Title == "" {
		return fmt.Errorf("pdfa: document Title is required for PDF/A conformance")
	}

	return nil
}

// buildXMPMetadata generates the XMP metadata stream for PDF/A identification.
func buildXMPMetadata(info Info, level PdfALevel, xmpExtension string, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfIndirectReference {
	part := pdfAPartNumber(level)
	conf := pdfALevelString(level)

	now := time.Now()
	if !info.CreationDate.IsZero() {
		now = info.CreationDate
	}
	dateStr := now.Format("2006-01-02T15:04:05-07:00")

	title := xmlEscape(info.Title)
	author := xmlEscape(info.Author)
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

	// Dublin Core (title, creator)
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

	// PDF/A identification
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">`)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(`<pdfaid:part>%d</pdfaid:part>`, part))
	b.WriteString("\n")
	b.WriteString(`<pdfaid:conformance>` + conf + `</pdfaid:conformance>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// PDF/A-3 requires declaring the AF property via the pdfaExtension schema.
	// Without this block strict validators (e.g. veraPDF) report a schema error.
	if level == PdfA3B {
		b.WriteString(`<rdf:Description rdf:about=""`)
		b.WriteString(` xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"`)
		b.WriteString(` xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#"`)
		b.WriteString(` xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">`)
		b.WriteString("\n")
		b.WriteString(`<pdfaExtension:schemas><rdf:Bag><rdf:li rdf:parseType="Resource">`)
		b.WriteString(`<pdfaSchema:schema>PDF/A-3 Association File Attachment</pdfaSchema:schema>`)
		b.WriteString(`<pdfaSchema:namespaceURI>http://www.aiim.org/pdfa/ns/f#</pdfaSchema:namespaceURI>`)
		b.WriteString(`<pdfaSchema:prefix>pdfaf</pdfaSchema:prefix>`)
		b.WriteString(`<pdfaSchema:property><rdf:Seq><rdf:li rdf:parseType="Resource">`)
		b.WriteString(`<pdfaProperty:name>file</pdfaProperty:name>`)
		b.WriteString(`<pdfaProperty:valueType>URI</pdfaProperty:valueType>`)
		b.WriteString(`<pdfaProperty:category>external</pdfaProperty:category>`)
		b.WriteString(`<pdfaProperty:description>Associated file</pdfaProperty:description>`)
		b.WriteString(`</rdf:li></rdf:Seq></pdfaSchema:property>`)
		b.WriteString(`</rdf:li></rdf:Bag></pdfaExtension:schemas>`)
		b.WriteString("\n")
		b.WriteString(`</rdf:Description>`)
		b.WriteString("\n")
	}

	// Application-specific XMP extension (e.g. Factur-X/ZUGFeRD fx: block).
	if xmpExtension != "" {
		b.WriteString(xmpExtension)
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
