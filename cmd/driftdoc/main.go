// driftdoc is a generic doc↔code drift engine: it checks machine-verifiable
// claims (declared in a sidecar drift.yaml) against a codebase, so documentation
// fails CI when it stops being true instead of rotting silently.
//
// Build:   go build -trimpath -ldflags='-s -w -X main.version=<v>' -o bin/driftdoc ./cmd/driftdoc
// Run:     driftdoc check          (in a dir with drift.yaml, or pass --config)
//
// Design + roadmap: DESIGN.md. This binary is the MVP surface (check / init /
// version); report / gen / fix / impact land in later phases per the design.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags '-X main.version=<value>'.
var version = "0.1.0-dev"

// BinaryName is the single source for this tool's name in every help/hint
// string. Renaming the binary is then one constant edit + rebuild (mirrors the
// lore convention).
const BinaryName = "driftdoc"

// rootCmd is the cobra root. We render our own errors (SilenceErrors) and never
// dump usage on a runtime failure (SilenceUsage) — a failed `check` should print
// the drift, not the help text.
var rootCmd = &cobra.Command{
	Use:   BinaryName,
	Short: "driftdoc — keep docs honest: check machine-verifiable claims against code",
	Long: `driftdoc is a generic doc↔code drift engine.

You declare claims in a sidecar drift.yaml — each claim says "in <where>,
<match> <expect> should hold" (e.g. an endpoint path exists, a removed symbol is
absent, a value appears exactly N times). 'driftdoc check' verifies them and
exits non-zero on drift, so stale docs break CI instead of misleading readers.

Works for any codebase (Go, TS, SQL, config, markdown) — matchers operate on
file content, not on any one language. See DESIGN.md for the roadmap.`,
	Version:       version,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	rootCmd.AddCommand(newCheckCommand())
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newReportCommand())
	rootCmd.AddCommand(newImpactCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Commands return a cliError when they want a specific exit code; default
		// to 1 for any other error.
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		if ce, ok := err.(cliError); ok {
			os.Exit(ce.code)
		}
		os.Exit(1)
	}
}

// cliError carries an explicit process exit code out of a command's RunE so
// `main` can distinguish "drift found" (1) from "config/IO error" (2) — the
// contract CI relies on.
type cliError struct {
	code int
	msg  string
}

func (e cliError) Error() string { return e.msg }
