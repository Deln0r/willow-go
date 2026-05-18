// Package meadowcap implements the Meadowcap capability system. See
// https://willowprotocol.org/specs/meadowcap/.
//
// Pre-MVP scope is deliberately narrow: communal genesis capabilities only,
// no delegation chains. A communal capability binds a single receiver
// (user_key) to a single subspace within a namespace. The
// AuthorisationToken type wraps such a capability with a signature over the
// entry's canonical encoding by the receiver's keypair, and is the unit a
// Store uses to gate entry insertions.
//
// Delegation chains (multi-step capability handover) and owned namespaces
// are tracked in TECH_DEBT.md; both depend on Area relative encoding which
// is not yet implemented.
package meadowcap

import (
	"crypto/ed25519"
	"errors"

	"github.com/Deln0r/willow-go/datamodel"
)

// AccessMode is the kind of access a capability grants.
type AccessMode uint8

const (
	// AccessModeRead grants permission to read entries within the granted area.
	AccessModeRead AccessMode = 0
	// AccessModeWrite grants permission to write entries within the granted area.
	AccessModeWrite AccessMode = 1
)

// String returns "read" or "write".
func (a AccessMode) String() string {
	switch a {
	case AccessModeRead:
		return "read"
	case AccessModeWrite:
		return "write"
	default:
		return "unknown"
	}
}

// Errors.
var (
	ErrCapabilityRejectsEntry = errors.New("meadowcap: capability does not authorise entry")
	ErrInvalidSignature       = errors.New("meadowcap: invalid signature")
	ErrInvalidNamespaceKey    = errors.New("meadowcap: namespace key wrong length")
	ErrInvalidUserKey         = errors.New("meadowcap: user key wrong length")
)

// CommunalCapability is the simplest Meadowcap capability: a single receiver
// (user_key) granted access to a single subspace within a namespace. Both
// the namespace key and the user key are Ed25519 public keys (32 bytes
// each), matching the willow25 specialisation.
//
// This pre-MVP version does NOT support delegation. The granted area is
// always the full subspace area belonging to the user_key.
type CommunalCapability struct {
	Mode         AccessMode
	NamespaceKey ed25519.PublicKey
	UserKey      ed25519.PublicKey
}

// NewCommunal returns a new communal capability. Returns an error if either
// key has the wrong length for Ed25519.
func NewCommunal(mode AccessMode, namespaceKey, userKey ed25519.PublicKey) (CommunalCapability, error) {
	if len(namespaceKey) != ed25519.PublicKeySize {
		return CommunalCapability{}, ErrInvalidNamespaceKey
	}
	if len(userKey) != ed25519.PublicKeySize {
		return CommunalCapability{}, ErrInvalidUserKey
	}
	return CommunalCapability{
		Mode:         mode,
		NamespaceKey: append(ed25519.PublicKey(nil), namespaceKey...),
		UserKey:      append(ed25519.PublicKey(nil), userKey...),
	}, nil
}

// Receiver returns the public key of the capability's receiver — the entity
// authorised to sign entries under this cap. With no delegations the
// receiver equals the user_key.
func (c CommunalCapability) Receiver() ed25519.PublicKey { return c.UserKey }

// GrantedArea returns the area within which this capability grants access:
// the full subspace area belonging to the user_key.
func (c CommunalCapability) GrantedArea(limits datamodel.Limits) datamodel.Area {
	return datamodel.SubspaceArea(limits, c.UserKey)
}

// IncludesEntry reports whether c grants access to the given entry: the
// entry's namespace must match the capability's namespace, and the entry's
// subspace must equal the capability's receiver.
func (c CommunalCapability) IncludesEntry(e datamodel.Entry) bool {
	if !bytesEqual(e.NamespaceID, c.NamespaceKey) {
		return false
	}
	return bytesEqual(e.SubspaceID, c.UserKey)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
