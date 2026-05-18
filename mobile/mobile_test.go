package mobile

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/willow25"
)

func TestHashPayload(t *testing.T) {
	t.Parallel()
	got := HashPayload([]byte("hello"))
	if len(got) != 32 {
		t.Errorf("digest length = %d, want 32", len(got))
	}
	want := willow25.HashPayload([]byte("hello"))
	if !bytes.Equal(got, want[:]) {
		t.Errorf("digest mismatch")
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	b := Limits()
	if len(b) != 12 {
		t.Fatalf("len = %d, want 12", len(b))
	}
	mcl := binary.BigEndian.Uint32(b[0:4])
	mcc := binary.BigEndian.Uint32(b[4:8])
	mpl := binary.BigEndian.Uint32(b[8:12])
	if mcl != 4096 || mcc != 4096 || mpl != 4096 {
		t.Errorf("got mcl/mcc/mpl = %d/%d/%d, want 4096/4096/4096", mcl, mcc, mpl)
	}
}

func TestPathBuilder(t *testing.T) {
	t.Parallel()
	b := NewPathBuilder()
	if b.ComponentCount() != 0 {
		t.Error("fresh builder should have 0 components")
	}
	b.AddComponent([]byte("folder"))
	b.AddComponent([]byte("file"))
	if b.ComponentCount() != 2 {
		t.Errorf("after 2 AddComponent: got %d, want 2", b.ComponentCount())
	}

	encoded, err := b.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Verify by decoding through the underlying Go API.
	decoded, n, err := datamodel.Decode(willow25.Limits(), encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d/%d", n, len(encoded))
	}
	if decoded.ComponentCount() != 2 {
		t.Errorf("decoded count = %d, want 2", decoded.ComponentCount())
	}
	if !bytes.Equal(decoded.Component(0), []byte("folder")) || !bytes.Equal(decoded.Component(1), []byte("file")) {
		t.Errorf("decoded components mismatch")
	}
}

func TestEntryBuilder_HappyPath(t *testing.T) {
	t.Parallel()
	ns := bytes.Repeat([]byte{0x11}, 32)
	sub := bytes.Repeat([]byte{0x22}, 32)
	digest := bytes.Repeat([]byte{0x33}, 32)

	b := NewEntryBuilder()
	b.SetNamespaceID(ns)
	b.SetSubspaceID(sub)
	b.AddPathComponent([]byte("hello"))
	b.AddPathComponent([]byte("world"))
	b.SetTimestamp(1234567)
	b.SetPayloadLength(42)
	b.SetPayloadDigest(digest)

	encoded, err := b.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Round-trip through datamodel.DecodeEntry.
	decoded, n, err := datamodel.DecodeEntry(willow25.EntrySpec(), encoded)
	if err != nil {
		t.Fatalf("DecodeEntry: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d/%d", n, len(encoded))
	}
	if !bytes.Equal(decoded.NamespaceID, ns) || !bytes.Equal(decoded.SubspaceID, sub) {
		t.Error("id mismatch")
	}
	if decoded.Timestamp != 1234567 || decoded.PayloadLength != 42 {
		t.Errorf("timestamp/length mismatch: %d / %d", decoded.Timestamp, decoded.PayloadLength)
	}
	if !bytes.Equal(decoded.PayloadDigest, digest) {
		t.Error("digest mismatch")
	}
}

func TestEntryBuilder_RejectsWrongIDSize(t *testing.T) {
	t.Parallel()
	b := NewEntryBuilder()
	b.SetNamespaceID([]byte{0x11}) // too short
	b.SetSubspaceID(bytes.Repeat([]byte{0x22}, 32))
	b.SetPayloadDigest(bytes.Repeat([]byte{0x33}, 32))
	if _, err := b.Encode(); err == nil {
		t.Error("expected error for short namespace id")
	}
}

func TestHashAndEncodeEntry(t *testing.T) {
	t.Parallel()
	ns := bytes.Repeat([]byte{0x11}, 32)
	sub := bytes.Repeat([]byte{0x22}, 32)
	payload := []byte("the quick brown fox")

	pb := NewPathBuilder()
	pb.AddComponent([]byte("notes"))
	pb.AddComponent([]byte("today.txt"))

	encoded, err := HashAndEncodeEntry(ns, sub, pb, 99, payload)
	if err != nil {
		t.Fatalf("HashAndEncodeEntry: %v", err)
	}
	decoded, _, err := datamodel.DecodeEntry(willow25.EntrySpec(), encoded)
	if err != nil {
		t.Fatalf("DecodeEntry: %v", err)
	}
	if decoded.PayloadLength != uint64(len(payload)) {
		t.Errorf("payload length mismatch: got %d want %d", decoded.PayloadLength, len(payload))
	}
	expected := willow25.HashPayload(payload)
	if !bytes.Equal(decoded.PayloadDigest, expected[:]) {
		t.Error("payload digest mismatch")
	}
}
