package datamodel

import (
	"bytes"
	"errors"
	"testing"
)

var testLimits = Limits{MaxComponentLength: 64, MaxComponentCount: 16, MaxPathLength: 256}

func pathOf(t *testing.T, comps ...string) Path {
	t.Helper()
	byteComps := make([][]byte, len(comps))
	for i, c := range comps {
		byteComps[i] = []byte(c)
	}
	p, err := FromSlices(testLimits, byteComps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	return p
}

func TestPath_Compare(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b Path
		want int
	}{
		{"equal_empty", pathOf(t), pathOf(t), 0},
		{"equal_single", pathOf(t, "a"), pathOf(t, "a"), 0},
		{"empty_less_than_any", pathOf(t), pathOf(t, "a"), -1},
		{"shorter_prefix_less", pathOf(t, "a"), pathOf(t, "a", "b"), -1},
		{"first_component_decides", pathOf(t, "a", "z"), pathOf(t, "b", "a"), -1},
		{"second_component_decides", pathOf(t, "a", "a"), pathOf(t, "a", "b"), -1},
		{"byte_order_within_component", pathOf(t, "\x00"), pathOf(t, "\x01"), -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.a.Compare(tc.b); got != tc.want {
				t.Errorf("a.Compare(b) = %d, want %d", got, tc.want)
			}
			if got := tc.b.Compare(tc.a); got != -tc.want {
				t.Errorf("b.Compare(a) = %d, want %d", got, -tc.want)
			}
		})
	}
}

func TestTimeRange_IncludesValue(t *testing.T) {
	t.Parallel()
	closed, err := NewTimeRangeClosed(10, 20)
	if err != nil {
		t.Fatal(err)
	}
	open := NewTimeRangeOpen(10)

	cases := []struct {
		name  string
		r     TimeRange
		value uint64
		want  bool
	}{
		{"closed_below", closed, 5, false},
		{"closed_start_inclusive", closed, 10, true},
		{"closed_middle", closed, 15, true},
		{"closed_end_exclusive", closed, 20, false},
		{"closed_above", closed, 25, false},
		{"open_below", open, 5, false},
		{"open_start_inclusive", open, 10, true},
		{"open_far_above", open, 1 << 40, true},
		{"full_includes_zero", FullTimeRange(), 0, true},
		{"full_includes_max", FullTimeRange(), ^uint64(0), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.r.IncludesValue(tc.value); got != tc.want {
				t.Errorf("IncludesValue(%d) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestTimeRange_Intersect(t *testing.T) {
	t.Parallel()

	r1, _ := NewTimeRangeClosed(10, 30)
	r2, _ := NewTimeRangeClosed(20, 40)
	r3, _ := NewTimeRangeClosed(50, 60)
	full := FullTimeRange()
	open := NewTimeRangeOpen(25)

	if got, err := r1.Intersect(r2); err != nil || got.Open || got.Start != 20 || got.End != 30 {
		t.Errorf("closed/closed overlap: got %+v err=%v, want {20,30}", got, err)
	}
	if _, err := r1.Intersect(r3); !errors.Is(err, ErrEmptyGrouping) {
		t.Errorf("disjoint: err=%v, want ErrEmptyGrouping", err)
	}
	if got, _ := r1.Intersect(full); got != r1 {
		t.Errorf("intersect with full: got %+v, want %+v", got, r1)
	}
	if got, _ := r1.Intersect(open); !(got.Start == 25 && got.End == 30 && !got.Open) {
		t.Errorf("closed/open with overlap: got %+v, want {25,30}", got)
	}
}

func TestSubspaceRange_IncludesAndIntersect(t *testing.T) {
	t.Parallel()
	a := []byte{0x10}
	b := []byte{0x20}
	c := []byte{0x30}
	d := []byte{0x40}

	closed, _ := NewSubspaceRangeClosed(a, c)
	open := NewSubspaceRangeOpen(b)
	full := FullSubspaceRange()

	if !closed.IncludesValue(b) {
		t.Error("closed should include b")
	}
	if closed.IncludesValue(c) {
		t.Error("closed should exclude c (end-exclusive)")
	}
	if !open.IncludesValue(d) {
		t.Error("open should include d")
	}
	if !full.IncludesValue([]byte{}) {
		t.Error("full should include empty subspace")
	}

	inter, err := closed.Intersect(open)
	if err != nil || inter.Open {
		t.Fatalf("closed ∩ open: got %+v err=%v", inter, err)
	}
	if !bytes.Equal(inter.Start, b) || !bytes.Equal(inter.End, c) {
		t.Errorf("closed ∩ open: start/end %x %x, want %x %x", inter.Start, inter.End, b, c)
	}
}

func TestPathRange_IncludesAndIntersect(t *testing.T) {
	t.Parallel()
	a := pathOf(t, "a")
	b := pathOf(t, "b")
	c := pathOf(t, "c")
	d := pathOf(t, "d")

	closed, _ := NewPathRangeClosed(a, c)
	open := NewPathRangeOpen(b)
	full := FullPathRange(testLimits)

	if !closed.IncludesValue(b) {
		t.Error("closed should include b")
	}
	if closed.IncludesValue(c) {
		t.Error("closed should exclude c")
	}
	if !open.IncludesValue(d) {
		t.Error("open should include d")
	}
	if !full.IncludesValue(EmptyPath(testLimits)) {
		t.Error("full should include empty path")
	}

	if _, err := NewPathRangeClosed(c, a); !errors.Is(err, ErrEmptyGrouping) {
		t.Errorf("inverted range: err=%v, want ErrEmptyGrouping", err)
	}
}

func TestRange3d_IncludesAndIntersect(t *testing.T) {
	t.Parallel()
	subA, _ := NewSubspaceRangeClosed([]byte{0x10}, []byte{0x40})
	pathR, _ := NewPathRangeClosed(pathOf(t, "alpha"), pathOf(t, "zeta"))
	timeR, _ := NewTimeRangeClosed(100, 1000)
	r := Range3d{Subspaces: subA, Paths: pathR, Times: timeR}

	in := Coordinate{Subspace: []byte{0x20}, Path: pathOf(t, "beta"), Timestamp: 500}
	outSub := Coordinate{Subspace: []byte{0x50}, Path: pathOf(t, "beta"), Timestamp: 500}
	outPath := Coordinate{Subspace: []byte{0x20}, Path: pathOf(t, "zzz"), Timestamp: 500}
	outTime := Coordinate{Subspace: []byte{0x20}, Path: pathOf(t, "beta"), Timestamp: 50}

	if !r.Includes(in) {
		t.Error("in-range coordinate should be included")
	}
	if r.Includes(outSub) {
		t.Error("subspace-out coordinate should be excluded")
	}
	if r.Includes(outPath) {
		t.Error("path-out coordinate should be excluded")
	}
	if r.Includes(outTime) {
		t.Error("time-out coordinate should be excluded")
	}

	full := FullRange3d(testLimits)
	if !full.IsFull() {
		t.Error("FullRange3d should be full")
	}
	if !full.Includes(outSub) {
		t.Error("full range should include any coordinate")
	}

	inter, err := r.Intersect(full)
	if err != nil {
		t.Fatalf("intersect with full: %v", err)
	}
	if inter.Subspaces.Open || inter.Paths.Open || inter.Times.Open {
		t.Error("intersect with full should preserve r")
	}

	// Disjoint along time axis.
	otherTime, _ := NewTimeRangeClosed(2000, 3000)
	other := Range3d{Subspaces: subA, Paths: pathR, Times: otherTime}
	if _, err := r.Intersect(other); !errors.Is(err, ErrEmptyGrouping) {
		t.Errorf("disjoint intersect: err=%v, want ErrEmptyGrouping", err)
	}
}

func TestRange3d_IncludesEntry(t *testing.T) {
	t.Parallel()
	r := FullRange3d(testLimits)
	e := Entry{
		NamespaceID:   make([]byte, 32),
		SubspaceID:    []byte{0xAB},
		Path:          pathOf(t, "x"),
		Timestamp:     42,
		PayloadLength: 17,
		PayloadDigest: make([]byte, 32),
	}
	if !r.IncludesEntry(e) {
		t.Error("FullRange3d should include any entry")
	}
}
