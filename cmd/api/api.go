package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"simple/internal/config"
	"simple/internal/router"
	"simple/internal/svc"
	"simple/pkg/chain"
	"simple/pkg/middleware"
)

func Run() error {
	fmt.Println("Run api server")

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	svc := svc.New(cfg)
	mux := http.NewServeMux()

	router.RegisterV1(mux, svc)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           chain.New(middleware.Logger, middleware.Recover).Then(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown for Ctrl+C / kill
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}
	slog.Info("http server listening", "addr", cfg.Addr, "env", cfg.Env)
	fmt.Println("\x1b[1;34mEverything is Simple ~\x1b[0m")

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("shutting down ...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func setupLogger(cfg config.Conf) {
	level := slog.LevelInfo
	if cfg.Env == "dev" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
