package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dangy/pr-reviewer-assignment-service/internal/config"
	"github.com/dangy/pr-reviewer-assignment-service/internal/http/handlers"
	"github.com/dangy/pr-reviewer-assignment-service/internal/repository"
	"github.com/dangy/pr-reviewer-assignment-service/internal/service"
	"github.com/dangy/pr-reviewer-assignment-service/internal/storage/schema"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	if err := schema.Ensure(ctx, pool); err != nil {
		log.Fatalf("failed to ensure schema: %v", err)
	}

	repo := repository.New(pool)
	svc := service.New(repo)
	handler := handlers.New(svc, cfg.AdminToken, cfg.UserToken)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Printf("server stopped")
	}
}
