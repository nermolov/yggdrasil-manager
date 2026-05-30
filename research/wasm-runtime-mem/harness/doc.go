//go:build wasmmem

// Package harness provides measurement primitives and a runtime registry for the
// WASM-runtime memory benchmark suite.
package harness

import _ "embed"

//go:embed testdata/document.am
var DocumentAM []byte
