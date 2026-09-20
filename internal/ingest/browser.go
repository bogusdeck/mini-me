package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"

	"mini-me/internal/logger"
	"mini-me/internal/store"
)

type BrowserHistoryItem struct {
	URL        string
	Title      string
	VisitCount int
	LastVisit  time.Time
	Browser    string
}

type BrowserScanner struct {
	store       *store.Store
	embedClient Embedder
	chunker     *MarkdownChunker
}

func NewBrowserScanner(st *store.Store, embedClient Embedder) *BrowserScanner {
	return &BrowserScanner{
		store:       st,
		embedClient: embedClient,
		chunker:     NewMarkdownChunker(),
	}
}

func (bs *BrowserScanner) ScanChromeHistory(ctx context.Context) ([]*BrowserHistoryItem, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	chromePath := filepath.Join(userHome, "Library", "Application Support", "Google", "Chrome", "Default", "History")
	if _, err := os.Stat(chromePath); os.IsNotExist(err) {
		return nil, nil
	}

	return bs.readChromeHistoryFile(ctx, chromePath)
}

func (bs *BrowserScanner) ScanSafariHistory(ctx context.Context) ([]*BrowserHistoryItem, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	safariPath := filepath.Join(userHome, "Library", "Safari", "History.db")
	if _, err := os.Stat(safariPath); os.IsNotExist(err) {
		return nil, nil
	}

	return bs.readSafariHistoryFile(ctx, safariPath)
}

func (bs *BrowserScanner) readChromeHistoryFile(ctx context.Context, dbPath string) ([]*BrowserHistoryItem, error) {
	tmpCopyPath := filepath.Join(os.TempDir(), fmt.Sprintf("chrome_hist_%d.db", time.Now().UnixNano()))
	if err := copyFile(dbPath, tmpCopyPath); err != nil {
		return nil, fmt.Errorf("failed creating chrome history copy: %w", err)
	}
	defer os.Remove(tmpCopyPath)

	db, err := sql.Open("sqlite3", "file:"+tmpCopyPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Chrome timestamp is microseconds since 1601-01-01 00:00:00 UTC
	query := `
		SELECT url, title, visit_count, last_visit_time
		FROM urls
		WHERE title != '' AND url NOT LIKE 'chrome://%' AND url NOT LIKE 'about:%'
		ORDER BY last_visit_time DESC
		LIMIT 200
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*BrowserHistoryItem
	chromeEpoch := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)

	for rows.Next() {
		var urlStr, title string
		var visitCount int
		var lastVisitMicro int64
		if err := rows.Scan(&urlStr, &title, &visitCount, &lastVisitMicro); err == nil {
			visitTime := chromeEpoch.Add(time.Duration(lastVisitMicro) * time.Microsecond)
			items = append(items, &BrowserHistoryItem{
				URL:        urlStr,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  visitTime,
				Browser:    "Chrome",
			})
		}
	}
	return items, rows.Err()
}

func (bs *BrowserScanner) readSafariHistoryFile(ctx context.Context, dbPath string) ([]*BrowserHistoryItem, error) {
	tmpCopyPath := filepath.Join(os.TempDir(), fmt.Sprintf("safari_hist_%d.db", time.Now().UnixNano()))
	if err := copyFile(dbPath, tmpCopyPath); err != nil {
		return nil, fmt.Errorf("failed creating safari history copy: %w", err)
	}
	defer os.Remove(tmpCopyPath)

	db, err := sql.Open("sqlite3", "file:"+tmpCopyPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Safari timestamp is seconds since 2001-01-01 00:00:00 UTC
	query := `
		SELECT i.url, v.title, i.visit_count, v.visit_time
		FROM history_items i
		JOIN history_visits v ON i.id = v.history_item
		WHERE v.title IS NOT NULL AND v.title != ''
		ORDER BY v.visit_time DESC
		LIMIT 200
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*BrowserHistoryItem
	safariEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

	for rows.Next() {
		var urlStr, title string
		var visitCount int
		var visitSec float64
		if err := rows.Scan(&urlStr, &title, &visitCount, &visitSec); err == nil {
			visitTime := safariEpoch.Add(time.Duration(visitSec * float64(time.Second)))
			items = append(items, &BrowserHistoryItem{
				URL:        urlStr,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  visitTime,
				Browser:    "Safari",
			})
		}
	}
	return items, rows.Err()
}

func (bs *BrowserScanner) IngestBrowserHistory(ctx context.Context) (int, error) {
	var allItems []*BrowserHistoryItem

	if chromeItems, err := bs.ScanChromeHistory(ctx); err == nil {
		allItems = append(allItems, chromeItems...)
	}
	if safariItems, err := bs.ScanSafariHistory(ctx); err == nil {
		allItems = append(allItems, safariItems...)
	}

	if len(allItems) == 0 {
		logger.Info("no browser history items found to ingest")
		return 0, nil
	}

	// Group into Markdown summary blocks of 10 items
	var chunksCount int
	for i := 0; i < len(allItems); i += 10 {
		end := i + 10
		if end > len(allItems) {
			end = len(allItems)
		}

		batch := allItems[i:end]
		var sb strings.Builder
		sb.WriteString("Breadcrumb: # Browser History > ## Recent Browsing Activity\n\n")

		for _, item := range batch {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n  URL: %s\n  Visited: %s (Visits: %d)\n\n",
				item.Title, item.Browser, item.URL, item.LastVisit.Format("2006-01-02 15:04"), item.VisitCount))
		}

		fullText := sb.String()
		chunkResults, err := bs.chunker.ChunkMarkdown(fullText)
		if err != nil || len(chunkResults) == 0 {
			continue
		}

		for _, res := range chunkResults {
			existing, _ := bs.store.GetChunksByPath(ctx, "browser_history")
			alreadyExists := false
			for _, ec := range existing {
				if ec.Hash == res.Hash {
					alreadyExists = true
					break
				}
			}

			if alreadyExists {
				continue
			}

			vecs, err := bs.embedClient.EmbedDocumentChunks(ctx, []string{res.Text})
			if err != nil || len(vecs) == 0 {
				continue
			}

			chunkRow := &store.Chunk{
				SourceType:     "browser_history",
				Path:           "browser_history",
				Project:        "browser",
				Text:           res.Text,
				Hash:           res.Hash,
				TS:             time.Now().Unix(),
				EmbedModel:     bs.embedClient.ModelName() + ":256",
				ChunkerVersion: "v1",
			}

			if _, err := bs.store.InsertChunk(ctx, chunkRow, vecs[0]); err == nil {
				chunksCount++
			}
		}
	}

	return chunksCount, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
