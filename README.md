# willow-go

Pure-Go implementation of the [Willow Protocol](https://willowprotocol.org) — a
peer-to-peer protocol for synchronizable data stores with capability-based
permissions.

Tracks the Rust reference [`willow_rs`](https://codeberg.org/worm-blossom/willow_rs)
and the protocol specification. Pre-MVP scope: data model, capabilities, and
in-memory store; WGPS sync deferred to a later phase.

## Status

Pre-MVP, private development. Not yet ready for external use.

## Layout

```
datamodel/  Paths, Entries, Groupings, byte-encodings, Store interface
meadowcap/  Capability-based authorisation (Ed25519 signatures)
willow25/   Default-parametrized bundle for interop with Rust impls
encoding/   Shared binary encoders/decoders
internal/   Implementation details, not part of the public API
testdata/   Upstream willow_test_vectors fixtures
cmd/
  willow-smoketest/  CLI that runs upstream fixtures and asserts byte-identity
```

## License

MIT.
