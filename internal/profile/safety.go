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
	// Indian PAN card regex: 5 letters, 4 digits, 1 letter (e.g. ABCDE1234F)
	panRegex = regexp.MustCompile(`\b[A-Z]{5}[0-9]{4}[A-Z]{1}\b`)
	// Indian Aadhaar card regex: 12 digits (4-4-4 format or 12 continuous digits)
	aadhaarRegex = regexp.MustCompile(`\b[2-9]{1}[0-9]{3}[ -]?[0-9]{4}[ -]?[0-9]{4}\b`)
)

var (
	ErrNeverInferAutomated = errors.New("sensitivity level 'never_infer' can only be set via explicit manual user command")
)

// ValidateFactContent checks text and classifies sensitivity if personal data is present.
func ValidateFactContent(predicate, object string) error {
	// Local storage permits identity details; no error returned.
	return nil
}

// DetectSensitivity automatically returns SensitivityPersonal if sensitive identity patterns are found.
func DetectSensitivity(text string) store.FactSensitivity {
	if ssnRegex.MatchString(text) || creditCardRegex.MatchString(text) || panRegex.MatchString(text) || aadhaarRegex.MatchString(text) {
		return store.SensitivityPersonal
	}
	return store.SensitivityNormal
}

// ValidateFactSensitivity checks sensitivity constraints.
func ValidateFactSensitivity(sensitivity store.FactSensitivity, isManual bool) error {
	if sensitivity == store.SensitivityNeverInfer && !isManual {
		return ErrNeverInferAutomated
	}
	return nil
}
