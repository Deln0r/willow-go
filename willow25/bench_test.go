package willow25

import (
	"crypto/rand"
	"testing"
)

func BenchmarkHashPayload_32B(b *testing.B) {
	p := make([]byte, 32)
	rand.Read(p)
	b.ResetTimer()
	b.SetBytes(int64(len(p)))
	for i := 0; i < b.N; i++ {
		_ = HashPayload(p)
	}
}

func BenchmarkHashPayload_1KB(b *testing.B) {
	p := make([]byte, 1024)
	rand.Read(p)
	b.ResetTimer()
	b.SetBytes(int64(len(p)))
	for i := 0; i < b.N; i++ {
		_ = HashPayload(p)
	}
}

func BenchmarkHashPayload_8KB(b *testing.B) {
	p := make([]byte, 8192)
	rand.Read(p)
	b.ResetTimer()
	b.SetBytes(int64(len(p)))
	for i := 0; i < b.N; i++ {
		_ = HashPayload(p)
	}
}

func BenchmarkHashPayload_1MB(b *testing.B) {
	p := make([]byte, 1024*1024)
	rand.Read(p)
	b.ResetTimer()
	b.SetBytes(int64(len(p)))
	for i := 0; i < b.N; i++ {
		_ = HashPayload(p)
	}
}
