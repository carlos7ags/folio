// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
)

// --- Test helpers ---

// generateTestCert creates a self-signed certificate and private key for testing.
func generateTestRSACert(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test Signer",
			Organization: []string{"Folio Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return key, cert
}

func generateTestECDSACert(t *testing.T) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "Test ECDSA Signer",
			Organization: []string{"Folio Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return key, cert
}

// minimalPDF returns a valid minimal PDF produced by the document package.
func minimalPDF(t *testing.T) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.AddPage()
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("generate minimal PDF: %v", err)
	}
	return buf.Bytes()
}

// --- Algorithm tests ---

func TestAlgorithmHashFunc(t *testing.T) {
	tests := []struct {
		algo Algorithm
		want crypto.Hash
	}{
		{SHA256WithRSA, crypto.SHA256},
		{SHA384WithRSA, crypto.SHA384},
		{SHA512WithRSA, crypto.SHA512},
		{SHA256WithECDSA, crypto.SHA256},
		{SHA384WithECDSA, crypto.SHA384},
		{SHA512WithECDSA, crypto.SHA512},
	}
	for _, tt := range tests {
		if got := tt.algo.HashFunc(); got != tt.want {
			t.Errorf("Algorithm(%d).HashFunc() = %v, want %v", tt.algo, got, tt.want)
		}
	}
}

func TestAlgorithmOIDs(t *testing.T) {
	// Verify SHA256WithRSA returns correct OIDs.
	algo := SHA256WithRSA
	if !algo.DigestOID().Equal(oidSHA256) {
		t.Errorf("DigestOID = %v, want %v", algo.DigestOID(), oidSHA256)
	}
	if !algo.SignatureOID().Equal(oidSHA256WithRSA) {
		t.Errorf("SignatureOID = %v, want %v", algo.SignatureOID(), oidSHA256WithRSA)
	}

	// ECDSA variant.
	algo = SHA256WithECDSA
	if !algo.DigestOID().Equal(oidSHA256) {
		t.Errorf("ECDSA DigestOID = %v, want %v", algo.DigestOID(), oidSHA256)
	}
	if !algo.SignatureOID().Equal(oidECDSAWithSHA256) {
		t.Errorf("ECDSA SignatureOID = %v, want %v", algo.SignatureOID(), oidECDSAWithSHA256)
	}
}

// --- Signer tests ---

func TestNewLocalSignerRSA(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	if signer.Algorithm() != SHA256WithRSA {
		t.Errorf("Algorithm = %v, want SHA256WithRSA", signer.Algorithm())
	}
	if len(signer.CertificateChain()) != 1 {
		t.Errorf("CertificateChain length = %d, want 1", len(signer.CertificateChain()))
	}

	// Test signing.
	digest := hashBytes(crypto.SHA256, []byte("test data"))
	sig, err := signer.Sign(digest)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Error("Sign returned empty signature")
	}
}

func TestNewLocalSignerECDSA(t *testing.T) {
	key, cert := generateTestECDSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	if signer.Algorithm() != SHA256WithECDSA {
		t.Errorf("Algorithm = %v, want SHA256WithECDSA", signer.Algorithm())
	}

	digest := hashBytes(crypto.SHA256, []byte("test data"))
	sig, err := signer.Sign(digest)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Error("Sign returned empty signature")
	}
}

func TestNewLocalSignerNoCerts(t *testing.T) {
	key, _ := generateTestRSACert(t)
	_, err := NewLocalSigner(key, nil)
	if err == nil {
		t.Error("expected error for nil certs")
	}
}

func TestExternalSigner(t *testing.T) {
	_, cert := generateTestRSACert(t)
	signFn := func(digest []byte) ([]byte, error) {
		return []byte("fake-sig"), nil
	}
	signer, err := NewExternalSigner(signFn, []*x509.Certificate{cert}, SHA256WithRSA)
	if err != nil {
		t.Fatalf("NewExternalSigner: %v", err)
	}
	sig, err := signer.Sign([]byte("digest"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if string(sig) != "fake-sig" {
		t.Errorf("Sign = %q, want %q", sig, "fake-sig")
	}
}

// --- CMS builder tests ---

func TestBuildCMS_RSA(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	digest := hashBytes(crypto.SHA256, []byte("test document content"))
	signingTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	cms, err := buildCMS(digest, signer, signingTime, nil)
	if err != nil {
		t.Fatalf("buildCMS: %v", err)
	}

	// Verify it's valid ASN.1 by parsing the outer ContentInfo.
	var ci contentInfo
	rest, err := asn1.Unmarshal(cms, &ci)
	if err != nil {
		t.Fatalf("unmarshal ContentInfo: %v", err)
	}
	if len(rest) != 0 {
		t.Errorf("trailing bytes after ContentInfo: %d", len(rest))
	}
	if !ci.ContentType.Equal(oidSignedData) {
		t.Errorf("ContentType = %v, want %v", ci.ContentType, oidSignedData)
	}
}

func TestBuildCMS_ECDSA(t *testing.T) {
	key, cert := generateTestECDSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	digest := hashBytes(crypto.SHA256, []byte("test document content"))
	signingTime := time.Now()

	cms, err := buildCMS(digest, signer, signingTime, nil)
	if err != nil {
		t.Fatalf("buildCMS: %v", err)
	}

	var ci contentInfo
	_, err = asn1.Unmarshal(cms, &ci)
	if err != nil {
		t.Fatalf("unmarshal ContentInfo: %v", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		t.Errorf("ContentType = %v, want %v", ci.ContentType, oidSignedData)
	}
}

func TestBuildCMS_NoCerts(t *testing.T) {
	_, cert := generateTestRSACert(t)
	signer, _ := NewExternalSigner(
		func(d []byte) ([]byte, error) { return []byte("sig"), nil },
		[]*x509.Certificate{cert},
		SHA256WithRSA,
	)
	// Override to empty chain to test the error path.
	signer.certs = nil

	digest := hashBytes(crypto.SHA256, []byte("test"))
	_, err := buildCMS(digest, signer, time.Now(), nil)
	if err == nil {
		t.Error("expected error for no certificates")
	}
}

// --- Incremental writer tests ---

func TestIncrementalWriter(t *testing.T) {
	pdf := minimalPDF(t)

	prevXref, err := findStartXref(pdf)
	if err != nil {
		t.Fatalf("findStartXref: %v", err)
	}

	// Simulate a trailer.
	trailer := buildFakeTrailer()

	iw := newIncrementalWriter(pdf, prevXref, trailer)

	// Add a new object.
	testDict := newTestDict()
	iw.addObject(5, testDict)

	result, err := iw.write()
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// The result should start with the original PDF.
	if !bytes.HasPrefix(result, pdf) {
		t.Error("result does not start with original PDF")
	}

	// Should contain the new object.
	if !bytes.Contains(result, []byte("5 0 obj")) {
		t.Error("result does not contain new object")
	}

	// Should have xref, trailer, startxref.
	if !bytes.Contains(result, []byte("xref")) {
		t.Error("result does not contain xref")
	}
	if !bytes.Contains(result, []byte("trailer")) {
		t.Error("result does not contain trailer")
	}
	if !bytes.Contains(result, []byte("/Prev")) {
		t.Error("result does not contain /Prev in trailer")
	}
	if !bytes.Contains(result, []byte("startxref")) {
		t.Error("result does not contain startxref")
	}
}

func TestFindStartXref(t *testing.T) {
	pdf := minimalPDF(t)
	offset, err := findStartXref(pdf)
	if err != nil {
		t.Fatalf("findStartXref: %v", err)
	}
	if offset <= 0 {
		t.Errorf("startxref offset = %d, want > 0", offset)
	}
}

// --- Placeholder tests ---

func TestSigDictWriteTo(t *testing.T) {
	sd := buildSigDict("Test User", "Test City", "Testing", "test@example.com")

	var buf bytes.Buffer
	_, err := sd.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	output := buf.String()

	// Check required fields.
	checks := []string{
		"/Type /Sig",
		"/Filter /Adobe.PPKLite",
		"/SubFilter /ETSI.CAdES.detached",
		"/Name (Test User)",
		"/Location (Test City)",
		"/Reason (Testing)",
		"/ContactInfo (test@example.com)",
		"/ByteRange",
		"/Contents <",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(output), []byte(check)) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestPatchByteRange(t *testing.T) {
	sd := buildSigDict("", "", "", "")
	var buf bytes.Buffer
	buf.WriteString("5 0 obj\n")
	sd.WriteTo(&buf)
	buf.WriteString("\nendobj\n")

	pdf := buf.Bytes()

	ph, err := locatePlaceholders(pdf, 5)
	if err != nil {
		t.Fatalf("locatePlaceholders: %v", err)
	}

	patchByteRange(pdf, ph)

	// After patching, /ByteRange should contain real values.
	brArea := string(pdf[ph.ByteRangeOffset : ph.ByteRangeOffset+len(byteRangePlaceholder)])
	if brArea[0] != '[' {
		t.Errorf("patched ByteRange does not start with [: %q", brArea)
	}
	// The first value should be 0000000000 (offset 0).
	if !bytes.Contains([]byte(brArea), []byte("0000000000")) {
		t.Errorf("patched ByteRange missing offset 0: %q", brArea)
	}
}

func TestPatchContents(t *testing.T) {
	sd := buildSigDict("", "", "", "")
	var buf bytes.Buffer
	buf.WriteString("5 0 obj\n")
	sd.WriteTo(&buf)
	buf.WriteString("\nendobj\n")

	pdf := buf.Bytes()

	ph, err := locatePlaceholders(pdf, 5)
	if err != nil {
		t.Fatalf("locatePlaceholders: %v", err)
	}

	sig := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if err := patchContents(pdf, ph, sig); err != nil {
		t.Fatalf("patchContents: %v", err)
	}

	// The hex string should start with DEADBEEF followed by zeros.
	contentsArea := string(pdf[ph.ContentsOffset+1 : ph.ContentsOffset+9])
	if contentsArea != "DEADBEEF" {
		t.Errorf("patched Contents = %q, want DEADBEEF...", contentsArea)
	}
}

func TestPatchContentsTooLarge(t *testing.T) {
	sd := buildSigDict("", "", "", "")
	var buf bytes.Buffer
	buf.WriteString("5 0 obj\n")
	sd.WriteTo(&buf)
	buf.WriteString("\nendobj\n")

	pdf := buf.Bytes()

	ph, err := locatePlaceholders(pdf, 5)
	if err != nil {
		t.Fatalf("locatePlaceholders: %v", err)
	}

	// Create a signature that's too large.
	sig := make([]byte, contentsPlaceholderLen/2+1)
	if err := patchContents(pdf, ph, sig); err == nil {
		t.Error("expected error for oversized signature")
	}
}

// --- SignPDF integration test ---

func TestSignPDF_BB(t *testing.T) {
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

	// Signed PDF should be larger than original.
	if len(signed) <= len(pdf) {
		t.Errorf("signed PDF (%d bytes) not larger than original (%d bytes)", len(signed), len(pdf))
	}

	// Should start with original PDF.
	if !bytes.HasPrefix(signed, pdf) {
		t.Error("signed PDF does not start with original")
	}

	// Should contain signature dictionary markers.
	if !bytes.Contains(signed, []byte("/Type /Sig")) {
		t.Error("signed PDF missing /Type /Sig")
	}
	if !bytes.Contains(signed, []byte("/SubFilter /ETSI.CAdES.detached")) {
		t.Error("signed PDF missing PAdES SubFilter")
	}
	if !bytes.Contains(signed, []byte("/ByteRange")) {
		t.Error("signed PDF missing /ByteRange")
	}

	// /Contents should not be all zeros (signature was patched).
	contentsIdx := bytes.Index(signed, []byte("/Contents <"))
	if contentsIdx < 0 {
		t.Fatal("signed PDF missing /Contents")
	}
	hexStart := contentsIdx + len("/Contents <")
	// Check first few hex chars are not all zeros.
	hexArea := signed[hexStart : hexStart+16]
	allZero := true
	for _, b := range hexArea {
		if b != '0' {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("/Contents appears to be all zeros — signature not patched")
	}
}

func TestSignPDF_ECDSA(t *testing.T) {
	key, cert := generateTestECDSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	signed, err := SignPDF(minimalPDF(t), Options{
		Signer: signer,
		Level:  LevelBB,
	})
	if err != nil {
		t.Fatalf("SignPDF: %v", err)
	}
	if !bytes.Contains(signed, []byte("/Type /Sig")) {
		t.Error("signed PDF missing /Type /Sig")
	}
}

func TestSignPDF_NilSigner(t *testing.T) {
	_, err := SignPDF(minimalPDF(t), Options{})
	if err == nil {
		t.Error("expected error for nil signer")
	}
}

func TestSignPDF_BTWithoutTSA(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, _ := NewLocalSigner(key, []*x509.Certificate{cert})

	_, err := SignPDF(minimalPDF(t), Options{
		Signer: signer,
		Level:  LevelBT,
	})
	if err == nil {
		t.Error("expected error for B-T without TSA")
	}
}

// --- TLV / ASN.1 helper tests ---

func TestMarshalTLV(t *testing.T) {
	// Short form: content < 128 bytes.
	content := []byte{0x01, 0x02, 0x03}
	tlv := marshalTLV(0x30, content)
	if tlv[0] != 0x30 {
		t.Errorf("tag = 0x%02X, want 0x30", tlv[0])
	}
	if tlv[1] != 3 {
		t.Errorf("length = %d, want 3", tlv[1])
	}
	if !bytes.Equal(tlv[2:], content) {
		t.Errorf("content mismatch")
	}

	// Long form: content >= 128 bytes.
	bigContent := make([]byte, 200)
	tlv = marshalTLV(0x31, bigContent)
	if tlv[0] != 0x31 {
		t.Errorf("tag = 0x%02X, want 0x31", tlv[0])
	}
	if tlv[1]&0x80 == 0 {
		t.Error("expected long form length encoding")
	}
}

func TestStripTag(t *testing.T) {
	// Short form.
	data := []byte{0x31, 0x03, 0xAA, 0xBB, 0xCC}
	inner := stripTag(data)
	if !bytes.Equal(inner, []byte{0xAA, 0xBB, 0xCC}) {
		t.Errorf("stripTag short = %X, want AABBCC", inner)
	}

	// Long form.
	data = []byte{0x31, 0x81, 0x80}
	data = append(data, make([]byte, 128)...)
	inner = stripTag(data)
	if len(inner) != 128 {
		t.Errorf("stripTag long len = %d, want 128", len(inner))
	}
}

// --- DSS tests ---

func TestDSSBuild(t *testing.T) {
	dss := NewDSS()

	// Add some fake data.
	cert1 := []byte("cert-1-der")
	cert2 := []byte("cert-2-der")
	ocsp1 := []byte("ocsp-1-der")
	sigContents := []byte("fake-signature-contents")

	dss.AddSignatureValidation(sigContents, []*x509.Certificate{
		{Raw: cert1}, {Raw: cert2},
	}, [][]byte{ocsp1}, nil)

	if len(dss.Certs) != 2 {
		t.Errorf("Certs count = %d, want 2", len(dss.Certs))
	}
	if len(dss.OCSPs) != 1 {
		t.Errorf("OCSPs count = %d, want 1", len(dss.OCSPs))
	}
	if len(dss.VRI) != 1 {
		t.Errorf("VRI count = %d, want 1", len(dss.VRI))
	}

	// Build the DSS dictionary.
	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		num := len(objects) + 100
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(num, 0)
	}

	dssDict := dss.Build(addObject)
	if dssDict.Get("Certs") == nil {
		t.Error("DSS missing /Certs")
	}
	if dssDict.Get("OCSPs") == nil {
		t.Error("DSS missing /OCSPs")
	}
	if dssDict.Get("VRI") == nil {
		t.Error("DSS missing /VRI")
	}
}

func TestDSSDeduplicate(t *testing.T) {
	dss := NewDSS()

	cert := []byte("same-cert-der")
	sig1 := []byte("sig-1")
	sig2 := []byte("sig-2")

	dss.AddSignatureValidation(sig1, []*x509.Certificate{{Raw: cert}}, nil, nil)
	dss.AddSignatureValidation(sig2, []*x509.Certificate{{Raw: cert}}, nil, nil)

	// The same cert should appear only once in the global list.
	if len(dss.Certs) != 1 {
		t.Errorf("Certs count = %d, want 1 (dedup)", len(dss.Certs))
	}
	// But two VRI entries.
	if len(dss.VRI) != 2 {
		t.Errorf("VRI count = %d, want 2", len(dss.VRI))
	}
}

func TestComputeVRIKey(t *testing.T) {
	key := computeVRIKey([]byte("test"))
	// SHA-1 of "test" = A94A8FE5CCB19BA61C4C0873D391E987982FBBD3
	if len(key) != 40 {
		t.Errorf("VRI key length = %d, want 40", len(key))
	}
	// Must be uppercase hex.
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("VRI key has non-uppercase-hex char: %c", c)
			break
		}
	}
}

func TestAddDSS(t *testing.T) {
	pdf := minimalPDF(t)

	dss := NewDSS()
	dss.addCert([]byte("test-cert"))

	result, err := AddDSS(pdf, dss)
	if err != nil {
		t.Fatalf("AddDSS: %v", err)
	}

	if len(result) <= len(pdf) {
		t.Error("result should be larger than input")
	}
	if !bytes.Contains(result, []byte("/DSS")) {
		t.Error("result missing /DSS in catalog")
	}
}

// --- OCSP tests ---

func TestBuildOCSPRequest(t *testing.T) {
	_, cert := generateTestRSACert(t)
	_, issuer := generateTestRSACert(t)

	reqDER, err := buildOCSPRequest(cert, issuer)
	if err != nil {
		t.Fatalf("buildOCSPRequest: %v", err)
	}
	if len(reqDER) == 0 {
		t.Error("OCSP request is empty")
	}

	// Should be valid ASN.1.
	var req ocspRequest
	_, err = asn1.Unmarshal(reqDER, &req)
	if err != nil {
		t.Fatalf("unmarshal OCSP request: %v", err)
	}
	if len(req.TBSRequest.RequestList) != 1 {
		t.Errorf("RequestList length = %d, want 1", len(req.TBSRequest.RequestList))
	}
}

func TestValidateOCSPResponse(t *testing.T) {
	// Build a minimal valid OCSP response (status=successful).
	resp := ocspResponse{ResponseStatus: 0}
	der, _ := asn1.Marshal(resp)
	if err := validateOCSPResponse(der); err != nil {
		t.Errorf("validate successful response: %v", err)
	}

	// Build a failed response.
	resp.ResponseStatus = 2 // malformedRequest
	der, _ = asn1.Marshal(resp)
	if err := validateOCSPResponse(der); err == nil {
		t.Error("expected error for failed OCSP response")
	}
}

// --- Document timestamp tests ---

func TestDocTimestampDictWriteTo(t *testing.T) {
	d := &docTimestampDict{}
	var buf bytes.Buffer
	_, err := d.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	output := buf.String()
	checks := []string{
		"/Type /DocTimeStamp",
		"/Filter /Adobe.PPKLite",
		"/SubFilter /ETSI.RFC3161",
		"/ByteRange",
		"/Contents <",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(output), []byte(check)) {
			t.Errorf("output missing %q", check)
		}
	}
}

// --- TSA tests ---

func TestBuildTimestampReq(t *testing.T) {
	digest := hashBytes(crypto.SHA256, []byte("test data"))
	reqDER, err := buildTimestampReq(digest, crypto.SHA256)
	if err != nil {
		t.Fatalf("buildTimestampReq: %v", err)
	}
	if len(reqDER) == 0 {
		t.Error("timestamp request is empty")
	}

	// Parse back.
	var req timeStampReq
	_, err = asn1.Unmarshal(reqDER, &req)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Version != 1 {
		t.Errorf("version = %d, want 1", req.Version)
	}
	if !req.CertReq {
		t.Error("CertReq should be true")
	}
}

func TestBuildTimestampReqUnsupported(t *testing.T) {
	_, err := buildTimestampReq([]byte("x"), crypto.SHA1)
	if err == nil {
		t.Error("expected error for unsupported hash")
	}
}

// --- SignPDF B-LT integration test ---

func TestSignPDF_BLT(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, _ := NewLocalSigner(key, []*x509.Certificate{cert})

	// B-LT requires TSA — we can't hit a real TSA in unit tests,
	// so we test the validation path separately.
	// Instead, test that B-LT without TSA returns an error.
	_, err := SignPDF(minimalPDF(t), Options{
		Signer: signer,
		Level:  LevelBLT,
	})
	if err == nil {
		t.Error("expected error for B-LT without TSA")
	}
}

// --- Helpers ---

func buildFakeTrailer() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Size", core.NewPdfInteger(5))
	d.Set("Root", core.NewPdfIndirectReference(1, 0))
	return d
}

func newTestDict() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Test"))
	return d
}
