// SPDX-FileCopyrightText: 2026 Ian Chechin
// SPDX-License-Identifier: MIT

package datamodel

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Deln0r/willow-go/encoding"
)

// Upstream test vectors live in testdata/upstream_vectors/, a git submodule
// pointing at https://codeberg.org/worm-blossom/willow_test_vectors (the
// GitHub copy was archived in August 2026). The corpus has two halves:
//
//   - codec/<name>: yay/ inputs that must decode and nay/ inputs that must
//     not. Sets named after an encoding relation (EncodePath) accept any
//     valid code and give the canonical re-encoding in reencoded/; sets
//     named after an encoding function (encode_path) accept only canonical
//     codes. Relative encodings add yay_relative_to/ and nay_relative_to/
//     holding the reference value.
//   - data_model/<name>: predicates (true/ and false/ inputs) and operators
//     (input/ and output/) over Willow data types.
//
// We run every set whose types this package implements. Capability,
// authorisation token, private area, absolute Area and Range3d encodings are
// not implemented here, and store_pruning needs authorised entry decoding;
// see TECH_DEBT.md.
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

// The area-in-area sets give their reference areas in the absolute
// encode_area format, which this package does not implement. decodeAreaVector
// and encodeAreaVector are a test-only transcription of it: a header byte
// (bit 7 set when there is no subspace id, bit 6 set for an open time range,
// low 6 bits the tag of the start time), the subspace id if any, the path,
// the start time, and for a closed range the standalone end - start - 1.
// TestUpstream_Codecs checks them against the EncodeArea and encode_area sets
// before trusting them as references.

func decodeAreaVector(src []byte) (Area, int, error) {
	if len(src) < 1 {
		return Area{}, 0, encoding.ErrShortBuffer
	}
	header := src[0]
	pos := 1
	var sub *[]byte
	if header&0b1000_0000 == 0 {
		if len(src) < pos+willow25SubspaceIDWidth {
			return Area{}, 0, encoding.ErrShortBuffer
		}
		id := append([]byte(nil), src[pos:pos+willow25SubspaceIDWidth]...)
		sub = &id
		pos += willow25SubspaceIDWidth
	}
	path, n, err := Decode(willow25Limits(), src[pos:])
	if err != nil {
		return Area{}, 0, err
	}
	pos += n
	start, n, err := encoding.DecodeCU64(encoding.ExtractTag(header, 6, 2), 6, src[pos:], false)
	if err != nil {
		return Area{}, 0, err
	}
	pos += n
	times := NewTimeRangeOpen(start)
	if header&0b0100_0000 == 0 {
		gap, n, err := encoding.DecodeCU64Standalone(src[pos:], false)
		if err != nil {
			return Area{}, 0, err
		}
		pos += n
		if start == ^uint64(0) || gap > ^uint64(0)-(start+1) {
			return Area{}, 0, fmt.Errorf("area end overflows")
		}
		if times, err = NewTimeRangeClosed(start, start+1+gap); err != nil {
			return Area{}, 0, err
		}
	}
	return Area{Subspace: sub, PathPrefix: path, Times: times}, pos, nil
}

func encodeAreaVector(a Area) []byte {
	header := encoding.WriteTag(0, 6, 2, a.Times.Start)
	if a.Subspace == nil {
		header |= 0b1000_0000
	}
	if a.Times.Open {
		header |= 0b0100_0000
	}
	out := []byte{header}
	if a.Subspace != nil {
		out = append(out, *a.Subspace...)
	}
	out = append(out, a.PathPrefix.Encode()...)
	out = encoding.AppendCU64(out, a.Times.Start, 6)
	if !a.Times.Open {
		out = encoding.AppendCU64Standalone(out, a.Times.End-a.Times.Start-1)
	}
	return out
}

func decodeAreaReferenceVector(_ *testing.T, _, src []byte) (func() []byte, int, error) {
	a, n, err := decodeAreaVector(src)
	return func() []byte { return encodeAreaVector(a) }, n, err
}

func mustDecodeAreaVector(t *testing.T, src []byte) Area {
	t.Helper()
	a, n, err := decodeAreaVector(src)
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
		// Test harness self-check for the reference area decoder.
		{"EncodeArea", false, false, decodeAreaReferenceVector},
		{"encode_area", false, true, decodeAreaReferenceVector},

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

// runPredicateVectors exercises a data_model predicate: each input is the
// concatenated encodings of the arguments, and pred must hold exactly for the
// inputs under true/.
func runPredicateVectors[T any](t *testing.T, name string, arity int, decode func([]byte) (T, int, error), pred func(args []T) bool) {
	t.Helper()
	requireCorpus(t)
	var pass, fail int
	for _, want := range []bool{true, false} {
		dir := upstreamDir("data_model", name, fmt.Sprint(want), "input")
		for _, n := range listVectors(t, dir) {
			src := readVector(t, filepath.Join(dir, n))
			args := make([]T, 0, arity)
			pos := 0
			for len(args) < arity {
				v, used, err := decode(src[pos:])
				if err != nil {
					t.Fatalf("%s %v/%s: argument %d: %v", name, want, n, len(args), err)
				}
				args = append(args, v)
				pos += used
			}
			if pos != len(src) {
				t.Fatalf("%s %v/%s: %d trailing bytes", name, want, n, len(src)-pos)
			}
			if got := pred(args); got != want {
				t.Errorf("%s %v/%s: got %v", name, want, n, got)
				fail++
				continue
			}
			pass++
		}
	}
	t.Logf("%s: %d/%d pass", name, pass, pass+fail)
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

func TestUpstream_path_least_path_lexicographically_greater_but_not_prefixed_by_original(t *testing.T) {
	requireCorpus(t)
	dir := upstreamDir("data_model", "path_least_path_lexicographically_greater_but_not_prefixed_by_original")
	var pass, fail int
	for _, kind := range []string{"some", "none"} {
		for _, n := range listVectors(t, filepath.Join(dir, kind, "input")) {
			p := mustDecodePathVector(t, readVector(t, filepath.Join(dir, kind, "input", n)))
			got, ok := p.GreaterButNotPrefixed()
			if kind == "none" {
				if ok {
					t.Errorf("none/%s: got %x, want no output", n, got.Encode())
					fail++
					continue
				}
			} else if want := readVector(t, filepath.Join(dir, kind, "output", n)); !ok || !bytes.Equal(got.Encode(), want) {
				t.Errorf("some/%s: got %x (ok=%v), want %x", n, got.Encode(), ok, want)
				fail++
				continue
			}
			pass++
		}
	}
	t.Logf("path_least_path_lexicographically_greater_but_not_prefixed_by_original: %d/%d pass", pass, pass+fail)
}

// TestUpstream_CoverageSummary logs a per-set inventory of the upstream
// corpus and marks the sets the tests above exercise, as a reminder of the
// gap. EncodeArea and encode_area only check the test-only reference decoder,
// so they are marked but not counted.
func TestUpstream_CoverageSummary(t *testing.T) {
	requireCorpus(t)
	exercised := map[string]bool{
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
	}
	harness := map[string]bool{"codec/EncodeArea": true, "codec/encode_area": true}

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
	for _, half := range []string{"codec", "data_model"} {
		sets, err := os.ReadDir(upstreamDir(half))
		if err != nil {
			t.Fatalf("read %s: %v", half, err)
		}
		for _, s := range sets {
			if !s.IsDir() {
				continue
			}
			key := half + "/" + s.Name()
			var count int
			if half == "codec" {
				count = countFiles(half, s.Name(), "yay") + countFiles(half, s.Name(), "nay")
			} else {
				for _, sub := range []string{"true", "false", "some", "none"} {
					count += countFiles(half, s.Name(), sub, "input")
				}
				count += countFiles(half, s.Name(), "input")
			}
			mark := " "
			switch {
			case exercised[key]:
				mark = "X"
				covered += count
			case harness[key]:
				mark = "H"
			}
			total += count
			t.Logf("  [%s] %-90s %d", mark, key, count)
		}
	}
	t.Logf("exercised sets cover %d of %d vectors", covered, total)
}
