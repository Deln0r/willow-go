package datamodel

import (
	"bytes"
	"errors"
)

// ErrEmptyGrouping indicates an operation (typically an intersection) would
// have produced an empty grouping, which the Willow data model forbids.
var ErrEmptyGrouping = errors.New("datamodel: empty grouping")

// Coordinate identifies a single point in 3-d Willow space — the (subspace,
// path, timestamp) triple used to test inclusion in a Range3d or Area.
type Coordinate struct {
	Subspace  []byte
	Path      Path
	Timestamp uint64
}

// TimeRange is a half-open range of timestamps: [Start, End) when Open is
// false, or [Start, +inf) when Open is true.
type TimeRange struct {
	Start uint64
	End   uint64
	Open  bool
}

// NewTimeRangeClosed returns a closed [start, end) TimeRange or
// ErrEmptyGrouping if start >= end.
func NewTimeRangeClosed(start, end uint64) (TimeRange, error) {
	if start >= end {
		return TimeRange{}, ErrEmptyGrouping
	}
	return TimeRange{Start: start, End: end}, nil
}

// NewTimeRangeOpen returns an open [start, +inf) TimeRange.
func NewTimeRangeOpen(start uint64) TimeRange {
	return TimeRange{Start: start, Open: true}
}

// FullTimeRange returns the TimeRange containing every timestamp.
func FullTimeRange() TimeRange { return TimeRange{Start: 0, Open: true} }

// IsFull reports whether r contains every timestamp.
func (r TimeRange) IsFull() bool { return r.Open && r.Start == 0 }

// IncludesValue reports whether t falls within r.
func (r TimeRange) IncludesValue(t uint64) bool {
	if t < r.Start {
		return false
	}
	if r.Open {
		return true
	}
	return t < r.End
}

// Intersect returns the intersection of r and other, or ErrEmptyGrouping if
// the result would be empty.
func (r TimeRange) Intersect(other TimeRange) (TimeRange, error) {
	start := r.Start
	if other.Start > start {
		start = other.Start
	}

	if r.Open && other.Open {
		return TimeRange{Start: start, Open: true}, nil
	}
	if r.Open {
		// other is closed
		if start >= other.End {
			return TimeRange{}, ErrEmptyGrouping
		}
		return TimeRange{Start: start, End: other.End}, nil
	}
	if other.Open {
		if start >= r.End {
			return TimeRange{}, ErrEmptyGrouping
		}
		return TimeRange{Start: start, End: r.End}, nil
	}
	end := r.End
	if other.End < end {
		end = other.End
	}
	if start >= end {
		return TimeRange{}, ErrEmptyGrouping
	}
	return TimeRange{Start: start, End: end}, nil
}

// SubspaceRange is a half-open range of subspace IDs ordered lexicographically.
// Start of nil is the least possible subspace.
type SubspaceRange struct {
	Start []byte
	End   []byte
	Open  bool
}

// NewSubspaceRangeClosed returns a closed [start, end) range, or
// ErrEmptyGrouping if bytes.Compare(start, end) >= 0.
func NewSubspaceRangeClosed(start, end []byte) (SubspaceRange, error) {
	if bytes.Compare(start, end) >= 0 {
		return SubspaceRange{}, ErrEmptyGrouping
	}
	return SubspaceRange{Start: cloneBytes(start), End: cloneBytes(end)}, nil
}

// NewSubspaceRangeOpen returns an open [start, +inf) range.
func NewSubspaceRangeOpen(start []byte) SubspaceRange {
	return SubspaceRange{Start: cloneBytes(start), Open: true}
}

// SingletonSubspaceRange returns the half-open range [s, s+0x00) which
// contains exactly s. Used by Area.AsRange3d to express a subspace-specific
// Area as a Range3d.
func SingletonSubspaceRange(s []byte) SubspaceRange {
	end := make([]byte, len(s)+1)
	copy(end, s)
	return SubspaceRange{Start: cloneBytes(s), End: end}
}

// FullSubspaceRange returns the SubspaceRange containing every subspace.
func FullSubspaceRange() SubspaceRange { return SubspaceRange{Open: true} }

// IsFull reports whether r contains every subspace.
func (r SubspaceRange) IsFull() bool { return r.Open && len(r.Start) == 0 }

// IncludesValue reports whether s falls within r.
func (r SubspaceRange) IncludesValue(s []byte) bool {
	if bytes.Compare(s, r.Start) < 0 {
		return false
	}
	if r.Open {
		return true
	}
	return bytes.Compare(s, r.End) < 0
}

// Intersect returns the intersection of r and other.
func (r SubspaceRange) Intersect(other SubspaceRange) (SubspaceRange, error) {
	start := r.Start
	if bytes.Compare(other.Start, start) > 0 {
		start = other.Start
	}

	if r.Open && other.Open {
		return SubspaceRange{Start: cloneBytes(start), Open: true}, nil
	}
	if r.Open {
		if bytes.Compare(start, other.End) >= 0 {
			return SubspaceRange{}, ErrEmptyGrouping
		}
		return SubspaceRange{Start: cloneBytes(start), End: cloneBytes(other.End)}, nil
	}
	if other.Open {
		if bytes.Compare(start, r.End) >= 0 {
			return SubspaceRange{}, ErrEmptyGrouping
		}
		return SubspaceRange{Start: cloneBytes(start), End: cloneBytes(r.End)}, nil
	}
	end := r.End
	if bytes.Compare(other.End, end) < 0 {
		end = other.End
	}
	if bytes.Compare(start, end) >= 0 {
		return SubspaceRange{}, ErrEmptyGrouping
	}
	return SubspaceRange{Start: cloneBytes(start), End: cloneBytes(end)}, nil
}

// PathRange is a half-open range of paths ordered per Path.Compare.
type PathRange struct {
	Start Path
	End   Path
	Open  bool
}

// NewPathRangeClosed returns a closed [start, end) range or ErrEmptyGrouping
// if start.Compare(end) >= 0.
func NewPathRangeClosed(start, end Path) (PathRange, error) {
	if start.Compare(end) >= 0 {
		return PathRange{}, ErrEmptyGrouping
	}
	return PathRange{Start: start, End: end}, nil
}

// NewPathRangeOpen returns an open [start, +inf) range.
func NewPathRangeOpen(start Path) PathRange {
	return PathRange{Start: start, Open: true}
}

// FullPathRange returns the PathRange containing every path. The empty path
// is the least possible path.
func FullPathRange(limits Limits) PathRange {
	return PathRange{Start: EmptyPath(limits), Open: true}
}

// IsFull reports whether r contains every path.
func (r PathRange) IsFull() bool {
	return r.Open && r.Start.ComponentCount() == 0
}

// IncludesValue reports whether p falls within r.
func (r PathRange) IncludesValue(p Path) bool {
	if p.Compare(r.Start) < 0 {
		return false
	}
	if r.Open {
		return true
	}
	return p.Compare(r.End) < 0
}

// Intersect returns the intersection of r and other.
func (r PathRange) Intersect(other PathRange) (PathRange, error) {
	start := r.Start
	if other.Start.Compare(start) > 0 {
		start = other.Start
	}

	if r.Open && other.Open {
		return PathRange{Start: start, Open: true}, nil
	}
	if r.Open {
		if start.Compare(other.End) >= 0 {
			return PathRange{}, ErrEmptyGrouping
		}
		return PathRange{Start: start, End: other.End}, nil
	}
	if other.Open {
		if start.Compare(r.End) >= 0 {
			return PathRange{}, ErrEmptyGrouping
		}
		return PathRange{Start: start, End: r.End}, nil
	}
	end := r.End
	if other.End.Compare(end) < 0 {
		end = other.End
	}
	if start.Compare(end) >= 0 {
		return PathRange{}, ErrEmptyGrouping
	}
	return PathRange{Start: start, End: end}, nil
}

// Range3d is a 3-d box in Willow space: a subspace range, a path range, and
// a time range. See https://willowprotocol.org/specs/grouping-entries/index.html#D3Range.
type Range3d struct {
	Subspaces SubspaceRange
	Paths     PathRange
	Times     TimeRange
}

// FullRange3d returns the Range3d containing every coordinate.
func FullRange3d(limits Limits) Range3d {
	return Range3d{
		Subspaces: FullSubspaceRange(),
		Paths:     FullPathRange(limits),
		Times:     FullTimeRange(),
	}
}

// IsFull reports whether r contains every coordinate.
func (r Range3d) IsFull() bool {
	return r.Subspaces.IsFull() && r.Paths.IsFull() && r.Times.IsFull()
}

// Includes reports whether the given coordinate falls within r.
func (r Range3d) Includes(c Coordinate) bool {
	return r.Subspaces.IncludesValue(c.Subspace) &&
		r.Paths.IncludesValue(c.Path) &&
		r.Times.IncludesValue(c.Timestamp)
}

// IncludesEntry reports whether e (interpreted as a Coordinate) falls within r.
func (r Range3d) IncludesEntry(e Entry) bool {
	return r.Includes(Coordinate{Subspace: e.SubspaceID, Path: e.Path, Timestamp: e.Timestamp})
}

// Intersect returns the intersection of r and other, or ErrEmptyGrouping if
// the result would be empty along any axis.
func (r Range3d) Intersect(other Range3d) (Range3d, error) {
	subs, err := r.Subspaces.Intersect(other.Subspaces)
	if err != nil {
		return Range3d{}, err
	}
	paths, err := r.Paths.Intersect(other.Paths)
	if err != nil {
		return Range3d{}, err
	}
	times, err := r.Times.Intersect(other.Times)
	if err != nil {
		return Range3d{}, err
	}
	return Range3d{Subspaces: subs, Paths: paths, Times: times}, nil
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
