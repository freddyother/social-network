package auth

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type PasswordResetMailer interface {
	SendPasswordResetEmail(ctx context.Context, toEmail, toName, resetLink string) error
}

type SMTPMailerConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type SMTPPasswordResetMailer struct {
	config SMTPMailerConfig
}

func NewSMTPPasswordResetMailer(config SMTPMailerConfig) *SMTPPasswordResetMailer {
	if strings.TrimSpace(config.Host) == "" || config.Port <= 0 || strings.TrimSpace(config.FromEmail) == "" {
		return nil
	}

	return &SMTPPasswordResetMailer{config: config}
}

func (m *SMTPPasswordResetMailer) SendPasswordResetEmail(_ context.Context, toEmail, toName, resetLink string) error {
	if m == nil {
		return fmt.Errorf("password reset mailer is not configured")
	}

	from := strings.TrimSpace(m.config.FromEmail)
	host := strings.TrimSpace(m.config.Host)
	addr := fmt.Sprintf("%s:%d", host, m.config.Port)
	subject := "Reset your NEXO password"
	recipientName := strings.TrimSpace(toName)
	if recipientName == "" {
		recipientName = "there"
	}

	message := strings.Join([]string{
		fmt.Sprintf("From: %s", formatAddress(m.config.FromName, from)),
		fmt.Sprintf("To: %s", toEmail),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		fmt.Sprintf("Hi %s,", recipientName),
		"",
		"We received a request to reset your NEXO password.",
		"",
		"Use the link below to choose a new password:",
		resetLink,
		"",
		"If you did not request this, you can safely ignore this email.",
	}, "\r\n")

	var auth smtp.Auth
	if strings.TrimSpace(m.config.Username) != "" || strings.TrimSpace(m.config.Password) != "" {
		auth = smtp.PlainAuth("", m.config.Username, m.config.Password, host)
	}

	return smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(message))
}

func formatAddress(name, email string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return email
	}

	return fmt.Sprintf("%s <%s>", trimmedName, email)
}
