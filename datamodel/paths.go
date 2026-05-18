// Package datamodel implements the Willow data model: Paths, Entries,
// Groupings, and the Store interface. See https://willowprotocol.org/specs/data-model/.
//
// This file implements Path. A Path is an ordered sequence of byte-slice
// components subject to three size constraints (max_component_length,
// max_component_count, max_path_length) that callers configure per-Path via a
// Limits value. Paths are immutable once constructed.
package datamodel

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Deln0r/willow-go/encoding"
)

// Limits bounds the size of a Path. willow25 fixes all three to 4096.
type Limits struct {
	MaxComponentLength int // MCL
	MaxComponentCount  int // MCC
	MaxPathLength      int // MPL
}

// Errors returned by Path constructors and decoders.
var (
	ErrPathTooLong       = errors.New("datamodel: path exceeds max_path_length")
	ErrTooManyComponents = errors.New("datamodel: path exceeds max_component_count")
	ErrComponentTooLong  = errors.New("datamodel: component exceeds max_component_length")
	ErrMalformedPath     = errors.New("datamodel: malformed path encoding")
)

// Path is an immutable Willow path.
type Path struct {
	components [][]byte
	limits     Limits
}

// EmptyPath returns a zero-component Path with the given limits.
func EmptyPath(limits Limits) Path {
	return Path{limits: limits}
}

// FromSlice constructs a single-component Path from a byte slice. The bytes
// are defensively copied.
func FromSlice(limits Limits, comp []byte) (Path, error) {
	if len(comp) > limits.MaxComponentLength {
		return Path{}, ErrComponentTooLong
	}
	if limits.MaxComponentCount < 1 {
		return Path{}, ErrTooManyComponents
	}
	if len(comp) > limits.MaxPathLength {
		return Path{}, ErrPathTooLong
	}
	owned := make([]byte, len(comp))
	copy(owned, comp)
	return Path{components: [][]byte{owned}, limits: limits}, nil
}

// FromSlices constructs a multi-component Path. All component byte slices are
// defensively copied.
func FromSlices(limits Limits, comps [][]byte) (Path, error) {
	if len(comps) > limits.MaxComponentCount {
		return Path{}, ErrTooManyComponents
	}
	total := 0
	for _, c := range comps {
		if len(c) > limits.MaxComponentLength {
			return Path{}, ErrComponentTooLong
		}
		total += len(c)
	}
	if total > limits.MaxPathLength {
		return Path{}, ErrPathTooLong
	}
	owned := make([][]byte, len(comps))
	for i, c := range comps {
		owned[i] = make([]byte, len(c))
		copy(owned[i], c)
	}
	return Path{components: owned, limits: limits}, nil
}

// ComponentCount returns the number of components in the path.
func (p Path) ComponentCount() int { return len(p.components) }

// TotalLength returns the sum of all component byte-lengths.
func (p Path) TotalLength() int {
	n := 0
	for _, c := range p.components {
		n += len(c)
	}
	return n
}

// Component returns the i-th component as a byte slice. The returned slice
// must not be mutated by callers.
func (p Path) Component(i int) []byte { return p.components[i] }

// Components returns all components as a slice of byte slices. The returned
// slices must not be mutated by callers.
func (p Path) Components() [][]byte { return p.components }

// Limits returns the limits this path was constructed with.
func (p Path) Limits() Limits { return p.limits }

// LongestCommonPrefix returns the longest path that is a prefix of both p and
// other. The result inherits p's limits.
func (p Path) LongestCommonPrefix(other Path) Path {
	n := len(p.components)
	if m := len(other.components); m < n {
		n = m
	}
	shared := 0
	for shared < n && bytes.Equal(p.components[shared], other.components[shared]) {
		shared++
	}
	return Path{components: p.components[:shared], limits: p.limits}
}

// IsEmpty reports whether p has zero components.
func (p Path) IsEmpty() bool { return len(p.components) == 0 }

// IsPrefixOf reports whether p is a prefix of other (component-wise).
// A path is a prefix of itself; the empty path is a prefix of every path.
func (p Path) IsPrefixOf(other Path) bool {
	if len(p.components) > len(other.components) {
		return false
	}
	for i, c := range p.components {
		if !bytes.Equal(c, other.components[i]) {
			return false
		}
	}
	return true
}

// IsPrefixedBy reports whether other is a prefix of p (inverse of IsPrefixOf).
func (p Path) IsPrefixedBy(other Path) bool { return other.IsPrefixOf(p) }

// IsRelatedTo reports whether either of p, other is a prefix of the other.
// Equivalent to "they share a common prefix that equals the shorter one."
func (p Path) IsRelatedTo(other Path) bool {
	return p.IsPrefixOf(other) || other.IsPrefixOf(p)
}

// LeastUpperBound returns the longer of two related paths and true. If p and
// other are not related (neither is a prefix of the other), returns the zero
// Path and false.
func (p Path) LeastUpperBound(other Path) (Path, bool) {
	if p.IsPrefixOf(other) {
		return other, true
	}
	if other.IsPrefixOf(p) {
		return p, true
	}
	return Path{}, false
}

// GreaterButNotPrefixed returns the least Path strictly greater than p in
// lex order that is NOT prefixed by p (in the path-prefix sense, i.e., does
// not extend p as a child). Returns (zero, false) if no such path fits within
// p's limits.
//
// Algorithm (from willow_rs paths/mod.rs): scan components right-to-left. For
// each component at index i, try two transformations in order:
//
//  1. Append a single 0x00 byte to component i (dropping components after i),
//     provided len(component[i]) < MCL and prefix_length_through_i < MPL. The
//     result is lex-greater than p and shares only the first i components
//     with p, so p is not a path-prefix of it.
//
//  2. Find the rightmost byte in component[i] that is < 0xFF. Truncate
//     component[i] to that byte's position+1 and increment that byte by 1,
//     dropping all components after i.
//
// If neither transformation succeeds for any component, returns false.
func (p Path) GreaterButNotPrefixed() (Path, bool) {
	prefixLen := 0
	// Precompute cumulative byte lengths so we can probe "prefix_length_through_i".
	cumLen := make([]int, len(p.components)+1)
	for i, c := range p.components {
		cumLen[i+1] = cumLen[i] + len(c)
	}

	for i := len(p.components) - 1; i >= 0; i-- {
		comp := p.components[i]
		prefixLen = cumLen[i+1] // bytes through and including component i

		// Branch 1: append 0x00 to component i.
		if len(comp) < p.limits.MaxComponentLength && prefixLen < p.limits.MaxPathLength {
			extended := make([]byte, len(comp)+1)
			copy(extended, comp)
			// extended[len(comp)] = 0 by default
			newComps := make([][]byte, i+1)
			for j := 0; j < i; j++ {
				newComps[j] = cloneBytes(p.components[j])
			}
			newComps[i] = extended
			return Path{components: newComps, limits: p.limits}, true
		}

		// Branch 2: find rightmost byte < 0xFF in comp and increment it.
		incrementAt := -1
		for j := len(comp) - 1; j >= 0; j-- {
			if comp[j] < 0xFF {
				incrementAt = j
				break
			}
		}
		if incrementAt >= 0 {
			truncated := make([]byte, incrementAt+1)
			copy(truncated, comp[:incrementAt+1])
			truncated[incrementAt]++
			newComps := make([][]byte, i+1)
			for j := 0; j < i; j++ {
				newComps[j] = cloneBytes(p.components[j])
			}
			newComps[i] = truncated
			return Path{components: newComps, limits: p.limits}, true
		}
	}
	return Path{}, false
}

// Compare returns -1, 0, or +1 comparing p to other lexicographically by
// component. When all shared components are equal, the shorter path is less.
// Matches the Willow spec ordering on paths.
func (p Path) Compare(other Path) int {
	n := len(p.components)
	if m := len(other.components); m < n {
		n = m
	}
	for i := 0; i < n; i++ {
		if c := bytes.Compare(p.components[i], other.components[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(p.components) < len(other.components):
		return -1
	case len(p.components) > len(other.components):
		return 1
	default:
		return 0
	}
}

// Equal reports whether p and other have identical components.
func (p Path) Equal(other Path) bool {
	if len(p.components) != len(other.components) {
		return false
	}
	for i, c := range p.components {
		if !bytes.Equal(c, other.components[i]) {
			return false
		}
	}
	return true
}

// Encode returns the byte-encoding of p per
// https://willowprotocol.org/specs/encodings/index.html#enc_path.
func (p Path) Encode() []byte {
	return appendAbsolutePath(nil, p.TotalLength(), p.components)
}

// EncodeRelativeTo returns the byte-encoding of p relative to ref per
// https://willowprotocol.org/specs/encodings/index.html#path_rel_path.
func (p Path) EncodeRelativeTo(ref Path) []byte {
	lcp := p.LongestCommonPrefix(ref)
	dst := encoding.AppendCU64Standalone(nil, uint64(lcp.ComponentCount()))
	suffix := p.components[lcp.ComponentCount():]
	suffixLen := p.TotalLength() - lcp.TotalLength()
	return appendAbsolutePath(dst, suffixLen, suffix)
}

// SuffixComponents returns the components of p starting from prefixCount.
// Panics if prefixCount > p.ComponentCount(). Returned slices share backing
// memory with p and must not be mutated.
func (p Path) SuffixComponents(prefixCount int) [][]byte {
	return p.components[prefixCount:]
}

// EncodeExtending returns the byte-encoding of p relative to prefix per
// https://willowprotocol.org/specs/encodings/index.html#path_extends_path,
// emitting only the suffix-as-absolute-path. Panics if prefix is not a
// path-prefix of p (caller responsibility).
func (p Path) EncodeExtending(prefix Path) []byte {
	if !prefix.IsPrefixOf(p) {
		panic("datamodel: EncodeExtending requires prefix to be a prefix of self")
	}
	suffix := p.SuffixComponents(prefix.ComponentCount())
	suffixLen := p.TotalLength() - prefix.TotalLength()
	return appendAbsolutePath(nil, suffixLen, suffix)
}

// DecodeExtending reads a path-extends-path encoding from src and returns
// the full reconstructed path (prefix + decoded suffix) plus bytes consumed.
// The reconstructed path uses prefix's Limits.
func DecodeExtending(prefix Path, src []byte) (Path, int, error) {
	suffix, n, err := Decode(prefix.limits, src)
	if err != nil {
		return Path{}, 0, err
	}
	combined := make([][]byte, 0, prefix.ComponentCount()+suffix.ComponentCount())
	for _, c := range prefix.components {
		clone := make([]byte, len(c))
		copy(clone, c)
		combined = append(combined, clone)
	}
	combined = append(combined, suffix.components...)
	out, err := FromSlices(prefix.limits, combined)
	if err != nil {
		return Path{}, 0, err
	}
	return out, n, nil
}

// Decode reads an absolute path encoding from src using the given limits.
// Returns the decoded path and the number of bytes consumed.
func Decode(limits Limits, src []byte) (Path, int, error) {
	pos := 0
	totalLength, count, n, err := decodePathHeader(src[pos:], limits)
	if err != nil {
		return Path{}, 0, err
	}
	pos += n

	components, n, err := decodeComponents(src[pos:], totalLength, count, limits)
	if err != nil {
		return Path{}, 0, err
	}
	pos += n
	return Path{components: components, limits: limits}, pos, nil
}

// DecodeRelative reads a relative path encoding (relative to ref) from src.
// Returns the decoded path and the number of bytes consumed.
func DecodeRelative(limits Limits, ref Path, src []byte) (Path, int, error) {
	pos := 0
	prefixCount, n, err := encoding.DecodeCU64Standalone(src[pos:], false)
	if err != nil {
		return Path{}, 0, wrapDecodeErr(err)
	}
	pos += n
	if int(prefixCount) > ref.ComponentCount() {
		return Path{}, 0, fmt.Errorf("%w: prefix count %d exceeds reference component count %d", ErrMalformedPath, prefixCount, ref.ComponentCount())
	}

	suffixLength, suffixCount, n, err := decodePathHeader(src[pos:], limits)
	if err != nil {
		return Path{}, 0, err
	}
	pos += n

	suffixComponents, n, err := decodeComponents(src[pos:], suffixLength, suffixCount, limits)
	if err != nil {
		return Path{}, 0, err
	}
	pos += n

	combined := make([][]byte, 0, int(prefixCount)+len(suffixComponents))
	for i := 0; i < int(prefixCount); i++ {
		clone := make([]byte, len(ref.components[i]))
		copy(clone, ref.components[i])
		combined = append(combined, clone)
	}
	combined = append(combined, suffixComponents...)

	out, err := FromSlices(limits, combined)
	if err != nil {
		return Path{}, 0, err
	}
	return out, pos, nil
}

// appendAbsolutePath writes the header byte (two packed 4-bit CompactU64 tags
// for total length and component count), the spillover int encodings, then
// each component prefixed by a standalone CompactU64 length — except the
// last, whose length the decoder infers.
func appendAbsolutePath(dst []byte, totalLength int, components [][]byte) []byte {
	var header uint8
	header = encoding.WriteTag(header, 4, 0, uint64(totalLength))
	header = encoding.WriteTag(header, 4, 4, uint64(len(components)))
	dst = append(dst, header)
	dst = encoding.AppendCU64(dst, uint64(totalLength), 4)
	dst = encoding.AppendCU64(dst, uint64(len(components)), 4)
	for i, c := range components {
		if i+1 != len(components) {
			dst = encoding.AppendCU64Standalone(dst, uint64(len(c)))
		}
		dst = append(dst, c...)
	}
	return dst
}

// decodePathHeader reads the header byte plus the two spillover CompactU64s
// and returns (totalLength, componentCount, bytesConsumed, err).
func decodePathHeader(src []byte, limits Limits) (totalLength, count int, consumed int, err error) {
	if len(src) < 1 {
		return 0, 0, 0, wrapDecodeErr(encoding.ErrShortBuffer)
	}
	header := src[0]
	pos := 1
	lengthTag := encoding.ExtractTag(header, 4, 0)
	countTag := encoding.ExtractTag(header, 4, 4)

	totalLength64, n, err := encoding.DecodeCU64(lengthTag, 4, src[pos:], false)
	if err != nil {
		return 0, 0, 0, wrapDecodeErr(err)
	}
	pos += n
	if totalLength64 > uint64(limits.MaxPathLength) {
		return 0, 0, 0, ErrPathTooLong
	}

	count64, n, err := encoding.DecodeCU64(countTag, 4, src[pos:], false)
	if err != nil {
		return 0, 0, 0, wrapDecodeErr(err)
	}
	pos += n
	if count64 > uint64(limits.MaxComponentCount) {
		return 0, 0, 0, ErrTooManyComponents
	}

	return int(totalLength64), int(count64), pos, nil
}

// decodeComponents reads `count` components totalling `totalLength` bytes
// from src. All component lengths except the last are explicit; the last is
// inferred.
func decodeComponents(src []byte, totalLength, count int, limits Limits) ([][]byte, int, error) {
	if count == 0 {
		if totalLength != 0 {
			return nil, 0, fmt.Errorf("%w: count 0 but non-zero total length %d", ErrMalformedPath, totalLength)
		}
		return nil, 0, nil
	}
	pos := 0
	components := make([][]byte, 0, count)
	consumedBytes := 0

	for i := 0; i < count-1; i++ {
		length64, n, err := encoding.DecodeCU64Standalone(src[pos:], false)
		if err != nil {
			return nil, 0, wrapDecodeErr(err)
		}
		pos += n
		// Guard against attacker-supplied huge length64 (e.g., 2^63+):
		// reject before int conversion which would otherwise become negative
		// and bypass the limit check.
		if length64 > uint64(limits.MaxComponentLength) {
			return nil, 0, ErrComponentTooLong
		}
		length := int(length64)
		if consumedBytes+length > totalLength {
			return nil, 0, fmt.Errorf("%w: component lengths exceed total path length", ErrMalformedPath)
		}
		if pos+length > len(src) {
			return nil, 0, wrapDecodeErr(encoding.ErrShortBuffer)
		}
		comp := make([]byte, length)
		copy(comp, src[pos:pos+length])
		components = append(components, comp)
		pos += length
		consumedBytes += length
	}

	// Final component: length inferred from totalLength minus the rest.
	finalLength := totalLength - consumedBytes
	if finalLength < 0 {
		return nil, 0, fmt.Errorf("%w: prior components overran total path length", ErrMalformedPath)
	}
	if finalLength > limits.MaxComponentLength {
		return nil, 0, ErrComponentTooLong
	}
	if pos+finalLength > len(src) {
		return nil, 0, wrapDecodeErr(encoding.ErrShortBuffer)
	}
	finalComp := make([]byte, finalLength)
	copy(finalComp, src[pos:pos+finalLength])
	components = append(components, finalComp)
	pos += finalLength

	return components, pos, nil
}

func wrapDecodeErr(err error) error {
	return fmt.Errorf("%w: %v", ErrMalformedPath, err)
}
