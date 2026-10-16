// SPDX-FileCopyrightText: 2026 Ian Chechin
// SPDX-License-Identifier: MIT

package datamodel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Upstream test vectors live in testdata/upstream_vectors/, a git submodule
// pointing at https://codeberg.org/worm-blossom/willow_test_vectors (the
// GitHub copy was archived in August 2026). The corpus is split into:
//
//   - codec/<name>: yay/ inputs that must decode and nay/ inputs that must
//     not. Sets named after an encoding relation (EncodePath) accept any
//     valid code and give the canonical re-encoding in reencoded/; sets
//     named after an encoding function (encode_path) accept only canonical
//     codes. Relative encodings add yay_relative_to/ and nay_relative_to/
//     holding the reference value.
//   - data_model/<name> and groupings/<name>: predicates (true/ and false/
//     inputs), operators (input/ and output/) and constants (value) over
//     Willow data types and over Area and Range3d.
//   - drop_format/codec: the sideloading format.
//
// We run every set whose types this package implements. Capability,
// authorisation token, private area and Range3d encodings are not implemented
// here (so neither are the groupings sets over Range3d), store_pruning needs
// authorised entry decoding, and the drop format belongs to sideloading; see
// TECH_DEBT.md.
//
// To initialize the submodule: `git submodule update --init`.

// willow25Limits returns the path size limits used by all upstream test
// vectors ("All data sets use Willow'25 parameters"). Duplicates
// willow25.Limits() so this test file does not import the willow25 package
// and create a cycle.
func willow25Limits() Limits {
	return Limits{MaxComponentLength: 4096, MaxComponentCount: 4096, MaxPathLength: 4096}
}

// willow25EntrySpec duplicates willow25.EntrySpec() for the same reason.
func willow25EntrySpec() EntrySpec {
	return EntrySpec{Limits: willow25Limits(), NamespaceIDLength: 32, SubspaceIDLength: 32, PayloadDigestLength: 32}
}

const willow25SubspaceIDWidth = 32

func upstreamDir(parts ...string) string {
	return filepath.Join(append([]string{"..", "testdata", "upstream_vectors"}, parts...)...)
}

// requireCorpus skips when the submodule is not initialized and fails when
// the checkout predates the codec/ + data_model/ layout, so a stale
// submodule cannot pass by running nothing.
func requireCorpus(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(upstreamDir())
	if err != nil || len(entries) == 0 {
		t.Skip("upstream_vectors submodule not initialized; run `git submodule update --init` to enable")
	}
	if _, err := os.Stat(upstreamDir("codec")); err != nil {
		t.Fatalf("upstream_vectors checkout has no codec/ directory; run `git submodule update` to move to the pinned commit")
	}
}

// listVectors lists the files in dir, sorted by name.
func listVectors(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
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

func readVector(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// codecDecoder decodes src (relative to the value encoded in ref, for
// relative encodings) and returns a function that re-encodes the result the
// way the vector set encodes, plus the number of bytes consumed.
type codecDecoder func(t *testing.T, ref, src []byte) (reencode func() []byte, n int, err error)

// runCodecVectors exercises one codec/<name> set. Every yay input must
// decode in full and re-encode to its canonical code. Every nay input must be
// rejected. Our decoders are lenient (they accept non-canonical codes), so
// for an encoding function set a nay input also counts as rejected when it
// decodes but is not the canonical code of the value it decodes to.
func runCodecVectors(t *testing.T, name string, relative, canonic bool, decode codecDecoder) {
	t.Helper()
	requireCorpus(t)
	dir := upstreamDir("codec", name)

	var yayPass, yayFail int
	for _, n := range listVectors(t, filepath.Join(dir, "yay")) {
		src := readVector(t, filepath.Join(dir, "yay", n))
		var ref []byte
		if relative {
			ref = readVector(t, filepath.Join(dir, "yay_relative_to", n))
		}
		reencode, consumed, err := decode(t, ref, src)
		if err != nil {
			t.Errorf("%s yay/%s: decode error: %v (bytes: %x)", name, n, err, src)
			yayFail++
			continue
		}
		if consumed != len(src) {
			t.Errorf("%s yay/%s: consumed %d of %d bytes", name, n, consumed, len(src))
			yayFail++
			continue
		}
		want := src
		if !canonic {
			want = readVector(t, filepath.Join(dir, "reencoded", n))
		}
		if got := reencode(); !bytes.Equal(got, want) {
			t.Errorf("%s yay/%s: re-encode mismatch\n  got:  %x\n  want: %x", name, n, got, want)
			yayFail++
			continue
		}
		yayPass++
	}

	var nayPass, nayFail int
	for _, n := range listVectors(t, filepath.Join(dir, "nay")) {
		src := readVector(t, filepath.Join(dir, "nay", n))
		var ref []byte
		if relative {
			ref = readVector(t, filepath.Join(dir, "nay_relative_to", n))
		}
		reencode, consumed, err := decode(t, ref, src)
		rejected := err != nil
		if !rejected && canonic {
			rejected = !bytes.Equal(reencode(), src[:consumed])
		}
		if !rejected {
			t.Errorf("%s nay/%s: decode unexpectedly succeeded (bytes: %x)", name, n, src)
			nayFail++
			continue
		}
		nayPass++
	}

	t.Logf("%s: yay %d/%d pass, nay %d/%d pass", name, yayPass, yayPass+yayFail, nayPass, nayPass+nayFail)
}

func decodePathVector(_ *testing.T, _, src []byte) (func() []byte, int, error) {
	p, n, err := Decode(willow25Limits(), src)
	return p.Encode, n, err
}

func decodeEntryVector(_ *testing.T, _, src []byte) (func() []byte, int, error) {
	e, n, err := DecodeEntry(willow25EntrySpec(), src)
	return e.Encode, n, err
}

func decodePathExtendsPathVector(t *testing.T, ref, src []byte) (func() []byte, int, error) {
	prefix := mustDecodePathVector(t, ref)
	p, n, err := DecodeExtending(prefix, src)
	return func() []byte { return p.EncodeExtending(prefix) }, n, err
}

func decodePathRelativePathVector(t *testing.T, ref, src []byte) (func() []byte, int, error) {
	base := mustDecodePathVector(t, ref)
	p, n, err := DecodeRelative(willow25Limits(), base, src)
	return func() []byte { return p.EncodeRelativeTo(base) }, n, err
}

func decodeAreaInAreaVector(t *testing.T, ref, src []byte) (func() []byte, int, error) {
	outer := mustDecodeAreaVector(t, ref)
	a, n, err := DecodeAreaRelativeTo(willow25Limits(), outer, willow25SubspaceIDWidth, src)
	return func() []byte { return a.EncodeRelativeTo(outer) }, n, err
}

func mustDecodePathVector(t *testing.T, src []byte) Path {
	t.Helper()
	p, n, err := Decode(willow25Limits(), src)
	if err != nil || n != len(src) {
		t.Fatalf("reference path %x: consumed %d, error %v", src, n, err)
	}
	return p
}

func decodeAreaVector(_ *testing.T, _, src []byte) (func() []byte, int, error) {
	a, n, err := DecodeArea(willow25Limits(), willow25SubspaceIDWidth, src)
	return a.Encode, n, err
}

func mustDecodeAreaVector(t *testing.T, src []byte) Area {
	t.Helper()
	a, n, err := DecodeArea(willow25Limits(), willow25SubspaceIDWidth, src)
	if err != nil || n != len(src) {
		t.Fatalf("reference area %x: consumed %d, error %v", src, n, err)
	}
	return a
}

func TestUpstream_Codecs(t *testing.T) {
	sets := []struct {
		name     string
		relative bool
		canonic  bool
		decode   codecDecoder
	}{
		{"EncodeArea", false, false, decodeAreaVector},
		{"encode_area", false, true, decodeAreaVector},

		{"EncodePath", false, false, decodePathVector},
		{"encode_path", false, true, decodePathVector},
		{"EncodeEntry", false, false, decodeEntryVector},
		{"encode_entry", false, true, decodeEntryVector},
		{"EncodePathExtendsPath", true, false, decodePathExtendsPathVector},
		{"path_extends_path", true, true, decodePathExtendsPathVector},
		{"EncodePathRelativePath", true, false, decodePathRelativePathVector},
		{"path_rel_path", true, true, decodePathRelativePathVector},
		{"EncodeAreaInArea", true, false, decodeAreaInAreaVector},
		{"encode_area_in_area", true, true, decodeAreaInAreaVector},
	}
	for _, s := range sets {
		t.Run(s.name, func(t *testing.T) {
			runCodecVectors(t, s.name, s.relative, s.canonic, s.decode)
		})
	}
}

// runRawPredicateVectors exercises a predicate set under dir (for example
// "data_model" or "groupings"): eval decodes one input, the concatenated
// encodings of the arguments, and must report true exactly for the inputs
// under true/.
func runRawPredicateVectors(t *testing.T, dir, name string, eval func(t *testing.T, src []byte) bool) {
	t.Helper()
	requireCorpus(t)
	var pass, fail int
	for _, want := range []bool{true, false} {
		inputs := upstreamDir(dir, name, fmt.Sprint(want), "input")
		for _, n := range listVectors(t, inputs) {
			if got := eval(t, readVector(t, filepath.Join(inputs, n))); got != want {
				t.Errorf("%s %v/%s: got %v", name, want, n, got)
				fail++
				continue
			}
			pass++
		}
	}
	t.Logf("%s: %d/%d pass", name, pass, pass+fail)
}

// runPredicateVectors exercises a data_model predicate whose arity arguments
// all have the same type.
func runPredicateVectors[T any](t *testing.T, name string, arity int, decode func([]byte) (T, int, error), pred func(args []T) bool) {
	t.Helper()
	runRawPredicateVectors(t, "data_model", name, func(t *testing.T, src []byte) bool {
		args := make([]T, 0, arity)
		pos := 0
		for len(args) < arity {
			v, used, err := decode(src[pos:])
			if err != nil {
				t.Fatalf("%s: argument %d of %x: %v", name, len(args), src, err)
			}
			args = append(args, v)
			pos += used
		}
		if pos != len(src) {
			t.Fatalf("%s: %d trailing bytes in %x", name, len(src)-pos, src)
		}
		return pred(args)
	})
}

func decodePathArg(src []byte) (Path, int, error) { return Decode(willow25Limits(), src) }

func decodeEntryArg(src []byte) (Entry, int, error) { return DecodeEntry(willow25EntrySpec(), src) }

func TestUpstream_Predicates(t *testing.T) {
	t.Run("entry_is_newer_than", func(t *testing.T) {
		runPredicateVectors(t, "entry_is_newer_than", 2, decodeEntryArg, func(e []Entry) bool { return e[0].IsNewerThan(e[1]) })
	})
	t.Run("entry_prunes", func(t *testing.T) {
		runPredicateVectors(t, "entry_prunes", 2, decodeEntryArg, func(e []Entry) bool { return e[0].Prunes(e[1]) })
	})
	t.Run("entry_is_pruned_by", func(t *testing.T) {
		runPredicateVectors(t, "entry_is_pruned_by", 2, decodeEntryArg, func(e []Entry) bool { return e[1].Prunes(e[0]) })
	})
	t.Run("path_is_empty", func(t *testing.T) {
		runPredicateVectors(t, "path_is_empty", 1, decodePathArg, func(p []Path) bool { return p[0].IsEmpty() })
	})
	t.Run("path_is_prefix_of", func(t *testing.T) {
		runPredicateVectors(t, "path_is_prefix_of", 2, decodePathArg, func(p []Path) bool { return p[0].IsPrefixOf(p[1]) })
	})
	t.Run("path_is_prefixed_by", func(t *testing.T) {
		runPredicateVectors(t, "path_is_prefixed_by", 2, decodePathArg, func(p []Path) bool { return p[0].IsPrefixedBy(p[1]) })
	})
	t.Run("path_is_related_to", func(t *testing.T) {
		runPredicateVectors(t, "path_is_related_to", 2, decodePathArg, func(p []Path) bool { return p[0].IsRelatedTo(p[1]) })
	})
	t.Run("path_is_less_than_or_equal_to", func(t *testing.T) {
		runPredicateVectors(t, "path_is_less_than_or_equal_to", 2, decodePathArg, func(p []Path) bool { return p[0].Compare(p[1]) <= 0 })
	})
}

func TestUpstream_path_longest_common_prefix(t *testing.T) {
	requireCorpus(t)
	dir := upstreamDir("data_model", "path_longest_common_prefix")
	var pass, fail int
	for _, n := range listVectors(t, filepath.Join(dir, "input")) {
		src := readVector(t, filepath.Join(dir, "input", n))
		a, used, err := decodePathArg(src)
		if err != nil {
			t.Fatalf("input/%s: first path: %v", n, err)
		}
		b := mustDecodePathVector(t, src[used:])
		want := readVector(t, filepath.Join(dir, "output", n))
		if got := a.LongestCommonPrefix(b).Encode(); !bytes.Equal(got, want) {
			t.Errorf("input/%s: got %x, want %x", n, got, want)
			fail++
			continue
		}
		pass++
	}
	t.Logf("path_longest_common_prefix: %d/%d pass", pass, pass+fail)
}

// runPartialPathOperatorVectors exercises a data_model partial unary operator
// on paths: every input under some/ must map to the path in output/, and no
// input under none/ may have a result.
func runPartialPathOperatorVectors(t *testing.T, name string, op func(Path) (Path, bool)) {
	t.Helper()
	requireCorpus(t)
	dir := upstreamDir("data_model", name)
	var pass, fail int
	for _, kind := range []string{"some", "none"} {
		for _, n := range listVectors(t, filepath.Join(dir, kind, "input")) {
			p := mustDecodePathVector(t, readVector(t, filepath.Join(dir, kind, "input", n)))
			got, ok := op(p)
			if kind == "none" {
				if ok {
					t.Errorf("%s none/%s: got %x, want no output", name, n, got.Encode())
					fail++
					continue
				}
			} else if want := readVector(t, filepath.Join(dir, kind, "output", n)); !ok || !bytes.Equal(got.Encode(), want) {
				t.Errorf("%s some/%s: got %x (ok=%v), want %x", name, n, got.Encode(), ok, want)
				fail++
				continue
			}
			pass++
		}
	}
	t.Logf("%s: %d/%d pass", name, pass, pass+fail)
}

func TestUpstream_PathPartialOperators(t *testing.T) {
	for _, s := range []struct {
		name string
		op   func(Path) (Path, bool)
	}{
		{"path_least_path_lexicographically_greater_but_not_prefixed_by_original", Path.GreaterButNotPrefixed},
		{"path_successor", Path.Successor},
		{"path_predecessor", Path.Predecessor},
	} {
		t.Run(s.name, func(t *testing.T) {
			runPartialPathOperatorVectors(t, s.name, s.op)
		})
	}
}

// decodeAreaPrefix decodes the encode_area argument at the start of src and
// returns it with the number of bytes it took.
func decodeAreaPrefix(t *testing.T, src []byte) (Area, int) {
	t.Helper()
	a, n, err := DecodeArea(willow25Limits(), willow25SubspaceIDWidth, src)
	if err != nil {
		t.Fatalf("area argument of %x: %v", src, err)
	}
	return a, n
}

// TestUpstream_Groupings runs the groupings sets over Area; their areas are
// given in the absolute encode_area format. The sets over Range3d need the
// Range3d encoding, which this package does not implement.
func TestUpstream_Groupings(t *testing.T) {
	t.Run("area_includes", func(t *testing.T) {
		runRawPredicateVectors(t, "groupings", "area_includes", func(t *testing.T, src []byte) bool {
			a, n := decodeAreaPrefix(t, src)
			e, m, err := DecodeEntry(willow25EntrySpec(), src[n:])
			if err != nil || n+m != len(src) {
				t.Fatalf("entry argument of %x: consumed %d of %d bytes, error %v", src, n+m, len(src), err)
			}
			return a.IncludesEntry(e)
		})
	})

	t.Run("area_includes_area", func(t *testing.T) {
		runRawPredicateVectors(t, "groupings", "area_includes_area", func(t *testing.T, src []byte) bool {
			a, n := decodeAreaPrefix(t, src)
			return a.IncludesArea(mustDecodeAreaVector(t, src[n:]))
		})
	})

	t.Run("area_intersection", func(t *testing.T) {
		requireCorpus(t)
		dir := upstreamDir("groupings", "area_intersection")
		var pass, fail int
		for _, kind := range []string{"some", "none"} {
			for _, n := range listVectors(t, filepath.Join(dir, kind, "input")) {
				src := readVector(t, filepath.Join(dir, kind, "input", n))
				a, used := decodeAreaPrefix(t, src)
				got, err := a.Intersect(mustDecodeAreaVector(t, src[used:]))
				if kind == "none" {
					if !errors.Is(err, ErrEmptyGrouping) {
						t.Errorf("none/%s: got %x (err %v), want an empty intersection", n, got.Encode(), err)
						fail++
						continue
					}
				} else if want := readVector(t, filepath.Join(dir, kind, "output", n)); err != nil || !bytes.Equal(got.Encode(), want) {
					t.Errorf("some/%s: got %x (err %v), want %x", n, got.Encode(), err, want)
					fail++
					continue
				}
				pass++
			}
		}
		t.Logf("area_intersection: %d/%d pass", pass, pass+fail)
	})

	t.Run("area_subspace", func(t *testing.T) {
		requireCorpus(t)
		dir := upstreamDir("groupings", "area_subspace")
		var pass, fail int
		for _, n := range listVectors(t, filepath.Join(dir, "input")) {
			id := readVector(t, filepath.Join(dir, "input", n))
			if len(id) != willow25SubspaceIDWidth {
				t.Fatalf("input/%s: subspace id of %d bytes", n, len(id))
			}
			want := readVector(t, filepath.Join(dir, "output", n))
			if got := SubspaceArea(willow25Limits(), id).Encode(); !bytes.Equal(got, want) {
				t.Errorf("input/%s: got %x, want %x", n, got, want)
				fail++
				continue
			}
			pass++
		}
		t.Logf("area_subspace: %d/%d pass", pass, pass+fail)
	})

	t.Run("area_full", func(t *testing.T) {
		requireCorpus(t)
		want := readVector(t, upstreamDir("groupings", "area_full", "value"))
		if got := FullArea(willow25Limits()).Encode(); !bytes.Equal(got, want) {
			t.Errorf("full area: got %x, want %x", got, want)
		}
	})
}

// TestUpstream_CoverageSummary logs a per-set inventory of the upstream
// corpus and marks the sets the tests above exercise, as a reminder of the
// gap. A constant (a value file) counts as one vector.
func TestUpstream_CoverageSummary(t *testing.T) {
	requireCorpus(t)
	exercised := map[string]bool{
		"groupings/area_includes": true, "groupings/area_includes_area": true,
		"groupings/area_intersection": true, "groupings/area_subspace": true,
		"groupings/area_full": true,
		"codec/EncodeArea":    true, "codec/encode_area": true,
		"codec/EncodePath": true, "codec/encode_path": true,
		"codec/EncodeEntry": true, "codec/encode_entry": true,
		"codec/EncodePathExtendsPath": true, "codec/path_extends_path": true,
		"codec/EncodePathRelativePath": true, "codec/path_rel_path": true,
		"codec/EncodeAreaInArea": true, "codec/encode_area_in_area": true,
		"data_model/entry_is_newer_than": true, "data_model/entry_prunes": true,
		"data_model/entry_is_pruned_by": true, "data_model/path_is_empty": true,
		"data_model/path_is_prefix_of": true, "data_model/path_is_prefixed_by": true,
		"data_model/path_is_related_to": true, "data_model/path_is_less_than_or_equal_to": true,
		"data_model/path_longest_common_prefix":                                             true,
		"data_model/path_least_path_lexicographically_greater_but_not_prefixed_by_original": true,
		"data_model/path_successor":                                                         true, "data_model/path_predecessor": true,
	}

	countFiles := func(parts ...string) int {
		entries, _ := os.ReadDir(upstreamDir(parts...))
		c := 0
		for _, e := range entries {
			if !e.IsDir() {
				c++
			}
		}
		return c
	}

	var total, covered int
	t.Log("Upstream willow_test_vectors corpus (codeberg.org/worm-blossom/willow_test_vectors):")
	for _, half := range []string{"codec", "data_model", "groupings", "drop_format"} {
		sets, err := os.ReadDir(upstreamDir(half))
		if err != nil {
			t.Fatalf("read %s: %v", half, err)
		}
		for _, s := range sets {
			if !s.IsDir() {
				continue
			}
			key := half + "/" + s.Name()
			count := countFiles(half, s.Name(), "yay") + countFiles(half, s.Name(), "nay")
			for _, sub := range []string{"true", "false", "some", "none"} {
				count += countFiles(half, s.Name(), sub, "input")
			}
			count += countFiles(half, s.Name(), "input")
			if _, err := os.Stat(upstreamDir(half, s.Name(), "value")); err == nil {
				count++
			}
			mark := " "
			switch {
			case exercised[key]:
				mark = "X"
				covered += count
			}
			total += count
			t.Logf("  [%s] %-90s %d", mark, key, count)
		}
	}
	t.Logf("exercised sets cover %d of %d vectors", covered, total)
}
