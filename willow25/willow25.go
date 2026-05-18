// Package willow25 is the Willow'25 specialisation of the generic
// datamodel and meadowcap layers. It fixes path limits to 4096 / 4096 /
// 4096 bytes-per-component / components-per-path / total-path-bytes (per
// https://willowprotocol.org/specs/willow25/), pins NamespaceID and
// SubspaceID to 32-byte Ed25519 public keys, pins PayloadDigest to a
// 32-byte BLAKE3 hash, and provides convenience constructors that bundle
// these choices together.
//
// Application code that does not need to swap parameters should depend on
// this package and only drop down to datamodel / meadowcap / encoding for
// advanced cases.
package willow25

import (
	"github.com/Deln0r/willow-go/datamodel"
)

// Standard path parameters per the Willow'25 specification.
const (
	// MaxComponentLength is the per-component byte cap.
	MaxComponentLength = 4096
	// MaxComponentCount is the maximum number of components in any single path.
	MaxComponentCount = 4096
	// MaxPathLength is the maximum total path length in bytes.
	MaxPathLength = 4096
)

// Fixed widths for the Willow'25 id and digest types.
const (
	NamespaceIDWidth   = 32
	SubspaceIDWidth    = 32
	PayloadDigestWidth = 32
)

// Limits returns the Willow'25 datamodel.Limits.
func Limits() datamodel.Limits {
	return datamodel.Limits{
		MaxComponentLength: MaxComponentLength,
		MaxComponentCount:  MaxComponentCount,
		MaxPathLength:      MaxPathLength,
	}
}

// EntrySpec returns the Willow'25 datamodel.EntrySpec used when decoding
// Entry encodings (32-byte ids and digest, plus the standard path limits).
func EntrySpec() datamodel.EntrySpec {
	return datamodel.EntrySpec{
		Limits:              Limits(),
		NamespaceIDLength:   NamespaceIDWidth,
		SubspaceIDLength:    SubspaceIDWidth,
		PayloadDigestLength: PayloadDigestWidth,
	}
}
