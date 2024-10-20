package eventlogger

import (
	"log/slog"
	"sync"
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
