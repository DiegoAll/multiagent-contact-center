package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

const maxBodyBytes = 64 << 10 // 64 KiB

// TicketController agrupa los endpoints de tickets.
type TicketController struct {
	svc service.TicketService
}

// NewTicketController crea el controller.
func NewTicketController(svc service.TicketService) *TicketController {
	return &TicketController{svc: svc}
}

// EscalateRequest es el cuerpo para escalar un ticket a un agente humano.
type EscalateRequest struct {
	Reason string `json:"reason" example:"El cliente solicita hablar con un supervisor"`
}

// decodeBody decodifica JSON estricto con límite de tamaño.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: request body is not valid JSON", domain.ErrInvalidInput)
	}
	return nil
}

// Create godoc
// @Summary     Crea un ticket
// @Tags        tickets
// @Accept      json
// @Produce     json
// @Param       body body service.CreateTicketInput true "Datos del ticket"
// @Success     201 {object} domain.Ticket
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /tickets [post]
func (c *TicketController) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CreateTicketInput
	if err := decodeBody(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := c.svc.Create(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// Get godoc
// @Summary     Obtiene un ticket por id
// @Tags        tickets
// @Produce     json
// @Param       id path int true "ID del ticket"
// @Success     200 {object} domain.Ticket
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /tickets/{id} [get]
func (c *TicketController) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := c.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ListByCustomer godoc
// @Summary     Lista los tickets de un cliente
// @Tags        tickets
// @Produce     json
// @Param       id path int true "ID del cliente"
// @Success     200 {array}  domain.Ticket
// @Failure     400 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /customers/{id}/tickets [get]
func (c *TicketController) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := c.svc.ListByCustomer(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Escalate godoc
// @Summary     Escala un ticket a un agente humano
// @Tags        tickets
// @Accept      json
// @Produce     json
// @Param       id   path int             true "ID del ticket"
// @Param       body body EscalateRequest true "Motivo del escalamiento"
// @Success     200 {object} domain.Ticket
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /tickets/{id}/escalate [post]
func (c *TicketController) Escalate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in EscalateRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := c.svc.Escalate(r.Context(), id, in.Reason)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
