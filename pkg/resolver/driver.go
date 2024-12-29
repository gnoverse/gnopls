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

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnoverse/gnopls/internal/packages"
	"github.com/gnoverse/gnopls/pkg/eventlogger"
)

func Resolve(req *packages.DriverRequest, patterns ...string) (*packages.DriverResponse, error) {
	logger := eventlogger.EventLoggerWrapper()

	logger.Info("unmarshalled request",
		"mode", req.Mode.String(),
		"tests", req.Tests,
		"build-flags", req.BuildFlags,
		"overlay", req.Overlay,
	)

	// Inject examples

	gnoRoot, err := gnoenv.GuessRootDir()
	if err != nil {
		logger.Warn("can't find gno root, examples and std packages are ignored", slog.String("error", err.Error()))
	}

	targets := patterns

	if gnoRoot != "" {
		targets = append(targets, filepath.Join(gnoRoot, "examples", "..."))
	}

	pkgsCache := map[string]*packages.Package{}
	res := packages.DriverResponse{}

	// Inject stdlibs

	if gnoRoot != "" {
		libsRoot := filepath.Join(gnoRoot, "gnovm", "stdlibs")
		if err := fs.WalkDir(os.DirFS(libsRoot), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			pkgDir := filepath.Join(libsRoot, path)

			pkg := &packages.Package{}
			readPkg(pkg, pkgDir, path, logger)
			if pkg == nil {
				return nil
			}

			pkgsCache[path] = pkg
			res.Packages = append(res.Packages, pkg)

			logger.Info("injected stdlib", slog.String("path", pkg.PkgPath), slog.String("name", pkg.Name))

			return nil
		}); err != nil {
			logger.Warn("failed to inject all stdlibs", slog.String("error", err.Error()))
		}
	}

	// Discover packages

	gnomods := []string{}
	for _, target := range targets {
		dir, file := filepath.Split(target)
		if file == "..." {
			gnomodsRes, err := listGnomods(dir)
			if err != nil {
				logger.Error("failed to get pkg list", slog.String("error", err.Error()))
				return nil, err
			}
			gnomods = append(gnomods, gnomodsRes...)
		} else if strings.HasPrefix(target, "file=") {
			dir = strings.TrimPrefix(dir, "file=")
			gnomodsRes, err := listGnomods(dir)
			if err != nil {
				logger.Error("failed to get pkg", slog.String("error", err.Error()))
				return nil, err
			}
			if len(gnomodsRes) != 1 {
				logger.Warn("unexpected number of packages",
					slog.String("arg", target),
					slog.Int("count", len(gnomodsRes)),
				)
			}
			gnomods = append(gnomods, gnomodsRes...)
		} else {
			logger.Warn("unknown arg shape", slog.String("value", target))
		}
	}
	logger.Info("discovered packages", slog.Int("count", len(gnomods)))

	// Convert packages

	for _, gnomodPath := range gnomods {
		pkg := gnoPkgToGo(gnomodPath, logger)
		if pkg == nil {
			logger.Error("failed to convert gno pkg to go pkg", slog.String("gnomod", gnomodPath))
			continue
		}
		if _, ok := pkgsCache[pkg.PkgPath]; ok {
			// ignore duplicates in later targets, mostly useful to ignore examples present in explicit targets
			logger.Debug("ignored duplicate", slog.String("pkg-path", pkg.PkgPath), slog.String("new", gnomodPath))
			continue
		}
		pkgsCache[pkg.PkgPath] = pkg
		res.Packages = append(res.Packages, pkg)
		res.Roots = append(res.Roots, pkg.ID)
	}

	// Resolve imports

	for _, pkg := range res.Packages {
		toDelete := []string{}
		for importPath := range pkg.Imports {
			imp, ok := pkgsCache[importPath]
			if ok {
				pkg.Imports[importPath] = imp
				logger.Info("found import", slog.String("path", importPath))
			} else {
				logger.Info("missed import", slog.String("path", importPath))
				toDelete = append(toDelete, importPath)
			}
		}
		for _, toDel := range toDelete {
			delete(pkg.Imports, toDel)
		}
		logger.Info("converted package", slog.Any("pkg", pkg))
	}

	return &res, nil
}

func readPkg(pkg *packages.Package, dir string, pkgPath string, logger *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error("failed to read pkg dir", slog.String("dir", dir))
		return
	}

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

		file := filepath.Join(dir, filename)
		pkg.GoFiles = append(pkg.GoFiles, file)
		pkg.CompiledGoFiles = append(pkg.CompiledGoFiles, file)
	}

	if len(pkg.GoFiles) == 0 {
		return
	}

	pkg.ID = pkgPath
	pkg.PkgPath = pkgPath

	resolveNameAndImports(pkg, logger)
}

func resolveNameAndImports(pkg *packages.Package, logger *slog.Logger) {
	names := map[string]int{}
	imports := map[string]*packages.Package{}
	bestName := ""
	bestNameCount := 0
	for _, srcPath := range pkg.CompiledGoFiles {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, srcPath, nil, parser.SkipObjectResolution|parser.ImportsOnly)
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

		name := f.Name.String()
		if !strings.HasSuffix(name, "_test") {
			names[name] += 1
			count := names[name]
			if count > bestNameCount {
				bestName = name
				bestNameCount = count
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

	pkg.Name = bestName
	pkg.Imports = imports

	logger.Info("analyzed sources", slog.String("path", pkg.PkgPath), slog.String("name", bestName), slog.Any("imports", imports), slog.Any("errs", pkg.Errors), slog.Any("compfiles", pkg.CompiledGoFiles))
}
