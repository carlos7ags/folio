// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ChainStatus reports the result of validating a signer certificate chain
// against caller-supplied trust anchors.
type ChainStatus int

const (
	// ChainStatusNoRootsSupplied means VerifyOptions.Roots was nil, so no
	// chain was built. The crypto checks (DigestValid, SignatureValid) are
	// unaffected by this.
	ChainStatusNoRootsSupplied ChainStatus = iota

	// ChainStatusTrusted means the signer certificate chains to one of the
	// supplied roots.
	ChainStatusTrusted

	// ChainStatusUntrusted means chain building failed for a reason other
	// than expiry — most commonly, no path to a trusted root.
	ChainStatusUntrusted

	// ChainStatusExpired means the signer certificate was outside its
	// validity period at the validation time.
	ChainStatusExpired
)

// String returns the ETSI-style status name used in Report output.
func (s ChainStatus) String() string {
	switch s {
	case ChainStatusTrusted:
		return "TRUSTED"
	case ChainStatusUntrusted:
		return "UNTRUSTED"
	case ChainStatusExpired:
		return "EXPIRED"
	case ChainStatusNoRootsSupplied:
		return "NO_ROOTS_SUPPLIED"
	default:
		return "UNKNOWN"
	}
}

// VerifyOptions configures Verify.
type VerifyOptions struct {
	// Roots are the caller-supplied trust anchors used to build a chain for
	// each signer certificate. Nil skips chain validation entirely; every
	// SignatureResult.ChainStatus is then ChainStatusNoRootsSupplied.
	Roots *x509.CertPool

	// At is the time used for certificate validity checks. The zero value
	// uses each signature's own SigningTime.
	At time.Time
}

// SignatureResult reports what Verify found and checked for one signature
// in the PDF. A field being false never means "not checked" versus "checked
// and failed" — see the doc comment on each field for what it covers.
type SignatureResult struct {
	// Name, Reason, Location come from the signature dictionary, as set by
	// Options.Name/Reason/Location at signing time.
	Name, Reason, Location string

	// SignerCertificate is the first certificate embedded in the CMS
	// SignedData — the certificate the signature was checked against.
	SignerCertificate *x509.Certificate

	// SigningTime is the CMS signingTime signed attribute.
	SigningTime time.Time

	// DigestValid is true when the CMS messageDigest signed attribute
	// equals the hash of the bytes covered by /ByteRange.
	DigestValid bool

	// SignatureValid is true when the CMS signature cryptographically
	// verifies against SignerCertificate's public key over the DER
	// encoding of the CMS signed attributes.
	SignatureValid bool

	// ByteRangeCoversFile is true when /ByteRange spans the entire file
	// with no gap other than the /Contents hex string itself.
	ByteRangeCoversFile bool

	// ChainStatus is the result of building a chain from
	// SignerCertificate to VerifyOptions.Roots.
	ChainStatus ChainStatus

	// HasTimestamp reports whether an RFC 3161 timestamp token is present
	// as a CMS unsigned attribute. Its presence is reported, but the token
	// itself is not cryptographically validated.
	HasTimestamp bool

	// HasDSS reports whether the document catalog has a /DSS entry. Its
	// contents (OCSP responses, CRLs, certificates) are not validated.
	HasDSS bool
}

// Report is the result of Verify.
type Report struct {
	// Signatures holds one SignatureResult per signature found in the PDF,
	// in the order they appear in the file.
	Signatures []SignatureResult
}

// Verify checks every signature in a signed PDF and reports what it found.
// It performs OFFLINE checks only: it locates each signature dictionary,
// verifies the /ByteRange digest against the CMS messageDigest attribute,
// verifies the CMS signature over the signed attributes, and — when
// VerifyOptions.Roots is supplied — builds a certificate chain.
//
// Verify does not fetch revocation data (OCSP/CRL), does not validate any
// embedded RFC 3161 timestamp token or /DSS contents beyond their presence,
// and does not classify the PAdES conformance level of what it found.
//
// pdfBytes must be the exact bytes produced by SignPDF (or an equivalent
// signer) — Verify computes ByteRange offsets against pdfBytes directly, so
// re-encoding, trimming, or otherwise altering the file before calling
// Verify will misreport or fail the digest check. It returns an error only
// when the input cannot be parsed as a signed PDF at all (no signature
// found, malformed CMS); tamper and trust failures are reported through
// SignatureResult fields, never as an error.
func Verify(pdfBytes []byte, opts VerifyOptions) (*Report, error) {
	locs, err := locateSignatures(pdfBytes)
	if err != nil {
		return nil, err
	}
	if len(locs) == 0 {
		return nil, errors.New("sign: no signatures found in PDF")
	}

	hasDSS := bytes.Contains(pdfBytes, []byte("/DSS"))

	report := &Report{Signatures: make([]SignatureResult, 0, len(locs))}
	for _, loc := range locs {
		res, err := verifyLocation(pdfBytes, loc, opts)
		if err != nil {
			return nil, err
		}
		res.HasDSS = hasDSS
		report.Signatures = append(report.Signatures, res)
	}
	return report, nil
}

// verifyLocation runs all checks for one located signature dictionary.
func verifyLocation(pdf []byte, loc sigLocation, opts VerifyOptions) (SignatureResult, error) {
	res := SignatureResult{
		Name:     loc.name,
		Reason:   loc.reason,
		Location: loc.location,
	}
	res.ByteRangeCoversFile = byteRangeCoversFile(pdf, loc.byteRange, loc.contentsStart, loc.contentsEnd)

	cms, err := parseCMS(loc.der)
	if err != nil {
		return SignatureResult{}, fmt.Errorf("sign: parse CMS: %w", err)
	}
	res.SignerCertificate = cms.cert
	res.SigningTime = cms.signingTime
	res.HasTimestamp = cms.hasTimestamp

	if hashFn, ok := hashFuncForOID(cms.digestAlg); ok && hashFn.Available() {
		if digest, err := hashByteRange(pdf, loc.byteRange, hashFn); err == nil {
			res.DigestValid = bytes.Equal(digest, cms.messageDigest)
		}
	}

	res.SignatureValid = verifySignatureCMS(cms)

	at := opts.At
	if at.IsZero() {
		at = res.SigningTime
	}
	res.ChainStatus = classifyChain(cms.cert, opts.Roots, at)

	return res, nil
}

// classifyChain builds a certificate chain from cert to roots at time at.
func classifyChain(cert *x509.Certificate, roots *x509.CertPool, at time.Time) ChainStatus {
	if roots == nil {
		return ChainStatusNoRootsSupplied
	}
	if at.IsZero() {
		at = time.Now()
	}
	_, err := cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: at,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err == nil {
		return ChainStatusTrusted
	}
	var invalid x509.CertificateInvalidError
	if errors.As(err, &invalid) && invalid.Reason == x509.Expired {
		return ChainStatusExpired
	}
	return ChainStatusUntrusted
}

// --- Signature location ---

// sigLocation is a located, unparsed signature dictionary within a PDF.
type sigLocation struct {
	name, reason, location string
	byteRange              [4]int
	contentsStart          int // absolute offset of the /Contents '<'
	contentsEnd            int // absolute offset just past the /Contents '>'
	der                    []byte
}

// locateSignatures finds every /Type /Sig dictionary in pdf and returns
// their /ByteRange, /Contents, and descriptive fields, in file order.
//
// This scans raw bytes rather than walking the parsed object graph through
// reader.Parse: ByteRange offsets must be measured against exactly the
// bytes passed to Verify, and reader.Parse trims any garbage prefix before
// the %PDF- header (reader/reader.go), which would silently shift those
// offsets. The scan mirrors locatePlaceholders in sign.go, which finds
// these same markers in output folio itself just wrote.
func locateSignatures(pdf []byte) ([]sigLocation, error) {
	const dictMarker = "<< /Type /Sig"
	var locs []sigLocation

	for searchFrom := 0; ; {
		idx := bytes.Index(pdf[searchFrom:], []byte(dictMarker))
		if idx < 0 {
			break
		}
		dictStart := searchFrom + idx

		const brMarker = "/ByteRange "
		brIdx := bytes.Index(pdf[dictStart:], []byte(brMarker))
		if brIdx < 0 {
			return nil, errors.New("sign: signature dictionary missing /ByteRange")
		}
		brStart := dictStart + brIdx + len(brMarker)

		br, brEnd, err := parseByteRangeArray(pdf, brStart)
		if err != nil {
			return nil, err
		}

		const contentsMarker = "/Contents <"
		cIdx := bytes.Index(pdf[brEnd:], []byte(contentsMarker))
		if cIdx < 0 {
			return nil, errors.New("sign: signature dictionary missing /Contents")
		}
		contentsStart := brEnd + cIdx + len(contentsMarker) - 1 // offset of '<'

		relEnd := bytes.IndexByte(pdf[contentsStart+1:], '>')
		if relEnd < 0 {
			return nil, errors.New("sign: /Contents hex string is not terminated")
		}
		hexEnd := contentsStart + 1 + relEnd // offset of '>'
		contentsEnd := hexEnd + 1

		der, err := hex.DecodeString(string(pdf[contentsStart+1 : hexEnd]))
		if err != nil {
			return nil, fmt.Errorf("sign: decode /Contents hex: %w", err)
		}

		dictFields := pdf[dictStart:brStart]
		locs = append(locs, sigLocation{
			name:          extractLiteralString(dictFields, "Name"),
			reason:        extractLiteralString(dictFields, "Reason"),
			location:      extractLiteralString(dictFields, "Location"),
			byteRange:     br,
			contentsStart: contentsStart,
			contentsEnd:   contentsEnd,
			der:           der,
		})

		searchFrom = contentsEnd
	}

	return locs, nil
}

// parseByteRangeArray parses "[a b c d]" starting at pdf[start] and returns
// the four integers plus the offset just past the closing ']'.
func parseByteRangeArray(pdf []byte, start int) ([4]int, int, error) {
	var br [4]int
	if start >= len(pdf) || pdf[start] != '[' {
		return br, 0, errors.New("sign: /ByteRange does not start with '['")
	}
	relEnd := bytes.IndexByte(pdf[start:], ']')
	if relEnd < 0 {
		return br, 0, errors.New("sign: /ByteRange is not terminated")
	}
	end := start + relEnd

	fields := strings.Fields(string(pdf[start+1 : end]))
	if len(fields) != 4 {
		return br, 0, fmt.Errorf("sign: /ByteRange has %d values, want 4", len(fields))
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return br, 0, fmt.Errorf("sign: /ByteRange value %q is not a non-negative integer", f)
		}
		br[i] = n
	}
	return br, end + 1, nil
}

// extractLiteralString reads the PDF literal string value of "/key (...)"
// within region, unescaping \\, \(, and \) — the only escapes
// escapePdfString produces. It returns "" if key is not present.
func extractLiteralString(region []byte, key string) string {
	marker := []byte("/" + key + " (")
	idx := bytes.Index(region, marker)
	if idx < 0 {
		return ""
	}
	pos := idx + len(marker)
	var out []byte
	for pos < len(region) {
		c := region[pos]
		if c == '\\' && pos+1 < len(region) {
			pos++
			out = append(out, region[pos])
			pos++
			continue
		}
		if c == ')' {
			break
		}
		out = append(out, c)
		pos++
	}
	return string(out)
}

// byteRangeCoversFile reports whether br spans the entire file with no gap
// other than the /Contents hex string delimited by [contentsStart,
// contentsEnd). Skipping this check is a classic PDF-signature attack:
// content placed outside the signed ranges is invisible to the digest but
// still rendered by a viewer.
func byteRangeCoversFile(pdf []byte, br [4]int, contentsStart, contentsEnd int) bool {
	return contentsStart >= 0 && contentsEnd <= len(pdf) && contentsStart < contentsEnd &&
		br[0] == 0 &&
		br[0]+br[1] == contentsStart &&
		br[2] == contentsEnd &&
		br[2]+br[3] == len(pdf)
}

// hashByteRange hashes the two segments of pdf that br covers, exactly as
// a compliant signer's ByteRange digest is defined: everything except the
// /Contents hex string.
func hashByteRange(pdf []byte, br [4]int, hashFn crypto.Hash) ([]byte, error) {
	if br[0] < 0 || br[1] < 0 || br[2] < 0 || br[3] < 0 {
		return nil, errors.New("sign: /ByteRange has negative values")
	}
	seg1End := br[0] + br[1]
	seg2End := br[2] + br[3]
	if seg1End > len(pdf) || seg2End > len(pdf) || br[2] < seg1End {
		return nil, errors.New("sign: /ByteRange is out of bounds")
	}
	h := hashFn.New()
	h.Write(pdf[br[0]:seg1End])
	h.Write(pdf[br[2]:seg2End])
	return h.Sum(nil), nil
}

// --- CMS parsing ---

// signedCMS holds the parts of a CMS SignedData structure needed to verify
// it: the signer certificate, the pieces of SignerInfo, and the two signed
// attribute values Verify checks against the PDF.
type signedCMS struct {
	cert           *x509.Certificate
	digestAlg      asn1.ObjectIdentifier
	signature      []byte
	signedAttrsSet []byte // re-tagged SET OF — exactly what buildCMS hashed and signed
	messageDigest  []byte
	signingTime    time.Time
	hasTimestamp   bool
}

// parseCMS parses a detached CMS SignedData blob — as embedded in a PDF
// signature's /Contents — into the fields verifyLocation needs. It mirrors
// the structures buildCMS (cms.go) writes; the signed-attributes handling
// in particular must be its exact inverse: the signature is over the
// SignedAttrs SET OF DER encoding, not over the implicit [0]-tagged form
// SignerInfo carries on the wire.
func parseCMS(der []byte) (*signedCMS, error) {
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		return nil, fmt.Errorf("unmarshal ContentInfo: %w", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		return nil, fmt.Errorf("unsupported ContentType %v", ci.ContentType)
	}

	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return nil, fmt.Errorf("unmarshal SignedData: %w", err)
	}
	if len(sd.Certificates.Bytes) == 0 {
		return nil, errors.New("SignedData has no certificates")
	}
	cert, err := x509.ParseCertificate(sd.Certificates.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse signer certificate: %w", err)
	}

	var si signerInfo
	if _, err := asn1.Unmarshal(stripTag(sd.SignerInfos.FullBytes), &si); err != nil {
		return nil, fmt.Errorf("unmarshal SignerInfo: %w", err)
	}
	if len(si.SignedAttrs.Bytes) == 0 {
		return nil, errors.New("SignerInfo has no signed attributes")
	}

	// SignedAttrs.Bytes is the implicit [0]-tagged content; re-wrap it as a
	// universal SET to reconstruct what was actually hashed and signed.
	signedAttrsSet := marshalSet(si.SignedAttrs.Bytes)

	var messageDigest []byte
	var signingTime time.Time
	rest := si.SignedAttrs.Bytes
	for len(rest) > 0 {
		var attr attribute
		var err error
		rest, err = asn1.Unmarshal(rest, &attr)
		if err != nil {
			return nil, fmt.Errorf("unmarshal signed attribute: %w", err)
		}
		switch {
		case attr.Type.Equal(oidMessageDigest):
			var raw asn1.RawValue
			if _, err := asn1.Unmarshal(attr.Values.Bytes, &raw); err != nil {
				return nil, fmt.Errorf("unmarshal messageDigest: %w", err)
			}
			if raw.Tag != asn1.TagOctetString {
				return nil, errors.New("messageDigest attribute is not an OCTET STRING")
			}
			messageDigest = raw.Bytes
		case attr.Type.Equal(oidSigningTime):
			t, err := parseAttrTime(attr.Values.Bytes)
			if err != nil {
				return nil, fmt.Errorf("unmarshal signingTime: %w", err)
			}
			signingTime = t
		}
	}
	if messageDigest == nil {
		return nil, errors.New("CMS missing messageDigest signed attribute")
	}

	return &signedCMS{
		cert:           cert,
		digestAlg:      si.DigestAlgorithm.Algorithm,
		signature:      si.Signature,
		signedAttrsSet: signedAttrsSet,
		messageDigest:  messageDigest,
		signingTime:    signingTime,
		hasTimestamp:   len(si.UnsignedAttrs.Bytes) > 0,
	}, nil
}

// parseAttrTime decodes a CMS signingTime attribute value, which may be
// encoded as either UTCTime or GeneralizedTime depending on the year.
func parseAttrTime(values []byte) (time.Time, error) {
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(values, &raw); err != nil {
		return time.Time{}, err
	}
	var t time.Time
	var err error
	if raw.Tag == asn1.TagGeneralizedTime {
		_, err = asn1.UnmarshalWithParams(raw.FullBytes, &t, "generalized")
	} else {
		_, err = asn1.Unmarshal(raw.FullBytes, &t)
	}
	return t, err
}

// verifySignatureCMS checks the CMS signature over the signed-attributes
// SET using the embedded certificate's public key.
func verifySignatureCMS(c *signedCMS) bool {
	hashFn, ok := hashFuncForOID(c.digestAlg)
	if !ok || !hashFn.Available() {
		return false
	}
	h := hashFn.New()
	h.Write(c.signedAttrsSet)
	digest := h.Sum(nil)

	switch pub := c.cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pub, hashFn, digest, c.signature) == nil
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(pub, digest, c.signature)
	default:
		return false
	}
}
