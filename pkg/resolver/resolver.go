package resolver

import (
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/gnovm/pkg/gnomod"
	"github.com/gnoverse/gnopls/internal/packages"
	"github.com/gnoverse/gnopls/pkg/gnotypes"
)

func gnoPkgToGo(req *packages.DriverRequest, path string, logger *slog.Logger) []*packages.Package {
	mod, err := gnomod.ParseDir(path)
	if err != nil {
		logger.Error("failed to read mod file", "path", path, "err", err)
		return nil
	}

	// TODO: support subpkgs
	module := mod.Module
	return readPkg(req, path, module, logger)
}

// listPackagesPath recursively finds all gnomods at root
func listPackagesPath(root string) ([]string, error) {
	var gnomods []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		for _, fname := range []string{"gnomod.toml", "gno.mod"} {
			fpath := filepath.Join(path, fname)
			if _, err := os.Stat(fpath); err != nil {
				continue
			}

			gnomods = append(gnomods, path)
			break
		}
		return nil
	})
	return gnomods, err
}

func getBuiltinPkg() (*packages.Package, error) {
	const builtinPath = "builtin"

	builtindir, err := gnotypes.GuessBuiltinDir()
	if err != nil {
		return nil, fmt.Errorf("unable to guess builtin dir: %w", err)
	}

	var pkg packages.Package
	pkg.GoFiles = []string{filepath.Join(builtindir, "builtin.gno")}
	pkg.PkgPath = builtinPath
	pkg.ID = builtinPath
	pkg.Name = builtinPath

	return &pkg, nil
}

func readPkg(req *packages.DriverRequest, dir string, pkgPath string, logger *slog.Logger) []*packages.Package {
	mempkg, err := gnolang.ReadMemPackage(dir, pkgPath, gnolang.MPAnyAll)
	if err != nil {
		logger.Error("unable to parse mempkg", "dir", dir, "module", pkgPath, "err", err)
		return nil
	}

	pkg := &packages.Package{}
	xTestPkg := &packages.Package{}

	for _, file := range mempkg.Files {
		if !strings.HasSuffix(file.Name, ".gno") {
			continue
		}

		// ignore filetests
		if strings.HasSuffix(file.Name, "_filetest.gno") {
			continue
		}

		srcPath := filepath.Join(dir, file.Name)

		src := []byte(file.Body)
		if body, ok := req.Overlay[srcPath]; ok {
			src = body
		}

		// TODO: refacto this bit
		if strings.HasSuffix(file.Name, "_test.gno") {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, srcPath, src, parser.PackageClauseOnly)
			if err != nil {
				if errList, ok := err.(scanner.ErrorList); ok {
					for _, err := range errList {
						pkg.Errors = append(pkg.Errors, packages.Error{
							Pos:  err.Pos.String(),
							Msg:  err.Msg,
							Kind: packages.ParseError,
						})
					}
				} else {
					pkg.Errors = append(pkg.Errors, packages.Error{
						Pos:  fmt.Sprintf("%s:1", srcPath),
						Msg:  err.Error(),
						Kind: packages.ParseError,
					})
				}
			}
			if parsed != nil {
				if strings.HasSuffix(parsed.Name.String(), "_test") {
					xTestPkg.GoFiles = append(xTestPkg.GoFiles, srcPath)
					xTestPkg.CompiledGoFiles = append(xTestPkg.CompiledGoFiles, srcPath)
					continue
				}
			}
		}

		pkg.GoFiles = append(pkg.GoFiles, srcPath)
		pkg.CompiledGoFiles = append(pkg.CompiledGoFiles, srcPath)
	}

	pkg.ID = pkgPath
	pkg.PkgPath = pkgPath
	resolveNameAndImports(req, pkg, logger)

	xTestPkg.ID = pkgPath + "_test"
	xTestPkg.PkgPath = pkgPath + "_test"
	xTestPkg.Name = pkg.Name + "_test"
	resolveNameAndImports(req, xTestPkg, logger)

	return []*packages.Package{pkg, xTestPkg}
}

func resolveNameAndImports(req *packages.DriverRequest, pkg *packages.Package, logger *slog.Logger) {
	names := map[string]int{}
	imports := map[string]*packages.Package{}
	bestName := ""
	bestNameCount := 0

	for _, srcPath := range pkg.CompiledGoFiles {
		fset := token.NewFileSet()

		var src any
		if body, ok := req.Overlay[srcPath]; ok {
			src = body
		}

		f, err := parser.ParseFile(fset, srcPath, src, parser.SkipObjectResolution|parser.ImportsOnly)
		if err != nil {
			if errList, ok := err.(scanner.ErrorList); ok {
				for _, err := range errList {
					pkg.Errors = append(pkg.Errors, packages.Error{
						Pos:  err.Pos.String(),
						Msg:  err.Msg,
						Kind: packages.ParseError,
					})
				}
			} else {
				pkg.Errors = append(pkg.Errors, packages.Error{
					Pos:  fmt.Sprintf("%s:1", srcPath),
					Msg:  err.Error(),
					Kind: packages.ParseError,
				})
			}
		}

		if f == nil {
			continue
		}

		if pkg.Name == "" {
			name := f.Name.String()
			if !strings.HasSuffix(name, "_test") {
				names[name] += 1
				count := names[name]
				if count > bestNameCount {
					bestName = name
					bestNameCount = count
				}
			}
		}

		for _, imp := range f.Imports {
			importPath := imp.Path.Value
			if len(importPath) >= 2 {
				importPath = importPath[1 : len(importPath)-1]
			}
			imports[importPath] = nil
		}
	}

	if pkg.Name == "" {
		pkg.Name = bestName
	}

	pkg.Imports = imports

	// logger.Info("analyzed sources", slog.String("path", pkg.PkgPath), slog.String("name", bestName), slog.Any("imports", imports), slog.Any("errs", pkg.Errors), slog.Any("compfiles", pkg.CompiledGoFiles))
}
