# willow-go

A pure-Go implementation of the [Willow Protocol](https://willowprotocol.org).

[![CI](https://github.com/Deln0r/willow-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Deln0r/willow-go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26+-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Pre-MVP](https://img.shields.io/badge/status-pre--MVP-orange)]()
[![Byte-compat vs willow_rs](https://img.shields.io/badge/byte--compat-53%2F53%20fixtures-success)]()
[![Mobile](https://img.shields.io/badge/mobile-iOS%20%2B%20gomobile-success)]()

> Willow is a peer-to-peer protocol for synchronisable data stores with capability-based permissions. willow-go ports the data model, the Meadowcap capability layer, and the Willow'25 parameter bundle to idiomatic Go, with iOS and Android bindings via gomobile and zero cgo.

## Why willow-go

The [Rust reference implementation](https://codeberg.org/worm-blossom/willow_rs) is excellent for native Rust applications. willow-go exists for everything Rust does not reach cleanly:

| | willow_rs | willow-go |
| --- | --- | --- |
| Native iOS / Android bindings | requires cgo + cross-compile dance | `gomobile bind` produces an XCFramework / AAR |
| Go ecosystem (Matrix, NATS, gRPC, …) | needs FFI bridge | drop-in import |
| Container-friendly static binaries | needs musl + cross-compile | `go build` |
| WGPS sync protocol | YES (production) | Phase 2 — planned, see roadmap |

If you need WGPS sync today, use willow_rs. If you need Willow on a phone or in a Go service, this is the project.

## Status

Pre-MVP. The data-model layer, the Meadowcap capability layer (including multi-step delegation chains), and the Willow'25 parameter bundle are complete and validated byte-for-byte against the Rust reference + against the upstream `willow_test_vectors` corpus where the published spec is settled. WGPS sync is the explicit next phase — see the roadmap below.

53 fixtures from the upstream Rust harness pass byte-identical encode + lossless decode round-trip. 4 Meadowcap delegation chains signed by willow_rs verify under our Go IsValid. 176 additional vectors from the official upstream `willow_test_vectors` corpus (11 positive + 165 attacker-supplied negative cases for absolute path encodings) pass; the negative-test pass already found and fixed one panic-level bug in our decoder. 123 tests across 7 packages.

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

**A. Self-generated against willow_rs v0.7.0 (53 fixtures + 4 cross-impl Meadowcap chains)** — every byte-producing encoder is verified against the Rust reference by a fixture corpus generated from a pinned upstream commit:

```
testdata/_genfixtures/         Rust harness, pinned to willow_rs dd87996
testdata/paths/                {basic, limits, relative}.json  - 26 cases
testdata/paths_rel/extends.json                                - 7 cases
testdata/entries/basic.json                                    - 10 cases
testdata/areas/relative.json                                   - 8 cases
testdata/william3/digests.json                                 - 11 cases
testdata/meadowcap/delegation_chains.json                      - 4 cases (Ed25519 signed)
```

**B. Official upstream `willow_test_vectors` (176 vectors currently exercised)** — pulled in as a git submodule under `testdata/upstream_vectors/`. The submodule is checked out automatically by CI; locally you initialize it once with `git submodule update --init`. Adoption is in progress: absolute path encodings pass (encode_path + EncodePath = 11 positive + 165 negative cases). Relative encodings (path_rel_path, EncodePathRelativePath, path_extends_path, EncodePathExtendsPath) are deferred pending spec / willow_rs / test_vectors realignment — the upstream reencoded/ files differ from both willow_rs v0.7.0 and the spec text on willowprotocol.org as of May 2026. See `TECH_DEBT.md` (private notes) for the audit.

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

## Mobile (iOS and Android)

```sh
# Install the gomobile toolchain (one-off):
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init

# Build:
make mobile-ios       # produces Mobile.xcframework (verified on Xcode 26.5)
make mobile-android   # produces mobile.aar (requires Android NDK and a JDK)
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

## License

MIT. See [LICENSE](LICENSE).
