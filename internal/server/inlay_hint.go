// Copyright 2022 The Go Authors. All rights reserved.
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
)

func (s *server) InlayHint(ctx context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	ctx, done := event.Start(ctx, "lsp.Server.inlayHint", label.URI.Of(params.TextDocument.URI))
	defer done()

	fh, snapshot, release, err := s.fileOf(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer release()

	switch snapshot.FileKind(fh) {
	case file.Mod:
		return mod.InlayHint(ctx, snapshot, fh, params.Range)
	case file.Gno:
		return golang.InlayHint(ctx, snapshot, fh, params.Range)
	}
	return nil, nil // empty result
}
