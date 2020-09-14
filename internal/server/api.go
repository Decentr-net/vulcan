package server

import "github.com/go-openapi/strfmt"

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
	Email strfmt.Email `json:"email"`
}

// ConfirmResponse ...
// swagger:model
type ConfirmResponse struct {
	Address  string   `json:"address"`
	PubKey   string   `json:"public_key"`
	Mnemonic []string `json:"mnemonic"`
}
