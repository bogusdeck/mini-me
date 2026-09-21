# mini-me 🧠

> **Your privacy-first, local-first digital twin and RAG context engine.**

`mini-me` scans your browser history, documents, resumes, and local projects to build a secure, searchable personal knowledge graph—giving local LLMs and AI assistants deep context about **who you are, what you build, and what you know**, without sending a single byte to the cloud.

---

## 🎯 Why `mini-me`? (The Problem)

Every developer and power user faces **context fragmentation**:
- Your personal context lives scattered across browser history, local notes, project repositories, resumes, and system files.
- Cloud-based AI assistants require you to trade away total privacy, uploading your entire life's digital footprint, work, and history to third-party cloud servers.
- Local LLMs and AI coding assistants start with zero knowledge about your background, active projects, past solutions, or preferred tools.

### The Solution
`mini-me` acts as a **local-first digital twin**:
1. **Automatic Ingestion**: Scans local browser cache/history (Chrome, Brave, Safari), resumes, project repositories, and notes.
2. **Privacy by Design**: Runs 100% locally on your machine. Includes automated secret redaction and strict PII safety rules (never stores raw SSNs or financial credentials).
3. **Hybrid RAG & MCP Engine**: Exposes hybrid vector search (KNN + BM25 FTS5) via a local HTTP REST API and a Model Context Protocol (MCP) server so your local LLM or AI assistant can query your context seamlessly.

---

## 🚀 Quick Install

### Homebrew (macOS)
```bash
brew tap bogusdeck/mini-me https://github.com/bogusdeck/mini-me.git
brew install bogusdeck/mini-me/mini-me
brew services start mini-me
```

### Debian / Ubuntu (`apt-get`)
```bash
# Build and install .deb package
make deb
sudo apt-get install ./dist/mini-me_0.1.0_amd64.deb
```

### Arch Linux (`pacman`)
```bash
# Install via PKGBUILD
git clone https://github.com/bogusdeck/mini-me.git
cd mini-me
makepkg -si
```

### Go Install (Cross-platform)
```bash
go install github.com/bogusdeck/mini-me/cmd/mini-me@latest
```

---

## 🔌 Integration Guide (AI Agents & Developers)

Looking to integrate `mini-me` into your AI tools or custom projects? Check out the **[Integration Guide](docs/INTEGRATION.md)** for:
- 🤖 **AI Agents & MCP Clients** (Claude Desktop, Cursor IDE, Antigravity CLI, VS Code)
- 💻 **Application Developers** (Python, TypeScript/Node.js, Go, cURL REST API examples)

---

## ⚡ Quickstart

### 1. Run System Health Check
Verify SQLite Wasm engine and local Ollama embedding model (`nomic-embed-text`):
```bash
mini-me doctor
```

### 2. Set Up Your Profile
Initialize your core profile card:
```bash
mini-me init --name "Your Name" --email "you@example.com" --employer "Your Company"
mini-me profile
```

### 3. Scan Your Data
Scan browser history and ingest local files or project directories:
```bash
# Scan browser history (Chrome, Brave, Safari)
mini-me scan browser

# Ingest documents, resumes, or project directories
mini-me scan docs ~/Documents ~/Projects
```

### 4. Query Your Digital Twin
Perform hybrid search across your indexed context:
```bash
mini-me search "recent projects and resume details"
```

### 5. Run as Background Service
Cross-platform service management (launchd on macOS, systemd on Linux):
```bash
mini-me service install   # Installs & starts user service (launchd / systemd)
mini-me service status    # Checks status
mini-me service stop      # Stops service
```

---

## 🔒 Security & Privacy

- **Zero CGO / Pure Go**: Powered by WebAssembly SQLite (`ncruces/go-sqlite3`) + `sqlite-vec`.
- **Local Storage Only**: Database stored at `~/Library/Application Support/mini-me/mini-me.db` (or `~/.config/mini-me/mini-me.db` on Linux) with strict `0600` permissions.
- **Secret Redaction**: Pre-indexing regex and entropy scanners purge API tokens, SSH keys, and sensitive patterns.
- **Local Embeddings**: Integrates with local [Ollama](https://ollama.com) (`nomic-embed-text`) at `http://127.0.0.1:11434`.

---

## 📄 License

MIT License.
