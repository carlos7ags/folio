// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"errors"
	"fmt"
	"strings"
)

// This file defines the html package's error taxonomy. Callers that map
// folio failures onto a response status (e.g. an HTTP service rendering
// untrusted, tenant-authored templates) can branch with errors.As:
//
//   - *ParseError — the input HTML could not be parsed into a document.
//     A client/input fault.
//   - *AssetError — a referenced asset (image, font, stylesheet) failed to
//     load. The Category and Ref fields let the caller decide: a missing
//     local path or a policy denial is a client fault, while a remote
//     fetch timeout is transient/infrastructure. Inspect Unwrap for the
//     underlying cause (errors.Is against fs.ErrNotExist, ErrURLPolicyDenied,
//     net errors, etc.).
//   - *LimitError — conversion hit a configured resource ceiling
//     (Options.MaxElements / MaxDepth). A client/input fault: the input was
//     too large or too deeply nested to convert within the configured
//     budget. Conversion fails closed before exhausting memory or stack.
//
// Any error that is none of these should be treated as an internal folio
// fault. AssetError values surface only when Options.StrictAssets is set;
// otherwise asset failures are logged through Options.Logger and the
// conversion continues with degraded output.

// ParseError indicates the input HTML could not be parsed into a document
// tree. It wraps the underlying parser error.
//
// Folio's HTML parsing is lenient — malformed markup is recovered rather
// than rejected — so this is rare in practice. It exists to give callers a
// typed signal that the failure originated in the input, distinct from an
// internal fault. Test for it with errors.As(err, new(*html.ParseError)).
type ParseError struct {
	// Err is the underlying parser error.
	Err error
}

func (e *ParseError) Error() string {
	return "folio/html: parse: " + e.Err.Error()
}

// Unwrap returns the underlying parser error so errors.Is / errors.As can
// inspect the cause.
func (e *ParseError) Unwrap() error { return e.Err }

// AssetError indicates a referenced asset failed to load during conversion.
// It is the typed form of the failures collected when Options.StrictAssets
// is set; the joined error returned by Convert / ConvertFull contains one
// AssetError per failed reference, discoverable with errors.As.
//
// Category is a short label for the reference site ("image", "@font-face",
// "background-image", "stylesheet", "SVG image", "FallbackFontPath"). Ref
// is the offending URL or path when one is known. Err is the underlying
// cause, exposed via Unwrap so errors.Is keeps working (e.g. against
// fs.ErrNotExist or [ErrURLPolicyDenied]).
type AssetError struct {
	// Category labels the reference site, e.g. "image" or "@font-face".
	Category string
	// Ref is the offending src/href/url/path, or "" when not available.
	Ref string
	// Err is the underlying load failure.
	Err error

	// msg is the fully formatted prefix (category plus inlined attrs),
	// precomputed so Error never re-runs fmt verbs over user-controlled
	// attr values (a path may legitimately contain a % character).
	msg string
}

func (e *AssetError) Error() string {
	if e.Err == nil {
		return e.msg
	}
	return e.msg + ": " + e.Err.Error()
}

// Unwrap returns the underlying load failure so callers can errors.Is
// against fs.ErrNotExist, [ErrURLPolicyDenied], network errors, etc.
func (e *AssetError) Unwrap() error { return e.Err }

// refAttrKeys are the attr keys whose value is the offending reference.
// @font-face leads with "family", so we cannot simply take the first attr;
// we pick the first key that names a URL or path.
var refAttrKeys = map[string]bool{"src": true, "href": true, "url": true, "path": true}

// formatAssetError builds the *AssetError stored in strictErrs. Attrs are
// inlined into the message as space-separated key=value pairs so the joined
// error from errors.Join is self-describing without forcing callers to walk
// a structured tree. The message prefix is built as a plain string (not via
// fmt verbs) so user-controlled attr values containing % characters are not
// interpreted as format directives (e.g. a path like C:\Users\foo%bar.ttf
// must not corrupt the formatted output). An odd-length attrs slice records
// the unpaired key with !BADKEY= matching slog's convention; this is a
// programming error in the caller and never happens in code under our
// control. The returned value's Error() string is byte-identical to the
// historical fmt.Errorf("%s: %w", prefix, err) form.
func formatAssetError(category string, err error, attrs []any) error {
	var b strings.Builder
	b.WriteString("folio/html: ")
	b.WriteString(category)
	b.WriteString(" load failed")

	var ref string
	for i := 0; i < len(attrs); i += 2 {
		b.WriteString(" ")
		fmt.Fprintf(&b, "%v=", attrs[i])
		if i+1 < len(attrs) {
			fmt.Fprintf(&b, "%v", attrs[i+1])
			if key, ok := attrs[i].(string); ok && ref == "" && refAttrKeys[key] {
				ref = fmt.Sprintf("%v", attrs[i+1])
			}
		} else {
			b.WriteString("!BADKEY")
		}
	}

	return &AssetError{Category: category, Ref: ref, Err: err, msg: b.String()}
}

// ErrURLPolicyDenied wraps a URLPolicy rejection so that asset-loading
// code paths can distinguish a policy decision (the caller's intent)
// from a network or filesystem failure. When Options.StrictAssets is
// true, errors matching ErrURLPolicyDenied are still logged via
// Options.Logger but are not added to the joined return-error — the
// caller already received the signal they wired URLPolicy to produce.
// Use with errors.Is to test the cause.
var ErrURLPolicyDenied = errors.New("html: URL fetch blocked by URLPolicy")

// ErrRemoteFetchDisabled is returned when the document references an
// http(s) asset but Options.AllowRemoteFetch is false. Unlike an
// ErrURLPolicyDenied, it is reported normally — logged via Options.Logger
// and, under Options.StrictAssets, added to the joined return-error — so
// callers discover why the asset did not load. Use with errors.Is.
var ErrRemoteFetchDisabled = errors.New("html: remote fetch disabled (set Options.AllowRemoteFetch)")

// ErrAbsolutePathDenied is returned when the document references an
// absolute filesystem path but Options.AllowAbsolutePaths is false.
// Programmatic paths (Options.FallbackFontPath, system-font search) are
// not affected. Reported normally, like ErrRemoteFetchDisabled. Use with
// errors.Is.
var ErrAbsolutePathDenied = errors.New("html: absolute filesystem path denied (set Options.AllowAbsolutePaths)")

// ErrAssetBudgetExceeded is returned when a conversion's aggregate asset
// read size crosses Options.MaxTotalAssetBytes. The offending load and any
// later ones fail with it. Reported normally (logged, and surfaced under
// StrictAssets). Use with errors.Is.
var ErrAssetBudgetExceeded = errors.New("html: total asset byte budget exceeded")

// LimitKind identifies which resource ceiling a LimitError reports.
type LimitKind int

const (
	// LimitElements reports that Options.MaxElements was exceeded — the
	// input produced more layout elements than the configured budget.
	LimitElements LimitKind = iota
	// LimitDepth reports that Options.MaxDepth was exceeded — the input
	// nested elements more deeply than the configured budget.
	LimitDepth
)

func (k LimitKind) String() string {
	switch k {
	case LimitDepth:
		return "depth"
	default:
		return "elements"
	}
}

// LimitError indicates conversion was aborted because a configured resource
// ceiling (Options.MaxElements or Options.MaxDepth) was crossed. It is a
// terminal, input-side fault with no wrapped cause: the conversion stops and
// returns this error instead of continuing to allocate, so a very large or
// deeply-nested (e.g. programmatically-expanded) template cannot exhaust
// memory or the goroutine stack. Test for it with
// errors.As(err, new(*html.LimitError)).
type LimitError struct {
	// Kind is the ceiling that was crossed.
	Kind LimitKind
	// Limit is the configured maximum (the value of MaxElements or MaxDepth).
	Limit int
}

func (e *LimitError) Error() string {
	switch e.Kind {
	case LimitDepth:
		return fmt.Sprintf("folio/html: nesting depth exceeded limit of %d", e.Limit)
	default:
		return fmt.Sprintf("folio/html: element count exceeded limit of %d", e.Limit)
	}
}
