# `gnopls`, the Gno language server

`gnopls` is a modified fork of https://github.com/golang/tools/tree/master/gopls

⚠️  `gnopls` is in an experimental phase; use with caution.

[![PkgGnoDev](https://pkg.go.dev/badge/github.com/gnoverse/gnopls)](https://pkg.go.dev/github.com/gnoverse/gnopls)

It provides a wide variety of [IDE features](doc/features/README.md) to any [LSP]-compatible editor.

## Editor Setup

`gnopls` is compatible with any editor that supports the Language Server Protocol (LSP). Below are setup instructions for popular editors.

## Installation

For the most part, you should not need to install or update `gnopls`. Your editor should handle that step for you.

If you do want to get the latest stable version of `gnopls`, run the following command:

```sh
go install github.com/gnoverse/gnopls@latest
```

### Visual Studio Code

There is an unofficial [Visual Studio Code extension](https://marketplace.visualstudio.com/items?itemName=harry-hov.gno) for working with `*.gno` files that includes LSP support.

1. Install the extension from the VS Code Marketplace
2. The extension will automatically use `gnopls` if it's installed in your PATH

### Vim/Neovim

#### Prerequisites
- Set `GNOROOT` environment variable to your gno repository path

#### Using vim-lsp

Install the [`vim-lsp`](https://github.com/prabirshrestha/vim-lsp) plugin, then add to your `.vimrc`:

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
    " Autocompletion
    setlocal omnifunc=lsp#complete
    " Format on save
    autocmd BufWritePre <buffer> LspDocumentFormat
    " Key mappings
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

#### Using Neovim built-in LSP

For Neovim users, you can use the built-in LSP client. Add to your `init.lua`:

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
      root_dir = lspconfig.util.root_pattern('gnomod.toml', '.git'),
      settings = {},
    },
  }
end

lspconfig.gnopls.setup{}
```

### Emacs

1. Install [go-mode.el](https://github.com/dominikh/go-mode.el)
2. Add to your Emacs configuration:

```lisp
;; Define gno-mode based on go-mode
(define-derived-mode gno-mode go-mode "GNO"
  "Major mode for GNO files, an alias for go-mode."
  (setq-local tab-width 8))

(define-derived-mode gno-dot-mod-mode go-dot-mod-mode "GNO Mod"
  "Major mode for GNO mod files, an alias for go-dot-mod-mode.")

;; Register file associations
(add-to-list 'auto-mode-alist '("\\.gno\\'" . gno-mode))
(add-to-list 'auto-mode-alist '("gnomod\\.toml\\'" . gno-dot-mod-mode))
```

#### LSP Setup with lsp-mode

If using [lsp-mode](https://github.com/emacs-lsp/lsp-mode):

```lisp
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

#### Flycheck Integration

For linting with Flycheck:

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

### Sublime Text

There is a community-developed [Gno Language Server](https://github.com/jdkato/gnols) with installation instructions for Sublime Text.

### Other Editors

If you use `gnopls` with an editor that is not on this list, please send us a PR to add instructions!
