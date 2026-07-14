package encoding

import (
	"testing"
	"testing/quick"
)

// TestCU64_QuickRoundTrip is a property-based check that the packed CompactU64
// codec round-trips any uint64 at every valid tag width (2..8): encoding a
// value with its minimal tag and decoding it back yields the same value, and
// the reported width matches the bytes written.
func TestCU64_QuickRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	for tw := uint8(2); tw <= 8; tw++ {
		tw := tw
		f := func(n uint64) bool {
			tag := MinTag(n, tw)
			enc := AppendCU64(nil, n, tw)
			got, width, err := DecodeCU64(tag, tw, enc, true)
			return err == nil && got == n && width == len(enc)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Errorf("tag width %d: %v", tw, err)
		}
	}
}

// TestCU64Standalone_QuickRoundTrip is the same property for the 8-bit
// standalone framing (tag byte followed by its value bytes).
func TestCU64Standalone_QuickRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	f := func(n uint64) bool {
		enc := AppendCU64Standalone(nil, n)
		got, consumed, err := DecodeCU64Standalone(enc, true)
		return err == nil && got == n && consumed == len(enc)
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
