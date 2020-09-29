package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/Decentr-net/vulcan/internal/service"
)

var (
	errSkip = fmt.Errorf("skip")
	errTest = fmt.Errorf("test")
)

func Test_Register(t *testing.T) {
	tt := []struct {
		name       string
		body       []byte
		serviceErr error
		rcode      int
		rdata      string
		rlog       string
	}{
		{
			name:       "success",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"}`),
			serviceErr: nil,
			rcode:      http.StatusOK,
			rdata:      `{}`,
			rlog:       "",
		},
		{
			name:       "invalid email",
			body:       []byte(`{"email":"decentrdecentr.xyz", "address":"decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"}`),
			serviceErr: errSkip,
			rcode:      http.StatusBadRequest,
			rdata:      `{"error": "invalid request: invalid email"}`,
			rlog:       "",
		},
		{
			name:       "invalid address",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqg"}`),
			serviceErr: errSkip,
			rcode:      http.StatusBadRequest,
			rdata:      `{"error": "invalid request: invalid address"}`,
			rlog:       "",
		},
		{
			name:       "already registered",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"}`),
			serviceErr: service.ErrAlreadyExists,
			rcode:      http.StatusConflict,
			rdata:      `{"error": "email is busy"}`,
			rlog:       "",
		},
		{
			name:       "internal error",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga"}`),
			serviceErr: errTest,
			rcode:      http.StatusInternalServerError,
			rdata:      `{"error": "internal error"}`,
			rlog:       "failed to register request",
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, w, r := newTestParameters(t, http.MethodPost, "v1/register", tc.body)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := service.NewMockService(ctrl)

			if tc.serviceErr != errSkip {
				srv.EXPECT().Register(gomock.Not(gomock.Nil()), "decentr@decentr.xyz", "decentr1vg085ra5hw8mx5rrheqf8fruks0xv4urqkuqga").Return(tc.serviceErr)
			}

			router := chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					log := logrus.New()
					log.SetOutput(b)
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), logCtxKey{}, log)))
				})
			})

			s := server{s: srv}
			router.Post("/v1/register", s.register)

			router.ServeHTTP(w, r)

			assert.True(t, strings.Contains(b.String(), tc.rlog))
			assert.Equal(t, tc.rcode, w.Code)
			assert.JSONEq(t, tc.rdata, w.Body.String())
		})
	}
}

func Test_Confirm(t *testing.T) {
	tt := []struct {
		name       string
		serviceErr error
		rcode      int
		rdata      string
		rlog       string
	}{
		{
			name:       "success",
			serviceErr: nil,
			rcode:      http.StatusOK,
			rdata:      "{}",
			rlog:       "",
		},
		{
			name:       "not found",
			serviceErr: service.ErrNotFound,
			rcode:      http.StatusNotFound,
			rdata:      `{"error": "not found"}`,
			rlog:       "",
		},
		{
			name:       "internal error",
			serviceErr: errTest,
			rcode:      http.StatusInternalServerError,
			rdata:      `{"error": "internal error"}`,
			rlog:       "failed to confirm registration",
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, w, r := newTestParameters(t, http.MethodPost, "v1/confirm", []byte(`{"email":"e@mail.com", "code":"5678"}`))

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := service.NewMockService(ctrl)

			if tc.serviceErr != errSkip {
				srv.EXPECT().Confirm(gomock.Not(gomock.Nil()), "e@mail.com", "5678").Return(tc.serviceErr)
			}

			router := chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					log := logrus.New()
					log.SetOutput(b)
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), logCtxKey{}, log)))
				})
			})

			s := server{s: srv}
			router.Post("/v1/confirm", s.confirm)

			router.ServeHTTP(w, r)

			assert.True(t, strings.Contains(b.String(), tc.rlog))
			assert.Equal(t, tc.rcode, w.Code)
			assert.JSONEq(t, tc.rdata, w.Body.String())
		})
	}
}
