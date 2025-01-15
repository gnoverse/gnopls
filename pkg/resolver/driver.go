package resolver

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
		testLibsRoot := filepath.Join(gnoRoot, "gnovm", "tests", "stdlibs")
		if err := fs.WalkDir(os.DirFS(libsRoot), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			pkgDir := filepath.Join(libsRoot, path)

			pkgs := readPkg(req, pkgDir, path, logger)
			for _, pkg := range pkgs {
				if len(pkg.GoFiles) == 0 {
					continue
				}

				res.Packages = append(res.Packages, pkg)
				if !strings.HasSuffix(pkg.Name, "_test") {
					pkgsCache[path] = pkg
				}

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
	logger.Info("discovered packages", slog.Int("count", len(gnomods)+len(res.Packages)))

	// Convert packages

	for _, gnomodPath := range gnomods {
		pkgs := gnoPkgToGo(req, gnomodPath, logger)
		for _, pkg := range pkgs {
			if pkg == nil {
				logger.Error("failed to convert gno pkg to go pkg", slog.String("gnomod", gnomodPath))
				continue
			}
			if _, ok := pkgsCache[pkg.PkgPath]; ok {
				// ignore duplicates in later targets, mostly useful to ignore examples present in explicit targets
				logger.Debug("ignored duplicate", slog.String("pkg-path", pkg.PkgPath), slog.String("new", gnomodPath))
				continue
			}
			res.Packages = append(res.Packages, pkg)
			res.Roots = append(res.Roots, pkg.ID)
			if !strings.HasSuffix(pkg.Name, "_test") {
				pkgsCache[pkg.PkgPath] = pkg
			}
		}
	}

	// Resolve imports

	for _, pkg := range res.Packages {
		toDelete := []string{}
		for importPath := range pkg.Imports {
			imp, ok := pkgsCache[importPath]
			if ok {
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
