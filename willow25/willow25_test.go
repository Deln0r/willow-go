package willow25

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
)

type william3Case struct {
	Name      string `json:"name"`
	InputHex  string `json:"input_hex"`
	DigestHex string `json:"digest_hex"`
}

type william3File struct {
	ChunkSize   int            `json:"chunk_size"`
	DigestWidth int            `json:"digest_width"`
	Cases       []william3Case `json:"cases"`
}

func TestWilliam3_Fixtures(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "william3", "digests.json"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f william3File
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range f.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			input, err := hex.DecodeString(c.InputHex)
			if err != nil {
				t.Fatalf("decode input hex: %v", err)
			}
			want, err := hex.DecodeString(c.DigestHex)
			if err != nil {
				t.Fatalf("decode digest hex: %v", err)
			}
			got := HashPayload(input)
			if !bytes.Equal(got[:], want) {
				t.Errorf("WILLIAM3 mismatch\n got: %x\nwant: %x", got[:], want)
			}
		})
	}
}

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

func TestHashPayload_EmptyDifferentFromBlake3(t *testing.T) {
	t.Parallel()
	// WILLIAM3(empty) is the bab_rs upstream value, distinct from BLAKE3's
	// af1349b9... empty-input digest. Catches accidental regressions to
	// vanilla BLAKE3.
	wantHex := "3b638fc8f2fb68418325a36b4718ffb07de457ac301393a845466a79eea3286b"
	got := HashPayload(nil)
	gotHex := hex.EncodeToString(got[:])
	if gotHex != wantHex {
		t.Errorf("WILLIAM3(empty):\n got %s\nwant %s", gotHex, wantHex)
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
