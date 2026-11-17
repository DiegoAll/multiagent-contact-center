package controller

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	mw "github.com/diego-all/atento-agent-lab/backend/internal/middleware"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// Deps son las dependencias necesarias para construir el router.
type Deps struct {
	Customers       service.CustomerService
	Orders          service.OrderService
	Tickets         service.TicketService
	Chat            service.ChatService
	APIKey          string
	RateLimitPerMin int
	SwaggerFile     string
	Ready           func(ctx context.Context) error
	Logger          *slog.Logger
}

const swaggerUIHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Atento Agent Lab · API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/swagger/doc.json', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`

// NewRouter construye el router chi con todos los endpoints y middlewares.
func NewRouter(d Deps) http.Handler {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}

	customers := NewCustomerController(d.Customers)
	orders := NewOrderController(d.Orders)
	tickets := NewTicketController(d.Tickets)
	chat := NewChatController(d.Chat)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(mw.SecurityHeaders)
	r.Use(mw.RequestLogger(logger))
	r.Use(chimw.Timeout(90 * time.Second))

	// Sondas para Kubernetes / docker-compose.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if d.Ready != nil {
			if err := d.Ready(req.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "not ready"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Documentación OpenAPI (generada con swag desde las anotaciones).
	r.Get("/swagger", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/swagger/", http.StatusMovedPermanently)
	})
	r.Get("/swagger/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, req, d.SwaggerFile)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.APIKeyAuth(d.APIKey))
		r.Use(mw.RateLimit(d.RateLimitPerMin))

		r.Get("/customers", customers.List)
		r.Get("/customers/lookup", customers.Lookup)
		r.Get("/customers/{id}", customers.Get)
		r.Get("/customers/{id}/orders", orders.ListByCustomer)
		r.Get("/customers/{id}/tickets", tickets.ListByCustomer)

		r.Get("/orders/{id}", orders.Get)

		r.Post("/tickets", tickets.Create)
		r.Get("/tickets/{id}", tickets.Get)
		r.Post("/tickets/{id}/escalate", tickets.Escalate)

		r.Post("/chat", chat.Chat)
	})

	return r
}
