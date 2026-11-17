package postgres

import (
	"context"
	"database/sql"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

const orderCols = `id, customer_id, status, total_cents, currency, items, estimated_delivery, updated_at`

type orderRepo struct{ db *sql.DB }

// NewOrderRepository devuelve la implementación PostgreSQL del contrato.
func NewOrderRepository(db *sql.DB) repository.OrderRepository {
	return &orderRepo{db: db}
}

func scanOrder(s scanner) (*domain.Order, error) {
	var (
		o        domain.Order
		delivery sql.NullTime
	)
	if err := s.Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalCents, &o.Currency,
		&o.Items, &delivery, &o.UpdatedAt); err != nil {
		return nil, mapErr(err)
	}
	if delivery.Valid {
		t := delivery.Time
		o.EstimatedDelivery = &t
	}
	return &o, nil
}

func (r *orderRepo) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1`, id)
	return scanOrder(row)
}

func (r *orderRepo) ListByCustomer(ctx context.Context, customerID int64) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+orderCols+` FROM orders WHERE customer_id = $1 ORDER BY id DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}
