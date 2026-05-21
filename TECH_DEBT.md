# Tech Debt — willow-go

Running ledger of things deliberately deferred during the willow-go port. Each entry records **what** was skipped, **why**, **impact**, **when to revisit**, and any **blockers**.

> When closing an item: move it to the "Closed" section at the bottom with the commit SHA / PR that resolved it, do not delete.

Last updated: 21 May 2026.

---

## Phase 2 planned scope

These are planned scope items for Phase 2, deliberately deferred from pre-MVP to keep v0.1.0 reviewable. They are the focus of the post-MVP roadmap (see [README Phase 2 roadmap](README.md#phase-2-roadmap)).

### Confidential Sync (Willow General-Purpose Sync) protocol

- **Status:** deferred from pre-MVP.
- **Impact:** No peer-to-peer sync. The Go impl is currently a local-only data model + capability layer.
- **Blockers:** Need to design transport abstraction first (probably async I/O over ufotofu-equivalent in Go), then port the sync state machine, then LCMUX (logical channel multiplexing), then PIO (private interest overlap).
- **Note:** The protocol formerly known as "WGPS" was renamed to "Confidential Sync" by the worm-blossom team in May 2026; the reference Rust impl is itself undergoing rework of this layer.
- **Revisit:** Phase 2 work.

### Persistent store backend

- **Status:** in-memory only for pre-MVP, modernc.org/sqlite planned for Phase 2.
- **Impact:** All data lost on restart.
- **Revisit:** Phase 2. Decision pending: modernc.org/sqlite vs badger. Consistency with the broader Go-native, no-cgo stance favors modernc.org/sqlite.

### Transport encryption

- **Status:** Spec specifies a noise-style handshake.
- **Revisit:** Phase 2, after the Confidential Sync transport layer.

### Sideload (offline data exchange)

- **Status:** deferred from pre-MVP.
- **Revisit:** Phase 2, after persistent store.

### Fuzz harness

- **Status:** Rust has `fuzz/` directory with cargo-fuzz targets for paths, encodings, etc. Go impl has none.
- **Impact:** No property-based / structure-aware bug-finding. Currently relying on table-driven golden vectors + manual edge cases.
- **Revisit:** Phase 2 or earlier if a bug-finding sprint is justified. Go has `testing.F` for native fuzz tests (Go 1.18+).

---

## Pre-MVP deferred (close before public release polish)

These are pre-MVP scope but not yet implemented. Tracked here so they do not slip.

### `AreaOfInterest`

- **Origin:** Wraps `Area` with capacity limits (max_count, max_size); used by Confidential Sync to bound what each peer commits to reconciling per round.
- **Why deferred:** Only consumer is the sync layer (Phase 2). `Area` alone is sufficient for meadowcap capabilities.
- **Impact:** None until sync work begins.
- **Revisit:** Phase 2 alongside Confidential Sync.

### `Path.successor` / `Path.predecessor`

- **Origin:** `successor` returns the lex-next path in the data-model ordering, `predecessor` the lex-prev. Used by `Range3d.singleton(coord)` to express "exactly this coordinate" as a Range3d.
- **Why deferred:** Not used by current consumers. `Area.AsRange3d` uses `GreaterButNotPrefixed` instead, which has different semantics — it returns the lex-next path that is NOT a child of `self`, suitable for prefix-based range bounds.
- **Impact:** Cannot express `Range3d.singleton(coordinate)` per the Rust API. Workaround: build the Range3d directly with manual closed ranges.
- **Revisit:** When a consumer needs singleton Range3d. Likely Phase 2 alongside sync, possibly sooner if smoketest demands it.

### `Range3d` byte encoding (`encode_range_3d`)

- **Origin:** Spec defines an encoding used only by the Confidential Sync layer.
- **Why deferred:** Sync is Phase 2 (see above). Encoding would be dead code right now.
- **Impact:** None for pre-MVP. Required before any cross-impl interop.
- **Revisit:** Phase 2 alongside sync.

### `PathBuilder` API

- **Origin:** Rust offers an O(1)-append `PathBuilder` to avoid the O(n) per-append cost of `Path.from_components`.
- **Why deferred:** Pre-MVP has no hot path that benefits.
- **Impact:** Performance only. All current constructors are O(n+m) which is acceptable for tests and small inputs.
- **Revisit:** When a real workload benchmarks slow. Probably never before Phase 2.

### Validated `Component` type

- **Origin:** Rust wraps `&[u8]` in a `Component<MCL>` type that statically encodes the max-component-length invariant.
- **Why deferred:** Go has no const generics; mirroring would require a heavyweight phantom-type pattern that hurts ergonomics for marginal benefit.
- **Impact:** Component length is re-validated at every `Path` constructor instead of being baked into the type. Same end behavior, slightly more runtime work.
- **Revisit:** Probably never. This is an idiomatic-Go decision, not a debt to repay. Listed here so future readers do not "fix" it by accident.

### `EncodableKnownLength` / `len_of_encoding`

- **Origin:** Rust traits compute the encoded length without actually encoding, useful for pre-sizing buffers.
- **Why deferred:** Go uses `append` which grows automatically; the optimization is unnecessary for pre-MVP.
- **Impact:** Minor allocation overhead.
- **Revisit:** Only if benchmarks justify.

### Zero-copy `Bytes`-style sharing for `Path`

- **Origin:** Rust `Path` stores data in a ref-counted `bytes::Bytes`, so cloning paths and taking prefixes/suffixes is O(1) without allocation. Go impl defensively copies.
- **Why deferred:** Go has no widely-used analogue (`sync.Pool` or `unsafe` slicing would buy this back at a complexity cost). Pre-MVP correctness > performance.
- **Impact:** `LongestCommonPrefix` and similar slice the underlying components slice — but inner byte slices are still shared with the original Path (since we never mutate them). Full clone is O(total bytes).
- **Revisit:** Only if benchmarks show Path operations dominating. Note: `Components()` returns the internal `[][]byte` directly; callers must not mutate it (documented).

### Canonical decode mode wiring

- **Origin:** `encoding.DecodeCU64Standalone` accepts a `canonical bool` flag, and `Path.Decode` / `DecodeEntry` currently always pass `false`.
- **Why deferred:** Pre-MVP entries are produced by our own encoder, which is always minimal. No risk of non-canonical inputs in the local workflow.
- **Impact:** Cannot reject malformed-but-decodable inputs from a hostile peer. Required before any cross-impl interop where the peer is untrusted.
- **Revisit:** Phase 2 alongside Confidential Sync. Add `DecodeCanonic` variants on `Path` and `Entry` and wire them through.

### `Bytes32` (Rust harness) missing `Decodable` / `DecodableCanonic`

- **Origin:** `testdata/_genfixtures/src/bytes32.rs` only implements `Encodable` + `EncodableKnownLength` because the harness only encodes.
- **Why deferred:** Decoding traits hit a `ProduceAtLeastError` -> `DecodeError` mismatch that was not chased down; not needed for fixture generation.
- **Impact:** Cannot use the harness to round-trip-decode its own fixtures as an internal sanity check. Acceptable since the Go test suite already round-trips.
- **Revisit:** When the harness gets replaced by a Go generator at MVP (planned per `testdata/_genfixtures/README.md`). At that point Rust harness goes away entirely.

### Payload bytes storage

- **Origin:** `Store` interface holds only `Entry` metadata (digest, length) — no payload bytes. Rust `Store::create_entry` takes a payload producer and stores the bytes alongside.
- **Why deferred:** Pre-MVP focused on the prefix-pruning invariant and Range3d queries. Payload bytes add an orthogonal concern (size, streaming, slicing) that does not block the data-model verification work.
- **Impact:** No `GetPayload`, no `payload_slice` queries. A real consumer (smoketest, gomobile bind) would need to attach payload bytes out-of-band or hold them separately.
- **Revisit:** When smoketest or willow25 wants to round-trip payloads, OR Phase 2 with the persistent store. Plan: extend `Store` with `InsertWithPayload(e Entry, payload []byte)` + `GetPayload(ns, sub, path)` returning `[]byte`, OR define a parallel `PayloadStore` interface.

### `AuthorisedEntry` wrapping

- **Origin:** Rust `Store` operates on `AuthorisedEntry<...,AT>` (entry + authorisation token). Our `Store` takes raw `Entry`.
- **Why deferred:** Wrapping prematurely would shape the Store API around a missing dependency.
- **Impact:** Anyone can insert any entry into a Store; no capability check at insert time. Acceptable for pre-MVP since the Store is a local primitive, not yet a sync endpoint.
- **Revisit:** When `AuthorisedEntry` exists, add `Store.InsertAuthorised(AuthorisedEntry)` that verifies the token before delegating to `Insert`.

### `Area`-based Store queries

- **Origin:** Rust `Store::get_area` and `Store::forget_area` query / delete entries within an `Area`. Our `Store` exposes `Query(ns, Range3d)` and `ForgetEntry(ns, sub, path)` — no Area-keyed operations.
- **Why deferred:** `Area` itself now exists with `AsRange3d` conversion, but the `Store` interface was not extended to take Areas directly.
- **Impact:** Callers wanting Area semantics must do `store.Query(ns, area.AsRange3d())` themselves. Minor ergonomics gap; functionally equivalent.
- **Revisit:** Add convenience methods (`Store.QueryArea(ns, Area)`, `Store.ForgetArea(ns, Area)`) when a real consumer needs them. Cheap addition.

### `Area` byte encoding (`encode_area_in_area`)

- **Origin:** Spec defines a relative-to-area encoding used by Confidential Sync to compactly transmit area descriptors.
- **Why deferred:** Sync is Phase 2. Encoding would be dead code right now.
- **Impact:** None for pre-MVP. Required before any cross-impl interop.
- **Revisit:** Phase 2 alongside sync.

### Linear scan performance

- **Origin:** `InMemoryStore` walks all entries on Insert / Get / Query / Forget. O(n) per op, O(n^2) worst-case for prune scans.
- **Why deferred:** Pre-MVP correctness first; tests run on <100 entries.
- **Impact:** Won't scale beyond a few thousand entries.
- **Revisit:** When a benchmark or smoketest hits the slowdown. Plan: keep a per-namespace sub-map keyed by `string(subspace) + string(path.Encode())` for O(1) Get, plus a btree indexed by subspace then path for range queries. Out of scope for pre-MVP.

### Live subscriptions / notifications

- **Origin:** Rust `store.rs` has a TODO note about subscription subtraits.
- **Why deferred:** Pre-MVP needs no live-update consumers.
- **Impact:** No way to react to inserts/forgets in real time.
- **Revisit:** Phase 2 alongside Confidential Sync (which needs change notifications to drive outgoing-sync state).

### Read-mode delegation handover cross-impl fixture

- **Origin:** The cross-impl handover interop test in `meadowcap/handover_interop_test.go` covers 4 write-mode delegation chains; the originally-planned read-mode chain was dropped because `meadowcap::WriteCapability::new_communal` hard-wires `AccessMode::Write` — generating a read-mode fixture requires a parallel `ReadCapability` harness pathway with its own `try_delegate` plumbing.
- **Why deferred:** Write-mode interop alone validates the handover-byte composition for both first and subsequent delegations (the mode byte affects only the first communal handover and is otherwise unused by the spec layout). A read-mode fixture would confirm the mode-byte branch but the byte assembly is otherwise identical.
- **Impact:** Read-mode chains produced by willow_rs are not directly verified to validate in our Go impl. The Go side handles read mode in `modeByte` and produces byte-0 handovers, but we have not cross-validated this against the Rust impl.
- **Revisit:** Add `gen-handover` second build_chain pathway for `meadowcap::ReadCapability`. Probably ~40 LoC extra Rust. Low priority because the byte layout differs only in the first byte (0 vs 1) which is trivially testable in Go alone.

### Owned namespace capabilities

- **Origin:** Owned namespaces have a single owner whose signature is required to mint the genesis capability (see willow_rs `OwnedGenesis`). Owned cap of "full namespace" with delegations is the typical pattern for app-managed namespaces.
- **Why deferred:** Pre-MVP path is communal (each user gates their own subspace). Owned namespaces add an initial_authorisation signature step over the user_key but otherwise reuse the delegation chain machinery.
- **Impact:** Cannot represent app-owned namespaces (the willow25 default for production). Communal works for peer-to-peer demos.
- **Revisit:** Bundled with delegation chains in Phase 2.

### Read capabilities

- **Origin:** Rust has separate `ReadCapability` and `WriteCapability` types with parallel APIs. Our Go impl has a single `CommunalCapability` with an `AccessMode` field but no read-side enforcement (Store has no read ACL yet).
- **Why deferred:** No Store-level read ACL in pre-MVP. Read caps will matter at sync time (they gate what each peer is allowed to receive from the other).
- **Impact:** None for current scope — write auth is what gates `Store.Insert`. Read filtering will arrive when sync does.
- **Revisit:** Phase 2 alongside Confidential Sync, or sooner if a read-side gating API surfaces.

### Capability byte encoding (`encode_mc_capability`)

- **Origin:** The spec defines an encoding for capabilities used in sync messages.
- **Why deferred:** Same family as the sync layer — Phase 2.
- **Impact:** Cannot transmit capabilities to peers. Local-only use works.
- **Revisit:** Phase 2.

### willow25 typed wrappers

- **Origin:** Rust willow25 exposes wrapper types `NamespaceId` / `SubspaceId` / `PayloadDigest` that ensure compile-time correctness (e.g., a `NamespaceId` cannot be passed where a `SubspaceId` is expected even though both are 32 bytes). Our Go impl uses raw `[]byte` slices throughout, validated at runtime via length checks.
- **Why deferred:** Type-safe wrappers in Go add boilerplate (one struct per type with constructor + accessor + conversion) for marginal compile-time safety. Pre-MVP chose runtime validation.
- **Impact:** No compile-time prevention of accidentally swapping the two ids. Tests catch it but at runtime.
- **Revisit:** If a real consumer trips over a swap. Probably never given Go conventions.

### Android AAR build verification

- **Origin:** The `mobile/` package is gomobile-bindable. `make mobile-ios` produces `Mobile.xcframework` end-to-end (tested locally with Xcode 26.5 + iOS SDK 26.5). `make mobile-android` reaches the javac stage but errors out with "Unable to locate a Java Runtime" — no JDK on the current build host. Android NDK 27 is present at `~/Library/Android/sdk/ndk/27.0.12077973`.
- **Why deferred:** Installing a JDK is a one-off host setup. Documented in Makefile.
- **Impact:** Android target not directly tested on this host. The package is gomobile-compatible (verified via iOS), so the only remaining work is the JDK install + `make mobile-android` smoke run.
- **Revisit:** When the JDK install happens, or in CI on a Linux runner that already has it. Worth recording the produced .aar size as a README badge.

### Mobile API: gomobile end-to-end Swift / Kotlin demo

- **Origin:** The `mobile/` package exports PathBuilder, EntryBuilder, HashPayload, and HashAndEncodeEntry to gomobile. The iOS XCFramework was confirmed to build and to contain the expected Objective-C class headers (MobilePathBuilder, MobileEntryBuilder). A tiny Swift app that calls these and shows the encoded bytes was not built — that would be the strongest evidence for demos and ecosystem write-ups.
- **Why deferred:** Building an Xcode project + Swift driver is ~half a day, mostly Xcode UI plumbing.
- **Impact:** "It builds" is weaker evidence than "here is an asciinema of an iOS app hashing a payload via willow-go." For demos and write-ups, the latter is significantly more persuasive.
- **Revisit:** Pair with public-release README polish. Plan: a 1-screen SwiftUI app with a text input and a "Hash" button that calls `MobileHashPayload` and displays the hex.

### willow25 keypair generation helpers

- **Origin:** Rust willow25 has `randomly_generate_subspace(rng)` returning `(SubspaceId, SubspaceSecret)`. We expose nothing — callers must use `crypto/ed25519.GenerateKey(rand.Reader)` directly.
- **Why deferred:** Trivial wrappers, not blocking.
- **Impact:** Minor ergonomics. Anyone reading meadowcap tests sees direct `ed25519.GenerateKey` calls.
- **Revisit:** Add `RandomNamespace()` / `RandomSubspace()` wrappers when a public-API consumer needs them.

### Upstream willow_test_vectors: relative-encoding adoption blocked

- **Origin:** Discovered `github.com/worm-blossom/willow_test_vectors` and added as a git submodule at `testdata/upstream_vectors/`. Absolute path encodings (encode_path + EncodePath) adopted successfully — 11 positive + 165 attacker-supplied negative vectors pass; the negative-test pass found and fixed one panic-level bug in `datamodel/paths.go` decodeComponents (uint64 → int conversion without bounds check on attacker-supplied huge length64). Relative encodings (path_rel_path, EncodePathRelativePath, path_extends_path, EncodePathExtendsPath) **DO NOT** match our impl OR willow_rs v0.7.0 OR the spec text on willowprotocol.org as of May 2026.
- **Why deferred:** The upstream `reencoded/` files for relative path encodings show a canonical form that neither matches the documented prefix-count rule nor what willow_rs v0.7.0 emits. Per direct communication with the worm-blossom team (May 2026), some upstream test vectors are out of date because the encodings have changed since they were generated; the spec always trumps the test vectors. Without authoritative documentation of the new canonical rule, aligning our impl is speculative and would break our existing byte-compat claim vs willow_rs.
- **Impact:** Only 11/108 positive vectors and 165/2453 negative vectors are exercised. The remaining ~92 yay and ~2300 nay cover the encodings we cannot align without spec clarification.
- **Revisit:** When (a) willow_rs HEAD progresses past dd87996 with new encoder logic that matches the test_vectors, OR (b) the spec text on willowprotocol.org is updated to document the new canonical rule, OR (c) maintainers respond with clarification. Plan: re-enable the disabled runners (preserved as `runRelativePathRelPathVectors_DISABLED` etc.), fix any divergences, and commit a new pin.

### Upstream willow_test_vectors: capability + 3dRange encodings

- **Origin:** Among the 16 upstream encoding directories, several depend on encoders we have not implemented at all: EncodeCommunalCapability + EncodeOwnedCapability + EncodeMcCapability (need spec-compliant byte encoding of capabilities, currently we only have signature semantics), Encode3dRangeRelative3dRange (need 3dRange byte encoding, currently absent), EncodeEntryInNamespace3dRange (depends on 3dRange).
- **Why deferred:** These encodings are used by Confidential Sync, which is Phase 2. Implementing them in pre-MVP would be premature scope creep.
- **Impact:** ~30 positive vectors + ~1200 negative vectors not exercised.
- **Revisit:** Phase 2 alongside Confidential Sync.

### Path `Hash` implementation

- **Origin:** Rust impl uses `anyhash::Hasher` for stable cross-process hashing.
- **Why deferred:** Go map keys need comparable types; current `Path` (containing `[][]byte`) is not comparable, so it cannot be a map key directly. Workarounds: use `Path.Encode()` as the map key.
- **Impact:** Cannot use `Path` directly as a `map[Path]V` key. Use `map[string]V` with `string(p.Encode())` as the key.
- **Revisit:** If a hot path emerges. For now, encode-as-string is fine.

---

## Operational / process debt

### Rust toolchain dependency for fixture regeneration

- **Origin:** Generating fixtures requires `cargo` (rustup-installed).
- **Why:** Currently the harness is Rust; this is the chosen pre-MVP strategy.
- **Impact:** Anyone regenerating fixtures (CI, contributors) needs Rust 1.85+ installed. Documented in `testdata/_genfixtures/README.md`.
- **Revisit:** At MVP, the Rust harness gets rewritten in Go; fixtures themselves stay in the repo, regeneration uses only `go run`. At that point this debt closes.

### Upstream pin staleness

- **Origin:** `testdata/_genfixtures/Cargo.toml` pins `willow-data-model` to commit `dd879968840417bf258bf517ef5460768f72c494` (HEAD on 25 April 2026).
- **Why:** Determinism over freshness during the port.
- **Impact:** If upstream changes the encoding format, our fixtures will become stale. Per direct communication with the worm-blossom team (May 2026), the Rust codebase has been reworked since this pin and the team is actively iterating on the new structure. Some encodings have drifted from this pinned commit.
- **Revisit:** Periodically pull upstream HEAD and re-generate fixtures, verify all Go tests still pass. Update the pin once worm-blossom signals the new codebase is stable enough to track.

---

## Closed

### `Area` core type (formerly "Area and AreaOfInterest")

- **Closed by:** chunk 5.5.
- **Resolution:** Implemented `Area` struct with `Subspace *[]byte` (nil = any subspace), `PathPrefix`, `Times`. Methods: `Includes` / `IncludesEntry` / `Intersect` / `IsFull` / `FullArea` / `SubspaceArea` / `AsRange3d`. The `AreaOfInterest` wrapper remains deferred (see Phase-2-adjacent section above).

### `Path.greater_but_not_prefixed`

- **Closed by:** chunk 5.5.
- **Resolution:** Implemented per the willow_rs algorithm: scan components right-to-left, try (a) appending 0x00 to component i within MCL/MPL limits, else (b) increment the rightmost non-FF byte and truncate. Used by `Area.AsRange3d` for the path-axis bound.

### path_extends_path encoding

- **Closed by:** commit `e26253a`.
- **Resolution:** Added `Path.EncodeExtending(prefix)` and package-level `DecodeExtending(prefix, src)`. Validated byte-identical against 7 willow_rs harness fixtures covering empty prefix + empty path, empty prefix + single component, equal prefix and path, single-component prefix extending by one / by two, two-component prefix extending by one, and a suffix containing zero bytes.

### Area relative encoding (`encode_area_in_area`)

- **Closed by:** commit `f8c85f4`.
- **Resolution:** Added `Area.EncodeRelativeTo(rel)` and `DecodeAreaRelativeTo(limits, rel, subspaceWidth, src)`. Header byte packs subspace-encoded flag, times-end-open flag, two diff-from-start vs end flags, plus two 2-bit CompactU64 tags. Validated byte-identical against 8 willow_rs harness fixtures including all four start_from_start / end_from_start branches.

### William3 payload digest

- **Closed by:** chunk 9.
- **Resolution:** Ported the bab_rs v0.5.0 WILLIAM3 implementation directly into `willow25/william3.go` (~110 Go LoC). WILLIAM3 is the BLAKE3 compression function with a substituted IV (the BLAKE3 IV replaced by `[0xc88f633b, 0x4168fbf2, ...]` per bab_rs/src/william3/basics.rs); same message schedule, same chunk size (1024 bytes), same Merkle tree structure. `willow25.HashPayload` now calls `William3Sum`; the previous lukechampine.com/blake3 dependency is removed from go.mod. Validated byte-identical against 11 upstream fixtures covering empty input, single-byte inputs, sub-chunk inputs, exactly-1023 / 1024 / 1025-byte chunk boundary, multi-chunk inputs (2048, 3000, 5000 bytes).

### Meadowcap delegation chains

- **Closed by:** chunk 6.5c.
- **Resolution:** Extended `CommunalCapability` with `Delegations []Delegation` field. Added `AppendDelegation(prevPrivateKey, newArea, newUserKey)` that verifies area inclusion + key match and signs the handover bytes. Added `IsValid()` that walks the chain, verifying each delegation's area inclusion and signature against the previous receiver. `AuthorisationToken.Verify` now requires `IsValid()` to pass first. Handover bytes format: for first delegation `[mode_byte || namespace_key || area_encoded(initial_area) || new_receiver]`; for subsequent `[area_encoded(prev_area) || prev_signature || new_receiver]`. See related entry "Cross-impl interop test for delegation handover bytes" in active section above.
