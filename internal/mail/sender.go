// Package mail contains code for sending confirmation emails to users.
package mail

import (
	"context"
)

//go:generate mockgen -destination=./sender_mock.go -package=mail -source=sender.go

// Sender is interface for sending the emails.
type Sender interface {
	SendVerificationEmail(ctx context.Context, email, code string) error
	SendWelcomeEmailAsync(ctx context.Context, email string)
}
