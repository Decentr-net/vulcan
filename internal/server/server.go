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
	"github.com/Decentr-net/vulcan/internal/supply"
)

//go:generate swagger generate spec -t swagger -m -c . -o ../../static/swagger.json

const maxBodySize = 1024

type server struct {
	s   service.Service
	sup supply.Supply
}

// SetupRouter setups handlers to chi router.
func SetupRouter(s service.Service, sup supply.Supply, r chi.Router, timeout time.Duration) {
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
		s:   s,
		sup: sup,
	}

	r.Route("/v1", func(r chi.Router) {
		r.Post("/register", srv.register)
		r.Post("/confirm", srv.confirm)
		r.Get("/supply", srv.supply)
		r.Route("/referral", func(r chi.Router) {
			r.Get("/code/{address}", srv.getOwnReferralCode)
			r.Get("/code/{address}/registration", srv.getRegistrationReferralCode)
			r.Post("/track/install/{address}", srv.trackReferralBrowserInstallation)
			r.Get("/track/stats/{address}", srv.getReferralTrackingStats)
		})
	})
}
