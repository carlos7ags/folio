// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// CJK demonstrates PDF generation with Chinese, Japanese, and Korean text.
//
// It exercises CJK-aware line breaking: ideographs, kana, and hangul
// characters break at character boundaries, while kinsoku shori rules
// keep opening punctuation grouped with the following character and
// closing punctuation grouped with the preceding character.
//
// This example requires a Unicode font with CJK coverage. It searches
// common system font paths; if no CJK font is found, the example exits
// with a message indicating which paths were checked.
//
// Usage:
//
//	go run ./examples/cjk
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
)

func main() {
	fontPath := findCJKFont()
	if fontPath == "" {
		fmt.Fprintln(os.Stderr, "no CJK font found on this system; see source for checked paths")
		os.Exit(1)
	}

	htmlStr := buildHTML(fontPath)

	result, err := html.ConvertFull(htmlStr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		os.Exit(1)
	}

	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Folio CJK Text Layout"
	doc.Info.Author = "Folio"

	for _, e := range result.Elements {
		doc.Add(e)
	}

	if err := doc.Save("cjk.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created cjk.pdf")
}

func buildHTML(fontPath string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<style>
@font-face {
  font-family: 'CJK';
  src: url('%s');
}
@page { margin: 40px; }
body {
  font-family: 'CJK';
  font-size: 11px;
  color: #1a1a2e;
  line-height: 1.6;
}
h1 {
  font-size: 20px;
  margin-bottom: 6px;
  border-bottom: 2px solid #1a1a2e;
  padding-bottom: 4px;
}
h2 {
  font-size: 14px;
  color: #16213e;
  margin-top: 18px;
  margin-bottom: 6px;
}
h3 {
  font-size: 12px;
  color: #333;
  margin-top: 12px;
  margin-bottom: 4px;
}
p { margin-bottom: 8px; }
.narrow {
  width: 200px;
  border: 1px solid #ccc;
  padding: 6px;
  margin-bottom: 10px;
  background: #f8f9fa;
  break-inside: avoid;
}
.medium {
  width: 320px;
  border: 1px solid #ccc;
  padding: 6px;
  margin-bottom: 10px;
  background: #f8f9fa;
  break-inside: avoid;
}
.break-all { word-break: break-all; }
.note {
  font-size: 9px;
  color: #666;
  margin-top: 2px;
  margin-bottom: 12px;
}
table {
  border-collapse: collapse;
  margin-bottom: 12px;
  font-size: 10px;
}
th, td {
  border: 1px solid #999;
  padding: 4px 8px;
  text-align: left;
}
th { background: #e8e8e8; }
</style>
</head>
<body>

<h1>CJK Text Layout in Folio</h1>
<p>
  This document demonstrates CJK (Chinese, Japanese, Korean) text rendering
  and line-breaking behavior in the Folio PDF library.
</p>

<!-- ===== Section 1: Chinese ===== -->
<h2>1. Chinese (Simplified)</h2>
<p>
  Folio能够正确渲染中文文本。每个汉字都是一个独立的断行单元，
  使得文本能够在任意两个汉字之间自然换行，无需空格分隔。
  这是中文排版的基本要求。
</p>

<h3>Narrow container (200px)</h3>
<div class="narrow">
  中华人民共和国是一个历史悠久的文明古国。在漫长的历史发展过程中，
  中国人民创造了灿烂的文化，为人类文明的发展做出了重要贡献。
</div>
<p class="note">Characters wrap individually at container boundary.</p>

<!-- ===== Section 2: Japanese ===== -->
<h2>2. Japanese</h2>
<p>
  日本語のテキストはひらがな、カタカナ、漢字を組み合わせて使用します。
  Folioはこれらすべての文字を正しく認識し、適切な位置で改行します。
</p>

<h3>Hiragana</h3>
<div class="narrow">
  むかしむかし、あるところに、おじいさんとおばあさんがすんでいました。
  おじいさんはやまへしばかりに、おばあさんはかわへせんたくにいきました。
</div>

<h3>Katakana</h3>
<div class="narrow">
  コンピュータサイエンスはプログラミング、アルゴリズム、データベース、
  ネットワーク、セキュリティなどの分野から構成されています。
</div>

<h3>Mixed Japanese</h3>
<div class="medium">
  東京タワーは1958年に完成した総合電波塔です。高さは333メートルで、
  完成当時は世界一高い建造物でした。現在も東京のシンボルとして
  多くの観光客が訪れています。
</div>

<!-- ===== Section 3: Korean ===== -->
<h2>3. Korean (Hangul)</h2>
<p>
  한국어는 한글이라는 독자적인 문자 체계를 사용합니다.
  한글은 세종대왕이 창제한 과학적이고 체계적인 문자입니다.
</p>

<div class="narrow">
  대한민국은 동아시아에 위치한 민주공화국이다.
  수도는 서울특별시이며, 한반도의 남쪽 절반을 차지하고 있다.
</div>

<!-- ===== Section 4: Kinsoku Shori ===== -->
<h2>4. Kinsoku Shori (Line Break Prohibitions)</h2>
<p>
  Kinsoku shori rules prevent certain punctuation from appearing at
  incorrect positions. Opening brackets must stay with the following
  character; closing brackets, commas, and periods must stay with the
  preceding character.
</p>

<h3>Opening brackets group forward</h3>
<div class="narrow">
  日本語の「括弧」は正しく処理されます。「開き括弧」は次の文字と一緒に
  保持され、行頭に取り残されることはありません。
</div>
<p class="note">
  The left corner bracket (U+300C) stays with the character after it.
</p>

<h3>Closing punctuation groups backward</h3>
<div class="narrow">
  句読点の処理も重要です。句点「。」や読点「、」は前の文字と一緒に
  保持され、行頭に来ることはありません。
</div>
<p class="note">
  Period (U+3002) and comma (U+3001) stay with the character before them.
</p>

<h3>Fullwidth punctuation</h3>
<div class="narrow">
  全角記号のテスト：コロン、セミコロン；感嘆符！疑問符？
  これらの記号は前の文字と共に保持されます。
</div>

<!-- ===== Section 5: Mixed Scripts ===== -->
<h2>5. Mixed CJK and Latin Text</h2>
<p>
  CJK text often contains embedded Latin words, numbers, and punctuation.
  Folio handles transitions between scripts correctly.
</p>

<div class="medium">
  Folio是一个用Go语言编写的PDF库。它支持HTML转换、数字签名、
  PDF/A合规性等功能。版本号为v0.x，目标是达到v1.0稳定版本。
</div>

<div class="medium">
  東京の人口は約1400万人です。GDPは世界第3位で、
  Apple、Google、Microsoftなどの企業が日本市場に進出しています。
</div>

<div class="medium">
  한국의 GDP는 약 1.8조 달러이며, Samsung, LG, Hyundai 등
  글로벌 기업들이 세계 시장에서 활동하고 있습니다.
</div>

<!-- ===== Section 6: word-break: break-all ===== -->
<h2>6. CSS word-break: break-all</h2>
<p>
  With <code>word-break: break-all</code>, Latin words can also break at
  any character boundary, matching CJK behavior for all scripts.
</p>

<div class="narrow break-all">
  Supercalifragilisticexpialidocious is a very long English word that
  demonstrates break-all behavior alongside CJK text:
  一二三四五六七八九十。
</div>

<!-- ===== Section 7: word-break: keep-all ===== -->
<h2>7. CSS word-break: keep-all</h2>
<p>
  With <code>word-break: keep-all</code>, CJK text does not break at
  character boundaries. Line breaks occur only at spaces, matching
  the behavior of Latin text. This is commonly used for Korean, which
  uses spaces between words.
</p>

<h3>Normal (default) -- characters break individually</h3>
<div class="narrow">
  한국의 경제는 반도체 산업과 자동차 산업이 주요 성장 동력이다.
</div>

<h3>keep-all -- breaks only at spaces</h3>
<div class="narrow" style="word-break: keep-all">
  한국의 경제는 반도체 산업과 자동차 산업이 주요 성장 동력이다.
</div>
<p class="note">
  Same text, but words stay intact. Only spaces are break opportunities.
</p>

<!-- ===== Section 8: Feature Matrix ===== -->
<h2>8. CJK Feature Support</h2>
<table>
  <tr>
    <th>Feature</th>
    <th>Status</th>
    <th>Notes</th>
  </tr>
  <tr>
    <td>Character-level line breaking</td>
    <td>Supported</td>
    <td>CJK characters break individually</td>
  </tr>
  <tr>
    <td>Kinsoku shori</td>
    <td>Supported</td>
    <td>Opening/closing punct grouping</td>
  </tr>
  <tr>
    <td>CJK Unified Ideographs</td>
    <td>Supported</td>
    <td>Core + Extensions A-F</td>
  </tr>
  <tr>
    <td>Hiragana / Katakana</td>
    <td>Supported</td>
    <td>Including phonetic extensions</td>
  </tr>
  <tr>
    <td>Hangul</td>
    <td>Supported</td>
    <td>Syllables + Jamo</td>
  </tr>
  <tr>
    <td>Bopomofo (Zhuyin)</td>
    <td>Supported</td>
    <td>Including extended block</td>
  </tr>
  <tr>
    <td>CJK Radicals</td>
    <td>Supported</td>
    <td>Kangxi + Supplement</td>
  </tr>
  <tr>
    <td>Fullwidth forms</td>
    <td>Supported</td>
    <td>Fullwidth ASCII variants</td>
  </tr>
  <tr>
    <td>Mixed CJK/Latin text</td>
    <td>Supported</td>
    <td>Script transitions handled</td>
  </tr>
  <tr>
    <td>Font embedding + subsetting</td>
    <td>Supported</td>
    <td>CIDFont Type0 with Identity-H</td>
  </tr>
  <tr>
    <td>Text copy/paste (ToUnicode)</td>
    <td>Supported</td>
    <td>BMP + non-BMP (surrogate pairs)</td>
  </tr>
  <tr>
    <td>word-break: break-all</td>
    <td>Supported</td>
    <td>All characters break individually</td>
  </tr>
  <tr>
    <td>word-break: keep-all</td>
    <td>Supported</td>
    <td>CJK breaks only at spaces</td>
  </tr>
  <tr>
    <td>Vertical text</td>
    <td>Not planned</td>
    <td>writing-mode not supported</td>
  </tr>
  <tr>
    <td>Ruby / Furigana</td>
    <td>Not planned</td>
    <td>Annotation text not supported</td>
  </tr>
</table>

</body>
</html>`, fontPath)
}

// findCJKFont searches common system paths for a font with CJK coverage.
func findCJKFont() string {
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/STHeiti Light.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		}
	case "windows":
		paths = []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\msgothic.ttc`,
			`C:\Windows\Fonts\malgun.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
		}
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
