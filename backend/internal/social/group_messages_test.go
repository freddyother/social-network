package social

import (
	"strings"
	"testing"
)

func TestNormalizeSendGroupMessageInputTrimsBody(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeSendGroupMessageInput(SendGroupMessageInput{
		Body: "  hello group  ",
	})
	if err != nil {
		t.Fatalf("normalizeSendGroupMessageInput returned error: %v", err)
	}

	if normalized.Body != "hello group" {
		t.Fatalf("expected trimmed body, got %q", normalized.Body)
	}
}

func TestNormalizeSendGroupMessageInputRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty",
			body: "   ",
			want: "Write a message before sending.",
		},
		{
			name: "too long",
			body: strings.Repeat("a", maxGroupMessageBodyLength+1),
			want: "Messages must be 2000 characters or fewer.",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeSendGroupMessageInput(SendGroupMessageInput{Body: tt.body})
			if err == nil {
				t.Fatal("expected validation error")
			}

			validationErr, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("expected ValidationError, got %T", err)
			}

			if got := validationErr.Fields["body"]; got != tt.want {
				t.Fatalf("unexpected body error: %q", got)
			}
		})
	}
}
