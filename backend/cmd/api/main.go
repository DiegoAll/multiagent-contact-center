// Command api inicia el backend REST del Atento Agent Lab.
//
// @title       Atento Agent Lab API
// @version     1.0
// @description API del backend de atención al cliente con agente de IA (CRM simulado, tickets y chat).
// @BasePath    /api/v1
//
// @securityDefinitions.apikey ApiKeyAuth
// @in                         header
// @name                       X-API-Key
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

	"github.com/diego-all/atento-agent-lab/backend/internal/agentclient"
	"github.com/diego-all/atento-agent-lab/backend/internal/config"
	"github.com/diego-all/atento-agent-lab/backend/internal/controller"
	"github.com/diego-all/atento-agent-lab/backend/internal/migrate"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository/postgres"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate.Up(db); err != nil {
		return err
	}

	// Composición: repositorios (concretos) -> servicios -> controllers.
	customerRepo := postgres.NewCustomerRepository(db)
	orderRepo := postgres.NewOrderRepository(db)
	ticketRepo := postgres.NewTicketRepository(db)

	agent := agentclient.New(cfg.AgentURL, cfg.AgentToken, cfg.AgentTimeout)

	handler := controller.NewRouter(controller.Deps{
		Customers:       service.NewCustomerService(customerRepo),
		Orders:          service.NewOrderService(orderRepo),
		Tickets:         service.NewTicketService(ticketRepo, customerRepo),
		Chat:            service.NewChatService(agent),
		APIKey:          cfg.APIKey,
		RateLimitPerMin: cfg.RateLimitPerMin,
		SwaggerFile:     cfg.SwaggerFile,
		Ready:           func(c context.Context) error { return db.PingContext(c) },
		Logger:          logger,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      95 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
