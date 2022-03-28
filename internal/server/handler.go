package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"

	"github.com/Decentr-net/decentr/config"
	"github.com/Decentr-net/go-api"

	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/service"
	"github.com/Decentr-net/vulcan/internal/storage"
)

// register sends email with link to create new wallet.
func (s *server) register(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v1/register Vulcan Register
	//
	// Sends confirmation link via email. After confirmation stakes will be sent.
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: email
	//   in: body
	//   required: true
	//   schema:
	//     '$ref': '#/definitions/RegisterRequest'
	// responses:
	//   '200':
	//     description: confirmation link was sent.
	//     schema:
	//       "$ref": "#/definitions/EmptyResponse"
	//   '400':
	//      description: bad request.
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '422':
	//      description: referral code not found.
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '429':
	//      description: minute didn't pass after last try to send email
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '409':
	//      description: wallet has already created for this email.
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.validate(); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ReferralCode != nil {
		if err := s.s.CheckRecaptcha(r.Context(), "register", req.RecaptchaResponse); err != nil {
			if !errors.Is(err, service.ErrRecaptcha) {
				api.WriteInternalErrorf(r.Context(), w, err, "failed to check recaptcha")
				return
			}
			api.WriteError(w, http.StatusLocked, err.Error())
			return
		}
	}

	if err := s.s.Register(r.Context(), req.Email.String(), req.Address, req.ReferralCode); err != nil {
		switch {
		case errors.Is(err, service.ErrTooManyAttempts):
			api.WriteError(w, http.StatusTooManyRequests, "too many attempts")
		case errors.Is(err, service.ErrAlreadyExists):
			api.WriteError(w, http.StatusConflict, "email or address is already taken")
		case errors.Is(err, service.ErrReferralCodeNotFound):
			api.WriteError(w, http.StatusUnprocessableEntity, "referral code not found")
			logrus.WithField("request", req).Warn("referral code not found")
		case errors.Is(err, mail.ErrMailRejected):
			logrus.WithField("request", req).WithError(err).Error("failed to send email with rejected status")
			api.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			api.WriteInternalErrorf(r.Context(), w, err, "failed to register request")
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

// getRegisterStats ...
func (s *server) getRegisterStats(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/register/stats Vulcan RegisterStats
	//
	// Confirmed registrations stats
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// responses:
	//   '200':
	//     description: confirmation link was sent.
	//     schema:
	//       "$ref": "#/definitions/RegisterStats"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	dbStats, total, err := s.s.GetRegisterStats(r.Context())
	if err != nil {
		api.WriteInternalErrorf(r.Context(), w, err, "failed to get accounts stats")
		return
	}

	stats := make([]StatsItem, 0, len(dbStats))
	for _, item := range dbStats {
		stats = append(stats, StatsItem{
			Date:  item.Date.Format("2006-01-02"),
			Value: item.Value,
		})
	}

	api.WriteOK(w, http.StatusOK, RegisterStats{
		Total: total,
		Stats: stats,
	})
}

// confirm confirms registration and creates wallet.
func (s *server) confirm(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v1/confirm Vulcan Confirm
	//
	// Confirms registration and sends stakes.
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: code
	//   in: body
	//   required: true
	//   schema:
	//     '$ref': '#/definitions/ConfirmRequest'
	// responses:
	//   '200':
	//     description: stakes were sent
	//   '404':
	//      description: no one register request was found.
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '409':
	//      description: request is already confirmed.
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.s.Confirm(r.Context(), req.Email, req.Code); err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			api.WriteError(w, http.StatusNotFound, "not found")
		case errors.Is(err, service.ErrAlreadyConfirmed):
			logrus.WithField("request", req).Warn("already confirmed")
			api.WriteError(w, http.StatusConflict, "already confirmed")
		default:
			api.WriteInternalErrorf(r.Context(), w, err, "failed to confirm registration")
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

// supply returns sum of erc20 and native supply stakes.
func (s *server) supply(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/supply Vulcan Supply
	//
	// Returns sum of erc20 and native supply supply.
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//       type: number
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	amount, err := s.sup.GetCirculatingSupply()
	if err != nil {
		api.WriteInternalErrorf(r.Context(), w, err, "failed to get supply")
		return
	}

	api.WriteOK(w, http.StatusOK, amount)
}

// getReferralConfig returns referral config.
func (s *server) getReferralConfig(w http.ResponseWriter, _ *http.Request) {
	// swagger:operation GET /v1/referral/config Vulcan RetReferralParams
	//
	// Returns referral params
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/definitions/Config"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	api.WriteOK(w, http.StatusOK, s.s.GetReferralConfig())
}

// trackReferralBrowserInstallation tracks the Decentr browser installation.
func (s *server) trackReferralBrowserInstallation(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v1/referral/track/install/{address} Vulcan TrackBrowserInstallation
	//
	// Tracks the Decentr browser installation.
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     description: referral marked with installed status
	//   '404':
	//      description: referral tracking not found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '409':
	//      description: referral is already marked as installed
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	address := chi.URLParam(r, "address")

	if err := s.s.TrackReferralBrowserInstallation(r.Context(), address); err != nil {
		switch {
		case errors.Is(err, service.ErrReferralTrackingNotFound):
			api.WriteError(w, http.StatusNotFound, "not found")
		case errors.Is(err, service.ErrReferralTrackingInvalidStatus):
			api.WriteError(w, http.StatusConflict, "status is installed or confirmed")
		default:
			api.WriteInternalErrorf(r.Context(), w, err, "failed to track browser installation")
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

func (s *server) getReferralTrackingStats(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/referral/track/stats/{address} Vulcan GetReferralTrackingStats
	//
	// Returns a referral tracking stats of the given account
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/definitions/ReferralTrackingStatsResponse"
	//   '404':
	//      description: address not found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	address := chi.URLParam(r, "address")

	stats, err := s.s.GetReferralTrackingStats(r.Context(), address)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			api.WriteError(w, http.StatusNotFound, "not found")
		default:
			api.WriteInternalErrorf(r.Context(), w,
				fmt.Errorf("stats %s failed:%w", address, err), "failed to get referral tracking stats")
		}
		return
	}

	api.WriteOK(w, http.StatusOK, ReferralTrackingStatsResponse{
		Total:      toReferralTrackingStatsItem(*stats[0]),
		Last30Days: toReferralTrackingStatsItem(*stats[1]),
	})
}

// getOwnReferralCode return a referral code of the given account.
func (s *server) getOwnReferralCode(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/referral/code/{address} Vulcan GetOwnReferralCode
	//
	// Returns a referral code of the given account
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/definitions/ReferralCodeResponse"
	//   '404':
	//      description: referral code not found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	address := chi.URLParam(r, "address")

	code, err := s.s.GetOwnReferralCode(r.Context(), address)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			api.WriteError(w, http.StatusNotFound, "not found")
		default:
			api.WriteInternalErrorf(r.Context(), w, err, "failed to get own referral code")
		}
		return
	}
	api.WriteOK(w, http.StatusOK, ReferralCodeResponse{Code: code})
}

func (s *server) getRegistrationReferralCode(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/referral/code/{address}/registration Vulcan GetRegistrationReferralCode
	//
	// Returns a referral code the account was registered with
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/definitions/ReferralCodeResponse"
	//   '404':
	//      description: referral code not found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	address := chi.URLParam(r, "address")

	code, err := s.s.GetRegistrationReferralCode(r.Context(), address)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			api.WriteError(w, http.StatusNotFound, "not found")
		default:
			api.WriteInternalErrorf(r.Context(), w, err, "failed to get registered referral code")
		}
		return
	}
	api.WriteOK(w, http.StatusOK, ReferralCodeResponse{Code: code})
}

func (s *server) registerTestnetAccount(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v1/hesoyam/{address} Vulcan GiveStakes
	//
	// Like a game cheat gives you test stakes. Works only for testnet.
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     description: stakes were sent
	//   '500':
	//      description: internal server error.
	//      schema:
	//        "$ref": "#/definitions/Error"

	address := chi.URLParam(r, "address")
	if !isAddressValid(address) {
		api.WriteError(w, http.StatusBadRequest, "invalid address")
		return
	}

	if err := s.s.RegisterTestnetAccount(r.Context(), address); err != nil {
		api.WriteInternalErrorf(r.Context(), w, err, "failed to give stakes")
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

func toReferralTrackingStatsItem(item storage.ReferralTrackingStats) ReferralTrackingStatsItem {
	return ReferralTrackingStatsItem{
		Registered: item.Registered,
		Installed:  item.Installed,
		Confirmed:  item.Confirmed,
		Reward:     sdk.NewCoin(config.DefaultBondDenom, item.Reward),
	}
}
