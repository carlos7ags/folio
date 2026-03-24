// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build js

package html

import (
	"fmt"

	folioimage "github.com/carlos7ags/folio/image"
)

// fetchImage is a stub for WASM builds where net/http is not available.
// HTTP/HTTPS image URLs cannot be fetched in the browser. Use base64
// data URIs instead: <img src="data:image/png;base64,...">
func (c *converter) fetchImage(url string) (*folioimage.Image, error) {
	return nil, fmt.Errorf("HTTP image URLs not supported in WASM (use data: URIs instead): %s", url)
}
