// Package health contains code for health checks.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Decentr-net/vulcan/internal/server"
)

// Pinger pings external service.
type Pinger interface {
	Ping(ctx context.Context) error
}

// SetupRouter setups all pingers to /health.
func SetupRouter(r chi.Router, p ...Pinger) {
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := context.WithTimeout(r.Context(), time.Second*5) // nolint:govet
		gr, ctx := errgroup.WithContext(ctx)

		for i := range p {
			v := p[i]
			gr.Go(func() error {
				if err := v.Ping(ctx); err != nil {
					logrus.WithError(err).Error("health check failed")
					return err
				}
				return nil
			})
		}

		if err := gr.Wait(); err != nil {
			data, _ := json.Marshal(server.Error{Error: err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data) // nolint
		}
	})
}
