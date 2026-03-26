//go:build ignore

// Generates template.pdf — a simple invoice shell with static labels.
// Run: go run gen_template.go
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

func main() {
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()

	// Company header.
	p.AddText("ACME CORP", font.HelveticaBold, 22, 72, 730)
	p.AddText("123 Main St, New York, NY 10001", font.Helvetica, 9, 72, 714)

	// Invoice label.
	p.AddText("INVOICE", font.HelveticaBold, 16, 450, 730)

	// Field labels.
	p.AddText("Invoice #:", font.Helvetica, 10, 450, 700)
	p.AddText("Date:", font.Helvetica, 10, 450, 686)
	p.AddText("Bill To:", font.HelveticaBold, 10, 72, 660)
	p.AddText("Amount Due:", font.HelveticaBold, 12, 350, 200)

	if err := doc.Save("template.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created template.pdf")
}
