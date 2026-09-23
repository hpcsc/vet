//go:build unit

package questions

import (
	"fmt"
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
			file, err := Parse([]byte(example))

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
			_, err := Parse([]byte("version: 2\nrules: []"))

			require.Error(t, err)
		})

		t.Run("rejects text that is not yaml", func(t *testing.T) {
			_, err := Parse([]byte("version: ["))

			require.Error(t, err)
		})
	})

	t.Run("validate", func(t *testing.T) {
		parse := func(t *testing.T, yaml string) error {
			t.Helper()

			_, err := Parse([]byte(yaml))
			return err
		}

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
