package encoding

import (
	"bytes"
	"testing"
)

func TestCU64Standalone_RoundTrip(t *testing.T) {
	t.Parallel()
	values := []uint64{
		0, 1, 11, 12, 100, 251, 252, 255,
		256, 257, 65535,
		65536, 1 << 24, (1 << 32) - 1,
		1 << 32, 1<<63 + 7,
	}
	for _, n := range values {
		enc := AppendCU64Standalone(nil, n)
		got, consumed, err := DecodeCU64Standalone(enc, true)
		if err != nil {
			t.Fatalf("decode %d: %v", n, err)
		}
		if consumed != len(enc) {
			t.Fatalf("decode %d: consumed %d of %d bytes", n, consumed, len(enc))
		}
		if got != n {
			t.Fatalf("round-trip %d: got %d", n, got)
		}
	}
}

func TestCU64Standalone_MinimalWidth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n    uint64
		want int // total bytes including tag
	}{
		{0, 1},
		{251, 1},
		{252, 2},
		{255, 2},
		{256, 3},
		{65535, 3},
		{65536, 5},
		{1<<32 - 1, 5},
		{1 << 32, 9},
	}
	for _, tc := range cases {
		enc := AppendCU64Standalone(nil, tc.n)
		if len(enc) != tc.want {
			t.Errorf("standalone %d: got %d bytes, want %d", tc.n, len(enc), tc.want)
		}
	}
}

func TestCU64_PackedFourBitTags(t *testing.T) {
	t.Parallel()
	// Mirrors the compact_u64 README example: two four-bit tags packed into a
	// single header byte, followed by their int encodings.
	n1, n2 := uint64(258), uint64(7)
	var header uint8
	header = WriteTag(header, 4, 0, n1)
	header = WriteTag(header, 4, 4, n2)
	if header != 0xD7 {
		t.Fatalf("header: got 0x%02X, want 0xD7", header)
	}
	buf := []byte{header}
	buf = AppendCU64(buf, n1, 4)
	buf = AppendCU64(buf, n2, 4)
	if !bytes.Equal(buf, []byte{0xD7, 1, 2}) {
		t.Fatalf("packed encoding: got %x, want d70102", buf)
	}

	tag1 := ExtractTag(header, 4, 0)
	tag2 := ExtractTag(header, 4, 4)
	if tag1 != 0xD || tag2 != 0x7 {
		t.Fatalf("extract: got tag1=0x%X tag2=0x%X", tag1, tag2)
	}
	got1, c1, err := DecodeCU64(tag1, 4, buf[1:], true)
	if err != nil {
		t.Fatalf("decode n1: %v", err)
	}
	got2, c2, err := DecodeCU64(tag2, 4, buf[1+c1:], true)
	if err != nil {
		t.Fatalf("decode n2: %v", err)
	}
	if got1 != n1 || got2 != n2 {
		t.Fatalf("decoded: got %d %d, want %d %d", got1, got2, n1, n2)
	}
	if c1+c2 != 2 {
		t.Fatalf("packed consumed: got %d, want 2", c1+c2)
	}
}

func TestCU64_RejectsNonMinimal(t *testing.T) {
	t.Parallel()
	// Standalone tag 0xFE indicates a 4-byte int encoding. A value <256 in
	// that form is non-minimal.
	nonMinimal := []byte{0xFE, 0, 0, 0, 5}
	if _, _, err := DecodeCU64Standalone(nonMinimal, true); err != ErrNotMinimal {
		t.Fatalf("expected ErrNotMinimal, got %v", err)
	}
	// Same buffer, non-canonical mode: should decode to 5.
	v, _, err := DecodeCU64Standalone(nonMinimal, false)
	if err != nil || v != 5 {
		t.Fatalf("non-canonical: got v=%d err=%v", v, err)
	}
}

func TestCU64_ShortBuffer(t *testing.T) {
	t.Parallel()
	// Tag 0xFD = 2-byte follow, but only 1 byte after.
	if _, _, err := DecodeCU64Standalone([]byte{0xFD, 0}, false); err != ErrShortBuffer {
		t.Fatalf("expected ErrShortBuffer, got %v", err)
	}
}
