package controller

import (
	"net/http"

	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// CustomerController agrupa los endpoints de clientes.
type CustomerController struct {
	svc service.CustomerService
}

// NewCustomerController crea el controller.
func NewCustomerController(svc service.CustomerService) *CustomerController {
	return &CustomerController{svc: svc}
}

// List godoc
// @Summary     Lista clientes
// @Tags        customers
// @Produce     json
// @Param       limit  query int false "Máximo de resultados (1-100)" default(20)
// @Param       offset query int false "Desplazamiento" default(0)
// @Success     200 {array}  domain.Customer
// @Failure     500 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /customers [get]
func (c *CustomerController) List(w http.ResponseWriter, r *http.Request) {
	out, err := c.svc.List(r.Context(), queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Get godoc
// @Summary     Obtiene un cliente por id
// @Tags        customers
// @Produce     json
// @Param       id path int true "ID del cliente"
// @Success     200 {object} domain.Customer
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /customers/{id} [get]
func (c *CustomerController) Get(w http.ResponseWriter, r *http.Request) {
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

// Lookup godoc
// @Summary     Busca un cliente por email
// @Tags        customers
// @Produce     json
// @Param       email query string true "Email del cliente"
// @Success     200 {object} domain.Customer
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /customers/lookup [get]
func (c *CustomerController) Lookup(w http.ResponseWriter, r *http.Request) {
	out, err := c.svc.GetByEmail(r.Context(), r.URL.Query().Get("email"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
