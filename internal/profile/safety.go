package profile

import (
	"errors"
	"regexp"

	"mini-me/internal/store"
)

var (
	// SSN regex: 3 digits - 2 digits - 4 digits
	ssnRegex = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	// Credit card regex: 13-19 digits, possibly separated by spaces or hyphens
	creditCardRegex = regexp.MustCompile(`\b(?:[0-9]{4}[ -]?){3}[0-9]{1,4}\b`)
)

var (
	ErrGovernmentIDDetected = errors.New("security violation: storing government IDs (e.g. SSN) is strictly prohibited")
	ErrCreditCardDetected   = errors.New("security violation: storing payment card numbers is strictly prohibited")
	ErrNeverInferAutomated  = errors.New("sensitivity level 'never_infer' can only be set via explicit manual user command")
)

// ValidateFactContent checks text against forbidden sensitive data patterns (Government IDs, Credit Cards).
func ValidateFactContent(predicate, object string) error {
	fullText := predicate + " " + object

	if ssnRegex.MatchString(fullText) {
		return ErrGovernmentIDDetected
	}

	if creditCardRegex.MatchString(fullText) {
		return ErrCreditCardDetected
	}

	return nil
}

// ValidateFactSensitivity checks sensitivity constraints.
func ValidateFactSensitivity(sensitivity store.FactSensitivity, isManual bool) error {
	if sensitivity == store.SensitivityNeverInfer && !isManual {
		return ErrNeverInferAutomated
	}
	return nil
}
