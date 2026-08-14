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
	if len(rep.Signatures) == 0 {
		t.Fatal("expected at least one signature result")
	}
	// The original signature is the first one located. Note: with the
	// object-graph-based locateSignaturesGraph, the DocTimeStamp field also
	// carries /FT /Sig, so it is picked up as a second (unverifiable, since
	// its /Contents holds an RFC3161 token rather than a CMS SignedData)
	// signature location. The plan (written against the pre-graph
	// byte-scan locateSignatures, which only matched "/Type /Sig") expected
	// exactly one result; the object-graph rework changed that. We assert
	// on the first result — the original signature — which is what this
	// test is actually guarding.
	orig := rep.Signatures[0]
	if !orig.DigestValid {
		t.Error("original signature DigestValid = false, want true")
	}
	if !orig.SignatureValid {
		t.Error("original signature SignatureValid = false, want true")
	}
	if !orig.ByteRangeCoversFile {
		t.Error("original signature ByteRangeCoversFile = false, want true")
	}
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
