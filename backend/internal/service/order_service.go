package service

import (
	"context"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

type orderService struct {
	repo repository.OrderRepository
}

// NewOrderService crea el servicio de pedidos.
func NewOrderService(repo repository.OrderRepository) OrderService {
	return &orderService{repo: repo}
}

func (s *orderService) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	if id <= 0 {
		return nil, invalid("order id must be positive")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *orderService) ListByCustomer(ctx context.Context, customerID int64) ([]domain.Order, error) {
	if customerID <= 0 {
		return nil, invalid("customer id must be positive")
	}
	return s.repo.ListByCustomer(ctx, customerID)
}
