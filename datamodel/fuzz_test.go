package datamodel

import (
	"bytes"
	"testing"
)

// fuzzLimits mirrors the Willow'25 parameter bundle (4096/4096/4096) without
// importing willow25 (which would be an import cycle).
var fuzzLimits = Limits{MaxComponentLength: 4096, MaxComponentCount: 4096, MaxPathLength: 4096}

// FuzzDecodePath feeds arbitrary bytes to the absolute Path decoder. It must
// never panic on attacker-supplied input (this is the regression net for the
// 2^64-1 component-length panic the upstream willow_test_vectors corpus found)
// and must respect the configured limits.
//
// The decoder is lenient: it accepts non-minimal encodings, so the bytes it
// consumes need not equal the canonical re-encoding. The invariant that does
// hold is encode-idempotence: re-encoding an accepted path yields a canonical
// form that decodes back to an equal path and re-encodes to the same bytes.
func FuzzDecodePath(f *testing.F) {
	for _, p := range []Path{
		EmptyPath(fuzzLimits),
		mustSeed(f, [][]byte{{}}),
		mustSeed(f, [][]byte{{0x01}}),
		mustSeed(f, [][]byte{{0x61, 0x62}, {0x63}}),
		mustSeed(f, [][]byte{{0x00}, {0x00}, {0x00}}),
	} {
		f.Add(p.Encode())
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		p, n, err := Decode(fuzzLimits, data)
		if err != nil {
			return
		}
		if n < 0 || n > len(data) {
			t.Fatalf("consumed %d out of bounds for %d-byte input", n, len(data))
		}
		if p.ComponentCount() > fuzzLimits.MaxComponentCount {
			t.Fatalf("decoded %d components over limit %d", p.ComponentCount(), fuzzLimits.MaxComponentCount)
		}
		if p.TotalLength() > fuzzLimits.MaxPathLength {
			t.Fatalf("decoded total length %d over limit %d", p.TotalLength(), fuzzLimits.MaxPathLength)
		}

		// encode-idempotence: the canonical re-encoding decodes back to an
		// equal path, consuming all of itself, and re-encodes identically.
		enc1 := p.Encode()
		p2, n2, err := Decode(fuzzLimits, enc1)
		if err != nil {
			t.Fatalf("re-decode of canonical encoding failed: %v (bytes %x)", err, enc1)
		}
		if n2 != len(enc1) {
			t.Fatalf("canonical encoding not fully consumed: %d of %d", n2, len(enc1))
		}
		if enc2 := p2.Encode(); !bytes.Equal(enc1, enc2) {
			t.Fatalf("encode not idempotent: %x vs %x", enc1, enc2)
		}
	})
}

// FuzzDecodeExtending feeds arbitrary bytes to the relative path-extends-path
// decoder against a fixed prefix. It must never panic, and accepted paths must
// satisfy the same encode-idempotence property relative to that prefix.
func FuzzDecodeExtending(f *testing.F) {
	prefix := mustSeed(f, [][]byte{{0x61}, {0x62}})
	for _, suffix := range []Path{
		prefix,
		mustSeed(f, [][]byte{{0x61}, {0x62}, {0x63}}),
	} {
		f.Add(suffix.EncodeExtending(prefix))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		p, n, err := DecodeExtending(prefix, data)
		if err != nil {
			return
		}
		if n < 0 || n > len(data) {
			t.Fatalf("consumed %d out of bounds for %d-byte input", n, len(data))
		}

		enc1 := p.EncodeExtending(prefix)
		p2, n2, err := DecodeExtending(prefix, enc1)
		if err != nil {
			t.Fatalf("re-decode of canonical encoding failed: %v (bytes %x)", err, enc1)
		}
		if n2 != len(enc1) {
			t.Fatalf("canonical encoding not fully consumed: %d of %d", n2, len(enc1))
		}
		if enc2 := p2.EncodeExtending(prefix); !bytes.Equal(enc1, enc2) {
			t.Fatalf("encode not idempotent: %x vs %x", enc1, enc2)
		}
	})
}

func mustSeed(f *testing.F, comps [][]byte) Path {
	f.Helper()
	p, err := FromSlices(fuzzLimits, comps)
	if err != nil {
		f.Fatalf("seed path: %v", err)
	}
	return p
}
