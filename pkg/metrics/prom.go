package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TransformationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgo_transformation_errors_total",
			Help: "Total number of transformation errors by type and pipeline",
		},
		[]string{"error_type", "pipeline", "source", "sink"},
	)

	PublishErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgo_publish_errors_total",
			Help: "Total number of publish errors by sink",
		},
		[]string{"sink"},
	)

	ProcessedEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgo_processed_events_total",
			Help: "Total number of processed events by pipeline",
		},
		[]string{"pipeline", "source", "sink"},
	)

	EventProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pgo_event_processing_duration_seconds",
			Help:    "Duration of event processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"pipeline", "source", "sink"},
	)
)
