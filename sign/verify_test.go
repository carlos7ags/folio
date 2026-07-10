// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// parsedCMS holds the pieces of a CMS SignedData needed for verification.
type parsedCMS struct {
	cert           *x509.Certificate // first certificate in SignedData
	signerInfo     signerInfo
	signedAttrsSet []byte // SET OF encoding, i.e. what was hashed and signed
	messageDigest  []byte // value of the messageDigest signed attribute
}

// parseCMS parses a CMS SignedData blob back into the parts needed to check
// it cryptographically, reusing the ASN.1 structs buildCMS uses to build it.
func parseCMS(t *testing.T, der []byte) parsedCMS {
	t.Helper()

	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		t.Fatalf("unmarshal ContentInfo: %v", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		t.Fatalf("ContentType = %v, want %v", ci.ContentType, oidSignedData)
	}

	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		t.Fatalf("unmarshal SignedData: %v", err)
	}

	// Certificates.Bytes is the concatenated certificate DER; parsing
	// consumes the first certificate.
	cert, err := x509.ParseCertificate(sd.Certificates.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	var si signerInfo
	if _, err := asn1.Unmarshal(stripTag(sd.SignerInfos.FullBytes), &si); err != nil {
		t.Fatalf("unmarshal SignerInfo: %v", err)
	}

	// SignedAttrs.Bytes is the implicit-tagged content; re-wrap it as a
	// universal SET to reconstruct what was actually hashed and signed.
	signedAttrsSet := marshalSet(si.SignedAttrs.Bytes)

	var messageDigest []byte
	rest := si.SignedAttrs.Bytes
	for len(rest) > 0 {
		var attr attribute
		rest, err = asn1.Unmarshal(rest, &attr)
		if err != nil {
			t.Fatalf("unmarshal signed attribute: %v", err)
		}
		if !attr.Type.Equal(oidMessageDigest) {
			continue
		}
		var raw asn1.RawValue
		if _, err := asn1.Unmarshal(attr.Values.Bytes, &raw); err != nil {
			t.Fatalf("unmarshal messageDigest value: %v", err)
		}
		if raw.Tag != asn1.TagOctetString {
			t.Fatalf("messageDigest tag = %d, want OCTET STRING", raw.Tag)
		}
		messageDigest = raw.Bytes
	}
	if messageDigest == nil {
		t.Fatal("messageDigest attribute not found")
	}

	return parsedCMS{
		cert:           cert,
		signerInfo:     si,
		signedAttrsSet: signedAttrsSet,
		messageDigest:  messageDigest,
	}
}

// verifySignature checks the CMS signature over the signed-attributes SET
// using the certificate's public key. NewLocalSigner always selects SHA-256.
func verifySignature(t *testing.T, p parsedCMS) error {
	t.Helper()

	h := sha256.Sum256(p.signedAttrsSet)
	switch pub := p.cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], p.signerInfo.Signature)
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, h[:], p.signerInfo.Signature) {
			return errors.New("ecdsa signature verification failed")
		}
		return nil
	default:
		return fmt.Errorf("unsupported public key type %T", pub)
	}
}

// extractByteRange reads the four fixed-width /ByteRange integers that
// patchByteRange writes into a signed PDF.
func extractByteRange(t *testing.T, signed []byte) [4]int {
	t.Helper()

	marker := []byte("/ByteRange [")
	idx := bytes.Index(signed, marker)
	if idx < 0 {
		t.Fatal("signed PDF missing /ByteRange")
	}

	pos := idx + len(marker)
	var br [4]int
	for i := range br {
		n, err := strconv.Atoi(string(signed[pos : pos+byteRangeWidth]))
		if err != nil {
			t.Fatalf("parse ByteRange[%d]: %v", i, err)
		}
		br[i] = n
		pos += byteRangeWidth + 1 // skip the separating space or trailing ]
	}
	return br
}

// extractContents hex-decodes the /Contents value (CMS DER plus zero
// padding) from a signed PDF.
func extractContents(t *testing.T, signed []byte) []byte {
	t.Helper()

	marker := []byte("/Contents <")
	idx := bytes.Index(signed, marker)
	if idx < 0 {
		t.Fatal("signed PDF missing /Contents")
	}

	start := idx + len(marker)
	raw, err := hex.DecodeString(string(signed[start : start+contentsPlaceholderLen]))
	if err != nil {
		t.Fatalf("decode /Contents hex: %v", err)
	}
	return raw
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

			p := parseCMS(t, cms)

			if !bytes.Equal(p.messageDigest, digest) {
				t.Errorf("messageDigest = %X, want %X", p.messageDigest, digest)
			}
			if err := verifySignature(t, p); err != nil {
				t.Errorf("verifySignature: %v", err)
			}
			if p.cert.SerialNumber.Cmp(cert.SerialNumber) != 0 {
				t.Errorf("cert serial = %v, want %v", p.cert.SerialNumber, cert.SerialNumber)
			}
		})
	}
}

func TestSignPDF_RoundTripVerifies(t *testing.T) {
	for _, kg := range testKeyGens {
		t.Run(kg.name, func(t *testing.T) {
			key, cert := kg.gen(t)
			signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
			if err != nil {
				t.Fatalf("NewLocalSigner: %v", err)
			}

			signed, err := SignPDF(minimalPDF(t), Options{
				Signer:      signer,
				Level:       LevelBB,
				SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("SignPDF: %v", err)
			}

			br := extractByteRange(t, signed)
			if br[0] != 0 {
				t.Errorf("ByteRange[0] = %d, want 0", br[0])
			}
			if br[1] >= br[2] {
				t.Errorf("ByteRange start (%d) >= end (%d)", br[1], br[2])
			}
			if br[2]+br[3] != len(signed) {
				t.Errorf("ByteRange[2]+[3] = %d, want %d (len(signed))", br[2]+br[3], len(signed))
			}
			if got, want := br[2]-br[1], contentsPlaceholderLen+2; got != want {
				t.Errorf("Contents span = %d, want %d", got, want)
			}

			h := sha256.New()
			h.Write(signed[br[0] : br[0]+br[1]])
			h.Write(signed[br[2] : br[2]+br[3]])
			digest := h.Sum(nil)

			p := parseCMS(t, extractContents(t, signed))
			if !bytes.Equal(digest, p.messageDigest) {
				t.Errorf("recomputed digest = %X, want %X", digest, p.messageDigest)
			}
			if err := verifySignature(t, p); err != nil {
				t.Errorf("verifySignature: %v", err)
			}
		})
	}
}

func TestSignPDF_TamperDetected(t *testing.T) {
	freshSigned := func(t *testing.T) []byte {
		t.Helper()
		key, cert := generateTestRSACert(t)
		signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
		if err != nil {
			t.Fatalf("NewLocalSigner: %v", err)
		}
		signed, err := SignPDF(minimalPDF(t), Options{
			Signer:      signer,
			Level:       LevelBB,
			SigningTime: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("SignPDF: %v", err)
		}
		return signed
	}

	// recomputeDigest re-derives the ByteRange digest exactly as a verifier would.
	recomputeDigest := func(signed []byte, br [4]int) []byte {
		h := sha256.New()
		h.Write(signed[br[0] : br[0]+br[1]])
		h.Write(signed[br[2] : br[2]+br[3]])
		return h.Sum(nil)
	}

	t.Run("flip byte in first ByteRange segment", func(t *testing.T) {
		signed := freshSigned(t)
		br := extractByteRange(t, signed)
		p := parseCMS(t, extractContents(t, signed))

		if !bytes.Equal(recomputeDigest(signed, br), p.messageDigest) {
			t.Fatal("untampered digest does not match messageDigest")
		}

		signed[5] ^= 0xFF
		if bytes.Equal(recomputeDigest(signed, br), p.messageDigest) {
			t.Error("tampered digest still matches messageDigest")
		}
	})

	t.Run("flip byte in second ByteRange segment", func(t *testing.T) {
		signed := freshSigned(t)
		br := extractByteRange(t, signed)
		p := parseCMS(t, extractContents(t, signed))

		if !bytes.Equal(recomputeDigest(signed, br), p.messageDigest) {
			t.Fatal("untampered digest does not match messageDigest")
		}

		signed[len(signed)-3] ^= 0xFF
		if bytes.Equal(recomputeDigest(signed, br), p.messageDigest) {
			t.Error("tampered digest still matches messageDigest")
		}
	})

	t.Run("corrupt signature bytes", func(t *testing.T) {
		signed := freshSigned(t)
		p := parseCMS(t, extractContents(t, signed))

		if err := verifySignature(t, p); err != nil {
			t.Fatalf("untampered signature does not verify: %v", err)
		}

		p.signerInfo.Signature[0] ^= 0xFF
		if err := verifySignature(t, p); err == nil {
			t.Error("tampered signature still verifies")
		}
	})

	t.Run("corrupt signed attributes", func(t *testing.T) {
		signed := freshSigned(t)
		p := parseCMS(t, extractContents(t, signed))

		if err := verifySignature(t, p); err != nil {
			t.Fatalf("untampered signature does not verify: %v", err)
		}

		p.signedAttrsSet[2] ^= 0xFF
		if err := verifySignature(t, p); err == nil {
			t.Error("tampered signed attributes still verify")
		}
	})

	t.Run("wrong certificate", func(t *testing.T) {
		signed := freshSigned(t)
		p := parseCMS(t, extractContents(t, signed))

		if err := verifySignature(t, p); err != nil {
			t.Fatalf("untampered signature does not verify: %v", err)
		}

		_, otherCert := generateTestRSACert(t)
		p.cert = otherCert
		if err := verifySignature(t, p); err == nil {
			t.Error("signature verifies against the wrong certificate")
		}
	})
}
