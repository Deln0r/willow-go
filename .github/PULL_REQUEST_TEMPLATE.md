<!--
Thanks for the PR. Please read CONTRIBUTING.md first if you have not.
Fill in the sections below and delete any that do not apply.
-->

## What and why

<!-- A short description of the change and the reason for it. Link any related issue. -->

## Type of change

- [ ] Bug fix
- [ ] Byte-encoder change (adds or modifies a wire encoding)
- [ ] Mobile bindings (`mobile/`)
- [ ] CI / tooling
- [ ] Documentation
- [ ] Other

## Cross-implementation impact

<!--
Does this change the bytes produced or accepted on the wire?
If yes: which fixtures did you add or update in testdata/, and was the
divergence checked against willow_rs? Cross-impl interop is highest priority.
If no: state "no wire change".
-->

## Checklist

- [ ] `make test` passes (all packages green)
- [ ] `make smoketest` passes (byte-compat fixtures 0-fail)
- [ ] `gofmt -s -w .` run on changed files; `go vet ./...` passes
- [ ] Added a test that fails without this change and passes with it
- [ ] Encoder changes only: extended the `testdata/` fixture corpus and added a smoketest case (code-only encoder changes are not merged)
- [ ] Mobile changes only: validated the affected binding(s) per CONTRIBUTING.md
- [ ] Scope confirmed: not a Phase 2 item (Confidential Sync, persistent store, transport encryption, owned/read capabilities). If unsure, I opened an issue first.
- [ ] Commits use standard git authorship (no tool-generated attribution trailers)
