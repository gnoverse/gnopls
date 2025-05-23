// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package useany defines an Analyzer that checks for usage of interface{} in
// constraints, rather than the predeclared any.
package useany

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `check for constraints that could be simplified to "any"`

var Analyzer = &analysis.Analyzer{
	Name:     "useany",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
	URL:      "https://pkg.go.dev/github.com/gnoverse/gnopls/internal/analysis/useany",
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	universeAny := types.Universe.Lookup("any")

	nodeFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
		(*ast.FuncType)(nil),
	}

	inspect.Preorder(nodeFilter, func(node ast.Node) {
		var tparams *ast.FieldList
		switch node := node.(type) {
		case *ast.TypeSpec:
			tparams = node.TypeParams
		case *ast.FuncType:
			tparams = node.TypeParams
		default:
			panic(fmt.Sprintf("unexpected node type %T", node))
		}
		if tparams.NumFields() == 0 {
			return
		}

		for _, field := range tparams.List {
			typ := pass.TypesInfo.Types[field.Type].Type
			if typ == nil {
				continue // something is wrong, but not our concern
			}
			iface, ok := typ.Underlying().(*types.Interface)
			if !ok {
				continue // invalid constraint
			}

			// If the constraint is the empty interface, offer a fix to use 'any'
			// instead.
			if iface.Empty() {
				id, _ := field.Type.(*ast.Ident)
				if id != nil && pass.TypesInfo.Uses[id] == universeAny {
					continue
				}

				diag := analysis.Diagnostic{
					Pos:     field.Type.Pos(),
					End:     field.Type.End(),
					Message: `could use "any" for this empty interface`,
				}

				// Only suggest a fix to 'any' if we actually resolve the predeclared
				// any in this scope.
				if scope := pass.TypesInfo.Scopes[node]; scope != nil {
					if _, any := scope.LookupParent("any", token.NoPos); any == universeAny {
						diag.SuggestedFixes = []analysis.SuggestedFix{{
							Message: `use "any"`,
							TextEdits: []analysis.TextEdit{{
								Pos:     field.Type.Pos(),
								End:     field.Type.End(),
								NewText: []byte("any"),
							}},
						}}
					}
				}

				pass.Report(diag)
			}
		}
	})
	return nil, nil
}
