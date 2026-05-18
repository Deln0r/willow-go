package meadowcap

import (
	"crypto/ed25519"
	"errors"

	"github.com/Deln0r/willow-go/datamodel"
)

// Errors returned by delegation operations.
var (
	ErrAreaNotIncluded     = errors.New("meadowcap: new area is not included in previous granted area")
	ErrDelegationSignature = errors.New("meadowcap: delegation signature does not verify")
	ErrPrivateKeyMismatch  = errors.New("meadowcap: private key does not correspond to current receiver")
)

// AppendDelegation signs (newArea, newUserKey) using prevPrivateKey (which
// must correspond to c.Receiver()) and appends the delegation to c. Returns
// ErrAreaNotIncluded if newArea is not contained in the current granted
// area, or ErrPrivateKeyMismatch if prevPrivateKey's public component does
// not equal the current receiver.
//
// The newArea may have any subspace, but for a communal-rooted cap the
// inclusion check enforces that the subspace equals the genesis user_key
// (transitively, because the initial granted area is SubspaceArea(user_key)).
func (c *CommunalCapability) AppendDelegation(
	prevPrivateKey ed25519.PrivateKey,
	newArea datamodel.Area,
	newUserKey ed25519.PublicKey,
) error {
	if len(prevPrivateKey) != ed25519.PrivateKeySize {
		return ErrPrivateKeyMismatch
	}
	if len(newUserKey) != ed25519.PublicKeySize {
		return ErrInvalidUserKey
	}

	// Private key derives the corresponding public key via the second half
	// of the seed.PrivateKey representation (ed25519.PrivateKey is
	// seed||pub). Use ed25519.PrivateKey.Public() for the canonical check.
	prevPub, ok := prevPrivateKey.Public().(ed25519.PublicKey)
	if !ok {
		return ErrPrivateKeyMismatch
	}
	if !bytesEqual(prevPub, c.Receiver()) {
		return ErrPrivateKeyMismatch
	}

	limits := newArea.PathPrefix.Limits()
	currentArea := c.GrantedArea(limits)
	if !areaIncludes(currentArea, newArea) {
		return ErrAreaNotIncluded
	}

	handover := c.handoverBytesForAppend(newArea, newUserKey)
	signature := ed25519.Sign(prevPrivateKey, handover)

	c.Delegations = append(c.Delegations, Delegation{
		Area:       newArea,
		NewUserKey: append(ed25519.PublicKey(nil), newUserKey...),
		Signature:  signature,
	})
	return nil
}

// IsValid reports whether c is a structurally and cryptographically valid
// capability: each delegation's Area is included in the previous granted
// area, and each delegation's Signature verifies under the previous
// receiver's public key over the canonical handover bytes.
func (c CommunalCapability) IsValid() bool {
	if len(c.NamespaceKey) != ed25519.PublicKeySize || len(c.UserKey) != ed25519.PublicKeySize {
		return false
	}

	prevReceiver := ed25519.PublicKey(c.UserKey)
	for i, deleg := range c.Delegations {
		if len(deleg.NewUserKey) != ed25519.PublicKeySize {
			return false
		}
		if len(deleg.Signature) != ed25519.SignatureSize {
			return false
		}

		limits := deleg.Area.PathPrefix.Limits()
		var prevArea datamodel.Area
		if i == 0 {
			prevArea = datamodel.SubspaceArea(limits, c.UserKey)
		} else {
			prevArea = c.Delegations[i-1].Area
		}
		if !areaIncludes(prevArea, deleg.Area) {
			return false
		}

		handover := c.handoverBytesAt(i)
		if !ed25519.Verify(prevReceiver, handover, deleg.Signature) {
			return false
		}
		prevReceiver = deleg.NewUserKey
	}
	return true
}

// handoverBytesForAppend computes the bytes to sign for appending a new
// delegation with (newArea, newUserKey) to the current chain.
func (c CommunalCapability) handoverBytesForAppend(
	newArea datamodel.Area,
	newUserKey ed25519.PublicKey,
) []byte {
	limits := newArea.PathPrefix.Limits()
	var prevArea datamodel.Area
	if n := len(c.Delegations); n == 0 {
		prevArea = datamodel.SubspaceArea(limits, c.UserKey)
		return c.firstHandoverBytes(prevArea, newArea, newUserKey)
	} else {
		prev := c.Delegations[n-1]
		return c.subsequentHandoverBytes(prev, newArea, newUserKey)
	}
}

// handoverBytesAt computes the bytes that should have been signed by
// delegations[i-1].NewUserKey (or genesis.UserKey if i == 0) over
// delegations[i].
func (c CommunalCapability) handoverBytesAt(i int) []byte {
	deleg := c.Delegations[i]
	limits := deleg.Area.PathPrefix.Limits()
	if i == 0 {
		prevArea := datamodel.SubspaceArea(limits, c.UserKey)
		return c.firstHandoverBytes(prevArea, deleg.Area, deleg.NewUserKey)
	}
	prev := c.Delegations[i-1]
	return c.subsequentHandoverBytes(prev, deleg.Area, deleg.NewUserKey)
}

// firstHandoverBytes computes the bytes signed for the first delegation off
// a communal genesis: [access_mode] + namespace_key + new_area encoded
// relative to the initial subspace area + new_receiver. Matches the
// upstream create_handover for the Communal+None case.
func (c CommunalCapability) firstHandoverBytes(
	initialArea datamodel.Area,
	newArea datamodel.Area,
	newUserKey ed25519.PublicKey,
) []byte {
	out := make([]byte, 0, 1+ed25519.PublicKeySize+64+ed25519.PublicKeySize)
	out = append(out, modeByte(c.Mode))
	out = append(out, c.NamespaceKey...)
	out = append(out, newArea.EncodeRelativeTo(initialArea)...)
	out = append(out, newUserKey...)
	return out
}

// subsequentHandoverBytes computes the bytes signed for a non-first
// delegation: new_area encoded relative to prev_area + prev_signature +
// new_receiver.
func (c CommunalCapability) subsequentHandoverBytes(
	prev Delegation,
	newArea datamodel.Area,
	newUserKey ed25519.PublicKey,
) []byte {
	out := make([]byte, 0, 64+ed25519.SignatureSize+ed25519.PublicKeySize)
	out = append(out, newArea.EncodeRelativeTo(prev.Area)...)
	out = append(out, prev.Signature...)
	out = append(out, newUserKey...)
	return out
}

func modeByte(m AccessMode) byte {
	if m == AccessModeRead {
		return 0
	}
	return 1
}

// areaIncludes reports whether outer contains inner: outer is a subset
// constraint, inner must be more specific (or equal). This mirrors
// Area.includesArea in datamodel/area.go, exposed locally because that
// method is package-private.
func areaIncludes(outer, inner datamodel.Area) bool {
	if outer.Subspace != nil {
		if inner.Subspace == nil {
			return false
		}
		if !bytesEqual(*outer.Subspace, *inner.Subspace) {
			return false
		}
	}
	if !outer.PathPrefix.IsPrefixOf(inner.PathPrefix) {
		return false
	}
	if inner.Times.Start < outer.Times.Start {
		return false
	}
	if outer.Times.Open {
		return true
	}
	if inner.Times.Open {
		return false
	}
	return inner.Times.End <= outer.Times.End
}
