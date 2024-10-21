// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"context"

	"github.com/gnoverse/gnopls/internal/event"
	"github.com/gnoverse/gnopls/internal/file"
	"github.com/gnoverse/gnopls/internal/golang"
	"github.com/gnoverse/gnopls/internal/label"
	"github.com/gnoverse/gnopls/internal/protocol"
)

func (s *server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	ctx, done := event.Start(ctx, "lsp.Server.signatureHelp", label.URI.Of(params.TextDocument.URI))
	defer done()

	fh, snapshot, release, err := s.fileOf(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer release()

	if snapshot.FileKind(fh) != file.Gno {
		return nil, nil // empty result
	}

	info, activeParameter, err := golang.SignatureHelp(ctx, snapshot, fh, params.Position)
	if err != nil {
		// TODO(rfindley): is this correct? Apparently, returning an error from
		// signatureHelp is distracting in some editors, though I haven't confirmed
		// that recently.
		//
		// It's unclear whether we still need to avoid returning this error result.
		event.Error(ctx, "signature help failed", err, label.Position.Of(params.Position))
		return nil, nil
	}
	if info == nil {
		return nil, nil
	}
	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{*info},
		ActiveParameter: uint32(activeParameter),
	}, nil
}
