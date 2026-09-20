# Memex Architecture & Design Document

`memex` is a local-first, single-user personal knowledge base for macOS (and Linux) stored in a single SQLite database file. It provides local CLI tools, HTTP APIs, and an Model Context Protocol (MCP) server to allow personal tools and projects (like NeuralForm) to query structured profile data, projects, entities, facts, and searchable notes with zero telemetry or cloud dependencies.

---

## 1. System Architecture

```
+-------------------------------------------------------------------------+
|                              CLI Commands                               |
|   (add, reindex, search, init, fact, profile, doctor, serve, mcp, etc.) |
+------------------+-----------------------------------+------------------+
                   |                                   |
                   v                                   v
+--------------------------------------+   +------------------------------+
|         Internal Packages            |   |        Ollama Client         |
|  - config / paths                    |   |  - http://127.0.0.1:11434    |
|  - store (SQLite + vec0 + FTS5)      |   |  - nomic-embed-text          |
|  - ingest (chunker, secrets, diff)   |   |  - 768d -> 256d + L2 Norm    |
|  - search (KNN + FTS5 + RRF)         |   +------------------------------+
|  - profile / facts                   |
|  - server / mcp                      |
+------------------+-------------------+
                   |
                   v
+-------------------------------------------------------------------------+
|                       SQLite Database (memex.db)                        |
|  - File: os.UserConfigDir()/memex/memex.db                              |
|  - Mode: WAL mode, permissions 0600                                     |
|  - Driver: github.com/ncruces/go-sqlite3 (No CGO)                        |
|  - Vector Search: vec0 (sqlite-vec)                                     |
|  - Keyword Search: FTS5                                                 |
+-------------------------------------------------------------------------+
```

### Key Principles & Constraints
1. **Local-First & Offline**: All state lives in `$HOME/.config/memex/memex.db` (or OS equivalent). No remote HTTP calls except to local Ollama (`http://127.0.0.1:11434`).
2. **CGO-Free SQLite**: Powered by `github.com/ncruces/go-sqlite3` compiled to WebAssembly with `sqlite-vec` extension (`github.com/asg017/sqlite-vec-go-bindings/ncruces`).
3. **Single File Database**: File permissions set strictly to `0600` (directory `0700`).

---

## 2. Database Schema & Migrations

Database versioning is managed via a `schema_version` tracking table. Migrations run idempotently on database initialization.

### Schema Definitions

```sql
-- Migration Tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Text Chunks
CREATE TABLE IF NOT EXISTS chunk (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,       -- e.g. "markdown", "note"
    path TEXT NOT NULL,              -- absolute file path
    project TEXT NOT NULL DEFAULT '',-- optional project moniker
    text TEXT NOT NULL,              -- chunk content (with heading breadcrumb prepended)
    hash TEXT NOT NULL,              -- SHA-256 of raw chunk content
    ts INTEGER NOT NULL,             -- unix timestamp of modification
    embed_model TEXT NOT NULL,       -- e.g. "nomic-embed-text:256"
    chunker_version TEXT NOT NULL    -- e.g. "v1"
);

CREATE INDEX IF NOT EXISTS idx_chunk_path ON chunk(path);
CREATE INDEX IF NOT EXISTS idx_chunk_project ON chunk(project);
CREATE INDEX IF NOT EXISTS idx_chunk_hash ON chunk(hash);

-- sqlite-vec Vector Table (256 dimensions)
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vec USING vec0(
    embedding float[256]
);

-- FTS5 Full-Text Search Table
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_fts USING fts5(
    text,
    content='chunk',
    content_rowid='id'
);

-- Entities (People, Organizations, Projects, Topics, Places)
CREATE TABLE IF NOT EXISTS entity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('person', 'org', 'project', 'topic', 'place')),
    name TEXT NOT NULL UNIQUE,
    aliases TEXT NOT NULL DEFAULT '', -- JSON array or comma-separated list
    notes TEXT NOT NULL DEFAULT ''
);

-- Facts Knowledge Graph
CREATE TABLE IF NOT EXISTS fact (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER NOT NULL REFERENCES entity(id) ON DELETE CASCADE,
    predicate TEXT NOT NULL,          -- e.g. "works_at", "likes", "uses_tech"
    object TEXT NOT NULL,             -- value or entity reference
    status TEXT NOT NULL CHECK (status IN ('proposed', 'confirmed', 'rejected')),
    confidence REAL NOT NULL DEFAULT 1.0,
    sensitivity TEXT NOT NULL CHECK (sensitivity IN ('normal', 'personal', 'never_infer')),
    valid_from TIMESTAMP,
    valid_to TIMESTAMP,
    superseded_by INTEGER REFERENCES fact(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fact_subject ON fact(subject_id);
CREATE INDEX IF NOT EXISTS idx_fact_status ON fact(status);

-- Fact Evidence Link
CREATE TABLE IF NOT EXISTS fact_source (
    fact_id INTEGER NOT NULL REFERENCES fact(id) ON DELETE CASCADE,
    chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE,
    PRIMARY KEY (fact_id, chunk_id)
);

-- Profile Deterministic Fields (Form Filling & Identity)
CREATE TABLE IF NOT EXISTS field (
    key TEXT PRIMARY KEY,             -- e.g. "full_name", "email", "phone", "linkedin_url"
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## 3. Data Flow & Core Subsystems

### Ingestion Pipeline (`memex add`, `memex reindex`)
1. **File Walk & Filtering**:
   - Check `.gitignore` rules.
   - Apply denylist: `.ssh`, `.aws`, `.env*`, `node_modules`, build directories (`dist`, `build`, `target`), binaries, and files > size cap (e.g. 2MB).
2. **Secret Scanning**:
   - High-entropy strings and common key regex patterns (AWS keys, SSH private keys, API tokens).
   - If secrets found: skip chunk or redact content before further processing.
3. **Markdown Chunking**:
   - Heading-aware splitter preserving breadcrumbs (e.g., `# Project > ## Architecture > content`).
   - Token window ~200-500 tokens with slight overlap.
   - Frontmatter parsing for metadata.
4. **Content-Addressed Diffing**:
   - Compute SHA-256 hash per chunk.
   - Compare with stored `chunk` rows for `path`.
   - Update transaction: insert new chunks, delete removed chunks, clean up unreferenced `chunk_vec` and `chunk_fts` entries.
5. **Embedding Generation**:
   - Batch request to Ollama (`http://127.0.0.1:11434/api/embed`) using `search_document: ` prefix.
   - Slice 768 dims -> first 256 dims -> L2-normalize.
   - Store float vector in `chunk_vec`.

### Search Pipeline (`memex search`)
1. **Query Processing**:
   - Prefix search query string with `search_query: `.
   - Compute query vector via Ollama embeddings (256d L2 normalized).
2. **Hybrid Search Execution**:
   - **Vector Search**: KNN query via `chunk_vec` table for top K candidate IDs.
   - **Keyword Search**: BM25 ranking via `chunk_fts` table for top K candidate IDs.
   - **Reciprocal Rank Fusion (RRF)**: Fuse rank positions:
     \( RRF(d) = \sum_{m \in M} \frac{1}{k + r_m(d)} \) (default \( k = 60 \)).
3. **Staleness Validation**:
   - For each top result chunk, check if source file exists on disk and matches SHA-256 hash.
   - If missing or modified: drop result from response and flag file path for background/subsequent reindexing.

### Fact & Profile Engine (`memex init`, `fact`, `profile`)
- **Interactive Interview (`memex init`)**: Populates `field` table and creates initial `entity` + `confirmed` facts.
- **Review Queue (`memex review`)**: Holds facts proposed by external tools (e.g. via MCP). Facts default to `status = 'proposed'`.
- **Profile Generation (`memex profile`)**: Synthesizes profile card markdown (<= ~1500 tokens) using deterministic field mappings + confirmed facts + active projects.
- **Sensitivity Rules**: `never_infer` facts can only be added manually by the user. Government IDs and payment card details are strictly prohibited from being persisted.

### Serving & Integrations (`memex serve`, `memex mcp`)
- **HTTP Server (`memex serve`)**:
  - Bound exclusively to `127.0.0.1` (localhost).
  - Authenticated via bearer token stored in `$HOME/.config/memex/auth_token` (mode `0600`).
  - Strict endpoint contracts: `/v1/profile`, `/v1/profile/field`, `/v1/entity`, `/v1/search`.
- **MCP Server (`memex mcp`)**:
  - Stdio transport for Model Context Protocol.
  - Read-only tools: `get_profile`, `get_person`, `get_project`, `search`.
  - Proposal tool: `propose_fact` writes only to the review queue as `status='proposed'`.
  - All output marked as `UNTRUSTED RETRIEVED DATA`.

---

## 4. Failure Modes & Mitigations

| Failure Mode | Impact | Mitigation Strategy |
|--------------|--------|---------------------|
| Ollama Unavailable / Model Not Pulled | Embeddings fail during ingestion or search | `memex doctor` detects status and prints pull instructions. Ingestion fails gracefully; search can fall back to pure FTS5 keyword search. |
| Database Corruption / Lock contention | Data loss or process block | SQLite WAL mode enabled, single-writer multi-reader concurrency, bounded timeouts. |
| Stale Search Index | Outdated search results pointing to deleted/edited files | Pre-search staleness check verifies source file presence and SHA-256 hash. Triggers reindex on mismatch. |
| Secret Leakage in Notes | Secrets sent to Ollama or stored in plain DB | Pre-embedding regex and entropy secret scanner skips or redacts sensitive keys prior to chunk creation. |
| Sensitive Personal Data Exposure | Unauthorized extraction of sensitive info | `never_infer` facts require manual creation. Hard constraint against storing government IDs or payment cards. |
