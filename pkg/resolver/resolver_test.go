package resolver

import (
	"path/filepath"
	"testing"
)

func TestListPackagesPathSkipsNestedWorkspace(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "p", "parentlib", "gnomod.toml"), `module = "gno.land/p/parentlib"`+"\n")
	mustWriteFile(t, filepath.Join(root, "sub", "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "sub", "p", "childlib", "gnomod.toml"), `module = "gno.land/p/childlib"`+"\n")

	got, err := listPackagesPath(root)
	if err != nil {
		t.Fatalf("listPackagesPath() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("listPackagesPath() count = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != filepath.Join(root, "p", "parentlib") {
		t.Fatalf("listPackagesPath() = %v, want only parent workspace package", got)
	}
}

func TestListPackagesPathKeepsRootWorkspace(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "gnowork.toml"), "")
	mustWriteFile(t, filepath.Join(root, "r", "myapp", "gnomod.toml"), `module = "gno.land/r/myapp"`+"\n")

	got, err := listPackagesPath(root)
	if err != nil {
		t.Fatalf("listPackagesPath() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("listPackagesPath() count = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != filepath.Join(root, "r", "myapp") {
		t.Fatalf("listPackagesPath() = %v, want root workspace package", got)
	}
}
