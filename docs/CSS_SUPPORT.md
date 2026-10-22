# Folio CSS support

> Auto-generated from `html/css_props.go`, `html/css.go`, `html/css_selectors.go`, and the function parsers in `html/`. Do not edit by hand.
> Run `go generate ./html/...` to regenerate after changing the registry.

Folio's HTML-to-PDF converter recognizes the CSS properties listed below.
Properties not in this document are silently ignored at render time.

## How to read this document

Each per-category table lists the property name, any alternative names
(aliases) accepted by the parser, the value forms that are recognized,
and any notes about parsing or interactions with other properties.

Value forms are written in CSS spec shorthand: `<length>` means a
length value (e.g. `12px`, `1em`, `0.5in`); `<color>` means any
supported color form (named, hex, rgb/rgba, hsl/hsla, cmyk); and so on.
See [Value-form glossary](#value-form-glossary) below for the full list.

If you don't see a property here, Folio's parser silently ignores it
at render time — there is no warning. Use
[`html.Options.StrictAssets`](../html) to escalate certain asset failures,
but unknown CSS properties are always silent.

## At a glance

| Category | Properties |
|---|---:|
| Typography | 23 |
| Color | 1 |
| Backgrounds | 6 |
| BoxModel | 17 |
| Borders | 17 |
| Layout | 12 |
| Flexbox | 11 |
| Grid | 16 |
| MultiColumn | 9 |
| Tables | 2 |
| Pagination | 5 |
| Lists | 2 |
| Effects | 12 |
| PDF | 6 |
| **Total** | **139** |

## Value-form glossary

Angle-bracket placeholders used in the per-property tables below.

| Placeholder | Meaning |
|---|---|
| `<length>` | A CSS length: `<number><unit>` where unit is `px`, `pt`, `em`, `rem`, `cm`, `mm`, or `in`. Examples: `12px`, `1.5em`, `0.5in`. Distinct from `<percentage>`, which is listed separately as an alternative in per-property tables. |
| `<percentage>` | A `<number>%`. Resolves against the containing context (line-height, parent dimension, etc.). |
| `<number>` | A unitless real number, e.g. `1.5`, `0.7`, `-2`. |
| `<integer>` | A whole number, e.g. `0`, `5`, `-1`. Range constraints (e.g. `<integer 1..6>`) are listed in the per-property table. |
| `<string>` | A quoted text literal, e.g. `"My Title"` or `'caption'`. |
| `<color>` | Any of: `<named>` (`red`, `transparent`), `<hex>` (`#abc`, `#aabbcc`), `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()`. Folio renders sRGB only — `oklch()` and `color-mix()` are not supported. |
| `<named>`, `<hex>` | Component forms of `<color>`: `<named>` is a CSS named color (`red`, `aliceblue`, etc.); `<hex>` is `#RGB`, `#RGBA`, `#RRGGBB`, or `#RRGGBBAA`. |
| `<line-width>` | A `<length>` or one of the keywords `thin`, `medium`, `thick`. Used in border/outline shorthands. |
| `<line-style>` | One of `solid`, `dashed`, `dotted`, `double`, `none`. |
| `<position>` | A 1- or 2-component position keyword/length. Examples: `center`, `top right`, `50% 25%`, `10px 20px`. Applies to `background-position`, `object-position`, `transform-origin`. |
| `<grid-line>` | A grid line reference: an integer (e.g. `2`), a `span` keyword (`span 3`), or a named line (rare; line names not yet supported). |
| `<track-list>` | A space-separated list of track sizes for `grid-template-columns`/`-rows`. Examples: `1fr 1fr`, `100px auto`, `repeat(3, 1fr)`. |
| `<track-size>` | A single grid track size: `<length>`, `<percentage>`, `<number>fr`, `auto`, `min-content`, `max-content`. |
| `<ratio>` | An aspect ratio expressed as `<number>/<number>` or a single `<number>`. Example: `16/9`. |
| `<gradient>` | `linear-gradient(...)`, `repeating-linear-gradient(...)`, `radial-gradient(...)`, or `repeating-radial-gradient(...)`. |
| `<transform-function>` | A CSS transform: `translate()`, `translateX()`/`Y()`, `rotate()`, `scale()`/`X()`/`Y()`, `skew()`/`X()`/`Y()`. |
| `<offset-x>`, `<offset-y>`, `<blur>`, `<spread>` | Component lengths in shadow shorthands (`box-shadow`, `text-shadow`). All are `<length>`; spread accepts negatives to inset the shadow. |
| `<identifier>` | A custom name, e.g. for `counter-reset` or `string-set`. |
| `<absolute-size>` | A CSS font absolute size keyword: `xx-small`, `x-small`, `small`, `medium`, `large`, `x-large`, `xx-large`. |
| `<relative-size>` | A CSS font relative size keyword: `smaller`, `larger`. Resolves relative to the parent's font size. |
| `<font-size>` | A `<length>` or `<percentage>` for font-size. Percentages resolve against the parent's font size. |
| `<font-weight>` | A `<number>` (100–900) or keyword: `normal` (400), `bold` (700), `lighter`, `bolder`. |
| `<font-family>` | A font family name, optionally quoted. Multiple families can be comma-separated. |
| `<line-height>` | A `<number>`, `<length>`, or `<percentage>` for line-height. Unitless values are multiplied by font-size. |
| `<column-gap>` | A `<length>` or `<percentage>` for column-gap/row-gap. `normal` uses the browser default (1em for multi-column). |
| `<flex-basis>` | A flex-basis value: `<length>`, `<percentage>`, or `auto`. |
| `<flex-shrink>` | A unitless `<number>` for flex-shrink (default 1). |
| `<integer 100..900>` | A font-weight integer in the range 100–900. |

**`calc()`, `min()`, `max()`, `clamp()`** are accepted everywhere a `<length>` or `<percentage>` is. The parser preserves them as single tokens through shorthand splitting.

## Box-alignment properties

`justify-content`, `align-items`, `align-self`, and `align-content` are
listed under Flexbox or Grid in the per-category tables for grouping,
but per CSS Box Alignment Level 3 they apply to BOTH flex and grid
containers. Folio honors them in either context.

Similarly, `gap` (and its alias `grid-gap`) is grouped under Grid but
also takes effect on flex containers as the gap between items.

## Typography

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `baseline-shift` | — | `super`, `sub`, `baseline`, `<length>`, `<percentage>` | CSS Inline Layout Module Level 3 §4.3 — percentages resolve against line-height. |
| `direction` | — | `ltr`, `rtl` | Interacts with unicode-bidi; together they control bidi paragraph direction. |
| `font` | — | `<font-style>? <font-weight>? <font-size>[/<line-height>]? <font-family>` | — |
| `font-family` | — | `<family-name>`, `<generic-family>` | — |
| `font-size` | — | `<absolute-size>`, `<relative-size>`, `<length>`, `<percentage>` | — |
| `font-style` | — | `normal`, `italic`, `oblique` | — |
| `font-weight` | — | `normal`, `bold`, `bolder`, `lighter`, `<integer 100..900>` | — |
| `hyphens` | `-webkit-hyphens` | `none`, `manual`, `auto` | — |
| `letter-spacing` | — | `<length>`, `normal` | — |
| `line-height` | — | `<number>`, `<length>`, `<percentage>`, `normal` | — |
| `text-align` | — | `left`, `right`, `center`, `justify`, `start`, `end` | — |
| `text-align-last` | — | `left`, `right`, `center`, `justify`, `start`, `end` | — |
| `text-decoration` | — | `none`, `underline`, `overline`, `line-through`, `blink` | — |
| `text-decoration-color` | — | `<color>` | — |
| `text-decoration-style` | — | `solid`, `dashed`, `dotted`, `double`, `wavy` | — |
| `text-indent` | — | `<length>`, `<percentage>` | — |
| `text-shadow` | — | `<offset-x> <offset-y> [<blur>] [<color>]`, `none` | — |
| `text-transform` | — | `uppercase`, `lowercase`, `capitalize`, `none` | — |
| `unicode-bidi` | — | `normal`, `embed`, `bidi-override`, `isolate`, `isolate-override`, `plaintext` | Interacts with direction; together they control bidi paragraph layout. |
| `vertical-align` | — | `top`, `middle`, `bottom`, `super`, `sub`, `baseline`, `text-top`, `text-bottom`, `<length>`, `<percentage>` | — |
| `white-space` | — | `normal`, `nowrap`, `pre`, `pre-wrap`, `pre-line` | — |
| `word-break` | — | `normal`, `break-all`, `keep-all`, `break-word` | — |
| `word-spacing` | — | `<length>`, `normal` | — |

## Color

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `color` | — | `<named>`, `<hex>`, `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()` | sRGB only. oklch() and color-mix() are not supported. |

## Backgrounds

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `background` | — | `<color>`, `<gradient>`, `url(...)` | Background shorthand: dispatches to BackgroundImage for gradient/url, BackgroundColor otherwise. |
| `background-color` | — | `<color>` | — |
| `background-image` | — | `<gradient>`, `url(...)`, `none` | — |
| `background-position` | — | `<position>` | — |
| `background-repeat` | — | `repeat`, `repeat-x`, `repeat-y`, `no-repeat`, `space`, `round` | — |
| `background-size` | — | `<length>`, `<percentage>`, `auto`, `cover`, `contain` | — |

## BoxModel

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `aspect-ratio` | — | `<ratio>`, `auto` | — |
| `height` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin` | — | `<length>`, `<percentage>`, `auto`, `<1-4 of these>` | auto keyword sets MarginTopAuto/LeftAuto/RightAuto per CSS shorthand position rules. |
| `margin-bottom` | — | `<length>`, `<percentage>` | margin-top, margin-left, margin-right also accept `auto`; margin-bottom does not. |
| `margin-left` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin-right` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin-top` | — | `<length>`, `<percentage>`, `auto` | — |
| `max-height` | — | `<length>`, `<percentage>`, `none` | — |
| `max-width` | — | `<length>`, `<percentage>`, `none` | — |
| `min-height` | — | `<length>`, `<percentage>` | — |
| `min-width` | — | `<length>`, `<percentage>` | — |
| `padding` | — | `<length>`, `<percentage>`, `<1-4 of these>` | — |
| `padding-bottom` | — | `<length>`, `<percentage>` | — |
| `padding-left` | — | `<length>`, `<percentage>` | — |
| `padding-right` | — | `<length>`, `<percentage>` | — |
| `padding-top` | — | `<length>`, `<percentage>` | — |
| `width` | — | `<length>`, `<percentage>`, `auto` | — |

## Borders

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `border` | — | `<line-width> <line-style> <color>` | Sets all 12 fields (4 sides × {width, style, color}) at once. |
| `border-bottom` | — | `<line-width> <line-style> <color>` | — |
| `border-bottom-left-radius` | — | `<length>`, `<percentage>` | — |
| `border-bottom-right-radius` | — | `<length>`, `<percentage>` | — |
| `border-bottom-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-color` | — | `<color>` | — |
| `border-left` | — | `<line-width> <line-style> <color>` | — |
| `border-left-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-radius` | — | `<length>`, `<percentage>`, `<1-4 of these>` | splitTopLevelFields preserves calc()/min()/max()/clamp() as single tokens. |
| `border-right` | — | `<line-width> <line-style> <color>` | — |
| `border-right-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-style` | — | `solid`, `dashed`, `dotted`, `double`, `none`, `hidden`, `groove`, `ridge`, `inset`, `outset` | groove/ridge/inset/outset are rendered as a single solid stroke per side with the spec's per-side dark/light color modulation, rather than the strict two-half-width split bevel. |
| `border-top` | — | `<line-width> <line-style> <color>` | — |
| `border-top-left-radius` | — | `<length>`, `<percentage>` | — |
| `border-top-right-radius` | — | `<length>`, `<percentage>` | — |
| `border-top-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-width` | — | `<line-width>` | — |

## Layout

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `bottom` | — | `<length>`, `<percentage>`, `auto` | — |
| `box-sizing` | — | `content-box`, `border-box` | — |
| `clear` | — | `left`, `right`, `both`, `none` | — |
| `display` | — | `block`, `inline`, `inline-block`, `flex`, `grid`, `table`, `table-row`, `table-cell`, `list-item`, `none` | — |
| `float` | — | `left`, `right`, `none` | — |
| `left` | — | `<length>`, `<percentage>`, `auto` | — |
| `overflow` | — | `hidden`, `visible`, `auto`, `scroll` | — |
| `position` | — | `static`, `relative`, `absolute`, `fixed` | — |
| `right` | — | `<length>`, `<percentage>`, `auto` | — |
| `top` | — | `<length>`, `<percentage>`, `auto` | — |
| `visibility` | — | `visible`, `hidden`, `collapse` | — |
| `z-index` | — | `<integer>`, `auto` | — |

## Flexbox

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `align-items` | — | `stretch`, `flex-start`, `flex-end`, `center`, `baseline`, `start`, `end` | — |
| `align-self` | — | `auto`, `stretch`, `flex-start`, `flex-end`, `center`, `baseline` | — |
| `flex` | — | `<flex-grow> <flex-shrink>? <flex-basis>?`, `none`, `auto` | — |
| `flex-basis` | — | `<length>`, `<percentage>`, `auto`, `content` | — |
| `flex-direction` | — | `row`, `row-reverse`, `column`, `column-reverse` | — |
| `flex-flow` | — | `<flex-direction> <flex-wrap>` | — |
| `flex-grow` | — | `<number>` | — |
| `flex-shrink` | — | `<number>` | — |
| `flex-wrap` | — | `nowrap`, `wrap`, `wrap-reverse` | — |
| `justify-content` | — | `flex-start`, `flex-end`, `center`, `space-between`, `space-around`, `space-evenly`, `start`, `end` | — |
| `order` | — | `<integer>` | — |

## Grid

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `align-content` | — | `normal`, `stretch`, `flex-start`, `flex-end`, `center`, `space-between`, `space-around`, `space-evenly` | — |
| `gap` | `grid-gap` | `<row-gap>`, `<row-gap> <column-gap>` | Sets RowGap and GridColumnGap; in flex contexts, also Gap. |
| `grid-area` | — | `<grid-line> [/ <grid-line>]{0..3}` | — |
| `grid-auto-flow` | — | `row`, `column`, `dense`, `row dense`, `column dense` | — |
| `grid-auto-rows` | — | `<track-size>` | — |
| `grid-column` | — | `<grid-line> [/ <grid-line>]?` | — |
| `grid-column-end` | — | `<integer>` | — |
| `grid-column-start` | — | `<integer>` | — |
| `grid-row` | — | `<grid-line> [/ <grid-line>]?` | — |
| `grid-row-end` | — | `<integer>` | — |
| `grid-row-start` | — | `<integer>` | — |
| `grid-template-areas` | — | `<string>+`, `none` | — |
| `grid-template-columns` | — | `<track-list>`, `none` | — |
| `grid-template-rows` | — | `<track-list>`, `none` | — |
| `justify-items` | — | `start`, `end`, `center`, `stretch` | — |
| `row-gap` | — | `<length>`, `<percentage>`, `normal` | — |

## MultiColumn

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `column-count` | — | `<integer>`, `auto` | — |
| `column-gap` | — | `<length>`, `normal` | — |
| `column-rule` | — | `<line-width> <line-style> <color>` | — |
| `column-rule-color` | — | `<color>` | — |
| `column-rule-style` | — | `solid`, `dashed`, `dotted`, `double`, `none` | — |
| `column-rule-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `column-span` | — | `none`, `all` | — |
| `column-width` | — | `<length>`, `auto` | — |
| `columns` | — | `<column-count>`, `<column-width>`, `<column-count> <column-width>` | — |

## Tables

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `border-collapse` | — | `collapse`, `separate` | — |
| `border-spacing` | — | `<length>`, `<length> <length>` | — |

## Pagination

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `orphans` | — | `<integer>` | — |
| `page-break-after` | `break-after` | `always`, `page`, `avoid`, `avoid-page`, `auto` | — |
| `page-break-before` | `break-before` | `always`, `page`, `avoid`, `avoid-page`, `auto` | — |
| `page-break-inside` | `break-inside` | `avoid`, `avoid-page`, `auto` | — |
| `widows` | — | `<integer>` | — |

## Lists

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `list-style-position` | — | `inside`, `outside` | — |
| `list-style-type` | `list-style` | `disc`, `circle`, `square`, `decimal`, `lower-alpha`, `upper-alpha`, `lower-roman`, `upper-roman`, `none` | list-style is a shorthand; type and position tokens are extracted. |

## Effects

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `box-shadow` | — | `<offset-x> <offset-y> [<blur>] [<spread>] [<color>] [inset]`, `none` | Supports comma-separated multiple shadows. |
| `object-fit` | — | `contain`, `cover`, `fill`, `none`, `scale-down` | — |
| `object-position` | — | `<position>` | — |
| `opacity` | — | `<number 0..1>` | Values outside 0..1 are clamped. |
| `outline` | — | `<line-width> <line-style> <color>` | — |
| `outline-color` | — | `<color>` | — |
| `outline-offset` | — | `<length>` | — |
| `outline-style` | — | `solid`, `dashed`, `dotted`, `double`, `none` | — |
| `outline-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `text-overflow` | — | `clip`, `ellipsis` | — |
| `transform` | — | `<transform-function>+`, `none` | — |
| `transform-origin` | — | `<position>` | — |

## PDF

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `bookmark-label` | — | `<string>`, `content()`, `attr(<identifier>)` | content() and attr() are resolved at element-conversion time. |
| `bookmark-level` | — | `<integer 1..6>`, `none` | Per CSS GCPM. Levels are clamped to Folio's H1-H6 range. |
| `bookmark-state` | — | `open`, `closed` | — |
| `counter-increment` | — | `<identifier> [<integer>]+` | — |
| `counter-reset` | — | `<identifier> [<integer>]+` | — |
| `string-set` | — | `<identifier> <content-list>` | Used by @page margin boxes for running headers/footers. |

## Selectors

CSS selectors recognized by Folio's stylesheet parser. Selectors not
listed here are silently dropped at parse time — the rule's declarations
never apply to any element.

### Combinators

| Combinator | Example | Meaning |
|---|---|---|
| descendant (space) | `article p` | `p` anywhere inside `article`. |
| `>` | `ul > li` | Direct child only. |
| `+` | `h2 + p` | Immediately-following sibling. |
| `~` | `h2 ~ p` | Any later sibling. |

### Simple selectors

| Selector | Example | Notes |
|---|---|---|
| Type | `p`, `h1` | Element name match. |
| Class | `.note` | Matches elements whose `class` attribute contains the name. Multiple classes can be chained: `.note.warning`. |
| ID | `#title` | Matches the element with the given `id`. |
| Universal | `*` | Matches every element. |
| Attribute | `[lang]`, `[lang="en"]` | See attribute operators below. |

Selectors compose: `article.featured > p.lead` matches a `p` with class `lead` that is a direct child of an `article` with class `featured`.

### Attribute operators

| Operator | Example | Matches when... |
|---|---|---|
| presence | `[hidden]` | Attribute is present (any value, including empty). |
| `=` | `[type="submit"]` | Attribute value equals the operand exactly. |
| `^=` | `[href^="https://"]` | Value starts with the operand. |
| `$=` | `[src$=".pdf"]` | Value ends with the operand. |
| `*=` | `[class*="btn"]` | Value contains the operand as a substring. |
| `~=` | `[rel~="author"]` | Value, treated as a whitespace-separated list, contains the operand as a whole word. |
| `|=` | `[lang|="en"]` | Value equals the operand or starts with `operand-`. |

Case-sensitivity flags (`[lang="EN" i]`) are not parsed.

### Pseudo-classes

| Pseudo-class | Notes |
|---|---|
| `:root` | The document root (`<html>`). |
| `:empty` | Element with no element children and no non-empty text nodes. |
| `:first-child` | First child of its parent. |
| `:last-child` | Last child of its parent. |
| `:nth-child(<expr>)` | Position match. `<expr>` accepts `odd`, `even`, an integer, or `An+B` form (e.g. `2n+1`, `3n`, `-n+3`). |
| `:nth-last-child(<expr>)` | Same as `:nth-child` but counted from the end. |
| `:first-of-type` | First element of its tag type among siblings. |
| `:last-of-type` | Last element of its tag type among siblings. |
| `:nth-of-type(<expr>)` | Position match restricted to the element's tag type. |
| `:nth-last-of-type(<expr>)` | Same, counted from the end. |
| `:not(<simple>)` | Negation. Argument is a single simple selector — selector lists inside `:not()` are not parsed. |
| `:is(<list>)` | Matches if any selector in the comma-separated list matches. Specificity follows the highest-specificity argument. |
| `:where(<list>)` | Same matching as `:is()` but contributes zero specificity. |

Interaction-state pseudo-classes (`:hover`, `:focus`, `:active`, `:visited`, `:target`, `:checked`, `:disabled`) are not supported — PDFs are static.

### Pseudo-elements

| Pseudo-element | Notes |
|---|---|
| `::before` | Inserts generated content before the element. Driven by the `content` declaration. |
| `::after` | Inserts generated content after the element. |
| `::marker` | Styles the list marker on `<li>` elements (`color`, `font-size`, etc.). |
| `::placeholder` | Styles the placeholder text on form fields. |

The double-colon form is required — single-colon legacy forms (`:before`, `:after`) are not recognized. `::first-letter`, `::first-line`, `::selection`, `::backdrop` are not supported.

## At-rules

CSS at-rules recognized by Folio's stylesheet parser. Anything not listed here
is silently dropped during parsing — there is no warning.

| Rule | Selectors / context | Notes |
|---|---|---|
| `@font-face` | — | Declares a custom font face. Recognized descriptors: `font-family`, `src`, `font-weight`, `font-style`. The `format()` annotation in `src` is advisory; Folio inspects the URL contents to determine format (WOFF1, TTF, TTC). WOFF2 is not supported. |
| `@page` | `:first`, `:left`, `:right`, no selector | Page-level styling: page size, margins, and nested margin boxes. Pseudo-selectors target the first page or left/right pages in a duplex flow. |
| `@page` margin boxes | `@top-left`, `@top-center`, `@top-right`, `@bottom-left`, `@bottom-center`, `@bottom-right` | Running headers/footers, declared inside an `@page` block. Populate via static `content`, `string()`, or `counter(page)`. The four corner boxes (`@top-left-corner`, etc.) and the `@left-*` / `@right-*` boxes are not interpreted. |
| `@supports` | `(<property>: <value>)`, `not (...)`, `and`, `or` | Feature query. Inner rules are parsed only if the condition evaluates true against Folio's actual support — useful for shipping fallbacks alongside Folio-specific styling. |
| `@media print` | — | Treated as unconditional (PDF is a print medium). Inner rules are parsed as if at the top level. Other `@media` queries are silently discarded; see below. |

### Silently ignored at-rules

Listed for evaluators migrating from a browser-based renderer. None of
these produce a warning — the rule and its body are dropped during parsing.

| Rule | Why |
|---|---|
| `@media screen`, `@media (max-width: ...)`, etc. | Only `@media print` is interpreted; PDF output has fixed page geometry, so viewport breakpoints have no analogue. |
| `@import` | External stylesheet imports are not followed during CSS parsing. Use `<link rel="stylesheet">` in the HTML instead — those are loaded through the asset resolver. |
| `@keyframes`, `@-webkit-keyframes` | PDF has no animation timeline. |
| `@counter-style` | Custom list counter styles are not parsed; only the keywords listed under `list-style-type` are recognized. |
| `@namespace`, `@charset` | Not interpreted. |
| `@layer`, `@scope`, `@container`, `@property` | Newer CSS spec features; not interpreted. |

## Functions

CSS functional values recognized by Folio's parsers, grouped by category.
Functions not listed here pass through as opaque text and almost always
cause the containing declaration to be discarded.

### Math

Accepted everywhere a `<length>` or `<percentage>` is expected.
Folio's parser preserves these as single tokens through shorthand splitting,
so they survive inside `margin`, `padding`, `flex`, `transform()`, etc.

| Function | Notes |
|---|---|
| `calc()` | Supports `+`, `-`, `*`, `/` with operator precedence and nested parentheses. Mixed units (e.g. `calc(100% - 20px)`) resolve at layout time. |
| `min()` | Comma-separated argument list. Returns the smallest resolved value. |
| `max()` | Comma-separated argument list. Returns the largest resolved value. |
| `clamp()` | `clamp(<min>, <preferred>, <max>)`. |

Known limitations: `calc()` does not yet expand inside `rotate()`, `scale()`, `skew()`, `background-position`, or `linear-gradient()` color stops — see issues #265, #266, #274, #275.

### Color

Accepted everywhere a `<color>` is expected. Output is sRGB regardless of input form.

| Function | Notes |
|---|---|
| `rgb()` | `rgb(R, G, B)` or `rgb(R G B)`. Components are 0-255 integers or 0-100% percentages. |
| `rgba()` | `rgba(R, G, B, A)`. Alpha is 0-1 or 0-100%. |
| `hsl()` | `hsl(H, S%, L%)`. Hue in degrees. |
| `hsla()` | `hsla(H, S%, L%, A)`. |
| `cmyk()` / `device-cmyk()` | `cmyk(C, M, Y, K)` with components as 0-1 or 0-100%. Folio converts to sRGB for raster compositing; the original CMYK is preserved in the PDF color space for print pipelines. |

Known unsupported color functions: `oklch()`, `oklab()`, `lch()`, `lab()`, `color-mix()`, `color()` — see [Known unsupported features](#known-unsupported-features) for workarounds.

### Gradients

Accepted as `background-image` values.

| Function | Notes |
|---|---|
| `linear-gradient()` | Direction (`to right`, `45deg`, etc.) plus 2+ `<color>` stops. |
| `repeating-linear-gradient()` | Same syntax; tiles the gradient pattern. |
| `radial-gradient()` | Shape (`circle`, `ellipse`), size, and `<color>` stops. |
| `repeating-radial-gradient()` | Same syntax; tiles the gradient pattern. |

`conic-gradient()` is not supported.

### Content and counters

Used in `string-set`, `bookmark-label`, `content`, and `@page` margin boxes.

| Function | Notes |
|---|---|
| `var()` | CSS custom property reference. Supports a fallback as the second argument: `var(--c, #000)`. Resolved BEFORE per-property dispatch, so functions and gradients receive resolved values. |
| `attr()` | Reads an HTML attribute. Used in `bookmark-label`. |
| `content()` | Substitutes the element's text content. Used in `string-set` and `bookmark-label`. |
| `counter()` | `counter(<name>)` or `counter(<name>, <list-style>)`. Page counter `counter(page)` is supported in `@page` margin boxes. |
| `counters()` | `counters(<name>, <separator>)` for nested counter chains. |
| `string()` | Reads the latest value of a named string set via `string-set` (used in running headers). |

Known unsupported: `target-counter()` for cross-references — tracked as #222.

### Transform

Used in `transform`. Multiple functions compose in the listed order.

| Function | Notes |
|---|---|
| `translate()` | `translate(<tx>)` or `translate(<tx>, <ty>)`. Lengths in any supported unit; bare numbers treated as px. |
| `translateX()` | Single `<length>` argument. |
| `translateY()` | Single `<length>` argument. |
| `rotate()` | Single `<angle>`: `deg`, `rad`, `grad`, `turn`, or bare number (degrees). |
| `scale()` | `scale(<s>)` (uniform) or `scale(<sx>, <sy>)`. |
| `scaleX()` | Single `<number>` argument. |
| `scaleY()` | Single `<number>` argument. |
| `skew()` | `skew(<ax>)` or `skew(<ax>, <ay>)`. |
| `skewX()` | Single `<angle>` argument. |
| `skewY()` | Single `<angle>` argument. |

Known unsupported: `matrix()`, `matrix3d()`, `translate3d()`, `rotate3d()`, `scale3d()`, `perspective()` — Folio renders 2D only.

### Other

| Function | Notes |
|---|---|
| `url()` | Used in `background-image`, `@font-face` `src`, and asset references. Resolves through Folio's asset loader (BaseFS or HTTP via Client, subject to `Options.URLPolicy`). |

## Known unsupported features

These properties / values are commonly requested but NOT recognized by Folio.
Folio silently ignores unknown property names, so a stylesheet that uses
any of these will render — just without the styling those declarations
would have applied in a browser.

| Feature | Why | Workaround |
|---|---|---|
| `oklch()`, `oklab()`, `lch()`, `lab()` color | Folio renders sRGB only; no ICC profile support. | Precompute the sRGB equivalent and use `#hex` or `rgb()`. |
| `color-mix()` | Folio's parser doesn't expand the function. | Precompute the mixed color, or assign it to a CSS variable: `--btn-tint: #c44;`. |
| `-webkit-line-clamp` / `line-clamp` | PDFs are paginated, not scrollable; the property has no analogue. | Truncate before HTML emission, or use `layout.Paragraph.SplitAfterLine` for first-N-lines-plus-appendix flows. |
| `text-wrap: pretty` / `text-wrap: balance` | Browser-only line-break heuristic; cosmetic. | Render without it. |
| `filter`, `backdrop-filter`, `mix-blend-mode` | PDF lacks an analogue for screen-compositing. | Pre-bake effects into images. |
| `:hover`, `:focus`, `:active` | PDF has no interaction state. | Style the static state directly. |
| Custom HTML elements / Web Components | Folio's HTML parser handles a fixed element set. | Pre-render to a known element (`<div>` / `<span>`) before passing to Folio. |
| `position: sticky` | Has no analogue in paginated layout. | Use `@page` running headers/footers via margin boxes. |
| ICC profiles for color management | Folio is sRGB-only. | Use sRGB-correct hex values; convert assets to sRGB before embedding. |

## Adding a new CSS property

1. Append a `cssProperty` entry to `cssProperties` in `html/css_props.go`.
   Required: `Name` and `Apply`. Recommended: `Category`, `Values`, `Notes`.
2. Run `go generate ./html/...` to regenerate this document.
3. Add at least one row to `TestCSSPropertyParitySnapshot` in
   `html/css_props_test.go` asserting the new property's behavior.
4. CI guards: `TestCSSDocsInSync` ensures the doc matches the registry,
   and `TestNoSwitchRegistryOverlap` ensures no legacy switch case is
   reintroduced for a registered property.
