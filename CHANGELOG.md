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
- Native `testing.F` fuzz harness: `FuzzDecodeCU64Standalone` (encoding), `FuzzDecodePath` and `FuzzDecodeExtending` (datamodel), seeded from existing fixtures and run for 20s each in CI. Targets assert the decoders never panic on attacker-supplied input, stay within bounds and limits, and satisfy encode-idempotence. Closes the open `TECH_DEBT.md` fuzz item.
- CI `staticcheck` step pinned to v0.7.0 (2026.1).
- `cmd/willow-cli`: a read-only inspector with `path encode`/`path decode`, `entry decode`, and `digest` (WILLIAM3) subcommands, for cross-implementation interop debugging. No network, no sync, no key generation. Capability encodings are intentionally not covered (no canonical wire format yet, Phase 2).
- `HACKING.md` contributor onboarding (repo map, everyday commands, fuzzing, fixture regeneration) and a `make help` target.

### Changed
- Android AAR verified end-to-end: `gomobile bind` builds all four ABIs (arm64-v8a, armeabi-v7a, x86, x86_64) on NDK 27 + OpenJDK 26, `classes.jar` exposes the mobile API and each ABI ships `libgojni.so`. README `mobile/Android` Status row moved to "Stable" with that evidence, and the Makefile `mobile-android` target now builds the full ABI set instead of arm64 only.
- CI: `actions/checkout` bumped to v7, `actions/setup-go` bumped to v6 (Dependabot).

### Fixed
- WILLIAM3 corrected to bab_rs 0.8.0. The earlier port tracked bab_rs 0.5.0, which computed WILLIAM3 incorrectly: it did not compress a block for empty input, and passed a fixed block length of 64 to the compression instead of the real (possibly partial) block length. Both are fixed in `willow25/william3.go`, the 11 digest fixtures are regenerated, and the upstream `william3vectors.txt` (18 cases) is committed and verified. Reported by Aljoscha Meyer (issue #4). This changes the payload-digest values for any input that is empty or not a multiple of 64 bytes.
- Fixture and test counts reconciled with the smoketest output and the actual test tree: 51 byte-compat fixtures (badge and prose previously said 53), test counts now stated as top-level test functions (previous counts mixed conventions).
- Stale commit references and cross-references in `TECH_DEBT.md` now point at commits that exist in the public history.
- staticcheck S1038 cleanup in `datamodel/upstream_vectors_test.go` (`t.Logf` instead of `t.Log(fmt.Sprintf(...))`) and the now-unused `fmt` import dropped.

### Documentation
- Removed forward-looking funding claims and a stale private-notes path from README.
- Renamed "WGPS" to "Confidential Sync" across the docs and code comments to track the worm-blossom team's May 2026 rename. The first README mention and `SECURITY.md` keep a "(formerly WGPS)" bridge; `TECH_DEBT.md` documents the rename.

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

Confidential Sync (set reconciliation, LCMUX, PIO, transport encryption), persistent store backend, and owned / read Meadowcap capabilities are explicit Phase 2 scope. See `TECH_DEBT.md` and the README roadmap.
