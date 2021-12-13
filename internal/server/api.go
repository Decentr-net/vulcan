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
	Email        strfmt.Email `json:"email"`
	Address      string       `json:"address"`
	ReferralCode *string      `json:"referralCode"`
}

// ConfirmRequest ...
// swagger:model
type ConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// ReferralCodeResponse ...
// swagger:model
type ReferralCodeResponse struct {
	Code string `json:"code"`
}

// ReferralTrackingStatsItem ...
// swagger:model
type ReferralTrackingStatsItem struct {
	Registered int      `json:"registered"`
	Installed  int      `json:"installed"`
	Confirmed  int      `json:"confirmed"`
	Reward     sdk.Coin `json:"reward"`
}

// ReferralTrackingStatsResponse ...
// swagger:model
type ReferralTrackingStatsResponse struct {
	Total      ReferralTrackingStatsItem `json:"total"`
	Last30Days ReferralTrackingStatsItem `json:"last30Days"`
}

// RegisterStats ...
// swagger:model
type RegisterStats struct {
	Total int         `json:"total"`
	Stats []StatsItem `json:"stats"`
}

// StatsItem ...
// Date is RFC3999 date, value is number of new accounts.
type StatsItem struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

func (r RegisterRequest) validate() error {
	if !isEmailValid(r.Email.String()) {
		return fmt.Errorf("%w: invalid email", errInvalidRequest)
	}
	if !isAddressValid(r.Address) {
		return fmt.Errorf("%w: invalid address", errInvalidRequest)
	}

	return nil
}

func isAddressValid(s string) bool {
	_, err := sdk.AccAddressFromBech32(s)

	return err == nil && len(s) != 0
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
