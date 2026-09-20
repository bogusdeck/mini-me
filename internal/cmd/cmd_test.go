package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestAddSearchReindexCmd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cmd_search_test.db")
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
	if err := os.WriteFile(sampleFile, []byte("# Test Note\nThis is sample content for testing add and search commands."), 0644); err != nil {
		t.Fatalf("failed creating sample file: %v", err)
	}

	// 1. Test add
	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"add", sampleFile, "--project", "test-project"})

	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// 2. Test search
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"search", "sample content", "--project", "test-project", "-k", "5"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	// 3. Test reindex
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"reindex"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("reindex command failed: %v", err)
	}
}

func TestProfileFactInitReviewCmds(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cmd_profile_test.db")
	t.Setenv(config.EnvDBPath, dbPath)

	// 1. Test init
	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"init", "--name", "Alice Engineer", "--email", "alice@test.com", "--employer", "Acme Corp"})

	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	// 2. Test fact add
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"fact", "add", "Alice Engineer", "likes", "Go Programming", "--status", "confirmed", "--sensitivity", "normal"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("fact add command failed: %v", err)
	}

	// 3. Test fact add proposed
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"fact", "add", "Alice Engineer", "knows", "Bob Developer", "--status", "proposed"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("fact add proposed command failed: %v", err)
	}

	// 4. Test review
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"review"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("review command failed: %v", err)
	}

	// 5. Test fact confirm
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"fact", "confirm", "2"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("fact confirm command failed: %v", err)
	}

	// 6. Test profile card generation
	buf.Reset()
	cmd.RootCmd.SetArgs([]string{"profile"})
	if err := cmd.RootCmd.Execute(); err != nil {
		t.Fatalf("profile command failed: %v", err)
	}
}

func TestGovernmentIDRejectionInFactAdd(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cmd_security_test.db")
	t.Setenv(config.EnvDBPath, dbPath)

	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"fact", "add", "Alice", "ssn", "123-45-6789"})

	err := cmd.RootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error when adding SSN fact, got nil")
	}
	if !strings.Contains(err.Error(), "government IDs") {
		t.Errorf("expected government ID error, got: %v", err)
	}
}
