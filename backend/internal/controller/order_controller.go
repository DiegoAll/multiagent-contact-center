package controller

import (
	"net/http"

	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// OrderController agrupa los endpoints de pedidos.
type OrderController struct {
	svc service.OrderService
}

// NewOrderController crea el controller.
func NewOrderController(svc service.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

// Get godoc
// @Summary     Obtiene un pedido por id
// @Tags        orders
// @Produce     json
// @Param       id path int true "ID del pedido"
// @Success     200 {object} domain.Order
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /orders/{id} [get]
func (c *OrderController) Get(w http.ResponseWriter, r *http.Request) {
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
// @Summary     Lista los pedidos de un cliente
// @Tags        orders
// @Produce     json
// @Param       id path int true "ID del cliente"
// @Success     200 {array}  domain.Order
// @Failure     400 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /customers/{id}/orders [get]
func (c *OrderController) ListByCustomer(w http.ResponseWriter, r *http.Request) {
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
