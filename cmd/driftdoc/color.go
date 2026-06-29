package main

import "os"

// resolveColor turns the --color mode flag into a concrete on/off decision.
// "always"/"never" are explicit; "auto" (the default) enables color only when
// stdout is a real terminal and NO_COLOR is unset (the de-facto standard) — so
// piping to a file or CI produces clean, uncolored output.
func resolveColor(mode string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		fi, err := os.Stdout.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}
}
