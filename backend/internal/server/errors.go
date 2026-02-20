package server

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simonjohansson/kanban/backend/internal/service"
)

func stringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func toHumaError(err error) error {
	code := service.CodeOf(err)
	msg := service.MessageOf(err)
	switch code {
	case service.CodeConflict:
		return huma.Error409Conflict(msg)
	case service.CodeNotFound:
		return huma.Error404NotFound(msg)
	case service.CodeValidation:
		return huma.Error400BadRequest(msg)
	default:
		return huma.Error500InternalServerError(msg)
	}
}

func statusForError(err error) int {
	code := service.CodeOf(err)
	switch code {
	case service.CodeConflict:
		return http.StatusConflict
	case service.CodeNotFound:
		return http.StatusNotFound
	case service.CodeValidation:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
