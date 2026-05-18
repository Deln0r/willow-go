# `_genfixtures` — Rust fixture generator

Throwaway harness that drives the upstream Rust `willow_data_model` encoders
on a fixed input set and dumps JSON fixtures into `testdata/paths/`,
`testdata/entries/`, etc.

The underscore prefix excludes the directory from `go build` / `go test`,
so the Rust source never appears in the Go module graph.

## Why it exists

The upstream `willow_rs` repository does not ship golden test vectors. To
verify byte-for-byte compatibility of the Go implementation against the
reference Rust encoders, we generate fixtures here and commit them to the
repository. Go tests read those fixtures and assert round-trip plus
byte-identical re-encode.

## Lifecycle

- **Now (pre-MVP):** Rust harness, pinned to a specific upstream commit.
- **At MVP:** replaced by a Go program in the same directory. Generated
  fixtures from the Go program are diffed against the Rust-generated ones
  committed today; if identical, the Rust dependency is dropped entirely.
  Fixtures themselves remain in the repository regardless.

## Regenerating fixtures

```sh
cd testdata/_genfixtures
cargo run --bin gen-paths
```

Cargo will fetch and compile the pinned `willow_data_model` revision on
first run (a few minutes). Subsequent runs are instant.

## Updating the upstream pin

Edit the `rev = "..."` in `Cargo.toml` to point at the new commit, then
re-run the generator. Commit both `Cargo.toml`, `Cargo.lock`, and the
regenerated fixtures together so reviewers can verify provenance.
