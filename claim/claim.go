// Package claim defines the core domain types of the driftdoc engine.
//
// A Claim is one machine-checkable assertion about a codebase — "in <Where>,
// <Match> <Expect> should hold." A Result is the outcome of checking a Claim.
// These types are language-agnostic on purpose: a matcher operates on file
// bytes (and, in later tiers, on parsed/structured/running artifacts), never on
// any single programming language. Every other package (config, match, runner,
// report) is built around these two types.
package claim

import "fmt"

// MatchKind is the CLOSED set of generic matchers a claim can use. Keeping it a
// typed string (not a bare literal at call sites) means: yaml unmarshals it
// directly, the validator can reject typos, and AllMatchKinds is the single
// canonical iteration list shared by read + write paths.
type MatchKind string

const (
	// MatchContains passes when the Expect substring appears in ≥1 matched file.
	// The default, language-agnostic check (a documented endpoint path, a scope
	// name, an SDL field — anything that is a stable contract string).
	MatchContains MatchKind = "contains"

	// MatchRegex passes when the Expect RE2 pattern matches in ≥1 matched file.
	// Uses Go's regexp (RE2 — linear time, no catastrophic backtracking on a
	// hostile pattern). Use when a literal substring is too blunt.
	MatchRegex MatchKind = "regex"

	// MatchAbsent passes when the Expect substring appears in ZERO matched files.
	// This is how a documented DELETION stays honest ("the robot user was
	// removed" → assert `EnsureSystemUser` no longer exists anywhere).
	MatchAbsent MatchKind = "absent"

	// MatchCount passes when the Expect substring appears EXACTLY Claim.Count
	// times across all matched files (e.g. "exactly one active-token index").
	MatchCount MatchKind = "count"
)

// AllMatchKinds is the canonical iteration list — seeds validation, help text,
// and any future UI so the read and write paths can never diverge.
var AllMatchKinds = []MatchKind{MatchContains, MatchRegex, MatchAbsent, MatchCount}

// Valid reports whether k is one of the known matcher kinds.
func (k MatchKind) Valid() bool {
	for _, known := range AllMatchKinds {
		if k == known {
			return true
		}
	}
	return false
}

// Claim is one machine-checkable assertion declared in drift.yaml. The yaml
// tags ARE the on-disk contract — renaming a field is a breaking change to every
// existing drift.yaml, so treat them as fixed.
type Claim struct {
	// ID is a stable, human-chosen identifier (e.g. "apiadmin-versioned"). It is
	// the key used in reports, --filter, and (later tiers) the `// drift:<id>`
	// code-marker convention. Required + unique within a registry.
	ID string `yaml:"id"`

	// Desc is the human-readable claim this row enforces — the sentence a reader
	// would find in the doc. Surfaced in reports and the AI fix bundle.
	Desc string `yaml:"desc"`

	// Where is a file path OR a doublestar glob (e.g. "modules/**/*.go"),
	// resolved relative to the registry's Root. The set of files the matcher runs
	// over.
	Where string `yaml:"where"`

	// Match selects the generic matcher (see the MatchKind constants).
	Match MatchKind `yaml:"match"`

	// Expect is the matcher's argument: a substring (contains/absent/count) or an
	// RE2 pattern (regex).
	Expect string `yaml:"expect"`

	// Count is the required occurrence count — ONLY consulted when Match is
	// MatchCount. Zero for every other kind.
	Count int `yaml:"count,omitempty"`

	// Doc optionally overrides the registry's default doc path this claim backs
	// (used by `report` to stamp status into the right file).
	Doc string `yaml:"doc,omitempty"`

	// Section optionally records the anchor in Doc where the claim is documented
	// (e.g. "#api-reference") — for navigation + the AI fix bundle.
	Section string `yaml:"section,omitempty"`

	// Note is free-form context for maintainers; never affects checking.
	Note string `yaml:"note,omitempty"`
}

// Validate returns an error describing the first problem with the claim, or nil.
// Called by config.Load after unmarshal so a malformed registry fails loudly
// instead of silently checking nothing.
func (c Claim) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("claim is missing required field 'id'")
	}
	if c.Where == "" {
		return fmt.Errorf("claim %q: missing required field 'where'", c.ID)
	}
	if !c.Match.Valid() {
		return fmt.Errorf("claim %q: unknown match kind %q (want one of %v)", c.ID, c.Match, AllMatchKinds)
	}
	if c.Expect == "" {
		return fmt.Errorf("claim %q: missing required field 'expect'", c.ID)
	}
	if c.Match == MatchCount && c.Count <= 0 {
		return fmt.Errorf("claim %q: match 'count' requires a positive 'count:' value", c.ID)
	}
	return nil
}

// Status is the outcome of checking a Claim.
type Status string

const (
	// StatusOK — the expectation holds; the doc claim is still true.
	StatusOK Status = "ok"
	// StatusDrift — the expectation no longer holds; the doc has drifted from code.
	StatusDrift Status = "drift"
	// StatusError — the claim could not be checked (bad glob, bad regex, IO
	// error, no files matched). Treated as a failure so misconfiguration is never
	// silently green.
	StatusError Status = "error"
)

// Result is the outcome of checking one Claim against the codebase.
type Result struct {
	Claim        Claim  // the claim that was checked
	Status       Status // ok | drift | error
	FilesMatched int    // how many files Where resolved to
	Hits         int    // occurrences of Expect found (kind-dependent)
	Detail       string // human explanation, especially for drift/error
}

// OK reports whether the result is a pass (StatusOK).
func (r Result) OK() bool { return r.Status == StatusOK }
