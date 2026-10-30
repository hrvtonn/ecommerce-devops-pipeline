package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

const maxBodyBytes = 10 << 20 // 10 MB

type envelope struct {
	Status     string            `json:"status"`
	Data       any               `json:"data"`
	Pagination *model.Pagination `json:"pagination,omitempty"`
}

type errorBody struct {
	Status  string                `json:"status"`
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Details []apperror.FieldError `json:"details,omitempty"`
}

// handlerFunc is an http.HandlerFunc that returns its error for central handling.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, r, err)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{Status: "success", Data: data})
}

func writePage[T any](w http.ResponseWriter, page *model.Page[T]) {
	writeJSON(w, http.StatusOK, envelope{Status: "success", Data: page.Data, Pagination: &page.Pagination})
}

// writeError formats every error response and logs it appropriately.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		validationErr *apperror.ValidationError
		appErr        *apperror.Error
		pgErr         *pgconn.PgError
	)

	switch {
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, errorBody{
			Status:  "error",
			Code:    "VALIDATION_ERROR",
			Message: "Validation failed",
			Details: validationErr.Details,
		})

	case errors.As(err, &appErr):
		slog.Warn("Operational error: "+appErr.Message,
			"code", appErr.Code, "statusCode", appErr.Status, "path", r.URL.Path)
		writeJSON(w, appErr.Status, errorBody{Status: "error", Code: appErr.Code, Message: appErr.Message})

	// PostgreSQL unique constraint violation
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeJSON(w, http.StatusConflict, errorBody{
			Status:  "error",
			Code:    "CONFLICT",
			Message: "A resource with this value already exists",
		})

	// PostgreSQL foreign key violation (e.g. deleting a category still
	// referenced by soft-deleted products)
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		writeJSON(w, http.StatusConflict, errorBody{
			Status:  "error",
			Code:    "CONFLICT",
			Message: "The resource is referenced by other records",
		})

	default:
		slog.Error("Unexpected error", "error", err, "path", r.URL.Path, "method", r.Method)
		writeJSON(w, http.StatusInternalServerError, errorBody{
			Status:  "error",
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred",
		})
	}
}

// decodeBody parses the JSON request body; an empty body decodes to {}.
func decodeBody(w http.ResponseWriter, r *http.Request) (any, error) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, apperror.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body is too large")
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var body any
	if err := dec.Decode(&body); err != nil || dec.More() {
		return nil, apperror.New(http.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON")
	}
	return body, nil
}

// pathID parses the {id} URL parameter.
func pathID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 || id > math.MaxInt32 {
		return 0, &apperror.ValidationError{Details: []apperror.FieldError{
			{Field: "id", Message: `"id" must be a positive integer`},
		}}
	}
	return id, nil
}
