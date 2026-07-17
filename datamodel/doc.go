// Package datamodel implements the Willow data model: Paths, Entries,
// Groupings (Range3d and Area), and the Store interface.
// See https://willowprotocol.org/specs/data-model/.
//
// A Path is an ordered, immutable sequence of byte-slice components subject to
// three size constraints (max_component_length, max_component_count,
// max_path_length) that callers configure per-Path through a Limits value. An
// Entry pairs a coordinate (namespace, subspace, path, timestamp) with a
// payload digest and length. Range3d and Area describe sets of coordinates;
// Area also encodes relative to an enclosing area, which is how Willow
// compresses shared context on the wire.
//
// The package stays generic over Willow's parameters. For the concrete
// Willow'25 bundle (4096/4096/4096 limits, 32-byte ids, WILLIAM3 digests) use
// the willow25 package instead of wiring Limits by hand.
//
// Only the in-memory Store ships today; it holds Entry metadata, not payload
// bytes. A persistent backend is Phase 2, as is payload storage. See
// TECH_DEBT.md.
package datamodel
