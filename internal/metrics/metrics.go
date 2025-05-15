package metrics

import (
	"database/sql"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	buildInfo       *prometheus.GaugeVec
	ReposCreated    prometheus.Counter
	RequestDuration *prometheus.HistogramVec
}

func NewMetrics(reg *prometheus.Registry, db *sql.DB) *Metrics {
	metrics := &Metrics{
		buildInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "mkrepo", Subsystem: "build", Name: "info",
			Help: "Build info",
		}, []string{"version", "revision", "go_version", "build_datetime"}),
		ReposCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "mkrepo", Subsystem: "repo", Name: "created_total",
			Help: "Total number of repositories created",
		}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "mkrepo", Subsystem: "http", Name: "request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10},
		}, []string{"method", "path", "status"}),
	}
	buildInfo := internal.ReadVersion()
	metrics.buildInfo.WithLabelValues(
		buildInfo.Version,
		buildInfo.Revision,
		buildInfo.GoVersion,
		buildInfo.BuildDatetime.Format(time.RFC3339),
	).Set(1)
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewDBStatsCollector(db, "mkrepo"))
	reg.MustRegister(metrics.buildInfo)
	reg.MustRegister(metrics.ReposCreated)
	reg.MustRegister(metrics.RequestDuration)
	return metrics
}
