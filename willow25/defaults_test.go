// SPDX-FileCopyrightText: 2026 Ian Chechin
// SPDX-License-Identifier: MIT

package willow25

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestUpstream_Defaults checks the default values against the upstream
// willow_test_vectors constants. The default payload digest is WILLIAM3 of
// the empty string, so its vector checks HashPayload(nil).
func TestUpstream_Defaults(t *testing.T) {
	dir := filepath.Join("..", "testdata", "upstream_vectors", "data_model")
	if entries, err := os.ReadDir(dir); err != nil || len(entries) == 0 {
		t.Skip("upstream_vectors submodule not initialized; run `git submodule update --init` to enable")
	}
	ns, ss, digest := DefaultNamespaceID(), DefaultSubspaceID(), HashPayload(nil)
	for _, c := range []struct {
		name string
		got  []byte
	}{
		{"namespace_id_default", ns[:]},
		{"subspace_id_default", ss[:]},
		{"payload_digest_default", digest[:]},
	} {
		want, err := os.ReadFile(filepath.Join(dir, c.name, "value"))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !bytes.Equal(c.got, want) {
			t.Errorf("%s: got %x, want %x", c.name, c.got, want)
		}
	}
}
