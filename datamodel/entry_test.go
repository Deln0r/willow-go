package datamodel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type entryParams struct {
	MCL                int `json:"mcl"`
	MCC                int `json:"mcc"`
	MPL                int `json:"mpl"`
	NamespaceIDWidth   int `json:"namespace_id_width"`
	SubspaceIDWidth    int `json:"subspace_id_width"`
	PayloadDigestWidth int `json:"payload_digest_width"`
}

type entryCase struct {
	Name              string   `json:"name"`
	NamespaceIDHex    string   `json:"namespace_id_hex"`
	SubspaceIDHex     string   `json:"subspace_id_hex"`
	PathComponentsHex []string `json:"path_components_hex"`
	Timestamp         uint64   `json:"timestamp"`
	PayloadLength     uint64   `json:"payload_length"`
	PayloadDigestHex  string   `json:"payload_digest_hex"`
	EncodedHex        string   `json:"encoded_hex"`
}

type entryFile struct {
	Params entryParams `json:"params"`
	Cases  []entryCase `json:"cases"`
}

func loadEntries(t *testing.T, name string) entryFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "entries", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var f entryFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return f
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex %q: %v", s, err)
	}
	return b
}

func entriesSpecFromParams(p entryParams) EntrySpec {
	return EntrySpec{
		Limits: Limits{
			MaxComponentLength: p.MCL,
			MaxComponentCount:  p.MCC,
			MaxPathLength:      p.MPL,
		},
		NamespaceIDLength:   p.NamespaceIDWidth,
		SubspaceIDLength:    p.SubspaceIDWidth,
		PayloadDigestLength: p.PayloadDigestWidth,
	}
}

func TestEntry_Fixtures(t *testing.T) {
	t.Parallel()
	file := loadEntries(t, "basic.json")
	spec := entriesSpecFromParams(file.Params)

	for _, c := range file.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			pathComps := decodeHexComponents(t, c.PathComponentsHex)
			path, err := FromSlices(spec.Limits, pathComps)
			if err != nil {
				t.Fatalf("FromSlices: %v", err)
			}
			entry := Entry{
				NamespaceID:   mustHex(t, c.NamespaceIDHex),
				SubspaceID:    mustHex(t, c.SubspaceIDHex),
				Path:          path,
				Timestamp:     c.Timestamp,
				PayloadLength: c.PayloadLength,
				PayloadDigest: mustHex(t, c.PayloadDigestHex),
			}
			want := mustHex(t, c.EncodedHex)

			got := entry.Encode()
			if !bytes.Equal(got, want) {
				t.Errorf("Encode mismatch\n got: %x\nwant: %x", got, want)
			}

			decoded, n, err := DecodeEntry(spec, want)
			if err != nil {
				t.Fatalf("DecodeEntry: %v", err)
			}
			if n != len(want) {
				t.Errorf("DecodeEntry consumed %d bytes, want %d", n, len(want))
			}
			if !bytes.Equal(decoded.NamespaceID, entry.NamespaceID) {
				t.Errorf("namespace mismatch: got %x want %x", decoded.NamespaceID, entry.NamespaceID)
			}
			if !bytes.Equal(decoded.SubspaceID, entry.SubspaceID) {
				t.Errorf("subspace mismatch: got %x want %x", decoded.SubspaceID, entry.SubspaceID)
			}
			if !decoded.Path.Equal(entry.Path) {
				t.Errorf("path mismatch")
			}
			if decoded.Timestamp != entry.Timestamp {
				t.Errorf("timestamp: got %d want %d", decoded.Timestamp, entry.Timestamp)
			}
			if decoded.PayloadLength != entry.PayloadLength {
				t.Errorf("payload_length: got %d want %d", decoded.PayloadLength, entry.PayloadLength)
			}
			if !bytes.Equal(decoded.PayloadDigest, entry.PayloadDigest) {
				t.Errorf("payload_digest mismatch")
			}
		})
	}
}
