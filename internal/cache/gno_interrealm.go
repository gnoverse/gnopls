// This file handles `gno` interrealm specificity by directly dealing with AST.
// Although it is somewhat hacky, it functions well for the current requirements.

package cache

import (
	"go/ast"
)

// gnoHandleInterRealm replaces and registers gno `crossing` and `cross` calls.
// XXX: make Godoc works
func gnoHandleInterRealm(files []*ast.File) {
	if len(files) == 0 {
		return
	}

	crossingDecl := &ast.FuncDecl{
		Name: ast.NewIdent("crossing"),
		Doc: &ast.CommentGroup{List: []*ast.Comment{
			{Text: "// crossing"}, // XXX: Make Godoc works
		}},
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{},
		},
	}

	crossParams := &ast.FieldList{
		List: []*ast.Field{
			{
				Names: []*ast.Ident{ast.NewIdent("args")},
				Type: &ast.Ellipsis{
					Elt: ast.NewIdent("any"),
				},
			},
		},
	}
	crossResults := &ast.FieldList{
		List: []*ast.Field{
			{
				Type: &ast.ArrayType{
					Elt: ast.NewIdent("any"),
				},
			},
		},
	}

	crossDecl := &ast.FuncDecl{
		Name: ast.NewIdent("cross"),
		Doc: &ast.CommentGroup{List: []*ast.Comment{
			{Text: "// cross"}, // XXX: Make Godoc works
		}},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("crossing")},
						Type:  ast.NewIdent("any"),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.FuncType{
							Params:  crossParams,
							Results: crossResults,
						},
					},
				},
			},
		},
	}

	files[0].Decls = append([]ast.Decl{crossingDecl, crossDecl}, files[0].Decls...)
}

// gnoCleanupCrossCall removes `cross` calls and returns a function to restore them.
func gnoCleanupCrossCall(files []*ast.File) (restore func()) {
	restoreCb := []func(){}
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			// Look for outer call expression
			outerCall, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if Fun is itself a CallExpr
			innerCall, ok := outerCall.Fun.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if innerCall is calling "cross"
			if ident, ok := innerCall.Fun.(*ast.Ident); ok && ident.Name == "cross" {
				// Replace outerCall.Fun with innerCall.Args[0] (i.e., pkg.Func)
				if len(innerCall.Args) == 1 {
					oldFun := outerCall.Fun
					outerCall.Fun = innerCall.Args[0]
					restoreCb = append(restoreCb, func() {
						outerCall.Fun = oldFun
					})
				}
			}

			return true
		})
	}

	return func() {
		for _, cl := range restoreCb {
			cl()
		}
	}
}
