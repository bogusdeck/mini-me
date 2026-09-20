package ingest

import (
	"math"
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	// AWS Key IDs
	regexp.MustCompile(`(?i)(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`),
	// Private Keys
	regexp.MustCompile(`-----BEGIN (RSA|OPENSSH|DSA|EC|PGP) PRIVATE KEY-----`),
	// API Tokens (Slack, GitHub, Stripe, OpenAI)
	regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z]{10,48}`),
	regexp.MustCompile(`gh[pousr]_[0-9a-zA-Z]{36}`),
	regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24}`),
	regexp.MustCompile(`sk-[a-zA-Z0-9]{32,48}`),
	// Generic Key/Secret assignments
	regexp.MustCompile(`(?i)(api_key|access_token|secret_key|private_key|auth_token)\s*[:=]\s*["']?([A-Za-z0-9_\-\./+]{20,})["']?`),
}

// CalculateEntropy computes Shannon entropy (in bits per character) for a string.
func CalculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0.0
	}
	counts := make(map[rune]float64)
	for _, r := range s {
		counts[r]++
	}
	length := float64(len([]rune(s)))
	var entropy float64
	for _, count := range counts {
		p := count / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// SecretScanner scans text for secrets and offers detection & redaction.
type SecretScanner struct{}

func NewSecretScanner() *SecretScanner {
	return &SecretScanner{}
}

// Scan returns true if a secret or high-entropy key pattern is detected.
func (s *SecretScanner) Scan(text string) bool {
	for _, pat := range secretPatterns {
		if pat.MatchString(text) {
			return true
		}
	}

	// Check words with high entropy (length >= 20 and entropy > 4.5)
	words := strings.Fields(text)
	for _, w := range words {
		wClean := strings.Trim(w, `"';:=,{}()[]`)
		if len(wClean) >= 20 && CalculateEntropy(wClean) >= 4.5 {
			return true
		}
	}

	return false
}

// Redact replaces detected secret patterns and high entropy words with [REDACTED_SECRET].
func (s *SecretScanner) Redact(text string) string {
	result := text

	// Apply pattern replacements
	for _, pat := range secretPatterns {
		result = pat.ReplaceAllStringFunc(result, func(match string) string {
			return "[REDACTED_SECRET]"
		})
	}

	// Apply high entropy word replacements
	words := strings.Fields(result)
	for _, w := range words {
		wClean := strings.Trim(w, `"';:=,{}()[]`)
		if len(wClean) >= 20 && CalculateEntropy(wClean) >= 4.5 {
			result = strings.ReplaceAll(result, w, "[REDACTED_SECRET]")
		}
	}

	return result
}
