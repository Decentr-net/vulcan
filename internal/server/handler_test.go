package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/Decentr-net/go-api/test"
	"github.com/Decentr-net/vulcan/internal/service"
	servicemock "github.com/Decentr-net/vulcan/internal/service/mock"
	"github.com/Decentr-net/vulcan/internal/storage"
	supplymock "github.com/Decentr-net/vulcan/internal/supply/mock"
)

var (
	errSkip = fmt.Errorf("skip")
	errTest = fmt.Errorf("test")
)

func Test_Register(t *testing.T) {
	tt := []struct {
		name         string
		body         []byte
		serviceErr   error
		rcode        int
		rdata        string
		rlog         string
		referralCode string
	}{
		{
			name:       "success",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m"}`),
			serviceErr: nil,
			rcode:      http.StatusOK,
			rdata:      `{}`,
			rlog:       "",
		},
		{
			name:       "invalid email",
			body:       []byte(`{"email":"decentrdecentr.xyz", "address":"decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m"}`),
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
			rdata:      `{"error": "invalid request: invalid address: decoding bech32 failed: checksum failed. Expected 6k4ypl, got rqkuqg."}`,
			rlog:       "",
		},
		{
			name:       "already registered",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m"}`),
			serviceErr: service.ErrAlreadyExists,
			rcode:      http.StatusConflict,
			rdata:      `{"error": "email or address is already taken"}`,
			rlog:       "",
		},
		{
			name:       "internal error",
			body:       []byte(`{"email":"decentr@decentr.xyz", "address":"decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m"}`),
			serviceErr: errTest,
			rcode:      http.StatusInternalServerError,
			rdata:      `{"error": "internal error"}`,
			rlog:       "failed to register request",
		},
		{
			name:         "referral code",
			body:         []byte(`{"email":"decentr@decentr.xyz", "address":"decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m", "referralCode": "abcdef12"}`),
			serviceErr:   nil,
			rcode:        http.StatusOK,
			rdata:        `{}`,
			rlog:         "",
			referralCode: "abcdef12",
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			l, w, r := test.NewAPITestParameters(http.MethodPost, "v1/register", tc.body)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := servicemock.NewMockService(ctrl)

			if tc.referralCode != "" {
				srv.EXPECT().Register(gomock.Not(gomock.Nil()), "decentr@decentr.xyz", "decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m", &tc.referralCode).Return(tc.serviceErr)
			} else if tc.serviceErr != errSkip {
				srv.EXPECT().Register(gomock.Not(gomock.Nil()), "decentr@decentr.xyz", "decentr18c2phdrfjkggr4afwf3rw4h4xsjvfhh2gl7t4m", nil).Return(tc.serviceErr)
			}

			router := chi.NewRouter()

			s := server{s: srv}
			router.Post("/v1/register", s.register)

			router.ServeHTTP(w, r)

			assert.True(t, strings.Contains(l.String(), tc.rlog))
			assert.Equal(t, tc.rcode, w.Code)
			assert.JSONEq(t, tc.rdata, w.Body.String())
		})
	}
}

func Test_GetRegisterStats(t *testing.T) {
	_, w, r := test.NewAPITestParameters(http.MethodGet, "v1/register/stats", []byte{})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := servicemock.NewMockService(ctrl)
	srv.EXPECT().GetRegisterStats(gomock.Not(gomock.Nil())).Return(
		[]*storage.RegisterStats{
			{Date: time.Date(2021, 10, 21, 0, 0, 0, 0, time.UTC), Value: 10},
			{Date: time.Date(2021, 10, 22, 0, 0, 0, 0, time.UTC), Value: 15},
		}, 100, nil)

	router := chi.NewRouter()

	s := server{s: srv}
	router.Get("/v1/register/stats", s.getRegisterStats)

	router.ServeHTTP(w, r)

	assert.JSONEq(t,
		`{
                     "accountsCount":100,
                     "stats": [
                        {"date":"2021-10-21", "value": 10},
                        {"date":"2021-10-22", "value": 15}
                     ]
                  }`,
		w.Body.String())
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
			serviceErr: service.ErrRequestNotFound,
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

			l, w, r := test.NewAPITestParameters(http.MethodPost, "v1/confirm", []byte(`{"email":"e@mail.com", "code":"5678"}`))

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := servicemock.NewMockService(ctrl)

			if tc.serviceErr != errSkip {
				srv.EXPECT().Confirm(gomock.Not(gomock.Nil()), "e@mail.com", "5678").Return(tc.serviceErr)
			}

			router := chi.NewRouter()

			s := server{s: srv}
			router.Post("/v1/confirm", s.confirm)

			router.ServeHTTP(w, r)

			assert.True(t, strings.Contains(l.String(), tc.rlog))
			assert.Equal(t, tc.rcode, w.Code)
			assert.JSONEq(t, tc.rdata, w.Body.String())
		})
	}
}

func Test_Circulating(t *testing.T) {
	tt := []struct {
		name   string
		amount int64
		err    error
		rcode  int
		rdata  string
	}{
		{
			name:   "success",
			amount: 1000,
			err:    nil,
			rcode:  http.StatusOK,
			rdata:  "1000",
		},
		{
			name:  "internal error",
			err:   errTest,
			rcode: http.StatusInternalServerError,
			rdata: `{"error": "internal error"}`,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, w, r := test.NewAPITestParameters(http.MethodGet, "v1/supply", nil)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sup := supplymock.NewMockSupply(ctrl)

			sup.EXPECT().GetCirculatingSupply().Return(tc.amount, tc.err)

			router := chi.NewRouter()

			s := server{sup: sup}
			router.Get("/v1/supply", s.supply)

			router.ServeHTTP(w, r)

			assert.Equal(t, tc.rcode, w.Code)
			assert.JSONEq(t, tc.rdata, w.Body.String())
		})
	}
}
