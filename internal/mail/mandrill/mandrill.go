// Package mandrill is implementation of sender interface.
package mandrill

import (
	"context"
	"fmt"

	"github.com/Decentr-net/vulcan/internal/mail"

	"github.com/keighl/mandrill"
)

const mandrillErrorStatus = "error"

type sender struct {
	config Config
	client *mandrill.Client
}

// Config ...
type Config struct {
	Subject      string
	TemplateName string

	FromName  string
	FromEmail string
}

// New returns new instance of mandrill sender.
func New(client *mandrill.Client, config Config) mail.Sender {
	s := &sender{
		client: client,
		config: config,
	}
	return s
}

// Send sends an email to account owner.
func (s *sender) Send(_ context.Context, email, owner, code string) error {
	message := mandrill.Message{
		Subject:   s.config.Subject,
		FromEmail: s.config.FromEmail,
		FromName:  s.config.FromName,
	}

	message.AddRecipient(email, "", "to")

	responses, err := s.client.MessagesSendTemplate(&message, s.config.TemplateName, map[string]interface{}{
		"owner": owner,
		"code":  code,
	})

	if err != nil {
		return err
	}

	for _, v := range responses {
		if v.Status == mandrillErrorStatus {
			return fmt.Errorf("failed to send email(%s) to %s: %s", v.Id, v.Email, v.RejectionReason) // nolint: goerr113
		}
	}

	return nil
}

// Ping ...
func (s *sender) Ping(ctx context.Context) error {
	_, err := s.client.Ping()
	return err
}
