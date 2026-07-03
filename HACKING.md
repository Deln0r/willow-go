# Hacking on willow-go

Contributor onboarding: where things live, how to build and test, and how the
cross-implementation fixtures are generated. For scope and PR conventions see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Prerequisites

- Go 1.26 or newer (`go.mod` pins `go 1.26.3`).
- For regenerating fixtures: Rust 1.85+ (`rustup install stable`).
- For mobile builds: the gomobile toolchain, plus Xcode (iOS) or the Android
  NDK + a JDK (Android). See the Mobile section of the README.

## First checkout

The upstream test vectors live in a git submodule, so clone with it or
initialise it after the fact:

```sh
git clone --recurse-submodules https://github.com/Deln0r/willow-go
# or, in an existing checkout:
git submodule update --init
```

Without the submodule the `datamodel` upstream-vector test skips rather than
fails, but you want it for full coverage.

## Repository map

```
datamodel/    Paths, Entries, Range3d, Area, in-memory Store, their encoders
encoding/     CompactU64 codec (shared binary primitive)
meadowcap/    Communal capabilities, Ed25519 delegation chains, auth tokens
willow25/     Willow'25 bundle: 4096/4096/4096 limits, WILLIAM3 digest, defaults
mobile/       gomobile-bindable wrappers (no []byte-slice arguments, no cgo)
cmd/
  willow-cli/         read-only encode/decode/digest inspector
  willow-smoketest/   cross-impl byte-compat acceptance gate
  willow-sync-demo/   pipe-based capability-exchange demo (not Confidential Sync)
internal/     helpers not part of the public API
testdata/
  _genfixtures/       Rust harness that generates the fixtures (see below)
  paths/ paths_rel/ entries/ areas/ william3/ meadowcap/   committed fixtures
  william3/william3vectors.txt   verbatim upstream WILLIAM3 vectors (bab_rs 0.8.0)
  upstream_vectors/   git submodule: worm-blossom/willow_test_vectors
```

## Everyday commands

```sh
make test         # go test ./... (all packages)
make smoketest    # byte-compat acceptance gate, must stay 0-fail
make bench        # microbenchmarks (datamodel + willow25)
make help         # list targets
```

Before sending a PR, run what CI runs: `go vet ./...`, `gofmt -s -l .` (must be
empty), `staticcheck ./...`, `make test`, and `make smoketest`.

## Fuzzing

The decoders have native `testing.F` targets that assert they never panic on
attacker-supplied input and satisfy encode-idempotence:

```sh
go test ./encoding/ -run '^$' -fuzz 'FuzzDecodeCU64Standalone$' -fuzztime 30s
go test ./datamodel/ -run '^$' -fuzz 'FuzzDecodePath$'          -fuzztime 30s
go test ./datamodel/ -run '^$' -fuzz 'FuzzDecodeExtending$'     -fuzztime 30s
```

CI runs each for 20s on every push. If a target finds a crasher it writes the
input under `testdata/fuzz/`; commit that file with the fix so it becomes a
permanent regression case.

## Cross-implementation fixtures

`willow_rs` does not ship golden vectors, so byte-compatibility is verified
against fixtures generated from a pinned upstream commit by the Rust harness in
`testdata/_genfixtures/`. The underscore prefix keeps that directory out of the
Go module graph.

The harness has one binary per fixture family:

| Binary | Writes |
| --- | --- |
| `gen-paths` | `testdata/paths/` |
| `gen-path-extends-path` | `testdata/paths_rel/` |
| `gen-entries` | `testdata/entries/` |
| `gen-area-in-area` | `testdata/areas/` |
| `gen-william3` | `testdata/william3/digests.json` |
| `gen-handover` | `testdata/meadowcap/` |

Regenerate one family:

```sh
cd testdata/_genfixtures
cargo run --bin gen-paths
```

Cargo fetches and compiles the pinned `willow_data_model` / `meadowcap`
revision on first run (a few minutes); later runs are instant. The pin is the
`rev = "..."` in `testdata/_genfixtures/Cargo.toml`, currently willow_rs
`dd87996` (v0.7.0).

To move the pin: edit the `rev` in `Cargo.toml`, regenerate, and commit
`Cargo.toml`, `Cargo.lock`, and the regenerated fixtures together so a reviewer
can verify provenance in one diff.

The WILLIAM3 vectors are a special case: `testdata/william3/william3vectors.txt`
is copied verbatim from bab_rs (currently 0.8.0) and pins the digest directly to
the upstream reference rather than to our own generator.

## After changing an encoder

Encoder changes must extend the fixture corpus, not just the code. Add a fixture
under `testdata/` and a corresponding case in `cmd/willow-smoketest`, then
confirm `make smoketest` still reports 0 failures. A code-only encoder change
will not be merged.
