package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

type ChunkResult struct {
	Text     string            `json:"text"`     // Heading breadcrumb + content
	RawText  string            `json:"raw_text"` // Pure content chunk
	Hash     string            `json:"hash"`     // SHA-256 of Text
	Metadata map[string]string `json:"metadata"` // Frontmatter key-values
}

type ChunkerConfig struct {
	TargetTokens int // Target words/tokens (~250-350)
	Overlap      int // Overlap words (~30-50)
}

func DefaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		TargetTokens: 300,
		Overlap:      40,
	}
}

type MarkdownChunker struct {
	cfg ChunkerConfig
}

func NewMarkdownChunker(cfg ...ChunkerConfig) *MarkdownChunker {
	c := DefaultChunkerConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &MarkdownChunker{cfg: c}
}

var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// ParseFrontmatter extracts YAML frontmatter between starting and ending '---'.
func ParseFrontmatter(content string) (map[string]string, string) {
	meta := make(map[string]string)
	trimmed := strings.TrimSpace(content)

	if !strings.HasPrefix(trimmed, "---") {
		return meta, content
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return meta, content
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return meta, content
	}

	for i := 1; i < endIdx; i++ {
		line := lines[i]
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if k != "" {
				meta[k] = v
			}
		}
	}

	body := strings.Join(lines[endIdx+1:], "\n")
	return meta, body
}

// ChunkMarkdown parses markdown text, tracks heading stack, and splits into breadcrumb-prepended chunks.
func (mc *MarkdownChunker) ChunkMarkdown(content string) ([]*ChunkResult, error) {
	meta, body := ParseFrontmatter(content)
	lines := strings.Split(body, "\n")

	type section struct {
		heading string
		lines   []string
	}

	var headingStack []string
	var sections []section
	var currentSection section

	buildBreadcrumb := func() string {
		if len(headingStack) == 0 {
			return ""
		}
		return strings.Join(headingStack, " > ")
	}

	for _, line := range lines {
		matches := headingRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			// Save current accumulated section
			if len(currentSection.lines) > 0 || currentSection.heading != "" {
				sections = append(sections, currentSection)
			}

			level := len(matches[1])
			title := strings.TrimSpace(matches[2])
			headingTag := fmt.Sprintf("%s %s", matches[1], title)

			// Adjust stack depth
			if level <= len(headingStack) {
				headingStack = headingStack[:level-1]
			}
			headingStack = append(headingStack, headingTag)

			currentSection = section{
				heading: buildBreadcrumb(),
				lines:   nil,
			}
		} else {
			currentSection.lines = append(currentSection.lines, line)
		}
	}
	if len(currentSection.lines) > 0 || currentSection.heading != "" {
		sections = append(sections, currentSection)
	}

	var results []*ChunkResult

	for _, sec := range sections {
		secText := strings.TrimSpace(strings.Join(sec.lines, "\n"))
		if secText == "" {
			continue
		}

		words := strings.Fields(secText)
		if len(words) == 0 {
			continue
		}

		// Split text into sliding token windows if section is large
		step := mc.cfg.TargetTokens - mc.cfg.Overlap
		if step <= 0 {
			step = mc.cfg.TargetTokens
		}

		for i := 0; i < len(words); i += step {
			end := i + mc.cfg.TargetTokens
			if end > len(words) {
				end = len(words)
			}

			rawChunk := strings.Join(words[i:end], " ")

			var fullText string
			if sec.heading != "" {
				fullText = fmt.Sprintf("Breadcrumb: %s\n\n%s", sec.heading, rawChunk)
			} else {
				fullText = rawChunk
			}

			hashBytes := sha256.Sum256([]byte(fullText))
			hashHex := hex.EncodeToString(hashBytes[:])

			results = append(results, &ChunkResult{
				Text:     fullText,
				RawText:  rawChunk,
				Hash:     hashHex,
				Metadata: meta,
			})

			if end == len(words) {
				break
			}
		}
	}

	return results, nil
}
