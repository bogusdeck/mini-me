package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"mini-me/internal/profile"
	"mini-me/internal/search"
	"mini-me/internal/store"
)

const UntrustedHeader = "[UNTRUSTED RETRIEVED DATA]\n"

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type Server struct {
	store    *store.Store
	embedder search.QueryEmbedder
	in       io.Reader
	out      io.Writer
}

func NewServer(st *store.Store, embedder search.QueryEmbedder, in io.Reader, out io.Writer) *Server {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &Server{
		store:    st,
		embedder: embedder,
		in:       in,
		out:      out,
	}
}

func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		resp := s.handleRPC(ctx, &req)
		if resp != nil {
			respBytes, err := json.Marshal(resp)
			if err == nil {
				_, _ = fmt.Fprintf(s.out, "%s\n", respBytes)
			}
		}
	}
	return scanner.Err()
}

func (s *Server) handleRPC(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "mini-me",
					"version": "0.1.0",
				},
			},
		}

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": []map[string]any{
					{
						"name":        "get_profile",
						"description": "Get the personal profile card.",
						"inputSchema": map[string]any{
							"type":       "object",
							"properties": map[string]any{},
						},
					},
					{
						"name":        "get_person",
						"description": "Get details and facts for a person entity.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{"type": "string"},
							},
							"required": []string{"name"},
						},
					},
					{
						"name":        "get_project",
						"description": "Get details and facts for a project entity.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{"type": "string"},
							},
							"required": []string{"name"},
						},
					},
					{
						"name":        "search",
						"description": "Search the knowledge base.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query":   map[string]any{"type": "string"},
								"project": map[string]any{"type": "string"},
								"k":       map[string]any{"type": "integer"},
							},
							"required": []string{"query"},
						},
					},
					{
						"name":        "propose_fact",
						"description": "Propose a fact for user review. Writes ONLY to review queue with status=proposed.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"subject":   map[string]any{"type": "string"},
								"predicate": map[string]any{"type": "string"},
								"object":    map[string]any{"type": "string"},
							},
							"required": []string{"subject", "predicate", "object"},
						},
					},
				},
			},
		}

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: -32602, Message: "invalid params"},
			}
		}

		resText, err := s.ExecuteTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"isError": true,
					"content": []map[string]any{
						{"type": "text", "text": err.Error()},
					},
				},
			}
		}

		// Wrap all retrieved output in Untrusted Data header
		untrustedOutput := UntrustedHeader + resText

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": untrustedOutput},
				},
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: "method not found"},
		}
	}
}

// ExecuteTool dispatches tool execution.
func (s *Server) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	switch toolName {
	case "get_profile":
		gen := profile.NewGenerator(s.store)
		return gen.GenerateProfileCard(ctx)

	case "get_person":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("missing argument 'name'")
		}
		ent, err := s.store.GetEntityByName(ctx, name)
		if err != nil || ent == nil {
			return fmt.Errorf("person entity %q not found", name).Error(), nil
		}
		if ent.Type != store.EntityPerson {
			return fmt.Errorf("entity %q is type %s, not person", name, ent.Type).Error(), nil
		}
		facts, _ := s.store.ListFactsByStatus(ctx, store.FactStatusConfirmed)
		var text strings.Builder
		text.WriteString(fmt.Sprintf("Person: %s\nNotes: %s\nConfirmed Facts:\n", ent.Name, ent.Notes))
		for _, f := range facts {
			if f.SubjectID == ent.ID {
				text.WriteString(fmt.Sprintf("- %s %s %s\n", ent.Name, f.Predicate, f.Object))
			}
		}
		return text.String(), nil

	case "get_project":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("missing argument 'name'")
		}
		ent, err := s.store.GetEntityByName(ctx, name)
		if err != nil || ent == nil {
			return fmt.Errorf("project entity %q not found", name).Error(), nil
		}
		facts, _ := s.store.ListFactsByStatus(ctx, store.FactStatusConfirmed)
		var text strings.Builder
		text.WriteString(fmt.Sprintf("Project: %s\nNotes: %s\nConfirmed Facts:\n", ent.Name, ent.Notes))
		for _, f := range facts {
			if f.SubjectID == ent.ID {
				text.WriteString(fmt.Sprintf("- %s %s %s\n", ent.Name, f.Predicate, f.Object))
			}
		}
		return text.String(), nil

	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("missing argument 'query'")
		}
		proj, _ := args["project"].(string)
		k := 10
		if kVal, ok := args["k"].(float64); ok && kVal > 0 {
			k = int(kVal)
		}

		engine := search.NewSearchEngine(s.store, s.embedder)
		hits, err := engine.Search(ctx, query, search.SearchOptions{Project: proj, K: k})
		if err != nil {
			return "", err
		}
		if len(hits) == 0 {
			return "No search results found.", nil
		}
		var text strings.Builder
		for i, h := range hits {
			text.WriteString(fmt.Sprintf("%d. [%s] %s\n%s\n\n", i+1, h.Chunk.Project, h.Chunk.Path, h.Chunk.Text))
		}
		return text.String(), nil

	case "propose_fact":
		subj, _ := args["subject"].(string)
		pred, _ := args["predicate"].(string)
		obj, _ := args["object"].(string)
		if subj == "" || pred == "" || obj == "" {
			return "", fmt.Errorf("missing subject, predicate, or object")
		}

		// Security Check against SSN / Credit Cards
		if err := profile.ValidateFactContent(pred, obj); err != nil {
			return "", err
		}

		// Ensure entity exists or create it
		ent, _ := s.store.GetEntityByName(ctx, subj)
		var subjID int64
		if ent == nil {
			subjID, _ = s.store.CreateEntity(ctx, &store.Entity{
				Type: store.EntityPerson,
				Name: subj,
			})
		} else {
			subjID = ent.ID
		}

		// STRICT CONSTRAINT: MUST write status = 'proposed' ONLY!
		fact := &store.Fact{
			SubjectID:   subjID,
			Predicate:   pred,
			Object:      obj,
			Status:      store.FactStatusProposed, // ALWAYS proposed
			Sensitivity: store.SensitivityNormal,  // NEVER never_infer
		}

		factID, err := s.store.CreateFact(ctx, fact)
		if err != nil {
			return "", fmt.Errorf("failed creating proposed fact: %w", err)
		}

		return fmt.Sprintf("Fact #%d proposed successfully and written to review queue.", factID), nil

	default:
		return "", fmt.Errorf("unknown tool %q", toolName)
	}
}
