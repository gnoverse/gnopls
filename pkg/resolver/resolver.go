package resolver

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnoverse/gnopls/internal/packages"
	"golang.org/x/mod/modfile"
)

func gnoPkgToGo(gnomodPath string, logger *slog.Logger) (*packages.Package, error) {
	gnomodBytes, err := os.ReadFile(gnomodPath)
	if err != nil {
		return nil, err
	}
	gnomodFile, err := modfile.ParseLax(gnomodPath, gnomodBytes, nil)
	if err != nil {
		return nil, err
	}
	if gnomodFile == nil || gnomodFile.Module == nil {
		return nil, fmt.Errorf("%s:1:1: missing module", gnomodPath)
	}
	dir := filepath.Dir(gnomodPath)

	// TODO: support subpkgs

	pkgDir := filepath.Clean(dir)

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

	bestName, imports, err := resolveNameAndImports(gnoFiles, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve name and imports: %w", err)
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

func resolveNameAndImports(gnoFiles []string, logger *slog.Logger) (string, map[string]*packages.Package, error) {
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
		f, err := parser.ParseFile(fset, srcPath, src, parser.SkipObjectResolution|parser.ImportsOnly)
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
			if len(importPath) >= 2 {
				importPath = importPath[1 : len(importPath)-1]
			}
			imports[importPath] = nil
		}
	}
	logger.Info("analyzed sources", slog.String("name", bestName), slog.Any("imports", imports))

	return bestName, imports, nil
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
