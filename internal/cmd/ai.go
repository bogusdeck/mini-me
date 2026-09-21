package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/profile"
	"mini-me/internal/search"
	"mini-me/internal/store"
)

var (
	flagChatModel   string
	flagChatNoRAG   bool
	flagChatMaxHits int
)

var aiCmd = &cobra.Command{
	Use:     "ai [prompt]",
	Aliases: []string{"chat"},
	Short:   "Talk directly with mini-me using local Ollama LLM + RAG context",
	Long:    `ai (or chat) queries your local Ollama LLM enriched with your profile card and local RAG database context.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		embedClient := embed.NewClient()
		searchEngine := search.NewSearchEngine(st, embedClient)
		profileGen := profile.NewGenerator(st)

		// Determine chat model
		modelName, err := resolveOllamaChatModel(ctx, embedClient.BaseURL(), flagChatModel)
		if err != nil {
			return fmt.Errorf("ollama model error: %w\nHint: Install a model with 'ollama pull llama3.2' or 'ollama pull qwen2.5-coder'", err)
		}

		if len(args) > 0 {
			// Single query mode
			prompt := strings.Join(args, " ")
			return executeAIChat(ctx, st, searchEngine, profileGen, embedClient.BaseURL(), modelName, prompt)
		}

		// Interactive REPL mode
		fmt.Printf("🧠 mini-me Interactive Chat (Model: %s)\n", modelName)
		fmt.Println("Type your prompt and press Enter. Type 'exit' or 'quit' to stop.")
		fmt.Println(strings.Repeat("-", 60))

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("\nmini-me> ")
			if !scanner.Scan() {
				break
			}
			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				continue
			}
			if input == "exit" || input == "quit" {
				fmt.Println("Goodbye!")
				break
			}

			fmt.Println()
			if err := executeAIChat(ctx, st, searchEngine, profileGen, embedClient.BaseURL(), modelName, input); err != nil {
				fmt.Printf("\n[Error: %v]\n", err)
			}
			fmt.Println()
		}

		return nil
	},
}

type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type OllamaChatChunk struct {
	Model   string            `json:"model"`
	Message OllamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

func resolveOllamaChatModel(ctx context.Context, baseURL, userModel string) (string, error) {
	if userModel != "" {
		return userModel, nil
	}
	if envModel := os.Getenv("OLLAMA_CHAT_MODEL"); envModel != "" {
		return envModel, nil
	}

	// Fetch tags from Ollama
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot connect to Ollama at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	var tags struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}

	if len(tags.Models) == 0 {
		return "", fmt.Errorf("no models found in Ollama")
	}

	// Filter out embedding-only models
	var chatModels []string
	for _, m := range tags.Models {
		name := strings.ToLower(m.Name)
		if strings.Contains(name, "embed") || strings.Contains(name, "bge-") || strings.Contains(name, "minilm") {
			continue
		}
		chatModels = append(chatModels, m.Name)
	}

	if len(chatModels) > 0 {
		return chatModels[0], nil
	}

	// Fallback to first available model if none filtered
	return tags.Models[0].Name, nil
}

func executeAIChat(ctx context.Context, st *store.Store, engine *search.SearchEngine, pgen *profile.Generator, baseURL, model, prompt string) error {
	var systemPrompt strings.Builder
	systemPrompt.WriteString("You are mini-me, a personal digital twin AI assistant running locally on the user's machine.\n")
	systemPrompt.WriteString("Answer the user's question accurately using their local profile and retrieved context.\n\n")

	// 1. Fetch Profile Card
	profileCard, err := pgen.GenerateProfileCard(ctx)
	if err == nil && profileCard != "" {
		systemPrompt.WriteString("=== USER PROFILE CARD ===\n")
		systemPrompt.WriteString(profileCard)
		systemPrompt.WriteString("\n\n")
	}

	// 2. Perform RAG Search unless disabled
	if !flagChatNoRAG {
		hits, err := engine.Search(ctx, prompt, search.SearchOptions{K: flagChatMaxHits})
		if err == nil && len(hits) > 0 {
			systemPrompt.WriteString("=== RETRIEVED LOCAL KNOWLEDGE CONTEXT ===\n")
			for i, hit := range hits {
				projStr := ""
				if hit.Chunk.Project != "" {
					projStr = fmt.Sprintf(" [%s]", hit.Chunk.Project)
				}
				systemPrompt.WriteString(fmt.Sprintf("[%d]%s %s:\n%s\n\n", i+1, projStr, hit.Chunk.Path, hit.Chunk.Text))
			}
		}
	}

	reqPayload := OllamaChatRequest{
		Model: model,
		Messages: []OllamaChatMessage{
			{Role: "system", Content: systemPrompt.String()},
			{Role: "user", Content: prompt},
		},
		Stream: true,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed sending request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("Ollama chat error (status %d): %s", resp.StatusCode, buf.String())
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var chunk OllamaChatChunk
			if err := json.Unmarshal(line, &chunk); err == nil {
				if chunk.Message.Content != "" {
					fmt.Print(chunk.Message.Content)
				}
				if chunk.Done {
					break
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	fmt.Println()

	return nil
}

func init() {
	aiCmd.Flags().StringVarP(&flagChatModel, "model", "m", "", "Ollama model name to use for chat (e.g. llama3.2, qwen2.5-coder)")
	aiCmd.Flags().BoolVar(&flagChatNoRAG, "no-rag", false, "disable local RAG context retrieval")
	aiCmd.Flags().IntVarP(&flagChatMaxHits, "k", "k", 5, "number of RAG context chunks to inject")

	RootCmd.AddCommand(aiCmd)
}
