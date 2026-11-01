// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestSignatureTimestamp_OpenSSL uses OpenSSL as both issuer and verifier.
// Folio supplies the request and final PDF. All keys and files are temporary.
func TestSignatureTimestamp_OpenSSL(t *testing.T) {
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("OpenSSL is required for independent timestamp verification")
	}
	for _, algo := range []Algorithm{SHA256WithRSA, SHA384WithRSA, SHA512WithRSA, SHA256WithECDSA, SHA384WithECDSA, SHA512WithECDSA} {
		t.Run(algo.SignatureOID().String(), func(t *testing.T) {
			dir := t.TempDir()
			write := func(name string, data []byte) string {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			run := func(args ...string) ([]byte, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				return exec.CommandContext(ctx, openssl, args...).CombinedOutput()
			}
			mustRun := func(args ...string) {
				if out, err := run(args...); err != nil {
					t.Fatalf("openssl %v: %v\n%s", args, err, out)
				}
			}
			// RFC 3161 requires a critical EKU containing only timeStamping.
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				t.Fatal(err)
			}
			eku, err := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
			if err != nil {
				t.Fatal(err)
			}
			template := &x509.Certificate{
				SerialNumber: big.NewInt(10), Subject: pkix.Name{CommonName: "Folio test TSA"},
				NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true,
				ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true, Value: eku}},
			}
			certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
			if err != nil {
				t.Fatal(err)
			}
			certPath := write("tsa.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
			keyPath := write("tsa-key.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
			serialPath := write("serial", []byte("01\n"))
			config := write("tsa.cnf", []byte(fmt.Sprintf(`[tsa]
default_tsa = local
[local]
serial = %s
crypto_device = builtin
signer_cert = %s
signer_key = %s
signer_digest = sha256
default_policy = 1.2.3.4
digests = sha256,sha384,sha512
accuracy = secs:1
ordering = yes
tsa_name = yes
ess_cert_id_chain = no
`, filepath.ToSlash(serialPath), filepath.ToSlash(certPath), filepath.ToSlash(keyPath))))
			requests := 0
			client := timestampTestClient(t, func(request []byte) []byte {
				requests++
				query := write("request.tsq", request)
				reply := filepath.Join(dir, "response.tsr")
				mustRun("ts", "-reply", "-config", config, "-queryfile", query, "-out", reply)
				data, err := os.ReadFile(reply)
				if err != nil {
					t.Fatal(err)
				}
				return data
			})
			var signer *LocalSigner
			if algo <= SHA512WithRSA {
				k, c := generateTestRSACert(t)
				signer, err = NewLocalSigner(k, []*x509.Certificate{c})
			} else {
				k, c := generateTestECDSACert(t)
				signer, err = NewLocalSigner(k, []*x509.Certificate{c})
			}
			if err != nil {
				t.Fatal(err)
			}
			signer.SetAlgorithm(algo)
			pdf, err := SignPDF(minimalPDF(t), Options{Signer: signer, Level: LevelBT, TSAClient: client})
			if err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("TSA requests = %d", requests)
			}
			locations, _, err := locateSignaturesGraph(pdf)
			if err != nil {
				t.Fatal(err)
			}
			if len(locations) != 1 {
				t.Fatalf("signature entries = %d", len(locations))
			}
			loc := locations[0]
			si := timestampSignerInfo(t, loc.der)
			tokenPath := write("token.der", signatureTimestampToken(t, si))
			signaturePath := write("signature.bin", si.Signature)
			mustRun("ts", "-verify", "-token_in", "-in", tokenPath, "-data", signaturePath, "-CAfile", certPath)
			// The old binding must fail even though the TSA token is genuine.
			br := loc.byteRange
			content := append(bytes.Clone(pdf[br[0]:br[0]+br[1]]), pdf[br[2]:br[2]+br[3]]...)
			contentPath := write("content.bin", content)
			if out, err := run("ts", "-verify", "-token_in", "-in", tokenPath, "-data", contentPath, "-CAfile", certPath); err == nil || !bytes.Contains(out, []byte("message imprint mismatch")) {
				t.Fatalf("old binding: err = %v\n%s", err, out)
			}
			// Verify the detached PDF signature with a separate crypto engine.
			cmsPath := write("signature.der", loc.der)
			mustRun("cms", "-verify", "-binary", "-inform", "DER", "-in", cmsPath, "-content", contentPath, "-noverify", "-out", filepath.Join(dir, "verified.bin"))
		})
	}
}
