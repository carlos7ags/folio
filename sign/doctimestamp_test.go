// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
)

func signMinimalPDFForLTV(t *testing.T) []byte {
	t.Helper()
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	pdf := minimalPDF(t)
	signed, err := SignPDF(pdf, Options{
		Signer:      signer,
		Level:       LevelBB,
		Name:        "Test Signer",
		Reason:      "Testing",
		Location:    "Test Lab",
		SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SignPDF: %v", err)
	}
	return signed
}

func TestAddDocumentTimestamp(t *testing.T) {
	signed := signMinimalPDFForLTV(t)

	tsaSigner := testSigner(t)
	srv, _ := newTestTSA(t, tsaSigner)
	defer srv.Close()
	tsc := NewTSAClient(srv.URL)

	stamped, err := AddDocumentTimestamp(signed, tsc, crypto.SHA256)
	if err != nil {
		t.Fatalf("AddDocumentTimestamp: %v", err)
	}

	if len(stamped) <= len(signed) {
		t.Errorf("stamped PDF (%d bytes) not larger than signed (%d bytes)", len(stamped), len(signed))
	}
	if !bytes.Equal(stamped[:len(signed)], signed) {
		t.Error("incremental update is not append-only: stamped[:len(signed)] != signed")
	}

	if _, err := reader.Parse(stamped); err != nil {
		t.Errorf("reader.Parse(stamped): %v", err)
	}

	if !bytes.Contains(stamped, []byte("/DocTimeStamp")) {
		t.Error("stamped PDF missing /DocTimeStamp")
	}
	if !bytes.Contains(stamped, []byte("/ETSI.RFC3161")) {
		t.Error("stamped PDF missing /ETSI.RFC3161")
	}

	rep, err := Verify(stamped, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	// buildAcroFormWithExisting resolves the catalog's /AcroForm entry
	// (typically an indirect reference for a signed PDF) and carries the
	// original /Fields into the rebuilt AcroForm alongside the new
	// DocTimeStamp field, so both the original signature and the new
	// document timestamp remain reachable via the AcroForm graph.
	if len(rep.Signatures) != 2 {
		t.Fatalf("len(rep.Signatures) = %d, want 2", len(rep.Signatures))
	}

	var sawSignature, sawTimestamp bool
	for _, s := range rep.Signatures {
		if s.IsDocTimeStamp {
			sawTimestamp = true
			if !s.DigestValid {
				t.Error("document timestamp DigestValid = false, want true")
			}
			if !s.ByteRangeCoversFile {
				t.Error("document timestamp ByteRangeCoversFile = false, want true")
			}
		} else {
			sawSignature = true
			if !s.DigestValid {
				t.Error("original signature DigestValid = false, want true")
			}
			if !s.SignatureValid {
				t.Error("original signature SignatureValid = false, want true")
			}
		}
	}
	if !sawSignature {
		t.Error("expected one signature entry with IsDocTimeStamp = false (the original signature)")
	}
	if !sawTimestamp {
		t.Error("expected one signature entry with IsDocTimeStamp = true (the document timestamp)")
	}
}

// TestAddDocumentTimestamp_DirectAcroFormDict covers the case where the
// catalog's /AcroForm entry is a direct dictionary rather than an indirect
// reference, ensuring buildAcroFormWithExisting also preserves fields in
// that shape.
func TestAddDocumentTimestamp_DirectAcroFormDict(t *testing.T) {
	signed := signMinimalPDFForLTV(t)

	r, err := reader.Parse(signed)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	catalog := r.Catalog()
	if catalog == nil {
		t.Fatal("catalog is nil")
	}

	acroFormObj := catalog.Get("AcroForm")
	if _, isRef := acroFormObj.(*core.PdfIndirectReference); !isRef {
		t.Skip("catalog's /AcroForm is not an indirect reference in this fixture")
	}
	resolved, err := r.ResolveObject(acroFormObj)
	if err != nil {
		t.Fatalf("ResolveObject(AcroForm): %v", err)
	}
	acroFormDict, ok := resolved.(*core.PdfDictionary)
	if !ok {
		t.Fatal("resolved AcroForm is not a dictionary")
	}

	newFieldObjNum := r.MaxObjectNumber() + 100
	result := buildAcroFormWithExisting(nil, catalogWithDirectAcroForm(catalog, acroFormDict), newFieldObjNum)

	fieldsObj := result.Get("Fields")
	fieldsArr, ok := fieldsObj.(*core.PdfArray)
	if !ok {
		t.Fatal("Fields is not an array")
	}
	if fieldsArr.Len() < 2 {
		t.Fatalf("Fields array has %d entries, want at least 2 (existing + new)", fieldsArr.Len())
	}
}

// catalogWithDirectAcroForm returns a copy of catalog with /AcroForm set to
// the given dictionary directly (not an indirect reference), for testing the
// direct-dictionary code path of buildAcroFormWithExisting.
func catalogWithDirectAcroForm(catalog *core.PdfDictionary, acroForm *core.PdfDictionary) *core.PdfDictionary {
	d := core.NewPdfDictionary()
	for key, value := range catalog.All() {
		if key == "AcroForm" {
			continue
		}
		d.Set(key, value)
	}
	d.Set("AcroForm", acroForm)
	return d
}

func TestAddDocumentTimestamp_NilClient(t *testing.T) {
	signed := signMinimalPDFForLTV(t)
	_, err := AddDocumentTimestamp(signed, nil, crypto.SHA256)
	if err == nil || !strings.Contains(err.Error(), "TSAClient is required") {
		t.Fatalf("err = %v, want containing 'TSAClient is required'", err)
	}
}

func TestAddDocumentTimestamp_TSAFailure(t *testing.T) {
	signed := signMinimalPDFForLTV(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	tsc := NewTSAClient(srv.URL)

	result, err := AddDocumentTimestamp(signed, tsc, crypto.SHA256)
	if err == nil || !strings.Contains(err.Error(), "TSA timestamp") {
		t.Fatalf("err = %v, want containing 'TSA timestamp'", err)
	}
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestAddDocumentTimestamp_NotAPDF(t *testing.T) {
	tsaSigner := testSigner(t)
	srv, _ := newTestTSA(t, tsaSigner)
	defer srv.Close()
	tsc := NewTSAClient(srv.URL)

	_, err := AddDocumentTimestamp([]byte("junk"), tsc, crypto.SHA256)
	if err == nil || !strings.Contains(err.Error(), "parse PDF") {
		t.Fatalf("err = %v, want containing 'parse PDF'", err)
	}
}

func TestAddDSS_VerifyReportsHasDSS(t *testing.T) {
	signed := signMinimalPDFForLTV(t)

	dss := NewDSS()
	dss.AddSignatureValidation([]byte("fake-contents"), nil, nil, nil)

	stamped, err := AddDSS(signed, dss)
	if err != nil {
		t.Fatalf("AddDSS: %v", err)
	}

	if !bytes.Equal(stamped[:len(signed)], signed) {
		t.Error("incremental update is not append-only: stamped[:len(signed)] != signed")
	}

	rep, err := Verify(stamped, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(rep.Signatures) == 0 {
		t.Fatal("expected at least one signature result")
	}
	if !rep.Signatures[0].HasDSS {
		t.Error("Signatures[0].HasDSS = false, want true")
	}
}
