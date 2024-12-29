package resolver

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gnoverse/gnopls/internal/packages"
	"golang.org/x/mod/modfile"
)

func gnoPkgToGo(gnomodPath string, logger *slog.Logger) *packages.Package {
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
	pkg := &packages.Package{
		Module: &packages.Module{
			Path: gnomodPath,
			Dir:  dir,
		},
	}
	readPkg(pkg, dir, pkgPath, logger)
	return pkg
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
