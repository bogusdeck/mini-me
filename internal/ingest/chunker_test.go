package ingest_test

import (
	"strings"
	"testing"

	"mini-me/internal/ingest"
)

func TestParseFrontmatter(t *testing.T) {
	content := `---
title: My Project
project: mini-me
---
# Heading
Body text goes here.`

	meta, body := ingest.ParseFrontmatter(content)
	if meta["title"] != "My Project" {
		t.Errorf("expected title 'My Project', got %q", meta["title"])
	}
	if meta["project"] != "mini-me" {
		t.Errorf("expected project 'mini-me', got %q", meta["project"])
	}
	if !strings.Contains(body, "# Heading") {
		t.Errorf("expected body to contain heading, got %q", body)
	}
}

func TestChunkMarkdownHeadingBreadcrumbs(t *testing.T) {
	chunker := ingest.NewMarkdownChunker()

	markdown := `# Mini-Me Architecture

## Core Database
SQLite database WAL mode storing vectors and chunks.

## Ingestion Pipeline
Heading aware markdown chunking algorithm.`

	results, err := chunker.ChunkMarkdown(markdown)
	if err != nil {
		t.Fatalf("unexpected chunking error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(results))
	}

	if !strings.Contains(results[0].Text, "Breadcrumb: # Mini-Me Architecture > ## Core Database") {
		t.Errorf("expected breadcrumb for chunk 0, got:\n%s", results[0].Text)
	}
	if !strings.Contains(results[1].Text, "Breadcrumb: # Mini-Me Architecture > ## Ingestion Pipeline") {
		t.Errorf("expected breadcrumb for chunk 1, got:\n%s", results[1].Text)
	}
	if results[0].Hash == "" || results[1].Hash == "" {
		t.Errorf("expected SHA-256 hashes for all chunks")
	}
}
