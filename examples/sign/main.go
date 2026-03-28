// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Sign demonstrates digitally signing a PDF with a PAdES B-B signature.
//
// The example generates a self-signed RSA certificate at runtime (for
// demonstration only — production use requires a real certificate from
// a trusted CA), creates a PDF, and signs it.
//
// PAdES conformance levels supported by Folio:
//   - B-B:   basic signature (this example)
//   - B-T:   adds an RFC 3161 timestamp (requires a TSA server)
//   - B-LT:  embeds revocation data in a Document Security Store
//   - B-LTA: adds a document timestamp for long-term archival
//
// Usage:
//
//	go run ./examples/sign
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
	"github.com/carlos7ags/folio/sign"
)

func main() {
	// --- Step 1: Generate a self-signed certificate ---
	fmt.Println("Generating RSA-2048 key pair and self-signed certificate...")
	key, cert, err := generateCert()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cert:", err)
		os.Exit(1)
	}
	fmt.Printf("  Subject: %s\n", cert.Subject.CommonName)
	fmt.Printf("  Valid:   %s to %s\n",
		cert.NotBefore.Format("2006-01-02"),
		cert.NotAfter.Format("2006-01-02"))

	// --- Step 2: Create a PDF ---
	fmt.Println("\nCreating PDF...")
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Signed Document"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewParagraph("Signed Document", font.HelveticaBold, 20))
	doc.Add(layout.NewLineSeparator().
		SetWidth(1).
		SetColor(layout.RGB(0.7, 0.7, 0.7)).
		SetSpaceBefore(6).
		SetSpaceAfter(12))
	doc.Add(layout.NewParagraph(
		"This PDF has been digitally signed with a PAdES B-B signature. "+
			"The signature proves the document has not been modified since signing. "+
			"Open in Adobe Acrobat to see the signature panel.",
		font.Helvetica, 11))
	doc.Add(layout.NewStyledParagraph(
		layout.NewRun("Signer: ", font.HelveticaBold, 10),
		layout.NewRun(cert.Subject.CommonName, font.Helvetica, 10),
	).SetSpaceBefore(8))
	doc.Add(layout.NewStyledParagraph(
		layout.NewRun("Organization: ", font.HelveticaBold, 10),
		layout.NewRun(strings.Join(cert.Subject.Organization, ", "), font.Helvetica, 10),
	))
	doc.Add(layout.NewStyledParagraph(
		layout.NewRun("Signed at: ", font.HelveticaBold, 10),
		layout.NewRun(time.Now().Format("2006-01-02 15:04:05 MST"), font.Helvetica, 10),
	))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("  Unsigned PDF: %d bytes\n", buf.Len())

	// --- Step 3: Sign the PDF ---
	fmt.Println("\nSigning with PAdES B-B (RSA + SHA-256)...")
	signer, err := sign.NewLocalSigner(key, []*x509.Certificate{cert})
	if err != nil {
		fmt.Fprintln(os.Stderr, "signer:", err)
		os.Exit(1)
	}

	signed, err := sign.SignPDF(buf.Bytes(), sign.Options{
		Signer:      signer,
		Level:       sign.LevelBB,
		Name:        cert.Subject.CommonName,
		Reason:      "Document approval",
		Location:    "New York, NY",
		ContactInfo: "legal@example.com",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "sign:", err)
		os.Exit(1)
	}
	fmt.Printf("  Signed PDF: %d bytes (+%d bytes for signature)\n",
		len(signed), len(signed)-buf.Len())

	// --- Step 4: Save ---
	if err := os.WriteFile("signed.pdf", signed, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}

	// --- Step 5: Verify the signed PDF is readable ---
	r, err := reader.Parse(signed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify parse:", err)
		os.Exit(1)
	}
	fmt.Printf("\nVerification:\n")
	fmt.Printf("  Pages: %d\n", r.PageCount())
	fmt.Printf("  Version: %s\n", r.Version())
	title, author, _, _, _ := r.Info()
	fmt.Printf("  Title: %s\n", title)
	fmt.Printf("  Author: %s\n", author)

	// Check for signature dictionary in the PDF.
	if strings.Contains(string(signed), "/Type /Sig") {
		fmt.Println("  Signature: present")
	}
	if strings.Contains(string(signed), "/ByteRange") {
		fmt.Println("  ByteRange: present")
	}

	fmt.Println("\nCreated signed.pdf — open in Adobe Acrobat to view the signature panel")
}

// generateCert creates a self-signed RSA certificate for demonstration.
// Production use requires a certificate from a trusted CA.
func generateCert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Folio Example Signer",
			Organization: []string{"Folio PDF Library"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate: %w", err)
	}

	return key, cert, nil
}
