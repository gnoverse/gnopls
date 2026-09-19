# `gnopls`, the Gno language server

`gnopls` gives any [LSP]-capable editor Gno support: diagnostics, go-to-definition,
references, document symbols, folding, and formatting through `gno fmt`. It is a fork of
[`gopls`](https://github.com/golang/tools/tree/master/gopls), the Go language server, with
the package resolver and the type checker taught about Gno.

> [!WARNING]
> **Experimental.** Cross-package and stdlib resolution is unreliable — see
> [Known limitations](#known-limitations) before you rely on it.

[![PkgGnoDev](https://pkg.go.dev/badge/github.com/gnoverse/gnopls)](https://pkg.go.dev/github.com/gnoverse/gnopls)

## Install

Most editors install and update `gnopls` for you — check [Editor setup](#editor-setup)
first, and skip this section if yours does.

```sh
go install github.com/gnoverse/gnopls@v0.1.0
```

- **Go 1.26.4 or newer** is required (`toolchain go1.26.4` in `go.mod`). Older toolchains
  are refused at build time rather than producing a broken binary.
- Prebuilt binaries for linux, macOS and Windows are attached to every
  [release](https://github.com/gnoverse/gnopls/releases).
- `@latest` tracks `main` and is where fixes land first; `@v0.1.0` is the pinnable one.

You also need the [`gno` toolchain](https://github.com/gnolang/gno) on your `PATH`:
formatting shells out to `gno fmt`, and package resolution needs a `GNOROOT` that points at
a real gno checkout.

Confirm the install actually runs — a language server that crashes at startup looks
identical to one that is missing:

```sh
gnopls version
```

## Editor setup

| Editor | Use | Maintained |
|---|---|---|
| **VS Code** | [Gnolang](https://marketplace.visualstudio.com/items?itemName=Gnoverse.gnolang) (`Gnoverse.gnolang`), from [`gnoverse/vscode-gno`](https://github.com/gnoverse/vscode-gno) — installs and updates `gnopls` for you | ✅ official |
| **Zed** | [`julienrbrt/zed-gno`](https://github.com/julienrbrt/zed-gno) | ✅ community |
| **JetBrains / GoLand** | [`gnoverse/intellij-gno`](https://github.com/gnoverse/intellij-gno) | ✅ community |
| **Neovim / Vim** | manual config, below | — |
| **Emacs** | manual config, below | — |
| **Sublime Text** | no `gnopls` client. [`jdkato/gnols`](https://github.com/jdkato/gnols) is a *different*, unrelated language server, unmaintained since 2023 | ❌ |

`x1unix/gno.nvim` was archived in January 2025; use the Neovim configuration below instead.

End-user setup is also documented at
[docs.gno.land/builders/editor-setup](https://docs.gno.land/builders/editor-setup), which is
the canonical page for getting an editor working. This README owns installation,
limitations and development.

<details>
<summary><b>Neovim</b> (built-in LSP)</summary>

```lua
-- Register .gno files
vim.filetype.add({
  extension = {
    gno = 'gno',
  },
})

-- Set up gnopls
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

if not configs.gnopls then
  configs.gnopls = {
    default_config = {
      cmd = {'gnopls'},
      filetypes = {'gno'},
      root_dir = lspconfig.util.root_pattern('gnomod.toml', 'gnowork.toml', '.git'),
      settings = {},
    },
  }
end

lspconfig.gnopls.setup{}
```

Neovim 0.11 and newer also offer the built-in `vim.lsp.config` / `vim.lsp.enable` API; this
snippet has not been retested against it.
</details>

<details>
<summary><b>Vim</b> (vim-lsp)</summary>

Install [`vim-lsp`](https://github.com/prabirshrestha/vim-lsp), then add to your `.vimrc`:

```vim
augroup gno_autocmd
    autocmd!
    autocmd BufNewFile,BufRead *.gno
        \ set filetype=gno |
        \ set syntax=go
augroup END

if (executable('gnopls'))
    au User lsp_setup call lsp#register_server({
        \ 'name': 'gnopls',
        \ 'cmd': ['gnopls'],
        \ 'allowlist': ['gno'],
        \ 'config': {},
        \ 'languageId': {server_info->'gno'},
    \ })
else
    echomsg 'gnopls binary not found: LSP disabled for Gno files'
endif

function! s:on_lsp_buffer_enabled() abort
    setlocal omnifunc=lsp#complete
    autocmd BufWritePre <buffer> LspDocumentFormat
    nmap <buffer> gd <plug>(lsp-definition)
    nmap <buffer> <leader>rr <Plug>(lsp-rename)
    nmap <buffer> <leader>ri <Plug>(lsp-implementation)
    nmap <buffer> <leader>rf <Plug>(lsp-references)
    nmap <buffer> <leader>i <Plug>(lsp-hover)
endfunction

augroup lsp_install
    au!
    autocmd User lsp_buffer_enabled call s:on_lsp_buffer_enabled()
augroup END
```
</details>

<details>
<summary><b>Emacs</b> (lsp-mode)</summary>

```lisp
;; gno-mode as an alias for go-mode
(define-derived-mode gno-mode go-mode "GNO"
  "Major mode for GNO files, an alias for go-mode."
  (setq-local tab-width 8))

(define-derived-mode gno-dot-mod-mode go-dot-mod-mode "GNO Mod"
  "Major mode for GNO mod files, an alias for go-dot-mod-mode.")

(add-to-list 'auto-mode-alist '("\\.gno\\'" . gno-mode))
(add-to-list 'auto-mode-alist '("gnomod\\.toml\\'" . gno-dot-mod-mode))

(with-eval-after-load 'lsp-mode
  (add-to-list 'lsp-language-id-configuration '(gno-mode . "gno"))
  (lsp-register-client
   (make-lsp-client
    :new-connection (lsp-stdio-connection "gnopls")
    :major-modes '(gno-mode)
    :language-id "gno"
    :server-id 'gnopls)))

(add-hook 'gno-mode-hook #'lsp-deferred)
```

Linting through Flycheck:

```lisp
(require 'flycheck)

(flycheck-define-checker gno-lint
  "A GNO syntax checker using the gno lint tool."
  :command ("gno" "lint" source-original)
  :error-patterns ((error line-start (file-name) ":" line ": " (message) " (code=" (id (one-or-more digit)) ")." line-end))
  :predicate (lambda ()
               (and (not (bound-and-true-p polymode-mode))
                    (flycheck-buffer-saved-p)))
  :modes gno-mode)

(add-to-list 'flycheck-checkers 'gno-lint)
```
</details>

Using an editor that is not listed? Send a PR.

## What works

Checked with the `gnopls` CLI against a one-package workspace (`gnowork.toml` +
`gnomod.toml`) on 2026-09-19, v0.1.0:

| | |
|---|---|
| Diagnostics (`check`) | ✅ |
| Document symbols | ✅ types, fields, methods, functions |
| Go to definition, **same package** | ✅ including methods |
| Find references, **same package** | ✅ |
| Folding ranges | ✅ |
| Formatting | ✅ via `gno fmt`; needs `gno` on `PATH` and a valid `GNOROOT` |
| Go to definition, **imported package or stdlib** | ❌ `no package data` ([#8], [#15], [#16]) |
| Document links | ⚠️ point at `pkg.go.dev` instead of gnoweb ([#14], fix in [#17]) |

Completion, hover and inlay hints are served over LSP only and were not measured here.

### Known limitations

- **Cross-package resolution is the weak spot.** Definition and references work inside a
  package; as soon as a symbol comes from an import — including `std` — the server answers
  `no package data`. Setting `GNOROOT` does not change it. [#8], [#15], [#16].
- Import links in hovers resolve to `pkg.go.dev`, which does not host Gno packages. [#14]
- The server logs `builtin type "x" has been registered` and a debug line to stderr on every
  start. Harmless, but it clutters editor logs.

## Troubleshooting

**The editor says the server crashed, or nothing happens.** Run `gnopls version` in a
terminal. If that prints a version, the binary is fine and the problem is the editor's
configuration; if it panics or prints nothing, reinstall — and check your Go version, since
a toolchain older than 1.26.4 cannot build it.

**`can't format: running 'gno fmt': exit status 1`.** Your `gno` binary cannot find its
standard library. This usually means `GNOROOT` is unset, or the binary was built with a
`GNOROOT` baked in that no longer exists. `gno fmt <file>` in the same directory reproduces
it outside the editor; set `GNOROOT` to a gno checkout and restart the server.

**`no packages found for open file` / `missing metadata for import`.** Known — see
[Known limitations](#known-limitations). Restarting the server sometimes clears it.

**Where are the logs?** `gnopls` writes to stderr; your editor decides where that lands
(`:LspLog` in Neovim, the *Output → gnopls* panel in VS Code).

## Development

```sh
make install         # go install .
make test            # ./pkg/... — the Gno-specific code
make test-internal   # the inherited gopls suite that passes; see below
make lint            # gofmt + go vet, scoped to the Gno-owned code
make smoke           # build, then actually start the binary and query it
```

The tree is in two halves:

- **`pkg/`** and **`internal/gnolang/`** are Gno-specific — the resolver, the Gno builtins,
  the `gno fmt` bridge. This is where nearly all Gno work happens, and the only code `make
  lint` covers.
- **`internal/`** is a vendored `gopls` tree, forked at `golang.org/x/tools v0.25.0`
  (October 2024) and not resynced since. Expect upstream conventions there.

`make test-internal` runs everything under `internal/` except the packages named in
[`.github/known-broken-tests.txt`](.github/known-broken-tests.txt), which lists what fails
and why. Shrinking that file is welcome work. It sets `GOPACKAGESDRIVER=off`: without it the
fallback driver is `os.Executable()`, which inside a test binary means the test binary
re-executing itself until the timeout.

CI runs all of the above on every PR, over a Go version matrix, plus cross-compilation for
linux, macOS and Windows.

### Releasing

Tag `vX.Y.Z` and push it. `.github/workflows/release.yml` builds the five platform binaries,
writes `checksums.txt` and attaches everything to the GitHub release.

## Documentation

`doc/` is the upstream `gopls` documentation, kept for reference — it describes `gopls`, and
most of it has not been adapted to Gno. See [`doc/README.md`](doc/README.md).

[LSP]: https://microsoft.github.io/language-server-protocol/
[#8]: https://github.com/gnoverse/gnopls/issues/8
[#14]: https://github.com/gnoverse/gnopls/issues/14
[#15]: https://github.com/gnoverse/gnopls/issues/15
[#16]: https://github.com/gnoverse/gnopls/issues/16
[#17]: https://github.com/gnoverse/gnopls/pull/17
