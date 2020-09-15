// Package mail contains code for sending confirmation emails to users.
package mail

import (
	"context"
)

//go:generate mockgen -destination=./sender_mock.go -package=mail -source=sender.go

// Sender is interface for sending the emails.
type Sender interface {
	Send(ctx context.Context, email, owner, code string) error
}
