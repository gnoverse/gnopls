# This directory is upstream `gopls` documentation

`gnopls` is a fork of [`gopls`](https://github.com/golang/tools/tree/master/gopls) taken at
`golang.org/x/tools v0.25.0` (October 2024). Everything in `doc/` came with that fork and
has, with few exceptions, **not been adapted to Gno**.

Read it as background on how the server works internally — the architecture, the LSP
plumbing, the feature vocabulary — and not as a description of what `gnopls` does. Pages
here refer to `gopls`, to Go modules, to `go.mod`, and to features this fork may not have
wired up at all. `doc/release/` is upstream's release notes, for upstream's versions.

For `gnopls` itself:

- [`../README.md`](../README.md) — install, editor setup, what actually works, development.
- [docs.gno.land/builders/editor-setup](https://docs.gno.land/builders/editor-setup) — the
  canonical end-user setup page.
- [Issues](https://github.com/gnoverse/gnopls/issues) — known problems.

Rewriting a page here for Gno is welcome; say so in the PR so the file can be moved out of
this reference pile.
