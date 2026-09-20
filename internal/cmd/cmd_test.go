package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"mini-me/internal/cmd"
	"mini-me/internal/config"
	"mini-me/internal/embed"
)

func TestVersionCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"version"})

	err := cmd.RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "doctor_test.db")
	t.Setenv(config.EnvDBPath, dbPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ollama is running"))
			return
		}
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(embed.TagsResponse{
				Models: []embed.ModelInfo{
					{Name: "nomic-embed-text:latest", Model: "nomic-embed-text:latest"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)

	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"doctor"})

	err := cmd.RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error running doctor: %v", err)
	}

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("[OK]")) {
		t.Errorf("expected [OK] in doctor output, got:\n%s", output)
	}
}

func TestAddAndReindexCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cmd_ingest_test.db")
	t.Setenv(config.EnvDBPath, dbPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			w.Header().Set("Content-Type", "application/json")
			vecs := [][]float32{make([]float32, 768)}
			_ = json.NewEncoder(w).Encode(embed.EmbedResponse{Embeddings: vecs})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)

	sampleFile := filepath.Join(tmpDir, "sample.md")
	if err := os.WriteFile(sampleFile, []byte("# Test Note\nThis is sample content for testing add command."), 0644); err != nil {
		t.Fatalf("failed creating sample file: %v", err)
	}

	// Test add
	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"add", sampleFile, "--project", "test-project"})

	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Test reindex
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"reindex"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("reindex command failed: %v", err)
	}
}
