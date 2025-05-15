package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

var _ http.ResponseWriter = &responseWriter{}

func (w *responseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func Metrics(metrics *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rw.statusCode)).Observe(duration)
		})
	}
}
