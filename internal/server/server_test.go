package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"mini-me/internal/server"
	"mini-me/internal/store"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return make([]float32, 256), nil
}

func TestHTTPServerAuthAndEndpoints(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_server.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	if err := st.SetField(ctx, "full_name", "Test User"); err != nil {
		t.Fatalf("failed setting field: %v", err)
	}

	token := "test-secret-bearer-token-123456789"
	srv := server.NewServer(st, &mockEmbedder{}, "127.0.0.1", 0, token)

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Give server a moment to bind
	time.Sleep(100 * time.Millisecond)
	defer srv.Close()

	serverURL := "http://" + srv.Addr()

	// 1. Test Without Bearer Token (Should Return 401 Unauthorized)
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/v1/profile", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
	}

	// 2. Test With Valid Bearer Token (GET /v1/profile)
	req, _ = http.NewRequest(http.MethodGet, serverURL+"/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated http request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var profileData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&profileData); err != nil {
		t.Fatalf("failed decoding profile json: %v", err)
	}
	if _, ok := profileData["profile_card"]; !ok {
		t.Errorf("expected 'profile_card' in response")
	}

	// 3. Test GET /v1/profile/field?name=full_name
	req, _ = http.NewRequest(http.MethodGet, serverURL+"/v1/profile/field?name=full_name", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("field request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for field, got %d", resp.StatusCode)
	}

	var fieldData store.Field
	if err := json.NewDecoder(resp.Body).Decode(&fieldData); err != nil {
		t.Fatalf("failed decoding field json: %v", err)
	}
	if fieldData.Value != "Test User" {
		t.Errorf("expected field value 'Test User', got %q", fieldData.Value)
	}

	// 4. Test POST /v1/search
	searchBody, _ := json.Marshal(map[string]any{"query": "test query", "k": 5})
	req, _ = http.NewRequest(http.MethodPost, serverURL+"/v1/search", bytes.NewReader(searchBody))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("search request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for search, got %d", resp.StatusCode)
	}
}
