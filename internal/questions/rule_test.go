//go:build unit

package questions

import (
	"fmt"
	"testing"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/stretchr/testify/require"
)

func TestRule(t *testing.T) {
	parse := func(t *testing.T, yaml string) error {
		t.Helper()

		_, err := Parse([]byte(yaml), "")
		return err
	}

	t.Run("validate", func(t *testing.T) {
		t.Run("a rule with no id is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - type: noul\n    noulLimit: 0.5")

			require.ErrorContains(t, err, "id")
		})

		t.Run("a rule with empty instructions is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    type: noul\n    noulLimit: 0.5")

			require.ErrorContains(t, err, "instructions")
		})

		t.Run("a noulLimit below 0 is rejected", func(t *testing.T) {
			err := parse(t, noulRule(-0.1))

			require.ErrorContains(t, err, "noulLimit")
		})

		t.Run("a noulLimit above 1 is rejected", func(t *testing.T) {
			err := parse(t, noulRule(1.1))

			require.ErrorContains(t, err, "noulLimit")
		})

		t.Run("a violationsWhen that is not a choice key is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: what changed?\n    type: choice\n    choices:\n      no-db: no database\n    violatesWhen: schema")

			require.ErrorContains(t, err, "violatesWhen")
		})

		t.Run("a scoreLimit outside the scores range is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: how good?\n    type: score\n    scores:\n      - good\n      - bad\n    scoreLimit: 3")

			require.ErrorContains(t, err, "scoreLimit")
		})

		t.Run("an unknown type is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: what changed?\n    type: number")

			require.ErrorContains(t, err, "type")
		})

		t.Run("an invalid files pattern is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5\n    files:\n      - \"[bad\"")

			require.ErrorContains(t, err, "files pattern")
		})

		t.Run("an invalid exclude pattern is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5\n    exclude:\n      - \"[bad\"")

			require.ErrorContains(t, err, "exclude pattern")
		})

		t.Run("an invalid requiresAddedLine pattern is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5\n    requiresAddedLine: \"[bad\"")

			require.ErrorContains(t, err, "requiresAddedLine")
		})

		t.Run("an invalid requiresRemovedLine pattern is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5\n    requiresRemovedLine: \"[bad\"")

			require.ErrorContains(t, err, "requiresRemovedLine")
		})
	})

	t.Run("applies-to", func(t *testing.T) {
		t.Run("a rule with no files applies to every path", func(t *testing.T) {
			rule := Rule{ID: "a-rule"}

			require.True(t, rule.AppliesTo(diff.File{Path: "README.md"}))
			require.True(t, rule.AppliesTo(diff.File{Path: "internal/repo.go"}))
		})

		t.Run("a files pattern crosses directories", func(t *testing.T) {
			rule := Rule{ID: "a-rule", Files: []string{"**/*.go"}}

			require.True(t, rule.AppliesTo(diff.File{Path: "internal/repo.go"}))
			require.True(t, rule.AppliesTo(diff.File{Path: "repo.go"}))
			require.False(t, rule.AppliesTo(diff.File{Path: "README.md"}))
		})

		t.Run("an exclude pattern removes a path the files pattern matched", func(t *testing.T) {
			rule := Rule{ID: "a-rule", Files: []string{"**/*.go"}, Exclude: []string{"**/*_test.go"}}

			require.False(t, rule.AppliesTo(diff.File{Path: "internal/repo_test.go"}))
			require.True(t, rule.AppliesTo(diff.File{Path: "internal/repo.go"}))
		})

		t.Run("an exclude alone applies to every path but the matched ones", func(t *testing.T) {
			rule := Rule{ID: "a-rule", Exclude: []string{"**/*_test.go"}}

			require.False(t, rule.AppliesTo(diff.File{Path: "internal/repo_test.go"}))
			require.True(t, rule.AppliesTo(diff.File{Path: "README.md"}))
		})
	})

	t.Run("needs", func(t *testing.T) {
		change := func(patch string) diff.File {
			return diff.File{Path: "repo.go", Diff: patch}
		}

		t.Run("a rule with no requirement applies to a change with no lines", func(t *testing.T) {
			rule := Rule{ID: "a-rule"}

			require.True(t, rule.AppliesTo(change("")))
		})

		t.Run("requiresAddedLine skips a change that adds no matching line", func(t *testing.T) {
			rule := Rule{ID: "a-rule", RequiresAddedLine: `^func Test`}

			require.False(t, rule.AppliesTo(change("@@ -0,0 +1 @@\n+package repo\n")))
			require.True(t, rule.AppliesTo(change("@@ -0,0 +1 @@\n+func TestX(t *testing.T) {\n")))
		})

		t.Run("requiresAddedLine ignores a matching line the change only removes", func(t *testing.T) {
			rule := Rule{ID: "a-rule", RequiresAddedLine: `^func Test`}

			require.False(t, rule.AppliesTo(change("@@ -1 +0,0 @@\n-func TestX(t *testing.T) {\n")))
		})

		t.Run("requiresRemovedLine skips a change that removes no matching line", func(t *testing.T) {
			rule := Rule{ID: "a-rule", RequiresRemovedLine: `^\s*//`}

			require.False(t, rule.AppliesTo(change("@@ -0,0 +1 @@\n+// a new comment\n")))
			require.True(t, rule.AppliesTo(change("@@ -1 +0,0 @@\n-// a comment that explained the code\n")))
		})

		t.Run("both requirements must be met", func(t *testing.T) {
			rule := Rule{ID: "a-rule",
				RequiresAddedLine:   `^\s*//`,
				RequiresRemovedLine: `^\s*//`,
			}

			require.False(t, rule.AppliesTo(change("@@ -0,0 +1 @@\n+// new\n")))
			require.False(t, rule.AppliesTo(change("@@ -1 +0,0 @@\n-// old\n")))
			require.True(t, rule.AppliesTo(change("@@ -1 +1 @@\n-// old\n+// new\n")))
		})

		t.Run("a requirement skips a change with no hunk at all", func(t *testing.T) {
			rule := Rule{ID: "a-rule", RequiresAddedLine: `.`}

			pure := change("diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n")

			require.False(t, rule.AppliesTo(pure))
		})
	})
}

func noulRule(limit float64) string {
	return fmt.Sprintf(`version: 1
rules:
  - id: a-rule
    instructions: does it?
    type: noul
    noulLimit: %g
`, limit)
}