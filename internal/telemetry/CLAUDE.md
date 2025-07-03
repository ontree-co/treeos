# Telemetry Package

This package provides OpenTelemetry integration for distributed tracing in the onTree Node application.

## Configuration

The telemetry system supports two modes:
1. **Honeycomb** (Production): Set `HONEYCOMB_API_KEY` environment variable
2. **Jaeger** (Local Development): Default when no Honeycomb key is set

### Environment Variables

- `OTEL_SERVICE_NAME`: Service name (default: "ontree-node")
- `OTEL_SERVICE_VERSION`: Service version (default: "unknown")
- `OTEL_ENVIRONMENT`: Environment name (default: "development")
- `HONEYCOMB_API_KEY`: Honeycomb API key for production
- `HONEYCOMB_ENDPOINT`: Honeycomb endpoint (default: "api.honeycomb.io")
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint for Jaeger (default: "localhost:4318")

## Usage

### HTTP Request Tracing

All HTTP requests are automatically traced through the `TracingMiddleware` in the server package.

### Custom Spans

Docker operations are traced with custom spans:
- `docker.scan_apps`
- `docker.get_app_details`
- `docker.start_app`
- `docker.stop_app`
- `docker.recreate_app`
- `docker.delete_app`
- `docker.pull_images`

## Testing with Jaeger

1. Start Jaeger: `docker-compose -f jaeger-compose.yml up -d`
2. Run the server with default settings
3. Access Jaeger UI at http://localhost:16686
4. Look for "ontree-node" service in the service dropdown