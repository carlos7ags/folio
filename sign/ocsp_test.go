// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// generateOCSPCertPair mints a self-signed CA and a leaf signed by it, with
// the leaf's OCSPServer pointed at responderURL. Pass "" to omit OCSPServer.
func generateOCSPCertPair(t *testing.T, responderURL string) (leaf, issuer *x509.Certificate) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Test Leaf"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if responderURL != "" {
		leafTemplate.OCSPServer = []string{responderURL}
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse leaf certificate: %v", err)
	}

	return leafCert, caCert
}

func okOCSPHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		der, err := asn1.Marshal(ocspResponse{ResponseStatus: 0})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(der)
	}
}

func TestOCSPClient_FetchResponse(t *testing.T) {
	var handlerDER []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		der, err := asn1.Marshal(ocspResponse{ResponseStatus: 0})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		handlerDER = der
		_, _ = w.Write(der)
	}))
	defer srv.Close()

	leaf, issuer := generateOCSPCertPair(t, srv.URL)

	resp, err := NewOCSPClient().FetchResponse(leaf, issuer)
	if err != nil {
		t.Fatalf("FetchResponse: %v", err)
	}
	if string(resp) != string(handlerDER) {
		t.Error("returned response bytes do not match the handler's DER")
	}
}

func TestOCSPClient_FetchResponse_NoResponderURL(t *testing.T) {
	leaf, issuer := generateOCSPCertPair(t, "")
	_, err := NewOCSPClient().FetchResponse(leaf, issuer)
	if err == nil || !strings.Contains(err.Error(), "certificate has no OCSP responder URL") {
		t.Fatalf("err = %v, want containing 'certificate has no OCSP responder URL'", err)
	}
}

func TestOCSPClient_FetchResponse_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	leaf, issuer := generateOCSPCertPair(t, srv.URL)
	_, err := NewOCSPClient().FetchResponse(leaf, issuer)
	if err == nil || !strings.Contains(err.Error(), "OCSP responder returned status 500") {
		t.Fatalf("err = %v, want containing 'OCSP responder returned status 500'", err)
	}
}

func TestOCSPClient_FetchResponse_MalformedDER(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not der"))
	}))
	defer srv.Close()

	leaf, issuer := generateOCSPCertPair(t, srv.URL)
	_, err := NewOCSPClient().FetchResponse(leaf, issuer)
	if err == nil || !strings.Contains(err.Error(), "invalid OCSP response") {
		t.Fatalf("err = %v, want containing 'invalid OCSP response'", err)
	}
}

func TestOCSPClient_FetchResponse_NonZeroStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		der, err := asn1.Marshal(ocspResponse{ResponseStatus: 1})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(der)
	}))
	defer srv.Close()

	leaf, issuer := generateOCSPCertPair(t, srv.URL)
	_, err := NewOCSPClient().FetchResponse(leaf, issuer)
	if err == nil || !strings.Contains(err.Error(), "OCSP response status 1") {
		t.Fatalf("err = %v, want containing 'OCSP response status 1'", err)
	}
}

func TestOCSPClient_FetchChainResponses(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		srv := httptest.NewServer(okOCSPHandler(t))
		defer srv.Close()

		leaf, issuer := generateOCSPCertPair(t, srv.URL)
		responses, err := NewOCSPClient().FetchChainResponses([]*x509.Certificate{leaf, issuer})
		if err != nil {
			t.Fatalf("FetchChainResponses: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("len(responses) = %d, want 1", len(responses))
		}
	})

	t.Run("BestEffortSkip", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		leaf, issuer := generateOCSPCertPair(t, srv.URL)
		responses, err := NewOCSPClient().FetchChainResponses([]*x509.Certificate{leaf, issuer})
		if err != nil {
			t.Fatalf("FetchChainResponses: unexpected error %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) = %d, want 0", len(responses))
		}
	})
}

func TestCollectValidationData(t *testing.T) {
	t.Run("NilClient", func(t *testing.T) {
		leaf, issuer := generateOCSPCertPair(t, "")
		resp, err := CollectValidationData([]*x509.Certificate{leaf, issuer}, nil)
		if err != nil {
			t.Fatalf("CollectValidationData: unexpected error %v", err)
		}
		if resp != nil {
			t.Fatalf("resp = %v, want nil", resp)
		}
	})

	t.Run("Delegates", func(t *testing.T) {
		srv := httptest.NewServer(okOCSPHandler(t))
		defer srv.Close()

		leaf, issuer := generateOCSPCertPair(t, srv.URL)
		resp, err := CollectValidationData([]*x509.Certificate{leaf, issuer}, NewOCSPClient())
		if err != nil {
			t.Fatalf("CollectValidationData: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("len(resp) = %d, want 1", len(resp))
		}
	})
}
