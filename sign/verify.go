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
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
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

	// SignerCertificate is the certificate embedded in the CMS SignedData
	// that matches the SignerInfo's SignerIdentifier (issuer + serial
	// number) — the certificate the signature was checked against.
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

	// HasDSS reports whether the document catalog has a /DSS entry —
	// catalog-derived (via the parsed object graph) when the file parses,
	// falling back to a raw "/DSS" byte scan otherwise. Its contents (OCSP
	// responses, CRLs, certificates) are not validated.
	HasDSS bool
}

// OK reports the aggregate verdict for this signature: the ByteRange digest
// matched, the CMS signature verified, /ByteRange covers the entire file
// (no unsigned trailing bytes — the classic incremental-update tamper), and
// the signer chains to a caller-supplied trust root. It is false when
// VerifyOptions.Roots was nil: an aggregate verdict without trust anchors
// would be meaningless. Callers that intentionally skip chain validation
// should check the individual fields instead.
func (r SignatureResult) OK() bool {
	return r.DigestValid && r.SignatureValid && r.ByteRangeCoversFile &&
		r.ChainStatus == ChainStatusTrusted
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
// Callers should generally check SignatureResult.OK(), the single
// aggregate verdict, rather than inspecting individual fields.
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
	locs, hasDSS, err := locateSignaturesGraph(pdfBytes)
	if err != nil || len(locs) == 0 {
		// Fallback: raw byte scan (pre-existing behavior) for files the
		// reader cannot parse or serializations the graph walk missed.
		var scanErr error
		locs, scanErr = locateSignatures(pdfBytes)
		if scanErr != nil {
			return nil, scanErr
		}
		hasDSS = bytes.Contains(pdfBytes, []byte("/DSS"))
	}
	if len(locs) == 0 {
		return nil, errors.New("sign: no signatures found in PDF")
	}

	report := &Report{}
	for _, loc := range locs {
		results, err := verifyLocation(pdfBytes, loc, opts)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].HasDSS = hasDSS
		}
		report.Signatures = append(report.Signatures, results...)
	}
	return report, nil
}

// verifyLocation runs all checks for one located signature dictionary,
// producing one SignatureResult per SignerInfo embedded in its CMS blob
// (ordinarily one; more than one only if the signer chose to co-sign).
func verifyLocation(pdf []byte, loc sigLocation, opts VerifyOptions) ([]SignatureResult, error) {
	byteRangeOK := byteRangeCoversFile(pdf, loc.byteRange, loc.contentsStart, loc.contentsEnd)

	all, err := parseCMSAll(loc.der)
	if err != nil {
		return nil, fmt.Errorf("sign: parse CMS: %w", err)
	}

	results := make([]SignatureResult, 0, len(all))
	for _, cms := range all {
		res := SignatureResult{
			Name:                loc.name,
			Reason:              loc.reason,
			Location:            loc.location,
			ByteRangeCoversFile: byteRangeOK,
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
		res.ChainStatus = classifyChain(cms.cert, cms.otherCerts, opts.Roots, at)

		results = append(results, res)
	}

	return results, nil
}

// classifyChain builds a certificate chain from cert to roots at time at,
// using intermediates (any other certificates embedded in the CMS) to fill
// gaps between the leaf and a trusted root.
func classifyChain(cert *x509.Certificate, intermediates []*x509.Certificate, roots *x509.CertPool, at time.Time) ChainStatus {
	if roots == nil {
		return ChainStatusNoRootsSupplied
	}
	if at.IsZero() {
		at = time.Now()
	}
	var pool *x509.CertPool
	if len(intermediates) > 0 {
		pool = x509.NewCertPool()
		for _, c := range intermediates {
			pool.AddCert(c)
		}
	}
	_, err := cert.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: pool,
		CurrentTime:   at,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
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

// locateSignaturesGraph finds signatures by walking the parsed object
// graph (via the reader package) instead of scanning raw bytes: it treats
// /AcroForm/Fields as the authority for *which* dictionaries are
// signatures, and the catalog's /DSS entry as the authority for HasDSS —
// removing the false positives raw byte scanning is prone to (decoy
// "<< /Type /Sig ...>>" bytes inside a content stream or string, or a
// literal "/DSS" anywhere in the file). It returns the located signatures
// and whether the catalog has a /DSS entry.
//
// The raw ByteRange/Contents offsets Verify needs are still measured
// against pdf directly and cross-checked against what the graph reports:
// the object graph supplies *what* a signature claims, the raw bytes
// confirm *where* it actually is. If pdf does not start with "%PDF-",
// reader.Parse would have trimmed a garbage prefix and shifted every
// offset, so this function refuses to run (an error tells the caller to
// fall back to locateSignatures, which works on exact bytes).
func locateSignaturesGraph(pdf []byte) ([]sigLocation, bool, error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, false, errors.New("sign: pdf does not start with %PDF- (reader would trim a prefix and shift offsets)")
	}

	r, err := reader.Parse(pdf)
	if err != nil {
		return nil, false, fmt.Errorf("sign: parse PDF for signature location: %w", err)
	}

	catalog := r.Catalog()
	if catalog == nil {
		return nil, false, errors.New("sign: PDF has no catalog")
	}

	hasDSS := false
	if dssEntry := catalog.Get("DSS"); dssEntry != nil {
		resolved, err := r.ResolveObject(dssEntry)
		if err == nil {
			if _, ok := resolved.(*core.PdfDictionary); ok {
				hasDSS = true
			}
		}
	}

	sigDicts, err := collectSignatureDicts(r, catalog)
	if err != nil {
		return nil, false, err
	}

	seen := make(map[int]bool)
	var locs []sigLocation
	for _, sd := range sigDicts {
		loc, err := sigLocationFromDict(pdf, sd)
		if err != nil {
			return nil, false, err
		}
		if seen[loc.contentsStart] {
			continue // two fields can share one /V
		}
		seen[loc.contentsStart] = true
		locs = append(locs, loc)
	}

	return locs, hasDSS, nil
}

// collectSignatureDicts walks /AcroForm/Fields (recursing into /Kids) and
// returns the resolved /V dictionary of every terminal signature field.
func collectSignatureDicts(r *reader.PdfReader, catalog *core.PdfDictionary) ([]*core.PdfDictionary, error) {
	afEntry := catalog.Get("AcroForm")
	if afEntry == nil {
		return nil, nil
	}
	afObj, err := r.ResolveObject(afEntry)
	if err != nil {
		return nil, fmt.Errorf("sign: resolve AcroForm: %w", err)
	}
	afDict, ok := afObj.(*core.PdfDictionary)
	if !ok {
		return nil, nil
	}
	fieldsEntry := afDict.Get("Fields")
	if fieldsEntry == nil {
		return nil, nil
	}
	fieldsObj, err := r.ResolveObject(fieldsEntry)
	if err != nil {
		return nil, fmt.Errorf("sign: resolve AcroForm Fields: %w", err)
	}
	fieldsArr, ok := fieldsObj.(*core.PdfArray)
	if !ok {
		return nil, nil
	}

	var dicts []*core.PdfDictionary
	for _, fieldRef := range fieldsArr.All() {
		if err := walkField(r, fieldRef, &dicts); err != nil {
			return nil, err
		}
	}
	return dicts, nil
}

// walkField resolves one AcroForm field (or widget), recursing into /Kids,
// and appends the signature dictionary of any terminal /FT /Sig field
// (or any field whose /V resolves to a /Type /Sig dictionary, to tolerate
// merged widgets that omit /FT) to dicts.
func walkField(r *reader.PdfReader, fieldRef core.PdfObject, dicts *[]*core.PdfDictionary) error {
	fieldObj, err := r.ResolveObject(fieldRef)
	if err != nil {
		return fmt.Errorf("sign: resolve field: %w", err)
	}
	fieldDict, ok := fieldObj.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	if kidsEntry := fieldDict.Get("Kids"); kidsEntry != nil {
		kidsObj, err := r.ResolveObject(kidsEntry)
		if err == nil {
			if kidsArr, ok := kidsObj.(*core.PdfArray); ok {
				for _, kid := range kidsArr.All() {
					if err := walkField(r, kid, dicts); err != nil {
						return err
					}
				}
			}
		}
	}

	vEntry := fieldDict.Get("V")
	if vEntry == nil {
		return nil
	}
	vObj, err := r.ResolveObject(vEntry)
	if err != nil {
		return nil //nolint:nilerr // a dangling /V is not a location error
	}
	vDict, ok := vObj.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	ftName, isFT := fieldDict.Get("FT").(*core.PdfName)
	isSigField := isFT && ftName.Value == "Sig"
	typeName, hasType := vDict.Get("Type").(*core.PdfName)
	isSigDict := hasType && typeName.Value == "Sig"
	if isSigField || isSigDict {
		*dicts = append(*dicts, vDict)
	}
	return nil
}

// sigLocationFromDict builds a sigLocation from a graph-resolved signature
// dictionary, anchoring its /ByteRange and /Contents to the raw bytes of
// pdf: the graph tells us what the signature claims, this confirms where
// it actually is. Any mismatch — an attacker-controlled /ByteRange
// pointing at unrelated bytes — is an error, not a silently accepted
// location.
func sigLocationFromDict(pdf []byte, sd *core.PdfDictionary) (sigLocation, error) {
	brArr, ok := sd.Get("ByteRange").(*core.PdfArray)
	if !ok || brArr.Len() != 4 {
		return sigLocation{}, errors.New("sign: signature dictionary /ByteRange is missing or malformed")
	}
	var br [4]int
	for i := 0; i < 4; i++ {
		n, ok := brArr.At(i).(*core.PdfNumber)
		if !ok {
			return sigLocation{}, errors.New("sign: /ByteRange element is not a number")
		}
		v := n.IntValue()
		if v < 0 {
			return sigLocation{}, errors.New("sign: /ByteRange has a negative value")
		}
		br[i] = v
	}

	contentsStr, ok := sd.Get("Contents").(*core.PdfString)
	if !ok {
		return sigLocation{}, errors.New("sign: signature dictionary /Contents is missing or not a string")
	}
	der := []byte(contentsStr.Text())

	contentsStart := br[0] + br[1]
	contentsEnd := br[2]
	if contentsStart < 0 || contentsEnd <= contentsStart || contentsEnd > len(pdf) {
		return sigLocation{}, errors.New("sign: /ByteRange does not describe a valid span of the file")
	}
	if pdf[contentsStart] != '<' || pdf[contentsEnd-1] != '>' {
		return sigLocation{}, errors.New("sign: /ByteRange does not point at a /Contents hex string")
	}
	rawDER, err := decodePdfHex(pdf[contentsStart+1 : contentsEnd-1])
	if err != nil {
		return sigLocation{}, fmt.Errorf("sign: decode /Contents hex at claimed offset: %w", err)
	}
	if !bytes.Equal(rawDER, der) {
		return sigLocation{}, errors.New("sign: /ByteRange-anchored /Contents does not match the signature dictionary's /Contents")
	}

	return sigLocation{
		name:          pdfStringField(sd, "Name"),
		reason:        pdfStringField(sd, "Reason"),
		location:      pdfStringField(sd, "Location"),
		byteRange:     br,
		contentsStart: contentsStart,
		contentsEnd:   contentsEnd,
		der:           der,
	}, nil
}

// pdfStringField reads a string-valued dictionary entry, returning "" if
// absent or not a string.
func pdfStringField(d *core.PdfDictionary, key string) string {
	if s, ok := d.Get(key).(*core.PdfString); ok {
		return s.Text()
	}
	return ""
}

// locateSignatures finds every /Type /Sig dictionary in pdf and returns
// their /ByteRange, /Contents, and descriptive fields, in file order. It
// is the compatibility fallback for files locateSignaturesGraph cannot
// authoritatively locate signatures in (unparseable PDFs, or
// serializations the AcroForm walk misses) — locateSignaturesGraph is
// preferred whenever it succeeds and finds at least one signature.
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

		contentsStart, err := findContentsHexStart(pdf, brEnd)
		if err != nil {
			return nil, err
		}

		relEnd := bytes.IndexByte(pdf[contentsStart+1:], '>')
		if relEnd < 0 {
			return nil, errors.New("sign: /Contents hex string is not terminated")
		}
		hexEnd := contentsStart + 1 + relEnd // offset of '>'
		contentsEnd := hexEnd + 1

		der, err := decodePdfHex(pdf[contentsStart+1 : hexEnd])
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

// isPdfWhitespace reports whether b is PDF whitespace per ISO 32000-1
// §7.2.2 (used both around /Contents<...> and inside the hex string).
func isPdfWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', 0:
		return true
	default:
		return false
	}
}

// findContentsHexStart finds "/Contents" at or after from, followed by
// optional PDF whitespace then '<', and returns the offset of '<'. A
// conforming writer may emit "/Contents<" with no space or with several
// whitespace bytes; "/Contents" appearing elsewhere (e.g. inside a string)
// is skipped by resuming the search just past the match.
func findContentsHexStart(pdf []byte, from int) (int, error) {
	const marker = "/Contents"
	for searchFrom := from; ; {
		idx := bytes.Index(pdf[searchFrom:], []byte(marker))
		if idx < 0 {
			return 0, errors.New("sign: signature dictionary missing /Contents")
		}
		pos := searchFrom + idx + len(marker)
		for pos < len(pdf) && isPdfWhitespace(pdf[pos]) {
			pos++
		}
		if pos < len(pdf) && pdf[pos] == '<' {
			return pos, nil
		}
		searchFrom = searchFrom + idx + len(marker)
	}
}

// decodePdfHex decodes a PDF hex string's content (the bytes between < and
// >), tolerating embedded whitespace and an odd digit count — ISO 32000-1
// §7.3.4.3 permits both; an odd count is padded with a trailing '0'.
func decodePdfHex(span []byte) ([]byte, error) {
	filtered := make([]byte, 0, len(span))
	for _, b := range span {
		if isPdfWhitespace(b) {
			continue
		}
		filtered = append(filtered, b)
	}
	if len(filtered)%2 != 0 {
		filtered = append(filtered, '0')
	}
	return hex.DecodeString(string(filtered))
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
	otherCerts     []*x509.Certificate // all embedded certs except cert (potential intermediates)
	digestAlg      asn1.ObjectIdentifier
	signature      []byte
	signedAttrsSet []byte // re-tagged SET OF — exactly what buildCMS hashed and signed
	messageDigest  []byte
	signingTime    time.Time
	hasTimestamp   bool
}

// parseAllCertificates parses the concatenated certificate DER blobs from
// SignedData.certificates ([0] IMPLICIT CertificateSet).
func parseAllCertificates(der []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for len(der) > 0 {
		var raw asn1.RawValue
		rest, err := asn1.Unmarshal(der, &raw)
		if err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(raw.FullBytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		der = rest
	}
	return certs, nil
}

// parseCMS parses a detached CMS SignedData blob — as embedded in a PDF
// signature's /Contents — into the fields verifyLocation needs for its
// first SignerInfo. It mirrors the structures buildCMS (cms.go) writes; the
// signed-attributes handling in particular must be its exact inverse: the
// signature is over the SignedAttrs SET OF DER encoding, not over the
// implicit [0]-tagged form SignerInfo carries on the wire.
func parseCMS(der []byte) (*signedCMS, error) {
	all, err := parseCMSAll(der)
	if err != nil {
		return nil, err
	}
	return all[0], nil
}

// parseCMSAll parses a detached CMS SignedData blob into one *signedCMS per
// SignerInfo (SignerInfos is a SET OF, and a CMS blob may embed more than
// one signature over the same content).
func parseCMSAll(der []byte) ([]*signedCMS, error) {
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
	allCerts, err := parseAllCertificates(sd.Certificates.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificates: %w", err)
	}

	var results []*signedCMS
	rest := stripTag(sd.SignerInfos.FullBytes)
	for len(rest) > 0 {
		var si signerInfo
		var err error
		rest, err = asn1.Unmarshal(rest, &si)
		if err != nil {
			return nil, fmt.Errorf("unmarshal SignerInfo: %w", err)
		}
		cms, err := parseSignerInfo(si, allCerts)
		if err != nil {
			return nil, err
		}
		results = append(results, cms)
	}
	if len(results) == 0 {
		return nil, errors.New("SignedData has no SignerInfos")
	}
	return results, nil
}

// parseSignerInfo builds a signedCMS from one SignerInfo, selecting its
// signer certificate from allCerts by SID.
func parseSignerInfo(si signerInfo, allCerts []*x509.Certificate) (*signedCMS, error) {
	if len(si.SignedAttrs.Bytes) == 0 {
		return nil, errors.New("SignerInfo has no signed attributes")
	}

	// Select the signer certificate by SID (issuer + serial number); the
	// remaining certificates are potential intermediates for chain building.
	var serial *big.Int
	if _, err := asn1.Unmarshal(si.SID.SerialNumber.FullBytes, &serial); err != nil {
		return nil, fmt.Errorf("unmarshal SignerInfo serial number: %w", err)
	}
	var cert *x509.Certificate
	var otherCerts []*x509.Certificate
	for _, c := range allCerts {
		if cert == nil && bytes.Equal(c.RawIssuer, si.SID.Issuer.FullBytes) && c.SerialNumber.Cmp(serial) == 0 {
			cert = c
			continue
		}
		otherCerts = append(otherCerts, c)
	}
	if cert == nil {
		return nil, errors.New("no certificate matches SignerInfo SID")
	}

	// SignedAttrs.Bytes is the implicit [0]-tagged content; re-wrap it as a
	// universal SET to reconstruct what was actually hashed and signed.
	signedAttrsSet := marshalSet(si.SignedAttrs.Bytes)

	var messageDigest []byte
	var signingTime time.Time
	var hasContentType bool
	var contentType asn1.ObjectIdentifier
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
		case attr.Type.Equal(oidContentType):
			if _, err := asn1.Unmarshal(attr.Values.Bytes, &contentType); err != nil {
				return nil, fmt.Errorf("unmarshal contentType: %w", err)
			}
			hasContentType = true
		}
	}
	if messageDigest == nil {
		return nil, errors.New("CMS missing messageDigest signed attribute")
	}
	if !hasContentType || !contentType.Equal(oidData) {
		return nil, errors.New("CMS missing or wrong contentType signed attribute")
	}

	// hasTimestamp scans UnsignedAttrs for an RFC 3161 timestamp token
	// attribute specifically; unsigned attrs are unauthenticated, so a
	// malformed one just stops the scan rather than failing the parse.
	var hasTimestamp bool
	urest := si.UnsignedAttrs.Bytes
	for len(urest) > 0 {
		var attr attribute
		var err error
		urest, err = asn1.Unmarshal(urest, &attr)
		if err != nil {
			break
		}
		if attr.Type.Equal(oidTimeStampToken) {
			hasTimestamp = true
			break
		}
	}

	return &signedCMS{
		cert:           cert,
		otherCerts:     otherCerts,
		digestAlg:      si.DigestAlgorithm.Algorithm,
		signature:      si.Signature,
		signedAttrsSet: signedAttrsSet,
		messageDigest:  messageDigest,
		signingTime:    signingTime,
		hasTimestamp:   hasTimestamp,
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
