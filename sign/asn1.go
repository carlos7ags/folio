// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"encoding/asn1"
)

// OIDs for CMS/PKCS#7 structures.
var (
	oidData       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}

	// Signed attribute OIDs.
	oidContentType          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidSigningCertificateV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}

	// Unsigned attribute OIDs.
	oidTimeStampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}

	// Hash algorithm OIDs.
	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}

	// RSA signature algorithm OIDs.
	oidSHA256WithRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSHA384WithRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSHA512WithRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}

	// ECDSA signature algorithm OIDs.
	oidECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidECDSAWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidECDSAWithSHA512 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}

	// RSA encryption OID (used for issuer key identifier).
	oidRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
)

// Algorithm identifies a hash + signature algorithm pair.
type Algorithm int

const (
	// SHA256WithRSA is RSA PKCS#1 v1.5 with SHA-256.
	SHA256WithRSA Algorithm = iota

	// SHA384WithRSA is RSA PKCS#1 v1.5 with SHA-384.
	SHA384WithRSA

	// SHA512WithRSA is RSA PKCS#1 v1.5 with SHA-512.
	SHA512WithRSA

	// SHA256WithECDSA is ECDSA with SHA-256.
	SHA256WithECDSA

	// SHA384WithECDSA is ECDSA with SHA-384.
	SHA384WithECDSA

	// SHA512WithECDSA is ECDSA with SHA-512.
	SHA512WithECDSA
)

// HashFunc returns the crypto.Hash for this algorithm.
func (a Algorithm) HashFunc() crypto.Hash {
	switch a {
	case SHA256WithRSA, SHA256WithECDSA:
		return crypto.SHA256
	case SHA384WithRSA, SHA384WithECDSA:
		return crypto.SHA384
	case SHA512WithRSA, SHA512WithECDSA:
		return crypto.SHA512
	default:
		return crypto.SHA256
	}
}

// DigestOID returns the ASN.1 OID for the hash algorithm.
func (a Algorithm) DigestOID() asn1.ObjectIdentifier {
	switch a {
	case SHA256WithRSA, SHA256WithECDSA:
		return oidSHA256
	case SHA384WithRSA, SHA384WithECDSA:
		return oidSHA384
	case SHA512WithRSA, SHA512WithECDSA:
		return oidSHA512
	default:
		return oidSHA256
	}
}

// SignatureOID returns the ASN.1 OID for the signature algorithm.
func (a Algorithm) SignatureOID() asn1.ObjectIdentifier {
	switch a {
	case SHA256WithRSA:
		return oidSHA256WithRSA
	case SHA384WithRSA:
		return oidSHA384WithRSA
	case SHA512WithRSA:
		return oidSHA512WithRSA
	case SHA256WithECDSA:
		return oidECDSAWithSHA256
	case SHA384WithECDSA:
		return oidECDSAWithSHA384
	case SHA512WithECDSA:
		return oidECDSAWithSHA512
	default:
		return oidSHA256WithRSA
	}
}

// ASN.1 structure types for CMS/PKCS#7 SignedData (RFC 5652).

// contentInfo is the top-level CMS wrapper.
type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

// signedData is the CMS SignedData content.
type signedData struct {
	Version          int
	DigestAlgorithms asn1.RawValue    // SET OF AlgorithmIdentifier
	EncapContentInfo encapContentInfo // EncapsulatedContentInfo
	Certificates     asn1.RawValue    `asn1:"optional,tag:0"` // implicit SET OF Certificate
	SignerInfos      asn1.RawValue    // SET OF SignerInfo
}

// encapContentInfo identifies the content type being signed.
type encapContentInfo struct {
	ContentType asn1.ObjectIdentifier
	// eContent is omitted for detached signatures.
}

// signerInfo contains per-signer information.
type signerInfo struct {
	Version            int
	SID                issuerAndSerialNumber
	DigestAlgorithm    algorithmIdentifier
	SignedAttrs        asn1.RawValue `asn1:"optional,tag:0"` // implicit SET OF Attribute
	SignatureAlgorithm algorithmIdentifier
	Signature          []byte
	UnsignedAttrs      asn1.RawValue `asn1:"optional,tag:1"` // implicit SET OF Attribute
}

// issuerAndSerialNumber identifies a certificate.
type issuerAndSerialNumber struct {
	Issuer       asn1.RawValue
	SerialNumber asn1.RawValue
}

// algorithmIdentifier is the ASN.1 AlgorithmIdentifier.
type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

// attribute is a CMS Attribute (type + values).
type attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue `asn1:"set"`
}

// essCertIDv2 is the ESS signing-certificate-v2 attribute value (RFC 5035).
type essCertIDv2 struct {
	HashAlgorithm algorithmIdentifier `asn1:"optional"`
	CertHash      []byte
}

// signingCertificateV2 wraps a sequence of ESSCertIDv2.
type signingCertificateV2 struct {
	Certs []essCertIDv2
}
