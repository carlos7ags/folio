package html

import (
	"fmt"
	"testing"
)

func TestBrInsideAllInlineTags(t *testing.T) {
	tags := []struct {
		open, close string
	}{
		{"<strong>", "</strong>"},
		{"<em>", "</em>"},
		{"<b>", "</b>"},
		{"<i>", "</i>"},
		{"<u>", "</u>"},
		{"<s>", "</s>"},
		{"<del>", "</del>"},
		{"<mark>", "</mark>"},
		{"<small>", "</small>"},
		{"<sub>", "</sub>"},
		{"<sup>", "</sup>"},
		{"<code>", "</code>"},
		{"<span>", "</span>"},
		{`<a href="#">`, "</a>"},
	}
	for _, tag := range tags {
		name := tag.open
		t.Run(name, func(t *testing.T) {
			// In a paragraph
			src := fmt.Sprintf("<p>before %stext<br/>more%s after</p>", tag.open, tag.close)
			elems, err := Convert(src, nil)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if len(elems) == 0 {
				t.Fatal("no elements")
			}

			// In a list item
			src2 := fmt.Sprintf("<ol><li>%stext<br/>more%s</li></ol>", tag.open, tag.close)
			elems2, err := Convert(src2, nil)
			if err != nil {
				t.Fatalf("Convert (list): %v", err)
			}
			if len(elems2) == 0 {
				t.Fatal("no elements (list)")
			}
		})
	}
}
