// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package sign implements PAdES digital signatures for PDF documents.
//
// The signing flow uses incremental updates (PDF appends) so existing
// content and any prior signatures are preserved. The CMS/PKCS#7
// detached signature is built from scratch using encoding/asn1 — no
// external cryptography dependencies beyond the Go standard library.
//
// PAdES conformance levels:
//   - B-B: basic signature (always)
//   - B-T: adds a trusted timestamp from an RFC 3161 TSA
//   - B-LT: embeds revocation data via a Document Security Store
//   - B-LTA: adds a document timestamp for archival
package sign

import (
	"bytes"
	"crypto"
	"errors"
	"fmt"
	"time"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
)

// PAdESLevel specifies the PAdES conformance level.
type PAdESLevel int

const (
	// LevelBB is PAdES B-B (basic signature with ESS signing-certificate-v2).
	LevelBB PAdESLevel = iota

	// LevelBT is PAdES B-T (B-B + RFC 3161 timestamp).
	LevelBT

	// LevelBLT is PAdES B-LT (B-T + embedded revocation data in DSS).
	LevelBLT

	// LevelBLTA is PAdES B-LTA (B-LT + document timestamp).
	LevelBLTA
)

// Options configures the signing operation.
type Options struct {
	// Signer performs the cryptographic signing.
	Signer Signer

	// Level is the PAdES conformance level (default: LevelBB).
	Level PAdESLevel

	// Name is the signer's name (optional, shown in PDF viewer).
	Name string

	// Reason is the reason for signing (optional).
	Reason string

	// Location is the signing location (optional).
	Location string

	// ContactInfo is the signer's contact info (optional).
	ContactInfo string

	// SigningTime overrides the signing timestamp. If zero, time.Now() is used.
	SigningTime time.Time

	// TSAClient is the RFC 3161 TSA client (required for B-T and above).
	TSAClient *TSAClient

	// OCSPClient fetches OCSP responses (optional, used for B-LT and above).
	// If nil and Level >= LevelBLT, OCSP data is omitted from the DSS.
	OCSPClient *OCSPClient

	// CRLs provides pre-fetched CRL data (DER-encoded) for embedding in
	// the DSS. Optional alternative to OCSP for revocation data.
	CRLs [][]byte

	// ExtraCerts provides additional certificates to embed in the DSS
	// (e.g., intermediate CAs not in the signer's chain).
	ExtraCerts [][]byte
}

// SignPDF applies a PAdES digital signature to an existing PDF.
// It returns the signed PDF bytes (original + incremental update).
func SignPDF(pdfBytes []byte, opts Options) ([]byte, error) {
	if opts.Signer == nil {
		return nil, errors.New("sign: Signer is required")
	}
	if opts.Level >= LevelBT && opts.TSAClient == nil {
		return nil, errors.New("sign: TSAClient is required for PAdES B-T and above")
	}

	signingTime := opts.SigningTime
	if signingTime.IsZero() {
		signingTime = time.Now()
	}

	// Parse the existing PDF to get trailer and structure info.
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

	// Object layout:
	//   nextObjNum     = signature dictionary (with ByteRange/Contents placeholders)
	//   nextObjNum + 1 = signature field (widget annotation, /V → sig dict)
	//   nextObjNum + 2 = AcroForm dictionary (/Fields → sig field, /SigFlags 3)
	//   catalog obj    = updated catalog (overrides original, adds /AcroForm)
	sigDictObjNum := nextObjNum
	sigFieldObjNum := nextObjNum + 1
	acroFormObjNum := nextObjNum + 2

	// Get the catalog object number so we can override it in-place.
	catalogObjNum, err := getCatalogObjNum(trailer)
	if err != nil {
		return nil, err
	}

	// Build objects.
	sigDict := buildSigDict(opts.Name, opts.Location, opts.Reason, opts.ContactInfo)
	sigFieldDict := buildSigField(sigFieldObjNum, sigDictObjNum)
	acroFormDict := buildAcroForm(r, sigFieldObjNum)

	updatedCatalog, err := buildCatalogWithAcroForm(r, acroFormObjNum)
	if err != nil {
		return nil, err
	}

	// Prepare incremental update.
	iw := newIncrementalWriter(pdfBytes, prevXref, trailer)
	iw.addObject(sigDictObjNum, sigDict)
	iw.addObject(sigFieldObjNum, sigFieldDict)
	iw.addObject(acroFormObjNum, acroFormDict)
	iw.addObject(catalogObjNum, updatedCatalog) // overrides original catalog

	// Write the PDF with placeholders.
	signedPDF, err := iw.write()
	if err != nil {
		return nil, fmt.Errorf("sign: incremental write: %w", err)
	}

	// Find placeholder positions in the output.
	ph, err := locatePlaceholders(signedPDF, sigDictObjNum)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Patch /ByteRange with real offsets.
	patchByteRange(signedPDF, ph)

	// Compute the document digest over the byte ranges.
	digest, err := computeByteRangeDigest(signedPDF, ph, opts.Signer.Algorithm().HashFunc())
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Get TSA token if needed (B-T and above).
	var tsaToken []byte
	if opts.Level >= LevelBT && opts.TSAClient != nil {
		tsaToken, err = opts.TSAClient.Timestamp(digest, opts.Signer.Algorithm().HashFunc())
		if err != nil {
			return nil, fmt.Errorf("sign: TSA timestamp: %w", err)
		}
	}

	// Build the CMS detached signature.
	cmsSig, err := buildCMS(digest, opts.Signer, signingTime, tsaToken)
	if err != nil {
		return nil, fmt.Errorf("sign: build CMS: %w", err)
	}

	// Patch /Contents with the actual signature.
	if err := patchContents(signedPDF, ph, cmsSig); err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// B-LT: add Document Security Store with validation data.
	if opts.Level >= LevelBLT {
		signedPDF, err = addValidationData(signedPDF, opts, sigDictObjNum, cmsSig)
		if err != nil {
			return nil, err
		}
	}

	// B-LTA: add document timestamp.
	if opts.Level >= LevelBLTA {
		signedPDF, err = AddDocumentTimestamp(signedPDF, opts.TSAClient, opts.Signer.Algorithm().HashFunc())
		if err != nil {
			return nil, fmt.Errorf("sign: document timestamp: %w", err)
		}
	}

	return signedPDF, nil
}

// addValidationData collects and embeds validation data (DSS) for B-LT.
func addValidationData(pdfBytes []byte, opts Options, sigDictObjNum int, sigContents []byte) ([]byte, error) {
	dss := NewDSS()

	chain := opts.Signer.CertificateChain()

	// Collect OCSP responses.
	var ocspResponses [][]byte
	if opts.OCSPClient != nil {
		var err error
		ocspResponses, err = opts.OCSPClient.FetchChainResponses(chain)
		if err != nil {
			return nil, fmt.Errorf("sign: collect OCSP data: %w", err)
		}
	}

	// Add validation data for this signature.
	dss.AddSignatureValidation(sigContents, chain, ocspResponses, opts.CRLs)

	// Add any extra certificates.
	for _, certDER := range opts.ExtraCerts {
		dss.addCert(certDER)
	}

	return AddDSS(pdfBytes, dss)
}

// buildSigField creates a signature field dictionary with a widget annotation.
// The field name includes the object number to ensure uniqueness across
// multiple signatures on the same document.
func buildSigField(objNum, sigDictObjNum int) *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Annot"))
	d.Set("Subtype", core.NewPdfName("Widget"))
	d.Set("FT", core.NewPdfName("Sig"))
	d.Set("T", core.NewPdfLiteralString(fmt.Sprintf("Signature%d", objNum)))
	d.Set("V", core.NewPdfIndirectReference(sigDictObjNum, 0))
	d.Set("F", core.NewPdfInteger(132)) // Print + Locked
	// Invisible signature (zero-size rectangle).
	d.Set("Rect", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfInteger(0), core.NewPdfInteger(0),
	))
	return d
}

// buildAcroForm creates an AcroForm dictionary referencing the signature field.
// If the PDF already has an AcroForm with /Fields, existing fields are preserved
// so that prior signatures remain visible.
func buildAcroForm(r *reader.PdfReader, sigFieldObjNum int) *core.PdfDictionary {
	d := core.NewPdfDictionary()

	// Collect existing fields from prior AcroForm (if any).
	fields := core.NewPdfArray(core.NewPdfIndirectReference(sigFieldObjNum, 0))
	if catalog := r.Catalog(); catalog != nil {
		if af := catalog.Get("AcroForm"); af != nil {
			if afDict, ok := af.(*core.PdfDictionary); ok {
				if existingFields, ok := afDict.Get("Fields").(*core.PdfArray); ok {
					for _, f := range existingFields.All() {
						fields.Add(f)
					}
				}
			}
			// Also try resolving indirect reference to AcroForm.
			if afRef, ok := af.(*core.PdfIndirectReference); ok {
				if resolved, err := r.ResolveObject(afRef); err == nil {
					if afDict, ok := resolved.(*core.PdfDictionary); ok {
						if existingFields, ok := afDict.Get("Fields").(*core.PdfArray); ok {
							for _, f := range existingFields.All() {
								fields.Add(f)
							}
						}
					}
				}
			}
		}
	}

	d.Set("Fields", fields)
	d.Set("SigFlags", core.NewPdfInteger(3)) // SignaturesExist | AppendOnly
	return d
}

// getCatalogObjNum extracts the catalog object number from the trailer /Root.
func getCatalogObjNum(trailer *core.PdfDictionary) (int, error) {
	root := trailer.Get("Root")
	if root == nil {
		return 0, errors.New("sign: trailer has no /Root")
	}
	ref, ok := root.(*core.PdfIndirectReference)
	if !ok {
		return 0, errors.New("sign: trailer /Root is not an indirect reference")
	}
	return ref.Num(), nil
}

// buildCatalogWithAcroForm clones the original catalog and adds /AcroForm.
func buildCatalogWithAcroForm(r *reader.PdfReader, acroFormObjNum int) (*core.PdfDictionary, error) {
	catalog := r.Catalog()
	if catalog == nil {
		return nil, errors.New("sign: could not read catalog")
	}

	d := core.NewPdfDictionary()
	for key, value := range catalog.All() {
		if key == "AcroForm" {
			continue // Replace with our own.
		}
		d.Set(key, value)
	}
	d.Set("AcroForm", core.NewPdfIndirectReference(acroFormObjNum, 0))
	return d, nil
}

// locatePlaceholders scans the PDF output to find the byte positions
// of /ByteRange and /Contents placeholders in the signature dictionary.
func locatePlaceholders(pdf []byte, sigDictObjNum int) (signaturePlaceholder, error) {
	var ph signaturePlaceholder
	ph.SigDictObjNum = sigDictObjNum

	objHeader := fmt.Sprintf("%d 0 obj", sigDictObjNum)
	objStart := bytes.Index(pdf, []byte(objHeader))
	if objStart < 0 {
		return ph, fmt.Errorf("could not find signature object %d", sigDictObjNum)
	}

	searchArea := pdf[objStart:]

	brMarker := []byte("/ByteRange ")
	brIdx := bytes.Index(searchArea, brMarker)
	if brIdx < 0 {
		return ph, errors.New("could not find /ByteRange placeholder")
	}
	ph.ByteRangeOffset = objStart + brIdx + len(brMarker)

	contentsMarker := []byte("/Contents <")
	cIdx := bytes.Index(searchArea, contentsMarker)
	if cIdx < 0 {
		return ph, errors.New("could not find /Contents placeholder")
	}
	ph.ContentsOffset = objStart + cIdx + len("/Contents ")
	ph.ContentsLen = len(contentsPlaceholder)

	return ph, nil
}

// computeByteRangeDigest hashes the PDF bytes covered by the /ByteRange.
func computeByteRangeDigest(pdf []byte, ph signaturePlaceholder, hashFunc crypto.Hash) ([]byte, error) {
	contentsStart := ph.ContentsOffset
	contentsEnd := ph.ContentsOffset + ph.ContentsLen

	h := hashFunc.New()
	h.Write(pdf[:contentsStart])
	h.Write(pdf[contentsEnd:])

	return h.Sum(nil), nil
}
