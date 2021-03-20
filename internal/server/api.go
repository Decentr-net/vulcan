package server

import (
	"errors"
	"fmt"
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/go-openapi/strfmt"
)

var (
	emailRegExp       = regexp.MustCompile("(?:[a-z0-9!#$%&'*+\\/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+\\/=?^_`{|}~-]+)*|\"(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21\\x23-\\x5b\\x5d-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])*\")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21-\\x5a\\x53-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])+)\\])") // nolint
	errInvalidRequest = errors.New("invalid request")
)

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
	if !isEmailValid(r.Email.String()) {
		return fmt.Errorf("%w: invalid email", errInvalidRequest)
	}

	if addr, err := sdk.AccAddressFromBech32(r.Address); err != nil {
		return fmt.Errorf("%w: invalid address: %s", errInvalidRequest, err.Error())
	} else if len(addr) == 0 {
		return fmt.Errorf("%w: invalid address: can not be empty", errInvalidRequest)
	}

	return nil
}

func isEmailValid(e string) bool {
	if len(e) < 3 || len(e) > 254 {
		return false
	}
	if !emailRegExp.MatchString(e) {
		return false
	}

	return true
}
