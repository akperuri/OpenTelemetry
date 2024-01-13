package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"otelPOC/otelpackage"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	Propgator propagation.TextMapPropagator
	Carrier   propagation.MapCarrier
)

func GetPage(c *gin.Context) {
	Propgator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}) // The propogator is set to send this trace across other services

	ctx, span := otelpackage.StartSpan(c.Request.Context(), "Random Number Generation Span")
	span.AddEvent("Random Number Generation has begun")
	defer span.End()

	time.Sleep(1 * time.Second)
	randomNumber := GenerateRandomNumber(1, 10)
	span.SetAttributes(attribute.Bool("numberGenerated", true), attribute.Int("Number Generated", randomNumber), attribute.Int("Sender", 1))
	span.AddEvent("Random Number was generated")
	logrus.WithContext(ctx).WithField("random number", randomNumber).Infof("random number %d is generated", randomNumber)

	responseText := fmt.Sprintf("Random Number: %d", randomNumber)

	c.String(http.StatusOK, responseText)

	Carrier = propagation.MapCarrier{}
	Propgator.Inject(ctx, Carrier) // Injecting the parent span context into the Propogator

	SendTraceData(ctx, span, randomNumber)
}

func SendTraceData(ctx context.Context, span trace.Span, randomNumber int) {
	ctxSpan, span1 := otelpackage.Tracer.Start(ctx, "Send Traces Span")
	defer span1.End()
	span1.AddEvent("Sending Traces has begun")

	logrus.WithContext(ctxSpan).WithField("random number", randomNumber).Infof("random number %d is going to be sent", randomNumber)

	traceID := span.SpanContext().TraceID()

	traceData := otelpackage.TraceData{
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

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8080/receive", bytes.NewBuffer(jsonData))
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
