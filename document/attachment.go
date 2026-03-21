// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"time"

	"github.com/carlos7ags/folio/core"
)

// FileAttachment describes a file to be embedded in the PDF.
// PDF/A-3B is the only PDF/A level that permits file attachments (ISO 19005-3 §6.4).
type FileAttachment struct {
	// FileName is the name as it will appear in the PDF (e.g. "factur-x.xml").
	// Used for both /F (ASCII) and /UF (Unicode) entries in the filespec dictionary.
	FileName string

	// MIMEType is the MIME type of the file (e.g. "application/xml").
	// The '/' character will be encoded as '#2F' per PDF name syntax rules.
	MIMEType string

	// Description is an optional human-readable description of the file.
	Description string

	// AFRelationship is the PDF/A-3 association relationship key (ISO 19005-3 §6.4).
	// Valid values: "Alternative", "Source", "Data", "Supplement", "Unspecified".
	// Defaults to "Unspecified" if empty.
	// Use "Alternative" for ZUGFeRD/Factur-X where the XML is the machine-readable
	// equivalent of the PDF content.
	AFRelationship string

	// Data is the raw file content to embed.
	Data []byte
}

// AttachFile schedules a file to be embedded in the document.
// The document must be configured with PDF/A-3B (PdfAConfig{Level: PdfA3B});
// all other PDF/A levels forbid file attachments and will return an error on Save/WriteTo.
func (d *Document) AttachFile(a FileAttachment) {
	d.attachments = append(d.attachments, a)
}

// buildAttachments writes /EmbeddedFile streams and /Filespec dictionaries for
// all attached files, then wires them into the catalog via /AF and /Names.
// Must only be called when len(d.attachments) > 0.
func buildAttachments(
	attachments []FileAttachment,
	catalog *core.PdfDictionary,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
) {
	afArray := core.NewPdfArray()
	namesArr := core.NewPdfArray()

	now := time.Now()
	dateStr := now.Format("D:20060102150405")

	for _, att := range attachments {
		// ----------------------------------------------------------------
		// 1. /EmbeddedFile stream (ISO 32000-1 §7.11.4)
		// ----------------------------------------------------------------
		efStream := core.NewPdfStreamCompressed(att.Data)
		efStream.Dict.Set("Type", core.NewPdfName("EmbeddedFile"))

		// MIME type as a PDF name. '/' is a PDF delimiter and will be
		// encoded automatically as '#2F' by core.encodeName (ISO 32000-1 §7.3.5).
		mimeType := att.MIMEType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		efStream.Dict.Set("Subtype", core.NewPdfName(mimeType))

		// /Params: size and dates (ISO 32000-1 §7.11.4 Table 46).
		paramsDict := core.NewPdfDictionary()
		paramsDict.Set("Size", core.NewPdfInteger(len(att.Data)))
		paramsDict.Set("CreationDate", core.NewPdfLiteralString(dateStr))
		paramsDict.Set("ModificationDate", core.NewPdfLiteralString(dateStr))
		efStream.Dict.Set("Params", paramsDict)

		efRef := addObject(efStream)

		// ----------------------------------------------------------------
		// 2. /Filespec dictionary (ISO 32000-1 §7.11.3)
		// ----------------------------------------------------------------
		fsDict := core.NewPdfDictionary()
		fsDict.Set("Type", core.NewPdfName("Filespec"))
		fsDict.Set("F", core.NewPdfLiteralString(att.FileName))
		// /UF (Unicode filename) is required by PDF/A-3 even for ASCII names.
		fsDict.Set("UF", core.NewPdfLiteralString(att.FileName))

		// /EF holds a dictionary mapping /F to the EmbeddedFile stream ref.
		efHolder := core.NewPdfDictionary()
		efHolder.Set("F", efRef)
		fsDict.Set("EF", efHolder)

		if att.Description != "" {
			fsDict.Set("Desc", core.NewPdfLiteralString(att.Description))
		}

		rel := att.AFRelationship
		if rel == "" {
			rel = "Unspecified"
		}
		// /AFRelationship is mandatory for PDF/A-3 (ISO 19005-3 §6.4).
		fsDict.Set("AFRelationship", core.NewPdfName(rel))

		fsRef := addObject(fsDict)

		// Collect for /AF array and /EmbeddedFiles name tree.
		afArray.Add(fsRef)
		namesArr.Add(core.NewPdfLiteralString(att.FileName))
		namesArr.Add(fsRef)
	}

	// ----------------------------------------------------------------
	// 3. /AF on the catalog (ISO 19005-3 §6.4)
	// ----------------------------------------------------------------
	catalog.Set("AF", afArray)

	// ----------------------------------------------------------------
	// 4. /Names -> /EmbeddedFiles name tree (ISO 32000-1 §7.11.2)
	// ----------------------------------------------------------------
	embeddedFilesDict := core.NewPdfDictionary()
	embeddedFilesDict.Set("Names", namesArr)
	namesDict := core.NewPdfDictionary()
	namesDict.Set("EmbeddedFiles", embeddedFilesDict)
	catalog.Set("Names", namesDict)
}
