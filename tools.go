//go:build tools

// Package tools forces `go mod tidy` to keep build-time tooling
// dependencies in go.mod. The `tools` build tag excludes this file from
// normal builds — these imports never appear in any compiled binary.
package tools

import (
	// golang.org/x/mobile/bind is required by `gomobile bind` to generate
	// the cgo glue layer. Without this anchor `go mod tidy` would drop the
	// x/mobile dependency and the mobile-{ios,android} Makefile targets
	// would fail.
	_ "golang.org/x/mobile/bind"
)
