// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"testing"
	"time"
)

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
