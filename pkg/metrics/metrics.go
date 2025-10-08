package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all the Prometheus metrics
type Metrics struct {
	// Message metrics
	MessagesTotal        *prometheus.CounterVec
	MessagesProcessed    *prometheus.CounterVec
	MessageProcessingDuration *prometheus.HistogramVec
	MessagesInQueue      prometheus.Gauge
	
	// Webhook metrics
	WebhookRequestsTotal    *prometheus.CounterVec
	WebhookRequestDuration  *prometheus.HistogramVec
	WebhookRetries          *prometheus.CounterVec
	
	// Database metrics
	DatabaseConnectionsActive prometheus.Gauge
	DatabaseQueryDuration     *prometheus.HistogramVec
	DatabaseQueriesTotal      *prometheus.CounterVec
	
	// Cache metrics
	CacheHitsTotal   *prometheus.CounterVec
	CacheMissesTotal *prometheus.CounterVec
	CacheOperationDuration *prometheus.HistogramVec
	
	// System metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	ActiveConnections    prometheus.Gauge
}

// New creates a new Metrics instance with all Prometheus metrics
func New() *Metrics {
	return NewWithRegistry(prometheus.DefaultRegisterer)
}

// NewWithRegistry creates a new Metrics instance with a custom registry
func NewWithRegistry(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		// Message metrics
		MessagesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_messages_total",
				Help: "Total number of messages processed by status",
			},
			[]string{"status"}, // pending, sent, failed
		),
		
		MessagesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_messages_processed_total",
				Help: "Total number of messages processed by result",
			},
			[]string{"result"}, // success, error
		),
		
		MessageProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "insider_messaging_message_processing_duration_seconds",
				Help:    "Time spent processing messages",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"}, // process, retry
		),
		
		MessagesInQueue: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "insider_messaging_messages_in_queue",
				Help: "Current number of messages in queue",
			},
		),
		
		// Webhook metrics
		WebhookRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_webhook_requests_total",
				Help: "Total number of webhook requests by status code",
			},
			[]string{"status_code"},
		),
		
		WebhookRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "insider_messaging_webhook_request_duration_seconds",
				Help:    "Time spent on webhook requests",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"status_code"},
		),
		
		WebhookRetries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_webhook_retries_total",
				Help: "Total number of webhook retry attempts",
			},
			[]string{"reason"}, // timeout, server_error, client_error
		),
		
		// Database metrics
		DatabaseConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "insider_messaging_database_connections_active",
				Help: "Number of active database connections",
			},
		),
		
		DatabaseQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "insider_messaging_database_query_duration_seconds",
				Help:    "Time spent on database queries",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"operation"}, // select, insert, update, delete
		),
		
		DatabaseQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_database_queries_total",
				Help: "Total number of database queries by operation and result",
			},
			[]string{"operation", "result"}, // operation: select/insert/update/delete, result: success/error
		),
		
		// Cache metrics
		CacheHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"operation"}, // get, set, delete
		),
		
		CacheMissesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"operation"}, // get
		),
		
		CacheOperationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "insider_messaging_cache_operation_duration_seconds",
				Help:    "Time spent on cache operations",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
			[]string{"operation"}, // get, set, delete
		),
		
		// System metrics
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "insider_messaging_http_requests_total",
				Help: "Total number of HTTP requests by method and status code",
			},
			[]string{"method", "status_code", "endpoint"},
		),
		
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "insider_messaging_http_request_duration_seconds",
				Help:    "Time spent on HTTP requests",
				Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"method", "endpoint"},
		),
		
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "insider_messaging_active_connections",
				Help: "Number of active HTTP connections",
			},
		),
	}
	
	// Register all metrics with Prometheus
	registerer.MustRegister(
		m.MessagesTotal,
		m.MessagesProcessed,
		m.MessageProcessingDuration,
		m.MessagesInQueue,
		m.WebhookRequestsTotal,
		m.WebhookRequestDuration,
		m.WebhookRetries,
		m.DatabaseConnectionsActive,
		m.DatabaseQueryDuration,
		m.DatabaseQueriesTotal,
		m.CacheHitsTotal,
		m.CacheMissesTotal,
		m.CacheOperationDuration,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.ActiveConnections,
	)
	
	return m
}

// Handler returns the Prometheus metrics HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{})
}

// RecordMessageProcessed records a processed message
func (m *Metrics) RecordMessageProcessed(result string, duration time.Duration) {
	m.MessagesProcessed.WithLabelValues(result).Inc()
	m.MessageProcessingDuration.WithLabelValues("process").Observe(duration.Seconds())
}

// RecordMessageStatus records message status change
func (m *Metrics) RecordMessageStatus(status string) {
	m.MessagesTotal.WithLabelValues(status).Inc()
}

// RecordWebhookRequest records a webhook request
func (m *Metrics) RecordWebhookRequest(statusCode string, duration time.Duration) {
	m.WebhookRequestsTotal.WithLabelValues(statusCode).Inc()
	m.WebhookRequestDuration.WithLabelValues(statusCode).Observe(duration.Seconds())
}

// RecordWebhookRetry records a webhook retry attempt
func (m *Metrics) RecordWebhookRetry(reason string) {
	m.WebhookRetries.WithLabelValues(reason).Inc()
}

// RecordDatabaseQuery records a database query
func (m *Metrics) RecordDatabaseQuery(operation, result string, duration time.Duration) {
	m.DatabaseQueriesTotal.WithLabelValues(operation, result).Inc()
	m.DatabaseQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(operation string, duration time.Duration) {
	m.CacheHitsTotal.WithLabelValues(operation).Inc()
	m.CacheOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(operation string) {
	m.CacheMissesTotal.WithLabelValues(operation).Inc()
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, statusCode, endpoint string, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, statusCode, endpoint).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// SetMessagesInQueue sets the current number of messages in queue
func (m *Metrics) SetMessagesInQueue(count float64) {
	m.MessagesInQueue.Set(count)
}

// SetDatabaseConnections sets the number of active database connections
func (m *Metrics) SetDatabaseConnections(count float64) {
	m.DatabaseConnectionsActive.Set(count)
}

// SetActiveConnections sets the number of active HTTP connections
func (m *Metrics) SetActiveConnections(count float64) {
	m.ActiveConnections.Set(count)
}