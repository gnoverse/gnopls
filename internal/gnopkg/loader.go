package gnopkg

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnomod"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type PackageLoader interface {
	AddPackage(path string) error
	Update(path string) error
	LoadPackages() []*PackagePath
}

type Package struct {
	update       int
	parentsDeps  map[string]*Package
	childrenDeps map[string]*Package
	mempkg       std.MemPackage
}

type Loader struct {
	update int
	fset   *token.FileSet
	roots  []string // Root folder
	pkgs   map[string]*Package
}

func (l *Loader) AddPackage(path string) {
	return
}

func (l *Loader) LoadPackages() []*PackagePath {
	return nil
}

func (l *Loader) Invalidate(path string) []*PackagePath {
	return nil
}

// Resolve path string -> Package
// Package -> resolve Import
// Import Resolve path

func (l *Loader) getPkg(path string) *Package {
	lp, ok := l.pkgs[path]
	if !ok {
		lp := &Package{
			parentsDeps:  map[string]*Package{},
			childrenDeps: map[string]*Package{},
		}
		l.pkgs[path] = lp
	}
	return lp
}

func (l *Loader) Resolve(path string) {
	return
}

func (l *Loader) sortList() []*std.MemPackage {
	return nil
}

func (l *Loader) loadPackage(path string) (*Package, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read dir %q: %w", path, err)
	}

	if !filepath.IsAbs(path) {
		var impdir string
		for _, root := range l.roots {
			impdir = filepath.Join(root, path)
			_, err := os.Stat(impdir)
			if err != nil {
				continue
			}
		}

		if impdir == "" {
			return nil, fmt.Errorf("unable to resolve import %q", path)
		}

		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("unable to determine absolute path of %q: %w", path, err)
		}
	} else {
		path = filepath.Clean(path)
	}

	var pkgpath string

	// Check for a gno.mod, in which case it will define the module path
	gnoModPath := filepath.Join(path, "gno.mod")
	data, err := os.ReadFile(gnoModPath)
	switch {
	case os.IsNotExist(err):
	case err == nil:
		gnoMod, err := gnomod.Parse(gnoModPath, data)
		if err != nil {
			return nil, fmt.Errorf("unable to parse gnomod %q: %w", gnoModPath, err)
		}

		gnoMod.Sanitize()
		if err := gnoMod.Validate(); err != nil {
			return nil, fmt.Errorf("unable to validate gnomod %q: %w", gnoModPath, err)
		}

		pkgpath = gnoMod.Module.Mod.Path
	default:
		return nil, fmt.Errorf("unable to read %q: %w", gnoModPath, err)
	}

	if pkgpath == "" {
		return nil, fmt.Errorf("unable to determine package path of %q", path)
	}

	lp := l.getPkg(path)
	if lp.update == l.update {
		return lp, nil
	}

	// Resolve pkg deps
	var pkgname string
	imports := map[string]struct{}{}
	memfiles := []*std.MemFile{}
	for _, file := range files {
		name := file.Name()
		if !isGnoFile(name) || isTestFile(name) {
			continue
		}

		filepath := filepath.Join(path, name)
		body, err := os.ReadFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %q: %w", filepath, err)
		}

		mf := &std.MemFile{
			Name: name,
			Body: string(body),
		}

		f, err := parser.ParseFile(l.fset, name, body, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("unable to parse file %q: %w", name, err)
		}

		if pkgname != "" && pkgname != f.Name.Name {
			return nil, fmt.Errorf("conflict package name between %q and %q", pkgname, f.Name.Name)
		}

		for _, imp := range f.Imports {
			if len(imp.Path.Value) <= 2 {
				continue
			}

			val := imp.Path.Value[1 : len(imp.Path.Value)-1]
			imports[val] = struct{}{}
		}

		pkgname = f.Name.Name
		memfiles = append(memfiles, mf)
	}

	if len(memfiles) == 0 {
		return nil, fmt.Errorf("%q empty package", path)
	}

	for imp := range imports {
		child, err := l.loadPackage(imp)
		if err != nil {
			return nil, fmt.Errorf("unable to load %q: %w", imp, err)
		}

		child.parentsDeps[path] = lp
	}

	lp.mempkg = std.MemPackage{
		Name:  pkgname,
		Path:  pkgpath,
		Files: memfiles,
	}
	lp.update = l.update // set as updated
	return lp, nil
}

func isGnoFile(name string) bool {
	return filepath.Ext(name) == ".gno" && !strings.HasPrefix(name, ".")
}

func isTestFile(name string) bool {
	return strings.HasSuffix(name, "_filetest.gno") || strings.HasSuffix(name, "_test.gno")
}
