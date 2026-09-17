// SPDX-FileCopyrightText: 2026 Ian Chechin
// SPDX-License-Identifier: MIT

package datamodel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Deln0r/willow-go/encoding"
)

func TestPath_IsEmpty(t *testing.T) {
	t.Parallel()
	if !EmptyPath(testLimits).IsEmpty() {
		t.Error("EmptyPath should be empty")
	}
	if pathOf(t, "a").IsEmpty() {
		t.Error("single-component path should not be empty")
	}
}

func TestPath_IsRelatedTo_LUB(t *testing.T) {
	t.Parallel()
	a := pathOf(t, "alpha")
	ab := pathOf(t, "alpha", "beta")
	ac := pathOf(t, "alpha", "gamma")

	if !a.IsRelatedTo(ab) {
		t.Error("alpha and alpha/beta should be related")
	}
	if ab.IsRelatedTo(ac) {
		t.Error("alpha/beta and alpha/gamma should not be related (neither is prefix)")
	}

	lub, ok := a.LeastUpperBound(ab)
	if !ok || !lub.Equal(ab) {
		t.Errorf("LUB(alpha, alpha/beta) = (%v, %v), want (alpha/beta, true)", lub.Components(), ok)
	}
	if _, ok := ab.LeastUpperBound(ac); ok {
		t.Error("LUB(alpha/beta, alpha/gamma) should be false (not related)")
	}
}

func TestPath_GreaterButNotPrefixed(t *testing.T) {
	t.Parallel()
	bigLimits := Limits{MaxComponentLength: 8, MaxComponentCount: 4, MaxPathLength: 32}

	cases := []struct {
		name        string
		path        Path
		wantOK      bool
		wantPath    Path
		description string
	}{
		{
			name:        "single_component_appendable",
			path:        mustPath(t, bigLimits, [][]byte{[]byte("abc")}),
			wantOK:      true,
			wantPath:    mustPath(t, bigLimits, [][]byte{{'a', 'b', 'c', 0}}),
			description: "abc -> abc\\x00 (append 0)",
		},
		{
			name:        "single_byte_appendable",
			path:        mustPath(t, bigLimits, [][]byte{{0xFF}}),
			wantOK:      true,
			wantPath:    mustPath(t, bigLimits, [][]byte{{0xFF, 0}}),
			description: "[FF] -> [FF, 00] (append 0 since len 1 < MCL 8)",
		},
		{
			name:        "increment_when_max_length",
			path:        mustPath(t, Limits{MaxComponentLength: 3, MaxComponentCount: 4, MaxPathLength: 32}, [][]byte{{0x01, 0x02, 0x03}}),
			wantOK:      true,
			wantPath:    mustPath(t, Limits{MaxComponentLength: 3, MaxComponentCount: 4, MaxPathLength: 32}, [][]byte{{0x01, 0x02, 0x04}}),
			description: "at MCL, increment last byte",
		},
		{
			name:        "all_ff_at_mcl_returns_false",
			path:        mustPath(t, Limits{MaxComponentLength: 2, MaxComponentCount: 1, MaxPathLength: 8}, [][]byte{{0xFF, 0xFF}}),
			wantOK:      false,
			description: "single component [FF, FF] at MCL=2, MCC=1: cannot append (full MCL), cannot increment (all FF), no earlier components -> false",
		},
		{
			name:        "multi_component_appends_to_last",
			path:        mustPath(t, bigLimits, [][]byte{[]byte("a"), []byte("b")}),
			wantOK:      true,
			wantPath:    mustPath(t, bigLimits, [][]byte{[]byte("a"), {'b', 0}}),
			description: "a/b -> a/b\\x00",
		},
		{
			name:        "increment_in_middle_byte_after_ff_trailing",
			path:        mustPath(t, Limits{MaxComponentLength: 3, MaxComponentCount: 1, MaxPathLength: 8}, [][]byte{{0x05, 0xFF, 0xFF}}),
			wantOK:      true,
			wantPath:    mustPath(t, Limits{MaxComponentLength: 3, MaxComponentCount: 1, MaxPathLength: 8}, [][]byte{{0x06}}),
			description: "[05 FF FF] at MCL=3: truncate to [06] (rightmost non-FF is index 0)",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.path.GreaterButNotPrefixed()
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (%s)", ok, tc.wantOK, tc.description)
			}
			if !tc.wantOK {
				return
			}
			if !got.Equal(tc.wantPath) {
				t.Errorf("got components %v, want %v (%s)",
					componentDump(got), componentDump(tc.wantPath), tc.description)
			}
			// Sanity: result is lex-greater than input AND NOT prefixed by input.
			if got.Compare(tc.path) <= 0 {
				t.Errorf("result %v not greater than input %v",
					componentDump(got), componentDump(tc.path))
			}
			if tc.path.IsPrefixOf(got) {
				t.Errorf("result %v should NOT be prefixed by input %v",
					componentDump(got), componentDump(tc.path))
			}
		})
	}
}

func TestArea_FullAndSubspace(t *testing.T) {
	t.Parallel()
	full := FullArea(testLimits)
	if !full.IsFull() {
		t.Error("FullArea should be full")
	}

	sub := SubspaceArea(testLimits, []byte("alfie"))
	if sub.IsFull() {
		t.Error("SubspaceArea should not be full")
	}
	if sub.Subspace == nil || !bytes.Equal(*sub.Subspace, []byte("alfie")) {
		t.Errorf("SubspaceArea: subspace = %v, want alfie", sub.Subspace)
	}
}

func TestArea_Includes(t *testing.T) {
	t.Parallel()
	sub := []byte("alfie")
	pathPrefix := pathOf(t, "folder")
	timeR, _ := NewTimeRangeClosed(100, 1000)
	subPtr := sub
	a := Area{Subspace: &subPtr, PathPrefix: pathPrefix, Times: timeR}

	in := Coordinate{Subspace: sub, Path: pathOf(t, "folder", "file"), Timestamp: 500}
	outSub := Coordinate{Subspace: []byte("betty"), Path: pathOf(t, "folder", "file"), Timestamp: 500}
	outPath := Coordinate{Subspace: sub, Path: pathOf(t, "other"), Timestamp: 500}
	outTime := Coordinate{Subspace: sub, Path: pathOf(t, "folder", "file"), Timestamp: 50}

	if !a.Includes(in) {
		t.Error("in-range coordinate should be included")
	}
	if a.Includes(outSub) {
		t.Error("out-of-subspace coordinate should be excluded")
	}
	if a.Includes(outPath) {
		t.Error("path outside prefix should be excluded")
	}
	if a.Includes(outTime) {
		t.Error("out-of-time coordinate should be excluded")
	}

	// Area with nil subspace accepts any subspace.
	anySub := Area{Subspace: nil, PathPrefix: pathPrefix, Times: timeR}
	if !anySub.Includes(outSub) {
		t.Error("nil-subspace Area should accept any subspace")
	}
}

func TestArea_Intersect(t *testing.T) {
	t.Parallel()
	subA := []byte("alfie")
	subB := []byte("betty")
	subAPtr := subA
	subBPtr := subB

	folder := pathOf(t, "folder")
	folderFile := pathOf(t, "folder", "file")
	other := pathOf(t, "other")

	timeWide, _ := NewTimeRangeClosed(0, 1000)
	timeNarrow, _ := NewTimeRangeClosed(100, 200)

	// Two areas with related paths: intersection picks the longer path.
	a1 := Area{Subspace: nil, PathPrefix: folder, Times: timeWide}
	a2 := Area{Subspace: &subAPtr, PathPrefix: folderFile, Times: timeNarrow}
	got, err := a1.Intersect(a2)
	if err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	if got.Subspace == nil || !bytes.Equal(*got.Subspace, subA) {
		t.Errorf("intersect subspace = %v, want alfie", got.Subspace)
	}
	if !got.PathPrefix.Equal(folderFile) {
		t.Errorf("intersect path = %v, want folder/file", componentDump(got.PathPrefix))
	}
	if got.Times != timeNarrow {
		t.Errorf("intersect times = %+v, want %+v", got.Times, timeNarrow)
	}

	// Unrelated paths -> empty grouping.
	a3 := Area{Subspace: nil, PathPrefix: other, Times: timeWide}
	if _, err := a1.Intersect(a3); !errors.Is(err, ErrEmptyGrouping) {
		t.Errorf("unrelated paths: err = %v, want ErrEmptyGrouping", err)
	}

	// Conflicting concrete subspaces -> empty grouping.
	a4 := Area{Subspace: &subBPtr, PathPrefix: folder, Times: timeWide}
	a5 := Area{Subspace: &subAPtr, PathPrefix: folder, Times: timeWide}
	if _, err := a4.Intersect(a5); !errors.Is(err, ErrEmptyGrouping) {
		t.Errorf("conflicting subspaces: err = %v, want ErrEmptyGrouping", err)
	}
}

type areaJSON struct {
	SubspaceHex       *string  `json:"subspace_hex"`
	PathComponentsHex []string `json:"path_components_hex"`
	TimesStart        uint64   `json:"times_start"`
	TimesEnd          *uint64  `json:"times_end"`
}

type areaRelativeCase struct {
	Name       string   `json:"name"`
	Rel        areaJSON `json:"rel"`
	Target     areaJSON `json:"target"`
	EncodedHex string   `json:"encoded_hex"`
}

type areaRelativeFile struct {
	Params struct {
		MCL             int `json:"mcl"`
		MCC             int `json:"mcc"`
		MPL             int `json:"mpl"`
		SubspaceIDWidth int `json:"subspace_id_width"`
	} `json:"params"`
	Cases []areaRelativeCase `json:"cases"`
}

func areaFromJSON(t *testing.T, limits Limits, j areaJSON) Area {
	t.Helper()
	var sub *[]byte
	if j.SubspaceHex != nil {
		b, err := hex.DecodeString(*j.SubspaceHex)
		if err != nil {
			t.Fatalf("decode subspace hex: %v", err)
		}
		sub = &b
	}
	comps := decodeHexComponents(t, j.PathComponentsHex)
	path, err := FromSlices(limits, comps)
	if err != nil {
		t.Fatalf("FromSlices(path): %v", err)
	}
	var times TimeRange
	if j.TimesEnd == nil {
		times = NewTimeRangeOpen(j.TimesStart)
	} else {
		times, err = NewTimeRangeClosed(j.TimesStart, *j.TimesEnd)
		if err != nil {
			t.Fatalf("NewTimeRangeClosed: %v", err)
		}
	}
	return Area{Subspace: sub, PathPrefix: path, Times: times}
}

func areaEqual(a, b Area) bool {
	if (a.Subspace == nil) != (b.Subspace == nil) {
		return false
	}
	if a.Subspace != nil && !bytes.Equal(*a.Subspace, *b.Subspace) {
		return false
	}
	if !a.PathPrefix.Equal(b.PathPrefix) {
		return false
	}
	return a.Times == b.Times
}

func TestArea_RelativeEncodingFixtures(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "areas", "relative.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f areaRelativeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	limits := Limits{MaxComponentLength: f.Params.MCL, MaxComponentCount: f.Params.MCC, MaxPathLength: f.Params.MPL}

	for _, c := range f.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			rel := areaFromJSON(t, limits, c.Rel)
			target := areaFromJSON(t, limits, c.Target)
			want, err := hex.DecodeString(c.EncodedHex)
			if err != nil {
				t.Fatalf("decode encoded hex: %v", err)
			}

			got := target.EncodeRelativeTo(rel)
			if !bytes.Equal(got, want) {
				t.Errorf("EncodeRelativeTo mismatch\n got: %x\nwant: %x", got, want)
			}

			decoded, n, err := DecodeAreaRelativeTo(limits, rel, f.Params.SubspaceIDWidth, want)
			if err != nil {
				t.Fatalf("DecodeAreaRelativeTo: %v", err)
			}
			if n != len(want) {
				t.Errorf("consumed %d bytes, want %d", n, len(want))
			}
			if !areaEqual(decoded, target) {
				t.Errorf("DecodeAreaRelativeTo produced non-equal area:\n got %+v subspace=%x path=%v\nwant %+v subspace=%x path=%v",
					decoded.Times, deref(decoded.Subspace), componentDump(decoded.PathPrefix),
					target.Times, deref(target.Subspace), componentDump(target.PathPrefix))
			}
		})
	}
}

func deref(b *[]byte) []byte {
	if b == nil {
		return nil
	}
	return *b
}

func TestArea_EncodeRelativeTo_PanicsOnNonInclusion(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when rel does not include self")
		}
	}()
	rel := Area{Subspace: nil, PathPrefix: pathOf(t, "folder"), Times: FullTimeRange()}
	target := Area{Subspace: nil, PathPrefix: pathOf(t, "other"), Times: FullTimeRange()}
	_ = target.EncodeRelativeTo(rel)
}

// TestDecodeAreaRelativeTo_Containment covers the checks that keep the
// decoder from producing an area outside rel. The rejected shapes are the
// ones the upstream EncodeAreaInArea negative vectors exercise.
func TestDecodeAreaRelativeTo_Containment(t *testing.T) {
	t.Parallel()
	sub := bytes.Repeat([]byte{7}, 4)
	otherSub := bytes.Repeat([]byte{8}, 4)
	prefix := pathOf(t, "folder")
	closedTimes, _ := NewTimeRangeClosed(100, 200)
	closedRel := Area{PathPrefix: prefix, Times: closedTimes}
	openRel := Area{PathPrefix: prefix, Times: NewTimeRangeOpen(10)}
	subRel := Area{Subspace: &sub, PathPrefix: prefix, Times: FullTimeRange()}
	noSuffix := prefix.EncodeExtending(prefix)

	// encode builds header flags, the tagged time differences, an optional
	// explicit subspace, and an empty path suffix.
	encode := func(flags uint8, subspace []byte, startDiff uint64, endDiff *uint64) []byte {
		header := encoding.WriteTag(flags, 2, 4, startDiff)
		if endDiff != nil {
			header = encoding.WriteTag(header, 2, 6, *endDiff)
		}
		out := append([]byte{header}, subspace...)
		out = encoding.AppendCU64(out, startDiff, 2)
		if endDiff != nil {
			out = encoding.AppendCU64(out, *endDiff, 2)
		}
		return append(out, noSuffix...)
	}
	u := func(v uint64) *uint64 { return &v }

	const (
		subspaceBit       = 0b1000_0000
		openBit           = 0b0100_0000
		startFromStartBit = 0b0010_0000
		endFromStartBit   = 0b0001_0000
	)

	cases := []struct {
		name   string
		rel    Area
		src    []byte
		wantOK bool
	}{
		{"closed inside closed rel", closedRel, encode(startFromStartBit|endFromStartBit, nil, 10, u(50)), true},
		{"open range inside closed rel", closedRel, encode(openBit|startFromStartBit, nil, 10, nil), false},
		{"end past closed rel", closedRel, encode(startFromStartBit|endFromStartBit, nil, 10, u(150)), false},
		{"start overflow", openRel, encode(openBit|startFromStartBit, nil, math.MaxUint64, nil), false},
		{"end overflow", openRel, encode(startFromStartBit|endFromStartBit, nil, 0, u(math.MaxUint64)), false},
		{"end_from_start on open range", openRel, encode(openBit|startFromStartBit|endFromStartBit, nil, 0, nil), false},
		{"explicit subspace equal to rel's", subRel, encode(subspaceBit|openBit|startFromStartBit, sub, 0, nil), true},
		{"explicit subspace differs from rel's", subRel, encode(subspaceBit|openBit|startFromStartBit, otherSub, 0, nil), false},
	}
	for _, c := range cases {
		got, n, err := DecodeAreaRelativeTo(testLimits, c.rel, len(sub), c.src)
		if !c.wantOK {
			if err == nil {
				t.Errorf("%s: decoded %+v, want an error", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if n != len(c.src) || !c.rel.includesArea(got) {
			t.Errorf("%s: consumed %d of %d, included in rel: %v", c.name, n, len(c.src), c.rel.includesArea(got))
		}
	}
}

func TestArea_AsRange3d(t *testing.T) {
	t.Parallel()
	bigLimits := Limits{MaxComponentLength: 8, MaxComponentCount: 4, MaxPathLength: 32}

	// Full area -> full Range3d.
	full := FullArea(bigLimits)
	r := full.AsRange3d()
	if !r.IsFull() {
		t.Error("FullArea.AsRange3d should be full")
	}

	// Subspace-specific area with appendable path prefix.
	sub := []byte("alfie")
	subPtr := sub
	a := Area{
		Subspace:   &subPtr,
		PathPrefix: mustPath(t, bigLimits, [][]byte{[]byte("folder")}),
		Times:      FullTimeRange(),
	}
	r = a.AsRange3d()
	if r.Subspaces.Open {
		t.Error("subspace-specific area should not produce open subspace range")
	}
	if !bytes.Equal(r.Subspaces.Start, sub) {
		t.Errorf("subspace start = %x, want %x", r.Subspaces.Start, sub)
	}
	if r.Paths.Open {
		t.Error("appendable path prefix should produce closed PathRange (not open)")
	}
	if !r.Paths.Start.Equal(a.PathPrefix) {
		t.Errorf("path start = %v, want %v", componentDump(r.Paths.Start), componentDump(a.PathPrefix))
	}

	// Coordinate at exactly the path prefix should be included by the
	// resulting Range3d, matching Area inclusion semantics.
	c := Coordinate{Subspace: sub, Path: a.PathPrefix, Timestamp: 42}
	if !r.Includes(c) {
		t.Error("Range3d from Area should include the path prefix itself")
	}
	cChild := Coordinate{Subspace: sub, Path: mustPath(t, bigLimits, [][]byte{[]byte("folder"), []byte("file")}), Timestamp: 42}
	if !r.Includes(cChild) {
		t.Error("Range3d from Area should include children of the path prefix")
	}
	cSibling := Coordinate{Subspace: sub, Path: mustPath(t, bigLimits, [][]byte{[]byte("other")}), Timestamp: 42}
	if r.Includes(cSibling) {
		t.Error("Range3d from Area should not include siblings of the path prefix")
	}
}
