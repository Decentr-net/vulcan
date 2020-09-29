package server

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/go-openapi/strfmt"
)

var (
	addressRegExp     = regexp.MustCompile(`decentr[\d\w]{39}`) // nolint
	errInvalidRequest = errors.New("invalid request")
)

// Error ...
// swagger:model
type Error struct {
	Error string `json:"error"`
}

// EmptyResponse ...
// swagger:model
type EmptyResponse struct{}

// RegisterRequest ...
// swagger:model
type RegisterRequest struct {
	// required: true
	Email   strfmt.Email `json:"email"`
	Address string       `json:"address"`
}

// ConfirmRequest ...
// swagger:model
type ConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (r RegisterRequest) validate() error {
	if !strfmt.IsEmail(r.Email.String()) {
		return fmt.Errorf("%w: invalid email", errInvalidRequest)
	}

	if !addressRegExp.MatchString(r.Address) {
		return fmt.Errorf("%w: invalid address", errInvalidRequest)
	}

	return nil
}
