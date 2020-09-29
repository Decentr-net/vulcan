package mandrill

import (
	"context"
	"testing"

	"github.com/Decentr-net/vulcan/internal/mail"

	mc "github.com/keighl/mandrill"
	"github.com/stretchr/testify/assert"
)

func createSender(apiKey string) mail.Sender {
	return New(mc.ClientWithKey(apiKey), &Config{
		VerificationSubject:      "Welcome to Decentr",
		VerificationTemplateName: "welcome",
		WelcomeSubject:           "Welcome to Decentr",
		WelcomeTemplateName:      "welcome",
		FromName:                 "Decentr",
		FromEmail:                "noreply@decentrdev.com",
	})
}

func TestSender_SendVerificationEmailSend(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sender := createSender("SANDBOX_SUCCESS")
		err := sender.SendVerificationEmail(context.Background(), "test@decentrdev.com", "this is a code")
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		sender := createSender("SANDBOX_ERROR")
		err := sender.SendVerificationEmail(context.Background(), "test@decentrdev.com", "this is a code")
		assert.Error(t, err)
	})
}
