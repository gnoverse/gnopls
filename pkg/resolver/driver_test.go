package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gnoverse/gnopls/internal/packages"
)

func TestDiscoverWorkspaceRootFallbacks(t *testing.T) {
	t.Run("empty patterns", func(t *testing.T) {
		if got := discoverWorkspaceRoot(nil); got != "" {
			t.Fatalf("discoverWorkspaceRoot() = %q, want empty", got)
		}
	})

	t.Run("no workspace and no module", func(t *testing.T) {
		root := t.TempDir()
		got := discoverWorkspaceRoot([]string{"file=" + filepath.Join(root, "main.gno")})
		if got != "" {
			t.Fatalf("discoverWorkspaceRoot() = %q, want empty", got)
		}
	})
}

func TestDiscoverWorkspaceRootFromFilePattern(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "myapp.gno"), "package myapp\n")

	got := discoverWorkspaceRoot([]string{"file=" + filepath.Join(root, "r", "myapp", "myapp.gno")})
	if got != root {
		t.Fatalf("discoverWorkspaceRoot() = %q, want %q", got, root)
	}
}

func TestDiscoverWorkspaceRootFallsBackToSeedModule(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")

	got := discoverWorkspaceRoot([]string{filepath.Join(root, "...")})
	if got != root {
		t.Fatalf("discoverWorkspaceRoot() = %q, want %q", got, root)
	}
}

func TestResolveUsesRequestDirForRelativeWorkspacePattern(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "gnomod.toml"), `module = "gno.land/p/mylib"`+"\n")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "mylib.gno"), "package mylib\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "myapp.gno"), "package myapp\n\nimport \"gno.land/p/mylib\"\n")

	res, err := Resolve(&packages.DriverRequest{Dir: root}, "./...")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	pkgs := make(map[string]*packages.Package)
	for _, pkg := range res.Packages {
		pkgs[pkg.PkgPath] = pkg
	}

	app := pkgs["gno.land/r/myapp"]
	lib := pkgs["gno.land/p/mylib"]
	if app == nil || lib == nil {
		t.Fatalf("relative workspace load missed packages: app=%v lib=%v", app != nil, lib != nil)
	}
	if got := app.Imports["gno.land/p/mylib"]; got != lib {
		t.Fatalf("relative workspace import not resolved to workspace package: got %#v, want %#v", got, lib)
	}
}

func TestResolveDiscoversWorkspacePackages(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "gnomod.toml"), `module = "gno.land/p/mylib"`+"\n")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "mylib.gno"), "package mylib\n\nfunc Name() string {\n\treturn \"mylib\"\n}\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "myapp.gno"), "package myapp\n\nimport \"gno.land/p/mylib\"\n\nfunc Use() string {\n\treturn mylib.Name()\n}\n")

	res, err := Resolve(&packages.DriverRequest{}, "file="+filepath.Join(root, "r", "myapp", "myapp.gno"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	pkgs := make(map[string]*packages.Package)
	for _, pkg := range res.Packages {
		pkgs[pkg.PkgPath] = pkg
	}

	lib := pkgs["gno.land/p/mylib"]
	if lib == nil {
		t.Fatalf("workspace dependency package not discovered")
	}

	app := pkgs["gno.land/r/myapp"]
	if app == nil {
		t.Fatalf("workspace app package not discovered")
	}

	if got := app.Imports["gno.land/p/mylib"]; got != lib {
		t.Fatalf("app import not resolved to workspace package: got %#v, want %#v", got, lib)
	}
}

func TestResolveDeduplicatesWorkspaceTargets(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "gnomod.toml"), `module = "gno.land/p/mylib"`+"\n")
	mustWriteFile(t, filepath.Join(root, "p", "mylib", "mylib.gno"), "package mylib\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "myapp.gno"), "package myapp\n")

	res, err := Resolve(&packages.DriverRequest{}, filepath.Join(root, "r", "myapp", "..."))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	var appCount, libCount int
	for _, pkg := range res.Packages {
		switch pkg.PkgPath {
		case "gno.land/r/myapp":
			appCount++
		case "gno.land/p/mylib":
			libCount++
		}
	}

	if appCount != 1 {
		t.Fatalf("myapp package count = %d, want 1", appCount)
	}
	if libCount != 1 {
		t.Fatalf("mylib package count = %d, want 1", libCount)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
