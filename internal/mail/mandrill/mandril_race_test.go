// +build !race

package mandrill

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSender_SendWelcomeEmailAsync(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		b := bytes.NewBufferString("")
		log.SetOutput(b)

		sender := createSender("SANDBOX_SUCCESS")
		sender.SendWelcomeEmailAsync(context.Background(), "test@decentrdev.com")

		time.Sleep(500 * time.Millisecond)
		assert.Empty(t, b.String())
	})

	t.Run("error", func(t *testing.T) {
		b := bytes.NewBufferString("")
		log.SetOutput(b)

		sender := createSender("SANDBOX_ERROR")
		sender.SendWelcomeEmailAsync(context.Background(), "test@decentrdev.com")

		time.Sleep(500 * time.Millisecond)
		assert.NotEmpty(t, b.String())
	})
}
