package gnopkg

import (
	"go/types"
	"io/fs"
	"path/filepath"

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnoverse/gnopls/internal/packages"
)

type GnoPackage struct {
	packages.Package
}

func loadbuiltin() []packages.Package {
	rootDir := gnoenv.RootDir()
	stdlibsDir := filepath.Join(rootDir, "gnovm", "stdlibs")
	filepath.WalkDir(stdlibsDir, func(path string, d fs.DirEntry, err error) error {

		return nil
	})
	return []packages.Package{}
}

func LoadGnoPackages(query string) {
	rootdir := gnoenv.RootDir()
	res := packages.DriverResponse{}

	gopkgs := make([]*packages.Package, len(pkgs))
	for i, pkg := range pkgs {
		id := filepath.Join(pkg.Dir, pkg.Name)
		gopkgs[i] = &packages.Package{
			// ID is a unique identifier for a package,
			// in a syntax provided by the underlying build system.
			//
			// Because the syntax varies based on the build system,
			// clients should treat IDs as opaque and not attempt to
			// interpret them.
			ID: id,

			// Name is the package name as it appears in the package source code.
			Name: pkg.Name,

			// PkgPath is the package path as used by the go/types package.
			PkgPath: "",

			// Errors contains any errors encountered querying the metadata
			// of the package, or while parsing or type-checking its files.
			Errors: []packages.Error{},

			// TypeErrors contains the subset of errors produced during type checking.
			TypeErrors: []types.Error{},

			// GoFiles lists the absolute file paths of the package's Go source files.
			// It may include files that should not be compiled, for example because
			// they contain non-matching build tags, are documentary pseudo-files such as
			// unsafe/unsafe.go or builtin/builtin.go, or are subject to cgo preprocessing.
			GoFiles: []string{},

			// CompiledGoFiles lists the absolute file paths of the package's source
			// files that are suitable for type checking.
			// This may differ from GoFiles if files are processed before compilation.
			CompiledGoFiles: []string{},

			// OtherFiles lists the absolute file paths of the package's non-Go source files,
			// including assembly, C, C++, Fortran, Objective-C, SWIG, and so on.
			OtherFiles: []string{},

			// EmbedFileqs lists the absolute file paths of the package's files
			// embedded with go:embed.
			EmbedFiles: []string{},

			// EmbedPatterns lists the absolute file patterns of the package's
			// files embedded with go:embed.
			EmbedPatterns: []string{},

			// IgnoredFiles lists source files that are not part of the package
			// using the current build configuration but that might be part of
			// the package using other build configurations.
			IgnoredFiles: []string{},

			// ExportFile is the absolute path to a file containing type
			// information for the package as provided by the build system.
			ExportFile: "",

			// Imports maps import paths appearing in the package's Go source files
			// to corresponding loaded Packages.
			Imports: make(map[string]*packages.Package),

			// Module is the module information for the package if it exists.
			//
			// Note: it may be missing for std and cmd; see Go issue #65816.
			Module: &packages.Module{},
		}
	}
}
