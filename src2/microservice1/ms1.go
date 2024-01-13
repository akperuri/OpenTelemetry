package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
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
			attribute.String("service.name", "Otel POC Sender"),
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

type Result struct {
	Message string
}

type TraceData struct {
	TraceID      string `json:"traceID"`
	SpanID       string `json:"spanID"`
	RandomNumber int    `json:"randomNumber"`
	Sender       int    `json:"sender"`
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
	// tracer = otel.GetTracerProvider().Tracer("Number Receiver Trace")
	// resty := resty.New()

	router := gin.Default()
	router.Use(otelgin.Middleware("microservice-1"))
	router.GET("/generate", GetPage)
	router.Run(":8080")
}

func GetPage(c *gin.Context) {
	// result := Result{}
	resty := resty.New()
	req := resty.R().SetHeader("Content-Type", "application/json")
	req.SetContext(c.Request.Context())
	ctx := req.Context()
	span := trace.SpanFromContext(ctx)

	defer span.End()

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	span.AddEvent("Random Number Generation has begun")
	time.Sleep(1 * time.Second)
	randomNumber := GenerateRandomNumber(1, 10)
	span.SetAttributes(attribute.Bool("numberGenerated", true), attribute.Int("Number Generated", randomNumber), attribute.Int("Sender", 1))
	span.AddEvent("Random Number was generated")
	logrus.WithContext(ctx).WithField("random number", randomNumber).Infof("random number %d is generated", randomNumber)

	responseText := fmt.Sprintf("Random Number: %d", randomNumber)

	c.String(http.StatusOK, responseText)

	// SendTraceData(ctx, span, randomNumber)

	_, _ = req.Get("http://localhost:8081/receive")

	resp, err := req.SetBody([]byte(fmt.Sprintf("%d", randomNumber))).Post("http://localhost:8081/receive")
	if err != nil {
		// Handle the error
		return
	}

	// Check the response status code
	if resp.StatusCode() != http.StatusOK {
		// Handle non-OK status code
		return
	}
	resp.RawResponse.Body.Close()
	log.Printf("Generate request. TraceID: %s, SpanID: %s", span.SpanContext().TraceID(), span.SpanContext().SpanID())

}

func SendTraceData(ctx context.Context, span trace.Span, randomNumber int) {

	span1 := trace.SpanFromContext(ctx)
	ctxSpan := ctx
	defer span1.End()
	span1.AddEvent("Sending Traces has begun")

	logrus.WithContext(ctxSpan).WithField("random number", randomNumber).Infof("random number %d is going to be sent", randomNumber)

	traceID := span.SpanContext().TraceID()

	traceData := TraceData{
		TraceID:      traceID.String(),
		SpanID:       span.SpanContext().SpanID().String(),
		RandomNumber: randomNumber,
		Sender:       1,
	}

	jsonData, err := json.Marshal(traceData)
	if err != nil {
		span1.AddEvent("Traces sending Failed! -  Failed to Marshal JSON")
		span1.SetAttributes(attribute.Bool("Traces Sent", false), attribute.Int("Sender", 1))
		log.Printf("Failed to marshal trace data: %v", err)
		logrus.WithContext(ctxSpan).Errorf("Failed to marshal trace data: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8081/receive", bytes.NewBuffer(jsonData))
	if err != nil {
		span1.AddEvent("Traces sending Failed! -  Failed to create request")
		span1.SetAttributes(attribute.Bool("Traces Sent", false), attribute.Int("Sender", 1))
		log.Printf("Failed to create request: %v", err)
		logrus.WithContext(ctxSpan).Errorf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		span1.AddEvent("Traces sending Failed!")
		span1.SetAttributes(attribute.Bool("Traces Sent", false), attribute.Int("Sender", 1))
		log.Printf("Failed to send trace data: %v", err)
		logrus.WithContext(ctxSpan).Errorf("Failed to send trace data: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Received non-OK status code: %d", resp.StatusCode)
		span1.AddEvent("Received non-OK status code")
		span1.SetAttributes(attribute.Bool("Traces Sent", false), attribute.Int("Status Code", resp.StatusCode), attribute.Int("Sender", 1))
		logrus.WithContext(ctxSpan).Errorf("Received non-OK status code: %d", resp.StatusCode)
		return
	}

	log.Printf("Trace data sent successfully")
	span1.AddEvent("Traces were sent!")

	logrus.WithContext(ctxSpan).Infof("random number %d was sent!", randomNumber)

	span1.SetAttributes(attribute.Bool("Traces Sent", true), attribute.Int("Status Code", resp.StatusCode), attribute.Int("Sender", 1))
}

func GenerateRandomNumber(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}
