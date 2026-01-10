package otel

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/pixperk/goiler/internal/config"
)

// MeterProvider wraps the OpenTelemetry meter provider
type MeterProvider struct {
	provider *sdkmetric.MeterProvider
	meter    metric.Meter
	logger   *slog.Logger

	// Pre-defined metrics
	RequestCounter   metric.Int64Counter
	RequestDuration  metric.Float64Histogram
	ActiveRequests   metric.Int64UpDownCounter
	ErrorCounter     metric.Int64Counter
	DBQueryDuration  metric.Float64Histogram
	CacheHits        metric.Int64Counter
	CacheMisses      metric.Int64Counter
}

// NewMeterProvider creates a new meter provider with Prometheus exporter
func NewMeterProvider(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*MeterProvider, error) {
	if !cfg.OTEL.Enabled {
		logger.Info("OpenTelemetry metrics disabled")
		return &MeterProvider{
			meter:  otel.Meter(cfg.OTEL.ServiceName),
			logger: logger,
		}, nil
	}

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	// Set global meter provider
	otel.SetMeterProvider(provider)

	meter := provider.Meter(cfg.OTEL.ServiceName)

	mp := &MeterProvider{
		provider: provider,
		meter:    meter,
		logger:   logger,
	}

	// Initialize metrics
	if err := mp.initMetrics(); err != nil {
		return nil, err
	}

	logger.Info("OpenTelemetry metrics initialized")

	return mp, nil
}

// initMetrics initializes all pre-defined metrics
func (mp *MeterProvider) initMetrics() error {
	var err error

	mp.RequestCounter, err = mp.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mp.RequestDuration, err = mp.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mp.ActiveRequests, err = mp.meter.Int64UpDownCounter(
		"http_requests_active",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mp.ErrorCounter, err = mp.meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mp.DBQueryDuration, err = mp.meter.Float64Histogram(
		"db_query_duration_seconds",
		metric.WithDescription("Database query latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mp.CacheHits, err = mp.meter.Int64Counter(
		"cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mp.CacheMisses, err = mp.meter.Int64Counter(
		"cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Register runtime metrics
	mp.registerRuntimeMetrics()

	return nil
}

// registerRuntimeMetrics registers Go runtime metrics
func (mp *MeterProvider) registerRuntimeMetrics() {
	// Memory metrics
	mp.meter.Int64ObservableGauge(
		"go_memstats_alloc_bytes",
		metric.WithDescription("Number of bytes allocated and still in use"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			observer.Observe(int64(m.Alloc))
			return nil
		}),
	)

	mp.meter.Int64ObservableGauge(
		"go_goroutines",
		metric.WithDescription("Number of goroutines"),
		metric.WithUnit("1"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			observer.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	)
}

// Meter returns the meter instance
func (mp *MeterProvider) Meter() metric.Meter {
	return mp.meter
}

// Shutdown shuts down the meter provider
func (mp *MeterProvider) Shutdown(ctx context.Context) error {
	if mp.provider != nil {
		return mp.provider.Shutdown(ctx)
	}
	return nil
}

// RecordRequest records an HTTP request metric
func (mp *MeterProvider) RecordRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	mp.RequestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	mp.RequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordError records an error metric
func (mp *MeterProvider) RecordError(ctx context.Context, errorType string) {
	mp.ErrorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", errorType),
	))
}

// RecordDBQuery records a database query metric
func (mp *MeterProvider) RecordDBQuery(ctx context.Context, operation string, duration time.Duration) {
	mp.DBQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("operation", operation),
	))
}

// RecordCacheHit records a cache hit
func (mp *MeterProvider) RecordCacheHit(ctx context.Context, cache string) {
	mp.CacheHits.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache", cache),
	))
}

// RecordCacheMiss records a cache miss
func (mp *MeterProvider) RecordCacheMiss(ctx context.Context, cache string) {
	mp.CacheMisses.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache", cache),
	))
}

// IncrementActiveRequests increments active request count
func (mp *MeterProvider) IncrementActiveRequests(ctx context.Context) {
	mp.ActiveRequests.Add(ctx, 1)
}

// DecrementActiveRequests decrements active request count
func (mp *MeterProvider) DecrementActiveRequests(ctx context.Context) {
	mp.ActiveRequests.Add(ctx, -1)
}

// MetricsHandler returns an HTTP handler for Prometheus metrics
func MetricsHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		// The promhttp.Handler() would be used here in a real implementation
		// For now, we return a placeholder
		return c.String(http.StatusOK, "# Metrics endpoint - configure with promhttp.Handler()")
	}
}
