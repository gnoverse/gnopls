package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/gnovm/pkg/gnomod"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

func main() {
	// init logger

	conf := zap.NewDevelopmentConfig()
	// conf.OutputPaths = []string{"/tmp/gnopackagesdriver.log"}
	logger, err := conf.Build()
	if err != nil {
		panic(err)
	}

	// read request

	flag.Parse()
	args := flag.Args()

	logger.Info("started gnopackagesdriver", zap.Strings("args", args))

	reqBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		logger.Error("failed to read request", zap.Error(err))
		panic(err)
	}

	req := packages.DriverRequest{}
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		logger.Error("failed to unmarshal request", zap.Error(err))
		panic(err)
	}

	logger.Info("unmarshalled request", zap.String("mode", req.Mode.String()), zap.Bool("tests", req.Tests), zap.Strings("build-flags", req.BuildFlags), zap.Reflect("overlay", req.Overlay))

	// inject examples

	gnoRoot, err := gnoenv.GuessRootDir()
	if err != nil {
		logger.Warn("can't find gno root, examples and std packages are ignored", zap.Error(err))
	}

	targets := args

	if gnoRoot != "" {
		targets = append(args, filepath.Join(gnoRoot, "examples", "..."))
	}

	// inject stdlibs

	pkgsCache := map[string]*packages.Package{}
	res := packages.DriverResponse{}

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

			logger.Info("injecting stdlib", zap.String("path", path), zap.String("name", name))

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
			logger.Warn("failed to inject all stdlibs", zap.Error(err))
		}
	}

	// discover packages

	pkgs := gnomod.PkgList{}
	for _, target := range targets {
		// TODO: handle std libs and queries
		dir, file := filepath.Split(target)
		if file == "..." {
			pkgQueryRes, err := ListPkgs(dir)
			if err != nil {
				logger.Error("failed to get pkg list", zap.Error(err))
				panic(err)
			}
			pkgs = append(pkgs, pkgQueryRes...)
		} else if strings.HasPrefix(target, "file=") {
			dir = strings.TrimPrefix(dir, "file=")
			pkgQueryRes, err := ListPkgs(dir)
			if err != nil {
				logger.Error("failed to get pkg", zap.Error(err))
				panic(err)
			}
			if len(pkgQueryRes) != 1 {
				logger.Warn("unexpected number of packages", zap.String("arg", target), zap.Int("count", len(pkgQueryRes)))
			}
			pkgs = append(pkgs, pkgQueryRes...)
		} else {
			logger.Warn("unknown arg shape", zap.String("value", target))
		}
	}
	logger.Info("discovered packages", zap.Int("count", len(pkgs)))

	// convert packages

	for _, pkg := range pkgs {
		pkg, err := gnoPkgToGo(&pkg, logger)
		if err != nil {
			logger.Error("failed to convert gno pkg to go pkg", zap.Error(err))
			continue
		}
		pkgsCache[pkg.PkgPath] = pkg
		res.Packages = append(res.Packages, pkg)
		res.Roots = append(res.Roots, pkg.ID)
	}

	// resolve imports

	for _, pkg := range res.Packages {
		toDelete := []string{}
		for importPath := range pkg.Imports {
			imp, ok := pkgsCache[importPath]
			if ok {
				pkg.Imports[importPath] = imp
				logger.Info("found import", zap.String("path", importPath))
			} else {
				// TODO: load additional package
				logger.Info("missed import", zap.String("path", importPath))
				toDelete = append(toDelete, importPath)
			}
		}
		for _, toDel := range toDelete {
			delete(pkg.Imports, toDel)
		}
		logger.Info("converted package", zap.Reflect("pkg", pkg))
	}

	// respond

	out, err := json.Marshal(res)
	if err != nil {
		logger.Error("failed to marshall response", zap.Error(err))
		panic(err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		logger.Error("failed to write response", zap.Error(err))
		panic(err)
	}
	logger.Info("success")
}

func gnoPkgToGo(gnoPkg *gnomod.Pkg, logger *zap.Logger) (*packages.Package, error) {
	// TODO: support subpkgs

	gnomodFile, err := gnomod.ParseAt(gnoPkg.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gno module at %q: %w", gnoPkg.Dir, err)
	}

	pkgDir := filepath.Clean(gnoPkg.Dir)

	gnoFiles := []string{}
	otherFiles := []string{}
	dirEntries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read pkg dir %q: %w", pkgDir, err)
	}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		fpath := filepath.Join(pkgDir, entry.Name())
		if strings.HasSuffix(fpath, ".gno") {
			if !strings.HasSuffix(fpath, "_test.gno") && !strings.HasSuffix(fpath, "_filetest.gno") {
				gnoFiles = append(gnoFiles, fpath)
			}
		} else {
			// TODO: should we really include all other files?
			otherFiles = append(otherFiles, fpath)
		}
	}

	// find package name and imports
	bestName, imports, err := resolveNameAndImports(gnoFiles, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to resove name and imports: %w", err)
	}

	return &packages.Package{
		// Always required
		ID:     pkgDir,
		Errors: nil, // TODO

		// NeedName
		Name:    bestName,
		PkgPath: gnomodFile.Module.Mod.Path,

		// NeedFiles
		GoFiles:    gnoFiles,
		OtherFiles: otherFiles,

		// NeedCompiledGoFiles
		CompiledGoFiles: gnoFiles, // TODO: check if enough

		// NeedImports
		// if not NeedDeps, only ID filled
		Imports: imports,
	}, nil
}

func resolveNameAndImports(gnoFiles []string, logger *zap.Logger) (string, map[string]*packages.Package, error) {
	names := map[string]int{}
	imports := map[string]*packages.Package{}
	bestName := ""
	bestNameCount := 0
	for _, srcPath := range gnoFiles {
		src, err := os.ReadFile(srcPath)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read file %q: %w", srcPath, err)
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, srcPath, src,
			// SkipObjectResolution -- unused here.
			// ParseComments -- so that they show up when re-building the AST.
			parser.SkipObjectResolution|parser.ImportsOnly)
		if err != nil {
			return "", nil, fmt.Errorf("parse: %w", err)
		}

		name := f.Name.String()
		names[name] += 1
		count := names[name]
		if count > bestNameCount {
			bestName = name
			bestNameCount = count
		}

		for _, imp := range f.Imports {
			importPath := imp.Path.Value
			// trim quotes
			if len(importPath) >= 2 {
				importPath = importPath[1 : len(importPath)-1]
			}
			imports[importPath] = nil
		}
	}
	logger.Info("analyzed sources", zap.String("name", bestName), zap.Reflect("imports", imports))

	return bestName, imports, nil
}

// fork of gno/gnovm/pkg/gnomod/pkg.go that ignores malformed gnomods instead of error out
// ListPkgs lists all gno packages in the given root directory.
func ListPkgs(root string) (gnomod.PkgList, error) {
	var pkgs []gnomod.Pkg

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		gnoModPath := filepath.Join(path, "gno.mod")
		data, err := os.ReadFile(gnoModPath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}

		gnoMod, err := gnomod.Parse(gnoModPath, data)
		if err != nil {
			return nil
		}
		gnoMod.Sanitize()
		if err := gnoMod.Validate(); err != nil {
			return nil
		}

		pkgs = append(pkgs, gnomod.Pkg{
			Dir:   path,
			Name:  gnoMod.Module.Mod.Path,
			Draft: gnoMod.Draft,
			Requires: func() []string {
				var reqs []string
				for _, req := range gnoMod.Require {
					reqs = append(reqs, req.Mod.Path)
				}
				return reqs
			}(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return pkgs, nil
}
