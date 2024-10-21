package eventlogger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gfanton/gnopls/internal/event"
	"github.com/gfanton/gnopls/internal/event/keys"
	"github.com/gfanton/gnopls/internal/event/label"
)

var logger *slog.Logger
var once sync.Once

type noopWritter struct{}

func (noopWritter) Write(data []byte) (n int, err error) {
	return len(data), nil
}

func EventLoggerWrapper() *slog.Logger {
	once.Do(func() {
		subhandler := slog.NewTextHandler(&noopWritter{}, &slog.HandlerOptions{})
		logger = slog.New(&eventWrapper{Handler: subhandler})
	})
	return logger
}

type eventWrapper struct {
	slog.Handler
}

// Handle methods that produce output should observe the following rules:
//   - If r.Time is the zero time, ignore the time.
//   - If r.PC is zero, ignore it.
//   - Attr's values should be resolved.
//   - If an Attr's key and value are both the zero value, ignore the Attr.
//     This can be tested with attr.Equal(Attr{}).
//   - If a group's key is empty, inline the group's Attrs.
//   - If a group has no Attrs (even if it has a non-empty key),
//     ignore it.
func (e *eventWrapper) Handle(ctx context.Context, rec slog.Record) error {
	labels := make([]label.Label, 0)
	var err error
	rec.Attrs(func(attr slog.Attr) bool {
		if err == nil {
			if attrErr, ok := attr.Value.Any().(error); ok {
				err = attrErr
				return true
			}
		}

		labels = append(labels, label.OfString(
			keys.NewString(attr.Key, ""),
			attr.Value.String(),
		))
		return true
	})

	switch rec.Level {
	case slog.LevelInfo:
		event.Log(ctx, rec.Message, labels...)
	case slog.LevelError:
		event.Error(ctx, rec.Message, err, labels...)
	default:
		msg := fmt.Sprintf("[%s] - %s", rec.Level.String(), rec.Message)
		event.Log(ctx, msg, labels...)
	}

	return nil
}
