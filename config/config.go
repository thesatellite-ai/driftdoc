// Package config loads a drift.yaml registry from disk into validated Claims.
//
// drift.yaml is the SIDECAR registry — it lives next to a doc (or at a repo
// root) and lists every machine-checkable claim plus shared defaults. Keeping
// the registry separate from prose means the check-spec is one greppable file;
// `report` later stamps live status back into the doc to bridge the two.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thesatellite-ai/driftdoc/claim"
	"gopkg.in/yaml.v3"
)

// SchemaVersion is the drift.yaml format version this binary understands. A
// registry with a newer version is rejected rather than silently mis-read.
const SchemaVersion = 1

// Defaults are registry-wide values applied to every claim that doesn't override
// them. Root anchors all globs; Doc is the default doc the claims back (used by
// `report`).
type Defaults struct {
	// Root is the directory that claim globs resolve against. Relative to the
	// drift.yaml's own location. Defaults to "." (the registry's directory).
	Root string `yaml:"root"`
	// Doc is the default doc path these claims document (relative to Root).
	// Optional; a claim may override via its own Doc field.
	Doc string `yaml:"doc"`
}

// File is a parsed drift.yaml registry.
type File struct {
	Version  int           `yaml:"version"`
	Defaults Defaults      `yaml:"defaults"`
	Claims   []claim.Claim `yaml:"claims"`

	// path is the absolute path the registry was loaded from (for resolving
	// Root + Doc relative to the file, not the caller's CWD). Not serialized.
	path string `yaml:"-"`
}

// AbsRoot returns the absolute root directory that claim globs resolve against:
// Defaults.Root joined to the registry file's own directory.
func (f *File) AbsRoot() string {
	dir := filepath.Dir(f.path)
	root := f.Defaults.Root
	if root == "" {
		root = "."
	}
	if filepath.IsAbs(root) {
		return root
	}
	return filepath.Join(dir, root)
}

// EffectiveDoc returns the absolute path of the doc a claim backs — the claim's
// own Doc if set, else the registry default — resolved against AbsRoot. Returns
// "" when neither is set (the claim isn't tied to a doc, so `report` skips it).
func (f *File) EffectiveDoc(c claim.Claim) string {
	doc := c.Doc
	if doc == "" {
		doc = f.Defaults.Doc
	}
	if doc == "" {
		return ""
	}
	if filepath.IsAbs(doc) {
		return doc
	}
	return filepath.Join(f.AbsRoot(), doc)
}

// Load reads, parses, and validates a drift.yaml at path. It fails loudly on a
// missing file, malformed yaml, an unsupported version, an unknown matcher kind,
// or a duplicate claim id — so a broken registry can never masquerade as "all
// green, nothing to check."
func Load(path string) (*File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("config: resolving %q: %w", path, err)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("config: reading %q: %w", path, err)
	}

	var f File
	// KnownFields catches typo'd keys instead of silently ignoring them.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("config: parsing %q: %w", path, err)
	}
	f.path = abs

	if f.Version == 0 {
		return nil, fmt.Errorf("config: %q is missing 'version' (expected %d)", path, SchemaVersion)
	}
	if f.Version != SchemaVersion {
		return nil, fmt.Errorf("config: %q has version %d; this driftdoc understands version %d", path, f.Version, SchemaVersion)
	}
	if len(f.Claims) == 0 {
		return nil, fmt.Errorf("config: %q declares no claims", path)
	}

	seen := make(map[string]struct{}, len(f.Claims))
	for i := range f.Claims {
		c := f.Claims[i]
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		if _, dup := seen[c.ID]; dup {
			return nil, fmt.Errorf("config: duplicate claim id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
	}
	return &f, nil
}
