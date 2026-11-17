package controller

import (
	"net/http"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// ChatController expone la conversación con el agente.
type ChatController struct {
	svc service.ChatService
}

// NewChatController crea el controller.
func NewChatController(svc service.ChatService) *ChatController {
	return &ChatController{svc: svc}
}

// Chat godoc
// @Summary     Envía un mensaje al agente de atención al cliente
// @Description El agente (supervisor + especialistas) responde usando RAG y tools MCP.
// @Tags        chat
// @Accept      json
// @Produce     json
// @Param       body body domain.ChatRequest true "Mensaje del usuario"
// @Success     200 {object} domain.ChatResponse
// @Failure     400 {object} ErrorResponse
// @Failure     502 {object} ErrorResponse
// @Security    ApiKeyAuth
// @Router      /chat [post]
func (c *ChatController) Chat(w http.ResponseWriter, r *http.Request) {
	var in domain.ChatRequest
	if err := decodeBody(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := c.svc.Chat(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
