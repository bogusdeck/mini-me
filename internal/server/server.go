package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mini-me/internal/config"
	"mini-me/internal/profile"
	"mini-me/internal/search"
	"mini-me/internal/store"
)

const AuthTokenFileName = "auth_token"

type Server struct {
	store     *store.Store
	embedder  search.QueryEmbedder
	token     string
	host      string
	port      int
	listener  net.Listener
	httpServer *http.Server
}

func GetOrCreateAuthToken() (string, error) {
	configDir, err := config.DefaultConfigDir()
	if err != nil {
		return "", err
	}
	if err := config.EnsureDir(configDir); err != nil {
		return "", err
	}

	tokenPath := filepath.Join(configDir, AuthTokenFileName)
	if data, err := os.ReadFile(tokenPath); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	}

	// Generate 32-byte random hex token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	newToken := hex.EncodeToString(bytes)

	if err := os.WriteFile(tokenPath, []byte(newToken+"\n"), 0600); err != nil {
		return "", fmt.Errorf("failed to write auth token file: %w", err)
	}

	return newToken, nil
}

func NewServer(st *store.Store, embedder search.QueryEmbedder, host string, port int, token string) *Server {
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8080
	}
	return &Server{
		store:    st,
		embedder: embedder,
		host:     host,
		port:     port,
		token:    token,
	}
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}
	s.listener = l

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/profile", s.authMiddleware(s.handleProfile))
	mux.HandleFunc("/v1/profile/field", s.authMiddleware(s.handleProfileField))
	mux.HandleFunc("/v1/entity", s.authMiddleware(s.handleEntity))
	mux.HandleFunc("/v1/search", s.authMiddleware(s.handleSearch))

	s.httpServer = &http.Server{
		Handler: mux,
	}

	return s.httpServer.Serve(s.listener)
}

func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

func (s *Server) Close() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" || token != s.token {
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gen := profile.NewGenerator(s.store)
	card, err := gen.GenerateProfileCard(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	fields, _ := s.store.ListFields(r.Context())
	fieldMap := make(map[string]string)
	for _, f := range fields {
		fieldMap[f.Key] = f.Value
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"profile_card": card,
		"fields":       fieldMap,
	})
}

func (s *Server) handleProfileField(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("name")
	if key == "" {
		http.Error(w, `{"error": "missing name parameter"}`, http.StatusBadRequest)
		return
	}

	f, err := s.store.GetField(r.Context(), key)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, `{"error": "field not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f)
}

func (s *Server) handleEntity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, `{"error": "missing name parameter"}`, http.StatusBadRequest)
		return
	}

	ent, err := s.store.GetEntityByName(r.Context(), name)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}
	if ent == nil {
		http.Error(w, `{"error": "entity not found"}`, http.StatusNotFound)
		return
	}

	facts, _ := s.store.ListFactsByStatus(r.Context(), store.FactStatusConfirmed)
	var entFacts []*store.Fact
	for _, f := range facts {
		if f.SubjectID == ent.ID {
			entFacts = append(entFacts, f)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"entity": ent,
		"facts":  entFacts,
	})
}

type SearchRequest struct {
	Query   string            `json:"query"`
	Filters map[string]string `json:"filters"`
	K       int               `json:"k"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid json payload"}`, http.StatusBadRequest)
		return
	}

	engine := search.NewSearchEngine(s.store, s.embedder)
	opts := search.SearchOptions{
		K: req.K,
	}
	if req.Filters != nil {
		opts.Project = req.Filters["project"]
		opts.SourceType = req.Filters["source_type"]
	}

	hits, err := engine.Search(r.Context(), req.Query, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"hits": hits,
	})
}
