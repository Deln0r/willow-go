// Package encoding provides the shared binary primitives used by the
// higher-level Willow types.
//
// CompactU64 is a variable-length encoding for u64 values: small numbers are
// inlined into a tag, larger ones spill into 1, 2, 4, or 8 follow-up bytes.
// Tags can be 2 to 8 bits wide and may be packed alongside other tags into a
// single byte, which is how Willow fits several lengths into one header.
// See https://willowprotocol.org/specs/encodings/index.html#compact_integers.
//
// The encoding is canonical: every value has exactly one minimal
// representation. Decoders take a canonical flag, and passing true rejects
// non-minimal forms, which is what a peer should do with untrusted input.
//
// This is a port of the compact_u64 Rust crate (v0.6.0). The codec is covered
// by property-based round-trip tests across every tag width and by a fuzz
// target asserting the decoder never panics on attacker-supplied bytes.
package encoding
