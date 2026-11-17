// Command mcp inicia el servidor MCP con las tools del CRM simulado.
//
// Uso:
//
//	mcp            # Streamable HTTP en :MCP_PORT/mcp
//	mcp -stdio     # transporte stdio (clientes locales, p. ej. Claude Desktop)
//	mcp -catalog   # imprime el catálogo de tools en Markdown y termina
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/diego-all/atento-agent-lab/backend/internal/config"
	"github.com/diego-all/atento-agent-lab/backend/internal/mcpserver"
	mw "github.com/diego-all/atento-agent-lab/backend/internal/middleware"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository/postgres"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

func main() {
	catalog := flag.Bool("catalog", false, "imprime el catálogo de tools en Markdown y termina")
	stdio := flag.Bool("stdio", false, "sirve por stdio en lugar de HTTP")
	flag.Parse()

	if *catalog {
		fmt.Print(mcpserver.CatalogMarkdown(mcpserver.Tools(mcpserver.Services{})))
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger, *stdio); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, stdio bool) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	customerRepo := postgres.NewCustomerRepository(db)
	svc := mcpserver.Services{
		Customers: service.NewCustomerService(customerRepo),
		Orders:    service.NewOrderService(postgres.NewOrderRepository(db)),
		Tickets:   service.NewTicketService(postgres.NewTicketRepository(db), customerRepo),
	}
	mcpSrv := mcpserver.NewServer(svc)

	if stdio {
		return server.ServeStdio(mcpSrv)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// El endpoint MCP exige el token interno compartido con el agente.
	auth := mw.HeaderTokenAuth("X-Internal-Token", cfg.AgentToken)
	mux.Handle("/mcp", auth(server.NewStreamableHTTPServer(mcpSrv)))

	srv := &http.Server{
		Addr:              ":" + cfg.MCPPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("mcp server listening", "addr", srv.Addr, "path", "/mcp")
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
