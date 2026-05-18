package meadowcap

import (
	"crypto/ed25519"

	"github.com/Deln0r/willow-go/datamodel"
)

// AuthorisationToken pairs a CommunalCapability with a signature by the
// capability's receiver over the canonical entry encoding. A Store uses
// this token to gate entry insertions: an entry is authorised iff the
// capability includes the entry AND the signature verifies.
type AuthorisationToken struct {
	Capability CommunalCapability
	Signature  []byte // 64-byte Ed25519 signature
}

// NewAuthorisationToken creates a signed token for the given entry. The
// caller's private key must correspond to the capability's receiver public
// key. Returns ErrCapabilityRejectsEntry if the capability does not include
// the entry (saves the caller from producing a useless token).
func NewAuthorisationToken(
	cap CommunalCapability,
	receiverPrivateKey ed25519.PrivateKey,
	entry datamodel.Entry,
) (AuthorisationToken, error) {
	if !cap.IncludesEntry(entry) {
		return AuthorisationToken{}, ErrCapabilityRejectsEntry
	}
	signature := ed25519.Sign(receiverPrivateKey, entry.Encode())
	return AuthorisationToken{
		Capability: cap,
		Signature:  signature,
	}, nil
}

// Verify reports whether t authorises the given entry: the capability must
// include the entry AND t.Signature must be a valid Ed25519 signature by
// the capability's receiver over the entry's canonical encoding.
//
// Returns nil on success, ErrCapabilityRejectsEntry if the capability does
// not include the entry, or ErrInvalidSignature on signature failure.
func (t AuthorisationToken) Verify(entry datamodel.Entry) error {
	if !t.Capability.IncludesEntry(entry) {
		return ErrCapabilityRejectsEntry
	}
	if len(t.Signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	if !ed25519.Verify(t.Capability.Receiver(), entry.Encode(), t.Signature) {
		return ErrInvalidSignature
	}
	return nil
}
