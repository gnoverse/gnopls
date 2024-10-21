// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package file

import (
	"fmt"

	"github.com/gnoverse/gnopls/internal/protocol"
)

// Kind describes the kind of the file in question.
// It can be one of Go,mod, Sum, or Tmpl.
type Kind int

const (
	// UnknownKind is a file type we don't know about.
	UnknownKind = Kind(iota)

	// Gno is a Gno source file.
	Gno
	// Mod is a go.mod file.
	Mod
	// Sum is a go.sum file.
	Sum
	// Tmpl is a template file.
	Tmpl
	// Work is a go.work file.
	Work
)

func (k Kind) String() string {
	switch k {
	case Gno:
		return "gno"
	case Mod:
		return "gno.mod"
	case Sum:
		return "gno.sum"
	case Tmpl:
		return "tmpl"
	case Work:
		return "gno.work"
	default:
		return fmt.Sprintf("internal error: unknown file kind %d", k)
	}
}

// KindForLang returns the gopls file [Kind] associated with the given LSP
// LanguageKind string from protocol.TextDocumentItem.LanguageID,
// or UnknownKind if the language is not one recognized by gopls.
func KindForLang(langID protocol.LanguageKind) Kind {
	switch langID {
	case "gno":
		return Gno
	case "gno.mod":
		return Mod
	case "gno.sum":
		return Sum
	case "tmpl", "gotmpl":
		return Tmpl
	case "go.work":
		return Work
	default:
		return UnknownKind
	}
}
