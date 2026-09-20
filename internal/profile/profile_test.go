package profile_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mini-me/internal/profile"
	"mini-me/internal/store"
)

func TestSafetyValidator(t *testing.T) {
	// SSN Test
	err := profile.ValidateFactContent("ssn", "123-45-6789")
	if err != profile.ErrGovernmentIDDetected {
		t.Errorf("expected ErrGovernmentIDDetected, got %v", err)
	}

	// Credit Card Test
	err = profile.ValidateFactContent("payment_method", "4111-1111-1111-1111")
	if err != profile.ErrCreditCardDetected {
		t.Errorf("expected ErrCreditCardDetected, got %v", err)
	}

	// Never Infer Sensitivity Rule
	err = profile.ValidateFactSensitivity(store.SensitivityNeverInfer, false)
	if err != profile.ErrNeverInferAutomated {
		t.Errorf("expected ErrNeverInferAutomated when automated, got %v", err)
	}

	err = profile.ValidateFactSensitivity(store.SensitivityNeverInfer, true)
	if err != nil {
		t.Errorf("expected nil error when manual creation, got %v", err)
	}
}

func TestProfileGenerator(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "profile_gen.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	if err := st.SetField(ctx, "full_name", "Alice Engineer"); err != nil {
		t.Fatalf("failed setting field: %v", err)
	}
	if err := st.SetField(ctx, "email", "alice@example.com"); err != nil {
		t.Fatalf("failed setting field: %v", err)
	}

	ent := &store.Entity{
		Type: store.EntityPerson,
		Name: "Alice",
	}
	entID, err := st.CreateEntity(ctx, ent)
	if err != nil {
		t.Fatalf("failed creating entity: %v", err)
	}

	fact := &store.Fact{
		SubjectID:   entID,
		Predicate:   "likes",
		Object:      "Go Programming",
		Status:      store.FactStatusConfirmed,
		Sensitivity: store.SensitivityNormal,
	}
	if _, err := st.CreateFact(ctx, fact); err != nil {
		t.Fatalf("failed creating fact: %v", err)
	}

	gen := profile.NewGenerator(st)
	cardMarkdown, err := gen.GenerateProfileCard(ctx)
	if err != nil {
		t.Fatalf("failed generating profile card: %v", err)
	}

	if !strings.Contains(cardMarkdown, "Alice Engineer") {
		t.Errorf("expected profile card to contain full name 'Alice Engineer'")
	}
	if !strings.Contains(cardMarkdown, "likes Go Programming") {
		t.Errorf("expected profile card to contain confirmed fact 'likes Go Programming'")
	}
}
