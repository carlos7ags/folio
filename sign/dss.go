// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"crypto/sha1"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// DSS (Document Security Store) holds validation data for long-term
// signature verification (PAdES B-LT / B-LTA). It is embedded in the
// catalog dictionary per ETSI EN 319 142-1.
//
// Structure:
//
//	/DSS <<
//	  /Certs [stream ...]        % all certificates
//	  /OCSPs [stream ...]        % all OCSP responses
//	  /CRLs  [stream ...]        % all CRLs
//	  /VRI <<                    % per-signature validation data
//	    /AABBCCDD... <<          % key = uppercase hex SHA-1 of sig /Contents
//	      /Cert [stream ...]
//	      /OCSP [stream ...]
//	      /CRL  [stream ...]
//	    >>
//	  >>
//	>>
type DSS struct {
	Certs [][]byte // DER-encoded certificates
	OCSPs [][]byte // DER-encoded OCSP responses
	CRLs  [][]byte // DER-encoded CRLs

	// VRI maps signature Contents SHA-1 hex keys to per-signature validation data.
	VRI map[string]*VRIEntry
}

// VRIEntry holds validation data for a single signature.
type VRIEntry struct {
	Certs [][]byte
	OCSPs [][]byte
	CRLs  [][]byte
}

// NewDSS creates an empty DSS.
func NewDSS() *DSS {
	return &DSS{
		VRI: make(map[string]*VRIEntry),
	}
}

// AddSignatureValidation collects validation data for a signature and adds
// it to the DSS. The sigContents is the raw signature bytes from /Contents.
// The chain is the signer's certificate chain.
// OCSP responses and CRLs are optional (pass nil to skip).
func (d *DSS) AddSignatureValidation(sigContents []byte, chain []*x509.Certificate, ocspResponses, crls [][]byte) {
	// Compute VRI key: uppercase hex SHA-1 of signature contents.
	vriKey := computeVRIKey(sigContents)

	entry := &VRIEntry{}

	// Add certificates.
	for _, cert := range chain {
		d.addCert(cert.Raw)
		entry.Certs = append(entry.Certs, cert.Raw)
	}

	// Add OCSP responses.
	for _, ocsp := range ocspResponses {
		d.addOCSP(ocsp)
		entry.OCSPs = append(entry.OCSPs, ocsp)
	}

	// Add CRLs.
	for _, crl := range crls {
		d.addCRL(crl)
		entry.CRLs = append(entry.CRLs, crl)
	}

	d.VRI[vriKey] = entry
}

// Build creates the DSS dictionary and all referenced stream objects.
// The addObject callback assigns object numbers to each stream and returns
// indirect references for embedding in the dictionary.
func (d *DSS) Build(addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	dss := core.NewPdfDictionary()

	// Global arrays.
	if len(d.Certs) > 0 {
		dss.Set("Certs", d.buildStreamArray(d.Certs, addObject))
	}
	if len(d.OCSPs) > 0 {
		dss.Set("OCSPs", d.buildStreamArray(d.OCSPs, addObject))
	}
	if len(d.CRLs) > 0 {
		dss.Set("CRLs", d.buildStreamArray(d.CRLs, addObject))
	}

	// VRI dictionary.
	if len(d.VRI) > 0 {
		vri := core.NewPdfDictionary()
		for key, entry := range d.VRI {
			vriEntry := core.NewPdfDictionary()
			if len(entry.Certs) > 0 {
				vriEntry.Set("Cert", d.buildStreamArray(entry.Certs, addObject))
			}
			if len(entry.OCSPs) > 0 {
				vriEntry.Set("OCSP", d.buildStreamArray(entry.OCSPs, addObject))
			}
			if len(entry.CRLs) > 0 {
				vriEntry.Set("CRL", d.buildStreamArray(entry.CRLs, addObject))
			}
			vriRef := addObject(vriEntry)
			vri.Set(key, vriRef)
		}
		vriRef := addObject(vri)
		dss.Set("VRI", vriRef)
	}

	return dss
}

// buildStreamArray creates an array of indirect references to stream objects.
func (d *DSS) buildStreamArray(items [][]byte, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfArray {
	refs := make([]core.PdfObject, 0, len(items))
	for _, data := range items {
		stream := core.NewPdfStreamCompressed(data)
		ref := addObject(stream)
		refs = append(refs, ref)
	}
	return core.NewPdfArray(refs...)
}

// addCert appends a DER-encoded certificate if not already present.
func (d *DSS) addCert(der []byte) {
	if !d.containsBytes(d.Certs, der) {
		d.Certs = append(d.Certs, der)
	}
}

// addOCSP appends a DER-encoded OCSP response if not already present.
func (d *DSS) addOCSP(der []byte) {
	if !d.containsBytes(d.OCSPs, der) {
		d.OCSPs = append(d.OCSPs, der)
	}
}

// addCRL appends a DER-encoded CRL if not already present.
func (d *DSS) addCRL(der []byte) {
	if !d.containsBytes(d.CRLs, der) {
		d.CRLs = append(d.CRLs, der)
	}
}

// containsBytes reports whether slice already contains an entry with the same SHA-256 hash as item.
func (d *DSS) containsBytes(slice [][]byte, item []byte) bool {
	itemHash := hashBytes(crypto.SHA256, item)
	for _, existing := range slice {
		if string(hashBytes(crypto.SHA256, existing)) == string(itemHash) {
			return true
		}
	}
	return false
}

// computeVRIKey returns the uppercase hex SHA-1 of the signature contents.
func computeVRIKey(sigContents []byte) string {
	h := sha1.Sum(sigContents)
	return strings.ToUpper(fmt.Sprintf("%x", h))
}

// CollectValidationData gathers certificates, OCSP responses, and CRLs
// for the given signer's certificate chain. This is a convenience that
// uses OCSPClient to fetch revocation data.
func CollectValidationData(chain []*x509.Certificate, ocspClient *OCSPClient) (ocspResponses [][]byte, err error) {
	if ocspClient == nil {
		return nil, nil
	}
	return ocspClient.FetchChainResponses(chain)
}
