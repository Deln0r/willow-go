package datamodel

import (
	"bytes"
	"fmt"

	"github.com/Deln0r/willow-go/encoding"
)

// Area is a Willow area: a (possibly any-subspace) shaped grouping of entries
// whose paths extend a given prefix and whose timestamps fall in a TimeRange.
// See https://willowprotocol.org/specs/grouping-entries/#Area.
//
// Subspace is a pointer because nil means "any subspace" — distinct from an
// empty byte slice which would mean "exactly the empty subspace id".
// PathPrefix is the prefix all included entries' paths must extend (or equal).
type Area struct {
	Subspace   *[]byte
	PathPrefix Path
	Times      TimeRange
}

// FullArea returns the Area containing every coordinate.
func FullArea(limits Limits) Area {
	return Area{
		Subspace:   nil,
		PathPrefix: EmptyPath(limits),
		Times:      FullTimeRange(),
	}
}

// SubspaceArea returns the Area containing exactly the entries whose subspace
// id equals s, with any path and any timestamp.
func SubspaceArea(limits Limits, s []byte) Area {
	cloned := cloneBytes(s)
	return Area{
		Subspace:   &cloned,
		PathPrefix: EmptyPath(limits),
		Times:      FullTimeRange(),
	}
}

// IsFull reports whether a contains every coordinate.
func (a Area) IsFull() bool {
	return a.Subspace == nil && a.PathPrefix.IsEmpty() && a.Times.IsFull()
}

// Includes reports whether the given coordinate is in a.
func (a Area) Includes(c Coordinate) bool {
	if !a.Times.IncludesValue(c.Timestamp) {
		return false
	}
	if a.Subspace != nil && !bytes.Equal(*a.Subspace, c.Subspace) {
		return false
	}
	return a.PathPrefix.IsPrefixOf(c.Path)
}

// IncludesEntry reports whether e (interpreted as a Coordinate) is in a.
func (a Area) IncludesEntry(e Entry) bool {
	return a.Includes(Coordinate{Subspace: e.SubspaceID, Path: e.Path, Timestamp: e.Timestamp})
}

// Intersect returns the Area containing exactly the coordinates included in
// both a and other, or ErrEmptyGrouping if the result would be empty.
func (a Area) Intersect(other Area) (Area, error) {
	if !a.PathPrefix.IsRelatedTo(other.PathPrefix) {
		return Area{}, ErrEmptyGrouping
	}
	times, err := a.Times.Intersect(other.Times)
	if err != nil {
		return Area{}, err
	}
	// Longer of the two path prefixes — Will be the more-specific path.
	path, _ := a.PathPrefix.LeastUpperBound(other.PathPrefix)

	// Combine subspace constraints. nil means "any".
	var sub *[]byte
	switch {
	case a.Subspace == nil && other.Subspace == nil:
		sub = nil
	case a.Subspace == nil:
		cloned := cloneBytes(*other.Subspace)
		sub = &cloned
	case other.Subspace == nil:
		cloned := cloneBytes(*a.Subspace)
		sub = &cloned
	default:
		if !bytes.Equal(*a.Subspace, *other.Subspace) {
			return Area{}, ErrEmptyGrouping
		}
		cloned := cloneBytes(*a.Subspace)
		sub = &cloned
	}

	return Area{Subspace: sub, PathPrefix: path, Times: times}, nil
}

// AsRange3d converts a to the equivalent Range3d. For the path axis, this
// uses Path.GreaterButNotPrefixed to compute the smallest path that is
// lex-greater than a.PathPrefix but is not in the subtree rooted at it; if
// no such path fits within the limits, falls back to an open range starting
// at a.PathPrefix.
func (a Area) AsRange3d() Range3d {
	var subs SubspaceRange
	if a.Subspace == nil {
		subs = FullSubspaceRange()
	} else {
		subs = SingletonSubspaceRange(*a.Subspace)
	}

	var paths PathRange
	if succ, ok := a.PathPrefix.GreaterButNotPrefixed(); ok {
		paths = PathRange{Start: a.PathPrefix, End: succ}
	} else {
		paths = NewPathRangeOpen(a.PathPrefix)
	}

	return Range3d{Subspaces: subs, Paths: paths, Times: a.Times}
}

// EncodeRelativeTo returns the byte-encoding of a relative to rel per the
// Willow spec encode_area_in_area
// (https://willowprotocol.org/specs/encodings/index.html#encode_area_in_area).
// Caller must ensure rel includes a (a.IsIncludedIn(rel) must hold) — this
// is a panic-on-misuse precondition matching the upstream debug_assert.
//
// Subspace ID width is implicit in the byte slices: encoder emits len(a.Subspace)
// raw bytes for the explicit-subspace case. Decoders must know the width
// out-of-band; see DecodeAreaRelativeTo.
func (a Area) EncodeRelativeTo(rel Area) []byte {
	if !rel.includesArea(a) {
		panic("datamodel: EncodeRelativeTo requires rel to include self")
	}

	selfStart := a.Times.Start
	relStart := rel.Times.Start
	subspaceEncoded := a.Subspace != nil && rel.Subspace == nil
	timesEndOpen := a.Times.Open

	// Compute start_diff and start_from_start.
	var startDiff uint64
	var startFromStart bool
	if rel.Times.Open {
		startDiff = selfStart - relStart
		startFromStart = true
	} else {
		fromStart := selfStart - relStart
		fromEnd := rel.Times.End - selfStart
		if fromStart < fromEnd {
			startDiff = fromStart
			startFromStart = true
		} else {
			startDiff = fromEnd
			startFromStart = false
		}
	}

	// Compute end_diff and end_from_start (only meaningful when self.times is closed).
	var endDiff uint64
	var endFromStart bool
	if !timesEndOpen {
		selfEnd := a.Times.End
		if rel.Times.Open {
			endDiff = selfEnd - relStart
			endFromStart = true
		} else {
			fromStart := selfEnd - relStart
			fromEnd := rel.Times.End - selfEnd
			if fromStart < fromEnd {
				endDiff = fromStart
				endFromStart = true
			} else {
				endDiff = fromEnd
				endFromStart = false
			}
		}
	}

	// Build header byte.
	var header uint8
	if subspaceEncoded {
		header |= 0b1000_0000
	}
	if timesEndOpen {
		header |= 0b0100_0000
	}
	if startFromStart {
		header |= 0b0010_0000
	}
	if !timesEndOpen && endFromStart {
		header |= 0b0001_0000
	}
	header = encoding.WriteTag(header, 2, 4, startDiff)
	if !timesEndOpen {
		header = encoding.WriteTag(header, 2, 6, endDiff)
	}
	// When times is open, bits 6-7 stay zero (matches the upstream "end_diff.unwrap_or(0)" call).

	dst := []byte{header}

	// Body.
	if subspaceEncoded {
		dst = append(dst, *a.Subspace...)
	}
	dst = encoding.AppendCU64(dst, startDiff, 2)
	if !timesEndOpen {
		dst = encoding.AppendCU64(dst, endDiff, 2)
	}
	dst = append(dst, a.PathPrefix.EncodeExtending(rel.PathPrefix)...)
	return dst
}

// DecodeAreaRelativeTo reads an area-in-area encoding from src using rel as
// the reference. subspaceWidth is the byte length of explicit subspace ids
// — needed when rel.Subspace is nil and the encoded area carries its own
// subspace bytes. Returns the decoded area and the number of bytes consumed.
func DecodeAreaRelativeTo(limits Limits, rel Area, subspaceWidth int, src []byte) (Area, int, error) {
	if len(src) < 1 {
		return Area{}, 0, fmt.Errorf("area: %w", encoding.ErrShortBuffer)
	}
	header := src[0]
	pos := 1

	subspaceEncoded := header&0b1000_0000 != 0
	timesEndOpen := header&0b0100_0000 != 0
	startFromStart := header&0b0010_0000 != 0
	endFromStart := header&0b0001_0000 != 0

	// Decode subspace.
	var sub *[]byte
	if subspaceEncoded {
		if rel.Subspace != nil {
			return Area{}, 0, fmt.Errorf("area: subspace_encoded bit set but rel has a concrete subspace")
		}
		if pos+subspaceWidth > len(src) {
			return Area{}, 0, fmt.Errorf("area subspace: %w", encoding.ErrShortBuffer)
		}
		owned := make([]byte, subspaceWidth)
		copy(owned, src[pos:pos+subspaceWidth])
		sub = &owned
		pos += subspaceWidth
	} else if rel.Subspace != nil {
		owned := cloneBytes(*rel.Subspace)
		sub = &owned
	} else {
		sub = nil
	}

	// Decode start_diff.
	startTag := encoding.ExtractTag(header, 2, 4)
	startDiff, n, err := encoding.DecodeCU64(startTag, 2, src[pos:], false)
	if err != nil {
		return Area{}, 0, fmt.Errorf("area start_diff: %w", err)
	}
	pos += n

	var start uint64
	if startFromStart {
		start = rel.Times.Start + startDiff
	} else {
		if rel.Times.Open {
			return Area{}, 0, fmt.Errorf("area: start_from_start=false requires rel.times closed")
		}
		if startDiff > rel.Times.End {
			return Area{}, 0, fmt.Errorf("area start_diff: subtraction underflow")
		}
		start = rel.Times.End - startDiff
	}

	// Decode end_diff (only when self.times is closed).
	var end uint64
	if !timesEndOpen {
		endTag := encoding.ExtractTag(header, 2, 6)
		endDiff, n, err := encoding.DecodeCU64(endTag, 2, src[pos:], false)
		if err != nil {
			return Area{}, 0, fmt.Errorf("area end_diff: %w", err)
		}
		pos += n
		if endFromStart {
			end = rel.Times.Start + endDiff
		} else {
			if rel.Times.Open {
				return Area{}, 0, fmt.Errorf("area: end_from_start=false requires rel.times closed")
			}
			if endDiff > rel.Times.End {
				return Area{}, 0, fmt.Errorf("area end_diff: subtraction underflow")
			}
			end = rel.Times.End - endDiff
		}
	}

	// Build times range.
	var times TimeRange
	if timesEndOpen {
		times = NewTimeRangeOpen(start)
	} else {
		var err error
		times, err = NewTimeRangeClosed(start, end)
		if err != nil {
			return Area{}, 0, fmt.Errorf("area times: %w", err)
		}
	}

	// Decode path-extends-path.
	path, n, err := DecodeExtending(rel.PathPrefix, src[pos:])
	if err != nil {
		return Area{}, 0, fmt.Errorf("area path: %w", err)
	}
	pos += n

	// Ensure limits are respected.
	if path.Limits().MaxPathLength != limits.MaxPathLength {
		// Reconstruct under the requested limits.
		comps := make([][]byte, len(path.components))
		copy(comps, path.components)
		path, err = FromSlices(limits, comps)
		if err != nil {
			return Area{}, 0, fmt.Errorf("area path: %w", err)
		}
	}

	return Area{Subspace: sub, PathPrefix: path, Times: times}, pos, nil
}

// includesArea reports whether self contains the entire other area (every
// coordinate in other is also in self). Used as the precondition for
// EncodeRelativeTo.
func (a Area) includesArea(other Area) bool {
	// Subspace: a.Subspace=None accepts any other.Subspace; otherwise they
	// must equal.
	if a.Subspace != nil {
		if other.Subspace == nil {
			return false
		}
		if !bytes.Equal(*a.Subspace, *other.Subspace) {
			return false
		}
	}
	// Path: a.PathPrefix must be a prefix of other.PathPrefix.
	if !a.PathPrefix.IsPrefixOf(other.PathPrefix) {
		return false
	}
	// Times: a.Times must contain other.Times.
	if other.Times.Start < a.Times.Start {
		return false
	}
	if a.Times.Open {
		return true
	}
	// a closed: other must also be closed (else other extends past a)
	if other.Times.Open {
		return false
	}
	return other.Times.End <= a.Times.End
}
