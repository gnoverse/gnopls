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

	"github.com/gnoverse/gnopls/internal/packages"
	"golang.org/x/mod/modfile"
)

func gnoPkgToGo(req *packages.DriverRequest, gnomodPath string, logger *slog.Logger) []*packages.Package {
	gnomodBytes, err := os.ReadFile(gnomodPath)
	if err != nil {
		logger.Error("failed to read gno.mod", slog.String("path", gnomodPath), slog.String("err", err.Error()))
		return nil
	}
	gnomodFile, err := modfile.ParseLax(gnomodPath, gnomodBytes, nil)
	if err != nil {
		logger.Error("failed to parse lax gno.mod", slog.String("path", gnomodPath), slog.String("err", err.Error()))
		return nil
	}
	if gnomodFile == nil || gnomodFile.Module == nil {
		logger.Error("gno.mod has no module", slog.String("path", gnomodPath))
		return nil
	}
	dir := filepath.Dir(gnomodPath)

	// TODO: support subpkgs

	pkgPath := gnomodFile.Module.Mod.Path
	return readPkg(req, dir, pkgPath, logger)
}

// listGnomods recursively finds all gnomods at root
func listGnomods(root string) ([]string, error) {
	var gnomods []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		gnoModPath := filepath.Join(path, "gno.mod")
		if _, err := os.Stat(gnoModPath); err != nil {
			return nil
		}
		gnomods = append(gnomods, gnoModPath)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return gnomods, nil
}

func readPkg(req *packages.DriverRequest, dir string, pkgPath string, logger *slog.Logger) []*packages.Package {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error("failed to read pkg dir", slog.String("dir", dir))
		return nil
	}

	pkg := &packages.Package{}
	xTestPkg := &packages.Package{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".gno") {
			continue
		}

		// ignore filetests
		if strings.HasSuffix(filename, "_filetest.gno") {
			continue
		}

		srcPath := filepath.Join(dir, filename)

		var src any
		if body, ok := req.Overlay[srcPath]; ok {
			src = body
		}

		// TODO: refacto this bit
		if strings.HasSuffix(filename, "_test.gno") {
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
