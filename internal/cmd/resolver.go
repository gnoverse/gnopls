package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gnoverse/gnopls/internal/packages"
	"github.com/gnoverse/gnopls/pkg/eventlogger"
	"github.com/gnoverse/gnopls/pkg/resolver"
)

type Resolver struct {
	app *Application
}

// Name returns the application's name. It is used in help and error messages.
func (*Resolver) Name() string {
	return "resolve"
}

// Most of the help usage is automatically generated, this string should only
// describe the contents of non flag arguments.
func (*Resolver) Usage() string {
	return "TODO"
}

// ShortHelp returns the one line overview of the command.
func (*Resolver) ShortHelp() string {
	return "resolve gno package from stdin request"
}

// DetailedHelp should print a detailed help message. It will only ever be shown
// when the ShortHelp is also printed, so there is no need to duplicate
// anything from there.
// It is passed the flag set so it can print the default values of the flags.
// It should use the flag sets configured Output to write the help to.
func (*Resolver) DetailedHelp(f *flag.FlagSet) {
	fmt.Fprintf(f.Output(), "TODO")
}

// Run is invoked after all flag processing, and inside the profiling and
// error handling harness.
func (*Resolver) Run(ctx context.Context, args ...string) error {
	logger := eventlogger.EventLoggerWrapper()
	logger.Info("started gnopls resolver", slog.Any("args", args))

	reqBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err.Error)
	}

	req := packages.DriverRequest{}
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	res, err := resolver.Resolve(&req, args...)
	if err != nil {
		return fmt.Errorf("failed to resolve packages: %w", err)
	}

	out, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

func (s *Resolver) Parent() string { return s.app.Name() }
