package resolver

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	gnopackages "github.com/gnolang/gno/gnovm/pkg/packages"
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
		"patterns", patterns,
	)

	requireBuiltin := false
	loaderPatterns := []string{}
	for _, pattern := range patterns {
		// XXX: better support file pattern
		if strings.HasPrefix(pattern, "file=") {
			dir, _ := filepath.Split(pattern)
			dir = strings.TrimPrefix(dir, "file=")
			gnomodsRes, err := listPackagesPath(dir)
			if err != nil {
				logger.Error("failed to get pkg", slog.String("error", err.Error()))
				return nil, err
			}
			if len(gnomodsRes) != 1 {
				logger.Warn("unexpected number of packages",
					slog.String("arg", pattern),
					slog.Int("count", len(gnomodsRes)),
				)
			}
			loaderPatterns = append(loaderPatterns, gnomodsRes...)
			continue
		} else if pattern == "builtin" {
			requireBuiltin = true
			continue
		}

		loaderPatterns = append(loaderPatterns, pattern)
	}

	res := packages.DriverResponse{}

	findGoPkg := func(pkgpath string) *packages.Package {
		for _, pkg := range res.Packages {
			if pkg != nil && pkg.PkgPath == pkgpath {
				return pkg
			}
		}
		return nil
	}

	gnoRoot, err := gnoenv.GuessRootDir()
	if err != nil {
		logger.Warn("can't find gno root, examples and std packages are ignored", slog.String("error", err.Error()))
	}

	// Inject stdlibs
	if gnoRoot != "" {
		libsRoot := filepath.Join(gnoRoot, "gnovm", "stdlibs")
		testLibsRoot := filepath.Join(gnoRoot, "gnovm", "tests", "stdlibs")
		if err := fs.WalkDir(os.DirFS(libsRoot), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			if path == "." {
				return nil
			}

			pkgDir := filepath.Join(libsRoot, path)

			pkgs := readPkg(req, pkgDir, path, logger)
			for _, pkg := range pkgs {
				if len(pkg.GoFiles) == 0 {
					continue
				}

				res.Packages = append(res.Packages, pkg)

				testLibDir := filepath.Join(testLibsRoot, path)
				testsDir, err := os.ReadDir(testLibDir)
				if err != nil {
					continue
				}
				for _, entry := range testsDir {
					if entry.IsDir() {
						continue
					}

					filename := entry.Name()
					isDot := len(filename) > 0 && filename[0] == '.'
					if isDot || !strings.HasSuffix(filename, ".gno") {
						continue
					}

					deleteFn := func(src string) bool {
						return filepath.Base(src) == filename
					}
					pkg.GoFiles = slices.DeleteFunc(pkg.GoFiles, deleteFn)
					pkg.CompiledGoFiles = slices.DeleteFunc(pkg.CompiledGoFiles, deleteFn)

					file := filepath.Join(testLibDir, filename)
					pkg.GoFiles = append(pkg.GoFiles, file)
					pkg.CompiledGoFiles = append(pkg.CompiledGoFiles, file)
				}
			}

			// logger.Info("injected stdlib", slog.String("path", pkg.PkgPath), slog.String("name", pkg.Name))

			return nil
		}); err != nil {
			logger.Warn("failed to inject all stdlibs", slog.String("error", err.Error()))
		}

		// inject tests libs
		if err := fs.WalkDir(os.DirFS(testLibsRoot), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			if path == "." {
				return nil
			}

			if slices.ContainsFunc(res.Packages, func(p *packages.Package) bool { return p.PkgPath == path }) {
				return nil
			}

			pkgDir := filepath.Join(testLibsRoot, path)

			pkgs := readPkg(req, pkgDir, path, logger)
			for _, pkg := range pkgs {
				if len(pkg.GoFiles) == 0 {
					continue
				}

				res.Packages = append(res.Packages, pkg)

			}

			// logger.Info("injected stdlib", slog.String("path", pkg.PkgPath), slog.String("name", pkg.Name))

			return nil
		}); err != nil {
			logger.Warn("failed to inject all tests stdlibs", slog.String("error", err.Error()))
		}
	}

	if requireBuiltin {
		// Inject gnobuiltin
		if pkg, err := getBuiltinPkg(); err == nil {
			res.Packages = append(res.Packages, pkg)
			res.Roots = append(res.Roots, pkg.ID)
		}
	}

	loadCfg := gnopackages.LoadConfig{
		Test:    req.Tests,
		Out:     os.Stderr,
		Deps:    true,
		GnoRoot: gnoRoot,
		// Overlay: req.Overlay,
	}
	loadedPkgs, err := gnopackages.Load(loadCfg, loaderPatterns...)
	if err != nil {
		return nil, err
	}

	// Convert packages

	for _, gnopkg := range loadedPkgs {
		if gnolang.IsStdlib(gnopkg.ImportPath) {
			continue
		}
		pkgs := gnoPkgToGo(req, gnopkg, logger)
		res.Packages = append(res.Packages, pkgs...)
		if len(gnopkg.Match) != 0 {
			for _, pkg := range pkgs {
				res.Roots = append(res.Roots, pkg.ID)
			}
		}
	}

	logger.Info("discovered packages", slog.Int("count", len(res.Packages)))

	// Resolve imports

	for _, pkg := range res.Packages {
		if pkg.PkgPath == "builtin" {
			continue
		}

		toDelete := []string{}
		for importPath := range pkg.Imports {
			imp := findGoPkg(importPath)
			if imp != nil {
				pkg.Imports[importPath] = imp
				// logger.Info("found import", slog.String("path", importPath))
				continue
			}

			logger.Info("missed import", slog.String("pkg-path", pkg.PkgPath), slog.String("import", importPath))
			toDelete = append(toDelete, importPath)

		}
		for _, toDel := range toDelete {
			delete(pkg.Imports, toDel)
		}
		// logger.Info("converted package", slog.Any("pkg", pkg))
	}

	return &res, nil
}
