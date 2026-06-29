package willow25

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// william3vectors.txt is copied verbatim from bab_rs 0.8.0
// (https://codeberg.org/worm-blossom/bab_rs, william3vectors.txt). It is the
// upstream ground truth for the corrected WILLIAM3. Parsing it here pins our
// implementation directly to the reference rather than to self-generated
// expectations.
//
// Format per block:
//
//	input (N bytes):
//	[comma-separated byte values]
//
//	digest:
//	[comma-separated byte values]
//
//	======

var william3VectorRe = regexp.MustCompile(`(?s)input \((\d+) bytes\):\s*(\[.*?\])\s*digest:\s*(\[.*?\])`)

func parseByteList(s string) []byte {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]byte, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		out = append(out, byte(n))
	}
	return out
}

func TestWilliam3_UpstreamVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "william3", "william3vectors.txt"))
	if err != nil {
		t.Fatalf("read upstream vectors: %v", err)
	}
	checked := 0
	for _, block := range strings.Split(string(raw), "======") {
		m := william3VectorRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		input := parseByteList(m[2])
		want := parseByteList(m[3])
		if len(want) != PayloadDigestWidth {
			t.Fatalf("vector for %d-byte input has %d-byte digest", len(input), len(want))
		}
		got := William3Sum(input)
		for i := 0; i < PayloadDigestWidth; i++ {
			if got[i] != want[i] {
				t.Fatalf("WILLIAM3 mismatch for %d-byte input:\n got %x\nwant %x", len(input), got[:], want)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no vectors parsed")
	}
	t.Logf("verified %d upstream WILLIAM3 vectors", checked)
}
