// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package work

import (
	"context"

	"golang.org/x/mod/modfile"
	"github.com/gfanton/gnopls/internal/cache"
	"github.com/gfanton/gnopls/internal/file"
	"github.com/gfanton/gnopls/internal/protocol"
	"github.com/gfanton/gnopls/internal/diff"
	"github.com/gfanton/gnopls/internal/event"
)

func Format(ctx context.Context, snapshot *cache.Snapshot, fh file.Handle) ([]protocol.TextEdit, error) {
	ctx, done := event.Start(ctx, "work.Format")
	defer done()

	pw, err := snapshot.ParseWork(ctx, fh)
	if err != nil {
		return nil, err
	}
	formatted := modfile.Format(pw.File.Syntax)
	// Calculate the edits to be made due to the change.
	diffs := diff.Bytes(pw.Mapper.Content, formatted)
	return protocol.EditsFromDiffEdits(pw.Mapper, diffs)
}
