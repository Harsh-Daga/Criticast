package main

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func initTracing() (func(context.Context) error, error) {
	w := os.Stdout
	if path := os.Getenv("OTEL_TRACE_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open otel trace file: %w", err)
		}
		w = f
	}
	exp, err := stdouttrace.New(stdouttrace.WithWriter(w))
	if err != nil {
		return nil, fmt.Errorf("stdout trace exporter: %w", err)
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("p0b-adversarial"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("github.com/criticast/criticast/testdata/p0b/server")
	return tp.Shutdown, nil
}

func startSpan(ctx context.Context, name, token string) (context.Context, trace.Span) {
	ctx, sp := tracer.Start(ctx, name,
		trace.WithAttributes(attribute.String("request.token", token)))
	return ctx, sp
}

func endSpan(sp trace.Span, err error) {
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
	}
	sp.End()
}
