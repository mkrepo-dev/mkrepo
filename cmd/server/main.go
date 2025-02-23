package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/templates"
)

func main() {
	log.SetupLogger()

	version := internal.ReadVersion()
	slog.Info("Started mkrepo server",
		slog.String("version", version.Version),
		slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision[:7]),
		slog.String("buildDatetime", version.BuildDatetime),
	)

	handler := http.NewServeMux()
	handler.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("base.html").ParseFS(templates.TemplatesFS, "base.html", "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = tmpl.Execute(w, map[string]any{"Title": "Home", "Body": "This is the body"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	handler.HandleFunc("GET /new", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("base.html").ParseFS(templates.TemplatesFS, "base.html", "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		templates.Render(w, tmpl, map[string]any{"Title": "New", "Body": "This is the new body"})
	})

	server := &http.Server{
		Addr:         ":8000",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Failed to run server", log.Err(err))
		os.Exit(1)
	}
}
