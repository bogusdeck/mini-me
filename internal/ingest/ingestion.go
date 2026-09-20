package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mini-me/internal/logger"
	"mini-me/internal/store"
)

type Embedder interface {
	EmbedDocumentChunks(ctx context.Context, texts []string) ([][]float32, error)
	ModelName() string
}

type IngestStats struct {
	FilesProcessed int
	ChunksCreated   int
	ChunksDeleted   int
	ChunksUnchanged int
}

type Ingester struct {
	store       *store.Store
	embedClient Embedder
	chunker     *MarkdownChunker
	scanner     *SecretScanner
}

func NewIngester(st *store.Store, embedClient Embedder) *Ingester {
	return &Ingester{
		store:       st,
		embedClient: embedClient,
		chunker:     NewMarkdownChunker(),
		scanner:     NewSecretScanner(),
	}
}

// IngestPath ingests a file or directory path, diffing chunks per file in single transactions.
func (ing *Ingester) IngestPath(ctx context.Context, targetPath string, project string) (*IngestStats, error) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path %s: %w", targetPath, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat target path %s: %w", absPath, err)
	}

	stats := &IngestStats{}
	filter := NewFilter()

	if !info.IsDir() {
		if filter.ShouldSkipPath(absPath, info) {
			logger.Info("skipping file based on denylist/filter", "path", absPath)
			return stats, nil
		}
		if err := ing.IngestFile(ctx, absPath, project, stats); err != nil {
			return nil, fmt.Errorf("failed ingesting file %s: %w", absPath, err)
		}
		return stats, nil
	}

	// Walk Directory
	err = filepath.Walk(absPath, func(p string, fInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Load .gitignore if present
		if fInfo.Name() == ".gitignore" {
			_ = filter.LoadGitignore(p)
		}

		if filter.ShouldSkipPath(p, fInfo) {
			if fInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !fInfo.IsDir() {
			if err := ing.IngestFile(ctx, p, project, stats); err != nil {
				logger.Error("failed ingesting file during walk", "path", p, "error", err)
			}
		}
		return nil
	})

	return stats, err
}

// IngestFile processes a single file, diffing chunks against the database.
func (ing *Ingester) IngestFile(ctx context.Context, filePath string, project string, stats *IngestStats) error {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed reading file: %w", err)
	}
	rawContent := string(contentBytes)

	// Secret Scanner: Redact any detected secrets BEFORE chunking & embedding
	if ing.scanner.Scan(rawContent) {
		logger.Warn("detected potential secrets in file; redacting secrets before processing", "path", filePath)
		rawContent = ing.scanner.Redact(rawContent)
	}

	chunkResults, err := ing.chunker.ChunkMarkdown(rawContent)
	if err != nil {
		return fmt.Errorf("failed chunking file: %w", err)
	}

	existingChunks, err := ing.store.GetChunksByPath(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed querying existing chunks for path: %w", err)
	}

	existingMap := make(map[string]*store.Chunk)
	for _, ec := range existingChunks {
		existingMap[ec.Hash] = ec
	}

	newMap := make(map[string]*ChunkResult)
	for _, nc := range chunkResults {
		newMap[nc.Hash] = nc
	}

	// Identify chunks to delete
	var toDelete []int64
	for hash, ec := range existingMap {
		if _, exists := newMap[hash]; !exists {
			toDelete = append(toDelete, ec.ID)
		}
	}

	// Identify chunks to insert
	var toInsert []*ChunkResult
	for hash, nc := range newMap {
		if _, exists := existingMap[hash]; !exists {
			toInsert = append(toInsert, nc)
		} else {
			stats.ChunksUnchanged++
		}
	}

	// If no changes needed, finish early
	if len(toDelete) == 0 && len(toInsert) == 0 {
		stats.FilesProcessed++
		return nil
	}

	// Delete missing chunks
	for _, id := range toDelete {
		if err := ing.store.DeleteChunk(ctx, id); err != nil {
			return fmt.Errorf("failed deleting obsolete chunk %d: %w", id, err)
		}
		stats.ChunksDeleted++
	}

	// Embed & Insert new chunks
	if len(toInsert) > 0 {
		textsToEmbed := make([]string, len(toInsert))
		for i, res := range toInsert {
			textsToEmbed[i] = res.Text
		}

		embeddings, err := ing.embedClient.EmbedDocumentChunks(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed generating embeddings for file %s: %w", filePath, err)
		}

		embedModel := ing.embedClient.ModelName() + ":256"
		info, _ := os.Stat(filePath)
		modTime := info.ModTime().Unix()

		for i, res := range toInsert {
			proj := project
			if metaProj, ok := res.Metadata["project"]; ok && metaProj != "" {
				proj = metaProj
			}

			chunkRow := &store.Chunk{
				SourceType:     "markdown",
				Path:           filePath,
				Project:        proj,
				Text:           res.Text,
				Hash:           res.Hash,
				TS:             modTime,
				EmbedModel:     embedModel,
				ChunkerVersion: "v1",
			}

			if _, err := ing.store.InsertChunk(ctx, chunkRow, embeddings[i]); err != nil {
				return fmt.Errorf("failed inserting new chunk into store: %w", err)
			}
			stats.ChunksCreated++
		}
	}

	stats.FilesProcessed++
	return nil
}

// Reindex checks all stored files, updating modified files and purging deleted files.
func (ing *Ingester) Reindex(ctx context.Context) (*IngestStats, error) {
	rows, err := ing.store.DB().QueryContext(ctx, "SELECT DISTINCT path, project FROM chunk")
	if err != nil {
		return nil, fmt.Errorf("failed querying indexed paths: %w", err)
	}

	type pathItem struct {
		path    string
		project string
	}
	var items []pathItem

	for rows.Next() {
		var item pathItem
		if err := rows.Scan(&item.path, &item.project); err == nil {
			items = append(items, item)
		}
	}
	_ = rows.Close()

	stats := &IngestStats{}
	filter := NewFilter()

	for _, item := range items {
		info, err := os.Stat(item.path)
		if os.IsNotExist(err) || (err == nil && filter.ShouldSkipPath(item.path, info)) {
			// File was deleted or is now filtered out -> purge chunks
			if err := ing.store.DeleteChunksByPath(ctx, item.path); err != nil {
				logger.Error("failed purging deleted file chunks", "path", item.path, "error", err)
			} else {
				logger.Info("purged chunks for deleted/filtered file", "path", item.path)
			}
			continue
		}

		if err == nil {
			if err := ing.IngestFile(ctx, item.path, item.project, stats); err != nil {
				logger.Error("failed reindexing file", "path", item.path, "error", err)
			}
		}
	}

	return stats, nil
}
