// Package server Vulcan
//
// The Vulcan is an users' wallets creator.
//
//     Schemes: https
//     BasePath: /v1
//     Version: 1.0.0
//
//     Produces:
//     - application/json
//     Consumes:
//     - application/json
//
// swagger:meta
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/service"
)

//go:generate swagger generate spec -t swagger -m -c . -o ../../static/swagger.json

const maxBodySize = 1024

type server struct {
	s service.Service
}

// SetupRouter setups handlers to chi router.
func SetupRouter(s service.Service, r chi.Router) {
	r.Use(
		swaggerMiddleware,
		loggerMiddleware,
		setHeadersMiddleware,
		middleware.StripSlashes,
		recovererMiddleware,
		bodyLimiterMiddleware(maxBodySize),
	)

	srv := server{
		s: s,
	}

	r.Post("/v1/register", srv.register)
	r.Post("/v1/confirm", srv.confirm)
}

func getLogger(ctx context.Context) logrus.FieldLogger {
	return ctx.Value(logCtxKey{}).(logrus.FieldLogger)
}

func writeErrorf(w http.ResponseWriter, status int, format string, args ...interface{}) {
	body, _ := json.Marshal(Error{
		Error: fmt.Sprintf(format, args...),
	})

	w.WriteHeader(status)
	// nolint:gosec,errcheck
	w.Write(body)
}

func writeError(w http.ResponseWriter, s int, message string) {
	writeErrorf(w, s, message)
}

func writeInternalError(l logrus.FieldLogger, w http.ResponseWriter, message string) {
	l.Error(string(debug.Stack()))
	l.Error(message)
	// We don't want to expose internal error to user. So we will just send typical error.
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeOK(w http.ResponseWriter, status int, v interface{}) {
	body, _ := json.Marshal(v)

	w.WriteHeader(status)
	// nolint:gosec,errcheck
	w.Write(body)
}
