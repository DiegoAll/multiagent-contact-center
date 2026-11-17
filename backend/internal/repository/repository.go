// Package repository define los contratos (interfaces) de acceso a datos.
// Las implementaciones concretas viven en subpaquetes (p. ej. postgres), de
// modo que los servicios dependen solo de abstracciones.
package repository

import (
	"context"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// CustomerRepository abstrae el acceso a clientes (CRM simulado).
type CustomerRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.Customer, error)
	GetByEmail(ctx context.Context, email string) (*domain.Customer, error)
	List(ctx context.Context, limit, offset int) ([]domain.Customer, error)
}

// OrderRepository abstrae el acceso a pedidos.
type OrderRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.Order, error)
	ListByCustomer(ctx context.Context, customerID int64) ([]domain.Order, error)
}

// TicketRepository abstrae el acceso a tickets.
type TicketRepository interface {
	Create(ctx context.Context, t *domain.Ticket) error
	GetByID(ctx context.Context, id int64) (*domain.Ticket, error)
	ListByCustomer(ctx context.Context, customerID int64) ([]domain.Ticket, error)
	Escalate(ctx context.Context, id int64, reason string) (*domain.Ticket, error)
}
