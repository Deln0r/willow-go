package encoding

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrShortBuffer indicates the input is shorter than the CompactU64 encoding
// requires.
var ErrShortBuffer = errors.New("encoding: short buffer")

// ErrNotMinimal indicates a CompactU64 value was encoded with more bytes than
// strictly necessary, and the decoder was operating in canonical mode.
var ErrNotMinimal = errors.New("encoding: non-minimal CompactU64")

// MaxTag returns the maximal tag value for a given tag width (2..=8), i.e.,
// 2^tagWidth - 1.
func MaxTag(tagWidth uint8) uint8 {
	assertTagWidth(tagWidth)
	return uint8((uint16(1) << tagWidth) - 1)
}

// MinWidth returns the number of follow-up bytes (0, 1, 2, 4, or 8) needed to
// encode n with a tag of the given width.
func MinWidth(n uint64, tagWidth uint8) int {
	assertTagWidth(tagWidth)
	maxInline := (uint64(1) << tagWidth) - 4
	switch {
	case n < maxInline:
		return 0
	case n < 1<<8:
		return 1
	case n < 1<<16:
		return 2
	case n < 1<<32:
		return 4
	default:
		return 8
	}
}

// MinTag returns the minimal tag value used to encode n at the given tag
// width.
func MinTag(n uint64, tagWidth uint8) uint8 {
	assertTagWidth(tagWidth)
	maxInline := (uint64(1) << tagWidth) - 4
	if n < maxInline {
		return uint8(n)
	}
	maxTag := MaxTag(tagWidth)
	switch {
	case n < 1<<8:
		return maxTag - 3
	case n < 1<<16:
		return maxTag - 2
	case n < 1<<32:
		return maxTag - 1
	default:
		return maxTag
	}
}

// WriteTag returns tagByte with the minimal tag for n OR'd in at the given
// offset (0 = most significant bit, must satisfy tagOffset+tagWidth <= 8).
func WriteTag(tagByte uint8, tagWidth, tagOffset uint8, n uint64) uint8 {
	assertTagWidth(tagWidth)
	assertTagOffset(tagOffset, tagWidth)
	shift := 8 - (tagOffset + tagWidth)
	return tagByte | (MinTag(n, tagWidth) << shift)
}

// ExtractTag returns the tag value at the given offset/width inside tagByte.
func ExtractTag(tagByte, tagWidth, tagOffset uint8) uint8 {
	assertTagWidth(tagWidth)
	assertTagOffset(tagOffset, tagWidth)
	shift := 8 - (tagOffset + tagWidth)
	return (tagByte >> shift) & MaxTag(tagWidth)
}

// EncodingWidthFromTag returns the number of follow-up bytes (0/1/2/4/8) the
// given tag value implies at the given tag width.
func EncodingWidthFromTag(tag, tagWidth uint8) int {
	switch MaxTag(tagWidth) - tag {
	case 0:
		return 8
	case 1:
		return 4
	case 2:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
}

// AppendCU64 appends the minimal int encoding of n for the given tag width to
// dst. Returns the extended slice. Writes 0, 1, 2, 4, or 8 bytes (big-endian).
// Does not write the tag itself — callers compose tags into a header byte
// separately via WriteTag.
func AppendCU64(dst []byte, n uint64, tagWidth uint8) []byte {
	switch MinWidth(n, tagWidth) {
	case 0:
		return dst
	case 1:
		return append(dst, uint8(n))
	case 2:
		return binary.BigEndian.AppendUint16(dst, uint16(n))
	case 4:
		return binary.BigEndian.AppendUint32(dst, uint32(n))
	case 8:
		return binary.BigEndian.AppendUint64(dst, n)
	}
	panic("unreachable")
}

// AppendCU64Standalone appends the 8-bit minimal tag for n followed by its
// int encoding to dst. Returns the extended slice. Writes 1, 2, 3, 5, or 9
// bytes total.
func AppendCU64Standalone(dst []byte, n uint64) []byte {
	dst = append(dst, MinTag(n, 8))
	return AppendCU64(dst, n, 8)
}

// DecodeCU64 reads the int encoding implied by the given tag from src and
// returns (value, bytesConsumed). The tag must have been extracted from a
// tagByte separately. When canonical is true, returns ErrNotMinimal if the
// encoding used more bytes than necessary.
func DecodeCU64(tag, tagWidth uint8, src []byte, canonical bool) (uint64, int, error) {
	width := EncodingWidthFromTag(tag, tagWidth)
	if len(src) < width {
		return 0, 0, ErrShortBuffer
	}
	var v uint64
	switch width {
	case 0:
		v = uint64(tag)
	case 1:
		v = uint64(src[0])
	case 2:
		v = uint64(binary.BigEndian.Uint16(src[:2]))
	case 4:
		v = uint64(binary.BigEndian.Uint32(src[:4]))
	case 8:
		v = binary.BigEndian.Uint64(src[:8])
	}
	if canonical && MinTag(v, tagWidth) != tag {
		return 0, 0, ErrNotMinimal
	}
	return v, width, nil
}

// DecodeCU64Standalone reads an 8-bit tag from src followed by its int
// encoding. Returns (value, bytesConsumed). Pass canonical=true to reject
// non-minimal encodings.
func DecodeCU64Standalone(src []byte, canonical bool) (uint64, int, error) {
	if len(src) < 1 {
		return 0, 0, ErrShortBuffer
	}
	tag := src[0]
	v, n, err := DecodeCU64(tag, 8, src[1:], canonical)
	if err != nil {
		return 0, 0, err
	}
	return v, n + 1, nil
}

func assertTagWidth(tagWidth uint8) {
	if tagWidth < 2 || tagWidth > 8 {
		panic(fmt.Sprintf("encoding: tag width %d out of range [2,8]", tagWidth))
	}
}

func assertTagOffset(tagOffset, tagWidth uint8) {
	if tagOffset+tagWidth > 8 {
		panic(fmt.Sprintf("encoding: tag offset %d + width %d exceeds 8", tagOffset, tagWidth))
	}
}
