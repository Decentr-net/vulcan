// Package health contains code for health checks.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// nolint:gochecknoglobals
var (
	version = "dev"
	commit  = "undefined"
)

// VersionResponse ...
type VersionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Pinger pings external service.
type Pinger interface {
	Ping(ctx context.Context) error
}

type subjectPinger struct {
	f func(ctx context.Context) error
	s string
}

// Ping ...
func (p subjectPinger) Ping(ctx context.Context) error {
	if err := p.f(ctx); err != nil {
		return fmt.Errorf("failed to ping %s: %w", p.s, err)
	}

	return nil
}

// SubjectPinger returns wrapper over Ping function which adds subject to error message.
// It is helpful for external Ping function, e.g. (sql.DB).Ping.
func SubjectPinger(s string, f func(ctx context.Context) error) Pinger {
	return subjectPinger{
		f: f,
		s: s,
	}
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
			data, _ := json.Marshal(struct {
				VersionResponse
				Error string `json:"error"`
			}{
				Error:           err.Error(),
				VersionResponse: VersionResponse{Version: version, Commit: commit},
			})
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data) // nolint

			return
		}

		data, _ := json.Marshal(VersionResponse{Version: version, Commit: commit})
		w.Write(data) // nolint
	})
}
