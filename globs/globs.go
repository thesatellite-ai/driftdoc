// Package globs resolves a claim's Where (a file path or a doublestar glob)
// into the concrete list of regular files a matcher should read.
//
// doublestar gives us `**` (recursive) matching that stdlib path/filepath.Glob
// lacks — so "modules/**/*.go" works as expected. Resolution is rooted at the
// registry Root so claims stay portable regardless of the caller's CWD.
package globs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar"
)

// Resolve returns the regular files matching pattern, rooted at root.
//
// pattern may be a plain relative path ("modules/apiadmin/boot.go") or a
// doublestar glob ("modules/**/*.go"). Directories and symlinked dirs are
// filtered out — matchers only read file content. The returned paths are
// absolute so callers can read them directly.
//
// A pattern that matches nothing returns an empty slice + nil error; the caller
// (match.Run) decides whether "no files" is acceptable (it is for an `absent`
// claim, an error for every other kind).
func Resolve(root, pattern string) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("globs: empty pattern")
	}
	// Root-relative patterns resolve against root; absolute patterns are honored
	// as-is (escape hatch for cross-repo claims).
	joined := pattern
	if !filepath.IsAbs(pattern) {
		joined = filepath.Join(root, pattern)
	}
	matches, err := doublestar.Glob(joined)
	if err != nil {
		return nil, fmt.Errorf("globs: bad pattern %q: %w", pattern, err)
	}
	files := make([]string, 0, len(matches))
	for _, m := range matches {
		info, statErr := os.Stat(m)
		if statErr != nil {
			// A match that can't be stat'd (race / broken symlink) is skipped
			// rather than failing the whole claim — it simply isn't a readable file.
			continue
		}
		if info.IsDir() {
			continue
		}
		abs, absErr := filepath.Abs(m)
		if absErr != nil {
			abs = m
		}
		files = append(files, abs)
	}
	return files, nil
}
