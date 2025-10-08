package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()
	
	m := NewWithRegistry(registry)
	
	if m == nil {
		t.Fatal("Expected metrics instance, got nil")
	}
	
	// Test that all metrics are initialized
	if m.MessagesTotal == nil {
		t.Error("MessagesTotal not initialized")
	}
	if m.MessagesProcessed == nil {
		t.Error("MessagesProcessed not initialized")
	}
	if m.MessageProcessingDuration == nil {
		t.Error("MessageProcessingDuration not initialized")
	}
	if m.MessagesInQueue == nil {
		t.Error("MessagesInQueue not initialized")
	}
	if m.WebhookRequestsTotal == nil {
		t.Error("WebhookRequestsTotal not initialized")
	}
	if m.WebhookRequestDuration == nil {
		t.Error("WebhookRequestDuration not initialized")
	}
	if m.WebhookRetries == nil {
		t.Error("WebhookRetries not initialized")
	}
	if m.DatabaseConnectionsActive == nil {
		t.Error("DatabaseConnectionsActive not initialized")
	}
	if m.DatabaseQueryDuration == nil {
		t.Error("DatabaseQueryDuration not initialized")
	}
	if m.DatabaseQueriesTotal == nil {
		t.Error("DatabaseQueriesTotal not initialized")
	}
	if m.CacheHitsTotal == nil {
		t.Error("CacheHitsTotal not initialized")
	}
	if m.CacheMissesTotal == nil {
		t.Error("CacheMissesTotal not initialized")
	}
	if m.CacheOperationDuration == nil {
		t.Error("CacheOperationDuration not initialized")
	}
	if m.HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal not initialized")
	}
	if m.HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration not initialized")
	}
	if m.ActiveConnections == nil {
		t.Error("ActiveConnections not initialized")
	}
}

func TestHandler(t *testing.T) {
	// Use default registry for this test since handler uses default gatherer
	m := New()
	
	// Record some metrics first to ensure they appear in output
	m.RecordMessageStatus("pending")
	
	handler := m.Handler()
	if handler == nil {
		t.Fatal("Expected HTTP handler, got nil")
	}
	
	// Test that handler serves metrics
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "insider_messaging") {
		t.Errorf("Expected metrics output to contain 'insider_messaging', got: %s", body)
	}
}

func TestRecordMessageProcessed(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	// Record a successful message processing
	duration := 100 * time.Millisecond
	m.RecordMessageProcessed("success", duration)
	
	// Check counter
	expected := `
		# HELP insider_messaging_messages_processed_total Total number of messages processed by result
		# TYPE insider_messaging_messages_processed_total counter
		insider_messaging_messages_processed_total{result="success"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(expected), "insider_messaging_messages_processed_total"); err != nil {
		t.Errorf("Unexpected metric value: %v", err)
	}
	
	// Check histogram
	histogramExpected := `
		# HELP insider_messaging_message_processing_duration_seconds Time spent processing messages
		# TYPE insider_messaging_message_processing_duration_seconds histogram
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.005"} 0
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.01"} 0
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.025"} 0
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.05"} 0
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.1"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.25"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="0.5"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="1"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="2.5"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="5"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="10"} 1
		insider_messaging_message_processing_duration_seconds_bucket{operation="process",le="+Inf"} 1
		insider_messaging_message_processing_duration_seconds_sum{operation="process"} 0.1
		insider_messaging_message_processing_duration_seconds_count{operation="process"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(histogramExpected), "insider_messaging_message_processing_duration_seconds"); err != nil {
		t.Errorf("Unexpected histogram metric value: %v", err)
	}
}

func TestRecordMessageStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	m.RecordMessageStatus("pending")
	m.RecordMessageStatus("sent")
	m.RecordMessageStatus("failed")
	
	expected := `
		# HELP insider_messaging_messages_total Total number of messages processed by status
		# TYPE insider_messaging_messages_total counter
		insider_messaging_messages_total{status="failed"} 1
		insider_messaging_messages_total{status="pending"} 1
		insider_messaging_messages_total{status="sent"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(expected), "insider_messaging_messages_total"); err != nil {
		t.Errorf("Unexpected metric value: %v", err)
	}
}

func TestRecordWebhookRequest(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	duration := 250 * time.Millisecond
	m.RecordWebhookRequest("200", duration)
	
	// Check counter
	counterExpected := `
		# HELP insider_messaging_webhook_requests_total Total number of webhook requests by status code
		# TYPE insider_messaging_webhook_requests_total counter
		insider_messaging_webhook_requests_total{status_code="200"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(counterExpected), "insider_messaging_webhook_requests_total"); err != nil {
		t.Errorf("Unexpected counter metric value: %v", err)
	}
	
	// Check histogram
	histogramExpected := `
		# HELP insider_messaging_webhook_request_duration_seconds Time spent on webhook requests
		# TYPE insider_messaging_webhook_request_duration_seconds histogram
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="0.1"} 0
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="0.25"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="0.5"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="1"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="2.5"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="5"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="10"} 1
		insider_messaging_webhook_request_duration_seconds_bucket{status_code="200",le="+Inf"} 1
		insider_messaging_webhook_request_duration_seconds_sum{status_code="200"} 0.25
		insider_messaging_webhook_request_duration_seconds_count{status_code="200"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(histogramExpected), "insider_messaging_webhook_request_duration_seconds"); err != nil {
		t.Errorf("Unexpected histogram metric value: %v", err)
	}
}

func TestRecordWebhookRetry(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	m.RecordWebhookRetry("timeout")
	m.RecordWebhookRetry("server_error")
	
	expected := `
		# HELP insider_messaging_webhook_retries_total Total number of webhook retry attempts
		# TYPE insider_messaging_webhook_retries_total counter
		insider_messaging_webhook_retries_total{reason="server_error"} 1
		insider_messaging_webhook_retries_total{reason="timeout"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(expected), "insider_messaging_webhook_retries_total"); err != nil {
		t.Errorf("Unexpected metric value: %v", err)
	}
}

func TestRecordDatabaseQuery(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	duration := 5 * time.Millisecond
	m.RecordDatabaseQuery("select", "success", duration)
	
	// Check counter
	counterExpected := `
		# HELP insider_messaging_database_queries_total Total number of database queries by operation and result
		# TYPE insider_messaging_database_queries_total counter
		insider_messaging_database_queries_total{operation="select",result="success"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(counterExpected), "insider_messaging_database_queries_total"); err != nil {
		t.Errorf("Unexpected counter metric value: %v", err)
	}
	
	// Check histogram
	histogramExpected := `
		# HELP insider_messaging_database_query_duration_seconds Time spent on database queries
		# TYPE insider_messaging_database_query_duration_seconds histogram
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.001"} 0
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.005"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.01"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.05"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.1"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.25"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="0.5"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="1"} 1
		insider_messaging_database_query_duration_seconds_bucket{operation="select",le="+Inf"} 1
		insider_messaging_database_query_duration_seconds_sum{operation="select"} 0.005
		insider_messaging_database_query_duration_seconds_count{operation="select"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(histogramExpected), "insider_messaging_database_query_duration_seconds"); err != nil {
		t.Errorf("Unexpected histogram metric value: %v", err)
	}
}

func TestRecordCacheOperations(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	duration := 2 * time.Millisecond
	m.RecordCacheHit("get", duration)
	m.RecordCacheMiss("get")
	
	// Check cache hits
	hitsExpected := `
		# HELP insider_messaging_cache_hits_total Total number of cache hits
		# TYPE insider_messaging_cache_hits_total counter
		insider_messaging_cache_hits_total{operation="get"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(hitsExpected), "insider_messaging_cache_hits_total"); err != nil {
		t.Errorf("Unexpected cache hits metric value: %v", err)
	}
	
	// Check cache misses
	missesExpected := `
		# HELP insider_messaging_cache_misses_total Total number of cache misses
		# TYPE insider_messaging_cache_misses_total counter
		insider_messaging_cache_misses_total{operation="get"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(missesExpected), "insider_messaging_cache_misses_total"); err != nil {
		t.Errorf("Unexpected cache misses metric value: %v", err)
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	duration := 50 * time.Millisecond
	m.RecordHTTPRequest("POST", "200", "/api/messages", duration)
	
	// Check counter
	counterExpected := `
		# HELP insider_messaging_http_requests_total Total number of HTTP requests by method and status code
		# TYPE insider_messaging_http_requests_total counter
		insider_messaging_http_requests_total{endpoint="/api/messages",method="POST",status_code="200"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(counterExpected), "insider_messaging_http_requests_total"); err != nil {
		t.Errorf("Unexpected counter metric value: %v", err)
	}
	
	// Check histogram
	histogramExpected := `
		# HELP insider_messaging_http_request_duration_seconds Time spent on HTTP requests
		# TYPE insider_messaging_http_request_duration_seconds histogram
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.01"} 0
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.025"} 0
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.05"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.1"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.25"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="0.5"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="1"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="2.5"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="5"} 1
		insider_messaging_http_request_duration_seconds_bucket{endpoint="/api/messages",method="POST",le="+Inf"} 1
		insider_messaging_http_request_duration_seconds_sum{endpoint="/api/messages",method="POST"} 0.05
		insider_messaging_http_request_duration_seconds_count{endpoint="/api/messages",method="POST"} 1
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(histogramExpected), "insider_messaging_http_request_duration_seconds"); err != nil {
		t.Errorf("Unexpected histogram metric value: %v", err)
	}
}

func TestGaugeMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewWithRegistry(registry)
	
	// Test setting gauge values
	m.SetMessagesInQueue(42)
	m.SetDatabaseConnections(10)
	m.SetActiveConnections(5)
	
	// Check messages in queue
	queueExpected := `
		# HELP insider_messaging_messages_in_queue Current number of messages in queue
		# TYPE insider_messaging_messages_in_queue gauge
		insider_messaging_messages_in_queue 42
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(queueExpected), "insider_messaging_messages_in_queue"); err != nil {
		t.Errorf("Unexpected queue metric value: %v", err)
	}
	
	// Check database connections
	dbExpected := `
		# HELP insider_messaging_database_connections_active Number of active database connections
		# TYPE insider_messaging_database_connections_active gauge
		insider_messaging_database_connections_active 10
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(dbExpected), "insider_messaging_database_connections_active"); err != nil {
		t.Errorf("Unexpected database connections metric value: %v", err)
	}
	
	// Check active connections
	activeExpected := `
		# HELP insider_messaging_active_connections Number of active HTTP connections
		# TYPE insider_messaging_active_connections gauge
		insider_messaging_active_connections 5
	`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(activeExpected), "insider_messaging_active_connections"); err != nil {
		t.Errorf("Unexpected active connections metric value: %v", err)
	}
}