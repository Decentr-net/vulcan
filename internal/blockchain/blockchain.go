// Package blockchain contains code for interacting with the decentr blockchain.
package blockchain

import "context"

//go:generate mockgen -destination=./blockchain_mock.go -package=blockchain -source=blockchain.go

// AccountInfo ...
type AccountInfo struct {
	Mnemonic []string
	PubKey   string
	Address  string
}

// Blockchain is interface for interacting with the blockchain.
type Blockchain interface {
	SendStakes(ctx context.Context, address string, amount int64) error
}
