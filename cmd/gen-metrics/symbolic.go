// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// The symbolic standard fonts (Symbol and ZapfDingbats) do not use a
// Latin text encoding: their AFM files carry a built-in encoding whose
// glyph names are not in the Latin glyphToRune table (Symbol reuses some
// names, such as "mu", for different characters). Each therefore gets its
// own glyph-name table. Symbol follows Adobe's Symbol-to-Unicode mapping,
// preferring the Greek letters (U+0394, U+03A9, U+03BC) and the real
// ®/©/™ code points over their legacy and private-use alternates;
// ZapfDingbats follows Adobe's ZapfDingbats-to-Unicode mapping (the
// Unicode Dingbats block was laid out from this font's code order).

// symbolGlyphToRune maps Symbol AFM glyph names to Unicode.
var symbolGlyphToRune = map[string]rune{
	"space": ' ', "exclam": '!', "universal": 0x2200, "numbersign": '#',
	"existential": 0x2203, "percent": '%', "ampersand": '&',
	"suchthat": 0x220B, "parenleft": '(', "parenright": ')',
	"asteriskmath": 0x2217, "plus": '+', "comma": ',', "minus": 0x2212,
	"period": '.', "slash": '/', "zero": '0', "one": '1', "two": '2',
	"three": '3', "four": '4', "five": '5', "six": '6', "seven": '7',
	"eight": '8', "nine": '9', "colon": ':', "semicolon": ';', "less": '<',
	"equal": '=', "greater": '>', "question": '?', "congruent": 0x2245,
	"Alpha": 0x0391, "Beta": 0x0392, "Chi": 0x03A7, "Delta": 0x0394,
	"Epsilon": 0x0395, "Phi": 0x03A6, "Gamma": 0x0393, "Eta": 0x0397,
	"Iota": 0x0399, "theta1": 0x03D1, "Kappa": 0x039A, "Lambda": 0x039B,
	"Mu": 0x039C, "Nu": 0x039D, "Omicron": 0x039F, "Pi": 0x03A0,
	"Theta": 0x0398, "Rho": 0x03A1, "Sigma": 0x03A3, "Tau": 0x03A4,
	"Upsilon": 0x03A5, "sigma1": 0x03C2, "Omega": 0x03A9, "Xi": 0x039E,
	"Psi": 0x03A8, "Zeta": 0x0396, "bracketleft": '[', "therefore": 0x2234,
	"bracketright": ']', "perpendicular": 0x22A5, "underscore": '_',
	"radicalex": 0xF8E5, "alpha": 0x03B1, "beta": 0x03B2, "chi": 0x03C7,
	"delta": 0x03B4, "epsilon": 0x03B5, "phi": 0x03C6, "gamma": 0x03B3,
	"eta": 0x03B7, "iota": 0x03B9, "phi1": 0x03D5, "kappa": 0x03BA,
	"lambda": 0x03BB, "mu": 0x03BC, "nu": 0x03BD, "omicron": 0x03BF,
	"pi": 0x03C0, "theta": 0x03B8, "rho": 0x03C1, "sigma": 0x03C3,
	"tau": 0x03C4, "upsilon": 0x03C5, "omega1": 0x03D6, "omega": 0x03C9,
	"xi": 0x03BE, "psi": 0x03C8, "zeta": 0x03B6, "braceleft": '{', "bar": '|',
	"braceright": '}', "similar": 0x223C, "Euro": 0x20AC, "Upsilon1": 0x03D2,
	"minute": 0x2032, "lessequal": 0x2264, "fraction": 0x2044,
	"infinity": 0x221E, "florin": 0x0192, "club": 0x2663, "diamond": 0x2666,
	"heart": 0x2665, "spade": 0x2660, "arrowboth": 0x2194,
	"arrowleft": 0x2190, "arrowup": 0x2191, "arrowright": 0x2192,
	"arrowdown": 0x2193, "degree": 0x00B0, "plusminus": 0x00B1,
	"second": 0x2033, "greaterequal": 0x2265, "multiply": 0x00D7,
	"proportional": 0x221D, "partialdiff": 0x2202, "bullet": 0x2022,
	"divide": 0x00F7, "notequal": 0x2260, "equivalence": 0x2261,
	"approxequal": 0x2248, "ellipsis": 0x2026, "arrowvertex": 0xF8E6,
	"arrowhorizex": 0xF8E7, "carriagereturn": 0x21B5, "aleph": 0x2135,
	"Ifraktur": 0x2111, "Rfraktur": 0x211C, "weierstrass": 0x2118,
	"circlemultiply": 0x2297, "circleplus": 0x2295, "emptyset": 0x2205,
	"intersection": 0x2229, "union": 0x222A, "propersuperset": 0x2283,
	"reflexsuperset": 0x2287, "notsubset": 0x2284, "propersubset": 0x2282,
	"reflexsubset": 0x2286, "element": 0x2208, "notelement": 0x2209,
	"angle": 0x2220, "gradient": 0x2207, "registerserif": 0x00AE,
	"copyrightserif": 0x00A9, "trademarkserif": 0x2122, "product": 0x220F,
	"radical": 0x221A, "dotmath": 0x22C5, "logicalnot": 0x00AC,
	"logicaland": 0x2227, "logicalor": 0x2228, "arrowdblboth": 0x21D4,
	"arrowdblleft": 0x21D0, "arrowdblup": 0x21D1, "arrowdblright": 0x21D2,
	"arrowdbldown": 0x21D3, "lozenge": 0x25CA, "angleleft": 0x2329,
	"registersans": 0xF8E8, "copyrightsans": 0xF8E9, "trademarksans": 0xF8EA,
	"summation": 0x2211, "parenlefttp": 0xF8EB, "parenleftex": 0xF8EC,
	"parenleftbt": 0xF8ED, "bracketlefttp": 0xF8EE, "bracketleftex": 0xF8EF,
	"bracketleftbt": 0xF8F0, "bracelefttp": 0xF8F1, "braceleftmid": 0xF8F2,
	"braceleftbt": 0xF8F3, "braceex": 0xF8F4, "angleright": 0x232A,
	"integral": 0x222B, "integraltp": 0x2320, "integralex": 0xF8F5,
	"integralbt": 0x2321, "parenrighttp": 0xF8F6, "parenrightex": 0xF8F7,
	"parenrightbt": 0xF8F8, "bracketrighttp": 0xF8F9,
	"bracketrightex": 0xF8FA, "bracketrightbt": 0xF8FB,
	"bracerighttp": 0xF8FC, "bracerightmid": 0xF8FD, "bracerightbt": 0xF8FE,
}

// zapfDingbatsGlyphToRune maps ZapfDingbats AFM glyph names to Unicode.
var zapfDingbatsGlyphToRune = map[string]rune{
	"space": ' ', "a1": 0x2701, "a2": 0x2702, "a202": 0x2703, "a3": 0x2704,
	"a4": 0x260E, "a5": 0x2706, "a119": 0x2707, "a118": 0x2708,
	"a117": 0x2709, "a11": 0x261B, "a12": 0x261E, "a13": 0x270C,
	"a14": 0x270D, "a15": 0x270E, "a16": 0x270F, "a105": 0x2710,
	"a17": 0x2711, "a18": 0x2712, "a19": 0x2713, "a20": 0x2714, "a21": 0x2715,
	"a22": 0x2716, "a23": 0x2717, "a24": 0x2718, "a25": 0x2719, "a26": 0x271A,
	"a27": 0x271B, "a28": 0x271C, "a6": 0x271D, "a7": 0x271E, "a8": 0x271F,
	"a9": 0x2720, "a10": 0x2721, "a29": 0x2722, "a30": 0x2723, "a31": 0x2724,
	"a32": 0x2725, "a33": 0x2726, "a34": 0x2727, "a35": 0x2605, "a36": 0x2729,
	"a37": 0x272A, "a38": 0x272B, "a39": 0x272C, "a40": 0x272D, "a41": 0x272E,
	"a42": 0x272F, "a43": 0x2730, "a44": 0x2731, "a45": 0x2732, "a46": 0x2733,
	"a47": 0x2734, "a48": 0x2735, "a49": 0x2736, "a50": 0x2737, "a51": 0x2738,
	"a52": 0x2739, "a53": 0x273A, "a54": 0x273B, "a55": 0x273C, "a56": 0x273D,
	"a57": 0x273E, "a58": 0x273F, "a59": 0x2740, "a60": 0x2741, "a61": 0x2742,
	"a62": 0x2743, "a63": 0x2744, "a64": 0x2745, "a65": 0x2746, "a66": 0x2747,
	"a67": 0x2748, "a68": 0x2749, "a69": 0x274A, "a70": 0x274B, "a71": 0x25CF,
	"a72": 0x274D, "a73": 0x25A0, "a74": 0x274F, "a203": 0x2750,
	"a75": 0x2751, "a204": 0x2752, "a76": 0x25B2, "a77": 0x25BC,
	"a78": 0x25C6, "a79": 0x2756, "a81": 0x25D7, "a82": 0x2758, "a83": 0x2759,
	"a84": 0x275A, "a97": 0x275B, "a98": 0x275C, "a99": 0x275D,
	"a100": 0x275E, "a89": 0x2768, "a90": 0x2769, "a93": 0x276A,
	"a94": 0x276B, "a91": 0x276C, "a92": 0x276D, "a205": 0x276E,
	"a85": 0x276F, "a206": 0x2770, "a86": 0x2771, "a87": 0x2772,
	"a88": 0x2773, "a95": 0x2774, "a96": 0x2775, "a101": 0x2761,
	"a102": 0x2762, "a103": 0x2763, "a104": 0x2764, "a106": 0x2765,
	"a107": 0x2766, "a108": 0x2767, "a112": 0x2663, "a111": 0x2666,
	"a110": 0x2665, "a109": 0x2660, "a120": 0x2460, "a121": 0x2461,
	"a122": 0x2462, "a123": 0x2463, "a124": 0x2464, "a125": 0x2465,
	"a126": 0x2466, "a127": 0x2467, "a128": 0x2468, "a129": 0x2469,
	"a130": 0x2776, "a131": 0x2777, "a132": 0x2778, "a133": 0x2779,
	"a134": 0x277A, "a135": 0x277B, "a136": 0x277C, "a137": 0x277D,
	"a138": 0x277E, "a139": 0x277F, "a140": 0x2780, "a141": 0x2781,
	"a142": 0x2782, "a143": 0x2783, "a144": 0x2784, "a145": 0x2785,
	"a146": 0x2786, "a147": 0x2787, "a148": 0x2788, "a149": 0x2789,
	"a150": 0x278A, "a151": 0x278B, "a152": 0x278C, "a153": 0x278D,
	"a154": 0x278E, "a155": 0x278F, "a156": 0x2790, "a157": 0x2791,
	"a158": 0x2792, "a159": 0x2793, "a160": 0x2794, "a161": 0x2192,
	"a163": 0x2194, "a164": 0x2195, "a196": 0x2798, "a165": 0x2799,
	"a192": 0x279A, "a166": 0x279B, "a167": 0x279C, "a168": 0x279D,
	"a169": 0x279E, "a170": 0x279F, "a171": 0x27A0, "a172": 0x27A1,
	"a173": 0x27A2, "a162": 0x27A3, "a174": 0x27A4, "a175": 0x27A5,
	"a176": 0x27A6, "a177": 0x27A7, "a178": 0x27A8, "a179": 0x27A9,
	"a193": 0x27AA, "a180": 0x27AB, "a199": 0x27AC, "a181": 0x27AD,
	"a200": 0x27AE, "a182": 0x27AF, "a201": 0x27B1, "a183": 0x27B2,
	"a184": 0x27B3, "a197": 0x27B4, "a185": 0x27B5, "a194": 0x27B6,
	"a198": 0x27B7, "a186": 0x27B8, "a195": 0x27B9, "a187": 0x27BA,
	"a188": 0x27BB, "a189": 0x27BC, "a190": 0x27BD, "a191": 0x27BE,
}

// builtinEncoding is a symbolic font's built-in encoding as read from its
// AFM file: per code, the Unicode value and advance width of the glyph.
type builtinEncoding struct {
	name    string
	varName string
	runes   [256]rune
	names   [256]string
	widths  [256]int
}

// parseAFMEncoding reads the encoded glyphs (C >= 0) of an AFM file and
// maps their names through names. Every encoded glyph must be mapped, so a
// gap in a table fails generation instead of silently dropping a code.
func parseAFMEncoding(path string, names map[string]rune) (*builtinEncoding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	enc := &builtinEncoding{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "FontName ") {
			enc.name = strings.TrimPrefix(line, "FontName ")
			continue
		}
		if !strings.HasPrefix(line, "C ") {
			continue
		}
		code, width, name := -1, 0, ""
		for _, p := range strings.Split(line, ";") {
			p = strings.TrimSpace(p)
			switch {
			case strings.HasPrefix(p, "C "):
				if c, err := strconv.Atoi(strings.TrimPrefix(p, "C ")); err == nil {
					code = c
				}
			case strings.HasPrefix(p, "WX "):
				if w, err := strconv.Atoi(strings.TrimPrefix(p, "WX ")); err == nil {
					width = w
				}
			case strings.HasPrefix(p, "N "):
				name = strings.TrimPrefix(p, "N ")
			}
		}
		if code < 0 || code > 255 {
			continue // unencoded glyph (e.g. Symbol's "apple")
		}
		r, ok := names[name]
		if !ok {
			return nil, fmt.Errorf("%s: glyph %q (code %d) has no Unicode mapping", path, name, code)
		}
		enc.runes[code] = r
		enc.names[code] = name
		enc.widths[code] = width
	}
	return enc, scanner.Err()
}

// writeSymbolicFile renders the built-in encodings as Go source.
func writeSymbolicFile(encs []*builtinEncoding) string {
	var b strings.Builder
	b.WriteString(`// Code generated by cmd/gen-metrics -symbolic; DO NOT EDIT.
// Source: Adobe Font Metrics (AFM) files — public domain.

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

`)
	for _, e := range encs {
		fmt.Fprintf(&b, "// %sCodeToRune is the %s built-in encoding (ISO 32000-1 Annex D):\n", e.varName, e.name)
		b.WriteString("// the Unicode value of the glyph at each code; 0 marks an unused code.\n")
		fmt.Fprintf(&b, "var %sCodeToRune = [256]rune{\n", e.varName)
		for c, r := range e.runes {
			if r != 0 {
				fmt.Fprintf(&b, "\t0x%02X: %s, // %s\n", c, runeGoLit(r), e.names[c])
			}
		}
		b.WriteString("}\n\n")

		fmt.Fprintf(&b, "// %sCodeWidths holds the %s glyph advance at each built-in\n", e.varName, e.name)
		b.WriteString("// code in 1/1000 text-space units; 0 marks an unused code.\n")
		fmt.Fprintf(&b, "var %sCodeWidths = [256]int{\n", e.varName)
		for c, w := range e.widths {
			if e.runes[c] != 0 {
				fmt.Fprintf(&b, "\t0x%02X: %d,\n", c, w)
			}
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}
