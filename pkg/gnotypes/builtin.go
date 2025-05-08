package gnotypes

import (
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

var ErrUnableToGuessGnoBuiltin = errors.New("gno was unable to determine GNOBUILTIN. Please set the GNOBUILTIN environment variable")

// Can be set manually at build time using:
// -ldflags="-X github.com/gnolang/gnoverse/gnopls/pkg/gnotypes._GNOBUILTIN"
var _GNOBUILTIN string

// BuiltinDir guesses the Gno builtin directory and panics if it fails.
func BuiltinDir() string {
	builtin, err := GuessBuiltinDir()
	if err != nil {
		panic(err)
	}

	return builtin
}

var muGnoBuiltin sync.Mutex

// GuessBuiltinDir attempts to determine the Gno builtin directory using various strategies:
// 1. First, It tries to obtain it from the `GNOBUILTIN` environment variable.
// 2. If the env variable isn't set, It checks if `_GNOBUILTIN` has been previously determined or set with -ldflags.
// 3. If not, it uses the `go list` command to infer from go.mod.
// 4. As a last resort, it determines `GNOBUILTIN` based on the caller stack's file path.
func GuessBuiltinDir() (string, error) {
	muGnoBuiltin.Lock()
	defer muGnoBuiltin.Unlock()

	// First try to get the builtin directory from the `GNOBUILTIN` environment variable.
	if builtindir := os.Getenv("GNOBUILTIN"); builtindir != "" {
		return strings.TrimSpace(builtindir), nil
	}

	var err error
	if _GNOBUILTIN == "" {
		// Try to guess `GNOBUILTIN` using various strategies
		_GNOBUILTIN, err = guessBuiltinDir()
	}

	return _GNOBUILTIN, err
}

func guessBuiltinDir() (string, error) {
	// Attempt to guess `GNOBUILTIN` from go.mod by using the `go list` command.
	if gnopls, err := inferBuiltinFromGoMod(); err == nil {
		return filepath.Join(gnopls, "pkg", "resolver", "builtin"), nil
	}

	// If the above method fails, ultimately try to determine `GNOBUILTIN` based
	// on the caller stack's file path.
	// Path need to be absolute here, that mostly mean that if `-trimpath`
	// as been passed this method will not works.
	if _, filename, _, ok := runtime.Caller(1); ok && filepath.IsAbs(filename) {
		if currentDir := filepath.Dir(filename); currentDir != "" {
			// Deduce Gno builtin directory relative from the current file's path.
			builtindir, err := filepath.Abs(filepath.Join(currentDir, "builtin"))
			if err == nil {
				return builtindir, nil
			}
		}
	}

	return "", ErrUnableToGuessGnoBuiltin
}

func inferBuiltinFromGoMod() (string, error) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("unable to find `go` binary: %w", err)
	}

	cmd := exec.Command(gobin, "list", "-m", "-mod=mod", "-f", "{{.Dir}}", "github.com/gnoverse/gnopls")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to infer GnoBuiltin from go.mod: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

var gnoBuiltin builtinDef

// `IsGnoBuiltin` try to guess if the given object is a gno builtin.
// XXX: Since we cannot define true Go built-in types, we have to infer them in
// other ways
func IsGnoBuiltin(obj types.Object) bool {
	if obj.Exported() {
		return false
	}

	if types.Universe.Lookup(obj.Name()) == nil {
		return false
	}

	switch obj.(type) {
	case *types.Func:
		// lookup for function only
		if obj.Type().(*types.Signature).Recv() != nil {
			return false // method
		}

		_, ok := gnoBuiltin[obj.Name()]
		return ok
	}

	return false
}

var _excludeTypes = []string{
	"FloatType", "IntegerType", "Error", "Type", "Type1", "ComplexType",
}

type builtinDef map[string]types.Object

func newBuiltinDef(src []byte) builtinDef {
	defs := map[string]types.Object{}

	// Parse the file into an *ast.File
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "builtin.go", src, parser.AllErrors)
	if err != nil {
		panic(err)
	}

	// Give every bodyless declaration an empty body, to avoid parsing error
	// where generic func signature is not considered as valid
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body == nil {
			fn.Body = &ast.BlockStmt{Lbrace: fn.Type.End(), Rbrace: fn.Type.End()}
		}
	}

	// Prepare type-checking configuration
	conf := types.Config{
		Importer: importer.Default(), // Import standard packages if needed
	}
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object)}

	// Type-check the package
	if _, err = conf.Check("builtin", fset, []*ast.File{f}, info); err != nil {
		log.Printf("err: %s", err.Error())
	}

	for ident, obj := range info.Defs {
		if obj == nil || ident.Name == "_" {
			continue
		}

		// Skip type parameters
		if tn, ok := obj.(*types.TypeName); ok {
			if _, isTypeParam := tn.Type().(*types.TypeParam); isTypeParam {
				continue
			}
		}

		// Skip manual excluded types
		if slices.Contains(_excludeTypes, obj.Name()) {
			continue
		}

		switch obj.(type) {
		case *types.Func, *types.TypeName:
			// Chekc if this object have been already registered by go
			if types.Universe.Lookup(obj.Name()) != nil {
				continue
			}

			defs[obj.Name()] = obj
		default:
		}
	}

	return defs
}

//go:embed builtin/builtin.gno
var builtinFile []byte

func init() {
	// Build gno builtin defs
	gnoBuiltin = newBuiltinDef(builtinFile)

	// Register them
	for name, obj := range gnoBuiltin {
		switch o := obj.(type) {
		// XXX: handle types.TypeName
		case *types.Func:
			sig := o.Type().(*types.Signature)
			newFn := types.NewFunc(token.NoPos, nil, name, sig)
			types.Universe.Insert(newFn) // register func
		}
	}
}
