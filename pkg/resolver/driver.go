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

	targets := append([]string{}, patterns...)
	if workspaceRoot := discoverWorkspaceRoot(patterns); workspaceRoot != "" {
		targets = append(targets, filepath.Join(workspaceRoot, "..."))
	}

	if gnoRoot != "" {
		targets = append(targets, filepath.Join(gnoRoot, "examples", "..."))
	}

	pkgsCache := map[string]*packages.Package{}
	res := packages.DriverResponse{}

	// Inject gnobuiltin
	if pkg, err := getBuiltinPkg(); err == nil {
		pkgsCache[pkg.Name] = pkg
		res.Packages = append(res.Packages, pkg)
		res.Roots = append(res.Roots, pkg.ID)
	}

	// Inject stdlibs
	if gnoRoot != "" {
		libsRoot := filepath.Join(gnoRoot, "gnovm", "stdlibs")
		testLibsRoot := filepath.Join(gnoRoot, "gnovm", "tests", "stdlibs")

		// Track which packages we've already processed from libsRoot
		processedPkgs := make(map[string]bool)

		if err := fs.WalkDir(os.DirFS(libsRoot), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			// Skip the root directory itself, as "." is not a valid package path
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
				if !strings.HasSuffix(pkg.Name, "_test") {
					pkgsCache[path] = pkg
				}
				processedPkgs[path] = true

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

		// Inject test-only stdlibs (packages that exist only in tests/stdlibs)
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

			// Skip if we already processed this package from libsRoot
			if processedPkgs[path] {
				return nil
			}

			pkgDir := filepath.Join(testLibsRoot, path)

			pkgs := readPkg(req, pkgDir, path, logger)
			for _, pkg := range pkgs {
				if len(pkg.GoFiles) == 0 {
					continue
				}

				res.Packages = append(res.Packages, pkg)
				if !strings.HasSuffix(pkg.Name, "_test") {
					pkgsCache[path] = pkg
				}
			}

			// logger.Info("injected test-only stdlib", slog.String("path", path))

			return nil
		}); err != nil {
			logger.Warn("failed to inject test-only stdlibs", slog.String("error", err.Error()))
		}
	}

	// Discover packages

	pkgpaths := []string{}
	for _, target := range targets {
		dir, file := filepath.Split(target)
		if file == "..." {
			gnomodsRes, err := listPackagesPath(dir)
			if err != nil {
				logger.Error(
					"failed to get pkg list",
					slog.String("target", target),
					slog.String("file", file),
					slog.String("dir", dir),
					slog.String("error", err.Error()),
				)
				return nil, err
			}
			pkgpaths = append(pkgpaths, gnomodsRes...)
		} else if strings.HasPrefix(target, "file=") {
			dir = strings.TrimPrefix(dir, "file=")
			gnomodsRes, err := listPackagesPath(dir)
			if err != nil {
				logger.Error(
					"failed to get pkg",
					slog.String("target", target),
					slog.String("file", file),
					slog.String("dir", dir),
					slog.String("error", err.Error()),
				)
				return nil, err
			}
			if len(gnomodsRes) != 1 {
				logger.Warn("unexpected number of packages",
					slog.String("arg", target),
					slog.Int("count", len(gnomodsRes)),
				)
			}
			pkgpaths = append(pkgpaths, gnomodsRes...)
		} else {
			logger.Warn("unknown arg shape", slog.String("value", target))
		}
	}
	logger.Info("discovered packages", slog.Int("count", len(pkgpaths)+len(res.Packages)))

	// Convert packages

	for _, gnomodPath := range pkgpaths {
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
		if pkg.PkgPath == "builtin" {
			continue
		}

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

func discoverWorkspaceRoot(patterns []string) string {
	seedDir := workspaceSeedDir(patterns)
	if seedDir == "" {
		return ""
	}

	return findWorkspaceRoot(seedDir)
}

func workspaceSeedDir(patterns []string) string {
	for _, pattern := range patterns {
		if path, ok := strings.CutPrefix(pattern, "file="); ok {
			return filepath.Dir(path)
		}

		if base, ok := strings.CutSuffix(pattern, "..."); ok {
			return base
		}
	}

	return ""
}

func findWorkspaceRoot(seedDir string) string {
	if seedDir == "" {
		return ""
	}

	seedDir = filepath.Clean(seedDir)
	origSeedDir := seedDir
	for {
		if fileExists(filepath.Join(seedDir, "gnowork.toml")) {
			return seedDir
		}

		parent := filepath.Dir(seedDir)
		if parent == seedDir {
			break
		}
		seedDir = parent
	}

	if hasGnoModule(origSeedDir) {
		return origSeedDir
	}

	return ""
}

// gnoModFiles lists the recognized module definition file names for Gno
// packages, checked in priority order (gnomod.toml takes precedence).
var gnoModFiles = []string{"gnomod.toml", "gno.mod"}

// hasGnoModule reports whether dir contains a Gno module definition file.
func hasGnoModule(dir string) bool {
	for _, f := range gnoModFiles {
		if fileExists(filepath.Join(dir, f)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
