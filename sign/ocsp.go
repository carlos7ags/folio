// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// OCSPClient fetches OCSP responses for certificate revocation checking.
type OCSPClient struct {
	// HTTPClient is the HTTP client to use. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// NewOCSPClient creates a new OCSP client.
func NewOCSPClient() *OCSPClient {
	return &OCSPClient{}
}

// FetchResponse sends an OCSP request for the given certificate and returns
// the DER-encoded OCSP response. The issuer certificate is required to
// build the request.
func (c *OCSPClient) FetchResponse(cert, issuer *x509.Certificate) ([]byte, error) {
	if len(cert.OCSPServer) == 0 {
		return nil, errors.New("sign: certificate has no OCSP responder URL")
	}

	responderURL := cert.OCSPServer[0]

	reqDER, err := buildOCSPRequest(cert, issuer)
	if err != nil {
		return nil, fmt.Errorf("sign: build OCSP request: %w", err)
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Post(responderURL, "application/ocsp-request", bytes.NewReader(reqDER))
	if err != nil {
		return nil, fmt.Errorf("sign: OCSP request to %s: %w", responderURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sign: OCSP responder returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sign: read OCSP response: %w", err)
	}

	if err := validateOCSPResponse(body); err != nil {
		return nil, fmt.Errorf("sign: invalid OCSP response: %w", err)
	}

	return body, nil
}

// FetchChainResponses fetches OCSP responses for all certificates in the chain
// (except the root/self-signed). Returns responses in chain order.
// Certificates without OCSP responder URLs are skipped.
func (c *OCSPClient) FetchChainResponses(chain []*x509.Certificate) ([][]byte, error) {
	var responses [][]byte

	for i := 0; i < len(chain)-1; i++ {
		resp, err := c.FetchResponse(chain[i], chain[i+1])
		if err != nil {
			continue // OCSP is best-effort
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

// ASN.1 OCSP structures (RFC 6960).

type ocspRequest struct {
	TBSRequest tbsRequest
}

type tbsRequest struct {
	RequestList []request
}

type request struct {
	ReqCert certID
}

type certID struct {
	HashAlgorithm  algorithmIdentifier
	IssuerNameHash []byte
	IssuerKeyHash  []byte
	SerialNumber   asn1.RawValue
}

type ocspResponse struct {
	ResponseStatus asn1.Enumerated
	ResponseBytes  responseBytes `asn1:"explicit,tag:0,optional"`
}

type responseBytes struct {
	ResponseType asn1.ObjectIdentifier
	Response     []byte
}

// buildOCSPRequest creates a DER-encoded OCSP request for the given cert/issuer pair.
func buildOCSPRequest(cert, issuer *x509.Certificate) ([]byte, error) {
	issuerNameHash := hashBytes(crypto.SHA256, issuer.RawSubject)
	issuerKeyHash := hashBytes(crypto.SHA256, issuer.RawSubjectPublicKeyInfo)

	serialDER, err := asn1.Marshal(cert.SerialNumber)
	if err != nil {
		return nil, err
	}

	req := ocspRequest{
		TBSRequest: tbsRequest{
			RequestList: []request{{
				ReqCert: certID{
					HashAlgorithm:  algorithmIdentifier{Algorithm: oidSHA256},
					IssuerNameHash: issuerNameHash,
					IssuerKeyHash:  issuerKeyHash,
					SerialNumber:   asn1.RawValue{FullBytes: serialDER},
				},
			}},
		},
	}

	return asn1.Marshal(req)
}

// validateOCSPResponse performs basic structural validation of an OCSP response.
func validateOCSPResponse(data []byte) error {
	var resp ocspResponse
	_, err := asn1.Unmarshal(data, &resp)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if resp.ResponseStatus != 0 {
		return fmt.Errorf("OCSP response status %d (not successful)", resp.ResponseStatus)
	}
	return nil
}
