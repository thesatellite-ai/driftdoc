// Package report renders run results for humans (a terminal table) and machines
// (JSON). It owns all presentation — color, symbols, ordering — so the runner
// and matchers stay pure logic.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/runner"
)

// ANSI color codes, gated by a ColorMode so output is clean when piped or when
// the user forces --color=never. Kept as named constants — never inline escape
// strings at call sites.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
)

// statusGlyph maps a status to its display symbol — the single source for how a
// status looks, shared across the table and the banner.
var statusGlyph = map[claim.Status]string{
	claim.StatusOK:    "OK",
	claim.StatusDrift: "DRIFT",
	claim.StatusError: "ERROR",
}

// Terminal writes a human-readable table + a one-line summary banner to w.
// color enables ANSI coloring (the caller resolves auto/always/never).
func Terminal(w io.Writer, results []claim.Result, sum runner.Summary, color bool) {
	paint := painter(color)
	for _, r := range results {
		glyph := statusGlyph[r.Status]
		var colored string
		switch r.Status {
		case claim.StatusOK:
			colored = paint(ansiGreen, glyph)
		case claim.StatusDrift:
			colored = paint(ansiRed, glyph)
		default:
			colored = paint(ansiYellow, glyph)
		}
		// One line per claim: STATUS  id  — desc. Detail on a dim follow line
		// only when something needs the reader's attention.
		fmt.Fprintf(w, "%-7s %s %s\n", colored, paint(ansiBold, r.Claim.ID), paint(ansiDim, "— "+r.Claim.Desc))
		if r.Status != claim.StatusOK && r.Detail != "" {
			fmt.Fprintf(w, "        %s\n", paint(ansiDim, r.Detail))
		}
	}
	fmt.Fprintln(w, paint(ansiDim, strings.Repeat("─", 48)))
	fmt.Fprintln(w, Banner(sum, color))
}

// Banner returns the one-line summary (used at the end of a run and reusable as
// a doc status line): "✓ 12 ok · ✗ 1 drift · ! 0 error  (13 claims)".
func Banner(sum runner.Summary, color bool) string {
	paint := painter(color)
	ok := paint(ansiGreen, fmt.Sprintf("%d ok", sum.OK))
	drift := fmt.Sprintf("%d drift", sum.Drift)
	if sum.Drift > 0 {
		drift = paint(ansiRed, drift)
	}
	errs := fmt.Sprintf("%d error", sum.Errors)
	if sum.Errors > 0 {
		errs = paint(ansiYellow, errs)
	}
	verdict := paint(ansiGreen, "ALL CURRENT")
	if !sum.Clean() {
		verdict = paint(ansiRed, "NEEDS REVIEW")
	}
	return fmt.Sprintf("%s — %s · %s · %s  (%d claims)", verdict, ok, drift, errs, sum.Total)
}

// jsonResult is the stable machine shape emitted by JSON. Separate from
// claim.Result so the wire format is an explicit contract, not an accident of
// internal struct layout.
type jsonResult struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Match        string `json:"match"`
	Where        string `json:"where"`
	Expect       string `json:"expect"`
	FilesMatched int    `json:"filesMatched"`
	Hits         int    `json:"hits"`
	Detail       string `json:"detail,omitempty"`
}

type jsonReport struct {
	Summary struct {
		Total  int  `json:"total"`
		OK     int  `json:"ok"`
		Drift  int  `json:"drift"`
		Errors int  `json:"errors"`
		Clean  bool `json:"clean"`
	} `json:"summary"`
	Results []jsonResult `json:"results"`
}

// JSON writes machine-readable results to w (for CI, editors, the PR bot).
func JSON(w io.Writer, results []claim.Result, sum runner.Summary) error {
	var out jsonReport
	out.Summary.Total = sum.Total
	out.Summary.OK = sum.OK
	out.Summary.Drift = sum.Drift
	out.Summary.Errors = sum.Errors
	out.Summary.Clean = sum.Clean()
	out.Results = make([]jsonResult, 0, len(results))
	for _, r := range results {
		out.Results = append(out.Results, jsonResult{
			ID:           r.Claim.ID,
			Status:       string(r.Status),
			Match:        string(r.Claim.Match),
			Where:        r.Claim.Where,
			Expect:       r.Claim.Expect,
			FilesMatched: r.FilesMatched,
			Hits:         r.Hits,
			Detail:       r.Detail,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// ── Doc stamping (`report --write`) ──────────────────────────────────────
//
// driftdoc can write a live status block INTO a doc so a reader sees, at the
// top of the doc, exactly which documented claims are still verified. The block
// is wrapped in HTML-comment markers so re-running overwrites it in place
// (idempotent), and is plain GFM so it renders in filemark, GitHub, or VS Code.

const (
	// StatusMarkerOpen / StatusMarkerClose fence the generated block. A doc may
	// contain at most one; re-stamping replaces everything between them.
	StatusMarkerOpen  = "<!-- driftdoc:status -->"
	StatusMarkerClose = "<!-- /driftdoc:status -->"
)

// statusEmoji maps a status to the glyph used in the stamped doc table.
var statusEmoji = map[claim.Status]string{
	claim.StatusOK:    "✅",
	claim.StatusDrift: "❌",
	claim.StatusError: "⚠️",
}

// BuildStatusBlock renders the marker-wrapped status block for one doc's claims:
// a one-line verdict banner + a per-claim table. `now` is injected (not read
// from the clock here) so callers/tests control the stamped date.
func BuildStatusBlock(results []claim.Result, sum runner.Summary, now time.Time) string {
	var b strings.Builder
	b.WriteString(StatusMarkerOpen + "\n")
	b.WriteString("<!-- generated by driftdoc — do not edit by hand; run `driftdoc report --write` -->\n")
	verdict := "✅ ALL CURRENT"
	if !sum.Clean() {
		verdict = "⚠️ NEEDS REVIEW"
	}
	fmt.Fprintf(&b, "**📊 driftdoc — verified %s:** %s · %d ok · %d drift · %d error (%d claims)\n\n",
		now.Format("2006-01-02"), verdict, sum.OK, sum.Drift, sum.Errors, sum.Total)
	b.WriteString("| Claim | Status | Check |\n|---|---|---|\n")
	for _, r := range results {
		emoji := statusEmoji[r.Status]
		check := fmt.Sprintf("`%s` %s", r.Claim.Match, codeSpan(r.Claim.Expect))
		fmt.Fprintf(&b, "| `%s` — %s | %s %s | %s |\n", r.Claim.ID, escapePipes(r.Claim.Desc), emoji, r.Status, check)
	}
	b.WriteString(StatusMarkerClose)
	return b.String()
}

// StampDoc writes block into the doc at path: it replaces an existing marker
// block in place, or (first time) inserts it just after the doc's first H1.
// Returns changed=true only when the file content actually differs, so `report`
// can report a clean no-op. now is the stamp date for the (re)generated block.
func StampDoc(path, block string) (changed bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("report: reading %s: %w", path, err)
	}
	orig := string(raw)

	var next string
	if oi := strings.Index(orig, StatusMarkerOpen); oi >= 0 {
		ci := strings.Index(orig, StatusMarkerClose)
		if ci < 0 || ci < oi {
			return false, fmt.Errorf("report: %s has an opening driftdoc marker but no matching close", path)
		}
		ci += len(StatusMarkerClose)
		next = orig[:oi] + block + orig[ci:]
	} else {
		next = insertAfterH1(orig, block)
	}
	if next == orig {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, fmt.Errorf("report: writing %s: %w", path, err)
	}
	return true, nil
}

// insertAfterH1 places block right after the doc's first level-1 heading (with
// blank lines around it). If there's no H1, the block is prepended.
func insertAfterH1(doc, block string) string {
	lines := strings.Split(doc, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, "# ") {
			head := strings.Join(lines[:i+1], "\n")
			tail := strings.Join(lines[i+1:], "\n")
			return head + "\n\n" + block + "\n" + tail
		}
	}
	return block + "\n\n" + doc
}

// escapePipes keeps a claim description from breaking the markdown table.
func escapePipes(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

// codeSpan wraps s in backticks, trimming to keep the table compact.
func codeSpan(s string) string {
	const max = 40
	if len(s) > max {
		s = s[:max-1] + "…"
	}
	return "`" + s + "`"
}

// painter returns a colorizer: when on, wraps text in code+reset; when off,
// returns text unchanged. Centralizing this keeps every call site color-safe.
func painter(on bool) func(code, text string) string {
	if !on {
		return func(_, text string) string { return text }
	}
	return func(code, text string) string { return code + text + ansiReset }
}
