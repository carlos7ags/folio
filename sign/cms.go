// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"time"
)

// buildCMS constructs a DER-encoded CMS SignedData structure.
//
// Parameters:
//   - digest: hash of the data being signed (the PDF byte ranges)
//   - signer: provides the signing operation, algorithm, and certificates
//   - signingTime: timestamp for the signed attributes
//   - tsaToken: optional RFC 3161 timestamp token (DER); nil to omit
//
// The result is a detached CMS signature suitable for embedding in a PDF
// signature dictionary's /Contents field.
func buildCMS(digest []byte, signer Signer, signingTime time.Time, tsaToken []byte) ([]byte, error) {
	certs := signer.CertificateChain()
	if len(certs) == 0 {
		return nil, errors.New("sign: no certificates")
	}
	algo := signer.Algorithm()
	signingCert := certs[0]

	// Build signed attributes.
	signedAttrs, err := buildSignedAttributes(digest, signingTime, signingCert, algo)
	if err != nil {
		return nil, err
	}

	// DER-encode signed attributes as SET OF for hashing.
	signedAttrBytes, err := marshalAttributes(signedAttrs)
	if err != nil {
		return nil, err
	}

	// Hash the signed attributes.
	h := algo.HashFunc().New()
	h.Write(signedAttrBytes)
	attrDigest := h.Sum(nil)

	// Sign the attributes digest.
	sig, err := signer.Sign(attrDigest)
	if err != nil {
		return nil, err
	}

	// Encode certificates.
	var certsDER []byte
	for _, cert := range certs {
		certsDER = append(certsDER, cert.Raw...)
	}

	// Build digest algorithm SET.
	digestAlg := algorithmIdentifier{Algorithm: algo.DigestOID()}
	digestAlgDER, err := asn1.Marshal(digestAlg)
	if err != nil {
		return nil, err
	}
	digestAlgSet := marshalSet(digestAlgDER)

	// Build SignerInfo.
	siDER, err := marshalSignerInfo(signingCert, algo, signedAttrBytes, sig, tsaToken)
	if err != nil {
		return nil, err
	}
	siSet := marshalSet(siDER)

	// Build SignedData.
	sd := signedData{
		Version:          1,
		DigestAlgorithms: asn1.RawValue{FullBytes: digestAlgSet},
		EncapContentInfo: encapContentInfo{ContentType: oidData},
		Certificates:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: certsDER},
		SignerInfos:      asn1.RawValue{FullBytes: siSet},
	}
	sdDER, err := asn1.Marshal(sd)
	if err != nil {
		return nil, err
	}

	// Wrap in ContentInfo.
	ci := contentInfo{
		ContentType: oidSignedData,
		Content:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: sdDER},
	}
	return asn1.Marshal(ci)
}

// buildSignedAttributes creates the CMS signed attributes.
func buildSignedAttributes(digest []byte, signingTime time.Time, cert *x509.Certificate, algo Algorithm) ([]attribute, error) {
	// Content type attribute.
	contentTypeVal, err := asn1.Marshal(oidData)
	if err != nil {
		return nil, err
	}

	// Message digest attribute.
	digestVal, err := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal, Tag: asn1.TagOctetString, Bytes: digest,
	})
	if err != nil {
		return nil, err
	}

	// Signing time attribute.
	timeVal, err := asn1.Marshal(signingTime.UTC())
	if err != nil {
		return nil, err
	}

	attrs := []attribute{
		{Type: oidContentType, Values: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: contentTypeVal}},
		{Type: oidMessageDigest, Values: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: digestVal}},
		{Type: oidSigningTime, Values: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: timeVal}},
	}

	// ESS signing-certificate-v2 (required for PAdES B-B).
	certHash := hashBytes(algo.HashFunc(), cert.Raw)
	essCert := signingCertificateV2{
		Certs: []essCertIDv2{{
			HashAlgorithm: algorithmIdentifier{Algorithm: algo.DigestOID()},
			CertHash:      certHash,
		}},
	}
	essDER, err := asn1.Marshal(essCert)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, attribute{
		Type:   oidSigningCertificateV2,
		Values: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: essDER},
	})

	return attrs, nil
}

// marshalAttributes encodes attributes as a SET OF for hashing.
func marshalAttributes(attrs []attribute) ([]byte, error) {
	var attrsDER []byte
	for _, attr := range attrs {
		b, err := asn1.Marshal(attr)
		if err != nil {
			return nil, err
		}
		attrsDER = append(attrsDER, b...)
	}
	return marshalSet(attrsDER), nil
}

// marshalSignerInfo builds and encodes the CMS SignerInfo structure.
func marshalSignerInfo(cert *x509.Certificate, algo Algorithm, signedAttrsSet, sig, tsaToken []byte) ([]byte, error) {
	// Issuer RDNSequence — use the raw issuer bytes from the certificate.
	issuerDER := cert.RawIssuer

	// Serial number.
	serialDER, err := asn1.Marshal(cert.SerialNumber)
	if err != nil {
		return nil, err
	}

	// Strip the SET tag from signedAttrsSet to get inner bytes.
	// The signedAttrsSet starts with SET tag (0x31) + length.
	innerAttrs := stripTag(signedAttrsSet)

	si := signerInfo{
		Version: 1,
		SID: issuerAndSerialNumber{
			Issuer:       asn1.RawValue{FullBytes: issuerDER},
			SerialNumber: asn1.RawValue{FullBytes: serialDER},
		},
		DigestAlgorithm: algorithmIdentifier{Algorithm: algo.DigestOID()},
		SignedAttrs: asn1.RawValue{
			Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true,
			Bytes: innerAttrs,
		},
		SignatureAlgorithm: algorithmIdentifier{Algorithm: algo.SignatureOID()},
		Signature:          sig,
	}

	// Add timestamp as unsigned attribute.
	if len(tsaToken) > 0 {
		tsaAttr := attribute{
			Type:   oidTimeStampToken,
			Values: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: tsaToken},
		}
		tsaDER, err := asn1.Marshal(tsaAttr)
		if err != nil {
			return nil, err
		}
		si.UnsignedAttrs = asn1.RawValue{
			Class: asn1.ClassContextSpecific, Tag: 1, IsCompound: true,
			Bytes: tsaDER,
		}
	}

	return asn1.Marshal(si)
}

// marshalSet wraps DER content bytes in an ASN.1 SET tag.
func marshalSet(content []byte) []byte {
	// SET tag = 0x31
	return marshalTLV(0x31, content)
}

// marshalTLV creates a TLV (tag-length-value) encoding.
func marshalTLV(tag byte, content []byte) []byte {
	l := len(content)
	if l < 128 {
		result := make([]byte, 2+l)
		result[0] = tag
		result[1] = byte(l)
		copy(result[2:], content)
		return result
	}
	// Long form length.
	var lenBytes []byte
	n := l
	for n > 0 {
		lenBytes = append([]byte{byte(n & 0xFF)}, lenBytes...)
		n >>= 8
	}
	result := make([]byte, 2+len(lenBytes)+l)
	result[0] = tag
	result[1] = byte(0x80 | len(lenBytes))
	copy(result[2:], lenBytes)
	copy(result[2+len(lenBytes):], content)
	return result
}

// stripTag strips the outer ASN.1 tag and length, returning the value bytes.
func stripTag(der []byte) []byte {
	if len(der) < 2 {
		return der
	}
	// Skip tag byte.
	pos := 1
	// Parse length.
	if der[pos]&0x80 == 0 {
		// Short form.
		pos++
	} else {
		numBytes := int(der[pos] & 0x7F)
		pos += 1 + numBytes
	}
	if pos > len(der) {
		return der
	}
	return der[pos:]
}

// hashBytes computes the hash of data using the given algorithm.
func hashBytes(h crypto.Hash, data []byte) []byte {
	hasher := h.New()
	hasher.Write(data)
	return hasher.Sum(nil)
}
