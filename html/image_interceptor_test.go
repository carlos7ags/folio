package html

import (
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

func TestImageInterceptor(t *testing.T) {
	elems, err := Convert(`<img src="http://localhost/photo.jpg"/>`, &Options{
		URLPolicy: func(src string) error {
			t.Logf("URLPolicy called with src: %s, returning error", src)
			return fmt.Errorf("Loading external images is not allowed")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}

	// element should be a layout.Paragraph element, not an image element, since the URLPolicy returned an error to prevent loading
	_, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected a Paragraph element, got %T", elems[0])
	}
}
