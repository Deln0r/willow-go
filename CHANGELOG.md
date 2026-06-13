## Unreleased

### Added
- Codeberg mirror auto-synced from GitHub Actions on every push to `main` and on tags.
- Public `TECH_DEBT.md` enumerating known Phase 2 deferrals and closed items.
- README badges for Go Reference, Go Report Card, Codeberg mirror.
- README `Status` table, `Goals` / `Non-goals`, `Comparison`, `Performance`, `Acknowledgements` sections.
- `BENCHMARKS.md` and `bench_test.go` for `datamodel` and `willow25`.
- `SECURITY.md` with the vulnerability disclosure policy.
- Dependabot config for gomod and github-actions (weekly, minor+patch grouped).
- Pull request template.
- Issue templates (bug report, feature request) with a config linking to the Phase 2 roadmap and security policy.

### Changed
- README `mobile/Android` row corrected from "Stable" to "Partial": the `gomobile bind -target=android` target is wired in the Makefile and the underlying package compiles cleanly, but the build has not been exercised end-to-end on a host with a JDK + Android NDK.
- CI: `actions/checkout` bumped to v6, `actions/setup-go` bumped to v6 (Dependabot).

### Fixed
- Fixture and test counts reconciled with the smoketest output and the actual test tree: 51 byte-compat fixtures (badge and prose previously said 53), test counts now stated as top-level test functions (previous counts mixed conventions).
- Stale commit references and cross-references in `TECH_DEBT.md` now point at commits that exist in the public history.

### Documentation
- Removed forward-looking funding claims and a stale private-notes path from README.

---

## v0.1.0 - 2026-05-18

First tagged release. Pre-MVP. Datamodel, capability layer, and Willow'25 parameter bundle are validated byte-for-byte against the Rust reference (`willow_rs` v0.7.0) and against the official `willow_test_vectors` corpus where the published spec is settled.

### Packages

- `datamodel`: Path, Entry, Range3d, Area, in-memory Store with prefix pruning.
- `encoding`: CompactU64 codec (4-bit packed and 8-bit standalone variants).
- `meadowcap`: communal capabilities with Ed25519 delegation chains and entry authorisation tokens.
- `willow25`: Willow'25 parameter bundle, 4096/4096/4096 path limits, WILLIAM3 payload digest with BLAKE3 + custom IV, 32-byte subspace/namespace ids.
- `mobile`: gomobile-bindable API surface (no cgo).
- `cmd/willow-smoketest`: cross-impl byte-compat acceptance gate CLI.
- `cmd/willow-sync-demo`: end-to-end peer-to-peer sync proof-of-concept.

### Cross-impl validation

- 51 fixtures from the upstream Rust harness: byte-identical encode and lossless decode round-trip.
- 4 Meadowcap delegation chains signed by `willow_rs` verify under Go `IsValid`.
- 176 vectors from the official `worm-blossom/willow_test_vectors` corpus pass (11 positive + 165 attacker-supplied negative cases for absolute path encodings). The negative-test pass identified and fixed one panic-level bug in the decoder.

### Tests

70 test functions across the 5 packages with test files. CI runs on every push and pull request.

### Mobile

iOS XCFramework builds end-to-end via `gomobile bind` and was inspected on Xcode 26.5 / iOS SDK 26.5. Android target is wired in the Makefile but not yet built end-to-end.

### Not in this release

WGPS sync (set reconciliation, LCMUX, PIO, transport encryption), persistent store backend, and owned / read Meadowcap capabilities are explicit Phase 2 scope. See `TECH_DEBT.md` and the README roadmap.
