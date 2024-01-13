package otelpackage

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	Tracer       trace.Tracer
	OtlpEndpoint string
)

type TraceData struct {
	TraceID      string `json:"traceID"`
	SpanID       string `json:"spanID"`
	RandomNumber int    `json:"randomNumber"`
	Sender       int    `json:"sender"`
}

func NewConsoleExporter() (oteltrace.SpanExporter, error) {
	return stdouttrace.New()
}

func NewOTLPExporter(ctx context.Context, otlpEndpoint string) (oteltrace.SpanExporter, error) {
	insecureOpt := otlptracehttp.WithInsecure()
	endpointOpt := otlptracehttp.WithEndpoint(otlpEndpoint)

	return otlptracehttp.New(ctx, insecureOpt, endpointOpt)
}

func NewTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("Random Number Generation Application"),
			attribute.String("service.name", "Otel POC"),
			attribute.String("library.language", "go"),
		),
	)

	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

func StartSpan(ctx context.Context, operationName string) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer("Random Number Generation Application").Start(ctx, operationName)
}
