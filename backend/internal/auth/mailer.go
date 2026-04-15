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
		auth = newSMTPAuth(host, m.config.Username, m.config.Password)
	}

	return smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(message))
}

type usernamePasswordAuth struct {
	host      string
	username  string
	password  string
	mechanism string
}

func newSMTPAuth(host, username, password string) smtp.Auth {
	return &usernamePasswordAuth{
		host:     strings.TrimSpace(host),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
	}
}

func (a *usernamePasswordAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if a == nil {
		return "", nil, fmt.Errorf("smtp auth is not configured")
	}

	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, fmt.Errorf("smtp auth requires TLS")
	}

	if a.host != "" && !strings.EqualFold(server.Name, a.host) {
		return "", nil, fmt.Errorf("smtp server name mismatch: got %q", server.Name)
	}

	if supportsSMTPAuth(server.Auth, "LOGIN") {
		a.mechanism = "LOGIN"
		return "LOGIN", nil, nil
	}

	if supportsSMTPAuth(server.Auth, "PLAIN") {
		a.mechanism = "PLAIN"
		return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
	}

	return "", nil, fmt.Errorf("smtp server does not support LOGIN or PLAIN auth")
}

func (a *usernamePasswordAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("smtp auth is not configured")
	}

	if !more {
		return nil, nil
	}

	switch a.mechanism {
	case "LOGIN":
		challenge := strings.TrimSpace(strings.ToLower(string(fromServer)))
		switch challenge {
		case "username:", "user name:":
			return []byte(a.username), nil
		case "password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unexpected LOGIN challenge: %q", string(fromServer))
		}
	case "PLAIN":
		return nil, fmt.Errorf("unexpected additional challenge for PLAIN auth")
	default:
		return nil, fmt.Errorf("smtp auth mechanism was not negotiated")
	}
}

func supportsSMTPAuth(mechanisms []string, expected string) bool {
	for _, mechanism := range mechanisms {
		if strings.EqualFold(strings.TrimSpace(mechanism), expected) {
			return true
		}
	}

	return false
}

func isLocalhost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
}

func formatAddress(name, email string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return email
	}

	return fmt.Sprintf("%s <%s>", trimmedName, email)
}
