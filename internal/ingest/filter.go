package ingest

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const MaxFileSize = 2 * 1024 * 1024 // 2MB

var defaultIgnoredDirs = map[string]bool{
	".ssh":         true,
	".aws":         true,
	".git":         true,
	".gnupg":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"vendor":       true,
	"bin":          true,
	"obj":          true,
	".venv":        true,
	"__pycache__":  true,
}

var defaultIgnoredExts = map[string]bool{
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".svg":   true,
	".pdf":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".wasm":  true,
	".pyc":   true,
	".o":     true,
	".a":     true,
	".db":    true,
	".sqlite": true,
}

type Filter struct {
	ignorePatterns []string
}

func NewFilter() *Filter {
	return &Filter{}
}

// LoadGitignore reads patterns from a .gitignore file.
func (f *Filter) LoadGitignore(gitignorePath string) error {
	file, err := os.Open(gitignorePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f.ignorePatterns = append(f.ignorePatterns, line)
	}
	return scanner.Err()
}

// ShouldSkipPath returns true if the path should be ignored due to denylist, file size, or gitignore patterns.
func (f *Filter) ShouldSkipPath(path string, info os.FileInfo) bool {
	base := filepath.Base(path)

	// Check environment file patterns (.env, .env.local, etc.)
	if strings.HasPrefix(base, ".env") {
		return true
	}

	// Check Directory Denylist
	if info.IsDir() {
		if defaultIgnoredDirs[base] {
			return true
		}
		return false
	}

	// Check File Extension Denylist
	ext := strings.ToLower(filepath.Ext(base))
	if defaultIgnoredExts[ext] {
		return true
	}

	// Check File Size Cap (2MB)
	if info.Size() > MaxFileSize {
		return true
	}

	// Check gitignore simple pattern matching
	for _, pat := range f.ignorePatterns {
		patTrim := strings.TrimPrefix(strings.TrimSuffix(pat, "/"), "/")
		if base == patTrim || strings.HasPrefix(base, patTrim) {
			return true
		}
		if matched, err := filepath.Match(patTrim, base); err == nil && matched {
			return true
		}
	}

	return false
}
