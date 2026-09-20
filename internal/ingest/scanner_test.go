package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mini-me/internal/ingest"
	"mini-me/internal/store"
)

func TestDeepScannerDocumentClassification(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "scanner_test.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	// 1. Create a dummy Resume
	resumePath := filepath.Join(tmpDir, "John_Doe_Resume.md")
	_ = os.WriteFile(resumePath, []byte("# John Doe\nSoftware Engineer with Go & Python experience."), 0644)

	// 2. Create a dummy PAN Card file
	panPath := filepath.Join(tmpDir, "PAN_Card_ABCDE1234F.txt")
	_ = os.WriteFile(panPath, []byte("Income Tax Department\nPAN: ABCDE1234F\nName: John Doe"), 0644)

	// 3. Create a dummy Git Repo folder
	repoDir := filepath.Join(tmpDir, "my-awesome-app")
	_ = os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# My Awesome App\nA Go web application."), 0644)

	scanner := ingest.NewDeepScanner(st, &mockEmbedder{})
	stats, err := scanner.ScanUserDirectories(ctx, tmpDir)
	if err != nil {
		t.Fatalf("deep scan failed: %v", err)
	}

	if stats.ResumesFound != 1 {
		t.Errorf("expected 1 resume found, got %d", stats.ResumesFound)
	}
	if stats.IdentityDocsFound != 1 {
		t.Errorf("expected 1 identity doc found, got %d", stats.IdentityDocsFound)
	}
	if stats.ProjectsFound != 1 {
		t.Errorf("expected 1 project found, got %d", stats.ProjectsFound)
	}

	// Verify resume path saved in identity fields
	field, err := st.GetField(ctx, "latest_resume_path")
	if err != nil || field == nil {
		t.Fatalf("expected latest_resume_path field set, got err: %v", err)
	}
	if field.Value != resumePath {
		t.Errorf("expected latest_resume_path %q, got %q", resumePath, field.Value)
	}

	// Verify PAN card path saved in identity fields
	panField, err := st.GetField(ctx, "pan_card_path")
	if err != nil || panField == nil {
		t.Fatalf("expected pan_card_path field set, got err: %v", err)
	}
	if panField.Value != panPath {
		t.Errorf("expected pan_card_path %q, got %q", panPath, panField.Value)
	}
}
