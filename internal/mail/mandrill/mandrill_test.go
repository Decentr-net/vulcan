package mandrill

import (
	"context"
	"testing"

	"github.com/Decentr-net/vulcan/internal/mail"

	mc "github.com/keighl/mandrill"
	"github.com/stretchr/testify/assert"
)

func createSender(apiKey string) mail.Sender {
	return New(mc.ClientWithKey(apiKey), Config{
		Subject:      "Welcome to Decentr",
		TemplateName: "welcome",
		FromName:     "Decentr",
		FromEmail:    "noreply@decentrdev.com",
	})
}

func TestSend(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sender := createSender("SANDBOX_SUCCESS")
		err := sender.Send(context.Background(), "test@decentrdev.com", "owner", "this is a code")
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		sender := createSender("SANDBOX_ERROR")
		err := sender.Send(context.Background(), "test@decentrdev.com", "owner", "this is a code")
		assert.Error(t, err)
	})
}
