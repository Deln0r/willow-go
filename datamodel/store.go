package datamodel

import (
	"bytes"
	"sync"
)

// Store is a collection of Entry values that enforces Willow's prefix-pruning
// invariant: at any moment, no entry in the store prunes any other entry in
// the store.
//
// Insert is the central operation. Get / Query / ForgetEntry are
// straightforward lookups. Pre-MVP scope deliberately omits payload-bytes
// storage, AuthorisedEntry wrapping, and Area-based queries — see
// TECH_DEBT.md for the planned follow-ups.
type Store interface {
	// Insert stores e if no existing entry prunes it, removing any existing
	// entries that e itself prunes. Returns (prunedCount, stored).
	// stored is false when e was outdated and discarded; true otherwise.
	// prunedCount is the number of older entries removed by inserting e.
	Insert(e Entry) (pruned int, stored bool)

	// Get returns the entry at (namespace, subspace, path), if any.
	Get(namespace, subspace []byte, path Path) (Entry, bool)

	// Query returns all entries within namespace whose coordinate falls in r.
	// The returned slice is a snapshot; callers may mutate it freely.
	Query(namespace []byte, r Range3d) []Entry

	// ForgetEntry removes the entry at (namespace, subspace, path) if present.
	// Returns true if an entry was removed. Forgetting is local-only: a
	// subsequent join with another peer's store may re-introduce the entry.
	ForgetEntry(namespace, subspace []byte, path Path) bool

	// Len returns the total number of entries currently in the store.
	Len() int
}

// InMemoryStore is a simple in-memory Store. Suitable for tests and small
// workloads; uses a linear scan for queries.
type InMemoryStore struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewInMemoryStore returns an empty in-memory store.
func NewInMemoryStore() *InMemoryStore { return &InMemoryStore{} }

// Insert implements Store.
func (s *InMemoryStore) Insert(e Entry) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, check whether any existing entry would prune e. If so, e is
	// outdated and is discarded without modifying the store.
	for _, existing := range s.entries {
		if existing.Prunes(e) {
			return 0, false
		}
	}

	// Then collect indices of existing entries that e prunes and drop them.
	pruned := 0
	kept := s.entries[:0]
	for _, existing := range s.entries {
		if e.Prunes(existing) {
			pruned++
			continue
		}
		kept = append(kept, existing)
	}
	s.entries = append(kept, cloneEntry(e))
	return pruned, true
}

// Get implements Store.
func (s *InMemoryStore) Get(namespace, subspace []byte, path Path) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if bytes.Equal(e.NamespaceID, namespace) &&
			bytes.Equal(e.SubspaceID, subspace) &&
			e.Path.Equal(path) {
			return cloneEntry(e), true
		}
	}
	return Entry{}, false
}

// Query implements Store.
func (s *InMemoryStore) Query(namespace []byte, r Range3d) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Entry
	for _, e := range s.entries {
		if !bytes.Equal(e.NamespaceID, namespace) {
			continue
		}
		if r.IncludesEntry(e) {
			out = append(out, cloneEntry(e))
		}
	}
	return out
}

// ForgetEntry implements Store.
func (s *InMemoryStore) ForgetEntry(namespace, subspace []byte, path Path) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if bytes.Equal(e.NamespaceID, namespace) &&
			bytes.Equal(e.SubspaceID, subspace) &&
			e.Path.Equal(path) {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return true
		}
	}
	return false
}

// Len implements Store.
func (s *InMemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func cloneEntry(e Entry) Entry {
	return Entry{
		NamespaceID:   cloneBytes(e.NamespaceID),
		SubspaceID:    cloneBytes(e.SubspaceID),
		Path:          e.Path,
		Timestamp:     e.Timestamp,
		PayloadLength: e.PayloadLength,
		PayloadDigest: cloneBytes(e.PayloadDigest),
	}
}
