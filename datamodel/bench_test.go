package datamodel

import (
	"crypto/rand"
	"testing"
)

func benchLimits() Limits {
	return Limits{MaxComponentLength: 4096, MaxComponentCount: 4096, MaxPathLength: 4096}
}

func benchPath(t require, count int, compLen int) Path {
	comps := make([][]byte, count)
	for i := range comps {
		b := make([]byte, compLen)
		rand.Read(b)
		comps[i] = b
	}
	p, err := FromSlices(benchLimits(), comps)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type require interface {
	Fatal(args ...any)
}

func BenchmarkPath_Encode_Small(b *testing.B) {
	p := benchPath(b, 3, 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Encode()
	}
}

func BenchmarkPath_Encode_Medium(b *testing.B) {
	p := benchPath(b, 8, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Encode()
	}
}

func BenchmarkPath_Encode_Large(b *testing.B) {
	p := benchPath(b, 32, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Encode()
	}
}

func BenchmarkPath_Decode_Small(b *testing.B) {
	p := benchPath(b, 3, 16)
	encoded := p.Encode()
	limits := benchLimits()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = Decode(limits, encoded)
	}
}

func BenchmarkPath_Decode_Medium(b *testing.B) {
	p := benchPath(b, 8, 64)
	encoded := p.Encode()
	limits := benchLimits()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = Decode(limits, encoded)
	}
}

func BenchmarkEntry_Encode(b *testing.B) {
	p := benchPath(b, 3, 16)
	ns := make([]byte, 32)
	sub := make([]byte, 32)
	digest := make([]byte, 32)
	rand.Read(ns)
	rand.Read(sub)
	rand.Read(digest)
	e := Entry{
		NamespaceID:   ns,
		SubspaceID:    sub,
		Path:          p,
		Timestamp:     1700000000,
		PayloadLength: 1024,
		PayloadDigest: digest,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.Encode()
	}
}

func BenchmarkEntry_Decode(b *testing.B) {
	p := benchPath(b, 3, 16)
	ns := make([]byte, 32)
	sub := make([]byte, 32)
	digest := make([]byte, 32)
	rand.Read(ns)
	rand.Read(sub)
	rand.Read(digest)
	e := Entry{
		NamespaceID:   ns,
		SubspaceID:    sub,
		Path:          p,
		Timestamp:     1700000000,
		PayloadLength: 1024,
		PayloadDigest: digest,
	}
	encoded := e.Encode()
	spec := EntrySpec{
		Limits:              benchLimits(),
		NamespaceIDLength:   32,
		SubspaceIDLength:    32,
		PayloadDigestLength: 32,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = DecodeEntry(spec, encoded)
	}
}

func BenchmarkStore_Insert_Empty(b *testing.B) {
	p := benchPath(b, 3, 16)
	ns := make([]byte, 32)
	sub := make([]byte, 32)
	digest := make([]byte, 32)
	rand.Read(ns)
	rand.Read(sub)
	rand.Read(digest)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := NewInMemoryStore()
		s.Insert(Entry{
			NamespaceID:   ns,
			SubspaceID:    sub,
			Path:          p,
			Timestamp:     uint64(i),
			PayloadLength: 1024,
			PayloadDigest: digest,
		})
	}
}

func BenchmarkStore_Query_1000Entries_Linear(b *testing.B) {
	ns := make([]byte, 32)
	rand.Read(ns)
	s := NewInMemoryStore()
	for i := 0; i < 1000; i++ {
		sub := make([]byte, 32)
		rand.Read(sub)
		digest := make([]byte, 32)
		rand.Read(digest)
		p := benchPath(b, 2, 8)
		s.Insert(Entry{
			NamespaceID:   ns,
			SubspaceID:    sub,
			Path:          p,
			Timestamp:     uint64(i),
			PayloadLength: 100,
			PayloadDigest: digest,
		})
	}
	r := FullRange3d(benchLimits())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Query(ns, r)
	}
}
