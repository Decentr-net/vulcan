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
type Error struct {
	Error string `json:"error"`
}

// EmptyResponse ...
type EmptyResponse struct{}

// RegisterRequest ...
// swagger:model
type RegisterRequest struct {
	// required: true
	Email   strfmt.Email `json:"email"`
	Address string       `json:"address"`
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
