package auth

import (
	"net/smtp"
	"testing"
)

func TestUsernamePasswordAuthStartUsesLoginWhenAvailable(t *testing.T) {
	auth := newSMTPAuth("smtp.office365.com", "contact@example.com", "secret")

	proto, toServer, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.office365.com",
		TLS:  true,
		Auth: []string{"LOGIN", "XOAUTH2"},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if proto != "LOGIN" {
		t.Fatalf("expected LOGIN auth, got %q", proto)
	}

	if toServer != nil {
		t.Fatalf("expected LOGIN auth to wait for a challenge, got %q", string(toServer))
	}
}

func TestUsernamePasswordAuthLoginChallenges(t *testing.T) {
	auth, ok := newSMTPAuth("smtp.office365.com", "contact@example.com", "secret").(*usernamePasswordAuth)
	if !ok {
		t.Fatal("expected concrete usernamePasswordAuth")
	}

	auth.mechanism = "LOGIN"

	username, err := auth.Next([]byte("Username:"), true)
	if err != nil {
		t.Fatalf("username challenge returned error: %v", err)
	}

	if string(username) != "contact@example.com" {
		t.Fatalf("expected username response, got %q", string(username))
	}

	password, err := auth.Next([]byte("Password:"), true)
	if err != nil {
		t.Fatalf("password challenge returned error: %v", err)
	}

	if string(password) != "secret" {
		t.Fatalf("expected password response, got %q", string(password))
	}
}

func TestUsernamePasswordAuthStartFallsBackToPlain(t *testing.T) {
	auth := newSMTPAuth("smtp.example.com", "contact@example.com", "secret")

	proto, toServer, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.example.com",
		TLS:  true,
		Auth: []string{"PLAIN"},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if proto != "PLAIN" {
		t.Fatalf("expected PLAIN auth, got %q", proto)
	}

	if string(toServer) != "\x00contact@example.com\x00secret" {
		t.Fatalf("unexpected PLAIN auth payload %q", string(toServer))
	}
}
