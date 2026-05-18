# Contributing to willow-go

Thanks for considering a contribution. This project is at a pre-MVP stage and the maintainer is open to issues and pull requests but capacity is limited. Reading this whole document before opening anything saves everyone time.

## Scope

willow-go ports a subset of the Willow specifications to Go:

- **In scope:** data model (paths, entries, ranges, areas, store), Meadowcap capability layer, the Willow'25 parameter bundle, mobile bindings (gomobile), pure-Go testing infrastructure.
- **Out of scope until Phase 2 (planned future work):** WGPS sync, persistent stores, transport encryption, owned-namespace capabilities, read-side ACL enforcement. PRs that try to start any of these will be politely closed — they are reserved for a future dedicated development phase, and partial implementations would create maintenance debt.
- **Out of scope permanently:** features outside the Willow protocol family.

If you are unsure whether something is in scope, open an issue first.

## Bug reports

Useful bug reports include:

1. The version (commit SHA or release tag).
2. Minimal Go code that reproduces the issue.
3. Expected vs observed behavior.
4. If a byte-encoding bug: hex dumps of input + expected + actual output, ideally also the willow_rs equivalent if you have it.

Cross-implementation interop bugs are highest priority. If you find a byte-divergence vs willow_rs that is not already covered by a fixture in `testdata/`, please attach the discrepancy.

## Pull requests

Before opening a PR:

- Run `make test` (all 7 packages must stay green).
- Run `make smoketest` (51 byte-compat fixtures must stay 0-fail).
- Run `gofmt -s -w .` on changed files.
- Add a test that fails without the change and passes with it. For encoder changes, add a fixture in `testdata/` and a corresponding case in the smoketest CLI.

Commit message format: one short subject line under 70 characters, then a blank line, then a body explaining the *why*. Example:

```
datamodel: reject zero-length paths with non-empty timestamps

The spec says empty paths have zero total length but allows any timestamp.
Our decoder was conflating the two and rejecting valid encodings produced
by willow_rs (fixture entries/empty_path_nonzero_ts.json reproduces).
```

PRs that add or modify byte encoders MUST extend the fixture corpus. Code-only encoder changes without fixture coverage will not be merged.

### Authorship

This project uses standard git authorship. Please do not include AI agent attribution lines (`Co-Authored-By: Claude`, `🤖 Generated with [Claude Code]`, `noreply@anthropic.com`, etc.) in commit messages. PRs containing such trailers will be asked to amend.

## Cross-implementation fixtures

`testdata/_genfixtures/` is a Rust harness pinned to a specific `willow_rs` commit. To regenerate fixtures you need:

- Rust 1.85+ (any 2024-edition-capable stable; `rustup install stable` works).
- For meadowcap delegation fixtures: ed25519-dalek is pulled automatically.

After regeneration, commit:

1. The updated `Cargo.lock` (reproducibility pin).
2. The updated `.json` fixtures.
3. Any code changes needed to consume new fixtures.

Do NOT commit:

- `testdata/_genfixtures/target/` (gitignored).
- Build artifacts like `Mobile.xcframework/`, `mobile.aar` (gitignored).

## Mobile changes

If you touch the `mobile/` package, validate both bindings:

- `make mobile-ios` (requires Xcode + iOS SDK).
- `make mobile-android` (requires Android NDK and a JDK).

For the iOS build, verify the generated `Mobile.xcframework/ios-arm64/Mobile.framework/Headers/Mobile.objc.h` reflects your API change. For Android, verify the AAR contents include the expected Java classes.

## Code style

- Standard `gofmt`. `go vet ./...` must pass.
- Idiomatic Go: no Rust-style transliteration. Generics only where the Go standard library already uses them.
- No cgo. The whole project depends on `go build` + a few pure-Go deps; please keep it that way.
- Public API stability is not yet a priority — breaking changes are accepted until v0.1.
- Comments: doc.go for each package, godoc on all exported symbols. Short, not essays.

## Security disclosure

For security-relevant issues (signature forgery, capability scope leaks, panics on untrusted input), please email the maintainer privately first rather than opening a public issue. Once a fix is ready we will publish the fix + a CVE-style note together.

## Maintainer expectations

The maintainer is a single individual building this in spare time. Response times will vary. PRs may take a week or two to review. If something has been quiet for over a month, please ping politely on the original thread.

## License

By contributing you agree your contributions are licensed under MIT, matching the project license.
