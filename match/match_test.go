package match_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thesatellite-ai/driftdoc/claim"
	"github.com/thesatellite-ai/driftdoc/match"
)

// writeFixture creates a temp dir with the given files and returns its path.
func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestRun(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"a.go":       "package a\nconst URL = \"/api/admin/v1/query\"\n",
		"sub/b.go":   "package b\nfunc Foo() {}\nfunc Foo2() {}\n",
		"sub/c.txt":  "no robot here\n",
		"removed.go": "package r\n// nothing forbidden\n",
	})

	cases := []struct {
		name string
		c    claim.Claim
		want claim.Status
	}{
		{
			name: "contains present → ok",
			c:    claim.Claim{ID: "c1", Where: "a.go", Match: claim.MatchContains, Expect: "/api/admin/v1/query"},
			want: claim.StatusOK,
		},
		{
			name: "contains missing → drift",
			c:    claim.Claim{ID: "c2", Where: "a.go", Match: claim.MatchContains, Expect: "/api/admin/query"},
			want: claim.StatusDrift,
		},
		{
			name: "absent across tree → ok",
			c:    claim.Claim{ID: "c3", Where: "**/*.go", Match: claim.MatchAbsent, Expect: "EnsureSystemUser"},
			want: claim.StatusOK,
		},
		{
			name: "absent but present → drift",
			c:    claim.Claim{ID: "c4", Where: "**/*.go", Match: claim.MatchAbsent, Expect: "package"},
			want: claim.StatusDrift,
		},
		{
			name: "regex present → ok",
			c:    claim.Claim{ID: "c5", Where: "sub/b.go", Match: claim.MatchRegex, Expect: `func\s+Foo\d*\(`},
			want: claim.StatusOK,
		},
		{
			name: "count exact → ok",
			c:    claim.Claim{ID: "c6", Where: "sub/b.go", Match: claim.MatchCount, Expect: "func ", Count: 2},
			want: claim.StatusOK,
		},
		{
			name: "count wrong → drift",
			c:    claim.Claim{ID: "c7", Where: "sub/b.go", Match: claim.MatchCount, Expect: "func ", Count: 3},
			want: claim.StatusDrift,
		},
		{
			name: "glob matches nothing (non-absent) → error",
			c:    claim.Claim{ID: "c8", Where: "nope/**/*.go", Match: claim.MatchContains, Expect: "x"},
			want: claim.StatusError,
		},
		{
			name: "bad regex → error",
			c:    claim.Claim{ID: "c9", Where: "a.go", Match: claim.MatchRegex, Expect: "("},
			want: claim.StatusError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := match.Run(tc.c, root)
			if got.Status != tc.want {
				t.Fatalf("status = %q (detail: %s); want %q", got.Status, got.Detail, tc.want)
			}
		})
	}
}
