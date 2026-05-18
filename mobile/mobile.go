// Package mobile is the gomobile-bindable surface of willow-go. It exposes a
// small subset of the data-model and willow25 APIs through types that
// gomobile can translate to Java (Android AAR) and Objective-C (iOS
// Framework). The full Go API in datamodel/, meadowcap/, and willow25/ is
// not bindable as-is because it uses [][]byte, *[]byte, custom interfaces,
// and pointer receivers in ways gomobile does not directly support.
//
// To build:
//
//	gomobile bind -target=ios     ./mobile   # produces Mobile.xcframework
//	gomobile bind -target=android ./mobile   # produces mobile.aar
//
// See README.md for the full mobile workflow.
package mobile

import (
	"errors"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/willow25"
)

// HashPayload returns the 32-byte WILLIAM3 digest of payload. This is the
// payload-digest function specified by Willow'25.
func HashPayload(payload []byte) []byte {
	d := willow25.HashPayload(payload)
	return d[:]
}

// Limits returns the three Willow'25 path size limits as a triple-encoded
// blob: [mcl big-endian uint32][mcc big-endian uint32][mpl big-endian uint32].
// Mobile callers typically don't need this and can use the implicit Willow'25
// limits (4096 each) via the *Willow25 builders.
func Limits() []byte {
	out := make([]byte, 12)
	putU32BE(out[0:4], willow25.MaxComponentLength)
	putU32BE(out[4:8], willow25.MaxComponentCount)
	putU32BE(out[8:12], willow25.MaxPathLength)
	return out
}

// PathBuilder accumulates components for a Willow path, then encodes them.
// Builders are not reusable; call Encode at most once per builder.
type PathBuilder struct {
	components [][]byte
}

// NewPathBuilder returns an empty PathBuilder under Willow'25 limits.
func NewPathBuilder() *PathBuilder { return &PathBuilder{} }

// AddComponent appends one component. The bytes are defensively copied so
// callers may reuse the input buffer.
func (b *PathBuilder) AddComponent(c []byte) {
	owned := make([]byte, len(c))
	copy(owned, c)
	b.components = append(b.components, owned)
}

// ComponentCount returns how many components have been added so far.
func (b *PathBuilder) ComponentCount() int { return len(b.components) }

// Encode returns the canonical byte-encoding of the accumulated path, or an
// error if the path violates Willow'25 limits.
func (b *PathBuilder) Encode() ([]byte, error) {
	p, err := willow25.NewPath(b.components)
	if err != nil {
		return nil, err
	}
	return p.Encode(), nil
}

// EntryBuilder accumulates the fields of an Entry. NamespaceID, SubspaceID,
// and PayloadDigest must each be exactly 32 bytes when Encode is called.
type EntryBuilder struct {
	namespaceID   []byte
	subspaceID    []byte
	pathBuilder   *PathBuilder
	timestamp     int64
	payloadLength int64
	payloadDigest []byte
}

// NewEntryBuilder returns a fresh builder with a fresh nested PathBuilder.
func NewEntryBuilder() *EntryBuilder {
	return &EntryBuilder{pathBuilder: NewPathBuilder()}
}

// SetNamespaceID, SetSubspaceID, SetPayloadDigest, SetTimestamp, and
// SetPayloadLength populate the corresponding fields. The byte-slice setters
// defensively copy.
func (b *EntryBuilder) SetNamespaceID(ns []byte) {
	b.namespaceID = make([]byte, len(ns))
	copy(b.namespaceID, ns)
}

func (b *EntryBuilder) SetSubspaceID(sub []byte) {
	b.subspaceID = make([]byte, len(sub))
	copy(b.subspaceID, sub)
}

func (b *EntryBuilder) SetPayloadDigest(d []byte) {
	b.payloadDigest = make([]byte, len(d))
	copy(b.payloadDigest, d)
}

func (b *EntryBuilder) SetTimestamp(ts int64)     { b.timestamp = ts }
func (b *EntryBuilder) SetPayloadLength(pl int64) { b.payloadLength = pl }

// AddPathComponent forwards to the nested PathBuilder.
func (b *EntryBuilder) AddPathComponent(c []byte) { b.pathBuilder.AddComponent(c) }

// Encode produces the canonical byte-encoding of the Entry, or an error if
// any required field is missing / mis-sized or the path is invalid.
func (b *EntryBuilder) Encode() ([]byte, error) {
	if len(b.namespaceID) != willow25.NamespaceIDWidth {
		return nil, errors.New("mobile: namespace id must be 32 bytes")
	}
	if len(b.subspaceID) != willow25.SubspaceIDWidth {
		return nil, errors.New("mobile: subspace id must be 32 bytes")
	}
	if len(b.payloadDigest) != willow25.PayloadDigestWidth {
		return nil, errors.New("mobile: payload digest must be 32 bytes")
	}
	if b.timestamp < 0 {
		return nil, errors.New("mobile: timestamp must be non-negative")
	}
	if b.payloadLength < 0 {
		return nil, errors.New("mobile: payload length must be non-negative")
	}
	path, err := willow25.NewPath(b.pathBuilder.components)
	if err != nil {
		return nil, err
	}
	entry := datamodel.Entry{
		NamespaceID:   b.namespaceID,
		SubspaceID:    b.subspaceID,
		Path:          path,
		Timestamp:     uint64(b.timestamp),
		PayloadLength: uint64(b.payloadLength),
		PayloadDigest: b.payloadDigest,
	}
	return entry.Encode(), nil
}

// HashAndEncodeEntry is a one-shot convenience: hash the payload, populate
// the digest + length fields, and return the encoded Entry. NamespaceID and
// SubspaceID must be 32 bytes each.
func HashAndEncodeEntry(namespaceID, subspaceID []byte, pathBuilder *PathBuilder, timestamp int64, payload []byte) ([]byte, error) {
	b := NewEntryBuilder()
	b.SetNamespaceID(namespaceID)
	b.SetSubspaceID(subspaceID)
	b.pathBuilder = pathBuilder
	b.SetTimestamp(timestamp)
	b.SetPayloadLength(int64(len(payload)))
	digest := willow25.HashPayload(payload)
	b.SetPayloadDigest(digest[:])
	return b.Encode()
}

func putU32BE(dst []byte, v int) {
	dst[0] = byte(v >> 24)
	dst[1] = byte(v >> 16)
	dst[2] = byte(v >> 8)
	dst[3] = byte(v)
}
