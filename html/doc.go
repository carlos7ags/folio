// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package html converts HTML+CSS markup into layout elements that the
// [github.com/carlos7ags/folio/layout] package can render to PDF.
//
// The converter parses an HTML string (via golang.org/x/net/html),
// resolves inline and <style> CSS, and produces a tree of
// [layout.Element] values — Paragraphs, Divs, Tables, Lists, Images,
// and so on — ready to be fed to the layout engine.
//
// Supported HTML elements include headings (<h1>–<h6>), paragraphs,
// spans, divs, tables, lists, images, SVG, links, and line breaks.
// CSS features include selectors, the box model, flexbox, grid,
// floats, absolute positioning, @page rules with margin boxes,
// CSS counters, and web fonts via @font-face.
package html
