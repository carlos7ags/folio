// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package tmpl

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// executeHTML is a test helper that runs the template engine and returns
// the intermediate HTML string. This lets tests verify that template
// substitution actually happened, not just that elements were produced.
func executeHTML(t *testing.T, templateStr string, data any, opts *Options) string {
	t.Helper()
	html, err := execute(templateStr, "", data, opts)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return html
}

// --- Render tests ---

func TestRenderBasic(t *testing.T) {
	data := map[string]string{"Name": "Acme Corp"}
	html := executeHTML(t, `<p>Hello, {{.Name}}!</p>`, data, nil)
	if !strings.Contains(html, "Acme Corp") {
		t.Errorf("expected 'Acme Corp' in output, got: %s", html)
	}
	elems, err := Render(`<p>Hello, {{.Name}}!</p>`, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
}

func TestRenderMultipleFields(t *testing.T) {
	data := struct {
		Title    string
		Customer string
		Amount   float64
	}{"Invoice #1042", "Acme Corp", 1200.50}

	html := executeHTML(t, `
		<h1>{{.Title}}</h1>
		<p>Bill to: <strong>{{.Customer}}</strong></p>
		<p>Total: ${{printf "%.2f" .Amount}}</p>
	`, data, nil)
	for _, want := range []string{"Invoice #1042", "Acme Corp", "1200.50"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestRenderRange(t *testing.T) {
	data := struct{ Items []string }{Items: []string{"Widget A", "Widget B", "Widget C"}}
	html := executeHTML(t, `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`, data, nil)
	for _, item := range data.Items {
		if !strings.Contains(html, item) {
			t.Errorf("expected %q in output", item)
		}
	}
}

func TestRenderConditional(t *testing.T) {
	t.Run("true branch", func(t *testing.T) {
		html := executeHTML(t, `{{if .Paid}}<p>PAID</p>{{else}}<p>UNPAID</p>{{end}}`,
			struct{ Paid bool }{true}, nil)
		if !strings.Contains(html, "PAID") || strings.Contains(html, "UNPAID") {
			t.Errorf("expected PAID branch, got: %s", html)
		}
	})
	t.Run("false branch", func(t *testing.T) {
		html := executeHTML(t, `{{if .Paid}}<p>PAID</p>{{else}}<p>UNPAID</p>{{end}}`,
			struct{ Paid bool }{false}, nil)
		if !strings.Contains(html, "UNPAID") {
			t.Errorf("expected UNPAID branch, got: %s", html)
		}
	})
}

func TestRenderCustomFuncs(t *testing.T) {
	data := map[string]string{"Name": "acme"}
	opts := &Options{
		Funcs: htmltpl.FuncMap{"upper": strings.ToUpper},
	}
	html := executeHTML(t, `<p>{{upper .Name}}</p>`, data, opts)
	if !strings.Contains(html, "ACME") {
		t.Errorf("expected 'ACME' from upper func, got: %s", html)
	}
}

func TestRenderCustomFuncOverridesBuiltin(t *testing.T) {
	// User-provided "dict" should override the built-in.
	opts := &Options{
		Funcs: htmltpl.FuncMap{
			"dict": func() string { return "custom" },
		},
	}
	html := executeHTML(t, `<p>{{dict}}</p>`, map[string]string{}, opts)
	if !strings.Contains(html, "custom") {
		t.Errorf("expected 'custom' from overridden dict, got: %s", html)
	}
}

func TestRenderDictHelper(t *testing.T) {
	html := executeHTML(t, `{{$d := dict "a" "1" "b" "2"}}<p>{{$d.a}}-{{$d.b}}</p>`,
		map[string]string{}, nil)
	if !strings.Contains(html, "1-2") {
		t.Errorf("expected '1-2' from dict, got: %s", html)
	}
}

func TestRenderDictOddArgs(t *testing.T) {
	_, err := Render(`{{$d := dict "a" "1" "orphan"}}<p>ok</p>`, map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error for odd dict argument count")
	}
}

func TestRenderDictNonStringKey(t *testing.T) {
	_, err := Render(`{{$d := dict 42 "v"}}<p>ok</p>`, map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error for non-string dict key")
	}
}

// --- RenderDocument tests ---

func TestRenderDocument(t *testing.T) {
	data := map[string]string{"Title": "Hello"}
	doc, err := RenderDocument(`<h1>{{.Title}}</h1>`, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected non-nil document")
	}
}

func TestRenderDocumentCustomPageSize(t *testing.T) {
	m := layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72}
	doc, err := RenderDocument(`<p>ok</p>`, map[string]string{}, &Options{
		PageSize: document.PageSizeA4,
		Margins:  &m,
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected non-nil document")
	}
}

func TestRenderDocumentPageRulesOverrideOptions(t *testing.T) {
	// A template with @page { size: 200pt 300pt; } should override
	// the default page size even when Options.PageSize is set.
	tpl := `<html><head><style>@page { size: 200pt 300pt; }</style></head>
		<body><p>ok</p></body></html>`
	doc, err := RenderDocument(tpl, nil, &Options{
		PageSize: document.PageSizeLetter,
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected non-nil document")
	}
}

// --- RenderFile tests ---

func TestRenderFile(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.html")
	outPath := filepath.Join(dir, "test.pdf")

	if err := os.WriteFile(tmplPath, []byte(`
		<!DOCTYPE html>
		<html><body>
			<h1>{{.Title}}</h1>
			<p>Generated from a template file.</p>
		</body></html>
	`), 0644); err != nil {
		t.Fatal(err)
	}

	data := map[string]string{"Title": "File Template Test"}
	if err := RenderFile(tmplPath, data, nil, outPath); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 100 {
		t.Errorf("PDF too small: %d bytes", info.Size())
	}
}

func TestRenderFileDoesNotMutateOpts(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.html")
	outPath := filepath.Join(dir, "out.pdf")

	if err := os.WriteFile(tmplPath, []byte(`<p>{{.V}}</p>`), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &Options{}
	if err := RenderFile(tmplPath, map[string]string{"V": "test"}, opts, outPath); err != nil {
		t.Fatal(err)
	}
	// The caller's Options must NOT have been mutated.
	if opts.ConvertOpts != nil {
		t.Error("RenderFile mutated opts.ConvertOpts — should clone internally")
	}
}

func TestRenderFilePreservesExplicitBasePath(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.html")
	outPath := filepath.Join(dir, "out.pdf")

	if err := os.WriteFile(tmplPath, []byte(`<p>ok</p>`), 0644); err != nil {
		t.Fatal(err)
	}

	customBase := "/custom/base/path"
	opts := &Options{ConvertOpts: &foliohtml.Options{BasePath: customBase}}
	if err := RenderFile(tmplPath, map[string]string{}, opts, outPath); err != nil {
		t.Fatal(err)
	}
	// Pre-set BasePath must be preserved, not overwritten.
	if opts.ConvertOpts.BasePath != customBase {
		t.Errorf("BasePath mutated: got %q, want %q", opts.ConvertOpts.BasePath, customBase)
	}
}

func TestRenderFileNotFound(t *testing.T) {
	err := RenderFile("/nonexistent/path.html", nil, nil, "/tmp/out.pdf")
	if err == nil {
		t.Fatal("expected error for missing template file")
	}
	if !strings.Contains(err.Error(), "tmpl:") {
		t.Errorf("expected wrapped error with 'tmpl:' prefix, got: %v", err)
	}
}

func TestRenderFileUnwritableOutput(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "t.html")
	if err := os.WriteFile(tmplPath, []byte(`<p>ok</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	err := RenderFile(tmplPath, map[string]string{}, nil, "/nonexistent-dir/out.pdf")
	if err == nil {
		t.Fatal("expected error for non-writable output path")
	}
}

// --- RenderTo tests ---

func TestRenderTo(t *testing.T) {
	var buf bytes.Buffer
	err := RenderTo(&buf, `<p>{{.Msg}}</p>`, map[string]string{"Msg": "Hello PDF"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() < 100 {
		t.Errorf("PDF too small: %d bytes", buf.Len())
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("output does not start with %PDF- header")
	}
}

// --- Error handling tests ---

func TestRenderTemplateError(t *testing.T) {
	_, err := Render(`{{.Missing`, nil, nil)
	if err == nil {
		t.Fatal("expected template parse error")
	}
	if !strings.Contains(err.Error(), "tmpl:") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestRenderExecutionError(t *testing.T) {
	_, err := Render(`<p>{{.Explode}}</p>`, 42, nil)
	if err == nil {
		t.Fatal("expected template execution error")
	}
	if !strings.Contains(err.Error(), "tmpl:") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestRenderEmptyTemplate(t *testing.T) {
	elems, err := Render(``, map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 0 {
		t.Errorf("expected 0 elements from empty template, got %d", len(elems))
	}
}

func TestRenderNilOptions(t *testing.T) {
	elems, err := Render(`<p>ok</p>`, map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected element")
	}
}

func TestRenderHTMLEscaping(t *testing.T) {
	data := map[string]string{"Name": "<script>alert('xss')</script>"}
	html := executeHTML(t, `<p>{{.Name}}</p>`, data, nil)
	if strings.Contains(html, "<script>") {
		t.Error("raw <script> found in output — html/template should escape it")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped &lt;script&gt;, got: %s", html)
	}
}

func TestRenderBothFuncsAndConvertOpts(t *testing.T) {
	opts := &Options{
		Funcs:       htmltpl.FuncMap{"shout": strings.ToUpper},
		ConvertOpts: &foliohtml.Options{DefaultFontSize: 16},
	}
	elems, err := Render(`<p>{{shout .X}}</p>`, map[string]string{"X": "hello"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected element")
	}
}

// --- Concurrency test ---

func TestRenderConcurrent(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data := map[string]int{"I": i}
			elems, err := Render(`<p>{{.I}}</p>`, data, nil)
			if err != nil {
				errs <- err
				return
			}
			if len(elems) == 0 {
				errs <- fmt.Errorf("goroutine %d: no elements", i)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// --- Large output test ---

func TestRenderLargeOutput(t *testing.T) {
	items := make([]string, 200)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d", i)
	}
	data := map[string]any{"Items": items}
	elems, err := Render(`{{range .Items}}<p>{{.}}</p>{{end}}`, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 100 {
		t.Errorf("expected many elements from 200 items, got %d", len(elems))
	}
}

// --- Template define/template nesting ---

func TestRenderDefineAndTemplate(t *testing.T) {
	tpl := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}{{template "header" .}}<p>body</p>`
	html := executeHTML(t, tpl, map[string]string{"Title": "Invoice"}, nil)
	if !strings.Contains(html, "Invoice") {
		t.Errorf("expected 'Invoice' from nested template, got: %s", html)
	}
}

// --- Template partials via BaseTemplate ---

func TestRenderWithBaseTemplate(t *testing.T) {
	// Pre-parse shared partials into a base template tree.
	base := htmltpl.Must(htmltpl.New("base").Funcs(defaultFuncs()).Parse(
		`{{define "header"}}<header><h1>{{.Title}}</h1></header>{{end}}` +
			`{{define "footer"}}<footer>Page {{.Page}}</footer>{{end}}`))

	opts := &Options{BaseTemplate: base}
	data := map[string]any{"Title": "Invoice #1042", "Page": 1}
	html := executeHTML(t,
		`{{template "header" .}}<p>Body content</p>{{template "footer" .}}`,
		data, opts)

	for _, want := range []string{"Invoice #1042", "Body content", "Page 1"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in output, got: %s", want, html)
		}
	}
}

func TestRenderWithBaseTemplateDoesNotMutateBase(t *testing.T) {
	base := htmltpl.Must(htmltpl.New("base").Parse(
		`{{define "shared"}}<p>shared</p>{{end}}`))

	// First render adds a "page" define to the clone.
	opts := &Options{BaseTemplate: base}
	_, err := Render(
		`{{define "page"}}<p>page1</p>{{end}}{{template "shared" .}}{{template "page" .}}`,
		map[string]string{}, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Second render with a different "page" define should not see the
	// first render's definition — the base must be untouched.
	html := executeHTML(t,
		`{{define "page"}}<p>page2</p>{{end}}{{template "shared" .}}{{template "page" .}}`,
		map[string]string{}, opts)
	if !strings.Contains(html, "page2") {
		t.Errorf("expected 'page2' from fresh clone, got: %s", html)
	}
	if strings.Contains(html, "page1") {
		t.Error("base template was mutated by previous render — clone failed")
	}
}

func TestRenderWithBaseTemplateAndCustomFuncs(t *testing.T) {
	base := htmltpl.Must(htmltpl.New("base").Funcs(htmltpl.FuncMap{
		"shout": strings.ToUpper,
	}).Parse(`{{define "name"}}<b>{{shout .Name}}</b>{{end}}`))

	opts := &Options{BaseTemplate: base}
	html := executeHTML(t,
		`<p>Hello {{template "name" .}}</p>`,
		map[string]string{"Name": "acme"}, opts)
	if !strings.Contains(html, "ACME") {
		t.Errorf("expected 'ACME' from base template's shout func, got: %s", html)
	}
}

// --- RenderFileTo ---

func TestRenderFileTo(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.html")
	if err := os.WriteFile(tmplPath, []byte(`<h1>{{.Title}}</h1>`), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := RenderFileTo(&buf, tmplPath, map[string]string{"Title": "Hello"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() < 100 {
		t.Errorf("PDF too small: %d bytes", buf.Len())
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("output does not start with %PDF- header")
	}
}

func TestRenderFileToNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := RenderFileTo(&buf, "/nonexistent/path.html", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing template file")
	}
}

// --- Zero margins ---

func TestRenderDocumentZeroMargins(t *testing.T) {
	// A non-nil pointer to zero-valued Margins should produce a
	// full-bleed document, not fall back to 36pt defaults.
	zeroMargins := &layout.Margins{}
	opts := &Options{Margins: zeroMargins}
	m := opts.margins()
	if m.Top != 0 || m.Right != 0 || m.Bottom != 0 || m.Left != 0 {
		t.Errorf("expected zero margins, got %+v", m)
	}
}

func TestRenderDocumentNilMarginsUsesDefaults(t *testing.T) {
	opts := &Options{}
	m := opts.margins()
	if m.Top != 36 || m.Right != 36 || m.Bottom != 36 || m.Left != 36 {
		t.Errorf("expected 36pt default margins, got %+v", m)
	}
}

func TestRenderDocumentPartialMargins(t *testing.T) {
	// A Margins with only Top set should be used as-is (other fields
	// stay zero), not trigger the default.
	partial := &layout.Margins{Top: 10}
	opts := &Options{Margins: partial}
	m := opts.margins()
	if m.Top != 10 || m.Right != 0 || m.Bottom != 0 || m.Left != 0 {
		t.Errorf("expected {10,0,0,0}, got %+v", m)
	}
}
