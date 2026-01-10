package otel

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns an Echo middleware for distributed tracing
func TracingMiddleware(serviceName string) echo.MiddlewareFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			// Extract trace context from incoming request
			ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))

			// Start span
			spanName := c.Path()
			if spanName == "" {
				spanName = req.URL.Path
			}

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(req.Method),
					semconv.URLFull(req.URL.String()),
					semconv.URLPath(req.URL.Path),
					semconv.ServerAddress(req.Host),
					semconv.URLScheme(req.URL.Scheme),
					semconv.UserAgentOriginal(req.UserAgent()),
					attribute.String("http.request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				),
			)
			defer span.End()

			// Update request context
			c.SetRequest(req.WithContext(ctx))

			// Process request
			err := next(c)

			// Record response attributes
			statusCode := c.Response().Status
			span.SetAttributes(
				semconv.HTTPResponseStatusCode(statusCode),
				attribute.Int64("http.response_size", c.Response().Size),
			)

			// Set span status based on HTTP status code
			if statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}

			if err != nil {
				span.RecordError(err)
			}

			return err
		}
	}
}

// MetricsMiddleware returns an Echo middleware for metrics collection
func MetricsMiddleware(mp *MeterProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			ctx := c.Request().Context()

			// Increment active requests
			mp.IncrementActiveRequests(ctx)
			defer mp.DecrementActiveRequests(ctx)

			// Process request
			err := next(c)

			// Record metrics
			duration := time.Since(start)
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}

			mp.RecordRequest(ctx, c.Request().Method, path, c.Response().Status, duration)

			if err != nil {
				mp.RecordError(ctx, "http")
			}

			return err
		}
	}
}

// CombinedMiddleware returns a combined tracing and metrics middleware
func CombinedMiddleware(serviceName string, mp *MeterProvider) echo.MiddlewareFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			ctx := req.Context()

			// Extract trace context
			ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))

			// Start span
			spanName := c.Path()
			if spanName == "" {
				spanName = req.URL.Path
			}

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(req.Method),
					semconv.URLPath(req.URL.Path),
				),
			)
			defer span.End()

			// Update request context
			c.SetRequest(req.WithContext(ctx))

			// Increment active requests
			if mp != nil {
				mp.IncrementActiveRequests(ctx)
				defer mp.DecrementActiveRequests(ctx)
			}

			// Process request
			err := next(c)

			// Record span attributes
			statusCode := c.Response().Status
			span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))

			if statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}

			if err != nil {
				span.RecordError(err)
			}

			// Record metrics
			if mp != nil {
				duration := time.Since(start)
				mp.RecordRequest(ctx, req.Method, spanName, statusCode, duration)
				if err != nil {
					mp.RecordError(ctx, "http")
				}
			}

			return err
		}
	}
}

// DBTracingWrapper wraps database operations with tracing
type DBTracingWrapper struct {
	tracer trace.Tracer
	mp     *MeterProvider
}

// NewDBTracingWrapper creates a new database tracing wrapper
func NewDBTracingWrapper(serviceName string, mp *MeterProvider) *DBTracingWrapper {
	return &DBTracingWrapper{
		tracer: otel.Tracer(serviceName + "-db"),
		mp:     mp,
	}
}

// TraceQuery traces a database query
func (w *DBTracingWrapper) TraceQuery(ctx context.Context, operation, query string, fn func() error) error {
	start := time.Now()

	ctx, span := w.tracer.Start(ctx, "db."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			attribute.String("db.operation", operation),
			attribute.String("db.statement", truncateQuery(query, 1000)),
		),
	)
	defer span.End()

	err := fn()

	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}

	if w.mp != nil {
		w.mp.RecordDBQuery(ctx, operation, duration)
	}

	return err
}

// truncateQuery truncates a query to a maximum length
func truncateQuery(query string, maxLen int) string {
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}
