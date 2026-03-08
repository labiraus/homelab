package prometheusutil

import (
	"net/http"
	"time"

	"pkg/base"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	opsProcessed *prometheus.CounterVec
	opDuration   *prometheus.HistogramVec
)

func IncrementProcessed(method string, state string) {
	opsProcessed.WithLabelValues(method, state).Inc()
}

func OpDuration(method string, duration time.Duration) {
	opDuration.WithLabelValues(method).Observe(duration.Seconds())
}

func Start(mux *http.ServeMux) {
	opDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    base.ServiceName + "_processing_duration_seconds",
		Help:    "The duration of the processing of the events",
		Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
	}, []string{"method"})
	opsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: base.ServiceName + "_ops_total",
		Help: "The total number of processed events",
	}, []string{"method", "state"})

	registry := prometheus.NewRegistry()

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		opsProcessed,
		opDuration,
	)

	// Expose /metrics HTTP endpoint using the created custom registry.
	mux.Handle(
		"/metrics", promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
}
