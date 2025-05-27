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
		return filepath.Join(gnopls, "pkg", "gnotypes", "builtin"), nil
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

	case *types.TypeName:
	}

	_, ok := gnoBuiltin[obj.Name()]
	return ok
}

func GnoBuiltin(includeFn bool) []types.Object {
	objs := make([]types.Object, 0, len(gnoBuiltin))

	for _, obj := range gnoBuiltin {
		if !includeFn {
			if _, ok := obj.(*types.Func); ok {
				continue
			}
		}

		objs = append(objs, obj)
	}

	objs = slices.Clip(objs)
	slices.SortFunc(objs, func(a, b types.Object) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return objs
}

func AdditionalGnoPredeclared() []types.Type {
	gnobuiltins := GnoBuiltin(false)
	fmt.Println(gnobuiltins)
	ts := make([]types.Type, len(gnobuiltins))
	for i, obj := range gnobuiltins {
		ts[i] = types.Universe.Lookup(obj.Name()).Type() // register func
	}
	return ts
}

var _excludeTypes = []string{
	"FloatType", "IntegerType", "Error", "Type", "Type1", "ComplexType",
}

type builtinDef map[string]types.Object

func newBuiltinDef(src []byte) []types.Object {
	defs := []types.Object{}

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

		switch obj.(type) {
		case *types.Func, *types.TypeName:
			defs = append(defs, obj)
		default:
		}
	}

	return defs
}

// cloneContext holds the state for a cloning operation, primarily the cloneMap.
type cloneContext struct {
	cloneMap map[types.Type]types.Type
}

func (ctx *cloneContext) CacheType(t types.Type) {
	ctx.cloneMap[t] = t
}

// CloneTypeWithNilPackage creates a deep clone of a types.Type,
// setting the package of any encountered NamedType to nil.
// It also attempts to make associated elements (like struct fields, method signatures)
// consistent with a package-less owner.
func (ctx *cloneContext) CloneTypeWithNilPackage(t types.Type) types.Type {
	if t == nil {
		panic("t cannot be nil")
	}

	if cloned, ok := ctx.cloneMap[t]; ok {
		return cloned.(types.Type)
	}

	var result types.Type

	switch T := t.(type) {
	case *types.Basic:
		result = T // Basic types are canonical and package-less.
		// No need to add to cloneMap as they are singletons and not containers for cycles relevant here.

	case *types.Named:
		origObj := T.Obj()

		// Placeholder strategy for cycle handling:
		// Create a pointer to a Named value. This pointer's value (the Named struct)
		// will be filled in after its components (underlying type, methods) are cloned.

		newTypeName := types.NewTypeName(token.NoPos, nil /* pkg = nil */, origObj.Name(), nil)
		underlying := ctx.CloneTypeWithNilPackage(T.Underlying())
		for {
			if _, ok := underlying.(*types.Named); !ok {
				break
			}
			// namedUnderlying is the *cloned* version of T.Underlying().
			// Its own .Underlying() field should have been set correctly (to a non-Named)
			// by its own NewNamed call during the recursive CloneTypeWithNilPackage.
			underlying = underlying.Underlying()
		}

		var newMethods []*types.Func
		if T.NumMethods() > 0 {
			newMethods = make([]*types.Func, T.NumMethods())
			for i := range T.NumMethods() {
				origMethod := T.Method(i)
				// Clone the signature. Types within the signature will also be cloned.
				clonedSig := ctx.CloneTypeWithNilPackage(origMethod.Type()).(*types.Signature)
				// Create the new Func. types.NewNamed will set its Pkg based on newTypeName.Pkg (which is nil).
				newMethods[i] = types.NewFunc(token.NoPos, nil /* pkg */, origMethod.Name(), clonedSig)
			}
		}

		// Construct the final *types.Named object.
		// This will also correctly set newTypeName.obj and newNamedInstance.obj.
		result = types.NewNamed(newTypeName, underlying, newMethods)
		ctx.cloneMap[T] = result // Map original T to the placeholder

	case *types.Pointer:
		elem := ctx.CloneTypeWithNilPackage(T.Elem())
		newPointerType := types.NewPointer(elem)
		ctx.cloneMap[T] = newPointerType
		result = newPointerType
	case *types.Slice:
		elem := ctx.CloneTypeWithNilPackage(T.Elem())
		newSliceType := types.NewSlice(elem)
		ctx.cloneMap[T] = newSliceType
		result = newSliceType
	case *types.Array:
		elem := ctx.CloneTypeWithNilPackage(T.Elem())
		newArrayType := types.NewArray(elem, T.Len())
		ctx.cloneMap[T] = newArrayType
		result = newArrayType
	case *types.Map:
		key := ctx.CloneTypeWithNilPackage(T.Key())
		elem := ctx.CloneTypeWithNilPackage(T.Elem())
		newMapType := types.NewMap(key, elem)
		ctx.cloneMap[T] = newMapType
		result = newMapType
	case *types.Chan:
		elem := ctx.CloneTypeWithNilPackage(T.Elem())
		newChanType := types.NewChan(T.Dir(), elem)
		ctx.cloneMap[T] = newChanType
		result = newChanType

	case *types.Struct:
		// Placeholder for struct to handle cycles in field types
		newStructType := new(types.Struct)
		ctx.cloneMap[T] = newStructType

		fields := make([]*types.Var, T.NumFields())
		tags := make([]string, T.NumFields())
		for i := range T.NumFields() {
			origField := T.Field(i)
			fieldType := ctx.CloneTypeWithNilPackage(origField.Type())
			// Fields of a package-less struct should also be package-less.
			clonedField := types.NewField(token.NoPos, nil /* pkg */, origField.Name(), fieldType, origField.Embedded())
			fields[i] = clonedField
			tags[i] = T.Tag(i)
		}
		result = types.NewStruct(fields, tags)

	case *types.Tuple:
		// Placeholder for tuple to handle cycles
		newTupleType := new(types.Tuple)
		ctx.cloneMap[T] = newTupleType

		vars := make([]*types.Var, T.Len())
		for i := range T.Len() {
			origVar := T.At(i)
			varType := ctx.CloneTypeWithNilPackage(origVar.Type())
			// Vars (like parameters) in a package-less context should be package-less.
			clonedVar := types.NewVar(token.NoPos, nil /* pkg */, origVar.Name(), varType)
			vars[i] = clonedVar
		}
		result = types.NewTuple(vars...)

	case *types.Signature:
		// Placeholder for signature
		newSigType := new(types.Signature)
		ctx.cloneMap[T] = newSigType

		var newRecv *types.Var
		if T.Recv() != nil {
			origRecvVar := T.Recv()
			recvType := ctx.CloneTypeWithNilPackage(origRecvVar.Type())
			newRecv = types.NewVar(token.NoPos, nil /* pkg */, origRecvVar.Name(), recvType)
		}

		params := ctx.CloneTypeWithNilPackage(T.Params()).(*types.Tuple)
		results := ctx.CloneTypeWithNilPackage(T.Results()).(*types.Tuple)

		var clonedRecvTypeParams []*types.TypeParam
		if rtp := T.RecvTypeParams(); rtp != nil { // T.RecvTypeParams() returns []*TypeParam
			clonedRecvTypeParams = make([]*types.TypeParam, rtp.Len())
			for i := range rtp.Len() {
				tp := rtp.At(i)
				clonedRecvTypeParams[i] = ctx.CloneTypeWithNilPackage(tp).(*types.TypeParam)
			}
		}

		var clonedTypeParams []*types.TypeParam
		if tpSlice := T.TypeParams(); tpSlice != nil { // T.TypeParams() returns []*TypeParam
			clonedTypeParams = make([]*types.TypeParam, tpSlice.Len())
			for i := range tpSlice.Len() {
				tp := tpSlice.At(i)
				clonedTypeParams[i] = ctx.CloneTypeWithNilPackage(tp).(*types.TypeParam)
			}
		}

		result = types.NewSignatureType(
			newRecv, clonedRecvTypeParams, clonedTypeParams, params, results, T.Variadic(),
		)

	case *types.Interface:
		// Placeholder for interface
		newIfaceType := new(types.Interface)
		ctx.cloneMap[T] = newIfaceType

		numExplicitMethods := T.NumExplicitMethods()
		newExplicitMethods := make([]*types.Func, numExplicitMethods)
		for i := range numExplicitMethods {
			origMethod := T.ExplicitMethod(i)
			clonedSig := ctx.CloneTypeWithNilPackage(origMethod.Type()).(*types.Signature)
			// Interface methods are typically Pkg=nil in NewFunc.
			newMethod := types.NewFunc(token.NoPos, nil, origMethod.Name(), clonedSig)
			newExplicitMethods[i] = newMethod
		}

		numEmbeddeds := T.NumEmbeddeds()
		newEmbeddeds := make([]types.Type, numEmbeddeds)
		for i := range numEmbeddeds {
			newEmbeddeds[i] = ctx.CloneTypeWithNilPackage(T.EmbeddedType(i))
		}

		finalInterface := types.NewInterfaceType(newExplicitMethods, newEmbeddeds)
		finalInterface.Complete() // Crucial for interfaces
		result = finalInterface
	case *types.Alias:
		// Alias only support known types
		result = ctx.CloneTypeWithNilPackage(T.Rhs())

	case *types.Union: // Go 1.18+ for type sets in interfaces
		terms := make([]*types.Term, T.Len())
		for i := range T.Len() {
			origTerm := T.Term(i)
			clonedType := ctx.CloneTypeWithNilPackage(origTerm.Type())
			terms[i] = types.NewTerm(origTerm.Tilde(), clonedType)
		}
		result = types.NewUnion(terms)

	case *types.TypeParam: // Go 1.18+ for generics
		origObj := T.Obj()

		// Placeholder strategy for TypeParam
		clonedConstraint := ctx.CloneTypeWithNilPackage(T.Constraint())

		// TypeName for a TypeParam should also be package-less in this context.
		newTypeName := types.NewTypeName(token.NoPos, nil /* pkg = nil */, origObj.Name(), nil)

		result = types.NewTypeParam(newTypeName, clonedConstraint)

	default:
		panic(fmt.Sprintf("CloneTypeWithNilPackage: unhandled type %T (value: %v)\n", t, t))
	}

	ctx.cloneMap[t] = result
	return result
}

//go:embed builtin/builtin.gno
var builtinFile []byte

func init() {
	gnoBuiltin = map[string]types.Object{}
	// Build gno builtin defs
	builtins := newBuiltinDef(builtinFile)
	ctx := &cloneContext{
		cloneMap: map[types.Type]types.Type{},
	}

	// Cache basic types
	// gnoObjs := []types.Object{}
	for _, obj := range builtins {
		// Skip manual excluded types
		if slices.Contains(_excludeTypes, obj.Name()) {
			continue
		}

		if o := types.Universe.Lookup(obj.Name()); o != nil {
			ctx.cloneMap[obj.Type()] = o.Type()
			continue
		}

		if !isMethod(obj) {
			gnoBuiltin[obj.Name()] = obj
		}
	}

	for name, obj := range gnoBuiltin {
		switch o := obj.(type) {
		// XXX: handle types.TypeName
		case *types.Func:
			sig := o.Type().(*types.Signature)
			newFn := types.NewFunc(token.NoPos, nil, name, sig) // a builtin don't have a pos
			types.Universe.Insert(newFn)                        // register func
			log.Printf("builtin func %q has been registered", o.Name())
		case *types.TypeName:
			// We need to deep copy type to create a pkgless type to
			// be consider as builtin for the checker
			t := ctx.CloneTypeWithNilPackage(o.Type())
			nameobj := types.NewTypeName(token.NoPos, nil, o.Name(), t) // a builtin don't have a pos
			types.Universe.Insert(nameobj)                              // register type
			log.Printf("builtin type %q has been registered", o.Name())
		}

	}
}

func isMethod(obj types.Object) bool {
	// Check if the object is a function
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	// Check if the function has a receiver (i.e., is a method)
	return fn.Type().(*types.Signature).Recv() != nil
}
