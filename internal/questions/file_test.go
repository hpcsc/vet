//go:build unit

package questions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	const example = `
version: 1
rules:
  - id: no-flag-field
    instructions: |
      The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5

  - id: database-migration
    instructions: |
      Which option describes the change best?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates

  - id: log-guideline
    instructions: |
      Rate how well the change follows the logging guideline.
    type: score
    scores:
      - Logs with slog
      - Logs directly to stdout
      - Adds or keeps prohibited logging
    scoreLimit: 2
`

	t.Run("parse", func(t *testing.T) {
		t.Run("loads all three rule kinds with their fields", func(t *testing.T) {
			file, err := Parse([]byte(example), "")

			require.NoError(t, err)
			require.Equal(t, 1, file.Version)
			require.Len(t, file.Rules, 3)
			require.Equal(t, "no-flag-field", file.Rules[0].ID)
			require.Equal(t, Noul, file.Rules[0].Type)
			require.Equal(t, 0.5, *file.Rules[0].NoulLimit)
			require.Equal(t, Choice, file.Rules[1].Type)
			require.Equal(t, map[string]string{
				"no-db":    "The change does not touch the database.",
				"uses-db":  "The change reads or writes the database.",
				"migrates": "The change alters the schema.",
			}, file.Rules[1].Choices)
			require.Equal(t, "migrates", file.Rules[1].ViolatesWhen)
			require.Equal(t, Score, file.Rules[2].Type)
			require.Equal(t, []string{"Logs with slog", "Logs directly to stdout", "Adds or keeps prohibited logging"}, file.Rules[2].Scores)
			require.Equal(t, 2, *file.Rules[2].ScoreLimit)
		})

		t.Run("rejects a version that is not 1", func(t *testing.T) {
			_, err := Parse([]byte("version: 2\nrules: []"), "")

			require.Error(t, err)
		})

		t.Run("rejects an invalid top-level files pattern", func(t *testing.T) {
			_, err := Parse([]byte("version: 1\nfiles:\n  - \"[bad\"\nrules: []"), "")

			require.ErrorContains(t, err, "files pattern")
		})

		t.Run("rejects an invalid top-level exclude pattern", func(t *testing.T) {
			_, err := Parse([]byte("version: 1\nexclude:\n  - \"[bad\"\nrules: []"), "")

			require.ErrorContains(t, err, "exclude pattern")
		})

		t.Run("rejects text that is not yaml", func(t *testing.T) {
			_, err := Parse([]byte("version: ["), "")

			require.Error(t, err)
		})
	})

	t.Run("references", func(t *testing.T) {
		write := func(t *testing.T, dir string, name, content string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
		}

		t.Run("a context that names a file loads the file beside the questions file", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "guidelines.md", "use slog")
			path := filepath.Join(dir, "questions.yaml")
			write(t, dir, "questions.yaml", "version: 1\ncontext: \"@guidelines.md\"\nrules:\n  - id: only-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")

			file, err := Load(path)

			require.NoError(t, err)
			require.Equal(t, "use slog", file.Context)
		})

		t.Run("an instruction that names a file loads the file beside the questions file", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "questions.yaml")
			write(t, dir, "explain.md", "tells what changed")
			write(t, dir, "questions.yaml", "version: 1\nrules:\n  - id: only-rule\n    instructions: \"@explain.md\"\n    type: noul\n    noulLimit: 0.5")

			file, err := Load(path)

			require.NoError(t, err)
			require.Equal(t, "tells what changed", file.Rules[0].Instructions)
		})

		t.Run("a ~ path expands to the home directory", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "guidelines.md", "use slog")
			t.Setenv("HOME", dir)
			text := "version: 1\ncontext: \"@~/guidelines.md\"\nrules: []"

			file, err := Parse([]byte(text), "")

			require.NoError(t, err)
			require.Equal(t, "use slog", file.Context)
		})

		t.Run("a reference to a missing file is an error naming the rule", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "questions.yaml")
			write(t, dir, "questions.yaml", "version: 1\nrules:\n  - id: only-rule\n    instructions: \"@missing.md\"\n    type: noul\n    noulLimit: 0.5")

			_, err := Load(path)

			require.Error(t, err)
			require.ErrorContains(t, err, "only-rule")
			require.ErrorContains(t, err, "missing.md")
		})

		t.Run("a value that does not start with @ stays literal", func(t *testing.T) {
			text := "version: 1\nrules:\n  - id: only-rule\n    instructions: Review the @-sign usage\n    type: noul\n    noulLimit: 0.5"

			file, err := Parse([]byte(text), "")

			require.NoError(t, err)
			require.Equal(t, "Review the @-sign usage", file.Rules[0].Instructions)
		})
	})

	t.Run("for-path", func(t *testing.T) {
		file, err := Parse([]byte(example), "")
		require.NoError(t, err)
		for i := range file.Rules {
			file.Rules[i].Files = []string{"**/*.go"}
		}

		t.Run("keeps every rule that applies to the path", func(t *testing.T) {
			scoped, ok := file.ForPath("internal/repo.go")

			require.True(t, ok)
			require.Equal(t, []string{"no-flag-field", "database-migration", "log-guideline"}, ruleIDs(scoped))
		})

		t.Run("a path no rule applies to yields nothing", func(t *testing.T) {
			_, ok := file.ForPath("README.md")

			require.False(t, ok)
		})
	})

	t.Run("file scope", func(t *testing.T) {
		const scoped = `version: 1
files:
  - "**/*.go"
exclude:
  - "**/*_test.go"
rules:
  - id: inherited
    instructions: does it?
    type: noul
    noulLimit: 0.5
  - id: own-files
    instructions: does it?
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"
  - id: own-exclude
    instructions: does it?
    type: noul
    noulLimit: 0.5
    exclude:
      - "**/mocks/**"
`

		t.Run("a rule without its own scope inherits the file scope", func(t *testing.T) {
			file, err := Parse([]byte(scoped), "")

			require.NoError(t, err)
			rule := file.Rules[0]
			require.Equal(t, []string{"**/*.go"}, rule.Files)
			require.Equal(t, []string{"**/*_test.go"}, rule.Exclude)
		})

		t.Run("a rule with its own files overrides the file files", func(t *testing.T) {
			file, err := Parse([]byte(scoped), "")

			require.NoError(t, err)
			rule := file.Rules[1]
			require.Equal(t, []string{"**/*_test.go"}, rule.Files)
			require.Equal(t, []string{"**/*_test.go"}, rule.Exclude)
		})

		t.Run("a rule keeps its own exclude alongside the file exclude", func(t *testing.T) {
			file, err := Parse([]byte(scoped), "")

			require.NoError(t, err)
			rule := file.Rules[2]
			require.Equal(t, []string{"**/*.go"}, rule.Files)
			require.Equal(t, []string{"**/*_test.go", "**/mocks/**"}, rule.Exclude)
		})

		t.Run("a path the file scope rejects applies to no rule", func(t *testing.T) {
			file, err := Parse([]byte(scoped), "")

			require.NoError(t, err)
			_, ok := file.ForPath("README.md")
			require.False(t, ok)
		})

		t.Run("a path the file exclude rejects applies to no rule", func(t *testing.T) {
			file, err := Parse([]byte(scoped), "")

			require.NoError(t, err)
			_, ok := file.ForPath("internal/repo_test.go")
			require.False(t, ok)
		})
	})

	t.Run("load", func(t *testing.T) {
		write := func(t *testing.T, dir string, name, content string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
		}

		t.Run("a single file loads its rules", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "questions.yaml")
			write(t, dir, "questions.yaml", "version: 1\nrules:\n  - id: only-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")

			file, err := Load(path)

			require.NoError(t, err)
			require.Len(t, file.Rules, 1)
			require.Equal(t, "only-rule", file.Rules[0].ID)
		})

		t.Run("labels a single file's rules with its name field, or the file name", func(t *testing.T) {
			t.Run("a file that names itself uses that name", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "rules.yaml")
				write(t, dir, "rules.yaml", "version: 1\nname: the rules\nrules:\n  - id: only-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")

				file, err := Load(path)

				require.NoError(t, err)
				require.Equal(t, "the rules", file.Rules[0].Source)
			})

			t.Run("a file without a name uses the file name", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "rules.yaml")
				write(t, dir, "rules.yaml", "version: 1\nrules:\n  - id: only-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")

				file, err := Load(path)

				require.NoError(t, err)
				require.Equal(t, "rules.yaml", file.Rules[0].Source)
			})

			t.Run("a directory file's rules use the file's own name", func(t *testing.T) {
				dir := t.TempDir()
				write(t, dir, "named.yaml", "version: 1\nname: naming rules\nrules:\n  - id: first\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
				write(t, dir, "plain.yaml", "version: 1\nrules:\n  - id: second\n    instructions: how good?\n    type: noul\n    noulLimit: 0.5")

				file, err := Load(dir)

				require.NoError(t, err)
				require.Equal(t, "naming rules", file.Rules[0].Source)
				require.Equal(t, "plain.yaml", file.Rules[1].Source)
			})
		})

		t.Run("a directory merges every questions file in filename order", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yaml", "version: 1\nrules:\n  - id: first\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
			write(t, dir, "b.yaml", "version: 1\nrules:\n  - id: second\n    instructions: how good?\n    type: score\n    scores:\n      - good\n      - bad\n    scoreLimit: 1")

			file, err := Load(dir)

			require.NoError(t, err)
			require.Equal(t, 1, file.Version)
			require.Len(t, file.Rules, 2)
			require.Equal(t, []string{"first", "second"}, []string{file.Rules[0].ID, file.Rules[1].ID})
		})

		t.Run("a directory merges the contexts of its questions files", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yaml", "version: 1\ncontext: use slog\nrules:\n  - id: first\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
			write(t, dir, "b.yaml", "version: 1\ncontext: keep secrets\nrules:\n  - id: second\n    instructions: how good?\n    type: noul\n    noulLimit: 0.5")

			file, err := Load(dir)

			require.NoError(t, err)
			require.Equal(t, "use slog\n\nkeep secrets", file.Context)
		})

		t.Run("a directory accepts yaml and yml and skips the rest", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yml", "version: 1\nrules:\n  - id: first\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
			write(t, dir, "notes.txt", "not questions")

			file, err := Load(dir)

			require.NoError(t, err)
			require.Len(t, file.Rules, 1)
		})

		t.Run("a directory with no questions files is an error", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "notes.txt", "not questions")

			_, err := Load(dir)

			require.ErrorContains(t, err, "no questions files")
		})

		t.Run("an invalid file makes the load fail with the file name", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yaml", "version: 3\nrules: []")
			write(t, dir, "b.yaml", "version: 1\nrules:\n  - id: fine\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")

			_, err := Load(dir)

			require.ErrorContains(t, err, "a.yaml")
		})

		t.Run("a directory folds each file's scope onto its own rules", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yaml", "version: 1\nfiles:\n  - \"**/*.go\"\n  - \"**/*_test.go\"\nrules:\n  - id: go-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
			write(t, dir, "b.yaml", "version: 1\nfiles:\n  - \"**/*_test.go\"\nrules:\n  - id: test-rule\n    instructions: how good?\n    type: noul\n    noulLimit: 0.5")

			file, err := Load(dir)

			require.NoError(t, err)
			scoped, ok := file.ForPath("internal/repo.go")
			require.True(t, ok)
			require.Equal(t, []string{"go-rule"}, ruleIDs(scoped))
			scoped, ok = file.ForPath("internal/repo_test.go")
			require.True(t, ok)
			require.Equal(t, []string{"go-rule", "test-rule"}, ruleIDs(scoped))
		})

		t.Run("an id in two files is rejected", func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.yaml", "version: 1\nrules:\n  - id: same\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5")
			write(t, dir, "b.yaml", "version: 1\nrules:\n  - id: same\n    instructions: how good?\n    type: noul\n    noulLimit: 1")

			_, err := Load(dir)

			require.ErrorContains(t, err, "more than one")
		})
	})
}

func ruleIDs(file File) []string {
	ids := make([]string, len(file.Rules))
	for i, rule := range file.Rules {
		ids[i] = rule.ID
	}
	return ids
}