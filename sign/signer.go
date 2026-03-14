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
	"io"
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

func (s *LocalSigner) Sign(digest []byte) ([]byte, error) {
	return s.key.Sign(rand.Reader, digest, s.hashOpts())
}

func (s *LocalSigner) Algorithm() Algorithm                  { return s.algo }
func (s *LocalSigner) CertificateChain() []*x509.Certificate { return s.certs }

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

func (s *ExternalSigner) Sign(digest []byte) ([]byte, error) {
	return s.signFn(digest)
}

func (s *ExternalSigner) Algorithm() Algorithm                  { return s.algo }
func (s *ExternalSigner) CertificateChain() []*x509.Certificate { return s.certs }

// LoadPKCS12 loads a PKCS#12 (.p12/.pfx) file and returns a LocalSigner.
func LoadPKCS12(data []byte, password string) (*LocalSigner, error) {
	key, cert, err := decodePKCS12(data, password)
	if err != nil {
		return nil, fmt.Errorf("sign: load PKCS12: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("sign: private key does not implement crypto.Signer")
	}
	return NewLocalSigner(signer, []*x509.Certificate{cert})
}

// decodePKCS12 is a minimal PKCS#12 decoder for the common case of
// one private key + one certificate. Uses the Go standard library.
func decodePKCS12(data []byte, password string) (crypto.PrivateKey, *x509.Certificate, error) {
	// Go 1.24+ has crypto/x509.ParsePKCS12 but for broader compatibility
	// we use the x/crypto/pkcs12 package pattern. For now, use a simple
	// implementation that works with the standard PKCS#12 format.
	//
	// This is a placeholder — in production, use golang.org/x/crypto/pkcs12.
	// For the initial implementation, users can provide pre-parsed keys.
	_, _, _ = data, password, io.Discard
	return nil, nil, errors.New("sign: PKCS12 loading requires golang.org/x/crypto/pkcs12; use NewLocalSigner with pre-parsed key and certificate")
}
