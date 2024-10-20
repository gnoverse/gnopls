package eventlogger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gfanton/gnopls/internal/event"
	"github.com/gfanton/gnopls/internal/event/keys"
	"github.com/gfanton/gnopls/internal/event/label"
)

type eventWrapper struct {
	slog.Handler
}

// // Enabled reports whether the handler handles records at the given level.
// // The handler ignores records whose level is lower.
// // It is called early, before any arguments are processed,
// // to save effort if the log event should be discarded.
// // If called from a Logger method, the first argument is the context
// // passed to that method, or context.Background() if nil was passed
// // or the method does not take a context.
// // The context is passed so Enabled can use its values
// // to make a decision.
// func (*eventWrapper) Enabled(ctx context.Context, lvl slog.Level) bool {
// 	return true
// }

// Handle handles the Record.
// It will only be called when Enabled returns true.
// The Context argument is as for Enabled.
// It is present solely to provide Handlers access to the context's values.
// Canceling the context should not affect record processing.
// (Among other things, log messages may be necessary to debug a
// cancellation-related problem.)
//
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

// // WithAttrs returns a new Handler whose attributes consist of
// // both the receiver's attributes and the arguments.
// // The Handler owns the slice: it may retain, modify or discard it.
// func (*eventWrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
// }

// // WithGroup returns a new Handler with the given group appended to
// // the receiver's existing groups.
// // The keys of all subsequent attributes, whether added by With or in a
// // Record, should be qualified by the sequence of group names.
// //
// // How this qualification happens is up to the Handler, so long as
// // this Handler's attribute keys differ from those of another Handler
// // with a different sequence of group names.
// //
// // A Handler should treat WithGroup as starting a Group of Attrs that ends
// // at the end of the log event. That is,
// //
// //	logger.WithGroup("s").LogAttrs(ctx, level, msg, slog.Int("a", 1), slog.Int("b", 2))
// //
// // should behave like
// //
// //	logger.LogAttrs(ctx, level, msg, slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
// //
// // If the name is empty, WithGroup returns the receiver.
// func (*eventWrapper) WithGroup(name string) slog.Handler {}