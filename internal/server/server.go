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
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"

	"github.com/Decentr-net/go-api"

	"github.com/Decentr-net/vulcan/internal/service"
)

//go:generate swagger generate spec -t swagger -m -c . -o ../../static/swagger.json

const maxBodySize = 1024

type server struct {
	s service.Service
}

// SetupRouter setups handlers to chi router.
func SetupRouter(s service.Service, r chi.Router, timeout time.Duration) {
	r.Use(
		api.FileServerMiddleware("/docs", "static"),
		api.LoggerMiddleware,
		middleware.StripSlashes,
		cors.AllowAll().Handler,
		api.RequestIDMiddleware,
		api.RecovererMiddleware,
		api.TimeoutMiddleware(timeout),
		api.BodyLimiterMiddleware(maxBodySize),
	)

	srv := server{
		s: s,
	}

	r.Post("/v1/register", srv.register)
	r.Post("/v1/confirm", srv.confirm)
}
