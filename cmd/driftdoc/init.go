package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// starterRegistry is the scaffold written by `driftdoc init` — a minimal, valid,
// self-documenting drift.yaml showing one claim per matcher kind so a new user
// can edit rather than start from a blank file.
const starterRegistry = `version: 1

# driftdoc registry — machine-checkable claims about this codebase.
# Run "driftdoc check" to verify. Exit 0 = all current, 1 = drift, 2 = error.
# Each claim: in <where>, <match> <expect> should hold.

defaults:
  root: .                      # globs resolve against this dir (relative to THIS file)
  # doc: docs/ARCHITECTURE.md  # default doc these claims back (used by 'report')

claims:
  - id: example-contains
    desc: the README mentions the project name
    where: README.md
    match: contains
    expect: "driftdoc"

  # - id: example-absent          # assert a removed thing stays gone
  #   desc: the deprecated flag was removed
  #   where: "**/*.go"
  #   match: absent
  #   expect: "OldDeprecatedFlag"

  # - id: example-regex           # assert a pattern exists
  #   desc: the main entrypoint defines a version var
  #   where: cmd/**/*.go
  #   match: regex
  #   expect: 'var version'

  # - id: example-count           # assert an exact occurrence count
  #   desc: there is exactly one root command
  #   where: cmd/**/*.go
  #   match: count
  #   expect: "rootCmd ="
  #   count: 1
`

// newInitCommand builds `driftdoc init` — scaffold a starter drift.yaml. Refuses
// to clobber an existing registry unless --force, so it's safe to run anywhere.
func newInitCommand() *cobra.Command {
	var (
		path  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter drift.yaml registry",
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := os.Stat(path); err == nil && !force {
				return cliError{code: 2, msg: fmt.Sprintf("%s already exists (use --force to overwrite)", path)}
			}
			if err := os.WriteFile(path, []byte(starterRegistry), 0o644); err != nil {
				return cliError{code: 2, msg: err.Error()}
			}
			fmt.Printf("wrote %s — edit the claims, then run `%s check`\n", path, BinaryName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&path, "config", "c", defaultConfigName, "path to write the registry to")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing registry")
	return cmd
}
