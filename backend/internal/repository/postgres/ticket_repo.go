package postgres

import (
	"context"
	"database/sql"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

const ticketCols = `id, customer_id, subject, description, priority, status, escalated, escalation_reason, created_at`

type ticketRepo struct{ db *sql.DB }

// NewTicketRepository devuelve la implementación PostgreSQL del contrato.
func NewTicketRepository(db *sql.DB) repository.TicketRepository {
	return &ticketRepo{db: db}
}

func scanTicket(s scanner) (*domain.Ticket, error) {
	var t domain.Ticket
	if err := s.Scan(&t.ID, &t.CustomerID, &t.Subject, &t.Description, &t.Priority,
		&t.Status, &t.Escalated, &t.EscalationReason, &t.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

func (r *ticketRepo) Create(ctx context.Context, t *domain.Ticket) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO tickets (customer_id, subject, description, priority, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, escalated, escalation_reason, created_at`,
		t.CustomerID, t.Subject, t.Description, t.Priority, t.Status,
	).Scan(&t.ID, &t.Escalated, &t.EscalationReason, &t.CreatedAt)
}

func (r *ticketRepo) GetByID(ctx context.Context, id int64) (*domain.Ticket, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+ticketCols+` FROM tickets WHERE id = $1`, id)
	return scanTicket(row)
}

func (r *ticketRepo) ListByCustomer(ctx context.Context, customerID int64) ([]domain.Ticket, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+ticketCols+` FROM tickets WHERE customer_id = $1 ORDER BY id DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *ticketRepo) Escalate(ctx context.Context, id int64, reason string) (*domain.Ticket, error) {
	row := r.db.QueryRowContext(ctx,
		`UPDATE tickets
		 SET escalated = TRUE, escalation_reason = $2, status = $3
		 WHERE id = $1
		 RETURNING `+ticketCols, id, reason, domain.TicketEscalated)
	return scanTicket(row)
}
