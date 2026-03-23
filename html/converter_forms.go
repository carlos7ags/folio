// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convertInput renders an <input> element as a visual representation.
func (c *converter) convertInput(n *html.Node, style computedStyle) []layout.Element {
	inputType := strings.ToLower(getAttr(n, "type"))
	if inputType == "" {
		inputType = "text"
	}

	switch inputType {
	case "hidden":
		return nil
	case "checkbox", "radio":
		return c.convertCheckboxRadio(n, style, inputType)
	case "submit", "reset", "button":
		return c.convertInputButton(n, style, inputType)
	default: // text, password, email, number, tel, url, search, date, etc.
		return c.convertInputText(n, style, inputType)
	}
}

// convertInputText renders a text-like input as a bordered box with value text.
func (c *converter) convertInputText(n *html.Node, style computedStyle, inputType string) []layout.Element {
	value := getAttr(n, "value")
	placeholder := getAttr(n, "placeholder")

	displayText := value
	textColor := style.Color
	if displayText == "" && placeholder != "" {
		displayText = placeholder
		textColor = layout.ColorGray
	}
	if displayText == "" {
		displayText = " " // ensure the box has content for sizing
	}
	if inputType == "password" && value != "" {
		displayText = strings.Repeat("●", len([]rune(value)))
	}

	f := resolveFont(style)
	p := layout.NewParagraph(displayText, f, style.FontSize)
	p.SetLeading(style.LineHeight)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 3, Right: 6, Bottom: 3, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)

	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	} else {
		div.SetBackground(layout.ColorWhite)
	}

	run := layout.TextRun{Text: displayText, Font: f, FontSize: style.FontSize, Color: textColor}
	_ = run // we used NewParagraph above

	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}

	return []layout.Element{div}
}

// convertCheckboxRadio renders a checkbox or radio button as a small box/circle with optional check.
func (c *converter) convertCheckboxRadio(n *html.Node, style computedStyle, inputType string) []layout.Element {
	checked := hasAttr(n, "checked")
	var symbol string
	if inputType == "checkbox" {
		if checked {
			symbol = "☑"
		} else {
			symbol = "☐"
		}
	} else { // radio
		if checked {
			symbol = "◉"
		} else {
			symbol = "○"
		}
	}

	f := resolveFont(style)
	p := layout.NewParagraph(symbol, f, style.FontSize)
	return []layout.Element{p}
}

// convertInputButton renders submit/reset/button inputs as a styled button box.
func (c *converter) convertInputButton(n *html.Node, style computedStyle, inputType string) []layout.Element {
	value := getAttr(n, "value")
	if value == "" {
		switch inputType {
		case "submit":
			value = "Submit"
		case "reset":
			value = "Reset"
		default:
			value = "Button"
		}
	}
	return c.buildButtonElement(value, style)
}

// convertButton renders a <button> element.
func (c *converter) convertButton(n *html.Node, style computedStyle) []layout.Element {
	text := collectText(n)
	if text == "" {
		text = "Button"
	}
	return c.buildButtonElement(text, style)
}

// buildButtonElement creates a styled button visual.
func (c *converter) buildButtonElement(text string, style computedStyle) []layout.Element {
	f := resolveFont(style)
	p := layout.NewParagraph(text, f, style.FontSize)
	p.SetAlign(layout.AlignCenter)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 4, Right: 12, Bottom: 4, Left: 12})
	div.SetBorderRadius(3)

	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	} else {
		div.SetBackground(layout.Gray(0.9))
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	} else {
		div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	}

	return []layout.Element{div}
}

// convertSelect renders a <select> as a dropdown-like box showing the selected option.
func (c *converter) convertSelect(n *html.Node, style computedStyle) []layout.Element {
	// Find the selected option, or use the first option.
	var selectedText string
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Option {
			text := collectText(child)
			if selectedText == "" {
				selectedText = text // first option as fallback
			}
			if hasAttr(child, "selected") {
				selectedText = text
				break
			}
		}
		// Handle <optgroup>.
		if child.Type == html.ElementNode && child.DataAtom == atom.Optgroup {
			for opt := child.FirstChild; opt != nil; opt = opt.NextSibling {
				if opt.Type == html.ElementNode && opt.DataAtom == atom.Option {
					text := collectText(opt)
					if selectedText == "" {
						selectedText = text
					}
					if hasAttr(opt, "selected") {
						selectedText = text
						break
					}
				}
			}
		}
	}
	if selectedText == "" {
		selectedText = " "
	}

	// Render as text + dropdown arrow in a bordered box.
	displayText := selectedText + " ▾"
	f := resolveFont(style)
	p := layout.NewParagraph(displayText, f, style.FontSize)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 3, Right: 6, Bottom: 3, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)
	div.SetBackground(layout.ColorWhite)

	return []layout.Element{div}
}

// convertTextarea renders a <textarea> as a multi-line bordered box.
func (c *converter) convertTextarea(n *html.Node, style computedStyle) []layout.Element {
	text := collectText(n)
	placeholder := getAttr(n, "placeholder")

	displayText := text
	textColor := style.Color
	if displayText == "" && placeholder != "" {
		displayText = placeholder
		textColor = layout.ColorGray
	}
	if displayText == "" {
		displayText = " \n \n " // empty textarea placeholder (3 lines)
	}

	f := resolveFont(style)
	run := layout.TextRun{Text: displayText, Font: f, FontSize: style.FontSize, Color: textColor}
	p := layout.NewStyledParagraph(run)
	p.SetLeading(style.LineHeight)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 4, Right: 6, Bottom: 4, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)
	div.SetBackground(layout.ColorWhite)

	return []layout.Element{div}
}

// convertFieldset renders a <fieldset> as a bordered container.
// <legend> children are rendered as a bold header paragraph.
func (c *converter) convertFieldset(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	div.SetPaddingAll(layout.Padding{Top: 8, Right: 8, Bottom: 8, Left: 8})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(3)

	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childStyle := c.computeElementStyle(child, style)
		if child.DataAtom == atom.Legend {
			text := collectText(child)
			if text != "" {
				f := resolveFont(childStyle)
				p := layout.NewParagraph(text, f, childStyle.FontSize)
				p.SetSpaceAfter(4)
				div.Add(p)
			}
		} else {
			elems := c.convertElement(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}

// hasAttr returns true if the node has the given attribute (regardless of value).
func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
