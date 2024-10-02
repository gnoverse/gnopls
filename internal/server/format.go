// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"context"

	"github.com/gfanton/gnopls/internal/event"
	"github.com/gfanton/gnopls/internal/file"
	"github.com/gfanton/gnopls/internal/golang"
	"github.com/gfanton/gnopls/internal/label"
	"github.com/gfanton/gnopls/internal/mod"
	"github.com/gfanton/gnopls/internal/protocol"
	"github.com/gfanton/gnopls/internal/work"
)

func (s *server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	ctx, done := event.Start(ctx, "lsp.Server.formatting", label.URI.Of(params.TextDocument.URI))
	defer done()

	fh, snapshot, release, err := s.fileOf(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer release()

	switch snapshot.FileKind(fh) {
	case file.Mod:
		return mod.Format(ctx, snapshot, fh)
	case file.Gno:
		return golang.Format(ctx, snapshot, fh)
	case file.Work:
		return work.Format(ctx, snapshot, fh)
	}
	return nil, nil // empty result
}
