// Package match runs a single Claim against the codebase and produces a Result.
//
// This is the heart of the engine and the reason it scales: there is NO
// per-claim code. A small set of generic matchers (Tier 1: text) operate on
// file bytes; each Claim simply names the matcher (claim.MatchKind) and supplies
// its argument (claim.Expect). Later tiers (structured/json-yaml, symbol/AST,
// behavioral/live) plug in here as additional kinds without changing callers.
package match

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/globs"
)

// Run resolves the claim's files (relative to root) and applies its matcher,
// returning a fully-populated Result. It never panics and never returns an
// error — every failure mode is encoded as claim.StatusError on the Result, so
// a misconfigured claim shows up as a visible failure rather than crashing the
// run or (worse) passing silently.
func Run(c claim.Claim, root string) claim.Result {
	res := claim.Result{Claim: c}

	files, err := globs.Resolve(root, c.Where)
	if err != nil {
		res.Status = claim.StatusError
		res.Detail = err.Error()
		return res
	}
	res.FilesMatched = len(files)

	// No files matched: trivially "absent" (the string can't exist where there
	// are no files), but an error for any kind that needs something to inspect —
	// almost always a stale/wrong `where`.
	if len(files) == 0 {
		if c.Match == claim.MatchAbsent {
			res.Status = claim.StatusOK
			res.Detail = fmt.Sprintf("0 files match %q — nothing can contain %q", c.Where, c.Expect)
			return res
		}
		res.Status = claim.StatusError
		res.Detail = fmt.Sprintf("glob %q matched no files", c.Where)
		return res
	}

	hits, hErr := countHits(c, files)
	if hErr != nil {
		res.Status = claim.StatusError
		res.Detail = hErr.Error()
		return res
	}
	res.Hits = hits

	switch c.Match {
	case claim.MatchContains, claim.MatchRegex:
		if hits >= 1 {
			res.Status = claim.StatusOK
		} else {
			res.Status = claim.StatusDrift
			res.Detail = fmt.Sprintf("expected %q in %d file(s); not found", c.Expect, len(files))
		}
	case claim.MatchAbsent:
		if hits == 0 {
			res.Status = claim.StatusOK
		} else {
			res.Status = claim.StatusDrift
			res.Detail = fmt.Sprintf("expected %q to be absent, but found it %d time(s)", c.Expect, hits)
		}
	case claim.MatchCount:
		if hits == c.Count {
			res.Status = claim.StatusOK
		} else {
			res.Status = claim.StatusDrift
			res.Detail = fmt.Sprintf("expected exactly %d occurrence(s) of %q, found %d", c.Count, c.Expect, hits)
		}
	default:
		// Unreachable when claims are validated, but never silently pass.
		res.Status = claim.StatusError
		res.Detail = fmt.Sprintf("unknown match kind %q", c.Match)
	}
	return res
}

// countHits totals occurrences of the claim's Expect across all files. For
// regex it counts non-overlapping pattern matches; for every other (substring)
// kind it counts substring occurrences. Compiling/reading errors abort the
// claim (returned to Run as a StatusError).
func countHits(c claim.Claim, files []string) (int, error) {
	var re *regexp.Regexp
	if c.Match == claim.MatchRegex {
		var err error
		re, err = regexp.Compile(c.Expect)
		if err != nil {
			return 0, fmt.Errorf("claim %q: invalid regex %q: %w", c.ID, c.Expect, err)
		}
	}
	total := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return 0, fmt.Errorf("claim %q: reading %s: %w", c.ID, f, err)
		}
		content := string(b)
		if re != nil {
			total += len(re.FindAllStringIndex(content, -1))
		} else {
			total += strings.Count(content, c.Expect)
		}
	}
	return total, nil
}
