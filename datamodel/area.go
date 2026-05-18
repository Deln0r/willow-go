package datamodel

import "bytes"

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
