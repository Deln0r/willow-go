// SPDX-FileCopyrightText: 2026 Ian Chechin
// SPDX-License-Identifier: MIT

package willow25

// defaultID is the public key the Willow'25 specification fixes as both the
// default namespace id and the default subspace id.
var defaultID = [32]byte{
	0x93, 0x4e, 0x60, 0x21, 0x33, 0x9e, 0x1f, 0x01, 0x3b, 0xa9, 0x49, 0x00, 0xed, 0xc2, 0x5d, 0x8d,
	0x74, 0xc0, 0xb4, 0xe5, 0x73, 0x76, 0x89, 0x10, 0xae, 0x0f, 0x50, 0x7d, 0x8c, 0x81, 0x73, 0x18,
}

// DefaultNamespaceID returns the Willow'25 default_namespace_id
// (https://willowprotocol.org/specs/willow25/index.html#willow25_default_namespace_id),
// the namespace that relative encodings fall back to. It identifies a
// communal namespace.
func DefaultNamespaceID() [NamespaceIDWidth]byte { return defaultID }

// DefaultSubspaceID returns the Willow'25 default_subspace_id
// (https://willowprotocol.org/specs/willow25/index.html#willow25_default_subspace_id).
// It has the same bytes as DefaultNamespaceID.
func DefaultSubspaceID() [SubspaceIDWidth]byte { return defaultID }
