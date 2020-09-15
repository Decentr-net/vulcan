// Package sendpulse is implementation of sender interface.
package sendpulse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2/clientcredentials"

	"github.com/Decentr-net/vulcan/internal/health"
	"github.com/Decentr-net/vulcan/internal/mail"
)

// nolint: gosec
const (
	tokenURL = "https://api.sendpulse.com/oauth/access_token"
	smtpURL  = "https://api.sendpulse.com/smtp/emails"
	pingURL  = "https://api.sendpulse.com/templates"
)

// Config ...
type Config struct {
	Subject    string
	TemplateID uint64

	FromName  string
	FromEmail string
}

type sender struct {
	client *http.Client

	config Config
}

type successResponse struct {
	Result bool   `json:"result"`
	ID     string `json:"id"`
}

type errorResponse struct {
	Code    int    `json:"error_code"`
	Message string `json:"message"`
}

// New returns new instance of sendpulse sender.
func New(clientID, clientSecret string, timeout time.Duration, c Config) (mail.Sender, health.Pinger) {
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
	}

	client := config.Client(context.Background())
	client.Timeout = timeout

	s := &sender{
		client: client,
		config: c,
	}

	return s, s
}

// Ping ...
func (s *sender) Ping(ctx context.Context) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return fmt.Errorf("sendpulse: failed to create request: %w", err)
	}

	resp, err := s.client.Do(r)
	if err != nil {
		return fmt.Errorf("sendpulse: failed to get ping url: %w", err)
	}
	defer resp.Body.Close() // nolint

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sendpulse: invalid status code(%d) was received", resp.StatusCode) // nolint
	}

	return nil
}

// Send sends an email to account owner.
func (s *sender) Send(ctx context.Context, email, owner, code string) error {
	data := map[string]interface{}{
		"email": map[string]interface{}{
			"subject": s.config.Subject,
			"template": map[string]interface{}{
				"id": s.config.TemplateID,
				"variables": map[string]interface{}{
					"owner": owner,
					"code":  code,
				},
			},
			"from": map[string]interface{}{
				"name":  s.config.FromName,
				"email": s.config.FromEmail,
			},
			"to": []interface{}{
				map[string]interface{}{
					"email": email,
				},
			},
		},
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, smtpURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close() // nolint

	if resp.StatusCode != http.StatusOK {
		var res errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return fmt.Errorf("failed to unmarshall error: %w", err)
		}

		return fmt.Errorf("sendpulse error %d: %s", res.Code, res.Message) // nolint
	}

	var res successResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return fmt.Errorf("failed to unmarshall response: %w", err)
	}

	if !res.Result {
		return fmt.Errorf("sendpulse returned false result with id=%s", res.ID) // nolint
	}

	return nil
}
