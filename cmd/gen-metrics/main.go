// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// gen-metrics parses Adobe AFM files and generates Go source for font width
// and kerning tables. The AFM files are public domain, published by Adobe.
//
// Usage:
//
//	go run ./cmd/gen-metrics > /dev/null       # dry run
//	go run ./cmd/gen-metrics -write            # overwrite font/widths.go and font/metrics_data.go
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// glyphToRune maps Adobe glyph names to Unicode code points.
// Source: Adobe Glyph List for New Fonts (AGLFN), public domain.
var glyphToRune = map[string]rune{
	"space": ' ', "exclam": '!', "quotedbl": '"', "numbersign": '#',
	"dollar": '$', "percent": '%', "ampersand": '&', "quotesingle": 0x0027,
	"parenleft": '(', "parenright": ')', "asterisk": '*', "plus": '+',
	"comma": ',', "hyphen": '-', "period": '.', "slash": '/',
	"zero": '0', "one": '1', "two": '2', "three": '3', "four": '4',
	"five": '5', "six": '6', "seven": '7', "eight": '8', "nine": '9',
	"colon": ':', "semicolon": ';', "less": '<', "equal": '=',
	"greater": '>', "question": '?', "at": '@',
	"A": 'A', "B": 'B', "C": 'C', "D": 'D', "E": 'E', "F": 'F',
	"G": 'G', "H": 'H', "I": 'I', "J": 'J', "K": 'K', "L": 'L',
	"M": 'M', "N": 'N', "O": 'O', "P": 'P', "Q": 'Q', "R": 'R',
	"S": 'S', "T": 'T', "U": 'U', "V": 'V', "W": 'W', "X": 'X',
	"Y": 'Y', "Z": 'Z',
	"bracketleft": '[', "backslash": '\\', "bracketright": ']',
	"asciicircum": '^', "underscore": '_', "grave": 0x0060,
	"a": 'a', "b": 'b', "c": 'c', "d": 'd', "e": 'e', "f": 'f',
	"g": 'g', "h": 'h', "i": 'i', "j": 'j', "k": 'k', "l": 'l',
	"m": 'm', "n": 'n', "o": 'o', "p": 'p', "q": 'q', "r": 'r',
	"s": 's', "t": 't', "u": 'u', "v": 'v', "w": 'w', "x": 'x',
	"y": 'y', "z": 'z',
	"braceleft": '{', "bar": '|', "braceright": '}', "asciitilde": '~',
	// Latin-1 Supplement (U+00A0–U+00FF)
	"nbspace": 0x00A0, "exclamdown": 0x00A1, "cent": 0x00A2,
	"sterling": 0x00A3, "currency": 0x00A4, "yen": 0x00A5,
	"brokenbar": 0x00A6, "section": 0x00A7, "dieresis": 0x00A8,
	"copyright": 0x00A9, "ordfeminine": 0x00AA, "guillemotleft": 0x00AB,
	"logicalnot": 0x00AC, "softhyphen": 0x00AD, "registered": 0x00AE,
	"macron": 0x00AF, "degree": 0x00B0, "plusminus": 0x00B1,
	"twosuperior": 0x00B2, "threesuperior": 0x00B3, "acute": 0x00B4,
	"mu": 0x00B5, "paragraph": 0x00B6, "periodcentered": 0x00B7,
	"cedilla": 0x00B8, "onesuperior": 0x00B9, "ordmasculine": 0x00BA,
	"guillemotright": 0x00BB, "onequarter": 0x00BC, "onehalf": 0x00BD,
	"threequarters": 0x00BE, "questiondown": 0x00BF,
	"Agrave": 0x00C0, "Aacute": 0x00C1, "Acircumflex": 0x00C2,
	"Atilde": 0x00C3, "Adieresis": 0x00C4, "Aring": 0x00C5,
	"AE": 0x00C6, "Ccedilla": 0x00C7, "Egrave": 0x00C8,
	"Eacute": 0x00C9, "Ecircumflex": 0x00CA, "Edieresis": 0x00CB,
	"Igrave": 0x00CC, "Iacute": 0x00CD, "Icircumflex": 0x00CE,
	"Idieresis": 0x00CF, "Eth": 0x00D0, "Ntilde": 0x00D1,
	"Ograve": 0x00D2, "Oacute": 0x00D3, "Ocircumflex": 0x00D4,
	"Otilde": 0x00D5, "Odieresis": 0x00D6, "multiply": 0x00D7,
	"Oslash": 0x00D8, "Ugrave": 0x00D9, "Uacute": 0x00DA,
	"Ucircumflex": 0x00DB, "Udieresis": 0x00DC, "Yacute": 0x00DD,
	"Thorn": 0x00DE, "germandbls": 0x00DF,
	"agrave": 0x00E0, "aacute": 0x00E1, "acircumflex": 0x00E2,
	"atilde": 0x00E3, "adieresis": 0x00E4, "aring": 0x00E5,
	"ae": 0x00E6, "ccedilla": 0x00E7, "egrave": 0x00E8,
	"eacute": 0x00E9, "ecircumflex": 0x00EA, "edieresis": 0x00EB,
	"igrave": 0x00EC, "iacute": 0x00ED, "icircumflex": 0x00EE,
	"idieresis": 0x00EF, "eth": 0x00F0, "ntilde": 0x00F1,
	"ograve": 0x00F2, "oacute": 0x00F3, "ocircumflex": 0x00F4,
	"otilde": 0x00F5, "odieresis": 0x00F6, "divide": 0x00F7,
	"oslash": 0x00F8, "ugrave": 0x00F9, "uacute": 0x00FA,
	"ucircumflex": 0x00FB, "udieresis": 0x00FC, "yacute": 0x00FD,
	"thorn": 0x00FE, "ydieresis": 0x00FF,
	// Extended Latin
	"Lslash": 0x0141, "lslash": 0x0142, "Scaron": 0x0160, "scaron": 0x0161,
	"Zcaron": 0x017D, "zcaron": 0x017E, "OE": 0x0152, "oe": 0x0153,
	"Ydieresis": 0x0178, "dotlessi": 0x0131, "florin": 0x0192,
	// Typographic symbols
	"endash": 0x2013, "emdash": 0x2014,
	"quoteleft": 0x2018, "quoteright": 0x2019,
	"quotesinglbase": 0x201A, "quotedblleft": 0x201C,
	"quotedblright": 0x201D, "quotedblbase": 0x201E,
	"dagger": 0x2020, "daggerdbl": 0x2021, "bullet": 0x2022,
	"ellipsis": 0x2026, "perthousand": 0x2030,
	"guilsinglleft": 0x2039, "guilsinglright": 0x203A,
	"trademark": 0x2122, "minus": 0x2212,
	"Euro": 0x20AC,
	// Ligatures
	"fi": 0xFB01, "fl": 0xFB02,
	// Additional symbols used in AFM files
	"fraction": 0x2044, "circumflex": 0x02C6, "tilde": 0x02DC,
	"caron": 0x02C7, "breve": 0x02D8, "dotaccent": 0x02D9,
	"ring": 0x02DA, "ogonek": 0x02DB, "hungarumlaut": 0x02DD,
}

type fontData struct {
	name    string
	varName string
	widths  map[rune]int
	kerns   map[[2]rune]int
}

func parseAFM(path string) (*fontData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fd := &fontData{
		widths: make(map[rune]int),
		kerns:  make(map[[2]rune]int),
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "FontName ") {
			fd.name = strings.TrimPrefix(line, "FontName ")
			continue
		}

		// CharMetrics: C <code> ; WX <width> ; N <name> ; B ...
		if strings.HasPrefix(line, "C ") {
			parts := strings.Split(line, ";")
			var width int
			var name string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "WX ") {
					w, err := strconv.Atoi(strings.TrimPrefix(p, "WX "))
					if err == nil {
						width = w
					}
				}
				if strings.HasPrefix(p, "N ") {
					name = strings.TrimPrefix(p, "N ")
				}
			}
			if name != "" && width > 0 {
				if r, ok := glyphToRune[name]; ok {
					fd.widths[r] = width
				}
			}
			continue
		}

		// KPX <left> <right> <value>
		if strings.HasPrefix(line, "KPX ") {
			fields := strings.Fields(line)
			if len(fields) != 4 {
				continue
			}
			leftName := fields[1]
			rightName := fields[2]
			val, err := strconv.Atoi(fields[3])
			if err != nil {
				continue
			}
			leftRune, lok := glyphToRune[leftName]
			rightRune, rok := glyphToRune[rightName]
			if lok && rok && val != 0 {
				fd.kerns[[2]rune{leftRune, rightRune}] = val
			}
			continue
		}
	}
	return fd, scanner.Err()
}

func runeGoLit(r rune) string {
	if r >= 0x20 && r <= 0x7E {
		switch r {
		case '\'':
			return "'\\''"
		case '\\':
			return "'\\\\'"
		default:
			return fmt.Sprintf("'%c'", r)
		}
	}
	return fmt.Sprintf("'\\u%04X'", r)
}

type runeWidth struct {
	r rune
	w int
}

func writeWidthsFile(fonts []*fontData) string {
	var b strings.Builder
	b.WriteString(`// Code generated by cmd/gen-metrics; DO NOT EDIT.
// Source: Adobe Font Metrics (AFM) files — public domain.

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

`)

	// Courier first (monospaced, hardcoded)
	b.WriteString("// courierWidths — all glyphs are 600 units wide (monospaced).\n")
	b.WriteString("var courierWidths = map[rune]int{\n\t0: 600, // default\n}\n\n")

	for _, fd := range fonts {
		// Sort widths by rune value for stable output
		var entries []runeWidth
		for r, w := range fd.widths {
			entries = append(entries, runeWidth{r, w})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].r < entries[j].r
		})

		// Determine default width (space width or first entry)
		defaultW := 0
		if w, ok := fd.widths[' ']; ok {
			defaultW = w
		}

		fmt.Fprintf(&b, "// %s — %s character widths (%d entries).\n",
			fd.varName, fd.name, len(entries))
		fmt.Fprintf(&b, "var %s = map[rune]int{\n", fd.varName)
		fmt.Fprintf(&b, "\t0: %d, // default\n", defaultW)

		for _, e := range entries {
			fmt.Fprintf(&b, "\t%s: %d,\n", runeGoLit(e.r), e.w)
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

type kernEntry struct {
	left, right rune
	value       int
}

func writeKernsFile(fonts []*fontData) string {
	var b strings.Builder
	b.WriteString(`// Code generated by cmd/gen-metrics; DO NOT EDIT.
// Source: Adobe Font Metrics (AFM) files — public domain.

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

`)

	for _, fd := range fonts {
		// Sort kerns for stable output
		var entries []kernEntry
		for k, v := range fd.kerns {
			entries = append(entries, kernEntry{k[0], k[1], v})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].left != entries[j].left {
				return entries[i].left < entries[j].left
			}
			return entries[i].right < entries[j].right
		})

		kernVar := strings.Replace(fd.varName, "Widths", "KernPairs", 1)
		fmt.Fprintf(&b, "// %s — %s kerning pairs (%d entries).\n",
			kernVar, fd.name, len(entries))
		fmt.Fprintf(&b, "var %s = map[kernKey]int{\n", kernVar)
		for _, e := range entries {
			fmt.Fprintf(&b, "\t{%s, %s}: %d,\n", runeGoLit(e.left), runeGoLit(e.right), e.value)
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

func main() {
	writeMode := len(os.Args) > 1 && os.Args[1] == "-write"
	afmDir := filepath.Join("cmd", "gen-metrics", "afm")

	type fontSpec struct {
		file    string
		varName string
	}
	specs := []fontSpec{
		{"Helvetica.afm", "helveticaWidths"},
		{"Helvetica-Bold.afm", "helveticaBoldWidths"},
		{"Times-Roman.afm", "timesRomanWidths"},
		{"Times-Bold.afm", "timesBoldWidths"},
		{"Times-Italic.afm", "timesItalicWidths"},
		{"Times-BoldItalic.afm", "timesBoldItalicWidths"},
	}

	var fonts []*fontData
	for _, spec := range specs {
		fd, err := parseAFM(filepath.Join(afmDir, spec.file))
		if err != nil {
			log.Fatalf("parsing %s: %v", spec.file, err)
		}
		fd.varName = spec.varName
		fmt.Fprintf(os.Stderr, "%s: %d widths, %d kern pairs\n", fd.name, len(fd.widths), len(fd.kerns))
		fonts = append(fonts, fd)
	}

	widthsSrc := writeWidthsFile(fonts)
	kernsSrc := writeKernsFile(fonts)

	if writeMode {
		if err := os.WriteFile(filepath.Join("font", "widths.go"), []byte(widthsSrc), 0644); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("font", "metrics_data.go"), []byte(kernsSrc), 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(os.Stderr, "Wrote font/widths.go and font/metrics_data.go")
	} else {
		fmt.Println("=== font/widths.go ===")
		fmt.Println(widthsSrc[:min(len(widthsSrc), 2000)], "...")
		fmt.Println("=== font/metrics_data.go ===")
		fmt.Println(kernsSrc[:min(len(kernsSrc), 2000)], "...")
		fmt.Fprintln(os.Stderr, "\nDry run. Use -write to overwrite files.")
	}
}
