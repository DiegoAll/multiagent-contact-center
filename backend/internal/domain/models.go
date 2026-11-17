// Package domain contiene las entidades del negocio y los errores compartidos
// entre capas. No depende de ninguna otra capa del proyecto.
package domain

import (
	"errors"
	"time"
)

// Errores de dominio. Las capas superiores los traducen a códigos HTTP o a
// mensajes de tools MCP.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUpstream     = errors.New("upstream service unavailable")
)

// Prioridades y estados válidos para los tickets.
const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"

	TicketOpen      = "open"
	TicketEscalated = "escalated"
)

// ValidPriority indica si p es una prioridad permitida.
func ValidPriority(p string) bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Customer simula un registro de CRM.
type Customer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	Segment   string    `json:"segment"`
	CreatedAt time.Time `json:"created_at"`
}

// Order es un pedido asociado a un cliente.
type Order struct {
	ID                int64      `json:"id"`
	CustomerID        int64      `json:"customer_id"`
	Status            string     `json:"status"`
	TotalCents        int64      `json:"total_cents"`
	Currency          string     `json:"currency"`
	Items             string     `json:"items"`
	EstimatedDelivery *time.Time `json:"estimated_delivery,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Ticket simula un caso en una herramienta de ticketing / contact center.
type Ticket struct {
	ID               int64     `json:"id"`
	CustomerID       int64     `json:"customer_id"`
	Subject          string    `json:"subject"`
	Description      string    `json:"description"`
	Priority         string    `json:"priority"`
	Status           string    `json:"status"`
	Escalated        bool      `json:"escalated"`
	EscalationReason string    `json:"escalation_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ChatRequest es el mensaje que el frontend envía al agente.
type ChatRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	Message       string `json:"message"`
	CustomerEmail string `json:"customer_email,omitempty"`
}

// ToolCall describe una tool que el agente invocó (trazabilidad para la UI).
type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// ChatResponse es la respuesta del agente.
type ChatResponse struct {
	SessionID  string     `json:"session_id"`
	Reply      string     `json:"reply"`
	Escalated  bool       `json:"escalated"`
	AgentsUsed []string   `json:"agents_used,omitempty"`
	Sources    []string   `json:"sources,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}
