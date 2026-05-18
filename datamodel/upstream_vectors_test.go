package datamodel

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Upstream test vectors live in testdata/upstream_vectors/, a git submodule
// pointing at https://github.com/worm-blossom/willow_test_vectors. The repo
// publishes positive (yay/) and negative (nay/) test vectors per encoding,
// plus reencoded/ which contains the spec-defined canonical re-encoding of
// each positive case.
//
// We currently adopt the ABSOLUTE path encodings only — encode_path
// (function) and EncodePath (relation). Relative encodings (path_rel_path,
// EncodePathRelativePath, path_extends_path, EncodePathExtendsPath) are
// EXCLUDED from this test runner because their upstream reencoded/ files
// diverge from both willow_rs v0.7.0 (the impl we are byte-compat with)
// and the spec text on willowprotocol.org as of May 2026. See
// TECH_DEBT.md for the audit notes and re-adoption plan.
//
// To initialize the submodule: `git submodule update --init`.

// willow25Limits returns the path size limits used by all upstream test
// vectors (per testdata/upstream_vectors/README: "All data sets use Willow'25
// parameters"). Duplicates willow25.Limits() so this test file does not
// import the willow25 package and create a cycle.
func willow25Limits() Limits {
	return Limits{MaxComponentLength: 4096, MaxComponentCount: 4096, MaxPathLength: 4096}
}

func upstreamDir(name string) string {
	return filepath.Join("..", "testdata", "upstream_vectors", name)
}

// listOrSkip lists files in dir, sorted by name. Returns nil and a skip if
// the directory does not exist (submodule not initialized).
func listOrSkip(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upstream_vectors submodule not initialized (missing %s); run `git submodule update --init` to enable", dir)
		}
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// runAbsolutePathVectors exercises an absolute Path encoding directory
// (encode_path or EncodePath). isFunction=true means re-encoding must
// equal yay/N (the canonical encoding function); false means it must
// equal reencoded/N (the encoding relation, yay may be non-canonical).
func runAbsolutePathVectors(t *testing.T, name string, isFunction bool) {
	t.Helper()
	base := upstreamDir(name)
	yayNames := listOrSkip(t, filepath.Join(base, "yay"))

	limits := willow25Limits()
	var yayPass, yayFail int
	for _, n := range yayNames {
		yayBytes, err := os.ReadFile(filepath.Join(base, "yay", n))
		if err != nil {
			t.Errorf("%s yay/%s: read: %v", name, n, err)
			yayFail++
			continue
		}
		decoded, _, err := Decode(limits, yayBytes)
		if err != nil {
			t.Errorf("%s yay/%s: decode error: %v (bytes: %x)", name, n, err, yayBytes)
			yayFail++
			continue
		}

		reencoded := decoded.Encode()
		var want []byte
		if isFunction {
			want = yayBytes
		} else {
			want, err = os.ReadFile(filepath.Join(base, "reencoded", n))
			if err != nil {
				t.Errorf("%s yay/%s: read reencoded: %v", name, n, err)
				yayFail++
				continue
			}
		}
		if !bytes.Equal(reencoded, want) {
			t.Errorf("%s yay/%s: re-encode mismatch\n  got:  %x\n  want: %x", name, n, reencoded, want)
			yayFail++
			continue
		}
		yayPass++
	}

	nayNames := listOrSkip(t, filepath.Join(base, "nay"))
	var nayPass, nayFail int
	for _, n := range nayNames {
		nayBytes, err := os.ReadFile(filepath.Join(base, "nay", n))
		if err != nil {
			t.Errorf("%s nay/%s: read: %v", name, n, err)
			nayFail++
			continue
		}
		_, _, err = Decode(limits, nayBytes)
		if err == nil {
			t.Errorf("%s nay/%s: decode unexpectedly succeeded (bytes: %x)", name, n, nayBytes)
			nayFail++
			continue
		}
		nayPass++
	}

	t.Logf("%s: yay %d/%d pass, nay %d/%d pass", name, yayPass, yayPass+yayFail, nayPass, nayPass+nayFail)
}

func TestUpstream_encode_path(t *testing.T) { runAbsolutePathVectors(t, "encode_path", true) }
func TestUpstream_EncodePath(t *testing.T)  { runAbsolutePathVectors(t, "EncodePath", false) }

// TestUpstream_CoverageSummary writes a per-encoding inventory of the
// upstream test_vectors corpus and how much of it our test runner above
// actually exercises. Useful for CI logs and as a periodic reminder of
// the gap.
func TestUpstream_CoverageSummary(t *testing.T) {
	base := filepath.Join("..", "testdata", "upstream_vectors")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Skipf("upstream_vectors submodule not initialized")
	}
	// "supported" = covered by the test runner above AND known to pass
	// against the current upstream vectors. Relative encodings are
	// excluded pending spec / willow_rs / test_vectors realignment;
	// see TECH_DEBT.md.
	supported := map[string]bool{
		"encode_path": true,
		"EncodePath":  true,
	}
	var totalYay, totalNay, supportedYay, supportedNay int
	type row struct {
		name string
		yay  int
		nay  int
		supp bool
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".git" {
			continue
		}
		yayDir := filepath.Join(base, e.Name(), "yay")
		nayDir := filepath.Join(base, e.Name(), "nay")
		yayFiles, _ := os.ReadDir(yayDir)
		nayFiles, _ := os.ReadDir(nayDir)
		var yc, nc int
		for _, f := range yayFiles {
			if !f.IsDir() {
				yc++
			}
		}
		for _, f := range nayFiles {
			if !f.IsDir() {
				nc++
			}
		}
		rows = append(rows, row{e.Name(), yc, nc, supported[e.Name()]})
		totalYay += yc
		totalNay += nc
		if supported[e.Name()] {
			supportedYay += yc
			supportedNay += nc
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	t.Log("Upstream willow_test_vectors corpus (worm-blossom/willow_test_vectors):")
	for _, r := range rows {
		mark := " "
		if r.supp {
			mark = "X"
		}
		t.Log(fmt.Sprintf("  [%s] %-40s yay=%d nay=%d", mark, r.name, r.yay, r.nay))
	}
	t.Log(fmt.Sprintf("supported encoders cover %d/%d yay + %d/%d nay vectors", supportedYay, totalYay, supportedNay, totalNay))
}
