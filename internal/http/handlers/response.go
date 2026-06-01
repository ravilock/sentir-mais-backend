package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
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
	respondJSON(w, status, apiresponses.ErrorResponse{Message: message})
}

func respondDecodeError(w http.ResponseWriter, err error) {
	if validationErrors, ok := errors.AsType[validator.ValidationErrors](err); ok {
		respondError(w, http.StatusUnprocessableEntity, validationMessages(validationErrors))
		return
	}

	respondError(w, http.StatusBadRequest, "invalid request body")
}

func validationMessages(errs validator.ValidationErrors) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, validationMessage(err))
	}

	return strings.Join(messages, ", ")
}

func validationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required", "notblank":
		return fmt.Sprintf("field '%s' is required", err.Field())
	case "min":
		return fmt.Sprintf("field '%s' minimum length is %s", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("field '%s' maximum length is %s", err.Field(), err.Param())
	case "email":
		return fmt.Sprintf("field '%s' must be a valid email", err.Field())
	default:
		return fmt.Sprintf("field '%s' is invalid", err.Field())
	}
}
