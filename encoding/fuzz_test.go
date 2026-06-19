package encoding

import "testing"

// FuzzDecodeCU64Standalone feeds arbitrary bytes to the standalone CompactU64
// decoder. The decoder must never panic on attacker-supplied input, must
// report a consumed length within the buffer, and on a canonical decode the
// value must re-encode to exactly the bytes it consumed.
func FuzzDecodeCU64Standalone(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
		{0x00},
		{0x01, 0xff},
		{0xfc, 0x01, 0x00},
		{0xfd, 0xff, 0xff},
		{0xfe, 0x01, 0x00, 0x00, 0x00},
		{0xff, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // 2^64-1
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Non-canonical decode: must not panic; on success, consumed length
		// must be within the buffer.
		if v, n, err := DecodeCU64Standalone(data, false); err == nil {
			if n < 1 || n > len(data) {
				t.Fatalf("consumed %d out of bounds for %d-byte input (value %d)", n, len(data), v)
			}
		}

		// Canonical decode: on success the value must round-trip back to the
		// exact bytes consumed.
		if v, n, err := DecodeCU64Standalone(data, true); err == nil {
			if n < 1 || n > len(data) {
				t.Fatalf("canonical consumed %d out of bounds for %d-byte input", n, len(data))
			}
			reencoded := AppendCU64Standalone(nil, v)
			if len(reencoded) != n {
				t.Fatalf("canonical decode of value %d consumed %d bytes but re-encodes to %d", v, n, len(reencoded))
			}
			for i := 0; i < n; i++ {
				if reencoded[i] != data[i] {
					t.Fatalf("canonical round-trip mismatch for value %d at byte %d: got %#x want %#x", v, i, reencoded[i], data[i])
				}
			}
		}
	})
}
