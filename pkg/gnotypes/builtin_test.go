package gnotypes_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/gnoverse/gnopls/internal/gcimporter"
	"golang.org/x/tools/go/gcexportdata"
)

func TestGnoBuiltin(t *testing.T) {
	fset := token.NewFileSet()
	export := make(map[string][]byte)

	// process parses and type-checks a single-file
	// package and saves its export data.
	process := func(path, content string) {
		syntax, err := parser.ParseFile(fset, path+"/x.gno", content, 0)
		if err != nil {
			t.Fatal(err)
		}
		packages := make(map[string]*types.Package) // keys are package paths
		cfg := &types.Config{
			Importer: importerFunc(func(path string) (*types.Package, error) {
				data, ok := export[path]
				if !ok {
					return nil, fmt.Errorf("missing export data for %s", path)
				}
				fmt.Println(path)
				return gcexportdata.Read(bytes.NewReader(data), fset, packages, path)
			}),
		}
		pkg := types.NewPackage(path, syntax.Name.Name)
		check := types.NewChecker(cfg, fset, pkg, nil)

		if err := check.Files([]*ast.File{syntax}); err != nil {
			t.Fatal(err)
		}
	}
	const pkgName = "mypkg"
	t.Run("cross func decl", func(t *testing.T) {
		content := fmt.Sprintf(`package %s;
func init() {
    crossing()
    cross(crossing)()
}`, pkgName)
		process(pkgName, content)
	})

	t.Run("type decl", func(t *testing.T) {
		cases := []string{"address", "gnocoin", "gnocoins", "realm" /* XXX: add more cases here */}

		for _, tc := range cases {
			content := fmt.Sprintf(`package %s; type subtyp %s`, pkgName, tc)
			process(pkgName, content)
		}
	})
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

func TestGnoBuiltinExporter(t *testing.T) {
	const src = `package deep

type MyRealm realm

func MyInterface interface {
    GetCoin() gnocoin
}

`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if f == nil {
		// Some test cases may have parse errors, but we must always have a
		// file.
		t.Fatalf("ParseFile returned nil file. Err: %v", err)
	}

	config := &types.Config{}
	pkg1, err := config.Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Export it. (Shallowness isn't important here.)
	_, err = gcimporter.IExportShallow(fset, pkg1, nil)
	if err != nil {
		t.Fatalf("export: %v", err) // any failure to export is a bug
	}

}

func TestGnoBuiltinExporterVar(t *testing.T) {
	const src = `package deep

func CrossingFunc(cur realm) {
    _ = cur
}

func init() {
    CrossingFunc(cross)
}

`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if f == nil {
		// Some test cases may have parse errors, but we must always have a
		// file.
		t.Fatalf("ParseFile returned nil file. Err: %v", err)
	}

	config := &types.Config{}
	pkg1, err := config.Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Export it. (Shallowness isn't important here.)
	_, err = gcimporter.IExportShallow(fset, pkg1, nil)
	if err != nil {
		t.Fatalf("export: %v", err) // any failure to export is a bug
	}

}
