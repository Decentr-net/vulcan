// Package mandrill is implementation of sender interface.
package mandrill

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/mail"

	"github.com/keighl/mandrill"
)

const mandrillErrorStatus = "error"

type sender struct {
	config *Config
	client *mandrill.Client
}

// Config ...
type Config struct {
	VerificationSubject      string
	VerificationTemplateName string
	WelcomeSubject           string
	WelcomeTemplateName      string

	FromName  string
	FromEmail string
}

// New returns new instance of mandrill sender.
func New(client *mandrill.Client, config *Config) mail.Sender {
	s := &sender{
		client: client,
		config: config,
	}
	return s
}

// SendVerificationEmail sends an email to account owner.
func (s *sender) SendVerificationEmail(_ context.Context, email, code string) error {
	message := mandrill.Message{
		Subject:   s.config.VerificationSubject,
		FromEmail: s.config.FromEmail,
		FromName:  s.config.FromName,
	}

	message.AddRecipient(email, "", "to")

	responses, err := s.client.MessagesSendTemplate(&message, s.config.VerificationTemplateName, map[string]interface{}{
		"code": code,
	})

	if err != nil {
		return err
	}

	for _, v := range responses {
		if v.Status == mandrillErrorStatus {
			return fmt.Errorf("failed to send verification email(%s) to %s: %s", v.Id, v.Email, v.RejectionReason) // nolint: goerr113
		}
	}

	return nil
}

// SendWelcomeEmailAsync sends an welcome email in async mode.
func (s *sender) SendWelcomeEmailAsync(_ context.Context, email string) {
	message := mandrill.Message{
		Subject:   s.config.WelcomeSubject,
		FromEmail: s.config.FromEmail,
		FromName:  s.config.FromName,
	}

	message.AddRecipient(email, "", "to")

	go func() {
		responses, err := s.client.MessagesSendTemplate(&message, s.config.WelcomeTemplateName, nil)

		if err != nil {
			log.WithError(err).WithField("email", email).Error("failed to send welcome email")
			return
		}

		for _, v := range responses {
			if v.Status == mandrillErrorStatus {
				log.WithError(errors.New(v.RejectionReason)).WithFields(map[string]interface{}{ // nolint: goerr113
					"email": email,
					"id":    v.Id,
				}).Errorf("failed to send welcome email")
				return
			}
		}
	}()
}
