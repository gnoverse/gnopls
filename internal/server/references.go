// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"context"

	"github.com/gfanton/gnopls/internal/event"
	"github.com/gfanton/gnopls/internal/file"
	"github.com/gfanton/gnopls/internal/golang"
	"github.com/gfanton/gnopls/internal/label"
	"github.com/gfanton/gnopls/internal/protocol"
	"github.com/gfanton/gnopls/internal/telemetry"
	"github.com/gfanton/gnopls/internal/template"
)

func (s *server) References(ctx context.Context, params *protocol.ReferenceParams) (_ []protocol.Location, rerr error) {
	recordLatency := telemetry.StartLatencyTimer("references")
	defer func() {
		recordLatency(ctx, rerr)
	}()

	ctx, done := event.Start(ctx, "lsp.Server.references", label.URI.Of(params.TextDocument.URI))
	defer done()

	fh, snapshot, release, err := s.fileOf(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer release()
	switch snapshot.FileKind(fh) {
	case file.Tmpl:
		return template.References(ctx, snapshot, fh, params)
	case file.Gno:
		return golang.References(ctx, snapshot, fh, params.Position, params.Context.IncludeDeclaration)
	}
	return nil, nil // empty result
}
