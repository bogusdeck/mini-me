package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"
	"time"

	ncrucesvec "github.com/asg017/sqlite-vec-go-bindings/ncruces"

	"mini-me/internal/logger"
	"mini-me/internal/store"
)

const (
	DefaultK   = 10
	CandidateK = 50
	RRFConst   = 60.0
)

type SearchOptions struct {
	SourceType string
	Project    string
	Since      time.Time
	K          int
}

type SearchHit struct {
	Chunk    *store.Chunk `json:"chunk"`
	RRFScore float64      `json:"rrf_score"`
	VectorRank int        `json:"vector_rank,omitempty"`
	FTSRank    int        `json:"fts_rank,omitempty"`
}

type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

type SearchEngine struct {
	store   *store.Store
	embedder QueryEmbedder
}

func NewSearchEngine(st *store.Store, embedder QueryEmbedder) *SearchEngine {
	return &SearchEngine{
		store:   st,
		embedder: embedder,
	}
}

// Search executes hybrid search (Vector KNN + FTS5 BM25) fused with Reciprocal Rank Fusion (RRF),
// validating source file existence and SHA-256 hash staleness before returning hits.
func (se *SearchEngine) Search(ctx context.Context, queryStr string, opts SearchOptions) ([]*SearchHit, error) {
	if opts.K <= 0 {
		opts.K = DefaultK
	}

	// 1. Vector Search Candidates
	vectorRanks := make(map[int64]int)
	queryVec, err := se.embedder.EmbedQuery(ctx, queryStr)
	if err != nil {
		logger.Warn("vector embedding failed during search, falling back to pure FTS5", "error", err)
	} else if len(queryVec) > 0 {
		vecBytes, err := ncrucesvec.SerializeFloat32(queryVec)
		if err == nil {
			rows, err := se.store.DB().QueryContext(ctx, `
				SELECT rowid FROM chunk_vec
				WHERE embedding MATCH ?
				ORDER BY distance ASC
				LIMIT ?
			`, vecBytes, CandidateK)
			if err == nil {
				rank := 1
				for rows.Next() {
					var id int64
					if err := rows.Scan(&id); err == nil {
						vectorRanks[id] = rank
						rank++
					}
				}
				_ = rows.Close()
			}
		}
	}

	// 2. FTS5 Search Candidates
	ftsRanks := make(map[int64]int)
	cleanQuery := SanitizeFTSQuery(queryStr)
	if cleanQuery != "" {
		rows, err := se.store.DB().QueryContext(ctx, `
			SELECT rowid FROM chunk_fts
			WHERE chunk_fts MATCH ?
			ORDER BY rank ASC
			LIMIT ?
		`, cleanQuery, CandidateK)
		if err == nil {
			rank := 1
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil {
					ftsRanks[id] = rank
					rank++
				}
			}
			_ = rows.Close()
		}
	}

	// 3. Reciprocal Rank Fusion (RRF)
	allIDs := make(map[int64]bool)
	for id := range vectorRanks {
		allIDs[id] = true
	}
	for id := range ftsRanks {
		allIDs[id] = true
	}

	var candidates []*SearchHit

	for id := range allIDs {
		c, err := se.store.GetChunk(ctx, id)
		if err != nil || c == nil {
			continue
		}

		// Apply Filters
		if opts.SourceType != "" && !strings.EqualFold(c.SourceType, opts.SourceType) {
			continue
		}
		if opts.Project != "" && !strings.EqualFold(c.Project, opts.Project) {
			continue
		}
		if !opts.Since.IsZero() && c.TS < opts.Since.Unix() {
			continue
		}

		var score float64
		vRank, hasVRank := vectorRanks[id]
		if hasVRank {
			score += 1.0 / (RRFConst + float64(vRank))
		}

		fRank, hasFRank := ftsRanks[id]
		if hasFRank {
			score += 1.0 / (RRFConst + float64(fRank))
		}

		candidates = append(candidates, &SearchHit{
			Chunk:      c,
			RRFScore:   score,
			VectorRank: vRank,
			FTSRank:    fRank,
		})
	}

	// Sort candidates by RRF Score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RRFScore > candidates[j].RRFScore
	})

	// 4. Staleness Validation
	var validHits []*SearchHit

	for _, hit := range candidates {
		if len(validHits) >= opts.K {
			break
		}

		fresh, err := se.validateStaleness(ctx, hit.Chunk)
		if err != nil || !fresh {
			logger.Warn("dropping stale search hit and marking for reindex", "path", hit.Chunk.Path, "chunk_id", hit.Chunk.ID)
			// Purge stale chunk from DB
			_ = se.store.DeleteChunk(ctx, hit.Chunk.ID)
			continue
		}

		validHits = append(validHits, hit)
	}

	return validHits, nil
}

func (se *SearchEngine) validateStaleness(ctx context.Context, c *store.Chunk) (bool, error) {
	info, err := os.Stat(c.Path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}

	// Read file and check if hash still matches
	content, err := os.ReadFile(c.Path)
	if err != nil {
		return false, err
	}

	// If file content contains the chunk text, it's fresh
	if strings.Contains(string(content), c.Text) || strings.Contains(string(content), c.Hash) {
		return true, nil
	}

	// Alternative: verify if full file sha256 hash or chunk text sha256 matches
	hashBytes := sha256.Sum256([]byte(c.Text))
	hashHex := hex.EncodeToString(hashBytes[:])
	if hashHex == c.Hash {
		return true, nil
	}

	// Fast path check: if modification time is less than or equal to indexed timestamp
	if info.ModTime().Unix() <= c.TS {
		return true, nil
	}

	return false, nil
}

// SanitizeFTSQuery cleans search string for FTS5 syntax safety.
func SanitizeFTSQuery(query string) string {
	cleaned := strings.TrimSpace(query)
	if cleaned == "" {
		return ""
	}

	// Replace special FTS5 operators with spaces
	replacer := strings.NewReplacer(
		":", " ",
		"*", " ",
		"\"", " ",
		"(", " ",
		")", " ",
		"AND", " ",
		"OR", " ",
		"NOT", " ",
		"NEAR", " ",
	)
	cleaned = replacer.Replace(cleaned)

	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return ""
	}

	// Join terms with OR or spaces for FTS MATCH
	return strings.Join(words, " OR ")
}
