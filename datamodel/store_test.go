package datamodel

import (
	"bytes"
	"testing"
)

var (
	nsA = []byte("namespace-a")
	nsB = []byte("namespace-b")
	subA = []byte("subspace-a")
	subB = []byte("subspace-b")
	digestA = []byte("digest-a-padding-padding-padding")
	digestB = []byte("digest-b-padding-padding-padding")
)

func makeEntry(t *testing.T, ns, sub []byte, comps []string, ts uint64, plen uint64, digest []byte) Entry {
	t.Helper()
	byteComps := make([][]byte, len(comps))
	for i, c := range comps {
		byteComps[i] = []byte(c)
	}
	p, err := FromSlices(testLimits, byteComps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	return Entry{
		NamespaceID:   ns,
		SubspaceID:    sub,
		Path:          p,
		Timestamp:     ts,
		PayloadLength: plen,
		PayloadDigest: digest,
	}
}

func TestEntry_IsNewerThan(t *testing.T) {
	t.Parallel()
	older := makeEntry(t, nsA, subA, []string{"x"}, 100, 10, digestA)
	newerTs := makeEntry(t, nsA, subA, []string{"x"}, 200, 10, digestA)
	sameTsBigDigest := makeEntry(t, nsA, subA, []string{"x"}, 100, 10, digestB)
	sameAllBigLen := makeEntry(t, nsA, subA, []string{"x"}, 100, 11, digestA)

	if !newerTs.IsNewerThan(older) {
		t.Error("newer timestamp should be newer")
	}
	if older.IsNewerThan(newerTs) {
		t.Error("older timestamp should not be newer")
	}
	if !sameTsBigDigest.IsNewerThan(older) {
		t.Error("same timestamp, bigger digest should be newer")
	}
	if !sameAllBigLen.IsNewerThan(older) {
		t.Error("same timestamp + digest, bigger length should be newer")
	}
	if older.IsNewerThan(older) {
		t.Error("entry is not newer than itself")
	}
}

func TestEntry_Prunes(t *testing.T) {
	t.Parallel()
	short := makeEntry(t, nsA, subA, []string{"folder"}, 200, 0, digestA)
	deep := makeEntry(t, nsA, subA, []string{"folder", "file"}, 100, 5, digestA)
	deepNewer := makeEntry(t, nsA, subA, []string{"folder", "file"}, 300, 5, digestA)
	otherSub := makeEntry(t, nsA, subB, []string{"folder"}, 50, 0, digestA)
	otherNs := makeEntry(t, nsB, subA, []string{"folder"}, 50, 0, digestA)
	siblingDeep := makeEntry(t, nsA, subA, []string{"other", "file"}, 50, 5, digestA)

	if !short.Prunes(deep) {
		t.Error("shorter newer should prune deeper older")
	}
	if deep.Prunes(short) {
		t.Error("deeper path is not a prefix of shorter, cannot prune")
	}
	if short.Prunes(deepNewer) {
		t.Error("shorter older should not prune deeper newer")
	}
	if short.Prunes(otherSub) {
		t.Error("different subspace should not be pruned")
	}
	if short.Prunes(otherNs) {
		t.Error("different namespace should not be pruned")
	}
	if short.Prunes(siblingDeep) {
		t.Error("sibling deep path is not prefixed by short.Path")
	}
}

func TestStore_InsertIntoEmpty(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	e := makeEntry(t, nsA, subA, []string{"a"}, 100, 1, digestA)
	pruned, stored := s.Insert(e)
	if pruned != 0 || !stored {
		t.Errorf("Insert into empty: got pruned=%d stored=%v, want 0 true", pruned, stored)
	}
	if s.Len() != 1 {
		t.Errorf("Len: got %d, want 1", s.Len())
	}
}

func TestStore_PruneOlderDeeperEntry(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	older := makeEntry(t, nsA, subA, []string{"folder", "file"}, 100, 5, digestA)
	newerShallower := makeEntry(t, nsA, subA, []string{"folder"}, 200, 0, digestA)

	s.Insert(older)
	pruned, stored := s.Insert(newerShallower)
	if pruned != 1 {
		t.Errorf("expected 1 pruned entry, got %d", pruned)
	}
	if !stored {
		t.Error("newer shallower entry should be stored")
	}
	if s.Len() != 1 {
		t.Errorf("Len: got %d, want 1", s.Len())
	}
	if _, ok := s.Get(nsA, subA, older.Path); ok {
		t.Error("older deeper entry should be gone")
	}
	if _, ok := s.Get(nsA, subA, newerShallower.Path); !ok {
		t.Error("newer shallower entry should be present")
	}
}

func TestStore_OutdatedInsertIsDiscarded(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	newerShallow := makeEntry(t, nsA, subA, []string{"folder"}, 200, 0, digestA)
	olderDeep := makeEntry(t, nsA, subA, []string{"folder", "file"}, 100, 5, digestA)

	s.Insert(newerShallow)
	pruned, stored := s.Insert(olderDeep)
	if pruned != 0 || stored {
		t.Errorf("outdated insert: got pruned=%d stored=%v, want 0 false", pruned, stored)
	}
	if s.Len() != 1 {
		t.Errorf("Len: got %d, want 1", s.Len())
	}
}

func TestStore_OverwriteAtSamePath(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	old := makeEntry(t, nsA, subA, []string{"file"}, 100, 5, digestA)
	new := makeEntry(t, nsA, subA, []string{"file"}, 200, 7, digestB)

	s.Insert(old)
	pruned, stored := s.Insert(new)
	if pruned != 1 || !stored {
		t.Errorf("overwrite: got pruned=%d stored=%v, want 1 true", pruned, stored)
	}
	got, ok := s.Get(nsA, subA, new.Path)
	if !ok || got.Timestamp != 200 || got.PayloadLength != 7 {
		t.Errorf("Get: got %+v ok=%v, want timestamp=200 length=7", got, ok)
	}
}

func TestStore_NamespaceIsolation(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	a := makeEntry(t, nsA, subA, []string{"x"}, 100, 1, digestA)
	b := makeEntry(t, nsB, subA, []string{"x"}, 100, 1, digestA)

	s.Insert(a)
	s.Insert(b)
	if s.Len() != 2 {
		t.Errorf("Len: got %d, want 2 (different namespaces should coexist)", s.Len())
	}
	if _, ok := s.Get(nsA, subA, a.Path); !ok {
		t.Error("nsA entry should be present")
	}
	if _, ok := s.Get(nsB, subA, b.Path); !ok {
		t.Error("nsB entry should be present")
	}
}

func TestStore_QueryByRange3d(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	e1 := makeEntry(t, nsA, []byte{0x10}, []string{"alpha"}, 100, 1, digestA)
	e2 := makeEntry(t, nsA, []byte{0x20}, []string{"beta"}, 200, 1, digestA)
	e3 := makeEntry(t, nsA, []byte{0x30}, []string{"gamma"}, 300, 1, digestA)
	e4 := makeEntry(t, nsB, []byte{0x20}, []string{"beta"}, 200, 1, digestA)
	for _, e := range []Entry{e1, e2, e3, e4} {
		s.Insert(e)
	}

	subRange, _ := NewSubspaceRangeClosed([]byte{0x10}, []byte{0x30})
	pathRange, _ := NewPathRangeClosed(pathOf(t, "alpha"), pathOf(t, "delta"))
	timeRange, _ := NewTimeRangeClosed(100, 300)
	r := Range3d{Subspaces: subRange, Paths: pathRange, Times: timeRange}

	results := s.Query(nsA, r)
	if len(results) != 2 {
		t.Fatalf("Query: got %d results, want 2 (e1 alpha t100 and e2 beta t200)", len(results))
	}
	gotPaths := map[string]bool{}
	for _, e := range results {
		gotPaths[string(e.Path.Encode())] = true
	}
	if !gotPaths[string(e1.Path.Encode())] || !gotPaths[string(e2.Path.Encode())] {
		t.Errorf("expected e1 and e2 in results, got paths %v", gotPaths)
	}

	full := FullRange3d(testLimits)
	if got := s.Query(nsA, full); len(got) != 3 {
		t.Errorf("Query(nsA, full): got %d, want 3", len(got))
	}
	if got := s.Query(nsB, full); len(got) != 1 {
		t.Errorf("Query(nsB, full): got %d, want 1", len(got))
	}
}

func TestStore_ForgetEntry(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	e := makeEntry(t, nsA, subA, []string{"x"}, 100, 1, digestA)
	s.Insert(e)
	if !s.ForgetEntry(nsA, subA, e.Path) {
		t.Error("ForgetEntry: expected true on existing entry")
	}
	if s.Len() != 0 {
		t.Errorf("Len after forget: got %d, want 0", s.Len())
	}
	if s.ForgetEntry(nsA, subA, e.Path) {
		t.Error("ForgetEntry: expected false on absent entry")
	}
}

func TestStore_GetReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()
	s := NewInMemoryStore()
	e := makeEntry(t, nsA, subA, []string{"x"}, 100, 1, digestA)
	s.Insert(e)

	got, ok := s.Get(nsA, subA, e.Path)
	if !ok {
		t.Fatal("Get: expected entry")
	}
	// Mutating the returned slice must not affect the stored entry.
	if len(got.NamespaceID) > 0 {
		got.NamespaceID[0] ^= 0xff
	}
	stored, _ := s.Get(nsA, subA, e.Path)
	if !bytes.Equal(stored.NamespaceID, nsA) {
		t.Errorf("Get did not return a defensive copy: stored NS now %x", stored.NamespaceID)
	}
}
