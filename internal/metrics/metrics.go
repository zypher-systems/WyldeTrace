package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wyldetrace_http_requests_total",
		Help: "Total HTTP requests by method, path group, and status class.",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "wyldetrace_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	SweepDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "wyldetrace_sweep_duration_seconds",
		Help:    "ICMP sweep wall time in seconds.",
		Buckets: []float64{0.5, 1, 2, 5, 15, 30, 60, 90},
	})

	SchedulerTicks = promauto.NewCounter(prometheus.CounterOpts{
		Name: "wyldetrace_scheduler_ticks_total",
		Help: "Discovery scheduler ticks that acquired the advisory lock.",
	})

	WebhookFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "wyldetrace_webhook_failures_total",
		Help: "Webhook deliveries that ended in error status.",
	})
)
