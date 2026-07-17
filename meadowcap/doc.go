// Package meadowcap implements the Meadowcap capability system.
// See https://willowprotocol.org/specs/meadowcap/.
//
// A communal capability binds a receiver to their own subspace within a
// namespace. Capabilities can be handed on: AppendDelegation narrows the
// granted area and transfers it to a new receiver, signing the canonical
// handover bytes with the current receiver's key, and IsValid walks the
// resulting chain verifying every link. AuthorisationToken wraps a capability
// with a signature over an entry's canonical encoding, and is the unit a Store
// uses to gate insertions.
//
// Scope today: communal capabilities including multi-step delegation chains,
// validated against delegation chains signed by willow_rs. Owned namespaces
// and read capabilities are Phase 2, as is the spec-canonical capability wire
// encoding (encode_mc_capability). See TECH_DEBT.md.
package meadowcap
