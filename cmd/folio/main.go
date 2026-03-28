// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Command folio is a CLI tool for PDF operations.
//
// Usage:
//
//	folio merge -o output.pdf input1.pdf input2.pdf [input3.pdf ...]
//	folio info file.pdf
//	folio pages file.pdf
//	folio text file.pdf [page_number]
//	folio extract file.pdf [-page N] [-strategy simple|location]
//	folio sign -cert cert.pem -key key.pem [-o signed.pdf] input.pdf
//	folio create -o output.pdf [-title "Title"] [-text "Content"]
//	folio blank -o output.pdf [-size letter|a4] [-pages N]
//	folio version
//	folio help
package main

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
	"github.com/carlos7ags/folio/sign"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "merge":
		err = cmdMerge(args)
	case "info":
		err = cmdInfo(args)
	case "pages":
		err = cmdPages(args)
	case "text":
		err = cmdText(args)
	case "create":
		err = cmdCreate(args)
	case "blank":
		err = cmdBlank(args)
	case "extract":
		err = cmdExtract(args)
	case "sign":
		err = cmdSign(args)
	case "version":
		fmt.Println("folio 0.1.1")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// printUsage writes the CLI help text to standard output.
func printUsage() {
	fmt.Print(`folio — A modern PDF toolkit

Usage:
  folio merge -o output.pdf input1.pdf input2.pdf ...
  folio info file.pdf
  folio pages file.pdf
  folio text file.pdf [page_number]
  folio extract file.pdf [-page N] [-strategy simple|location]
  folio sign -cert cert.pem -key key.pem [-o signed.pdf] input.pdf
  folio create -o output.pdf [-title "Title"] [-text "Content"]
  folio blank -o output.pdf [-size letter|a4] [-pages N]
  folio version

Commands:
  merge    Concatenate multiple PDFs into one
  info     Show PDF metadata (title, author, pages, version)
  pages    List page dimensions
  text     Extract text from a page (simple extraction)
  extract  Extract text with strategy (simple, location)
  sign     Digitally sign a PDF with PAdES
  create   Create a simple PDF with text content
  blank    Create a blank PDF with N pages
  version  Show folio version
`)
}

// cmdMerge concatenates multiple PDF files into a single output file.
func cmdMerge(args []string) error {
	output, inputs := parseMergeArgs(args)
	if output == "" || len(inputs) < 1 {
		return fmt.Errorf("usage: folio merge -o output.pdf input1.pdf input2.pdf")
	}

	var readers []*reader.PdfReader
	for _, path := range inputs {
		r, err := reader.Load(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		readers = append(readers, r)
	}

	m, err := reader.Merge(readers...)
	if err != nil {
		return err
	}

	if err := m.SaveTo(output); err != nil {
		return err
	}

	totalPages := 0
	for _, r := range readers {
		totalPages += r.PageCount()
	}
	fmt.Printf("Merged %d files (%d pages) → %s\n", len(inputs), totalPages, output)
	return nil
}

// parseMergeArgs extracts the output path and input file paths from the merge command arguments.
func parseMergeArgs(args []string) (output string, inputs []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			output = args[i+1]
			i++
		} else {
			inputs = append(inputs, args[i])
		}
	}
	return
}

// cmdInfo displays PDF metadata including title, author, version, and page count.
func cmdInfo(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: folio info file.pdf")
	}

	r, err := reader.Load(args[0])
	if err != nil {
		return err
	}

	title, author, subject, creator, producer := r.Info()

	fmt.Printf("File:     %s\n", args[0])
	fmt.Printf("Version:  PDF %s\n", r.Version())
	fmt.Printf("Pages:    %d\n", r.PageCount())
	if title != "" {
		fmt.Printf("Title:    %s\n", title)
	}
	if author != "" {
		fmt.Printf("Author:   %s\n", author)
	}
	if subject != "" {
		fmt.Printf("Subject:  %s\n", subject)
	}
	if creator != "" {
		fmt.Printf("Creator:  %s\n", creator)
	}
	if producer != "" {
		fmt.Printf("Producer: %s\n", producer)
	}

	return nil
}

// cmdPages lists the dimensions and rotation of each page in the PDF.
func cmdPages(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: folio pages file.pdf")
	}

	r, err := reader.Load(args[0])
	if err != nil {
		return err
	}

	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			continue
		}
		rot := ""
		if page.Rotate != 0 {
			rot = fmt.Sprintf(" (rotated %d°)", page.Rotate)
		}
		fmt.Printf("Page %d: %.1f x %.1f pt%s\n", i+1, page.Width, page.Height, rot)
	}

	return nil
}

// cmdText extracts text from all pages or a specific page using simple extraction.
func cmdText(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: folio text file.pdf [page_number]")
	}

	r, err := reader.Load(args[0])
	if err != nil {
		return err
	}

	// Specific page or all pages?
	startPage := 0
	endPage := r.PageCount()

	if len(args) >= 2 {
		pageNum, err := strconv.Atoi(args[1])
		if err != nil || pageNum < 1 || pageNum > r.PageCount() {
			return fmt.Errorf("invalid page number: %s (document has %d pages)", args[1], r.PageCount())
		}
		startPage = pageNum - 1
		endPage = pageNum
	}

	for i := startPage; i < endPage; i++ {
		page, err := r.Page(i)
		if err != nil {
			continue
		}
		text, err := page.ExtractText()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Page %d: %v\n", i+1, err)
			continue
		}
		if endPage-startPage > 1 {
			fmt.Printf("--- Page %d ---\n", i+1)
		}
		fmt.Println(strings.TrimSpace(text))
	}

	return nil
}

// cmdCreate generates a new PDF with optional title and text content.
func cmdCreate(args []string) error {
	output := ""
	title := ""
	text := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "-text":
			if i+1 < len(args) {
				text = args[i+1]
				i++
			}
		}
	}

	if output == "" {
		return fmt.Errorf("usage: folio create -o output.pdf [-title \"Title\"] [-text \"Content\"]")
	}

	doc := document.NewDocument(document.PageSizeLetter)
	if title != "" {
		doc.Info.Title = title
		doc.Add(layout.NewHeading(title, layout.H1))
	}
	if text != "" {
		doc.Add(layout.NewParagraph(text, font.Helvetica, 12))
	}

	if err := doc.Save(output); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", output)
	return nil
}

// cmdBlank creates a PDF with the specified number of empty pages and page size.
func cmdBlank(args []string) error {
	output := ""
	size := "letter"
	pages := 1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-size":
			if i+1 < len(args) {
				size = strings.ToLower(args[i+1])
				i++
			}
		case "-pages":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err == nil && n > 0 {
					pages = n
				}
				i++
			}
		}
	}

	if output == "" {
		return fmt.Errorf("usage: folio blank -o output.pdf [-size letter|a4] [-pages N]")
	}

	pageSize := document.PageSizeLetter
	switch size {
	case "a4":
		pageSize = document.PageSizeA4
	case "a3":
		pageSize = document.PageSizeA3
	case "legal":
		pageSize = document.PageSizeLegal
	case "tabloid":
		pageSize = document.PageSizeTabloid
	}

	doc := document.NewDocument(pageSize)
	for range pages {
		doc.AddPage()
	}

	if err := doc.Save(output); err != nil {
		return err
	}

	fmt.Printf("Created %s (%d %s pages)\n", output, pages, size)
	return nil
}

// cmdExtract extracts text from a PDF using a specified strategy (simple or location).
func cmdExtract(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: folio extract file.pdf [-page N] [-strategy simple|location]")
	}

	file := ""
	pageNum := -1
	strategy := "simple"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-page":
			if i+1 < len(args) {
				pageNum, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-strategy":
			if i+1 < len(args) {
				strategy = strings.ToLower(args[i+1])
				i++
			}
		default:
			if file == "" {
				file = args[i]
			}
		}
	}

	if file == "" {
		return fmt.Errorf("usage: folio extract file.pdf [-page N] [-strategy simple|location]")
	}

	r, err := reader.Load(file)
	if err != nil {
		return err
	}

	startPage := 0
	endPage := r.PageCount()
	if pageNum > 0 && pageNum <= r.PageCount() {
		startPage = pageNum - 1
		endPage = pageNum
	}

	for i := startPage; i < endPage; i++ {
		page, err := r.Page(i)
		if err != nil {
			continue
		}

		var s reader.ExtractionStrategy
		switch strategy {
		case "location":
			s = &reader.LocationStrategy{}
		default:
			s = &reader.SimpleStrategy{}
		}

		text, err := page.ExtractTextWithStrategy(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Page %d: %v\n", i+1, err)
			continue
		}

		if endPage-startPage > 1 {
			fmt.Printf("--- Page %d ---\n", i+1)
		}
		fmt.Println(strings.TrimSpace(text))
	}

	return nil
}

// cmdSign digitally signs a PDF using a PEM-encoded certificate and private key.
func cmdSign(args []string) error {
	certFile := ""
	keyFile := ""
	output := ""
	input := ""
	reason := ""
	location := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-cert":
			if i+1 < len(args) {
				certFile = args[i+1]
				i++
			}
		case "-key":
			if i+1 < len(args) {
				keyFile = args[i+1]
				i++
			}
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-reason":
			if i+1 < len(args) {
				reason = args[i+1]
				i++
			}
		case "-location":
			if i+1 < len(args) {
				location = args[i+1]
				i++
			}
		default:
			if input == "" {
				input = args[i]
			}
		}
	}

	if input == "" || certFile == "" || keyFile == "" {
		return fmt.Errorf("usage: folio sign -cert cert.pem -key key.pem [-o signed.pdf] input.pdf")
	}

	if output == "" {
		output = strings.TrimSuffix(input, ".pdf") + "_signed.pdf"
	}

	// Read PDF.
	pdfBytes, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}

	// Load certificate.
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("read cert %s: %w", certFile, err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("no PEM block found in %s", certFile)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse cert: %w", err)
	}

	// Load private key.
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("read key %s: %w", keyFile, err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("no PEM block found in %s", keyFile)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		// Try PKCS1 as fallback.
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return fmt.Errorf("parse key: %w", err)
		}
	}

	// Create signer.
	cryptoKey, ok := key.(crypto.Signer)
	if !ok {
		return fmt.Errorf("private key does not implement crypto.Signer")
	}
	signer, err := sign.NewLocalSigner(cryptoKey, []*x509.Certificate{cert})
	if err != nil {
		return fmt.Errorf("create signer: %w", err)
	}

	// Sign.
	opts := sign.Options{
		Signer:   signer,
		Reason:   reason,
		Location: location,
	}

	signed, err := sign.SignPDF(pdfBytes, opts)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	if err := os.WriteFile(output, signed, 0644); err != nil {
		return err
	}

	fmt.Printf("Signed %s → %s\n", input, output)
	return nil
}
