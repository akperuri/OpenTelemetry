package receiver

import (
	"context"
	"fmt"
	"net/http"
	"otelPOC/otelpackage"
	"otelPOC/server"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

func ReceiveTrace(c *gin.Context) {
	parentCtx := server.Propgator.Extract(context.Background(), server.Carrier) // Extracting the server trace here to be used
	_, span := otelpackage.StartSpan(parentCtx, "Random Number Receiver Span")
	span.AddEvent("Random Number Receiving has begun")
	defer span.End()

	var traceData otelpackage.TraceData

	err := c.ShouldBindJSON(&traceData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to decode trace data: %v", err)})
		span.SetAttributes(attribute.Bool("numberReceived", false), attribute.Int("Number Received", -1), attribute.Int("Received From", traceData.Sender))
		span.AddEvent("Failed to decode trace data")
		return
	}

	fmt.Printf("\nReceived Trace Data:\n"+
		"TraceID: %s\n"+
		"SpanID: %s\n"+
		"Random Number: %d\n"+
		"Sender: %d\n", traceData.TraceID, traceData.SpanID, traceData.RandomNumber, traceData.Sender)

	span.SetAttributes(attribute.Bool("numberReceived", true), attribute.Int("Number Received", traceData.RandomNumber), attribute.Int("Received From", traceData.Sender))
	span.AddEvent("Random Number was received")
	c.JSON(http.StatusOK, gin.H{"message": "Trace data received successfully"})
}
