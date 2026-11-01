// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type timestampTransport func(*http.Request) (*http.Response, error)

func (f timestampTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// timestampTestClient records the wire request. Its placeholder token tests
// transport and CMS assembly only; the OpenSSL tests use real RFC 3161 tokens.
func timestampTestClient(t *testing.T, reply func([]byte) []byte) *TSAClient {
	t.Helper()
	return &TSAClient{URL: "https://tsa.example.test", HTTPClient: &http.Client{
		Transport: timestampTransport(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			if r.Header.Get("Content-Type") != "application/timestamp-query" {
				t.Error("wrong TSA content type")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(reply(body)))}, nil
		}),
	}}
}

func timestampResponse(t *testing.T, token []byte) []byte {
	t.Helper()
	b, err := asn1.Marshal(timeStampResp{Status: pkiStatusInfo{Status: 0}, TimeStampToken: asn1.RawValue{FullBytes: token}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func timestampSignerInfo(t *testing.T, der []byte) signerInfo {
	t.Helper()
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		t.Fatal(err)
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		t.Fatal(err)
	}
	var si signerInfo
	if rest, err := asn1.Unmarshal(sd.SignerInfos.Bytes, &si); err != nil || len(rest) != 0 {
		t.Fatalf("SignerInfo: %v, trailing bytes %d", err, len(rest))
	}
	return si
}

func signatureTimestampToken(t *testing.T, si signerInfo) []byte {
	t.Helper()
	var attr attribute
	if rest, err := asn1.Unmarshal(si.UnsignedAttrs.Bytes, &attr); err != nil || len(rest) != 0 {
		t.Fatalf("timestamp attribute: %v", err)
	}
	if !attr.Type.Equal(oidTimeStampToken) {
		t.Fatalf("timestamp OID = %v", attr.Type)
	}
	return attr.Values.Bytes
}

func TestBuildCMS_TimestampBindsSignature(t *testing.T) {
	for _, algo := range []Algorithm{SHA256WithRSA, SHA384WithRSA, SHA512WithRSA, SHA256WithECDSA, SHA384WithECDSA, SHA512WithECDSA} {
		t.Run(algo.SignatureOID().String(), func(t *testing.T) {
			var key crypto.Signer
			var cert *x509.Certificate
			if algo <= SHA512WithRSA {
				key, cert = generateTestRSACert(t)
			} else {
				key, cert = generateTestECDSACert(t)
			}
			local, err := NewLocalSigner(key, []*x509.Certificate{cert})
			if err != nil {
				t.Fatal(err)
			}
			local.SetAlgorithm(algo)
			calls := 0
			var signedValue []byte
			external, err := NewExternalSigner(func(digest []byte) ([]byte, error) {
				calls++
				var err error
				signedValue, err = local.Sign(digest)
				return signedValue, err
			}, []*x509.Certificate{cert}, algo)
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			var imprint messageImprint
			token := []byte{0x30, 0x00}
			client := timestampTestClient(t, func(body []byte) []byte {
				requests++
				if calls != 1 {
					t.Errorf("sign calls before TSA = %d, want 1", calls)
				}
				var req timeStampReq
				if _, err := asn1.Unmarshal(body, &req); err != nil {
					t.Fatal(err)
				}
				imprint = req.MessageImprint
				return timestampResponse(t, token)
			})
			digest := hashBytes(algo.HashFunc(), []byte("PDF byte ranges"))
			cms, err := buildCMS(digest, external, time.Now(), client)
			if err != nil {
				t.Fatal(err)
			}
			si := timestampSignerInfo(t, cms)
			if calls != 1 || requests != 1 {
				t.Fatalf("sign calls = %d, TSA calls = %d", calls, requests)
			}
			if !bytes.Equal(si.Signature, signedValue) {
				t.Error("CMS contains a different signature")
			}
			if !bytes.Equal(imprint.HashedMessage, hashBytes(algo.HashFunc(), si.Signature)) {
				t.Error("TSA imprint does not hash the embedded signature")
			}
			if bytes.Equal(imprint.HashedMessage, digest) {
				t.Error("TSA imprint contains the PDF digest")
			}
			if !imprint.HashAlgorithm.Algorithm.Equal(algo.DigestOID()) {
				t.Error("wrong timestamp digest algorithm")
			}
			if !bytes.Equal(signatureTimestampToken(t, si), token) {
				t.Error("timestamp token changed")
			}
			parsed, err := parseCMS(cms)
			if err != nil {
				t.Fatal(err)
			}
			if !verifySignatureCMS(parsed) || !bytes.Equal(parsed.messageDigest, digest) {
				t.Error("CMS signature or document digest is invalid")
			}
		})
	}
}

func TestSignPDF_TimestampLevels(t *testing.T) {
	for _, level := range []PAdESLevel{LevelBB, LevelBT, LevelBLT, LevelBLTA} {
		t.Run(string(rune('0'+level)), func(t *testing.T) {
			var requests []timeStampReq
			client := timestampTestClient(t, func(body []byte) []byte {
				var req timeStampReq
				if _, err := asn1.Unmarshal(body, &req); err != nil {
					t.Fatal(err)
				}
				requests = append(requests, req)
				return timestampResponse(t, []byte{0x30, 0x00})
			})
			pdf, err := SignPDF(minimalPDF(t), Options{Signer: testSigner(t), Level: level, TSAClient: client})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if level == LevelBB {
				want = 0
			}
			if level == LevelBLTA {
				want = 2
			}
			if len(requests) != want {
				t.Fatalf("TSA calls = %d, want %d", len(requests), want)
			}
			locs, hasDSS, err := locateSignaturesGraph(pdf)
			if err != nil {
				t.Fatal(err)
			}
			if hasDSS != (level >= LevelBLT) {
				t.Error("unexpected DSS presence")
			}
			for _, loc := range locs {
				if loc.isDocTimeStamp {
					digest, err := hashByteRange(pdf, loc.byteRange, crypto.SHA256)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(requests[1].MessageImprint.HashedMessage, digest) {
						t.Error("document timestamp does not cover PDF bytes")
					}
					continue
				}
				parsed, err := parseCMS(loc.der)
				if err != nil {
					t.Fatal(err)
				}
				if !verifySignatureCMS(parsed) {
					t.Error("invalid signature")
				}
				if level >= LevelBT && !bytes.Equal(requests[0].MessageImprint.HashedMessage, hashBytes(crypto.SHA256, parsed.signature)) {
					t.Error("signature timestamp binding is wrong")
				}
				if parsed.hasTimestamp != (level >= LevelBT) {
					t.Error("unexpected timestamp presence")
				}
			}
			if len(locs) != 1+(want/2) {
				t.Fatalf("signature entries = %d", len(locs))
			}
		})
	}
}

func TestSignPDF_TimestampFailures(t *testing.T) {
	sentinel := errors.New("signer unavailable")
	local := testSigner(t)
	failing, err := NewExternalSigner(func([]byte) ([]byte, error) { return nil, sentinel }, local.CertificateChain(), local.Algorithm())
	if err != nil {
		t.Fatal(err)
	}
	client := timestampTestClient(t, func([]byte) []byte { t.Error("TSA called after signer failure"); return nil })
	pdf, err := SignPDF(minimalPDF(t), Options{Signer: failing, Level: LevelBT, TSAClient: client})
	if !errors.Is(err, sentinel) || pdf != nil {
		t.Fatalf("signer failure: PDF = %d bytes, err = %v", len(pdf), err)
	}
	for _, tc := range []struct {
		name   string
		status int
		body   []byte
		want   string
	}{
		{"HTTP", 500, nil, "status 500"},
		{"malformed", 200, []byte("invalid DER"), "parse TSA response"},
		{"rejected", 200, []byte{0x30, 5, 0x30, 3, 2, 1, 2}, "rejected"},
		{"missing token", 200, []byte{0x30, 5, 0x30, 3, 2, 1, 0}, "no timestamp token"},
	} {
		for _, level := range []PAdESLevel{LevelBT, LevelBLT, LevelBLTA} {
			t.Run(tc.name+"/"+string(rune('0'+level)), func(t *testing.T) {
				client := &TSAClient{URL: "https://tsa.example.test", HTTPClient: &http.Client{Transport: timestampTransport(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: tc.status, Body: io.NopCloser(bytes.NewReader(tc.body)), Header: make(http.Header)}, nil
				})}}
				pdf, err := SignPDF(minimalPDF(t), Options{Signer: local, Level: level, TSAClient: client})
				if err == nil || !strings.Contains(err.Error(), tc.want) || pdf != nil {
					t.Fatalf("PDF = %d bytes, err = %v, want %s", len(pdf), err, tc.want)
				}
			})
		}
	}
}

func TestSignPDF_DocumentTimestampFailure(t *testing.T) {
	calls := 0
	client := timestampTestClient(t, func([]byte) []byte {
		calls++
		if calls == 1 {
			return timestampResponse(t, []byte{0x30, 0x00})
		}
		return []byte("invalid DER")
	})
	pdf, err := SignPDF(minimalPDF(t), Options{Signer: testSigner(t), Level: LevelBLTA, TSAClient: client})
	if calls != 2 || pdf != nil || err == nil || !strings.Contains(err.Error(), "document timestamp") {
		t.Fatalf("calls = %d, PDF = %d bytes, err = %v", calls, len(pdf), err)
	}
}
