package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	logging "github.com/Decentr-net/logrus/context"
)

// Error ...
// swagger:model
type Error struct {
	Error string `json:"error"`
}

// WriteErrorf writes formatted error.
func WriteErrorf(w http.ResponseWriter, status int, format string, args ...interface{}) {
	body, _ := json.Marshal(Error{
		Error: fmt.Sprintf(format, args...),
	})

	w.WriteHeader(status)
	// nolint:gosec,errcheck
	w.Write(body)
}

// WriteError writes error.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteErrorf(w, status, message)
}

// WriteVerifyError writes sign verification(auth) error with proper status.
func WriteVerifyError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotVerified):
		WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrInvalidRequest):
		WriteError(w, http.StatusBadRequest, err.Error())
	default:
		WriteInternalError(ctx, w, err, "verify error")
	}
}

// WriteInternalError logs error and writes internal error.
func WriteInternalError(ctx context.Context, w http.ResponseWriter, err error, message string) {
	WriteInternalErrorf(ctx, w, err, message)
}

// WriteInternalErrorf logs formatted error and writes internal error.
func WriteInternalErrorf(ctx context.Context, w http.ResponseWriter, err error, format string, args ...interface{}) {
	if err != nil {
		logging.GetLogger(ctx).WithError(err).Errorf(format, args...)
	} else {
		logging.GetLogger(ctx).Errorf(format, args...)
	}

	// We don't want to expose internal error to user. So we will just send typical error.
	WriteError(w, http.StatusInternalServerError, "internal error")
}

// WriteOK writes json body.
func WriteOK(w http.ResponseWriter, status int, v interface{}) {
	body, _ := json.Marshal(v)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// nolint:gosec,errcheck
	w.Write(body)
}
