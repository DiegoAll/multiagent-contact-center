// Package service contiene la lógica de negocio. Expone interfaces que
// consumen los controllers y depende únicamente de las interfaces del
// paquete repository.
package service

import (
	"context"
	"fmt"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// CustomerService define los casos de uso sobre clientes.
type CustomerService interface {
	GetByID(ctx context.Context, id int64) (*domain.Customer, error)
	GetByEmail(ctx context.Context, email string) (*domain.Customer, error)
	List(ctx context.Context, limit, offset int) ([]domain.Customer, error)
}

// OrderService define los casos de uso sobre pedidos.
type OrderService interface {
	GetByID(ctx context.Context, id int64) (*domain.Order, error)
	ListByCustomer(ctx context.Context, customerID int64) ([]domain.Order, error)
}

// CreateTicketInput son los datos necesarios para abrir un ticket.
type CreateTicketInput struct {
	CustomerID  int64  `json:"customer_id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"`
}

// TicketService define los casos de uso sobre tickets.
type TicketService interface {
	Create(ctx context.Context, in CreateTicketInput) (*domain.Ticket, error)
	GetByID(ctx context.Context, id int64) (*domain.Ticket, error)
	ListByCustomer(ctx context.Context, customerID int64) ([]domain.Ticket, error)
	Escalate(ctx context.Context, id int64, reason string) (*domain.Ticket, error)
}

// AgentClient abstrae la comunicación con el servicio del agente (Python).
type AgentClient interface {
	Chat(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
}

// ChatService orquesta la conversación con el agente.
type ChatService interface {
	Chat(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error)
}

// invalid construye un error de validación envolviendo domain.ErrInvalidInput.
func invalid(msg string) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, msg)
}
