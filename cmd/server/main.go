package main

import (
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal"
)

func main() {
	internal.SetupLogger()

	version := internal.ReadVersion()
	slog.Info("Started mkrepo server",
		slog.String("version", version.Version),
		slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision),
		slog.String("buildDatetime", version.BuildDatetime),
	)

	_ = http.ListenAndServe(":8000", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Hello, World!")
		_, _ = w.Write([]byte("Hello, World"))
	}))
}
