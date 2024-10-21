package resolver

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/gnovm/pkg/gnomod"
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
			entries, err := os.ReadDir(pkgDir)
			if err != nil {
				return fmt.Errorf("failed to read dir %q: %w", path, err)
			}

			gnoFiles := []string{}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".gno") {
					continue
				}
				if strings.HasSuffix(e.Name(), "_test.gno") || strings.HasSuffix(e.Name(), "_filetest.gno") {
					continue
				}
				gnoFiles = append(gnoFiles, filepath.Join(pkgDir, e.Name()))
			}

			if len(gnoFiles) == 0 {
				return nil
			}

			name, imports, err := resolveNameAndImports(gnoFiles, logger)
			if err != nil {
				return fmt.Errorf("failed to resolve name and imports for %q: %w", path, err)
			}

			logger.Info("injecting stdlib", slog.String("path", path), slog.String("name", name))

			pkg := &packages.Package{
				ID:              path,
				Name:            name,
				PkgPath:         path,
				Imports:         imports,
				GoFiles:         gnoFiles,
				CompiledGoFiles: gnoFiles,
			}
			pkgsCache[path] = pkg
			res.Packages = append(res.Packages, pkg)

			return nil
		}); err != nil {
			logger.Warn("failed to inject all stdlibs", slog.String("error", err.Error()))
		}
	}

	// Discover packages

	pkgs := gnomod.PkgList{}
	for _, target := range targets {
		dir, file := filepath.Split(target)
		if file == "..." {
			pkgQueryRes, err := ListPkgs(dir)
			if err != nil {
				logger.Error("failed to get pkg list", slog.String("error", err.Error()))
				return nil, err
			}
			pkgs = append(pkgs, pkgQueryRes...)
		} else if strings.HasPrefix(target, "file=") {
			dir = strings.TrimPrefix(dir, "file=")
			pkgQueryRes, err := ListPkgs(dir)
			if err != nil {
				logger.Error("failed to get pkg", slog.String("error", err.Error()))
				return nil, err
			}
			if len(pkgQueryRes) != 1 {
				logger.Warn("unexpected number of packages",
					slog.String("arg", target),
					slog.Int("count", len(pkgQueryRes)),
				)
			}
			pkgs = append(pkgs, pkgQueryRes...)
		} else {
			logger.Warn("unknown arg shape", slog.String("value", target))
		}
	}
	logger.Info("discovered packages", slog.Int("count", len(pkgs)))

	// Convert packages

	for _, pkg := range pkgs {
		pkg, err := gnoPkgToGo(&pkg, logger)
		if err != nil {
			logger.Error("failed to convert gno pkg to go pkg", slog.String("error", err.Error()))
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
