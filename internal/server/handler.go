package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-openapi/strfmt"

	"github.com/Decentr-net/vulcan/internal/service"
)

// register sends email with link to create new wallet.
func (s *server) register(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /register Vulcan Register
	//
	// Sends confirmation link via email. After confirmation a wallet will be created.
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

	if !strfmt.IsEmail(req.Email.String()) {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}

	if err := s.s.Register(r.Context(), req.Email.String()); err != nil {
		if errors.Is(err, service.ErrEmailIsBusy) {
			writeError(w, http.StatusConflict, "email is busy")
		} else {
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
	// Confirms registration and creates wallet.
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
	//   '201':
	//     description: wallet was created
	//     schema:
	//       "$ref": "#/definitions/ConfirmResponse"
	//   '404':
	//      description: no one register request was found
	//      schema:
	//        "$ref": "#/definitions/Error"
	//   '500':
	//      description: internal server error
	//      schema:
	//        "$ref": "#/definitions/Error"

	owner, code := chi.URLParam(r, "owner"), chi.URLParam(r, "code")

	info, err := s.s.Confirm(r.Context(), owner, code)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
		} else {
			writeInternalError(getLogger(r.Context()).WithError(err), w, "failed to confirm registration")
		}
		return
	}

	writeOK(w, http.StatusCreated, ConfirmResponse{
		Address:  info.Address,
		PubKey:   info.PubKey,
		Mnemonic: info.Mnemonic,
	})
}
