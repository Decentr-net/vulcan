// Package mandrill is implementation of sender interface.
package mandrill

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/mail"

	"github.com/keighl/mandrill"
)

const mandrillSentStatus = "sent"
const mandrillQueuedStatus = "queued"

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

// SendVerificationEmailAsync sends an email to account owner.
func (s *sender) SendVerificationEmailAsync(_ context.Context, email, code string) {
	message := mandrill.Message{
		Subject:   s.config.VerificationSubject,
		FromEmail: s.config.FromEmail,
		FromName:  s.config.FromName,
		GlobalMergeVars: mandrill.ConvertMapToVariables(map[string]interface{}{
			"CODE": code,
		}),
	}

	message.AddRecipient(email, "", "to")

	go func() {
		responses, err := s.client.MessagesSendTemplate(&message, s.config.VerificationTemplateName, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"email": email,
			}).WithError(err).Error("failed to send email")
		}

		for _, v := range responses {
			if v.Status != mandrillSentStatus && v.Status != mandrillQueuedStatus {
				log.WithFields(log.Fields{
					"email":  email,
					"reason": v.RejectionReason,
					"id":     v.Id,
					"status": v.Status,
				}).WithError(mail.ErrMailRejected).Error("failed to send email")
				return
			}
		}
	}()
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
			if v.Status != mandrillSentStatus && v.Status != mandrillQueuedStatus {
				log.WithError(errors.New(v.RejectionReason)).WithFields(map[string]interface{}{ // nolint: goerr113
					"email":            email,
					"id":               v.Id,
					"status":           v.Status,
					"rejection_reason": v.RejectionReason,
				}).Errorf("failed to send welcome email")
				return
			}
		}
	}()
}
