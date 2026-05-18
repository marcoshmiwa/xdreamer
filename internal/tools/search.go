package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const maxSearchMatches = 200

type searchCodeTool struct{}

func NewSearchCodeTool() Tool { return &searchCodeTool{} }

func (t *searchCodeTool) Name() string        { return "search_code" }
func (t *searchCodeTool) Destructive() bool   { return false }
func (t *searchCodeTool) Description() string { return "Search files for lines matching a regex pattern." }
func (t *searchCodeTool) Parameters() map[string]any {
	return map[string]any{
		"pattern": map[string]any{"type": "string", "description": "Regex pattern to search for"},
		"glob":    map[string]any{"type": "string", "description": "File glob filter, e.g. **/*.go"},
		"root":    map[string]any{"type": "string", "description": "Directory to search in"},
	}
}

type searchCodeParams struct {
	Pattern string `json:"pattern"`
	Glob    string `json:"glob"`
	Root    string `json:"root"`
}

func (t *searchCodeTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p searchCodeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("search code: parse params: %w", err)
	}
	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return "", fmt.Errorf("search code: invalid pattern: %w", err)
	}
	return searchFiles(p.Root, p.Glob, re)
}

func searchFiles(root, glob string, re *regexp.Regexp) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := filepath.ToSlash(fpath)
		if r, relErr := filepath.Rel(root, fpath); relErr == nil {
			rel = filepath.ToSlash(r)
		}
		if !globMatches(glob, rel) {
			return nil
		}
		fileMatches, scanErr := scanFile(fpath, rel, re)
		if scanErr != nil {
			return scanErr
		}
		matches = append(matches, fileMatches...)
		if len(matches) >= maxSearchMatches {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search code: %w", err)
	}
	result := strings.Join(matches, "\n")
	if len(matches) >= maxSearchMatches {
		result += "\n[output truncated at 200 matches]"
	}
	return result, nil
}

func scanFile(fpath, rel string, re *regexp.Regexp) ([]string, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", fpath, err)
	}
	defer f.Close()

	var results []string
	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			results = append(results, fmt.Sprintf("%s:%d: %s", rel, lineNum, line))
		}
	}
	return results, scanner.Err()
}

// globMatches reports whether relPath matches the glob pattern.
// Supports the **/ prefix for recursive matching against the base name.
func globMatches(glob, relPath string) bool {
	if glob == "" {
		return true
	}
	if strings.HasPrefix(glob, "**/") {
		ok, _ := path.Match(glob[3:], path.Base(relPath))
		return ok
	}
	ok, _ := path.Match(glob, relPath)
	return ok
}
