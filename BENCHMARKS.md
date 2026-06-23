# Benchmarks

Performance numbers for willow-go's core encoders, decoders, and hash. All measurements taken on a single workstation; reproducible via `make bench` (or `go test -bench=. -benchmem -run=^$ ./...`).

**Test setup:**

- CPU: Apple M3 (8-core, 4 perf + 4 eff)
- OS: macOS 26.3.0 (Darwin 25.3.0)
- Go: 1.26.3
- Date: 18 May 2026
- No SIMD acceleration (the WILLIAM3 implementation in `willow25/william3.go` is a portable pure-Go port of bab_rs/portable.rs)

All numbers are single-threaded.

## Path encoding (datamodel)

| Path shape | Encode ns/op | Encode allocs | Decode ns/op | Decode allocs |
| --- | --- | --- | --- | --- |
| 3 components × 16 bytes (typical "ns/sub/file") | 87 | 4 | 75 | 4 |
| 8 components × 64 bytes (deep typed path) | 182 | 5 | 187 | 9 |
| 32 components × 128 bytes (pathological) | 1 399 | 10 | (not measured) | |

Allocations are dominated by the output byte slice and one slice per non-final component length. Encode allocates the new path; decode allocates the result components.

## Entry encoding (datamodel)

| Operation | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| Entry.Encode (32B ids + small path + 32B digest) | 149 | 560 | 6 |
| DecodeEntry (same shape) | 132 | 224 | 7 |

Entry encoding is dominated by id+digest byte copies; the path encoding contributes ~80 ns within the total.

## WILLIAM3 payload digest (willow25)

| Payload size | ns/op | Throughput | Allocs |
| --- | --- | --- | --- |
| 32 B | 170 | 188 MB/s | 0 |
| 1 KB (single chunk) | 2 595 | 395 MB/s | 0 |
| 8 KB (8 chunks) | 22 456 | 365 MB/s | 0 |
| 1 MB (1024 chunks, 10-level tree) | 2 906 241 | 361 MB/s | 0 |

Sustained throughput is ~360 MB/s on a single M3 core. The 32 B case is dominated by per-call setup; larger inputs amortize this and converge near the steady-state.

For comparison, BLAKE3 with full SIMD (AVX-512 / NEON) reaches multi-GB/s on similar hardware. willow-go's WILLIAM3 is a portable port; SIMD acceleration is tracked as future work (TECH_DEBT.md).

## Store (in-memory)

| Operation | ns/op | B/op | allocs |
| --- | --- | --- | --- |
| Insert into empty store | 86 | 288 | 5 |
| Query 1000 entries, full Range3d, linear scan | 87 907 | 586 104 | 3 012 |

The InMemoryStore uses a flat slice + linear scan. At 1000 entries the full-range query takes ~88 µs (88 ns per entry visited). For larger workloads, add an index by namespace + (subspace, path) — tracked in TECH_DEBT.md.

## Mobile bridge (gomobile)

The mobile package wraps the underlying datamodel + willow25 calls with thin builder types. No benchmark file is committed for the mobile package; the wrapper work is dominated by the underlying calls above. The gomobile-generated JNI/Objective-C bridge adds tens of microseconds per call due to thread switching; this dominates for short operations like HashPayload but is negligible for whole-Entry workflows.

## Reproducing

```sh
$ go test -bench=. -benchmem -run=^$ ./datamodel/ ./willow25/
```

For consistent numbers:

- Close other applications consuming the CPU.
- Pin the run to performance cores on macOS if possible (`sudo nice -n -20 go test -bench=...`).
- Run multiple times (`-count=5`) and use `benchstat` to estimate noise.

## Interpretation for production sizing

For a typical Willow workload (32 B namespaces, 32 B subspaces, 3-component paths, payloads in the 1 KB - 1 MB range):

- Insert + sign + verify cycle: ~50 µs (mostly Ed25519, ~30 µs) + ~150 ns (Entry encode) + ~360 MB/s WILLIAM3.
- Bulk replay of a 1000-entry store: < 1 ms (linear scan).
- A modern phone CPU should handle several thousand entries/sec in a single goroutine before WILLIAM3 throughput becomes the bottleneck.

Confidential Sync throughput will be bottlenecked by network and Ed25519 verification, not by anything measured here.
