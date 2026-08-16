package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hkjang/trace/internal/config"
	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/hkjang/trace/internal/database"
	"github.com/hkjang/trace/internal/httpapi"
	"github.com/hkjang/trace/internal/store"
	"github.com/hkjang/trace/internal/version"
	"github.com/hkjang/trace/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	vault, err := tracecrypto.NewVault(cfg.EncryptionKey)
	if err != nil {
		logger.Error("initialize encryption", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	db, err := database.Open(ctx, cfg.PostgresDSN)
	if err != nil {
		logger.Error("initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	dataStore := store.New(db, vault)
	if err := dataStore.Bootstrap(ctx, cfg.BootstrapAdmin, cfg.BootstrapAdminPassword); err != nil {
		logger.Error("bootstrap service", "error", err)
		os.Exit(1)
	}
	api := httpapi.New(dataStore, logger)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           api.Router(web.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		logger.Info("trace started", "address", cfg.ListenAddress, "version", version.Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
