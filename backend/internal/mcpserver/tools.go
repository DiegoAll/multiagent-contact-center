// Package mcpserver expone los servicios de negocio como tools MCP.
// Las tools se declaran una sola vez (ToolSpec) y de esa definición salen
// tanto el servidor MCP como el catálogo documentado.
package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// Services agrupa los casos de uso que las tools necesitan.
type Services struct {
	Customers service.CustomerService
	Orders    service.OrderService
	Tickets   service.TicketService
}

// ParamSpec describe un parámetro de una tool.
type ParamSpec struct {
	Name        string
	Type        string // "string" | "number"
	Description string
	Required    bool
}

// ToolSpec es la definición declarativa de una tool MCP.
type ToolSpec struct {
	Name        string
	Description string
	// Access documenta el efecto de la tool: "read" o "write".
	Access  string
	Params  []ParamSpec
	Handler func(ctx context.Context, args map[string]any) (any, error)
}

// customerView expone solo los campos mínimos necesarios al agente
// (minimización de datos: no se entrega el teléfono).
type customerView struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Segment string `json:"segment"`
}

// Tools devuelve las tools disponibles. Con Services vacío sirve para generar
// el catálogo (los handlers no se ejecutan).
func Tools(svc Services) []ToolSpec {
	return []ToolSpec{
		{
			Name:        "get_customer",
			Description: "Obtiene datos básicos de un cliente del CRM por email o por id. Indica al menos uno.",
			Access:      "read",
			Params: []ParamSpec{
				{Name: "email", Type: "string", Description: "Email del cliente"},
				{Name: "customer_id", Type: "number", Description: "ID numérico del cliente"},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				var (
					c   *domain.Customer
					err error
				)
				if email := argString(args, "email"); email != "" {
					c, err = svc.Customers.GetByEmail(ctx, email)
				} else if id, ok := argInt64(args, "customer_id"); ok {
					c, err = svc.Customers.GetByID(ctx, id)
				} else {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("provide email or customer_id"))
				}
				if err != nil {
					return nil, err
				}
				return customerView{ID: c.ID, Name: c.Name, Email: c.Email, Segment: c.Segment}, nil
			},
		},
		{
			Name:        "get_order_status",
			Description: "Consulta el estado, el total y la fecha estimada de entrega de un pedido.",
			Access:      "read",
			Params: []ParamSpec{
				{Name: "order_id", Type: "number", Description: "ID numérico del pedido", Required: true},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				id, ok := argInt64(args, "order_id")
				if !ok {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("order_id is required"))
				}
				return svc.Orders.GetByID(ctx, id)
			},
		},
		{
			Name:        "list_customer_orders",
			Description: "Lista los pedidos de un cliente, del más reciente al más antiguo.",
			Access:      "read",
			Params: []ParamSpec{
				{Name: "customer_id", Type: "number", Description: "ID numérico del cliente", Required: true},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				id, ok := argInt64(args, "customer_id")
				if !ok {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("customer_id is required"))
				}
				return svc.Orders.ListByCustomer(ctx, id)
			},
		},
		{
			// El agente la usa para verificar, en código, que un ticket pertenece
			// al cliente autenticado antes de permitir escalarlo (ver guard_tools).
			Name:        "list_customer_tickets",
			Description: "Lista los tickets de soporte de un cliente, del más reciente al más antiguo.",
			Access:      "read",
			Params: []ParamSpec{
				{Name: "customer_id", Type: "number", Description: "ID numérico del cliente", Required: true},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				id, ok := argInt64(args, "customer_id")
				if !ok {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("customer_id is required"))
				}
				return svc.Tickets.ListByCustomer(ctx, id)
			},
		},
		{
			Name:        "create_ticket",
			Description: "Abre un ticket de soporte para un cliente. Úsala solo cuando el cliente lo pida o el problema no se pueda resolver con la base de conocimiento.",
			Access:      "write",
			Params: []ParamSpec{
				{Name: "customer_id", Type: "number", Description: "ID del cliente", Required: true},
				{Name: "subject", Type: "string", Description: "Asunto breve del ticket", Required: true},
				{Name: "description", Type: "string", Description: "Descripción del problema", Required: true},
				{Name: "priority", Type: "string", Description: "low, medium, high o urgent (por defecto medium)"},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				id, ok := argInt64(args, "customer_id")
				if !ok {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("customer_id is required"))
				}
				return svc.Tickets.Create(ctx, service.CreateTicketInput{
					CustomerID:  id,
					Subject:     argString(args, "subject"),
					Description: argString(args, "description"),
					Priority:    argString(args, "priority"),
				})
			},
		},
		{
			Name:        "escalate_to_human",
			Description: "Escala un ticket existente a un agente humano indicando el motivo. Úsala ante quejas graves, solicitudes de supervisor o cuando el agente no pueda ayudar.",
			Access:      "write",
			Params: []ParamSpec{
				{Name: "ticket_id", Type: "number", Description: "ID del ticket a escalar", Required: true},
				{Name: "reason", Type: "string", Description: "Motivo del escalamiento", Required: true},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				id, ok := argInt64(args, "ticket_id")
				if !ok {
					return nil, errors.Join(domain.ErrInvalidInput, errors.New("ticket_id is required"))
				}
				return svc.Tickets.Escalate(ctx, id, argString(args, "reason"))
			},
		},
	}
}

func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

// argInt64 acepta los tipos numéricos que produce el decodificador JSON.
func argInt64(args map[string]any, key string) (int64, bool) {
	switch v := args[key].(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	case int:
		return int64(v), true
	case int64:
		return v, true
	}
	return 0, false
}

// userMessage convierte un error en un mensaje seguro para el LLM.
func userMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "No se encontró el recurso solicitado."
	case errors.Is(err, domain.ErrInvalidInput):
		return "Entrada inválida: " + cleanInvalid(err)
	default:
		return "Error interno al ejecutar la herramienta."
	}
}

// cleanInvalid recorta el prefijo técnico del error de validación.
func cleanInvalid(err error) string {
	msg := err.Error()
	msg = strings.ReplaceAll(msg, domain.ErrInvalidInput.Error()+": ", "")
	msg = strings.ReplaceAll(msg, domain.ErrInvalidInput.Error(), "")
	return strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
}
