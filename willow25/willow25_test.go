package willow25

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
)

func TestLimitsAndSpec(t *testing.T) {
	t.Parallel()
	l := Limits()
	if l.MaxComponentLength != 4096 || l.MaxComponentCount != 4096 || l.MaxPathLength != 4096 {
		t.Errorf("Limits: got %+v, want all 4096", l)
	}
	s := EntrySpec()
	if s.NamespaceIDLength != 32 || s.SubspaceIDLength != 32 || s.PayloadDigestLength != 32 {
		t.Errorf("EntrySpec widths: got %+v, want all 32", s)
	}
}

func TestHashPayload_KnownVector(t *testing.T) {
	t.Parallel()
	// Empty input BLAKE3-256 digest (the canonical reference vector).
	want := []byte{
		0xaf, 0x13, 0x49, 0xb9, 0xf5, 0xf9, 0xa1, 0xa6,
		0xa0, 0x40, 0x4d, 0xea, 0x36, 0xdc, 0xc9, 0x49,
		0x9b, 0xcb, 0x25, 0xc9, 0xad, 0xc1, 0x12, 0xb7,
		0xcc, 0x9a, 0x93, 0xca, 0xe4, 0x1f, 0x32, 0x62,
	}
	got := HashPayload(nil)
	if !bytes.Equal(got[:], want) {
		t.Errorf("BLAKE3(empty): got %x, want %x", got[:], want)
	}
}

func TestNewEntry_HashesPayload(t *testing.T) {
	t.Parallel()
	ns := bytes.Repeat([]byte{0x11}, 32)
	sub := bytes.Repeat([]byte{0x22}, 32)
	path, err := NewPath([][]byte{[]byte("folder"), []byte("file.txt")})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("the quick brown fox")
	entry, err := NewEntry(ns, sub, path, 12345, payload)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	if entry.PayloadLength != uint64(len(payload)) {
		t.Errorf("PayloadLength: got %d, want %d", entry.PayloadLength, len(payload))
	}
	wantDigest := HashPayload(payload)
	if !bytes.Equal(entry.PayloadDigest, wantDigest[:]) {
		t.Errorf("PayloadDigest mismatch:\n got %x\nwant %x", entry.PayloadDigest, wantDigest[:])
	}
	if !bytes.Equal(entry.NamespaceID, ns) {
		t.Errorf("NamespaceID mismatch")
	}
}

func TestNewEntry_RejectsWrongIDLength(t *testing.T) {
	t.Parallel()
	shortNs := []byte{0x11}
	sub := bytes.Repeat([]byte{0x22}, 32)
	path, _ := NewPath(nil)

	if _, err := NewEntry(shortNs, sub, path, 0, nil); err == nil {
		t.Error("expected error for short namespace id")
	}

	if _, err := NewEntry(bytes.Repeat([]byte{0x11}, 32), shortNs, path, 0, nil); err == nil {
		t.Error("expected error for short subspace id")
	}
}

func TestNewEntry_RoundTripWithEntrySpec(t *testing.T) {
	t.Parallel()
	ns := bytes.Repeat([]byte{0xAA}, 32)
	sub := bytes.Repeat([]byte{0xBB}, 32)
	path, err := NewPath([][]byte{[]byte("a"), []byte("b")})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := NewEntry(ns, sub, path, 999, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	encoded := entry.Encode()
	decoded, n, err := datamodel.DecodeEntry(EntrySpec(), encoded)
	if err != nil {
		t.Fatalf("DecodeEntry: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d bytes, want %d", n, len(encoded))
	}
	if !bytes.Equal(decoded.PayloadDigest, entry.PayloadDigest) {
		t.Error("round-trip digest mismatch")
	}
	if decoded.PayloadLength != entry.PayloadLength {
		t.Errorf("round-trip length: got %d, want %d", decoded.PayloadLength, entry.PayloadLength)
	}
}

func TestNewPath_Limits(t *testing.T) {
	t.Parallel()
	if _, err := NewPath([][]byte{bytes.Repeat([]byte{0x55}, 4097)}); !errors.Is(err, datamodel.ErrComponentTooLong) {
		t.Errorf("4097-byte component: got %v, want ErrComponentTooLong", err)
	}
}
