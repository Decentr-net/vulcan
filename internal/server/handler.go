package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"

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
	//     description: confirmation link was sent
	//     schema:
	//       "$ref": "#/definitions/EmptyResponse"
	//   '400':
	//      description: bad request
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '409':
	//      description: wallet has already created for this email
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error
	//      schema:
	//        "$ref": "#/definitions/Error"

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.s.Register(r.Context(), req.Email.String(), req.Address); err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyExists):
			writeError(w, http.StatusConflict, "email is busy")
		default:
			writeInternalError(getLogger(r.Context()).WithError(err), w, "failed to register request")
		}
		return
	}

	writeOK(w, http.StatusOK, EmptyResponse{})
}

// confirm confirms registration and creates wallet.
func (s *server) confirm(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /confirm/{owner}/{code} Vulcan Confirm
	//
	// Confirms registration and sends stakes.
	//
	// ---
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: owner
	//   description: email hash
	//   in: path
	//   required: true
	//   type: string
	// - name: code
	//   description: confirmation code
	//   in: path
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     description: stakes were sent
	//     schema:
	//       "$ref": "#/definitions/ConfirmResponse"
	//   '404':
	//      description: no one register request was found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '409':
	//      description: request is already confirmed
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error
	//      schema:
	//        "$ref": "#/definitions/Error"

	owner, code := chi.URLParam(r, "owner"), chi.URLParam(r, "code")

	if err := s.s.Confirm(r.Context(), owner, code); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, service.ErrAlreadyConfirmed):
			writeError(w, http.StatusConflict, "already confirmed")
		default:
			writeInternalError(getLogger(r.Context()).WithError(err), w, "failed to confirm registration")
		}
		return
	}

	writeOK(w, http.StatusOK, EmptyResponse{})
}
