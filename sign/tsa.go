// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// TSAClient is an RFC 3161 Time-Stamp Authority client.
// It sends a TimeStampReq to the TSA URL and returns the timestamp token.
type TSAClient struct {
	// URL is the TSA endpoint (e.g., "http://timestamp.digicert.com").
	URL string

	// HTTPClient is the HTTP client to use. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// NewTSAClient creates a TSA client for the given URL.
func NewTSAClient(url string) *TSAClient {
	return &TSAClient{URL: url}
}

// Timestamp sends a timestamp request and returns the DER-encoded timestamp token.
// The data parameter is the digest to timestamp, hashFunc identifies the hash algorithm.
func (c *TSAClient) Timestamp(digest []byte, hashFunc crypto.Hash) ([]byte, error) {
	if c.URL == "" {
		return nil, errors.New("sign: TSA URL is empty")
	}

	// Build RFC 3161 TimeStampReq.
	reqDER, err := buildTimestampReq(digest, hashFunc)
	if err != nil {
		return nil, fmt.Errorf("sign: build TSA request: %w", err)
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Post(c.URL, "application/timestamp-query", bytes.NewReader(reqDER))
	if err != nil {
		return nil, fmt.Errorf("sign: TSA request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sign: TSA returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sign: read TSA response: %w", err)
	}

	// Parse TimeStampResp and extract the timestamp token.
	return parseTimestampResp(body)
}

// timeStampReq is the ASN.1 TimeStampReq structure (RFC 3161).
type timeStampReq struct {
	Version        int
	MessageImprint messageImprint
	CertReq        bool `asn1:"optional"`
}

// messageImprint identifies the hash algorithm and digest value in a timestamp request.
type messageImprint struct {
	HashAlgorithm algorithmIdentifier
	HashedMessage []byte
}

// timeStampResp is the ASN.1 TimeStampResp structure (RFC 3161).
type timeStampResp struct {
	Status         pkiStatusInfo
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

// pkiStatusInfo is the status field of a TimeStampResp.
type pkiStatusInfo struct {
	Status int
}

// buildTimestampReq creates a DER-encoded RFC 3161 TimeStampReq.
func buildTimestampReq(digest []byte, hashFunc crypto.Hash) ([]byte, error) {
	var hashOID asn1.ObjectIdentifier
	switch hashFunc {
	case crypto.SHA256:
		hashOID = oidSHA256
	case crypto.SHA384:
		hashOID = oidSHA384
	case crypto.SHA512:
		hashOID = oidSHA512
	default:
		return nil, fmt.Errorf("unsupported hash function: %v", hashFunc)
	}

	req := timeStampReq{
		Version: 1,
		MessageImprint: messageImprint{
			HashAlgorithm: algorithmIdentifier{Algorithm: hashOID},
			HashedMessage: digest,
		},
		CertReq: true,
	}
	return asn1.Marshal(req)
}

// parseTimestampResp parses a DER-encoded TimeStampResp and returns the token.
func parseTimestampResp(data []byte) ([]byte, error) {
	var resp timeStampResp
	_, err := asn1.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("sign: parse TSA response: %w", err)
	}

	// Status 0 = granted, 1 = grantedWithMods.
	if resp.Status.Status > 1 {
		return nil, fmt.Errorf("sign: TSA rejected request (status %d)", resp.Status.Status)
	}

	if len(resp.TimeStampToken.FullBytes) == 0 {
		return nil, errors.New("sign: TSA response contains no timestamp token")
	}

	return resp.TimeStampToken.FullBytes, nil
}
