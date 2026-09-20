package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"mini-me/internal/mcp"
	"mini-me/internal/store"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return make([]float32, 256), nil
}

func TestMCPServerToolsAndUntrustedLabeling(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_mcp.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	if err := st.SetField(ctx, "full_name", "MCP Test User"); err != nil {
		t.Fatalf("failed setting field: %v", err)
	}

	inBuf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)

	// Send tools/call for get_profile
	callReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "get_profile", "arguments": {}}`),
	}
	callBytes, _ := json.Marshal(callReq)
	inBuf.Write(callBytes)
	inBuf.WriteString("\n")

	srv := mcp.NewServer(st, &mockEmbedder{}, inBuf, outBuf)
	if err := srv.Run(ctx); err != nil {
		t.Fatalf("mcp server error: %v", err)
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(outBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed parsing mcp response: %v", err)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result in mcp response")
	}

	contentList, ok := resMap["content"].([]any)
	if !ok || len(contentList) == 0 {
		t.Fatalf("expected non-empty content list")
	}

	firstContent := contentList[0].(map[string]any)
	textVal, ok := firstContent["text"].(string)
	if !ok {
		t.Fatalf("expected text string in content")
	}

	// Verify UNTRUSTED RETRIEVED DATA header
	if !strings.HasPrefix(textVal, mcp.UntrustedHeader) {
		t.Errorf("expected output to begin with untrusted header %q, got:\n%s", mcp.UntrustedHeader, textVal)
	}

	if !strings.Contains(textVal, "MCP Test User") {
		t.Errorf("expected text content to contain 'MCP Test User'")
	}
}

func TestMCPProposeFactOnlyWritesProposedStatus(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_mcp_propose.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	srv := mcp.NewServer(st, &mockEmbedder{}, nil, nil)

	// Execute propose_fact tool
	args := map[string]any{
		"subject":   "Charlie",
		"predicate": "likes",
		"object":    "Python",
	}

	resultText, err := srv.ExecuteTool(ctx, "propose_fact", args)
	if err != nil {
		t.Fatalf("propose_fact failed: %v", err)
	}
	if !strings.Contains(resultText, "proposed successfully") {
		t.Errorf("expected success message, got %q", resultText)
	}

	// Verify in DB that status is 'proposed', NOT 'confirmed'
	proposedFacts, err := st.ListFactsByStatus(ctx, store.FactStatusProposed)
	if err != nil || len(proposedFacts) != 1 {
		t.Fatalf("expected 1 proposed fact in review queue, got %d (err: %v)", len(proposedFacts), err)
	}

	if proposedFacts[0].Status != store.FactStatusProposed {
		t.Errorf("expected status 'proposed', got %s", proposedFacts[0].Status)
	}

	// Verify that confirmed facts queue is empty (MCP tool cannot mutate confirmed state!)
	confirmedFacts, err := st.ListFactsByStatus(ctx, store.FactStatusConfirmed)
	if err != nil {
		t.Fatalf("error checking confirmed facts: %v", err)
	}
	if len(confirmedFacts) != 0 {
		t.Errorf("expected 0 confirmed facts (MCP tools must be read-only for confirmed state), got %d", len(confirmedFacts))
	}
}
