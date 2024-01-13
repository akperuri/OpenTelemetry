package main

import (
	"context"
	"log"
	"os"
	"otelPOC/otelpackage"
	"otelPOC/receiver"
	"otelPOC/server"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
)

func init() {

	otelpackage.OtlpEndpoint = os.Getenv("OTLP_ENDPOINT")
	if otelpackage.OtlpEndpoint == "" {
		otelpackage.OtlpEndpoint = "localhost:4318"
	}
}

func main() {
	ctx := context.Background()

	exp, err := otelpackage.NewOTLPExporter(ctx, otelpackage.OtlpEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize exporter: %v", err)
	}

	tp := otelpackage.NewTraceProvider(exp)
	defer func() { _ = tp.Shutdown(ctx) }()

	otel.SetTracerProvider(tp)
	otelpackage.Tracer = otel.GetTracerProvider().Tracer("Random Number Generator Trace")

	r := gin.Default()
	r.Use(otelgin.Middleware("Random Number Generation Application"))

	logger := logrus.New()
	logger.SetReportCaller(true)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logrus.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
	)))

	r.GET("/generate", server.GetPage)
	r.POST("/receive", receiver.ReceiveTrace)
	r.Run(":8080")
}
