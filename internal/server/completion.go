// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gnopls/internal/event"
	"github.com/gnoverse/gnopls/internal/file"
	"github.com/gnoverse/gnopls/internal/golang"
	"github.com/gnoverse/gnopls/internal/golang/completion"
	"github.com/gnoverse/gnopls/internal/label"
	"github.com/gnoverse/gnopls/internal/protocol"
	"github.com/gnoverse/gnopls/internal/settings"
	"github.com/gnoverse/gnopls/internal/telemetry"
	"github.com/gnoverse/gnopls/internal/template"
	"github.com/gnoverse/gnopls/internal/work"
)

func (s *server) Completion(ctx context.Context, params *protocol.CompletionParams) (_ *protocol.CompletionList, rerr error) {
	recordLatency := telemetry.StartLatencyTimer("completion")
	defer func() {
		recordLatency(ctx, rerr)
	}()

	ctx, done := event.Start(ctx, "lsp.Server.completion", label.URI.Of(params.TextDocument.URI))
	defer done()

	fh, snapshot, release, err := s.fileOf(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer release()

	var candidates []completion.CompletionItem
	var surrounding *completion.Selection
	switch snapshot.FileKind(fh) {
	case file.Gno:
		candidates, surrounding, err = completion.Completion(ctx, snapshot, fh, params.Position, params.Context)
	case file.Mod:
		candidates, surrounding = nil, nil
	case file.Work:
		cl, err := work.Completion(ctx, snapshot, fh, params.Position)
		if err != nil {
			break
		}
		return cl, nil
	case file.Tmpl:
		var cl *protocol.CompletionList
		cl, err = template.Completion(ctx, snapshot, fh, params.Position, params.Context)
		if err != nil {
			break // use common error handling, candidates==nil
		}
		return cl, nil
	}
	if err != nil {
		event.Error(ctx, "no completions found", err, label.Position.Of(params.Position))
	}
	if candidates == nil || surrounding == nil {
		complEmpty.Inc()
		return &protocol.CompletionList{
			IsIncomplete: true,
			Items:        []protocol.CompletionItem{},
		}, nil
	}

	// When using deep completions/fuzzy matching, report results as incomplete so
	// client fetches updated completions after every key stroke.
	options := snapshot.Options()
	incompleteResults := options.DeepCompletion || options.Matcher == settings.Fuzzy

	items, err := toProtocolCompletionItems(candidates, surrounding, options)
	if err != nil {
		return nil, err
	}
	if snapshot.FileKind(fh) == file.Gno {
		s.saveLastCompletion(fh.URI(), fh.Version(), items, params.Position)
	}

	if len(items) > 10 {
		// TODO(pjw): long completions are ok for field lists
		complLong.Inc()
	} else {
		complShort.Inc()
	}
	return &protocol.CompletionList{
		IsIncomplete: incompleteResults,
		Items:        items,
	}, nil
}

func (s *server) saveLastCompletion(uri protocol.DocumentURI, version int32, items []protocol.CompletionItem, pos protocol.Position) {
	s.efficacyMu.Lock()
	defer s.efficacyMu.Unlock()
	s.efficacyVersion = version
	s.efficacyURI = uri
	s.efficacyPos = pos
	s.efficacyItems = items
}

func toProtocolCompletionItems(candidates []completion.CompletionItem, surrounding *completion.Selection, options *settings.Options) ([]protocol.CompletionItem, error) {
	replaceRng, err := surrounding.Range()
	if err != nil {
		return nil, err
	}
	insertRng0, err := surrounding.PrefixRange()
	if err != nil {
		return nil, err
	}
	suffix := surrounding.Suffix()

	var (
		items                  = make([]protocol.CompletionItem, 0, len(candidates))
		numDeepCompletionsSeen int
	)
	for i, candidate := range candidates {
		// Limit the number of deep completions to not overwhelm the user in cases
		// with dozens of deep completion matches.
		if candidate.Depth > 0 {
			if !options.DeepCompletion {
				continue
			}
			if numDeepCompletionsSeen >= completion.MaxDeepCompletions {
				continue
			}
			numDeepCompletionsSeen++
		}
		insertText := candidate.InsertText
		if options.InsertTextFormat == protocol.SnippetTextFormat {
			insertText = candidate.Snippet()
		}

		// This can happen if the client has snippets disabled but the
		// candidate only supports snippet insertion.
		if insertText == "" {
			continue
		}

		doc := &protocol.Or_CompletionItem_documentation{
			Value: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: golang.CommentToMarkdown(candidate.Documentation, options),
			},
		}
		if options.PreferredContentFormat != protocol.Markdown {
			doc.Value = candidate.Documentation
		}
		var edits *protocol.Or_CompletionItem_textEdit
		if options.InsertReplaceSupported {
			insertRng := insertRng0
			if suffix == "" || strings.Contains(insertText, suffix) {
				insertRng = replaceRng
			}
			// Insert and Replace ranges share the same start position and
			// the same text edit but the end position may differ.
			// See the comment for the CompletionItem's TextEdit field.
			// https://pkg.go.dev/github.com/gnoverse/gnopls/internal/protocol#CompletionItem
			edits = &protocol.Or_CompletionItem_textEdit{
				Value: protocol.InsertReplaceEdit{
					NewText: insertText,
					Insert:  insertRng, // replace up to the cursor position.
					Replace: replaceRng,
				},
			}
		} else {
			edits = &protocol.Or_CompletionItem_textEdit{
				Value: protocol.TextEdit{
					NewText: insertText,
					Range:   replaceRng,
				},
			}
		}
		item := protocol.CompletionItem{
			Label:               candidate.Label,
			Detail:              candidate.Detail,
			Kind:                candidate.Kind,
			TextEdit:            edits,
			InsertTextFormat:    &options.InsertTextFormat,
			AdditionalTextEdits: candidate.AdditionalTextEdits,
			// This is a hack so that the client sorts completion results in the order
			// according to their score. This can be removed upon the resolution of
			// https://github.com/Microsoft/language-server-protocol/issues/348.
			SortText: fmt.Sprintf("%05d", i),

			// Trim operators (VSCode doesn't like weird characters in
			// filterText).
			FilterText: strings.TrimLeft(candidate.InsertText, "&*"),

			Preselect:     i == 0,
			Documentation: doc,
			Tags:          protocol.NonNilSlice(candidate.Tags),
			Deprecated:    candidate.Deprecated,
		}
		items = append(items, item)
	}
	return items, nil
}
