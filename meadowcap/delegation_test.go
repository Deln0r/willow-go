package meadowcap

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
)

// pathOfStrings builds a Path from string components under testLimits.
func pathOfStrings(t *testing.T, comps ...string) datamodel.Path {
	t.Helper()
	byteComps := make([][]byte, len(comps))
	for i, c := range comps {
		byteComps[i] = []byte(c)
	}
	p, err := datamodel.FromSlices(testLimits, byteComps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	return p
}

func subspaceArea(t *testing.T, subspace []byte, pathPrefix datamodel.Path, times datamodel.TimeRange) datamodel.Area {
	t.Helper()
	subCopy := append([]byte(nil), subspace...)
	return datamodel.Area{
		Subspace:   &subCopy,
		PathPrefix: pathPrefix,
		Times:      times,
	}
}

func TestDelegation_AppendThenValid(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}

	// Delegate a narrower area within the root user's subspace.
	subArea := subspaceArea(t, rootPub, pathOfStrings(t, "folder"), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, subArea, delegatePub); err != nil {
		t.Fatalf("AppendDelegation: %v", err)
	}

	if !cap.IsValid() {
		t.Error("cap should be valid after honest AppendDelegation")
	}
	if got := cap.Receiver(); !ed25519.PublicKey(got).Equal(delegatePub) {
		t.Errorf("Receiver after delegation: got %x, want %x", got, delegatePub)
	}
	area := cap.GrantedArea(testLimits)
	if area.PathPrefix.ComponentCount() != 1 {
		t.Errorf("GrantedArea path prefix should be [folder], got %v components", area.PathPrefix.ComponentCount())
	}
}

func TestDelegation_MultiStepChain(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	bobPub, bobPriv := makeKeypair(t)
	carolPub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}

	// Root -> Bob with the full root subspace area.
	bobArea := subspaceArea(t, rootPub, datamodel.EmptyPath(testLimits), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, bobArea, bobPub); err != nil {
		t.Fatalf("Root->Bob: %v", err)
	}

	// Bob -> Carol with a narrower path prefix.
	carolArea := subspaceArea(t, rootPub, pathOfStrings(t, "shared"), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(bobPriv, carolArea, carolPub); err != nil {
		t.Fatalf("Bob->Carol: %v", err)
	}

	if !cap.IsValid() {
		t.Error("multi-step chain should be valid")
	}
	if got := cap.Receiver(); !ed25519.PublicKey(got).Equal(carolPub) {
		t.Errorf("Receiver after chain: got %x, want carol", got)
	}
}

func TestDelegation_RejectsAreaNotIncluded(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	otherPub, _ := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}

	// Try to delegate an area in a DIFFERENT subspace - not included.
	outsideArea := subspaceArea(t, otherPub, datamodel.EmptyPath(testLimits), datamodel.FullTimeRange())
	err = cap.AppendDelegation(rootPriv, outsideArea, delegatePub)
	if !errors.Is(err, ErrAreaNotIncluded) {
		t.Errorf("got %v, want ErrAreaNotIncluded", err)
	}
}

func TestDelegation_RejectsWrongPrivateKey(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, _ := makeKeypair(t)
	_, attackerPriv := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}

	subArea := subspaceArea(t, rootPub, datamodel.EmptyPath(testLimits), datamodel.FullTimeRange())
	err = cap.AppendDelegation(attackerPriv, subArea, delegatePub)
	if !errors.Is(err, ErrPrivateKeyMismatch) {
		t.Errorf("got %v, want ErrPrivateKeyMismatch", err)
	}
}

func TestDelegation_TamperedSignatureRejected(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}
	subArea := subspaceArea(t, rootPub, datamodel.EmptyPath(testLimits), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, subArea, delegatePub); err != nil {
		t.Fatal(err)
	}

	// Tamper a byte in the signature.
	cap.Delegations[0].Signature[0] ^= 0xFF
	if cap.IsValid() {
		t.Error("cap with tampered signature should not be valid")
	}
}

func TestDelegation_TamperedDelegationAreaRejected(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, err := NewCommunal(AccessModeWrite, nsPub, rootPub)
	if err != nil {
		t.Fatal(err)
	}
	subArea := subspaceArea(t, rootPub, pathOfStrings(t, "folder"), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, subArea, delegatePub); err != nil {
		t.Fatal(err)
	}

	// Mutate the delegation's path prefix - signature was over the original.
	cap.Delegations[0].Area.PathPrefix = pathOfStrings(t, "other_folder")
	if cap.IsValid() {
		t.Error("cap with tampered delegation area should not be valid")
	}
}

func TestAuthorisationToken_WithDelegatedCap(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	delegatePub, delegatePriv := makeKeypair(t)

	cap, _ := NewCommunal(AccessModeWrite, nsPub, rootPub)
	subArea := subspaceArea(t, rootPub, pathOfStrings(t, "folder"), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, subArea, delegatePub); err != nil {
		t.Fatal(err)
	}

	// Entry within the delegated area, signed by the delegate (current receiver).
	digest := make([]byte, 32)
	entry := makeEntry(t, nsPub, rootPub, []string{"folder", "file"}, 100, 17, digest)
	token, err := NewAuthorisationToken(cap, delegatePriv, entry)
	if err != nil {
		t.Fatalf("NewAuthorisationToken: %v", err)
	}
	if err := token.Verify(entry); err != nil {
		t.Errorf("Verify: %v", err)
	}

	// Entry outside the delegated path prefix.
	outsideEntry := makeEntry(t, nsPub, rootPub, []string{"other"}, 100, 17, digest)
	if _, err := NewAuthorisationToken(cap, delegatePriv, outsideEntry); !errors.Is(err, ErrCapabilityRejectsEntry) {
		t.Errorf("outside delegated area: got %v, want ErrCapabilityRejectsEntry", err)
	}
}

func TestAuthorisationToken_RejectsSignatureByOldReceiver(t *testing.T) {
	t.Parallel()
	nsPub, _ := makeKeypair(t)
	rootPub, rootPriv := makeKeypair(t)
	delegatePub, _ := makeKeypair(t)

	cap, _ := NewCommunal(AccessModeWrite, nsPub, rootPub)
	subArea := subspaceArea(t, rootPub, pathOfStrings(t, "folder"), datamodel.FullTimeRange())
	if err := cap.AppendDelegation(rootPriv, subArea, delegatePub); err != nil {
		t.Fatal(err)
	}

	digest := make([]byte, 32)
	entry := makeEntry(t, nsPub, rootPub, []string{"folder", "file"}, 100, 17, digest)

	// Root signs the entry (but root is no longer the receiver - delegate is).
	forgedSig := ed25519.Sign(rootPriv, entry.Encode())
	token := AuthorisationToken{Capability: cap, Signature: forgedSig}
	if err := token.Verify(entry); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("old receiver sig: got %v, want ErrInvalidSignature", err)
	}
}
