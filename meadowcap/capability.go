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

// CommunalCapability is a Meadowcap capability rooted in a communal genesis.
// A communal genesis binds a user_key to a subspace within a namespace; the
// granted area at that point is SubspaceArea(user_key). Subsequent
// Delegations narrow the granted area and hand control to a new user.
//
// The "receiver" of the cap is the genesis user_key if Delegations is empty,
// otherwise the final delegation's NewUserKey. All keys are Ed25519 public
// keys (32 bytes), matching willow25.
type CommunalCapability struct {
	Mode         AccessMode
	NamespaceKey ed25519.PublicKey
	UserKey      ed25519.PublicKey
	Delegations  []Delegation
}

// Delegation is a single step in a Meadowcap delegation chain: the previous
// receiver signs over (Area, NewUserKey) using their private key,
// authorising NewUserKey to receive caps within Area. Validation walks the
// chain and verifies each signature against the appropriate previous
// receiver's key, plus checks that each Area is included in the previous
// step's Area.
type Delegation struct {
	Area       datamodel.Area
	NewUserKey ed25519.PublicKey
	Signature  []byte // 64-byte Ed25519 signature over the delegation handover bytes
}

// NewCommunal returns a new communal capability with no delegations.
// Returns an error if either key has the wrong length for Ed25519.
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

// Receiver returns the public key of the capability's effective receiver —
// the entity authorised to sign entries under this cap. With no delegations
// this equals GenesisUserKey; with delegations it is the most recent
// NewUserKey.
func (c CommunalCapability) Receiver() ed25519.PublicKey {
	if n := len(c.Delegations); n > 0 {
		return c.Delegations[n-1].NewUserKey
	}
	return c.UserKey
}

// GrantedArea returns the area within which this capability currently
// grants access. With no delegations: the full subspace area belonging to
// the genesis user_key. With delegations: the area of the most recent
// delegation.
func (c CommunalCapability) GrantedArea(limits datamodel.Limits) datamodel.Area {
	if n := len(c.Delegations); n > 0 {
		return c.Delegations[n-1].Area
	}
	return datamodel.SubspaceArea(limits, c.UserKey)
}

// IncludesEntry reports whether c grants access to the given entry: the
// entry's namespace must match the capability's namespace, and the entry's
// coordinate (subspace, path, timestamp) must fall within the currently
// granted area.
func (c CommunalCapability) IncludesEntry(e datamodel.Entry) bool {
	if !bytesEqual(e.NamespaceID, c.NamespaceKey) {
		return false
	}
	limits := e.Path.Limits()
	area := c.GrantedArea(limits)
	return area.IncludesEntry(e)
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
