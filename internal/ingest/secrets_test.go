package ingest_test

import (
	"strings"
	"testing"

	"mini-me/internal/ingest"
)

func TestSecretScannerRegex(t *testing.T) {
	scanner := ingest.NewSecretScanner()

	tests := []struct {
		name     string
		input    string
		hasSecret bool
	}{
		{
			name:     "AWS Key",
			input:    "my aws key is AKIAIOSFODNN7EXAMPLE test",
			hasSecret: true,
		},
		{
			name:     "Private Key Header",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEA...",
			hasSecret: true,
		},
		{
			name:     "GitHub PAT",
			input:    "token = ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			hasSecret: true,
		},
		{
			name:     "Normal Markdown Text",
			input:    "# Project Overview\nThis is a standard markdown file describing architecture.",
			hasSecret: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.Scan(tt.input)
			if got != tt.hasSecret {
				t.Errorf("Scan() = %v, expected %v", got, tt.hasSecret)
			}
		})
	}
}

func TestSecretScannerRedact(t *testing.T) {
	scanner := ingest.NewSecretScanner()
	input := "Set token = ghp_1234567890abcdefghijklmnopqrstuvwxyz in environment."
	redacted := scanner.Redact(input)

	if strings.Contains(redacted, "ghp_1234567890abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("expected secret to be redacted, got: %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED_SECRET]") {
		t.Errorf("expected [REDACTED_SECRET] in output, got: %s", redacted)
	}
}
