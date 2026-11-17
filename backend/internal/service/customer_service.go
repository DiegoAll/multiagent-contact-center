package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

type customerService struct {
	repo repository.CustomerRepository
}

// NewCustomerService crea el servicio de clientes.
func NewCustomerService(repo repository.CustomerRepository) CustomerService {
	return &customerService{repo: repo}
}

func (s *customerService) GetByID(ctx context.Context, id int64) (*domain.Customer, error) {
	if id <= 0 {
		return nil, invalid("customer id must be positive")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *customerService) GetByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return nil, invalid("email is not valid")
	}
	return s.repo.GetByEmail(ctx, email)
}

func (s *customerService) List(ctx context.Context, limit, offset int) ([]domain.Customer, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}
