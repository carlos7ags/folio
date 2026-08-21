// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
)

// Signer performs the cryptographic signing operation.
type Signer interface {
	// Sign signs the given digest and returns the raw signature bytes.
	Sign(digest []byte) ([]byte, error)

	// Algorithm returns the signature algorithm.
	Algorithm() Algorithm

	// CertificateChain returns the signer's certificate chain.
	// The first certificate is the signing certificate.
	CertificateChain() []*x509.Certificate
}

// LocalSigner signs using a local private key.
type LocalSigner struct {
	key   crypto.Signer
	certs []*x509.Certificate
	algo  Algorithm
}

// NewLocalSigner creates a signer from a local private key and certificate chain.
// The algorithm is auto-detected from the key type (RSA or ECDSA) with SHA-256.
func NewLocalSigner(key crypto.Signer, certs []*x509.Certificate) (*LocalSigner, error) {
	if len(certs) == 0 {
		return nil, errors.New("sign: no certificates provided")
	}

	var algo Algorithm
	switch key.(type) {
	case *rsa.PrivateKey:
		algo = SHA256WithRSA
	case *ecdsa.PrivateKey:
		algo = SHA256WithECDSA
	default:
		return nil, fmt.Errorf("sign: unsupported key type %T", key)
	}

	return &LocalSigner{key: key, certs: certs, algo: algo}, nil
}

// SetAlgorithm overrides the auto-detected algorithm.
func (s *LocalSigner) SetAlgorithm(algo Algorithm) {
	s.algo = algo
}

// Sign signs the given digest using the local private key.
func (s *LocalSigner) Sign(digest []byte) ([]byte, error) {
	return s.key.Sign(rand.Reader, digest, s.hashOpts())
}

// Algorithm returns the signature algorithm.
func (s *LocalSigner) Algorithm() Algorithm { return s.algo }

// CertificateChain returns the signer's certificate chain.
func (s *LocalSigner) CertificateChain() []*x509.Certificate { return s.certs }

// hashOpts returns the crypto.SignerOpts for the configured algorithm.
func (s *LocalSigner) hashOpts() crypto.SignerOpts {
	return s.algo.HashFunc()
}

// ExternalSigner delegates signing to an external function (HSM, KMS, etc.).
type ExternalSigner struct {
	signFn func(digest []byte) ([]byte, error)
	certs  []*x509.Certificate
	algo   Algorithm
}

// NewExternalSigner creates a signer that delegates to the given function.
// The function receives a hash digest and must return raw signature bytes.
func NewExternalSigner(
	signFn func(digest []byte) ([]byte, error),
	certs []*x509.Certificate,
	algo Algorithm,
) (*ExternalSigner, error) {
	if signFn == nil {
		return nil, errors.New("sign: signFn must not be nil")
	}
	if len(certs) == 0 {
		return nil, errors.New("sign: no certificates provided")
	}
	return &ExternalSigner{signFn: signFn, certs: certs, algo: algo}, nil
}

// Sign delegates signing of the given digest to the external function.
func (s *ExternalSigner) Sign(digest []byte) ([]byte, error) {
	return s.signFn(digest)
}

// Algorithm returns the signature algorithm.
func (s *ExternalSigner) Algorithm() Algorithm { return s.algo }

// CertificateChain returns the signer's certificate chain.
func (s *ExternalSigner) CertificateChain() []*x509.Certificate { return s.certs }

// ParsePKCS12 parses a PKCS#12 (.p12/.pfx) archive from bytes and returns a LocalSigner.
//
// PKCS#12 decoding is not implemented: it would require
// golang.org/x/crypto/pkcs12. Callers should parse the archive themselves
// and use NewLocalSigner with the pre-parsed key and certificate.
func ParsePKCS12(data []byte, password string) (*LocalSigner, error) {
	_, _ = data, password
	return nil, errors.New("sign: load PKCS12: PKCS12 loading requires golang.org/x/crypto/pkcs12; use NewLocalSigner with pre-parsed key and certificate")
}
