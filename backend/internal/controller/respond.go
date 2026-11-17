// Package controller expone la API HTTP (chi). Los controllers dependen solo
// de las interfaces del paquete service.
package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// ErrorResponse es el formato de error de toda la API.
type ErrorResponse struct {
	Error string `json:"error" example:"resource not found"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError traduce errores de dominio a respuestas HTTP sin filtrar
// detalles internos.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "resource not found"})
	case errors.Is(err, domain.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrUpstream):
		slog.Error("upstream error", "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "agent service unavailable"})
	default:
		slog.Error("internal error", "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
	}
}

// pathID lee un parámetro de ruta numérico positivo.
func pathID(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: %s must be a positive integer", domain.ErrInvalidInput, key)
	}
	return id, nil
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
