// Package impact answers the REVERSE question — the engine's strongest lever.
//
// Forward (check): code changed → which doc claims lied?
// Reverse (impact): given a diff, which documented claims does it TOUCH — i.e.
// "you changed code these docs describe; review them before merging." The claim
// registry runs both directions over the same doc↔code map, which is what turns
// docs from an after-the-fact chore into a checklist the PR hands you.
package impact

import (
	"path/filepath"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/config"
	"github.com/thesatellite-ai/driftdoc/globs"
)

// Hit is one claim affected by the changed file set, with the specific changed
// files that fall under its Where and the doc it backs (for "go review this").
type Hit struct {
	Claim claim.Claim
	Doc   string   // effective doc path (may be "")
	Files []string // changed files that the claim's Where covers
}

// Analyze returns every claim whose Where resolves to at least one of the
// changed files. `changed` may be repo-relative or absolute; both are normalized
// to absolute, cleaned paths before the membership test, so the source of the
// diff (git, an explicit list, an editor) doesn't matter.
func Analyze(f *config.File, changed []string) []Hit {
	root := f.AbsRoot()

	changedSet := make(map[string]struct{}, len(changed))
	for _, c := range changed {
		if c == "" {
			continue
		}
		p := c
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, c)
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		changedSet[filepath.Clean(p)] = struct{}{}
	}
	if len(changedSet) == 0 {
		return nil
	}

	var hits []Hit
	for _, cl := range f.Claims {
		files, err := globs.Resolve(root, cl.Where)
		if err != nil {
			continue // a bad glob is surfaced by `check`, not here
		}
		var matched []string
		for _, fp := range files {
			if _, ok := changedSet[filepath.Clean(fp)]; ok {
				matched = append(matched, fp)
			}
		}
		if len(matched) > 0 {
			hits = append(hits, Hit{Claim: cl, Doc: f.EffectiveDoc(cl), Files: matched})
		}
	}
	return hits
}
