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
// # Supported HTML Elements
//
// Block: div, p, section, article, main, header, footer, nav, aside,
// blockquote, pre, figure, figcaption, h1–h6, hr, dl/dt/dd,
// fieldset, legend, details, summary.
//
// Inline: span, em, strong, b, i, u, s, del, mark, small, sub, sup,
// code, a, br, abbr, cite, dfn, kbd, q, samp, var.
//
// Lists: ul, ol, li (nested, ordered/unordered).
//
// Tables: table, thead, tbody, tfoot, tr, td, th, caption, colgroup, col.
//
// Media: img (JPEG, PNG, WebP, GIF, SVG, data URIs), svg (inline).
//
// Forms: input, button, select, textarea, label (visual only, not interactive).
//
// # Supported CSS Properties
//
// Text: font-family, font-size, font-weight, font-style, color,
// text-align, text-align-last, text-decoration (underline, line-through),
// text-decoration-color, text-decoration-style, text-transform,
// text-indent, text-shadow, text-overflow, letter-spacing, word-spacing,
// word-break, hyphens, white-space, line-height.
//
// Box model: margin (shorthand + individual sides, auto for centering),
// padding (shorthand + individual), border (width, style, color per side),
// border-radius (uniform + per-corner), border-collapse, border-spacing,
// box-sizing, width, height, min-width, max-width, min-height, max-height.
//
// Background: background-color, background-image (url, linear-gradient,
// radial-gradient, repeating variants), background-size, background-position,
// background-repeat.
//
// Layout: display (block, inline, flex, grid, table, none),
// float, clear, position (static, relative, absolute), top/left/right/bottom,
// z-index, overflow, visibility, opacity.
//
// Flexbox: flex-direction, flex-wrap, justify-content, align-items,
// align-self, align-content, flex-grow, flex-shrink, flex-basis, gap.
//
// Grid: grid-template-columns, grid-template-rows, grid-template-areas,
// grid-column-start/end, grid-row-start/end, grid-auto-flow, grid-auto-rows,
// row-gap, column-gap, gap.
//
// Multi-column: column-count, column-width, column-gap, column-rule
// (width, style, color).
//
// Lists: list-style-type, ::marker (color, font-size).
//
// Visual effects: box-shadow (multiple), outline (width, style, color, offset),
// transform (rotate, scale, translate, skew, matrix), transform-origin,
// object-fit, object-position.
//
// Paged media: @page (size, margins, :first/:left/:right), margin boxes
// (@top-center, @bottom-left, etc.) with counter(page)/counter(pages)/string(),
// page-break-before, page-break-after, page-break-inside (avoid),
// orphans, widows, string-set, bookmark-level, bookmark-label.
//
// Colors: named (148 CSS keywords), #RGB, #RRGGBB, #RGBA, #RRGGBBAA,
// rgb(), rgba(), hsl(), hsla(), cmyk(), device-cmyk().
//
// Values: px, pt, em, rem, %, mm, cm, in, calc(), min(), max(), clamp().
//
// @-rules: @page, @font-face, @media print, @supports.
//
// Selectors: type, .class, #id, *, descendant, child (>), adjacent (+),
// general sibling (~), [attr], [attr=val], [attr^=], [attr$=], [attr*=],
// [attr~=], [attr|=], :first-child, :last-child, :nth-child(), :nth-of-type(),
// :nth-last-child(), :nth-last-of-type(), :first-of-type, :last-of-type,
// :root, :not(), :is(), :where(), ::before, ::after, ::marker.
//
// CSS custom properties (variables) via var() with fallback values.
//
// CSS counters via counter-reset, counter-increment, counter().
//
// # Not Supported
//
// Interactive pseudo-classes (:hover, :focus, :active) — not applicable
// to static PDF. @media queries beyond print. Vendor prefixes.
// @import (use inline <style> or Options.ExternalCSS). :has() selector.
// CSS animations, transitions. clip-path, mask.
// Vertical writing modes. ::first-line, ::first-letter pseudo-elements.
package html
