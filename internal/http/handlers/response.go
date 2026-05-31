package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ravilock/sentir-mais-backend/internal/http/dto"
)

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, dto.ErrorResponse{Message: message})
}
