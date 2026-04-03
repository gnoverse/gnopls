// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package tokeninternal provides access to some internal features of the token
// package.
package tokeninternal

import (
	"go/token"
)

// GetLines returns the table of line-start offsets from a token.File.
func GetLines(file *token.File) []int {
	return file.Lines()
}

// AddExistingFiles adds the specified files to the FileSet if they
// are not already present. It panics if any pair of files in the
// resulting FileSet would overlap.
func AddExistingFiles(fset *token.FileSet, files []*token.File) {
	fset.AddExistingFiles(files...)
}

// FileSetFor returns a new FileSet containing a sequence of new Files with
// the same base, size, and line as the input files, for use in APIs that
// require a FileSet.
//
// Precondition: the input files must be non-overlapping, and sorted in order
// of their Base.
func FileSetFor(files ...*token.File) *token.FileSet {
	fset := token.NewFileSet()
	for _, f := range files {
		f2 := fset.AddFile(f.Name(), f.Base(), f.Size())
		lines := GetLines(f)
		f2.SetLines(lines)
	}
	return fset
}

// CloneFileSet creates a new FileSet holding all files in fset. It does not
// create copies of the token.Files in fset: they are added to the resulting
// FileSet unmodified.
func CloneFileSet(fset *token.FileSet) *token.FileSet {
	var files []*token.File
	fset.Iterate(func(f *token.File) bool {
		files = append(files, f)
		return true
	})
	newFileSet := token.NewFileSet()
	AddExistingFiles(newFileSet, files)
	return newFileSet
}
