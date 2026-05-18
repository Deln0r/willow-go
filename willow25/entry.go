package willow25

import (
	"fmt"

	"github.com/Deln0r/willow-go/datamodel"
)

// NewEntry returns a fully populated datamodel.Entry for a Willow'25
// payload. It hashes the payload to derive PayloadDigest and sets
// PayloadLength to len(payload). NamespaceID and SubspaceID must be 32
// bytes each.
func NewEntry(
	namespaceID, subspaceID []byte,
	path datamodel.Path,
	timestamp uint64,
	payload []byte,
) (datamodel.Entry, error) {
	if len(namespaceID) != NamespaceIDWidth {
		return datamodel.Entry{}, fmt.Errorf("willow25: namespace id must be %d bytes, got %d", NamespaceIDWidth, len(namespaceID))
	}
	if len(subspaceID) != SubspaceIDWidth {
		return datamodel.Entry{}, fmt.Errorf("willow25: subspace id must be %d bytes, got %d", SubspaceIDWidth, len(subspaceID))
	}
	digest := HashPayload(payload)
	return datamodel.Entry{
		NamespaceID:   append([]byte(nil), namespaceID...),
		SubspaceID:    append([]byte(nil), subspaceID...),
		Path:          path,
		Timestamp:     timestamp,
		PayloadLength: uint64(len(payload)),
		PayloadDigest: digest[:],
	}, nil
}

// NewPath is a thin wrapper around datamodel.FromSlices that uses the
// Willow'25 Limits, suitable for building paths without sprinkling
// willow25.Limits() at every call site.
func NewPath(components [][]byte) (datamodel.Path, error) {
	return datamodel.FromSlices(Limits(), components)
}
