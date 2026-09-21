# mini-me

`mini-me` is a local-first, single-user personal knowledge base for macOS (and Linux) stored in a single SQLite file. It provides local CLI tools, an HTTP API, and a Model Context Protocol (MCP) server to allow personal tools and projects (like NeuralForm) to query structured profile data, active projects, people graphs, and searchable notes.

---

## Key Features

- **Local-First & CGO-Free**: Powered by `github.com/ncruces/go-sqlite3` compiled to WebAssembly with `sqlite-vec` extension (`github.com/asg017/sqlite-vec-go-bindings/ncruces`). No CGO, no Docker, no external database server.
- **Single Database File**: State is stored in a single SQLite database file at `os.UserConfigDir()/mini-me/mini-me.db` with WAL mode and `0600` file permissions.
- **Local Embeddings**: Integrates with local Ollama (`http://127.0.0.1:11434`, model `nomic-embed-text`). Slices 768-dimensional embeddings to 256 dimensions and applies L2 normalization.
- **Heading-Aware Markdown Chunker**: Automatically preserves Markdown heading breadcrumbs (`# Title > ## Section`) for context-rich retrieval.
- **Hybrid Search**: Combines `sqlite-vec` (KNN vector search) and `FTS5` (BM25 keyword search) fused via Reciprocal Rank Fusion (RRF). Validates file content freshness before returning hits.
- **Deterministic Profile & Knowledge Graph**: Identity fields, entities (people, orgs, projects, topics, places), facts graph with sensitivity rules (`normal`, `personal`, `never_infer`), and zero-LLM deterministic profile card generation.
- **Security Safeguards**: Embedded secret scanner (regex + Shannon entropy) redacting tokens/keys before embedding. Strict hard rule preventing storage of SSNs or credit card numbers.
- **Serving Interfaces**:
  - `mini-me serve`: Localhost-only HTTP REST API (`127.0.0.1`) authenticated via a `0600` permissions bearer token.
  - `mini-me mcp`: Stdio Model Context Protocol (MCP) server with untrusted data labeling (`[UNTRUSTED RETRIEVED DATA]`) and proposed-fact review queue support.

---

## Installation

### Prerequisites
- Go 1.22 or higher
- [Ollama](https://ollama.com) installed and running locally (`ollama pull nomic-embed-text`)

### Install from Source
```bash
git clone https://github.com/bogusdeck/mini-me.git
cd mini-me
go build -o mini-me ./cmd/mini-me
```

---

## Quickstart Guide

### 1. System Health Diagnostics
Verify SQLite accessibility, Ollama connectivity, and model availability:
```bash
./mini-me doctor
```

### 2. Profile Setup Interview
Run the onboarding interview to populate your identity fields:
```bash
./mini-me init --name "Alice Engineer" --email "alice@example.com" --employer "Acme Corp"
```

Generate and view your deterministic Profile Card anytime:
```bash
./mini-me profile
```

### 3. Add Notes & Ingest Markdown Files
Recursively chunk, secret-scan, embed, and index markdown notes or directories:
```bash
./mini-me add docs/ --project "my-project"
```

### 4. Hybrid Search
Search notes using hybrid vector KNN + FTS5 BM25 with RRF ranking:
```bash
./mini-me search "database schema" --project "my-project" -k 5
```

### 5. Knowledge Graph Facts & Review Queue
Add facts to your knowledge graph or review proposed facts:
```bash
# Add a confirmed fact
./mini-me fact add "Alice Engineer" "likes" "Go Architecture"

# List facts
./mini-me fact list

# Review proposed facts queue (e.g. from MCP tools)
./mini-me review
```

### 6. Serve REST API & Background Service
Start the local HTTP API manually:
```bash
./mini-me serve --port 8080
```

Or run `mini-me` as a background service (runs automatically on login):

**Option A: Built-in macOS Service Manager**
```bash
# Install and start background service
mini-me service install

# Check service status
mini-me service status

# Stop or uninstall service
mini-me service stop
mini-me service uninstall
```

**Option B: Homebrew Services**
```bash
# Start via Homebrew Services
brew services start mini-me

# Check Homebrew service status
brew services list
```

### 7. Controls & Data Export
Inspect status, export all data, or purge specific paths:
```bash
# View statistics
./mini-me status

# Export backup copy, profile card, facts JSON, and chunks JSONL
./mini-me export ./export_backup/

# Purge chunks by file path
./mini-me forget --path docs/DESIGN.md
```

---

## License

MIT License.
