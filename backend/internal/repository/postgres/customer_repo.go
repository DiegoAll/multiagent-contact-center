package postgres

import (
	"context"
	"database/sql"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

const customerCols = `id, name, email, phone, segment, created_at`

type customerRepo struct{ db *sql.DB }

// NewCustomerRepository devuelve la implementación PostgreSQL del contrato.
func NewCustomerRepository(db *sql.DB) repository.CustomerRepository {
	return &customerRepo{db: db}
}

func scanCustomer(s scanner) (*domain.Customer, error) {
	var c domain.Customer
	if err := s.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Segment, &c.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (r *customerRepo) GetByID(ctx context.Context, id int64) (*domain.Customer, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+customerCols+` FROM customers WHERE id = $1`, id)
	return scanCustomer(row)
}

func (r *customerRepo) GetByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+customerCols+` FROM customers WHERE email = $1`, email)
	return scanCustomer(row)
}

func (r *customerRepo) List(ctx context.Context, limit, offset int) ([]domain.Customer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+customerCols+` FROM customers ORDER BY id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Customer, 0, limit)
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
