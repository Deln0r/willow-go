package datamodel

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Deln0r/willow-go/encoding"
)

// Entry is the metadata associated with a Willow payload. See
// https://willowprotocol.org/specs/data-model/index.html#Entry.
//
// NamespaceID, SubspaceID, and PayloadDigest are opaque byte strings whose
// length is determined out-of-band by the encoding parameters of the deployed
// Willow flavor (e.g. willow25 fixes all three to 32 bytes). They are encoded
// raw, with no length prefix.
type Entry struct {
	NamespaceID   []byte
	SubspaceID    []byte
	Path          Path
	Timestamp     uint64
	PayloadLength uint64
	PayloadDigest []byte
}

// EntrySpec carries the fixed-length parameters a decoder needs to parse an
// Entry encoding. The encoded stream itself does not include these widths,
// since both peers are expected to agree on the underlying Willow flavor
// before exchanging entries.
type EntrySpec struct {
	Limits              Limits
	NamespaceIDLength   int
	SubspaceIDLength    int
	PayloadDigestLength int
}

// Errors returned by Entry encoding / decoding.
var (
	ErrNamespaceIDLength   = errors.New("datamodel: namespace id length mismatch")
	ErrSubspaceIDLength    = errors.New("datamodel: subspace id length mismatch")
	ErrPayloadDigestLength = errors.New("datamodel: payload digest length mismatch")
)

// IsNewerThan reports whether e is strictly newer than other per the Willow
// data model tie-break order: timestamp first, then payload_digest lexicograpic,
// then payload_length. Used by prefix-pruning to decide which of two competing
// entries wins.
func (e Entry) IsNewerThan(other Entry) bool {
	if e.Timestamp != other.Timestamp {
		return e.Timestamp > other.Timestamp
	}
	if c := bytes.Compare(e.PayloadDigest, other.PayloadDigest); c != 0 {
		return c > 0
	}
	return e.PayloadLength > other.PayloadLength
}

// Prunes reports whether e prefix-prunes other per
// https://willowprotocol.org/specs/data-model/index.html#prefix_pruning:
// same namespace, same subspace, e.Path is a prefix of other.Path (or equal),
// and e is strictly newer than other.
func (e Entry) Prunes(other Entry) bool {
	if !bytes.Equal(e.NamespaceID, other.NamespaceID) {
		return false
	}
	if !bytes.Equal(e.SubspaceID, other.SubspaceID) {
		return false
	}
	if !e.Path.IsPrefixOf(other.Path) {
		return false
	}
	return e.IsNewerThan(other)
}

// Encode returns the canonical encoding of e per
// https://willowprotocol.org/specs/encodings/index.html#enc_entry.
func (e Entry) Encode() []byte {
	dst := make([]byte, 0, len(e.NamespaceID)+len(e.SubspaceID)+len(e.PayloadDigest)+32)
	dst = append(dst, e.NamespaceID...)
	dst = append(dst, e.SubspaceID...)
	dst = append(dst, e.Path.Encode()...)
	dst = encoding.AppendCU64Standalone(dst, e.Timestamp)
	dst = encoding.AppendCU64Standalone(dst, e.PayloadLength)
	dst = append(dst, e.PayloadDigest...)
	return dst
}

// DecodeEntry reads an entry encoding from src using the field widths and
// path limits in spec. Returns the decoded entry and the number of bytes
// consumed.
func DecodeEntry(spec EntrySpec, src []byte) (Entry, int, error) {
	pos := 0
	namespace, err := readFixed(src, &pos, spec.NamespaceIDLength, ErrNamespaceIDLength)
	if err != nil {
		return Entry{}, 0, err
	}
	subspace, err := readFixed(src, &pos, spec.SubspaceIDLength, ErrSubspaceIDLength)
	if err != nil {
		return Entry{}, 0, err
	}

	path, n, err := Decode(spec.Limits, src[pos:])
	if err != nil {
		return Entry{}, 0, fmt.Errorf("entry path: %w", err)
	}
	pos += n

	timestamp, n, err := encoding.DecodeCU64Standalone(src[pos:], false)
	if err != nil {
		return Entry{}, 0, fmt.Errorf("entry timestamp: %w", err)
	}
	pos += n

	payloadLength, n, err := encoding.DecodeCU64Standalone(src[pos:], false)
	if err != nil {
		return Entry{}, 0, fmt.Errorf("entry payload_length: %w", err)
	}
	pos += n

	digest, err := readFixed(src, &pos, spec.PayloadDigestLength, ErrPayloadDigestLength)
	if err != nil {
		return Entry{}, 0, err
	}

	return Entry{
		NamespaceID:   namespace,
		SubspaceID:    subspace,
		Path:          path,
		Timestamp:     timestamp,
		PayloadLength: payloadLength,
		PayloadDigest: digest,
	}, pos, nil
}

func readFixed(src []byte, pos *int, length int, lengthErr error) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("%w: negative length %d", lengthErr, length)
	}
	if *pos+length > len(src) {
		return nil, fmt.Errorf("entry: %w", encoding.ErrShortBuffer)
	}
	out := make([]byte, length)
	copy(out, src[*pos:*pos+length])
	*pos += length
	return out, nil
}
