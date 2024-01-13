package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer       trace.Tracer
	otlpEndpoint string
)

func init() {
	otlpEndpoint = os.Getenv("OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "localhost:4318"
	}
}

type TraceData struct {
	TraceID      string `json:"traceID"`
	SpanID       string `json:"spanID"`
	RandomNumber int    `json:"randomNumber"`
	Sender       int    `json:"sender"`
}

func newOTLPExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
	// Change default HTTPS -> HTTP
	insecureOpt := otlptracehttp.WithInsecure()

	endpointOpt := otlptracehttp.WithEndpoint(otlpEndpoint)

	return otlptracehttp.New(ctx, insecureOpt, endpointOpt)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("Random Number Receiver Application"),
			attribute.String("service.name", "Otel POC Receiver"),
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

func main() {
	ctx := context.Background()

	exp, err := newOTLPExporter(ctx)
	if err != nil {
		log.Fatalf("failed to initialize exporter: %v", err)
	}

	tp := newTraceProvider(exp)
	defer func() { _ = tp.Shutdown(ctx) }()

	otel.SetTracerProvider(tp)

	router := gin.Default()
	router.Use(otelgin.Middleware("microservice-2"))
	router.POST("/receive", Receive)
	router.GET("/receive", Pong)
	router.Run(":8081")
}

func Receive(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(c.Request.Header)))
	log.Printf("Received GET Receive request. TraceID: %s, SpanID: %s", span.SpanContext().TraceID(), span.SpanContext().SpanID())
	defer span.End()

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	receivedRandomNumber, err := strconv.Atoi(string(body))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse random number"})
		return
	}

	span.AddEvent("Random Number was received")

	logrus.WithContext(ctx).Info("random number was received!")

	fmt.Printf("Received Random Number: %d\n", receivedRandomNumber)
}

// func Receive(c *gin.Context) {
// 	ctx := c.Request.Context()
// 	span := trace.SpanFromContext(otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(c.Request.Header)))
// 	defer span.End()
// 	var traceData TraceData

// 	log.Printf("Received GET Receive request. TraceID: %s, SpanID: %s", span.SpanContext().TraceID(), span.SpanContext().SpanID())
// 	err := c.ShouldBindJSON(&traceData)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to decode trace data: %v", err)})
// 		span.SetAttributes(attribute.Bool("numberReceived", false), attribute.Int("Number Received", -1), attribute.Int("Received From", traceData.Sender))
// 		span.AddEvent("Failed to decode trace data")
// 		return
// 	}

// 	// fmt.Printf("\nReceived Trace Data:\n"+
// 	// 	"TraceID: %s\n"+
// 	// 	"SpanID: %s\n"+
// 	// 	"Random Number: %d\n"+
// 	// 	"Sender: %d\n", traceData.TraceID, traceData.SpanID, traceData.RandomNumber, traceData.Sender)

// 	span.SetAttributes(attribute.Bool("numberReceived", true), attribute.Int("Number Received", traceData.RandomNumber), attribute.Int("Received From", traceData.Sender))
// 	span.AddEvent("Random Number was received")
// 	c.JSON(http.StatusOK, gin.H{"message": "Trace data received successfully"})
// }

func Pong(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(c.Request.Header)))

	defer span.End()

	span.AddEvent("Pong")

	log.Printf("Received GET Pong request. TraceID: %s, SpanID: %s", span.SpanContext().TraceID(), span.SpanContext().SpanID())

}
