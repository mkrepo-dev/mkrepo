package log

import (
	"log/slog"
	"os"
)

func SetupLogger() *slog.Logger {
	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}),
	)
	slog.SetDefault(logger)
	return logger
}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}

func Component(logger *slog.Logger, component string) *slog.Logger {
	return logger.With(slog.String("component", component))
}
