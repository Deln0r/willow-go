---
name: Bug report
about: Report incorrect behaviour, a crash, or a byte-divergence vs willow_rs
title: ''
labels: bug
assignees: ''
---

<!-- Cross-implementation interop bugs are highest priority. See CONTRIBUTING.md. -->

## Version

<!-- Commit SHA or release tag (e.g. v0.1.0). -->

## What happened

<!-- Observed behaviour. -->

## What you expected

<!-- Expected behaviour. -->

## Minimal reproduction

<!-- Minimal Go code that reproduces the issue. -->

```go

```

## Byte-encoding bugs

<!--
If this is an encoding/decoding divergence, include hex dumps of:
- input
- expected output
- actual output
and, if you have it, the willow_rs equivalent. If the divergence is not
already covered by a fixture under testdata/, please say so.
-->

## Environment

- Go version (`go version`):
- OS / arch:
