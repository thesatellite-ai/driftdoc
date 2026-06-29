package main

import (
	"fmt"
	"os"
	"time"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/config"
	"github.com/thesatellite-ai/driftdoc/report"
	"github.com/thesatellite-ai/driftdoc/runner"
	"github.com/spf13/cobra"
)

// newReportCommand builds `driftdoc report` — verify claims and stamp a live
// status block into each doc they back (so a reader sees, at the top of the doc,
// which documented facts are still verified). Without --write it previews the
// blocks; with --write it edits the docs in place (idempotent marker block).
func newReportCommand() *cobra.Command {
	var (
		configPath string
		write      bool
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Stamp a live driftdoc status block into each documented doc",
		Long: `report verifies every claim, then groups them by the doc they back
(claim.doc or defaults.doc) and writes a status block into each doc — a verdict
banner + a per-claim table, fenced by markers so re-running overwrites it.

Without --write it prints what it would stamp. With --write it edits the docs.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			f, err := config.Load(configPath)
			if err != nil {
				return cliError{code: 2, msg: err.Error()}
			}
			results, _ := runner.Run(f)

			// Group results by the absolute doc path each claim backs, preserving
			// first-seen order for stable output.
			byDoc := map[string][]claim.Result{}
			order := []string{}
			skipped := 0
			for _, r := range results {
				doc := f.EffectiveDoc(r.Claim)
				if doc == "" {
					skipped++
					continue
				}
				if _, ok := byDoc[doc]; !ok {
					order = append(order, doc)
				}
				byDoc[doc] = append(byDoc[doc], r)
			}
			if len(order) == 0 {
				return cliError{code: 2, msg: "no claim references a doc (set defaults.doc or a claim's doc)"}
			}

			now := time.Now()
			for _, doc := range order {
				rs := byDoc[doc]
				block := report.BuildStatusBlock(rs, runner.Summarize(rs), now)
				if write {
					changed, err := report.StampDoc(doc, block)
					if err != nil {
						return cliError{code: 2, msg: err.Error()}
					}
					state := "unchanged"
					if changed {
						state = "updated"
					}
					fmt.Printf("%-9s %s  (%d claims)\n", state, doc, len(rs))
				} else {
					fmt.Fprintf(os.Stdout, "---- %s ----\n%s\n\n", doc, block)
				}
			}
			if skipped > 0 {
				fmt.Printf("(%d claim(s) had no doc and were skipped)\n", skipped)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigName, "path to the drift.yaml registry")
	cmd.Flags().BoolVar(&write, "write", false, "edit the docs in place (default: preview only)")
	return cmd
}
