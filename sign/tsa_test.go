// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestTSA returns an httptest.Server that parses the RFC 3161 request,
// builds a CMS token over the requested digest with buildCMS, and responds
// with a granted TimeStampResp. It records the last built token so tests can
// assert the client returned it unchanged.
func newTestTSA(t *testing.T, signer Signer) (*httptest.Server, *[]byte) {
	t.Helper()
	var lastToken []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/timestamp-query" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var req timeStampReq
		if _, err := asn1.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad req", http.StatusBadRequest)
			return
		}
		if !req.CertReq {
			t.Errorf("expected CertReq == true")
		}
		token, err := buildCMS(req.MessageImprint.HashedMessage, signer, time.Now(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		lastToken = token
		respDER, err := asn1.Marshal(timeStampResp{
			Status:         pkiStatusInfo{Status: 0},
			TimeStampToken: asn1.RawValue{FullBytes: token},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(respDER)
	}))
	return srv, &lastToken
}

func testSigner(t *testing.T) Signer {
	t.Helper()
	key, cert := generateTestRSACert(t)
	signer, err := NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	return signer
}

func TestTSAClient_Timestamp(t *testing.T) {
	signer := testSigner(t)
	srv, lastToken := newTestTSA(t, signer)
	defer srv.Close()

	digest := sha256.Sum256([]byte("hello world"))
	token, err := NewTSAClient(srv.URL).Timestamp(digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("Timestamp: %v", err)
	}
	if len(token) == 0 {
		t.Fatal("expected non-empty token")
	}
	if string(token) != string(*lastToken) {
		t.Error("returned token does not match the token the handler produced")
	}
}

func TestBuildTimestampReq_RoundTrip(t *testing.T) {
	digest := sha256.Sum256([]byte("some data"))
	der, err := buildTimestampReq(digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("buildTimestampReq: %v", err)
	}
	var req timeStampReq
	if _, err := asn1.Unmarshal(der, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Version != 1 {
		t.Errorf("Version = %d, want 1", req.Version)
	}
	if string(req.MessageImprint.HashedMessage) != string(digest[:]) {
		t.Error("digest mismatch")
	}
	if !req.MessageImprint.HashAlgorithm.Algorithm.Equal(oidSHA256) {
		t.Errorf("HashAlgorithm = %v, want SHA-256 OID", req.MessageImprint.HashAlgorithm.Algorithm)
	}
	if !req.CertReq {
		t.Error("expected CertReq == true")
	}
}

func TestBuildTimestampReq_UnsupportedHash(t *testing.T) {
	if _, err := buildTimestampReq([]byte("digest"), crypto.MD5); err == nil {
		t.Error("expected error for unsupported hash function")
	}
}

func TestTSAClient_Timestamp_Negative(t *testing.T) {
	t.Run("HTTPError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := NewTSAClient(srv.URL).Timestamp([]byte("digest"), crypto.SHA256)
		if err == nil || !strings.Contains(err.Error(), "TSA returned status 500") {
			t.Fatalf("err = %v, want containing 'TSA returned status 500'", err)
		}
	})

	t.Run("GarbageBody", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not der"))
		}))
		defer srv.Close()

		_, err := NewTSAClient(srv.URL).Timestamp([]byte("digest"), crypto.SHA256)
		if err == nil || !strings.Contains(err.Error(), "parse TSA response") {
			t.Fatalf("err = %v, want containing 'parse TSA response'", err)
		}
	})

	t.Run("RejectedStatus", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respDER, err := asn1.Marshal(timeStampResp{Status: pkiStatusInfo{Status: 2}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(respDER)
		}))
		defer srv.Close()

		_, err := NewTSAClient(srv.URL).Timestamp([]byte("digest"), crypto.SHA256)
		if err == nil || !strings.Contains(err.Error(), "TSA rejected request (status 2)") {
			t.Fatalf("err = %v, want containing 'TSA rejected request (status 2)'", err)
		}
	})

	t.Run("GrantedButEmptyToken", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respDER, err := asn1.Marshal(timeStampResp{Status: pkiStatusInfo{Status: 0}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(respDER)
		}))
		defer srv.Close()

		_, err := NewTSAClient(srv.URL).Timestamp([]byte("digest"), crypto.SHA256)
		if err == nil || !strings.Contains(err.Error(), "TSA response contains no timestamp token") {
			t.Fatalf("err = %v, want containing 'TSA response contains no timestamp token'", err)
		}
	})

	t.Run("EmptyURL", func(t *testing.T) {
		_, err := NewTSAClient("").Timestamp([]byte("digest"), crypto.SHA256)
		if err == nil || !strings.Contains(err.Error(), "TSA URL is empty") {
			t.Fatalf("err = %v, want containing 'TSA URL is empty'", err)
		}
	})
}
