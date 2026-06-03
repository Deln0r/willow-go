# Security Policy

## Supported Versions

willow-go is pre-MVP (v0.1.0). Security fixes are applied to the latest tagged release on `main`. Older minor versions are not patched.

| Version | Supported |
| ------- | --------- |
| 0.1.x   | yes       |
| < 0.1   | no        |

## Reporting a Vulnerability

If you find a security issue, please report it privately rather than opening a public GitHub issue.

Email: ian00chechin@gmail.com

Please include:

- A description of the issue and its impact
- Steps to reproduce, or a minimal proof-of-concept
- The affected version (or commit hash)
- Any suggested mitigation, if known

I aim to acknowledge reports within 5 working days and provide an initial assessment within 14 days. Coordinated disclosure timelines are negotiated case by case; the default window is 90 days from initial report to public advisory.

## Scope

In scope:

- Encoding and decoding of Willow data-model types (`Path`, `Entry`, `Range3d`, `Area`), including the `CompactU64` codec and absolute / relative path encodings
- Meadowcap capability verification: communal capability delegation chains, Ed25519 handover signatures, AuthorisedEntry validation
- WILLIAM3 payload digest (BLAKE3 compression function with a substituted IV) determinism and length-prefix handling
- Attacker-supplied input to any decoder that could cause panic, out-of-bounds read, infinite loop, or unbounded memory growth (the upstream `willow_test_vectors` negative-test corpus already exercises this surface; new attack patterns outside that corpus are explicitly welcome)
- gomobile-bound bindings (iOS, Android) where Go-side behaviour creates a vulnerability surfaced to the host application

Out of scope:

- Vulnerabilities in dependencies. willow-go currently has one direct dependency (`golang.org/x/mobile`) plus stdlib-adjacent indirects (`golang.org/x/mod`, `golang.org/x/sync`, `golang.org/x/tools`). Please report advisories for those upstream to the Go team.
- Confidential Sync (the layer formerly called WGPS), persistent store backend, owned and read capabilities, transport encryption. These are Phase 2 and not yet shipped; see [TECH_DEBT.md](TECH_DEBT.md). Reports against unshipped code are tracked as design feedback rather than as security advisories.
- Issues that require a non-default, intentionally insecure configuration

## Acknowledgements

Reporters will be credited in the release notes for the fix, unless anonymity is requested.
