// Package runner executes every claim in a registry and aggregates the results.
//
// It is deliberately thin: load → for each claim, match.Run → collect. No
// concurrency yet (claim counts are small and matchers are IO-light); if a
// registry ever grows large this is the one place to parallelize.
package runner

import (
	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/config"
	"github.com/thesatellite-ai/driftdoc/match"
)

// Summary is the aggregate outcome of a run — the numbers the status banner and
// the CLI exit code are derived from.
type Summary struct {
	Total  int
	OK     int
	Drift  int
	Errors int
}

// Clean reports whether every claim passed (no drift, no errors) — the signal
// for a green `check` (exit 0).
func (s Summary) Clean() bool { return s.Drift == 0 && s.Errors == 0 }

// Run checks every claim in f against the registry's resolved root and returns
// the per-claim results plus an aggregate Summary. Results preserve registry
// order so reports are stable across runs.
func Run(f *config.File) ([]claim.Result, Summary) {
	root := f.AbsRoot()
	results := make([]claim.Result, 0, len(f.Claims))
	for _, c := range f.Claims {
		results = append(results, match.Run(c, root))
	}
	return results, Summarize(results)
}

// Summarize tallies a slice of results into a Summary. Exposed so callers that
// work with a SUBSET of results (e.g. report grouping by doc) get the same
// counting logic instead of re-implementing it.
func Summarize(results []claim.Result) Summary {
	var sum Summary
	for _, r := range results {
		sum.Total++
		switch r.Status {
		case claim.StatusOK:
			sum.OK++
		case claim.StatusDrift:
			sum.Drift++
		case claim.StatusError:
			sum.Errors++
		}
	}
	return sum
}
