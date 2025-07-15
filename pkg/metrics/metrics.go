package metrics

import (
  "github.com/prometheus/client_golang/prometheus"
  "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
  // Ingest metrics
  IngestCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_ingest_events_total",
      Help: "Total raw events ingested",
    })
  IngestErrors = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_ingest_errors_total",
      Help: "Raw ingest errors",
    })
  IngestLatency = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "pipeline_ingest_latency_seconds",
      Help:    "Time to ingest one event",
      Buckets: prometheus.DefBuckets,
    })

  // Normalize metrics
  NormalizeLatency = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "pipeline_normalize_latency_seconds",
      Help:    "Time to normalize one event",
      Buckets: prometheus.DefBuckets,
    })
  NormalizeErrors = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_normalize_errors_total",
      Help: "Normalization errors",
    })
  NormalizeCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_normalize_events_total",
      Help: "Total events normalized",
    })

  // Cache/Pub metrics
  CachePubErrors = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_cachepub_errors_total",
      Help: "Cache/Pub/Sub errors",
    })
  CachePubCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_cachepub_events_total",
      Help: "Total cache/pub events processed",
    })
  CachePubLatency = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "pipeline_cachepub_latency_seconds",
      Help:    "Time to process cache/pub event",
      Buckets: prometheus.DefBuckets,
    })

  // Anomaly metrics
  AnomalyErrors = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_anomaly_errors_total",
      Help: "Anomaly detection errors",
    })
  AnomalyCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_anomaly_events_total",
      Help: "Total anomalies detected",
    })
  AnomalyLatency = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "pipeline_anomaly_latency_seconds",
      Help:    "Time to detect anomaly",
      Buckets: prometheus.DefBuckets,
    })

  // Archival metrics
  ArchivalSuccessCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_archival_success_total",
      Help: "Total successful archival operations",
    })
  ArchivalErrorCounter = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "pipeline_archival_errors_total",
      Help: "Total archival errors",
    })
  ArchivalLatency = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "pipeline_archival_latency_seconds",
      Help:    "Time to archive data",
      Buckets: prometheus.DefBuckets,
    })

  // API metrics
  APIRequestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
      Name:    "api_request_duration_seconds",
      Help:    "API request duration",
      Buckets: prometheus.DefBuckets,
    },
    []string{"method", "endpoint", "status"},
  )
  APIRequestTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "api_requests_total",
      Help: "Total API requests",
    },
    []string{"method", "endpoint", "status"},
  )

  // Redis metrics
  RedisOperationDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
      Name:    "redis_operation_duration_seconds",
      Help:    "Redis operation duration",
      Buckets: prometheus.DefBuckets,
    },
    []string{"operation", "status"},
  )
  RedisErrors = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "redis_errors_total",
      Help: "Total Redis errors",
    },
    []string{"operation"},
  )

  // Database metrics
  DatabaseHealthCheckDuration = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "database_health_check_duration_seconds",
      Help:    "Database health check duration",
      Buckets: prometheus.DefBuckets,
    })
  DatabaseHealthCheckSuccess = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "database_health_check_success_total",
      Help: "Total successful database health checks",
    })
  DatabaseHealthCheckErrors = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "database_health_check_errors_total",
      Help: "Total database health check errors",
    })
  DatabaseOperationDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
      Name:    "database_operation_duration_seconds",
      Help:    "Database operation duration",
      Buckets: prometheus.DefBuckets,
    },
    []string{"operation", "status"},
  )
  DatabaseOperations = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "database_operations_total",
      Help: "Total database operations",
    },
    []string{"operation", "status"},
  )
  DatabaseErrors = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "database_errors_total",
      Help: "Total database errors",
    },
    []string{"operation"},
  )

  // Authentication metrics
  AuthOperationDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
      Name:    "auth_operation_duration_seconds",
      Help:    "Authentication operation duration",
      Buckets: prometheus.DefBuckets,
    },
    []string{"operation", "status"},
  )
  AuthOperations = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "auth_operations_total",
      Help: "Total authentication operations",
    },
    []string{"operation", "status"},
  )
  AuthErrors = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "auth_errors_total",
      Help: "Total authentication errors",
    },
    []string{"operation"},
  )
  AuthMiddlewareDuration = prometheus.NewHistogram(
    prometheus.HistogramOpts{
      Name:    "auth_middleware_duration_seconds",
      Help:    "Authentication middleware duration",
      Buckets: prometheus.DefBuckets,
    })
  AuthMiddlewareSuccess = prometheus.NewCounter(
    prometheus.CounterOpts{
      Name: "auth_middleware_success_total",
      Help: "Total successful authentication middleware calls",
    })
  AuthMiddlewareErrors = prometheus.NewCounterVec(
    prometheus.CounterOpts{
      Name: "auth_middleware_errors_total",
      Help: "Total authentication middleware errors",
    },
    []string{"error_type"},
  )

  // System metrics
  ActiveConnections = prometheus.NewGauge(
    prometheus.GaugeOpts{
      Name: "system_active_connections",
      Help: "Number of active connections",
    })
  MemoryUsage = prometheus.NewGauge(
    prometheus.GaugeOpts{
      Name: "system_memory_usage_bytes",
      Help: "Current memory usage in bytes",
    })
  Goroutines = prometheus.NewGauge(
    prometheus.GaugeOpts{
      Name: "system_goroutines",
      Help: "Number of active goroutines",
    })
)

func init() {
  // MustRegister panics if registration fails (e.g. duplicate)
  prometheus.MustRegister(
    IngestCounter, IngestErrors, IngestLatency,
    NormalizeLatency, NormalizeErrors, NormalizeCounter,
    CachePubErrors, CachePubCounter, CachePubLatency,
    AnomalyErrors, AnomalyCounter, AnomalyLatency,
    ArchivalSuccessCounter, ArchivalErrorCounter, ArchivalLatency,
    APIRequestDuration, APIRequestTotal,
    RedisOperationDuration, RedisErrors,
    DatabaseHealthCheckDuration, DatabaseHealthCheckSuccess, DatabaseHealthCheckErrors,
    DatabaseOperationDuration, DatabaseOperations, DatabaseErrors,
    AuthOperationDuration, AuthOperations, AuthErrors,
    AuthMiddlewareDuration, AuthMiddlewareSuccess, AuthMiddlewareErrors,
    ActiveConnections, MemoryUsage, Goroutines,
  )
}
