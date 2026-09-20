package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mini-me/internal/logger"
	"mini-me/internal/profile"
	"mini-me/internal/store"
)

var (
	resumeKeywords   = regexp.MustCompile(`(?i)(resume|cv|curriculum[_\s]vitae)`)
	identityKeywords = regexp.MustCompile(`(?i)(pan[_\s]?card|aadhaar|aaddhar|passport|signature|photograph|voter[_\s]?id|national[_\s]?id|tax[_\s]?id)`)
)

type DocumentScanStats struct {
	FilesScanned     int
	ResumesFound     int
	ProjectsFound    int
	IdentityDocsFound int
	ChunksCreated    int
}

type DeepScanner struct {
	store       *store.Store
	embedClient Embedder
	chunker     *MarkdownChunker
}

func NewDeepScanner(st *store.Store, embedClient Embedder) *DeepScanner {
	return &DeepScanner{
		store:       st,
		embedClient: embedClient,
		chunker:     NewMarkdownChunker(),
	}
}

// ScanUserDirectories scans default personal folders (~/Documents, ~/Projects, ~/Desktop, ~/Downloads).
func (ds *DeepScanner) ScanUserDirectories(ctx context.Context, targetPaths ...string) (*DocumentScanStats, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed resolving home directory: %w", err)
	}

	if len(targetPaths) == 0 {
		targetPaths = []string{
			filepath.Join(userHome, "Documents"),
			filepath.Join(userHome, "Projects"),
			filepath.Join(userHome, "Desktop"),
			filepath.Join(userHome, "Downloads"),
		}
	}

	stats := &DocumentScanStats{}
	filter := NewFilter()

	for _, dir := range targetPaths {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		logger.Info("deep scanning directory", "path", dir)

		err := filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			// 1. Detect Project Git Repos
			if info.Name() == ".git" {
				parentDir := filepath.Dir(p)
				ds.processProjectRepo(ctx, parentDir, stats)
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if filter.ShouldSkipPath(p, info) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				stats.FilesScanned++
				fileName := info.Name()

				// 2. Detect Resume Files
				if resumeKeywords.MatchString(fileName) {
					ds.processResumeFile(ctx, p, stats)
				}

				// 3. Detect Identity Documents (PAN, Aadhaar, Passport, Photo, Signature)
				if identityKeywords.MatchString(fileName) {
					ds.processIdentityDocFile(ctx, p, stats)
				}
			}

			return nil
		})

		if err != nil {
			logger.Error("error scanning folder", "path", dir, "error", err)
		}
	}

	return stats, nil
}

func (ds *DeepScanner) processResumeFile(ctx context.Context, path string, stats *DocumentScanStats) {
	stats.ResumesFound++
	logger.Info("flagged resume document", "path", path)

	// Save identity field
	_ = ds.store.SetField(ctx, "latest_resume_path", path)

	// Read content if text/markdown
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text := string(content)

	// Ingest into knowledge base
	ingester := NewIngester(ds.store, ds.embedClient)
	subStats, err := ingester.IngestPath(ctx, path, "resume")
	if err == nil && subStats != nil {
		stats.ChunksCreated += subStats.ChunksCreated
	}

	// Create Fact
	ent, _ := ds.store.GetEntityByName(ctx, "Resume")
	var entID int64
	if ent == nil {
		entID, _ = ds.store.CreateEntity(ctx, &store.Entity{
			Type:  store.EntityTopic,
			Name:  "Resume",
			Notes: "Latest User CV / Resume",
		})
	} else {
		entID = ent.ID
	}

	_, _ = ds.store.CreateFact(ctx, &store.Fact{
		SubjectID:   entID,
		Predicate:   "located_at",
		Object:      path,
		Status:      store.FactStatusConfirmed,
		Sensitivity: store.SensitivityNormal,
	})

	// Detect if text contains skills or experience summary
	if len(text) > 0 {
		sens := profile.DetectSensitivity(text)
		_, _ = ds.store.CreateFact(ctx, &store.Fact{
			SubjectID:   entID,
			Predicate:   "summary",
			Object:      truncateText(text, 200),
			Status:      store.FactStatusConfirmed,
			Sensitivity: sens,
		})
	}
}

func (ds *DeepScanner) processIdentityDocFile(ctx context.Context, path string, stats *DocumentScanStats) {
	stats.IdentityDocsFound++
	base := filepath.Base(path)
	logger.Info("flagged personal identity document", "path", path)

	docType := "Personal Identity Document"
	lower := strings.ToLower(base)
	if strings.Contains(lower, "pan") {
		docType = "PAN Card"
		_ = ds.store.SetField(ctx, "pan_card_path", path)
	} else if strings.Contains(lower, "aadhaar") || strings.Contains(lower, "aaddhar") {
		docType = "Aadhaar Card"
		_ = ds.store.SetField(ctx, "aadhaar_card_path", path)
	} else if strings.Contains(lower, "passport") {
		docType = "Passport"
		_ = ds.store.SetField(ctx, "passport_path", path)
	} else if strings.Contains(lower, "signature") {
		docType = "Signature Image"
		_ = ds.store.SetField(ctx, "signature_path", path)
	} else if strings.Contains(lower, "photo") || strings.Contains(lower, "photograph") {
		docType = "Photograph"
		_ = ds.store.SetField(ctx, "photo_path", path)
	}

	ent, _ := ds.store.GetEntityByName(ctx, docType)
	var entID int64
	if ent == nil {
		entID, _ = ds.store.CreateEntity(ctx, &store.Entity{
			Type:  store.EntityTopic,
			Name:  docType,
			Notes: fmt.Sprintf("Identity Document: %s", base),
		})
	} else {
		entID = ent.ID
	}

	// Create location fact with SensitivityPersonal
	_, _ = ds.store.CreateFact(ctx, &store.Fact{
		SubjectID:   entID,
		Predicate:   "located_at",
		Object:      path,
		Status:      store.FactStatusConfirmed,
		Sensitivity: store.SensitivityPersonal,
	})

	// If file is text readable, extract contents
	content, err := os.ReadFile(path)
	if err == nil {
		text := string(content)
		sens := profile.DetectSensitivity(text)
		if sens == store.SensitivityNormal {
			sens = store.SensitivityPersonal
		}

		// Index into chunks
		chunkRow := &store.Chunk{
			SourceType:     "identity_doc",
			Path:           path,
			Project:        "identity",
			Text:           fmt.Sprintf("Breadcrumb: # Identity Document > ## %s\n\nFile Location: %s\nContent: %s", docType, path, text),
			Hash:           fmt.Sprintf("hash_%s_%d", base, len(text)),
			TS:             time.Now().Unix(),
			EmbedModel:     ds.embedClient.ModelName() + ":256",
			ChunkerVersion: "v1",
		}
		vecs, err := ds.embedClient.EmbedDocumentChunks(ctx, []string{chunkRow.Text})
		if err == nil && len(vecs) > 0 {
			_, _ = ds.store.InsertChunk(ctx, chunkRow, vecs[0])
			stats.ChunksCreated++
		}
	}
}

func (ds *DeepScanner) processProjectRepo(ctx context.Context, repoPath string, stats *DocumentScanStats) {
	stats.ProjectsFound++
	projName := filepath.Base(repoPath)
	logger.Info("discovered project repository", "name", projName, "path", repoPath)

	ent, _ := ds.store.GetEntityByName(ctx, projName)
	var entID int64
	if ent == nil {
		entID, _ = ds.store.CreateEntity(ctx, &store.Entity{
			Type:  store.EntityProject,
			Name:  projName,
			Notes: fmt.Sprintf("Local Git Repository at %s", repoPath),
		})
	} else {
		entID = ent.ID
	}

	_, _ = ds.store.CreateFact(ctx, &store.Fact{
		SubjectID:   entID,
		Predicate:   "repo_location",
		Object:      repoPath,
		Status:      store.FactStatusConfirmed,
		Sensitivity: store.SensitivityNormal,
	})

	// Read README.md if present
	readmePath := filepath.Join(repoPath, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		ingester := NewIngester(ds.store, ds.embedClient)
		ingStats := &IngestStats{}
		if err := ingester.IngestFile(ctx, readmePath, projName, ingStats); err == nil {
			stats.ChunksCreated += ingStats.ChunksCreated
		}
	}
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
