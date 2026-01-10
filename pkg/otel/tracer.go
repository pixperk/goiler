package otel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/pixperk/goiler/internal/config"
)

// TracerProvider wraps the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	logger   *slog.Logger
}

// NewTracerProvider creates a new tracer provider
func NewTracerProvider(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*TracerProvider, error) {
	if !cfg.OTEL.Enabled {
		logger.Info("OpenTelemetry tracing disabled")
		return &TracerProvider{
			tracer: otel.Tracer(cfg.OTEL.ServiceName),
			logger: logger,
		}, nil
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTEL.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.OTEL.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", cfg.App.Env),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry tracing initialized",
		slog.String("endpoint", cfg.OTEL.Endpoint),
		slog.String("service", cfg.OTEL.ServiceName),
	)

	return &TracerProvider{
		provider: provider,
		tracer:   provider.Tracer(cfg.OTEL.ServiceName),
		logger:   logger,
	}, nil
}

// Tracer returns the tracer instance
func (tp *TracerProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// Shutdown shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new span with the given name
func (tp *TracerProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tp.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// WithSpan starts a span and executes the function
func WithSpan(ctx context.Context, tracer trace.Tracer, name string, fn func(ctx context.Context) error, opts ...trace.SpanStartOption) error {
	ctx, span := tracer.Start(ctx, name, opts...)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		RecordError(span, err)
	}

	return err
}

// RecordError records an error on the span
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

// SetSpanAttributes sets attributes on the current span
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// GetTraceID returns the trace ID from context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().TraceID().String()
}

// GetSpanID returns the span ID from context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().SpanID().String()
}

// Common attribute helpers
func UserIDAttr(userID string) attribute.KeyValue {
	return attribute.String("user.id", userID)
}

func HTTPMethodAttr(method string) attribute.KeyValue {
	return attribute.String("http.method", method)
}

func HTTPPathAttr(path string) attribute.KeyValue {
	return attribute.String("http.path", path)
}

func HTTPStatusCodeAttr(code int) attribute.KeyValue {
	return attribute.Int("http.status_code", code)
}

func DBQueryAttr(query string) attribute.KeyValue {
	return attribute.String("db.query", query)
}

func DBOperationAttr(operation string) attribute.KeyValue {
	return attribute.String("db.operation", operation)
}
