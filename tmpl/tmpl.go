// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package tmpl provides Go html/template integration for Folio.
//
// It bridges Go's standard html/template engine with Folio's HTML-to-PDF
// converter, letting you define PDF templates as HTML files with Go
// template directives ({{.Field}}, {{range}}, {{if}}, etc.) and render
// them to layout elements or complete PDF documents with a single call.
//
// This is the "third input mode" alongside Folio's Go layout API and
// raw HTML string input: templates live in separate files, designers
// can preview them in a browser, and the rendering pipeline is
// html/template → html.ConvertFull → layout.Element[].
//
// All functions in this package are safe for concurrent use.
// User-provided Funcs override built-in helpers with the same name.
//
// Quick start:
//
//	// Render a template string to layout elements:
//	data := map[string]any{"Title": "Invoice #1042", "Customer": "Acme"}
//	elems, err := tmpl.Render(`<h1>{{.Title}}</h1><p>Bill to: {{.Customer}}</p>`, data, nil)
//
//	// Render a template file to a complete PDF:
//	err := tmpl.RenderFile("invoice.html", data, nil, "invoice.pdf")
//
//	// Use custom template functions:
//	opts := &tmpl.Options{
//	    Funcs: template.FuncMap{"upper": strings.ToUpper},
//	}
//	elems, err := tmpl.Render(`<p>{{upper .Name}}</p>`, data, opts)
package tmpl

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// Options configures template rendering. All fields are optional.
type Options struct {
	// Funcs is an optional function map passed to html/template.
	// Use this to register custom helpers (formatting, i18n, etc.)
	// available inside {{call}} directives in the template.
	// User-provided funcs override built-in helpers with the same
	// name (e.g. a custom "dict" replaces the default one).
	Funcs htmltpl.FuncMap

	// BaseTemplate is an optional pre-parsed template tree that the
	// template string is parsed into. Use this for shared partials:
	//
	//   base := template.Must(template.ParseGlob("partials/*.html"))
	//   opts := &tmpl.Options{BaseTemplate: base}
	//   elems, _ := tmpl.Render(`{{template "header" .}}...`, data, opts)
	//
	// When set, the template string is parsed into a clone of
	// BaseTemplate (via t.Clone().Parse()), so all {{define}} blocks
	// and {{template}} calls in BaseTemplate are available. The
	// original BaseTemplate is never modified.
	BaseTemplate *htmltpl.Template

	// ConvertOpts is passed through to html.ConvertFull. Use it to
	// set page dimensions, default font size, base path for assets,
	// etc. RenderFile clones this before setting BasePath, so the
	// caller's original value is never mutated.
	ConvertOpts *foliohtml.Options

	// PageSize sets the page size for RenderFile / RenderDocument.
	// Defaults to US Letter (612x792 pt). Ignored by Render, which
	// returns raw elements without page context. When the template
	// contains @page CSS rules, the @page size takes precedence.
	PageSize document.PageSize

	// Margins sets the page margins for RenderFile / RenderDocument.
	// Defaults to 36pt (0.5in) on all sides. Ignored by Render.
	// When the template contains @page margin rules, those take
	// precedence. Use a non-nil pointer to explicitly set zero
	// margins: &layout.Margins{} produces a full-bleed document.
	Margins *layout.Margins
}

func (o *Options) funcs() htmltpl.FuncMap {
	if o != nil && o.Funcs != nil {
		return o.Funcs
	}
	return nil
}

func (o *Options) convertOpts() *foliohtml.Options {
	if o != nil && o.ConvertOpts != nil {
		return o.ConvertOpts
	}
	return nil
}

func (o *Options) pageSize() document.PageSize {
	if o != nil && o.PageSize.Width > 0 && o.PageSize.Height > 0 {
		return o.PageSize
	}
	return document.PageSizeLetter
}

func (o *Options) margins() layout.Margins {
	if o != nil && o.Margins != nil {
		return *o.Margins
	}
	return layout.Margins{Top: 36, Right: 36, Bottom: 36, Left: 36}
}

func (o *Options) baseTemplate() *htmltpl.Template {
	if o != nil {
		return o.BaseTemplate
	}
	return nil
}

// cloneConvertOpts returns a shallow copy of the Options' ConvertOpts
// (or a fresh instance if nil). This prevents RenderFile from mutating
// the caller's original Options when setting BasePath.
func (o *Options) cloneConvertOpts() *foliohtml.Options {
	if o == nil || o.ConvertOpts == nil {
		return &foliohtml.Options{}
	}
	cp := *o.ConvertOpts
	return &cp
}

// Render executes a Go html/template string with the given data and
// converts the resulting HTML to Folio layout elements. This is the
// lowest-level entry point — use RenderDocument or RenderFile if you
// want a complete PDF.
func Render(templateStr string, data any, opts *Options) ([]layout.Element, error) {
	htmlStr, err := execute(templateStr, "", data, opts)
	if err != nil {
		return nil, err
	}
	result, err := foliohtml.ConvertFull(htmlStr, opts.convertOpts())
	if err != nil {
		return nil, fmt.Errorf("tmpl: html conversion failed: %w", err)
	}
	return result.Elements, nil
}

// RenderDocument executes a template string and returns a fully laid-out
// Document ready for Save(). The caller can add headers, footers, or
// additional elements before saving.
//
// When the template contains @page CSS rules (size, margins, margin
// boxes), they are applied to the document automatically.
func RenderDocument(templateStr string, data any, opts *Options) (*document.Document, error) {
	htmlStr, err := execute(templateStr, "", data, opts)
	if err != nil {
		return nil, err
	}
	result, err := foliohtml.ConvertFull(htmlStr, opts.convertOpts())
	if err != nil {
		return nil, fmt.Errorf("tmpl: html conversion failed: %w", err)
	}
	return buildDocumentFromResult(result, opts), nil
}

// RenderFile reads a template from disk, executes it with data, and
// writes the resulting PDF to outPath. The template's directory is used
// as the base path for resolving relative asset references (images,
// fonts, stylesheets) in the HTML unless ConvertOpts.BasePath is
// already set.
//
// RenderFile never mutates the caller's Options — it clones
// ConvertOpts internally before setting BasePath.
func RenderFile(templatePath string, data any, opts *Options, outPath string) error {
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("tmpl: read template %q: %w", templatePath, err)
	}

	// Clone ConvertOpts so we don't mutate the caller's struct.
	convOpts := opts.cloneConvertOpts()
	if convOpts.BasePath == "" {
		convOpts.BasePath = filepath.Dir(templatePath)
	}

	htmlStr, err := execute(string(tmplBytes), filepath.Base(templatePath), data, opts)
	if err != nil {
		return err
	}

	result, err := foliohtml.ConvertFull(htmlStr, convOpts)
	if err != nil {
		return fmt.Errorf("tmpl: html conversion failed: %w", err)
	}

	doc := buildDocumentFromResult(result, opts)
	if err := doc.Save(outPath); err != nil {
		return fmt.Errorf("tmpl: save %q: %w", outPath, err)
	}
	return nil
}

// RenderTo executes a template string and writes the resulting PDF to w.
func RenderTo(w io.Writer, templateStr string, data any, opts *Options) error {
	doc, err := RenderDocument(templateStr, data, opts)
	if err != nil {
		return err
	}
	_, err = doc.WriteTo(w)
	if err != nil {
		return fmt.Errorf("tmpl: write pdf: %w", err)
	}
	return nil
}

// RenderFileTo reads a template from disk, executes it with data, and
// writes the resulting PDF to w. This is the natural entry point for
// HTTP handlers that serve PDFs directly to the response writer.
//
// Like RenderFile, the template's directory is used as BasePath for
// asset resolution unless ConvertOpts.BasePath is already set.
// The caller's Options are never mutated.
func RenderFileTo(w io.Writer, templatePath string, data any, opts *Options) error {
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("tmpl: read template %q: %w", templatePath, err)
	}

	convOpts := opts.cloneConvertOpts()
	if convOpts.BasePath == "" {
		convOpts.BasePath = filepath.Dir(templatePath)
	}

	htmlStr, err := execute(string(tmplBytes), filepath.Base(templatePath), data, opts)
	if err != nil {
		return err
	}

	result, err := foliohtml.ConvertFull(htmlStr, convOpts)
	if err != nil {
		return fmt.Errorf("tmpl: html conversion failed: %w", err)
	}

	doc := buildDocumentFromResult(result, opts)
	_, err = doc.WriteTo(w)
	if err != nil {
		return fmt.Errorf("tmpl: write pdf: %w", err)
	}
	return nil
}

// execute runs the Go html/template engine on the input string.
// If opts.BaseTemplate is set, the template string is parsed into a
// clone of it so that shared partials ({{define}}/{{template}}) are
// available. The original BaseTemplate is never modified.
func execute(templateStr, name string, data any, opts *Options) (string, error) {
	if name == "" {
		name = "folio"
	}

	var t *htmltpl.Template
	if base := opts.baseTemplate(); base != nil {
		// Clone the base so we don't modify the caller's template
		// tree. Parse the template string into the clone, making all
		// of the base's {{define}} blocks available via {{template}}.
		clone, err := base.Clone()
		if err != nil {
			return "", fmt.Errorf("tmpl: clone base template: %w", err)
		}
		clone.Funcs(defaultFuncs())
		if fns := opts.funcs(); fns != nil {
			clone.Funcs(fns)
		}
		t, err = clone.New(name).Parse(templateStr)
		if err != nil {
			return "", fmt.Errorf("tmpl: parse template %q: %w", name, err)
		}
	} else {
		t = htmltpl.New(name).Funcs(defaultFuncs())
		if fns := opts.funcs(); fns != nil {
			t.Funcs(fns)
		}
		var err error
		t, err = t.Parse(templateStr)
		if err != nil {
			return "", fmt.Errorf("tmpl: parse template %q: %w", name, err)
		}
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("tmpl: execute template %q: %w", name, err)
	}
	return buf.String(), nil
}

// buildDocumentFromResult creates a Document from a ConvertResult,
// applying @page config (size, margins, margin boxes) when present.
func buildDocumentFromResult(result *foliohtml.ConvertResult, opts *Options) *document.Document {
	ps := opts.pageSize()
	margins := opts.margins()

	// @page rules from the template override Options.
	if pc := result.PageConfig; pc != nil {
		if pc.Width > 0 && pc.Height > 0 {
			ps = document.PageSize{Width: pc.Width, Height: pc.Height}
		}
		if pc.HasMargins {
			margins = layout.Margins{
				Top:    pc.MarginTop,
				Right:  pc.MarginRight,
				Bottom: pc.MarginBottom,
				Left:   pc.MarginLeft,
			}
		}
	}

	doc := document.NewDocument(ps)
	doc.SetMargins(margins)

	// Apply @page margin boxes (e.g. page numbers via @bottom-center).
	if len(result.MarginBoxes) > 0 {
		doc.SetMarginBoxes(result.MarginBoxes)
	}
	if len(result.FirstMarginBoxes) > 0 {
		doc.SetFirstMarginBoxes(result.FirstMarginBoxes)
	}

	// Apply @page :first / :left / :right margin overrides.
	if pc := result.PageConfig; pc != nil {
		if pc.First != nil && pc.First.HasMargins {
			doc.SetFirstMargins(layout.Margins{
				Top: pc.First.Top, Right: pc.First.Right,
				Bottom: pc.First.Bottom, Left: pc.First.Left,
			})
		}
	}

	// Apply document metadata from <title> and <meta> tags.
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}
	if result.Metadata.Subject != "" {
		doc.Info.Subject = result.Metadata.Subject
	}
	if result.Metadata.Keywords != "" {
		doc.Info.Keywords = result.Metadata.Keywords
	}

	for _, e := range result.Elements {
		doc.Add(e)
	}

	// Add absolutely positioned elements.
	for _, abs := range result.Absolutes {
		if abs.RightAligned {
			doc.AddAbsoluteRight(abs.Element, abs.X, abs.Y, abs.Width)
		} else {
			doc.AddAbsolute(abs.Element, abs.X, abs.Y, abs.Width)
		}
	}

	return doc
}

// defaultFuncs returns built-in template helpers.
func defaultFuncs() htmltpl.FuncMap {
	return htmltpl.FuncMap{
		// dict builds a map from alternating key-value pairs, useful for
		// passing multiple values to a nested template:
		//   {{template "header" dict "title" .Title "date" .Date}}
		//
		// Returns an error if called with an odd number of arguments or
		// if any key is not a string. html/template surfaces the error
		// as a template execution failure.
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments (%d)", len(pairs))
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				k, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key at position %d is %T, want string", i, pairs[i])
				}
				m[k] = pairs[i+1]
			}
			return m, nil
		},
	}
}
