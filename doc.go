// Package willow is a pure-Go implementation of the Willow Protocol
// (https://willowprotocol.org), a peer-to-peer protocol for synchronizable
// data stores with fine-grained capability-based permissions.
//
// The implementation tracks the Rust reference at willow_rs
// (https://codeberg.org/worm-blossom/willow_rs) and the protocol spec, and
// is validated against fixtures generated from that reference.
//
// Subpackages:
//
//   - datamodel: Paths, Entries, Groupings, byte-encodings, Store interface.
//   - meadowcap: capability-based authorisation (Ed25519 signatures).
//   - willow25:  the 2025 parameter bundle wrapping datamodel and meadowcap
//     for interop with default-parametrized Rust impls.
//   - encoding:  shared binary encoders/decoders used across the above.
//
// Pre-MVP scope deliberately excludes WGPS sync, persistent storage, and
// transport-encryption layers. Those are deferred to later phases.
package willow
