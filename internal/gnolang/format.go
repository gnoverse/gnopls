package gnolang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gnoverse/gnopls/internal/cache"
	"github.com/gnoverse/gnopls/internal/cache/parsego"
	"github.com/gnoverse/gnopls/internal/diff"
	"github.com/gnoverse/gnopls/internal/event"
	"github.com/gnoverse/gnopls/internal/file"
	"github.com/gnoverse/gnopls/internal/golang"
	"github.com/gnoverse/gnopls/internal/protocol"
)

// Format formats a file with a given range.
func Format(ctx context.Context, snapshot *cache.Snapshot, fh file.Handle) ([]protocol.TextEdit, error) {
	ctx, done := event.Start(ctx, "gnolang.Format")
	defer done()

	// Generated files shouldn't be edited. So, don't format them
	if golang.IsGenerated(ctx, snapshot, fh.URI()) {
		return nil, fmt.Errorf("can't format %q: file is generated", fh.URI().Path())
	}

	bz, err := formatSource(ctx, fh)
	if err != nil {
		return nil, fmt.Errorf("can't format %q: %w", fh.URI().Path(), err)
	}

	pgf, err := snapshot.ParseGo(ctx, fh, parsego.Full)
	if err != nil {
		return nil, err
	}
	return computeTextEdits(ctx, pgf, string(bz))
}

func formatSource(ctx context.Context, fh file.Handle) ([]byte, error) {
	_, done := event.Start(ctx, "gnolang.formatSource")
	defer done()
	// Write file content into tmpFile for gno fmt argument
	tmpDir, err := os.MkdirTemp(os.TempDir(), "gnofmt")
	if err != nil {
		return nil, fmt.Errorf("cant create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	data, err := fh.Content()
	if err != nil {
		return nil, fmt.Errorf("cant read file %s content: %v", fh.URI().Path(), err)
	}
	tmpFile := filepath.Join(tmpDir, filepath.Base(fh.URI().Path()))
	err = os.WriteFile(tmpFile, data, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("cant write file %s content: %v", tmpFile, err)
	}

	// Run gno fmt on tmpFile
	const gnoBin = "gno"
	args := []string{"fmt", tmpFile}
	bz, err := exec.Command(gnoBin, args...).Output()
	if err != nil {
		return bz, fmt.Errorf("running '%s %s': %w: %s", gnoBin, strings.Join(args, " "), err, string(bz))
	}

	return bz, nil
}

func computeTextEdits(ctx context.Context, pgf *parsego.File, formatted string) ([]protocol.TextEdit, error) {
	_, done := event.Start(ctx, "gnolang.computeTextEdits")
	defer done()

	edits := diff.Strings(string(pgf.Src), formatted)
	return protocol.EditsFromDiffEdits(pgf.Mapper, edits)
}
