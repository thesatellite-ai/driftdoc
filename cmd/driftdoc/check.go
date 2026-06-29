package main

import (
	"fmt"
	"os"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/config"
	"github.com/thesatellite-ai/driftdoc/report"
	"github.com/thesatellite-ai/driftdoc/runner"
	"github.com/spf13/cobra"
)

// defaultConfigName is the registry file `check` looks for when --config is not
// given. A constant so the value is stated once.
const defaultConfigName = "drift.yaml"

// newCheckCommand builds `driftdoc check` — load the registry, verify every
// claim, print results, and exit 0 (clean) / 1 (drift) / 2 (error). The exit
// codes are the CI contract: a non-zero exit fails the build on drift.
func newCheckCommand() *cobra.Command {
	var (
		configPath string
		jsonOut    bool
		colorMode  string
		filterID   string
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verify drift.yaml claims against the code (exit 1 on drift, 2 on error)",
		Long: `check loads a drift.yaml registry and verifies every claim against the
codebase. Exit code: 0 = all current · 1 = drift found · 2 = config/IO error.

Drop 'driftdoc check' into CI and a documented claim that stops being true will
fail the build.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			f, err := config.Load(configPath)
			if err != nil {
				return cliError{code: 2, msg: err.Error()}
			}

			// Optional single-claim filter (--filter id=foo) for quick iteration.
			if filterID != "" {
				f.Claims = filterClaims(f.Claims, filterID)
				if len(f.Claims) == 0 {
					return cliError{code: 2, msg: fmt.Sprintf("no claim matches id=%q", filterID)}
				}
			}

			results, sum := runner.Run(f)

			if jsonOut {
				if err := report.JSON(os.Stdout, results, sum); err != nil {
					return cliError{code: 2, msg: err.Error()}
				}
			} else {
				report.Terminal(os.Stdout, results, sum, resolveColor(colorMode))
			}

			// Translate the run outcome into the CI exit-code contract.
			if sum.Errors > 0 {
				return cliError{code: 2, msg: fmt.Sprintf("%d claim(s) could not be checked", sum.Errors)}
			}
			if sum.Drift > 0 {
				return cliError{code: 1, msg: fmt.Sprintf("%d claim(s) drifted", sum.Drift)}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigName, "path to the drift.yaml registry")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON instead of a table")
	cmd.Flags().StringVar(&colorMode, "color", "auto", "color output: auto | always | never")
	cmd.Flags().StringVar(&filterID, "filter", "", "check only the claim with id=<value>")
	return cmd
}

// filterClaims returns only the claim whose id matches the `id=<value>` filter.
// Kept tiny + explicit; a richer filter DSL can replace it later.
func filterClaims(claims []claim.Claim, filter string) []claim.Claim {
	const idPrefix = "id="
	if len(filter) <= len(idPrefix) || filter[:len(idPrefix)] != idPrefix {
		return nil
	}
	want := filter[len(idPrefix):]
	out := claims[:0:0]
	for _, c := range claims {
		if c.ID == want {
			out = append(out, c)
		}
	}
	return out
}
