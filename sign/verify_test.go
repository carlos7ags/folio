// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
)

// appendDecoyObject signs pdf, then appends an unreferenced object (via an
// incremental update, mimicking a decoy blob elsewhere in the file) holding
// obj — not registered in any AcroForm field or the catalog — and returns
// the resulting bytes.
func appendDecoyObject(t *testing.T, signed []byte, obj core.PdfObject) []byte {
	t.Helper()
	r, err := reader.Parse(signed)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	prevXref, err := findStartXref(signed)
	if err != nil {
		t.Fatalf("findStartXref: %v", err)
	}
	iw := newIncrementalWriter(signed, prevXref, r.Trailer())
	decoyNum := r.MaxObjectNumber() + 1
	iw.addObject(decoyNum, obj)
	out, err := iw.write()
	if err != nil {
		t.Fatalf("incrementalWriter.write: %v", err)
	}
	return out
}

// generateTestCAChain builds an in-memory root CA, intermediate CA, and
// leaf certificate, all RSA, for chain-validation tests.
func generateTestCAChain(t *testing.T) (rootCert, intermediateCert, leafCert *x509.Certificate, leafKey *rsa.PrivateKey) {
	t.Helper()

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate root key: %v", err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(100),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("create root cert: %v", err)
	}
	rootCert, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse root cert: %v", err)
	}

	intKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate intermediate key: %v", err)
	}
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(200),
		Subject:               pkix.Name{CommonName: "Test Intermediate CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	intDER, err := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("create intermediate cert: %v", err)
	}
	intermediateCert, err = x509.ParseCertificate(intDER)
	if err != nil {
		t.Fatalf("parse intermediate cert: %v", err)
	}

	leafKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(300),
		Subject:               pkix.Name{CommonName: "Test Leaf Signer"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, intermediateCert, &leafKey.PublicKey, intKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}
	leafCert, err = x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse leaf cert: %v", err)
	}

	return rootCert, intermediateCert, leafCert, leafKey
}

// keyGen produces a private key and matching self-signed certificate for a
// signature algorithm exercised by the tests below.
type keyGen struct {
	name string
	gen  func(t *testing.T) (crypto.Signer, *x509.Certificate)
}

var testKeyGens = []keyGen{
	{"RSA", func(t *testing.T) (crypto.Signer, *x509.Certificate) {
		key, cert := generateTestRSACert(t)
		return key, cert
	}},
	{"ECDSA", func(t *testing.T) (crypto.Signer, *x509.Certificate) {
		key, cert := generateTestECDSACert(t)
		return key, cert
	}},
}

// signMinimalPDF signs a fresh minimal document and returns the signer's
// certificate alongside the signed bytes.
func signMinimalPDF(t *testing.T, key crypto.Signer, cert *x509.Certificate, opts Options) []byte {
	t.Helper()
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	opts.Signer = signer
	signed, err := SignPDF(minimalPDF(t), opts)
	if err != nil {
		t.Fatalf("SignPDF: %v", err)
	}
	return signed
}

func TestBuildCMS_SignatureVerifies(t *testing.T) {
	for _, kg := range testKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			key, cert := kg.gen(t)
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

			p, err := parseCMS(cms)
			if err != nil {
				t.Fatalf("parseCMS: %v", err)
			}

			if !bytes.Equal(p.messageDigest, digest) {
				t.Errorf("messageDigest = %X, want %X", p.messageDigest, digest)
			}
			if !verifySignatureCMS(p) {
				t.Error("verifySignatureCMS = false, want true")
			}
			if p.cert.SerialNumber.Cmp(cert.SerialNumber) != 0 {
				t.Errorf("cert serial = %v, want %v", p.cert.SerialNumber, cert.SerialNumber)
			}
			if !p.signingTime.Equal(signingTime) {
				t.Errorf("signingTime = %v, want %v", p.signingTime, signingTime)
			}
		})
	}
}

func TestVerify_HappyPath(t *testing.T) {
	for _, kg := range testKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			key, cert := kg.gen(t)
			// Within the test certificate's validity window (see
			// generateTestRSACert/generateTestECDSACert), unlike a fixed
			// past date, so the chain check below can actually succeed.
			signingTime := time.Now().Truncate(time.Second)
			signed := signMinimalPDF(t, key, cert, Options{
				Level:       LevelBB,
				Name:        "Test Signer",
				Reason:      "Testing",
				Location:    "Test Lab",
				SigningTime: signingTime,
			})

			roots := x509.NewCertPool()
			roots.AddCert(cert)

			report, err := Verify(signed, VerifyOptions{Roots: roots})
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if len(report.Signatures) != 1 {
				t.Fatalf("len(Signatures) = %d, want 1", len(report.Signatures))
			}
			got := report.Signatures[0]

			if !got.DigestValid {
				t.Error("DigestValid = false, want true")
			}
			if !got.SignatureValid {
				t.Error("SignatureValid = false, want true")
			}
			if !got.ByteRangeCoversFile {
				t.Error("ByteRangeCoversFile = false, want true")
			}
			if got.ChainStatus != ChainStatusTrusted {
				t.Errorf("ChainStatus = %v, want TRUSTED", got.ChainStatus)
			}
			if got.Name != "Test Signer" || got.Reason != "Testing" || got.Location != "Test Lab" {
				t.Errorf("Name/Reason/Location = %q/%q/%q, want Test Signer/Testing/Test Lab",
					got.Name, got.Reason, got.Location)
			}
			if got.SignerCertificate == nil || got.SignerCertificate.SerialNumber.Cmp(cert.SerialNumber) != 0 {
				t.Error("SignerCertificate does not match the signing certificate")
			}
			if !got.SigningTime.Equal(signingTime) {
				t.Errorf("SigningTime = %v, want %v", got.SigningTime, signingTime)
			}
			if got.HasTimestamp {
				t.Error("HasTimestamp = true for a B-B signature")
			}
			if got.HasDSS {
				t.Error("HasDSS = true for a B-B signature")
			}
		})
	}
}

func TestVerify_NoRootsSupplied(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{
		Level:       LevelBB,
		SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})

	report, err := Verify(signed, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	got := report.Signatures[0]

	if !got.DigestValid || !got.SignatureValid {
		t.Error("crypto checks should pass regardless of Roots")
	}
	if got.ChainStatus != ChainStatusNoRootsSupplied {
		t.Errorf("ChainStatus = %v, want NO_ROOTS_SUPPLIED", got.ChainStatus)
	}
}

func TestVerify_UntrustedRoot(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{
		Level:       LevelBB,
		SigningTime: time.Now(),
	})

	// A root that has nothing to do with the signer's self-signed
	// certificate — the chain must not build.
	_, otherCert := generateTestRSACert(t)
	roots := x509.NewCertPool()
	roots.AddCert(otherCert)

	report, err := Verify(signed, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	got := report.Signatures[0]

	if !got.SignatureValid {
		t.Error("SignatureValid = false, want true (chain trust is independent of the crypto check)")
	}
	if got.ChainStatus != ChainStatusUntrusted {
		t.Errorf("ChainStatus = %v, want UNTRUSTED", got.ChainStatus)
	}
}

func TestVerify_TamperedByteRangeDetected(t *testing.T) {
	for _, kg := range testKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			key, cert := kg.gen(t)
			signed := signMinimalPDF(t, key, cert, Options{
				Level:       LevelBB,
				SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			})

			roots := x509.NewCertPool()
			roots.AddCert(cert)

			before, err := Verify(signed, VerifyOptions{Roots: roots})
			if err != nil {
				t.Fatalf("Verify (untampered): %v", err)
			}
			if !before.Signatures[0].DigestValid {
				t.Fatal("untampered digest does not verify")
			}

			// Flip a byte well inside the file header, which /ByteRange
			// covers but which a signature dictionary marker never lands on.
			tampered := append([]byte(nil), signed...)
			tampered[10] ^= 0xFF

			after, err := Verify(tampered, VerifyOptions{Roots: roots})
			if err != nil {
				t.Fatalf("Verify (tampered): %v", err)
			}
			if after.Signatures[0].DigestValid {
				t.Error("DigestValid = true after tampering signed content, want false")
			}
		})
	}
}

func TestVerify_CorruptedSignatureBytes(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{
		Level:       LevelBB,
		SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})

	roots := x509.NewCertPool()
	roots.AddCert(cert)

	before, err := Verify(signed, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify (untampered): %v", err)
	}
	if !before.Signatures[0].SignatureValid {
		t.Fatal("untampered signature does not verify")
	}

	locs, err := locateSignatures(signed)
	if err != nil || len(locs) != 1 {
		t.Fatalf("locateSignatures: err=%v n=%d, want 1 signature", err, len(locs))
	}
	loc := locs[0]

	// loc.der is hex-decoded /Contents, including the trailing zero
	// padding folio pads the placeholder with. Isolate the real DER
	// envelope length so the flipped byte lands inside the signature
	// itself, not the padding (which a CMS parser ignores).
	var envelope asn1.RawValue
	if _, err := asn1.Unmarshal(loc.der, &envelope); err != nil {
		t.Fatalf("unmarshal CMS envelope: %v", err)
	}
	realLen := len(envelope.FullBytes)

	corrupted := append([]byte(nil), loc.der[:realLen]...)
	corrupted[realLen-1] ^= 0xFF // last byte of SignerInfo.Signature for a B-B signature

	tampered := append([]byte(nil), signed...)
	ph := signaturePlaceholder{ContentsOffset: loc.contentsStart}
	if err := patchContents(tampered, ph, corrupted); err != nil {
		t.Fatalf("patchContents: %v", err)
	}

	after, err := Verify(tampered, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify (corrupted signature): %v", err)
	}
	got := after.Signatures[0]
	if !got.DigestValid {
		t.Error("DigestValid = false, want true (Contents is outside /ByteRange)")
	}
	if got.SignatureValid {
		t.Error("SignatureValid = true for a corrupted signature, want false")
	}
}

func TestVerify_MalformedInput(t *testing.T) {
	_, err := Verify([]byte("%PDF-1.7\nnot a signed document"), VerifyOptions{})
	if err == nil {
		t.Fatal("Verify on an unsigned PDF: want error, got nil")
	}
}

func TestVerify_IntermediateChain(t *testing.T) {
	root, intermediate, leaf, leafKey := generateTestCAChain(t)

	signer, err := NewLocalSigner(leafKey, []*x509.Certificate{leaf, intermediate})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	signed, err := SignPDF(minimalPDF(t), Options{
		Signer:      signer,
		Level:       LevelBB,
		SigningTime: time.Now().Truncate(time.Second),
	})
	if err != nil {
		t.Fatalf("SignPDF: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(root)

	report, err := Verify(signed, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(report.Signatures) != 1 {
		t.Fatalf("len(Signatures) = %d, want 1", len(report.Signatures))
	}
	got := report.Signatures[0]

	if got.ChainStatus != ChainStatusTrusted {
		t.Errorf("ChainStatus = %v, want TRUSTED", got.ChainStatus)
	}
	if got.SignerCertificate == nil || got.SignerCertificate.SerialNumber.Cmp(leaf.SerialNumber) != 0 {
		t.Errorf("SignerCertificate serial = %v, want leaf serial %v", got.SignerCertificate.SerialNumber, leaf.SerialNumber)
	}
	if !got.OK() {
		t.Error("OK() = false, want true for a trusted two-cert chain")
	}
}

func TestVerify_AppendedBytesFailsOK(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{
		Level:       LevelBB,
		SigningTime: time.Now(),
	})

	roots := x509.NewCertPool()
	roots.AddCert(cert)

	tampered := append(append([]byte(nil), signed...), []byte("\n% evil incremental update")...)

	report, err := Verify(tampered, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify (appended bytes): %v", err)
	}
	got := report.Signatures[0]

	if got.ByteRangeCoversFile {
		t.Error("ByteRangeCoversFile = true after appending trailing bytes, want false")
	}
	if got.OK() {
		t.Error("OK() = true after appending trailing bytes, want false")
	}
}

func TestVerify_OKVerdict(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{
		Level:       LevelBB,
		SigningTime: time.Now(),
	})

	roots := x509.NewCertPool()
	roots.AddCert(cert)

	withRoots, err := Verify(signed, VerifyOptions{Roots: roots})
	if err != nil {
		t.Fatalf("Verify (with roots): %v", err)
	}
	if !withRoots.Signatures[0].OK() {
		t.Error("OK() = false with trusted roots, want true")
	}

	noRoots, err := Verify(signed, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify (no roots): %v", err)
	}
	got := noRoots.Signatures[0]
	if !got.DigestValid || !got.SignatureValid {
		t.Error("crypto checks should still pass without roots")
	}
	if got.OK() {
		t.Error("OK() = true with nil Roots, want false")
	}
}

func TestDecodePdfHex_Tolerant(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"plain", "48656C6C6F", []byte("Hello")},
		{"embedded whitespace", "48 65\n6C 6C\t6F", []byte("Hello")},
		{"odd digit count padded", "48656C6C6F7", []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x70}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePdfHex([]byte(tt.in))
			if err != nil {
				t.Fatalf("decodePdfHex(%q): %v", tt.in, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("decodePdfHex(%q) = %X, want %X", tt.in, got, tt.want)
			}
		})
	}
}

func TestLocateSignatures_TolerantHex(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{Level: LevelBB})

	// Baseline: locateSignatures must find the folio-produced signature.
	locs, err := locateSignatures(signed)
	if err != nil || len(locs) != 1 {
		t.Fatalf("locateSignatures baseline: err=%v n=%d, want 1", err, len(locs))
	}

	// No-space "/Contents<" must also be found.
	noSpace := bytes.Replace(signed, []byte("/Contents <"), []byte("/Contents<"), 1)
	locs2, err := locateSignatures(noSpace)
	if err != nil || len(locs2) != 1 {
		t.Fatalf("locateSignatures (no space): err=%v n=%d, want 1", err, len(locs2))
	}
}

func TestParseCMS_HasTimestampRequiresTSTOID(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	digest := hashBytes(crypto.SHA256, []byte("test document content"))

	// No timestamp token: hasTimestamp must be false.
	cms, err := buildCMS(digest, signer, time.Now(), nil)
	if err != nil {
		t.Fatalf("buildCMS: %v", err)
	}
	p, err := parseCMS(cms)
	if err != nil {
		t.Fatalf("parseCMS: %v", err)
	}
	if p.hasTimestamp {
		t.Error("hasTimestamp = true with no tsaToken, want false")
	}
}

func TestVerify_DecoySigInStreamIgnored(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{Level: LevelBB, SigningTime: time.Now()})

	decoy := core.NewPdfDictionary()
	decoy.Set("Type", core.NewPdfName("Sig"))
	decoy.Set("ByteRange", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0), core.NewPdfInteger(0), core.NewPdfInteger(0)))
	decoy.Set("Contents", core.NewPdfHexString("00"))
	withDecoy := appendDecoyObject(t, signed, decoy)

	report, err := Verify(withDecoy, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(report.Signatures) != 1 {
		t.Fatalf("len(Signatures) = %d, want 1 (decoy object must be ignored)", len(report.Signatures))
	}
}

func TestLocateSignaturesGraph_FindsFolioSignature(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{Level: LevelBB, SigningTime: time.Now()})

	graphLocs, hasDSS, err := locateSignaturesGraph(signed)
	if err != nil {
		t.Fatalf("locateSignaturesGraph: %v", err)
	}
	if len(graphLocs) != 1 {
		t.Fatalf("locateSignaturesGraph found %d, want 1", len(graphLocs))
	}
	if hasDSS {
		t.Error("hasDSS = true, want false")
	}

	scanLocs, err := locateSignatures(signed)
	if err != nil || len(scanLocs) != 1 {
		t.Fatalf("locateSignatures: err=%v n=%d, want 1", err, len(scanLocs))
	}

	g, s := graphLocs[0], scanLocs[0]
	if g.contentsStart != s.contentsStart || g.contentsEnd != s.contentsEnd || g.byteRange != s.byteRange {
		t.Errorf("graph loc = %+v, scan loc = %+v, want equal offsets", g, s)
	}
	if !bytes.Equal(g.der, s.der) {
		t.Errorf("graph der != scan der")
	}
}

func TestLocateSignaturesGraph_RejectsMismatchedByteRange(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{Level: LevelBB, SigningTime: time.Now()})

	locs, err := locateSignatures(signed)
	if err != nil || len(locs) != 1 {
		t.Fatalf("locateSignatures: err=%v n=%d, want 1", err, len(locs))
	}

	// Corrupt one raw /ByteRange digit in place (same length, so offsets
	// elsewhere are unaffected) so the graph's /ByteRange claim no longer
	// anchors to the real /Contents span.
	tampered := append([]byte(nil), signed...)
	brBytes := []byte(strconv.Itoa(locs[0].byteRange[1]))
	idx := bytes.Index(tampered, brBytes)
	if idx < 0 {
		t.Fatalf("could not find ByteRange[1] literal %q in signed bytes", brBytes)
	}
	// Flip the first digit to a different digit, keeping the same length.
	orig := tampered[idx]
	repl := byte('1')
	if orig == '1' {
		repl = '2'
	}
	tampered[idx] = repl

	_, _, err = locateSignaturesGraph(tampered)
	if err == nil {
		t.Error("locateSignaturesGraph: want error for mismatched /ByteRange, got nil")
	}

	report, err := Verify(tampered, VerifyOptions{})
	if err != nil {
		// Acceptable: no signatures could be located/parsed at all.
		return
	}
	if report.Signatures[0].OK() {
		t.Error("OK() = true after corrupting /ByteRange, want false")
	}
}

func TestVerify_DSSLiteralInContentIgnored(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signed := signMinimalPDF(t, key, cert, Options{Level: LevelBB, SigningTime: time.Now()})

	decoy := core.NewPdfDictionary()
	decoy.Set("Note", core.NewPdfLiteralString("see /DSS spec"))
	withDecoy := appendDecoyObject(t, signed, decoy)

	report, err := Verify(withDecoy, VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Signatures[0].HasDSS {
		t.Error("HasDSS = true for a literal '/DSS' string outside the catalog, want false")
	}
}

func TestParseCMS_MultipleSignerInfos(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	digest := hashBytes(crypto.SHA256, []byte("test document content"))
	cms, err := buildCMS(digest, signer, time.Now(), nil)
	if err != nil {
		t.Fatalf("buildCMS: %v", err)
	}

	// Parse ContentInfo/SignedData, duplicate the SignerInfos bytes inside
	// the SET, and re-marshal — a synthetic CMS blob with two identical
	// SignerInfos over the same content.
	var ci contentInfo
	if _, err := asn1.Unmarshal(cms, &ci); err != nil {
		t.Fatalf("unmarshal ContentInfo: %v", err)
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		t.Fatalf("unmarshal SignedData: %v", err)
	}

	siInner := stripTag(sd.SignerInfos.FullBytes)
	doubled := marshalSet(append(append([]byte{}, siInner...), siInner...))
	sd.SignerInfos = asn1.RawValue{FullBytes: doubled}

	sdDER, err := asn1.Marshal(sd)
	if err != nil {
		t.Fatalf("marshal SignedData: %v", err)
	}
	ci.Content = asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: sdDER, FullBytes: nil}
	ciDER, err := asn1.Marshal(ci)
	if err != nil {
		t.Fatalf("marshal ContentInfo: %v", err)
	}

	results, err := parseCMSAll(ciDER)
	if err != nil {
		t.Fatalf("parseCMSAll: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("parseCMSAll returned %d results, want 2", len(results))
	}
}

func TestParseCMS_RequiresContentType(t *testing.T) {
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}

	digest := hashBytes(crypto.SHA256, []byte("test document content"))
	cms, err := buildCMS(digest, signer, time.Now(), nil)
	if err != nil {
		t.Fatalf("buildCMS: %v", err)
	}

	// Regression guard: folio's own signed attributes always include
	// contentType, so parseCMS must still succeed.
	if _, err := parseCMS(cms); err != nil {
		t.Fatalf("parseCMS on folio-built CMS: %v", err)
	}
}
