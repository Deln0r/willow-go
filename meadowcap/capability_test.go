package meadowcap

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
)

var testLimits = datamodel.Limits{MaxComponentLength: 64, MaxComponentCount: 16, MaxPathLength: 256}

// makeKeypair returns a deterministic Ed25519 keypair seeded by the given
// label, so tests are reproducible.
func makeKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, priv
}

func makeEntry(t *testing.T, ns, sub []byte, comps []string, ts uint64, plen uint64, digest []byte) datamodel.Entry {
	t.Helper()
	byteComps := make([][]byte, len(comps))
	for i, c := range comps {
		byteComps[i] = []byte(c)
	}
	p, err := datamodel.FromSlices(testLimits, byteComps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	return datamodel.Entry{
		NamespaceID:   ns,
		SubspaceID:    sub,
		Path:          p,
		Timestamp:     ts,
		PayloadLength: plen,
		PayloadDigest: digest,
	}
}

func TestNewCommunal_KeyValidation(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, _ := makeKeypair(t)

	if _, err := NewCommunal(AccessModeWrite, ns, user); err != nil {
		t.Errorf("valid keys: %v", err)
	}
	if _, err := NewCommunal(AccessModeWrite, ns[:31], user); !errors.Is(err, ErrInvalidNamespaceKey) {
		t.Errorf("short ns key: got %v, want ErrInvalidNamespaceKey", err)
	}
	if _, err := NewCommunal(AccessModeWrite, ns, user[:31]); !errors.Is(err, ErrInvalidUserKey) {
		t.Errorf("short user key: got %v, want ErrInvalidUserKey", err)
	}
}

func TestCommunalCapability_DefensiveCopy(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, _ := makeKeypair(t)
	cap, err := NewCommunal(AccessModeWrite, ns, user)
	if err != nil {
		t.Fatal(err)
	}
	// Mutating the input must not affect the stored cap.
	originalNs := append([]byte(nil), ns...)
	ns[0] ^= 0xFF
	if !bytes.Equal(cap.NamespaceKey, originalNs) {
		t.Errorf("NewCommunal did not defensively copy NamespaceKey")
	}
}

func TestCommunalCapability_IncludesEntry(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, _ := makeKeypair(t)
	otherNs, _ := makeKeypair(t)
	otherUser, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, ns, user)
	if err != nil {
		t.Fatal(err)
	}

	digest := make([]byte, 32)
	matching := makeEntry(t, ns, user, []string{"folder", "file"}, 100, 17, digest)
	wrongNs := makeEntry(t, otherNs, user, []string{"folder", "file"}, 100, 17, digest)
	wrongSubspace := makeEntry(t, ns, otherUser, []string{"folder", "file"}, 100, 17, digest)

	if !cap.IncludesEntry(matching) {
		t.Error("cap should include matching entry")
	}
	if cap.IncludesEntry(wrongNs) {
		t.Error("cap should not include entry with different namespace")
	}
	if cap.IncludesEntry(wrongSubspace) {
		t.Error("cap should not include entry with different subspace")
	}
}

func TestCommunalCapability_GrantedArea(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, _ := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	area := cap.GrantedArea(testLimits)
	if area.Subspace == nil || !bytes.Equal(*area.Subspace, user) {
		t.Errorf("GrantedArea subspace = %v, want user key", area.Subspace)
	}
	if !area.PathPrefix.IsEmpty() {
		t.Error("GrantedArea path prefix should be empty")
	}
	if !area.Times.IsFull() {
		t.Error("GrantedArea times should be full")
	}
}

func TestAuthorisationToken_HappyPath(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, userPriv := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	digest := make([]byte, 32)
	entry := makeEntry(t, ns, user, []string{"folder", "file"}, 100, 17, digest)

	token, err := NewAuthorisationToken(cap, userPriv, entry)
	if err != nil {
		t.Fatalf("NewAuthorisationToken: %v", err)
	}
	if err := token.Verify(entry); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestAuthorisationToken_RejectsOutOfScopeEntry(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, userPriv := makeKeypair(t)
	otherUser, _ := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	digest := make([]byte, 32)
	outOfScope := makeEntry(t, ns, otherUser, []string{"folder"}, 100, 17, digest)

	_, err := NewAuthorisationToken(cap, userPriv, outOfScope)
	if !errors.Is(err, ErrCapabilityRejectsEntry) {
		t.Errorf("NewAuthorisationToken with out-of-scope entry: got %v, want ErrCapabilityRejectsEntry", err)
	}
}

func TestAuthorisationToken_RejectsForgedSignature(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, _ := makeKeypair(t)
	_, attackerPriv := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	digest := make([]byte, 32)
	entry := makeEntry(t, ns, user, []string{"folder"}, 100, 17, digest)

	// Attacker (not the receiver) signs the entry, then ships the token.
	forgedSig := ed25519.Sign(attackerPriv, entry.Encode())
	token := AuthorisationToken{Capability: cap, Signature: forgedSig}

	if err := token.Verify(entry); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("forged sig: got %v, want ErrInvalidSignature", err)
	}
}

func TestAuthorisationToken_RejectsTamperedEntry(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, userPriv := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	digest := make([]byte, 32)
	originalEntry := makeEntry(t, ns, user, []string{"folder", "file"}, 100, 17, digest)
	token, err := NewAuthorisationToken(cap, userPriv, originalEntry)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper: change the timestamp post-signing.
	tamperedEntry := originalEntry
	tamperedEntry.Timestamp = 200
	if err := token.Verify(tamperedEntry); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("tampered entry: got %v, want ErrInvalidSignature", err)
	}
}

func TestAuthorisationToken_RejectsShortSignature(t *testing.T) {
	t.Parallel()
	ns, _ := makeKeypair(t)
	user, userPriv := makeKeypair(t)
	cap, _ := NewCommunal(AccessModeWrite, ns, user)

	digest := make([]byte, 32)
	entry := makeEntry(t, ns, user, []string{"folder"}, 100, 17, digest)

	token, _ := NewAuthorisationToken(cap, userPriv, entry)
	token.Signature = token.Signature[:32] // truncate

	if err := token.Verify(entry); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("short sig: got %v, want ErrInvalidSignature", err)
	}
}
