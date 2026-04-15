package config

import "testing"

func TestLoadUsesSpringMailFallbacks(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM_EMAIL", "")
	t.Setenv("SPRING_MAIL_HOST", "smtp.office365.com")
	t.Setenv("SPRING_MAIL_PORT", "587")
	t.Setenv("SPRING_MAIL_USERNAME", "contact@example.com")
	t.Setenv("SPRING_MAIL_PASSWORD", "secret")

	cfg := Load()

	if cfg.Mail.SMTPHost != "smtp.office365.com" {
		t.Fatalf("expected SMTP host fallback, got %q", cfg.Mail.SMTPHost)
	}

	if cfg.Mail.SMTPPort != 587 {
		t.Fatalf("expected SMTP port fallback, got %d", cfg.Mail.SMTPPort)
	}

	if cfg.Mail.Username != "contact@example.com" {
		t.Fatalf("expected SMTP username fallback, got %q", cfg.Mail.Username)
	}

	if cfg.Mail.Password != "secret" {
		t.Fatalf("expected SMTP password fallback, got %q", cfg.Mail.Password)
	}

	if cfg.Mail.FromEmail != "contact@example.com" {
		t.Fatalf("expected from email to fall back to SMTP username, got %q", cfg.Mail.FromEmail)
	}
}

func TestLoadPrefersExplicitSMTPVariables(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.sendgrid.net")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "nexo@example.com")
	t.Setenv("SMTP_PASSWORD", "smtp-secret")
	t.Setenv("SMTP_FROM_EMAIL", "support@nexo.example.com")
	t.Setenv("SPRING_MAIL_HOST", "smtp.office365.com")
	t.Setenv("SPRING_MAIL_PORT", "587")
	t.Setenv("SPRING_MAIL_USERNAME", "contact@example.com")
	t.Setenv("SPRING_MAIL_PASSWORD", "spring-secret")

	cfg := Load()

	if cfg.Mail.SMTPHost != "smtp.sendgrid.net" {
		t.Fatalf("expected explicit SMTP host, got %q", cfg.Mail.SMTPHost)
	}

	if cfg.Mail.SMTPPort != 2525 {
		t.Fatalf("expected explicit SMTP port, got %d", cfg.Mail.SMTPPort)
	}

	if cfg.Mail.Username != "nexo@example.com" {
		t.Fatalf("expected explicit SMTP username, got %q", cfg.Mail.Username)
	}

	if cfg.Mail.Password != "smtp-secret" {
		t.Fatalf("expected explicit SMTP password, got %q", cfg.Mail.Password)
	}

	if cfg.Mail.FromEmail != "support@nexo.example.com" {
		t.Fatalf("expected explicit from email, got %q", cfg.Mail.FromEmail)
	}
}
