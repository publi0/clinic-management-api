package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

type Config struct {
	Enabled     bool
	ServiceName string
}

func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	consoleHandler := slog.NewJSONHandler(os.Stdout, nil)

	if !cfg.Enabled {
		slog.SetDefault(slog.New(consoleHandler))
		return func(context.Context) error { return nil }, nil
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp metric exporter: %w", err)
	}

	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp log exporter: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithResource(res),
	)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	otelHandler := otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(loggerProvider))
	slog.SetDefault(slog.New(multiHandler{handlers: []slog.Handler{
		consoleHandler,
		otelHandler,
	}}))

	return func(shutdownCtx context.Context) error {
		var shutdownErr error
		shutdownErr = errors.Join(shutdownErr, loggerProvider.Shutdown(shutdownCtx))
		shutdownErr = errors.Join(shutdownErr, meterProvider.Shutdown(shutdownCtx))
		shutdownErr = errors.Join(shutdownErr, tracerProvider.Shutdown(shutdownCtx))
		return shutdownErr
	}, nil
}

type multiHandler struct {
	handlers []slog.Handler
}

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var handleErr error
	for _, handler := range h.handlers {
		handleErr = errors.Join(handleErr, handler.Handle(ctx, record.Clone()))
	}
	return handleErr
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithAttrs(attrs))
	}
	return multiHandler{handlers: next}
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithGroup(name))
	}
	return multiHandler{handlers: next}
}
