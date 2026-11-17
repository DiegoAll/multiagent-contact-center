package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/repository"
)

const (
	maxSubjectRunes     = 200
	maxDescriptionRunes = 4000
	maxReasonRunes      = 500
)

type ticketService struct {
	tickets   repository.TicketRepository
	customers repository.CustomerRepository
}

// NewTicketService crea el servicio de tickets.
func NewTicketService(tickets repository.TicketRepository, customers repository.CustomerRepository) TicketService {
	return &ticketService{tickets: tickets, customers: customers}
}

func (s *ticketService) Create(ctx context.Context, in CreateTicketInput) (*domain.Ticket, error) {
	subject := strings.TrimSpace(in.Subject)
	description := strings.TrimSpace(in.Description)
	priority := strings.ToLower(strings.TrimSpace(in.Priority))

	switch {
	case in.CustomerID <= 0:
		return nil, invalid("customer_id must be positive")
	case subject == "" || utf8.RuneCountInString(subject) > maxSubjectRunes:
		return nil, invalid("subject is required (max 200 characters)")
	case description == "" || utf8.RuneCountInString(description) > maxDescriptionRunes:
		return nil, invalid("description is required (max 4000 characters)")
	}
	if priority == "" {
		priority = domain.PriorityMedium
	}
	if !domain.ValidPriority(priority) {
		return nil, invalid("priority must be one of low, medium, high, urgent")
	}

	// El cliente debe existir (integridad lógica antes de tocar tickets).
	if _, err := s.customers.GetByID(ctx, in.CustomerID); err != nil {
		return nil, err
	}

	t := &domain.Ticket{
		CustomerID:  in.CustomerID,
		Subject:     subject,
		Description: description,
		Priority:    priority,
		Status:      domain.TicketOpen,
	}
	if err := s.tickets.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ticketService) GetByID(ctx context.Context, id int64) (*domain.Ticket, error) {
	if id <= 0 {
		return nil, invalid("ticket id must be positive")
	}
	return s.tickets.GetByID(ctx, id)
}

func (s *ticketService) ListByCustomer(ctx context.Context, customerID int64) ([]domain.Ticket, error) {
	if customerID <= 0 {
		return nil, invalid("customer id must be positive")
	}
	return s.tickets.ListByCustomer(ctx, customerID)
}

func (s *ticketService) Escalate(ctx context.Context, id int64, reason string) (*domain.Ticket, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case id <= 0:
		return nil, invalid("ticket id must be positive")
	case reason == "" || utf8.RuneCountInString(reason) > maxReasonRunes:
		return nil, invalid("reason is required (max 500 characters)")
	}
	return s.tickets.Escalate(ctx, id, reason)
}
