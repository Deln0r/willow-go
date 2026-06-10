# willow-go

A pure-Go implementation of the [Willow Protocol](https://willowprotocol.org).

[![CI](https://github.com/Deln0r/willow-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Deln0r/willow-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Deln0r/willow-go.svg)](https://pkg.go.dev/github.com/Deln0r/willow-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/Deln0r/willow-go)](https://goreportcard.com/report/github.com/Deln0r/willow-go)
[![Go](https://img.shields.io/badge/go-1.26+-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Pre-MVP](https://img.shields.io/badge/status-pre--MVP-orange)]()
[![Byte-compat vs willow_rs](https://img.shields.io/badge/byte--compat-51%2F51%20fixtures-success)]()
[![Mobile](https://img.shields.io/badge/mobile-iOS%20%2B%20gomobile-success)]()
[![Codeberg mirror](https://img.shields.io/badge/mirror-codeberg.org-2185d0)](https://codeberg.org/Deln0r/willow-go)

> Willow is a peer-to-peer protocol for synchronisable data stores with capability-based permissions. willow-go ports the data model, the Meadowcap capability layer, and the Willow'25 parameter bundle to idiomatic Go, with mobile bindings via gomobile (iOS verified end-to-end; Android target builds but is not yet validated on a host with a JDK) and zero cgo.

> **Mirrors.** Primary repository is on GitHub at [github.com/Deln0r/willow-go](https://github.com/Deln0r/willow-go); an EU-sovereign mirror auto-synced on every push lives on [codeberg.org/Deln0r/willow-go](https://codeberg.org/Deln0r/willow-go) (Codeberg e.V., Berlin) alongside the Rust reference implementation `willow_rs`.

## How to read this

This README claims a lot of green ticks; treat the [Status](#status) table as the source of truth. "Stable" means the byte format matches [willow_rs](https://codeberg.org/worm-blossom/willow_rs) v0.7.0 fixtures and the official [willow_test_vectors](https://github.com/worm-blossom/willow_test_vectors) where their reencoded files agree with the spec. "Partial" means the encoder is in place but parts of the cross-impl corpus are deferred — usually because the upstream vectors and the spec text on willowprotocol.org currently disagree. "Phase 2" means not implemented yet.

If you are evaluating this for a production dependency: the data model, capabilities, and Willow'25 bundle are usable today; WGPS sync is not. See the [Phase 2 roadmap](#phase-2-roadmap).

## Status

Pre-MVP. The data-model layer, the Meadowcap capability layer (including multi-step delegation chains), and the Willow'25 parameter bundle are complete and validated byte-for-byte against the Rust reference + against the upstream `willow_test_vectors` corpus where the published spec is settled. WGPS sync is the explicit next phase — see the roadmap below.

| Component | Status | Cross-impl evidence | Source |
| --- | --- | --- | --- |
| CompactU64 codec | Stable | 4-bit packed + 8-bit standalone, unit-tested | [`encoding/`](encoding/) |
| Paths (absolute) | Stable | 11 yay + 165 nay upstream vectors pass | [`datamodel/paths.go`](datamodel/paths.go) |
| Paths (relative / extends) | Partial | Encoder + decoder ship; upstream `reencoded/` deferred pending spec / willow_rs realignment | [`datamodel/paths.go`](datamodel/paths.go) |
| Entries | Stable | 10 fixtures byte-identical vs willow_rs v0.7.0 | [`datamodel/entry.go`](datamodel/entry.go) |
| Areas (incl. area-in-area) | Stable | 8 fixtures byte-identical | [`datamodel/area.go`](datamodel/area.go) |
| Range3d / groupings | Stable | Unit-tested | [`datamodel/groupings.go`](datamodel/groupings.go) |
| In-memory Store | Stable | Prefix-pruning + concurrent access | [`datamodel/store.go`](datamodel/store.go) |
| Persistent Store | Phase 2 | — | — |
| Meadowcap communal capabilities | Stable | 4 Ed25519 delegation chains signed by willow_rs verify | [`meadowcap/`](meadowcap/) |
| Meadowcap owned / read capabilities | Phase 2 | — | — |
| WILLIAM3 payload digest | Stable | 11 cross-impl digest fixtures match bab_rs | [`willow25/william3.go`](willow25/william3.go) |
| Willow'25 parameter bundle | Stable | 4096/4096/4096 limits, 32-byte ids | [`willow25/willow25.go`](willow25/willow25.go) |
| Mobile bindings — iOS | Stable | XCFramework built and inspected on Xcode 26.5 / iOS SDK 26.5 | [`mobile/`](mobile/) |
| Mobile bindings — Android | Partial | `gomobile bind -target=android` target wired in the Makefile; not yet built end-to-end on a host with a JDK + Android NDK (the underlying mobile package compiles cleanly) | [`mobile/`](mobile/) |
| WGPS sync (set reconciliation) | Phase 2 | — | — |
| Transport encryption | Phase 2 | — | — |

51 fixtures from the upstream Rust harness pass byte-identical encode + lossless decode round-trip (the smoketest output below is the authoritative count). 4 Meadowcap delegation chains signed by willow_rs verify under our Go IsValid. 176 additional vectors from the official upstream `willow_test_vectors` corpus (11 positive + 165 attacker-supplied negative cases for absolute path encodings) pass; the negative-test pass already found and fixed one panic-level bug in our decoder. 73 test functions (167 runs including subtests) across the 5 packages with test files.

## Goals

- A pure-Go Willow port that compiles to static binaries (no cgo) and to mobile XCFrameworks / AARs.
- Byte-for-byte compatibility with willow_rs v0.7.0 for every encoder we ship.
- Validation against the official spec test vectors wherever the published spec text and the test vectors agree.
- An idiomatic Go surface: stdlib types, no surprise globals, no panics on attacker-supplied input.
- Small, reviewable packages with one responsibility each.

## Non-goals

- Re-implementing every Willow encoding that the spec defines on day one. Partial / deferred items are listed in [Status](#status) and the [Phase 2 roadmap](#phase-2-roadmap).
- Out-performing the Rust reference. We aim for "fast enough to not be the bottleneck" — see [Performance](#performance).
- A new transport, framing, or wire protocol. Where WGPS-like demos exist in [`cmd/willow-sync-demo`](cmd/willow-sync-demo/), they are clearly marked as ad-hoc, not WGPS.
- Drop-in interop with non-Willow protocols (Iroh-style sync, Yjs, etc.). Different problem space.

## Comparison

| | willow-go | willow_rs | willow-js |
| --- | --- | --- | --- |
| Language | Go | Rust | TypeScript |
| Status | Pre-MVP (this repo) | Production | Reference impl |
| Native iOS / Android bindings | `gomobile bind` → XCFramework / AAR, no cgo | Possible via cbindgen + cross-compile | Via React Native bridge |
| Static container binaries | `go build` | Needs musl + cross-compile | Needs Node / Bun runtime |
| Go ecosystem (Matrix, NATS, gRPC, …) | Drop-in import | FFI bridge required | FFI / IPC required |
| WGPS sync | Phase 2 (roadmap) | Yes | Yes |
| Persistent store | Phase 2 (sqlite via modernc.org/sqlite planned) | Yes | Yes |
| License | MIT | MIT | MIT |

If you need WGPS sync today, use willow_rs or willow-js. If you need Willow on a phone or in a Go service, this is the project.

## Quick start

```sh
go get github.com/Deln0r/willow-go
```

```go
package main

import (
    "fmt"

    "github.com/Deln0r/willow-go/datamodel"
    "github.com/Deln0r/willow-go/willow25"
)

func main() {
    payload := []byte("hello, willow")

    path, _ := willow25.NewPath([][]byte{
        []byte("notes"),
        []byte("greeting.txt"),
    })

    namespace := make([]byte, 32)
    subspace := make([]byte, 32)

    entry, _ := willow25.NewEntry(namespace, subspace, path, 1700000000, payload)
    encoded := entry.Encode()

    fmt.Printf("entry encoding: %d bytes\n", len(encoded))
    fmt.Printf("payload digest: %x\n", entry.PayloadDigest)

    store := datamodel.NewInMemoryStore()
    pruned, stored := store.Insert(entry)
    fmt.Printf("stored=%v pruned=%d\n", stored, pruned)
}
```

## What is implemented

| Layer | Component | Source |
| --- | --- | --- |
| Data model | Paths, Entries, Range3d, Areas, Store | [`datamodel/`](datamodel/) |
| Encoding | CompactU64, path encoding, path-extends-path, area-in-area | [`encoding/`](encoding/), [`datamodel/area.go`](datamodel/area.go) |
| Capabilities | Communal write capability, multi-step Ed25519 delegation chains, AuthorisationToken | [`meadowcap/`](meadowcap/) |
| Willow'25 | 4096/4096/4096 limits, WILLIAM3 payload digest, 32-byte ids, convenience constructors | [`willow25/`](willow25/) |
| Mobile | gomobile-bindable API: PathBuilder, EntryBuilder, HashPayload | [`mobile/`](mobile/) |
| Tooling | Cross-impl smoketest CLI, end-to-end sync demo CLI | [`cmd/`](cmd/) |

Not yet implemented (see [roadmap](#phase-2-roadmap) below): WGPS sync, persistent store, transport encryption, owned-namespace capabilities, read capabilities.

## Cross-implementation validation

Two independent corpora exercise our encoders.

**A. Self-generated against willow_rs v0.7.0 (51 byte-compat fixtures + 11 WILLIAM3 digest fixtures + 4 cross-impl Meadowcap chains)** — every byte-producing encoder is verified against the Rust reference by a fixture corpus generated from a pinned upstream commit:

```
testdata/_genfixtures/         Rust harness, pinned to willow_rs dd87996
testdata/paths/                {basic, limits, relative}.json  - 26 cases
testdata/paths_rel/extends.json                                - 7 cases
testdata/entries/basic.json                                    - 10 cases
testdata/areas/relative.json                                   - 8 cases
testdata/william3/digests.json                                 - 11 cases
testdata/meadowcap/delegation_chains.json                      - 4 cases (Ed25519 signed)
```

**B. Official upstream `willow_test_vectors` (176 vectors currently exercised)** — pulled in as a git submodule under `testdata/upstream_vectors/`. The submodule is checked out automatically by CI; locally you initialize it once with `git submodule update --init`. Adoption is in progress: absolute path encodings pass (encode_path + EncodePath = 11 positive + 165 negative cases). Relative encodings (path_rel_path, EncodePathRelativePath, path_extends_path, EncodePathExtendsPath) are deferred pending spec / willow_rs / test_vectors realignment — the upstream reencoded/ files differ from both willow_rs v0.7.0 and the spec text on willowprotocol.org as of May 2026. See [TECH_DEBT.md](TECH_DEBT.md) for the full audit.

```sh
$ make smoketest
paths (absolute)       16 pass    0 fail
paths (relative)       10 pass    0 fail
paths (extends)         7 pass    0 fail
entries                10 pass    0 fail
areas (relative)        8 pass    0 fail

TOTAL: 51 pass / 0 fail (51 cases)
```

(The William3, Meadowcap, and upstream willow_test_vectors corpora are exercised by `go test ./willow25/...`, `go test ./meadowcap/...`, and `go test ./datamodel/...` respectively.)

To regenerate fixtures (Rust toolchain required):

```sh
cd testdata/_genfixtures
cargo run --bin gen-paths
cargo run --bin gen-entries
cargo run --bin gen-area-in-area
cargo run --bin gen-path-extends-path
cargo run --bin gen-william3
cargo run --bin gen-handover
```

## Mobile (iOS verified, Android target wired)

The `mobile/` package exposes a gomobile-bindable surface (builder types, no `[]byte` slice arguments) so the underlying datamodel can be called from Swift / Objective-C / Java / Kotlin without writing FFI by hand.

```sh
# Install the gomobile toolchain (one-off):
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init

# iOS — verified end-to-end on Xcode 26.5 / iOS SDK 26.5:
make mobile-ios       # produces Mobile.xcframework (~15 MB, arm64 device + arm64/x86_64 simulator)

# Android — target wired but not yet built end-to-end on this host
# (requires a JDK + Android NDK; the underlying Go package compiles cleanly):
make mobile-android   # produces mobile.aar
```

After `make mobile-ios`, drag `Mobile.xcframework` into an Xcode project. The bound API:

```swift
import Mobile

let digest = MobileHashPayload("hello".data(using: .utf8)!)
// digest is a 32-byte Data (WILLIAM3)

let builder = MobilePathBuilder()
builder.addComponent("notes".data(using: .utf8)!)
builder.addComponent("greeting.txt".data(using: .utf8)!)
let pathBytes = try builder.encode()
```

## End-to-end P2P demo

Two willow-go processes exchanging Ed25519-signed entries over a pipe:

```sh
$ go run ./cmd/willow-sync-demo --mode=gen --count=3 --tag=alice \
| go run ./cmd/willow-sync-demo --mode=recv --tag=bob

alice gen: namespace=58254987... root=a1b22c33...
alice gen: wrote 3 entries
bob recv: ACCEPT ns=58254987... path=notes/entry-000 ts=100 payload_len=16
bob recv: ACCEPT ns=58254987... path=notes/entry-001 ts=1100 payload_len=16
bob recv: ACCEPT ns=58254987... path=notes/entry-002 ts=2100 payload_len=16
bob recv: read=3 accepted=3 rejected=0 store_len=3
```

This is NOT WGPS (no set reconciliation, no fingerprint trees, no channel multiplexing) — that is Phase 2. This is the minimum viable proof that the data-model + capability layers compose correctly on a duplex transport.

## Performance

Headline numbers on Apple M3, Go 1.26.3, single-threaded, portable code (no SIMD). See [BENCHMARKS.md](BENCHMARKS.md) for full tables and reproduction notes.

| Operation | Result |
| --- | --- |
| WILLIAM3 sustained throughput | ~360 MB/s (8 KB+ payloads) |
| Path encode (3 components × 16 B) | 87 ns/op, 4 allocs |
| Entry.Encode (32 B ids + small path + 32 B digest) | 149 ns/op |
| Store.Query, 1000 entries, full Range3d | 88 µs |

For comparison, BLAKE3 with full AVX-512 / NEON reaches multi-GB/s on similar hardware. SIMD acceleration for WILLIAM3 is tracked as future work.

## Phase 2 roadmap

Planned scope, in rough priority order:

1. WGPS sync protocol — set reconciliation with cumulative hash fingerprints, LCMUX channel multiplexing, PIO private interest overlap.
2. Persistent store backend on top of `modernc.org/sqlite` (pure Go, no cgo).
3. Transport encryption.
4. `Area`-encoded WGPS message framing.
5. Polished error handling + fuzz harness + benchmarks.

Timeline depends on dedicated funding. If you are interested in sponsoring or contributing to this work, please open an issue.

## Architecture

```
willow-go/
├── encoding/          CompactU64 codec (4-bit packed + 8-bit standalone)
├── datamodel/         Paths, Entries, Range3d, Areas, Store, prefix-pruning
├── meadowcap/         Communal capabilities, delegations, AuthorisationToken
├── willow25/          Concrete 4096/4096/4096 + WILLIAM3 bundle
├── mobile/            gomobile-bindable wrappers
├── cmd/
│   ├── willow-smoketest/    Cross-impl byte-compat gate
│   └── willow-sync-demo/    Pipe-based P2P sync demonstration
└── testdata/
    ├── _genfixtures/   Rust harness (pinned to willow_rs dd87996)
    ├── paths/  paths_rel/  entries/  areas/  william3/  meadowcap/
```

## Contributing

Bug reports and PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the workflow and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). For larger contributions or design questions, please open an issue first to discuss scope.

## References

- [Willow Protocol specification](https://willowprotocol.org)
- [willow_rs (Rust reference)](https://codeberg.org/worm-blossom/willow_rs)
- [Meadowcap capability system](https://willowprotocol.org/specs/meadowcap/)
- [Willow'25 parameter bundle](https://willowprotocol.org/specs/willow25/)
- [bab_rs / WILLIAM3 hash function](https://codeberg.org/worm-blossom/bab_rs)

## Acknowledgements

- [Aljoscha Meyer](https://aljoscha-meyer.de) and [Sam Gwilym](https://gwil.garden), authors of the Willow Protocol and maintainers of the Rust reference implementation. The spec is unusually clear and the Rust code was an indispensable reference while writing this port. Any divergence from the spec in this codebase is a willow-go bug, not theirs.
- The [bab_rs](https://codeberg.org/worm-blossom/bab_rs) authors, whose portable WILLIAM3 implementation was ported verbatim to Go (same compression function, same message schedule, custom IV).
- The [BLAKE3](https://github.com/BLAKE3-team/BLAKE3) team — WILLIAM3 is BLAKE3's compression with a different IV, so most of the cryptographic work was theirs.

## License

MIT. See [LICENSE](LICENSE).
