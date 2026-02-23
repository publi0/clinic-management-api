package main

import (
	"context"
	"log/slog"
	"strings"

	"capim-test/internal/config"
	"capim-test/internal/db"
	httpapi "capim-test/internal/http"
	"capim-test/internal/service"
	"capim-test/internal/telemetry"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		return
	}

	shutdownTelemetry, err := telemetry.Setup(ctx, telemetry.Config{
		Enabled:     cfg.OTelEnabled,
		ServiceName: cfg.OTelServiceName,
	})
	if err != nil {
		slog.Error("setup telemetry", "error", err)
		return
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			slog.Error("shutdown telemetry", "error", err)
		}
	}()

	database, err := db.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		return
	}
	defer database.Close()

	svc := service.New(
		database,
		service.WithAuthConfig(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTokenTTL),
	)
	bootstrapEmail := strings.TrimSpace(cfg.BootstrapUserEmail)
	bootstrapPassword := strings.TrimSpace(cfg.BootstrapUserPassword)
	if bootstrapEmail != "" || bootstrapPassword != "" {
		if bootstrapEmail == "" || bootstrapPassword == "" {
			slog.Error("bootstrap user requires both AUTH_BOOTSTRAP_EMAIL and AUTH_BOOTSTRAP_PASSWORD")
			return
		}
		if err := svc.EnsureUser(ctx, bootstrapEmail, bootstrapPassword); err != nil {
			slog.Error("ensure bootstrap user", "error", err)
			return
		}
		slog.Info("bootstrap user ensured", "email", bootstrapEmail)
	}

	router := httpapi.NewRouter(svc, cfg.OTelServiceName)

	slog.Info("api listening", "port", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		slog.Error("run api", "error", err)
	}
}
