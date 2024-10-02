// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gopls (pronounced “go please”) is an LSP server for Go.
// The Language Server Protocol allows any text editor
// to be extended with IDE-like features;
// see https://langserver.org/ for details.
//
// See https://github.com/golang/tools/blob/master/gopls/README.md
// for the most up-to-date documentation.
package main // import "golang.org/x/tools/gnopls"

import (
	"context"
	"os"

	"github.com/gfanton/gnopls/internal/cmd"
	"github.com/gfanton/gnopls/internal/tool"
	versionpkg "github.com/gfanton/gnopls/internal/version"
	"golang.org/x/telemetry"
)

var version = "" // if set by the linker, overrides the gopls version

func main() {
	versionpkg.VersionOverride = version

	telemetry.Start(telemetry.Config{
		ReportCrashes: true,
		Upload:        true,
	})

	ctx := context.Background()
	tool.Main(ctx, cmd.New(), os.Args[1:])
}
