package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"

	"github.com/Decentr-net/go-api"
	"github.com/Decentr-net/vulcan/internal/mail"
	"github.com/Decentr-net/vulcan/internal/service"
)

// register sends email with link to create new wallet.
func (s *server) register(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /register Vulcan Register
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

	if err := s.s.Register(r.Context(), req.Email.String(), req.Address, req.ReferralCode); err != nil {
		switch {
		case errors.Is(err, service.ErrTooManyAttempts):
			api.WriteError(w, http.StatusTooManyRequests, "too many attempts")
		case errors.Is(err, service.ErrAlreadyExists):
			api.WriteError(w, http.StatusConflict, "email or address is already taken")
		case errors.Is(err, mail.ErrMailRejected):
			logrus.WithField("request", req).WithError(err).Error("failed to send email with rejected status")
			api.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			api.WriteInternalErrorf(r.Context(), w, "failed to register request: %s", err.Error())
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

// confirm confirms registration and creates wallet.
func (s *server) confirm(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /confirm Vulcan Confirm
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
			api.WriteError(w, http.StatusConflict, "already confirmed")
		default:
			api.WriteInternalErrorf(r.Context(), w, "failed to confirm registration: %s", err.Error())
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

// supply returns sum of erc20 and native supply stakes.
func (s *server) supply(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /supply Vulcan Supply
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
		api.WriteInternalErrorf(r.Context(), w, "failed to get supply: %s", err.Error())
		return
	}

	api.WriteOK(w, http.StatusOK, amount)
}

// trackReferralBrowserInstallation tracks the Decentr browser installation.
func (s *server) trackReferralBrowserInstallation(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /referral/track/install/{address} Vulcan TrackBrowserInstallation
	//
	// Tracks the Decentr browser installation.
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
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
			api.WriteInternalErrorf(r.Context(), w, "failed to track browser installation: %s", err.Error())
		}
		return
	}

	api.WriteOK(w, http.StatusOK, EmptyResponse{})
}

// getOwnReferralCode return a referral code of the given account.
func (s *server) getOwnReferralCode(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /referral/code/{address} Vulcan GetOwnReferralCode
	//
	// Returns a referral code of the given account
	//
	// ---
	// produces:
	// - application/json
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
			api.WriteInternalErrorf(r.Context(), w, "failed to get own referral code: %s", err.Error())
		}
		return
	}
	api.WriteOK(w, http.StatusOK, ReferralCodeResponse{Code: code})
}

func (s *server) getRegistrationReferralCode(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /referral/code/{address}/registration Vulcan GetRegistrationReferralCode
	//
	// Returns a referral code the account was registered with
	//
	// ---
	// produces:
	// - application/json
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
			api.WriteInternalErrorf(r.Context(), w, "failed to get registered referral code: %s", err.Error())
		}
		return
	}
	api.WriteOK(w, http.StatusOK, ReferralCodeResponse{Code: code})
}
