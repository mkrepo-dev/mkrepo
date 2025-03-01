package log

import (
	"log/slog"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

func SetupLogger() {
	config := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "ts",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.AddSync(os.Stderr), zap.DebugLevel)
	coreZapSlogOptions := []zapslog.HandlerOption{zapslog.WithCaller(true), zapslog.AddStacktraceAt(slog.LevelError)}

	handler := zapslog.NewHandler(core, coreZapSlogOptions...)
	logger := slog.New(handler)

	slog.SetDefault(logger)
}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}

func Fatal(msg string, err error, code int) {
	slog.Error(msg, Err(err))
	os.Exit(code)
}
