// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package settings

import (
	"sync"
	"time"

	"github.com/gnoverse/gnopls/internal/file"
	"github.com/gnoverse/gnopls/internal/protocol"
	"github.com/gnoverse/gnopls/internal/protocol/command"
)

var (
	optionsOnce    sync.Once
	defaultOptions *Options
)

// DefaultOptions is the options that are used for Gopls execution independent
// of any externally provided configuration (LSP initialization, command
// invocation, etc.).
//
// It is the source from which gopls/doc/settings.md is generated.
func DefaultOptions(overrides ...func(*Options)) *Options {
	optionsOnce.Do(func() {
		var commands []string
		for _, c := range command.Commands {
			commands = append(commands, c.String())
		}
		defaultOptions = &Options{
			ClientOptions: ClientOptions{
				InsertTextFormat:                           protocol.PlainTextTextFormat,
				PreferredContentFormat:                     protocol.Markdown,
				ConfigurationSupported:                     true,
				DynamicConfigurationSupported:              true,
				DynamicRegistrationSemanticTokensSupported: true,
				DynamicWatchedFilesSupported:               true,
				LineFoldingOnly:                            false,
				HierarchicalDocumentSymbolSupport:          true,
			},
			ServerOptions: ServerOptions{
				SupportedCodeActions: map[file.Kind]map[protocol.CodeActionKind]bool{
					file.Gno: {
						// This should include specific leaves in the tree,
						// (e.g. refactor.inline.call) not generic branches
						// (e.g. refactor.inline or refactor).
						protocol.SourceFixAll:            true,
						protocol.SourceOrganizeImports:   true,
						protocol.QuickFix:                false,
						GoAssembly:                       false,
						GoDoc:                            true,
						GoFreeSymbols:                    false,
						GoplsDocFeatures:                 true,
						RefactorRewriteChangeQuote:       true,
						RefactorRewriteFillStruct:        true,
						RefactorRewriteFillSwitch:        true,
						RefactorRewriteInvertIf:          true,
						RefactorRewriteJoinLines:         true,
						RefactorRewriteRemoveUnusedParam: true,
						RefactorRewriteSplitLines:        true,
						RefactorInlineCall:               true,
						RefactorExtractFunction:          true,
						RefactorExtractMethod:            true,
						RefactorExtractVariable:          true,
						RefactorExtractToNewFile:         true,
						// Not GoTest: it must be explicit in CodeActionParams.Context.Only
					},
					file.Mod: {
						protocol.SourceOrganizeImports: true,
						protocol.QuickFix:              true,
					},
					file.Work: {},
					file.Sum:  {},
					file.Tmpl: {},
				},
				SupportedCommands: commands,
			},
			UserOptions: UserOptions{
				BuildOptions: BuildOptions{
					ExpandWorkspaceToModule: true,
					DirectoryFilters:        []string{"-**/node_modules"},
					TemplateExtensions:      []string{},
					StandaloneTags:          []string{"ignore"},
				},
				UIOptions: UIOptions{
					DiagnosticOptions: DiagnosticOptions{
						Annotations: map[Annotation]bool{
							Bounds: true,
							Escape: true,
							Inline: true,
							Nil:    true,
						},
						Vulncheck:                 ModeVulncheckOff,
						DiagnosticsDelay:          1 * time.Second,
						DiagnosticsTrigger:        DiagnosticsOnEdit,
						AnalysisProgressReporting: true,
					},
					InlayHintOptions: InlayHintOptions{},
					DocumentationOptions: DocumentationOptions{
						HoverKind:    FullDocumentation,
						LinkTarget:   "pkg.go.dev",
						LinksInHover: LinksInHover_LinkTarget,
					},
					NavigationOptions: NavigationOptions{
						ImportShortcut: BothShortcuts,
						SymbolMatcher:  SymbolFastFuzzy,
						SymbolStyle:    DynamicSymbols,
						SymbolScope:    AllSymbolScope,
					},
					CompletionOptions: CompletionOptions{
						Matcher:                        Fuzzy,
						CompletionBudget:               100 * time.Millisecond,
						ExperimentalPostfixCompletions: true,
						CompleteFunctionCalls:          true,
					},
					Codelenses: map[CodeLensSource]bool{
						CodeLensGenerate:          false,
						CodeLensRegenerateCgo:     false,
						CodeLensTidy:              false,
						CodeLensGCDetails:         false,
						CodeLensUpgradeDependency: false,
						CodeLensVendor:            false,
						CodeLensRunGovulncheck:    false, // TODO(hyangah): enable
					},
				},
			},
			InternalOptions: InternalOptions{
				CompleteUnimported:          true,
				CompletionDocumentation:     true,
				DeepCompletion:              true,
				SubdirWatchPatterns:         SubdirWatchPatternsAuto,
				ReportAnalysisProgressAfter: 5 * time.Second,
				TelemetryPrompt:             false,
				LinkifyShowMessage:          false,
				IncludeReplaceInWorkspace:   false,
				ZeroConfig:                  true,
			},
		}
	})
	options := defaultOptions.Clone()
	for _, override := range overrides {
		if override != nil {
			override(options)
		}
	}
	return options
}
