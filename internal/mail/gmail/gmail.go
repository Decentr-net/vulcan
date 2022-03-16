// Package gmail is implementation of sender interface.
package gmail

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"net/smtp"
	"text/template"

	"github.com/sirupsen/logrus"

	"github.com/Decentr-net/vulcan/internal/mail"
)

const (
	host = "smtp.gmail.com"
)

// nolint:gochecknoglobals
var (
	//go:embed tmpl/*.html
	templates embed.FS
)

// Config ...
type Config struct {
	VerificationSubject string
	WelcomeSubject      string

	FromName     string
	FromPassword string
	FromEmail    string
}

type sender struct {
	config *Config
	auth   smtp.Auth

	templates *template.Template
}

// New returns new instance of mandrill sender.
func New(config *Config) mail.Sender {
	auth := smtp.PlainAuth(config.FromName, config.FromEmail, config.FromPassword, host)
	return &sender{
		auth:      auth,
		config:    config,
		templates: template.Must(template.ParseFS(templates, "tmpl/*")),
	}
}

// SendVerificationEmailAsync sends an email to account owner.
func (s *sender) SendVerificationEmailAsync(_ context.Context, email, code string) {
	log := logrus.WithFields(logrus.Fields{
		"to": email,
	})

	var body bytes.Buffer
	err := s.templates.ExecuteTemplate(&body, "confirm.html", struct {
		Code    string
		Subject string
	}{
		Code:    code,
		Subject: s.config.VerificationSubject,
	})

	if err != nil {
		log.WithError(err).Error("failed to execute confirm template")
		return
	}

	go func() {
		if err := s.sendEmail(s.config.VerificationSubject, email, body.String()); err != nil {
			log.WithError(err).Error("failed to send email")
		}
	}()
}

// SendWelcomeEmailAsync sends an welcome email in async mode.
func (s *sender) SendWelcomeEmailAsync(_ context.Context, email string) {
	log := logrus.WithFields(logrus.Fields{
		"to": email,
	})

	var body bytes.Buffer
	err := s.templates.ExecuteTemplate(&body, "welcome.html", struct {
		Subject string
	}{
		Subject: s.config.WelcomeSubject,
	})

	if err != nil {
		log.WithError(err).Error("failed to execute welcome template")
		return
	}

	go func() {
		if err := s.sendEmail(s.config.WelcomeSubject, email, body.String()); err != nil {
			log.WithError(err).Error("failed to send email")
		}
	}()
}

func (s *sender) sendEmail(subj, to, body string) error {
	headerSubj := fmt.Sprintf("Subject: %s\n", subj)
	headerTo := fmt.Sprintf("To: %s\n", to)
	headerMime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	return smtp.SendMail(
		host+":587", s.auth, s.config.FromName, []string{to},
		[]byte(headerSubj+headerTo+headerMime+body))
}
