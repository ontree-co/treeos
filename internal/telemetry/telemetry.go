// Package telemetry provides OpenTelemetry instrumentation for the OnTree application.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// Config holds the telemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Endpoint       string
	Headers        map[string]string
	Insecure       bool
}

// Initialize sets up OpenTelemetry with the given configuration
func Initialize(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// Create resource without merging with Default() to avoid schema conflicts
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter options
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithHeaders(cfg.Headers),
		otlptracehttp.WithTimeout(10 * time.Second),
	}

	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	// Create exporter
	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	tracer = tp.Tracer(cfg.ServiceName)

	// Return cleanup function
	cleanup := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	return cleanup, nil
}

// InitializeFromEnv initializes OpenTelemetry using environment variables
func InitializeFromEnv(ctx context.Context) (func(context.Context) error, error) {
	serviceName := getEnvOrDefault("OTEL_SERVICE_NAME", "ontree-node")
	serviceVersion := getEnvOrDefault("OTEL_SERVICE_VERSION", "unknown")
	environment := getEnvOrDefault("OTEL_ENVIRONMENT", "development")

	// Determine endpoint and headers based on environment
	var endpoint string
	var headers map[string]string
	var insecure bool

	if honeycombKey := os.Getenv("HONEYCOMB_API_KEY"); honeycombKey != "" {
		// Honeycomb configuration
		endpoint = getEnvOrDefault("HONEYCOMB_ENDPOINT", "api.honeycomb.io")
		headers = map[string]string{
			"x-honeycomb-team": honeycombKey,
		}
		insecure = false
	} else {
		// Default to Jaeger for local development
		endpoint = getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")
		headers = nil
		insecure = true
	}

	cfg := Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    environment,
		Endpoint:       endpoint,
		Headers:        headers,
		Insecure:       insecure,
	}

	return Initialize(ctx, cfg)
}

// GetTracer returns the global tracer instance
func GetTracer() trace.Tracer {
	if tracer == nil {
		// Return a noop tracer if not initialized
		return otel.Tracer("ontree-node")
	}
	return tracer
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name, opts...)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
