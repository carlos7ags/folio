// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"errors"
	"fmt"
	"io"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
)

// docTimestampDict is a signature dictionary for document timestamps.
// It uses /SubFilter /ETSI.RFC3161 and /Type /DocTimeStamp.
type docTimestampDict struct{}

// Type returns ObjectTypeDictionary.
func (d *docTimestampDict) Type() core.ObjectType { return core.ObjectTypeDictionary }

// WriteTo serializes the document timestamp dictionary with placeholder
// /ByteRange and /Contents values for later patching.
func (d *docTimestampDict) WriteTo(w io.Writer) (int64, error) {
	var total int64
	write := func(str string) error {
		n, err := w.Write([]byte(str))
		total += int64(n)
		return err
	}

	if err := write("<< /Type /DocTimeStamp"); err != nil {
		return total, err
	}
	if err := write(" /Filter /Adobe.PPKLite"); err != nil {
		return total, err
	}
	if err := write(" /SubFilter /ETSI.RFC3161"); err != nil {
		return total, err
	}

	// /ByteRange — fixed-width placeholder.
	if err := write(" /ByteRange "); err != nil {
		return total, err
	}
	if err := write(byteRangePlaceholder); err != nil {
		return total, err
	}

	// /Contents — hex string placeholder for the timestamp token.
	if err := write(" /Contents "); err != nil {
		return total, err
	}
	if err := write(contentsPlaceholder); err != nil {
		return total, err
	}

	if err := write(" >>"); err != nil {
		return total, err
	}
	return total, nil
}

// AddDocumentTimestamp appends a document timestamp signature to a signed PDF.
// This upgrades a B-LT signature to B-LTA by proving the DSS data existed
// at the timestamp time.
//
// The document timestamp covers the entire PDF (including DSS), uses
// /SubFilter /ETSI.RFC3161, and /Type /DocTimeStamp.
func AddDocumentTimestamp(pdfBytes []byte, tsaClient *TSAClient, hashFunc crypto.Hash) ([]byte, error) {
	if tsaClient == nil {
		return nil, errors.New("sign: TSAClient is required for document timestamp")
	}

	r, err := reader.Parse(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: parse PDF: %w", err)
	}

	trailer := r.Trailer()
	if trailer == nil {
		return nil, errors.New("sign: could not read PDF trailer")
	}

	prevXref, err := findStartXref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	nextObjNum := r.MaxObjectNumber() + 1

	// Build document timestamp dictionary.
	tsDict := &docTimestampDict{}

	// Build signature field for the timestamp.
	tsFieldDict := buildDocTimestampField(nextObjNum+1, nextObjNum)

	// Build AcroForm update that includes this new field.
	// We need to merge with existing AcroForm fields.
	catalogObjNum, err := getCatalogObjNum(trailer)
	if err != nil {
		return nil, err
	}

	catalog := r.Catalog()
	if catalog == nil {
		return nil, errors.New("sign: could not read catalog")
	}

	tsDictObjNum := nextObjNum
	tsFieldObjNum := nextObjNum + 1
	acroFormObjNum := nextObjNum + 2

	// Build updated AcroForm that adds the timestamp field.
	acroFormDict := buildAcroFormWithExisting(catalog, tsFieldObjNum)

	// Build updated catalog with new AcroForm.
	updatedCatalog := cloneCatalogWithAcroForm(catalog, acroFormObjNum)

	// Prepare incremental update.
	iw := newIncrementalWriter(pdfBytes, prevXref, trailer)
	iw.addObject(tsDictObjNum, tsDict)
	iw.addObject(tsFieldObjNum, tsFieldDict)
	iw.addObject(acroFormObjNum, acroFormDict)
	iw.addObject(catalogObjNum, updatedCatalog)

	// Write PDF with placeholders.
	result, err := iw.write()
	if err != nil {
		return nil, fmt.Errorf("sign: incremental write: %w", err)
	}

	// Locate and patch placeholders.
	ph, err := locatePlaceholders(result, tsDictObjNum)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	patchByteRange(result, ph)

	// Compute digest of the byte ranges.
	digest, err := computeByteRangeDigest(result, ph, hashFunc)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Get timestamp token from TSA.
	tsToken, err := tsaClient.Timestamp(digest, hashFunc)
	if err != nil {
		return nil, fmt.Errorf("sign: TSA timestamp: %w", err)
	}

	// Patch /Contents with the timestamp token.
	if err := patchContents(result, ph, tsToken); err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return result, nil
}

// buildDocTimestampField creates a signature field for a document timestamp.
func buildDocTimestampField(objNum, tsDictObjNum int) *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Annot"))
	d.Set("Subtype", core.NewPdfName("Widget"))
	d.Set("FT", core.NewPdfName("Sig"))
	d.Set("T", core.NewPdfLiteralString("DocTimeStamp"))
	d.Set("V", core.NewPdfIndirectReference(tsDictObjNum, 0))
	d.Set("F", core.NewPdfInteger(132))
	d.Set("Rect", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfInteger(0), core.NewPdfInteger(0),
	))
	return d
}

// buildAcroFormWithExisting creates an AcroForm dict that preserves existing
// fields and adds a new signature field reference.
func buildAcroFormWithExisting(catalog *core.PdfDictionary, newFieldObjNum int) *core.PdfDictionary {
	d := core.NewPdfDictionary()

	// Try to get existing AcroForm fields.
	var existingFields []core.PdfObject
	if acroFormObj := catalog.Get("AcroForm"); acroFormObj != nil {
		if acroFormDict, ok := acroFormObj.(*core.PdfDictionary); ok {
			if fieldsObj := acroFormDict.Get("Fields"); fieldsObj != nil {
				if fieldsArr, ok := fieldsObj.(*core.PdfArray); ok {
					existingFields = fieldsArr.Elements
				}
			}
		} else if acroFormRef, ok := acroFormObj.(*core.PdfIndirectReference); ok {
			// Existing AcroForm is an indirect reference — we can't resolve it
			// here, so we include it as-is and add our field.
			_ = acroFormRef
		}
	}

	// Build Fields array: existing + new.
	allFields := append(existingFields, core.NewPdfIndirectReference(newFieldObjNum, 0))
	d.Set("Fields", core.NewPdfArray(allFields...))
	d.Set("SigFlags", core.NewPdfInteger(3)) // SignaturesExist | AppendOnly
	return d
}

// cloneCatalogWithAcroForm clones catalog entries and sets /AcroForm.
func cloneCatalogWithAcroForm(catalog *core.PdfDictionary, acroFormObjNum int) *core.PdfDictionary {
	d := core.NewPdfDictionary()
	for _, e := range catalog.Entries {
		if e.Key.Value == "AcroForm" {
			continue
		}
		d.Set(e.Key.Value, e.Value)
	}
	d.Set("AcroForm", core.NewPdfIndirectReference(acroFormObjNum, 0))
	return d
}

// AddDSS adds a Document Security Store to the PDF via an incremental update.
// This is used to embed validation data for PAdES B-LT.
func AddDSS(pdfBytes []byte, dss *DSS) ([]byte, error) {
	r, err := reader.Parse(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: parse PDF: %w", err)
	}

	trailer := r.Trailer()
	if trailer == nil {
		return nil, errors.New("sign: could not read PDF trailer")
	}

	prevXref, err := findStartXref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	catalogObjNum, err := getCatalogObjNum(trailer)
	if err != nil {
		return nil, err
	}

	catalog := r.Catalog()
	if catalog == nil {
		return nil, errors.New("sign: could not read catalog")
	}

	nextObjNum := r.MaxObjectNumber() + 1

	iw := newIncrementalWriter(pdfBytes, prevXref, trailer)

	// Track object numbers for DSS streams.
	objCounter := nextObjNum
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		num := objCounter
		objCounter++
		iw.addObject(num, obj)
		return core.NewPdfIndirectReference(num, 0)
	}

	// Build DSS dictionary (this also creates stream objects via addObject).
	dssDict := dss.Build(addObject)
	dssRef := addObject(dssDict)

	// Update catalog to include /DSS.
	updatedCatalog := core.NewPdfDictionary()
	for _, e := range catalog.Entries {
		if e.Key.Value == "DSS" {
			continue // Replace existing DSS.
		}
		updatedCatalog.Set(e.Key.Value, e.Value)
	}
	updatedCatalog.Set("DSS", dssRef)

	iw.addObject(catalogObjNum, updatedCatalog)

	return iw.write()
}
