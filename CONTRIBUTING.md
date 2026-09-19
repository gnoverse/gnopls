# Contributing to `gnopls`

## Getting set up

```sh
git clone https://github.com/gnoverse/gnopls
cd gnopls
make install   # needs Go 1.26.4 or newer
make smoke     # builds, then actually starts the binary and queries it
```

You also want the [`gno` toolchain](https://github.com/gnolang/gno) on your `PATH` with a
valid `GNOROOT` — formatting and package resolution both go through it.

## Where things live

- **`pkg/`**, **`internal/gnolang/`** — the Gno-specific code: the package resolver, the Gno
  builtins, the `gno fmt` bridge. Nearly all Gno work happens here, and it is the only code
  `make lint` covers.
- **`internal/`** — a vendored `gopls` tree, forked at `golang.org/x/tools v0.25.0` (October
  2024). Follow upstream's conventions when you touch it, and keep diffs minimal: the
  smaller the divergence, the cheaper a future resync.
- **`doc/`** — upstream `gopls` documentation, not adapted. See [`doc/README.md`](doc/README.md).

## Before you open a PR

```sh
make lint
make test
make test-internal
```

CI runs all three plus a smoke test and cross-compilation, over a Go version matrix. A build
that compiles is not enough: `make smoke` starts the binary, because this project has
already shipped a release that compiled perfectly and panicked in `init()` on every
invocation ([#59](https://github.com/gnoverse/gnopls/issues/59)).

`make test-internal` skips the packages listed in
[`.github/known-broken-tests.txt`](.github/known-broken-tests.txt). If your change fixes one,
delete its line in the same PR — that file is the debt ledger and it should only ever
shrink.

## Reporting a bug

Include the output of `gnopls version`, your editor and its Gno extension, your Go version,
and whether `GNOROOT` is set. Server logs help enormously: `:LspLog` in Neovim, the
*Output → gnopls* panel in VS Code.

Security issues go through [`SECURITY.md`](SECURITY.md), not the issue tracker.
