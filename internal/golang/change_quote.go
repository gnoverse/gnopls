// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"github.com/gfanton/gnopls/internal/cache/parsego"
	"github.com/gfanton/gnopls/internal/file"
	"github.com/gfanton/gnopls/internal/protocol"
	"github.com/gfanton/gnopls/internal/settings"
	"github.com/gfanton/gnopls/internal/util/bug"
	"github.com/gfanton/gnopls/internal/util/safetoken"
	"github.com/gfanton/gnopls/internal/diff"
)

// convertStringLiteral reports whether we can convert between raw and interpreted
// string literals in the [start, end) range, along with a CodeAction containing the edits.
//
// Only the following conditions are true, the action in result is valid
//   - [start, end) is enclosed by a string literal
//   - if the string is interpreted string, need check whether the convert is allowed
func convertStringLiteral(pgf *parsego.File, fh file.Handle, startPos, endPos token.Pos) (protocol.CodeAction, bool) {
	path, _ := astutil.PathEnclosingInterval(pgf.File, startPos, endPos)
	lit, ok := path[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return protocol.CodeAction{}, false
	}

	str, err := strconv.Unquote(lit.Value)
	if err != nil {
		return protocol.CodeAction{}, false
	}

	interpreted := lit.Value[0] == '"'
	// Not all "..." strings can be represented as `...` strings.
	if interpreted && !strconv.CanBackquote(strings.ReplaceAll(str, "\n", "")) {
		return protocol.CodeAction{}, false
	}

	var (
		title   string
		newText string
	)
	if interpreted {
		title = "Convert to raw string literal"
		newText = "`" + str + "`"
	} else {
		title = "Convert to interpreted string literal"
		newText = strconv.Quote(str)
	}

	start, end, err := safetoken.Offsets(pgf.Tok, lit.Pos(), lit.End())
	if err != nil {
		bug.Reportf("failed to get string literal offset by token.Pos:%v", err)
		return protocol.CodeAction{}, false
	}
	edits := []diff.Edit{{
		Start: start,
		End:   end,
		New:   newText,
	}}
	textedits, err := protocol.EditsFromDiffEdits(pgf.Mapper, edits)
	if err != nil {
		bug.Reportf("failed to convert diff.Edit to protocol.TextEdit:%v", err)
		return protocol.CodeAction{}, false
	}
	return protocol.CodeAction{
		Title: title,
		Kind:  settings.RefactorRewriteChangeQuote,
		Edit:  protocol.NewWorkspaceEdit(protocol.DocumentChangeEdit(fh, textedits)),
	}, true
}
