// Package mail contains code for sending confirmation emails to users.
package mail

import (
	"context"
	"errors"
)

//go:generate mockgen -destination=./mock/sender.go -package=mock -source=sender.go

// ErrMailRejected is returned when email sending attempt is rejected.
var ErrMailRejected = errors.New("email is rejected")

// Sender is interface for sending the emails.
type Sender interface {
	SendVerificationEmail(ctx context.Context, email, code string) error
	SendWelcomeEmailAsync(ctx context.Context, email string)
}
