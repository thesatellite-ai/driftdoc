package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thesatellite-ai/driftdoc/config"
	"github.com/thesatellite-ai/driftdoc/impact"
	"github.com/spf13/cobra"
)

// newImpactCommand builds `driftdoc impact` — given a set of changed files
// (from git, or an explicit --files list), report which documented claims those
// changes touch, so a reviewer knows exactly which doc sections to re-check.
// Informational by default (exit 0); --exit-code makes it a gate (exit 1 when
// any claim is impacted) for PR bots that want to force a doc review.
func newImpactCommand() *cobra.Command {
	var (
		configPath string
		gitRef     string
		filesCSV   string
		exitCode   bool
	)
	cmd := &cobra.Command{
		Use:   "impact",
		Short: "Given a diff, list the documented claims (and docs) it touches",
		Long: `impact runs the registry in REVERSE: it takes a set of changed files and
reports which claims' 'where' covers them — i.e. which documented facts a diff
may have invalidated. Source the diff from git (--git <ref>, default HEAD) or an
explicit --files list.

Use it in a PR check to tell the author "you changed code these docs describe —
review them." Add --exit-code to make an impacted run fail (exit 1).`,
		RunE: func(_ *cobra.Command, _ []string) error {
			f, err := config.Load(configPath)
			if err != nil {
				return cliError{code: 2, msg: err.Error()}
			}

			changed, err := changedFiles(f, gitRef, filesCSV)
			if err != nil {
				return cliError{code: 2, msg: err.Error()}
			}
			if len(changed) == 0 {
				fmt.Println("no changed files detected — nothing to analyze")
				return nil
			}

			hits := impact.Analyze(f, changed)
			if len(hits) == 0 {
				fmt.Printf("%d changed file(s); no documented claims affected ✅\n", len(changed))
				return nil
			}

			fmt.Printf("%d changed file(s) touch %d documented claim(s) — review:\n\n", len(changed), len(hits))
			for _, h := range hits {
				fmt.Printf("  • %s — %s\n", h.Claim.ID, h.Claim.Desc)
				if h.Doc != "" {
					loc := h.Doc
					if h.Claim.Section != "" {
						loc += " " + h.Claim.Section
					}
					fmt.Printf("      doc:    %s\n", loc)
				}
				fmt.Printf("      via:    %s\n", summarizeFiles(rel(f.AbsRoot(), h.Files)))
			}
			if exitCode {
				return cliError{code: 1, msg: fmt.Sprintf("%d documented claim(s) impacted", len(hits))}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigName, "path to the drift.yaml registry")
	cmd.Flags().StringVar(&gitRef, "git", "HEAD", "diff against this git ref (e.g. origin/main); ignored if --files is set")
	cmd.Flags().StringVar(&filesCSV, "files", "", "comma-separated changed files (overrides --git)")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "exit 1 when any documented claim is impacted")
	return cmd
}

// changedFiles resolves the changed-file set: an explicit --files list wins;
// otherwise `git diff --name-only <ref>` run at the registry's git root.
func changedFiles(f *config.File, gitRef, filesCSV string) ([]string, error) {
	if strings.TrimSpace(filesCSV) != "" {
		parts := strings.Split(filesCSV, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	}
	return gitChangedFiles(f.AbsRoot(), gitRef)
}

// gitChangedFiles returns files changed vs ref, as absolute paths. It resolves
// the git toplevel (which may sit above the registry root) so the returned paths
// line up with the claims' resolved files.
func gitChangedFiles(root, ref string) ([]string, error) {
	top, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("impact: not a git repo at %s (use --files): %w", root, err)
	}
	gitRoot := strings.TrimSpace(string(top))

	out, err := exec.Command("git", "-C", gitRoot, "diff", "--name-only", ref).Output()
	if err != nil {
		return nil, fmt.Errorf("impact: git diff %s failed: %w", ref, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, filepath.Join(gitRoot, line))
		}
	}
	return files, nil
}

// summarizeFiles keeps the "via" line readable: a broad glob (modules/**) can
// match dozens of changed files; show the first few + a "+N more" tail.
func summarizeFiles(files []string) string {
	const show = 3
	if len(files) <= show {
		return strings.Join(files, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(files[:show], ", "), len(files)-show)
}

// rel shortens absolute paths to root-relative for display.
func rel(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if r, err := filepath.Rel(root, p); err == nil {
			out = append(out, r)
		} else {
			out = append(out, p)
		}
	}
	return out
}
