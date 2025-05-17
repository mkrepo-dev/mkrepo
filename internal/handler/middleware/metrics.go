package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
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

			ipVersion := requestRemoteIPVersion(r)
			metrics.RequestIPVersion.WithLabelValues(ipVersion).Inc()

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rw.statusCode)).Observe(duration)
		})
	}
}

func requestRemoteIPVersion(r *http.Request) string {
	var ipStr string
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ipStr = strings.Split(xff, ",")[0]
		ipStr = strings.TrimSpace(ipStr)
	} else if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ipStr = xrip
	} else {
		var err error
		ipStr, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "unknown"
		}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "unknown"
	}

	if ip.To4() != nil {
		return "ipv4"
	}
	if ip.To16() != nil {
		return "ipv6"
	}
	return "unknown"
}
