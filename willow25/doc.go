// Package willow25 is the Willow'25 specialisation of the generic datamodel
// and meadowcap layers. See https://willowprotocol.org/specs/willow25/.
//
// It fixes the path limits to 4096 bytes per component, 4096 components per
// path, and 4096 total path bytes; pins NamespaceID and SubspaceID to 32-byte
// Ed25519 public keys; pins PayloadDigest to the 32-byte WILLIAM3 hash; and
// provides convenience constructors that bundle those choices together.
//
// WILLIAM3 is the BLAKE3 compression function with a substituted IV, so it is
// domain-separated from vanilla BLAKE3 and yields different digests for the
// same input. The implementation is a pure-Go port of bab_rs 0.8.0, verified
// byte-for-byte against the upstream william3vectors.txt.
//
// Application code that does not need to swap parameters should depend on this
// package, dropping down to datamodel, meadowcap, or encoding only for
// advanced cases.
package willow25
